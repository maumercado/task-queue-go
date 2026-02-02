package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/maumercado/task-queue-go/internal/logger"
)

const (
	workerKeyPrefix     = "worker:"
	workerSetKey        = "workers:active"
	heartbeatKeySuffix  = ":heartbeat"
	workerInfoKeySuffix = ":info"
)

// WorkerInfo contains information about a worker
type WorkerInfo struct {
	ID            string    `json:"id"`
	State         string    `json:"state"`
	StartedAt     time.Time `json:"started_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ActiveTasks   int       `json:"active_tasks"`
	Concurrency   int       `json:"concurrency"`
	Version       string    `json:"version,omitempty"`
}

// Heartbeat manages worker heartbeat mechanism
type Heartbeat struct {
	client   *redis.Client
	workerID string
	interval time.Duration
	timeout  time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	info     *WorkerInfo
	infoMu   sync.RWMutex
}

// NewHeartbeat creates a new heartbeat manager
func NewHeartbeat(client *redis.Client, workerID string, interval, timeout time.Duration) *Heartbeat {
	return &Heartbeat{
		client:   client,
		workerID: workerID,
		interval: interval,
		timeout:  timeout,
		stopCh:   make(chan struct{}),
		info: &WorkerInfo{
			ID:        workerID,
			State:     "idle",
			StartedAt: time.Now().UTC(),
		},
	}
}

// Start begins sending heartbeats
func (h *Heartbeat) Start(ctx context.Context) {
	h.wg.Add(1)
	go h.heartbeatLoop(ctx)

	// Register worker
	h.register(ctx)

	logger.Info().
		Str("worker_id", h.workerID).
		Dur("interval", h.interval).
		Msg("heartbeat started")
}

// Stop stops sending heartbeats
func (h *Heartbeat) Stop() {
	close(h.stopCh)
	h.wg.Wait()

	// Deregister worker
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.deregister(ctx)

	logger.Info().Str("worker_id", h.workerID).Msg("heartbeat stopped")
}

// UpdateState updates the worker state
func (h *Heartbeat) UpdateState(state string) {
	h.infoMu.Lock()
	h.info.State = state
	h.infoMu.Unlock()
}

// UpdateActiveTasks updates the active task count
func (h *Heartbeat) UpdateActiveTasks(count int) {
	h.infoMu.Lock()
	h.info.ActiveTasks = count
	h.infoMu.Unlock()
}

// UpdateConcurrency updates the concurrency setting
func (h *Heartbeat) UpdateConcurrency(concurrency int) {
	h.infoMu.Lock()
	h.info.Concurrency = concurrency
	h.infoMu.Unlock()
}

func (h *Heartbeat) heartbeatLoop(ctx context.Context) {
	defer h.wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Send initial heartbeat
	h.sendHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.sendHeartbeat(ctx)
		}
	}
}

func (h *Heartbeat) sendHeartbeat(ctx context.Context) {
	heartbeatKey := h.heartbeatKey()
	now := time.Now().UTC()

	// Update heartbeat timestamp
	if err := h.client.Set(ctx, heartbeatKey, now.Unix(), h.timeout).Err(); err != nil {
		logger.Error().Err(err).Str("worker_id", h.workerID).Msg("failed to send heartbeat")
		return
	}

	// Update worker info
	h.infoMu.Lock()
	h.info.LastHeartbeat = now
	infoData, _ := json.Marshal(h.info)
	h.infoMu.Unlock()

	infoKey := h.infoKey()
	if err := h.client.Set(ctx, infoKey, infoData, h.timeout*2).Err(); err != nil {
		logger.Error().Err(err).Str("worker_id", h.workerID).Msg("failed to update worker info")
	}

	// Ensure worker is in active set
	h.client.SAdd(ctx, workerSetKey, h.workerID)
}

func (h *Heartbeat) register(ctx context.Context) {
	// Add to active workers set
	h.client.SAdd(ctx, workerSetKey, h.workerID)

	// Store initial info
	h.infoMu.Lock()
	h.info.StartedAt = time.Now().UTC()
	infoData, _ := json.Marshal(h.info)
	h.infoMu.Unlock()

	h.client.Set(ctx, h.infoKey(), infoData, h.timeout*2)
}

func (h *Heartbeat) deregister(ctx context.Context) {
	// Remove from active workers set
	h.client.SRem(ctx, workerSetKey, h.workerID)

	// Remove heartbeat and info keys
	h.client.Del(ctx, h.heartbeatKey(), h.infoKey())
}

func (h *Heartbeat) heartbeatKey() string {
	return fmt.Sprintf("%s%s%s", workerKeyPrefix, h.workerID, heartbeatKeySuffix)
}

func (h *Heartbeat) infoKey() string {
	return fmt.Sprintf("%s%s%s", workerKeyPrefix, h.workerID, workerInfoKeySuffix)
}

// GetActiveWorkers returns a list of active workers
func GetActiveWorkers(ctx context.Context, client *redis.Client) ([]WorkerInfo, error) {
	workerIDs, err := client.SMembers(ctx, workerSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get active workers: %w", err)
	}

	workers := make([]WorkerInfo, 0, len(workerIDs))
	for _, id := range workerIDs {
		infoKey := fmt.Sprintf("%s%s%s", workerKeyPrefix, id, workerInfoKeySuffix)
		data, err := client.Get(ctx, infoKey).Bytes()
		if err == redis.Nil {
			// Worker info expired, remove from set
			client.SRem(ctx, workerSetKey, id)
			continue
		}
		if err != nil {
			continue
		}

		var info WorkerInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		workers = append(workers, info)
	}

	return workers, nil
}

// IsWorkerAlive checks if a worker is still alive based on heartbeat
func IsWorkerAlive(ctx context.Context, client *redis.Client, workerID string) (bool, error) {
	heartbeatKey := fmt.Sprintf("%s%s%s", workerKeyPrefix, workerID, heartbeatKeySuffix)
	exists, err := client.Exists(ctx, heartbeatKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check worker heartbeat: %w", err)
	}
	return exists > 0, nil
}

// IsWorkerPaused checks if a worker has been paused via admin API
func IsWorkerPaused(ctx context.Context, client *redis.Client, workerID string) (bool, error) {
	pauseKey := fmt.Sprintf("%s%s:paused", workerKeyPrefix, workerID)
	exists, err := client.Exists(ctx, pauseKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check worker pause status: %w", err)
	}
	return exists > 0, nil
}
