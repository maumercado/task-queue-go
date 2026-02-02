package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/task"
)

const (
	scheduledSetKey = "tasks:scheduled"
	schedulerLockKey = "scheduler:lock"
	schedulerPollInterval = 1 * time.Second
	schedulerLockTTL = 5 * time.Second
)

// Scheduler polls the scheduled tasks set and moves due tasks to priority queues
type Scheduler struct {
	client       *redis.Client
	queue        *RedisQueue
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler(client *redis.Client, queue *RedisQueue) *Scheduler {
	return &Scheduler{
		client:       client,
		queue:        queue,
		pollInterval: schedulerPollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the scheduler loop
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.schedulerLoop(ctx)

	logger.Info().
		Dur("poll_interval", s.pollInterval).
		Msg("scheduler started")
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.Info().Msg("scheduler stopped")
}

func (s *Scheduler) schedulerLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processDueTasks(ctx)
		}
	}
}

func (s *Scheduler) processDueTasks(ctx context.Context) {
	// Try to acquire distributed lock to prevent multiple schedulers from processing
	locked, err := s.client.SetNX(ctx, schedulerLockKey, "1", schedulerLockTTL).Result()
	if err != nil || !locked {
		return // Another scheduler instance is processing
	}
	defer s.client.Del(ctx, schedulerLockKey)

	now := time.Now().UTC().Unix()

	// Get all tasks scheduled to run at or before now
	// ZRANGEBYSCORE tasks:scheduled -inf <now>
	taskIDs, err := s.client.ZRangeByScore(ctx, scheduledSetKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()

	if err != nil {
		logger.Error().Err(err).Msg("failed to get due tasks")
		return
	}

	if len(taskIDs) == 0 {
		return
	}

	logger.Debug().Int("count", len(taskIDs)).Msg("processing due scheduled tasks")

	for _, taskID := range taskIDs {
		if err := s.activateTask(ctx, taskID); err != nil {
			logger.Error().Err(err).Str("task_id", taskID).Msg("failed to activate scheduled task")
			continue
		}
	}
}

func (s *Scheduler) activateTask(ctx context.Context, taskID string) error {
	// Get task data
	t, err := s.queue.GetTask(ctx, taskID)
	if err != nil {
		// Task was deleted, remove from scheduled set
		s.client.ZRem(ctx, scheduledSetKey, taskID)
		return nil
	}

	// Transition from scheduled to pending
	if t.State != task.StateScheduled {
		// Task is no longer in scheduled state, remove from set
		s.client.ZRem(ctx, scheduledSetKey, taskID)
		return nil
	}

	sm := task.NewStateMachine(t)
	if err := sm.Transition(task.StatePending); err != nil {
		return fmt.Errorf("failed to transition task: %w", err)
	}

	// Update task in storage
	if err := s.queue.UpdateTask(ctx, t); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Add to appropriate priority stream
	streamName := t.Priority.StreamName(s.queue.streamPrefix)
	_, err = s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"task_id": t.ID,
			"type":    t.Type,
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add task to stream: %w", err)
	}

	// Remove from scheduled set
	s.client.ZRem(ctx, scheduledSetKey, taskID)

	logger.Info().
		Str("task_id", taskID).
		Str("type", t.Type).
		Str("priority", t.Priority.String()).
		Msg("scheduled task activated")

	return nil
}

// ScheduleTask adds a task to the scheduled set
func (s *Scheduler) ScheduleTask(ctx context.Context, t *task.Task, scheduledAt time.Time) error {
	// Store task data
	taskData, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	taskKey := fmt.Sprintf("task:%s", t.ID)
	if err := s.client.Set(ctx, taskKey, taskData, 0).Err(); err != nil {
		return fmt.Errorf("failed to store task data: %w", err)
	}

	// Add to scheduled sorted set with score = scheduled time
	err = s.client.ZAdd(ctx, scheduledSetKey, redis.Z{
		Score:  float64(scheduledAt.Unix()),
		Member: t.ID,
	}).Err()

	if err != nil {
		s.client.Del(ctx, taskKey) // Cleanup on failure
		return fmt.Errorf("failed to add task to scheduled set: %w", err)
	}

	return nil
}

// ScheduleTaskFunc returns a function that can schedule tasks (for use in handlers)
func ScheduleTaskFunc(client *redis.Client) func(ctx context.Context, t *task.Task, scheduledAt time.Time) error {
	return func(ctx context.Context, t *task.Task, scheduledAt time.Time) error {
		// Store task data
		taskData, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("failed to marshal task: %w", err)
		}

		taskKey := fmt.Sprintf("task:%s", t.ID)
		if err := client.Set(ctx, taskKey, taskData, 0).Err(); err != nil {
			return fmt.Errorf("failed to store task data: %w", err)
		}

		// Add to scheduled sorted set with score = scheduled time
		err = client.ZAdd(ctx, scheduledSetKey, redis.Z{
			Score:  float64(scheduledAt.Unix()),
			Member: t.ID,
		}).Err()

		if err != nil {
			client.Del(ctx, taskKey) // Cleanup on failure
			return fmt.Errorf("failed to add task to scheduled set: %w", err)
		}

		return nil
	}
}

// GetScheduledCount returns the number of scheduled tasks
func GetScheduledCount(ctx context.Context, client *redis.Client) (int64, error) {
	return client.ZCard(ctx, scheduledSetKey).Result()
}

// RemoveScheduledTask removes a task from the scheduled set
func RemoveScheduledTask(ctx context.Context, client *redis.Client, taskID string) error {
	return client.ZRem(ctx, scheduledSetKey, taskID).Err()
}
