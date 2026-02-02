package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/task"
)

// RedisQueue implements a priority queue using Redis Streams.
// Uses 4 separate streams (one per priority) for priority-based consumption.
type RedisQueue struct {
	client            *redis.Client
	streamPrefix      string        // Base name for streams (e.g., "tasks")
	consumerGroup     string        // Consumer group name for coordinated consumption
	blockTimeout      time.Duration // How long to block waiting for messages
	claimMinIdle      time.Duration // Min idle time before claiming orphaned messages
	taskRetentionDays int           // Days to retain completed tasks (0 = no expiry)
}

// NewRedisQueue creates a new Redis-backed queue and initializes streams
func NewRedisQueue(cfg *config.RedisConfig, queueCfg *config.QueueConfig) (*RedisQueue, error) {
	// Create Redis client with connection pooling
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Verify connection before proceeding
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	q := &RedisQueue{
		client:            client,
		streamPrefix:      queueCfg.StreamPrefix,
		consumerGroup:     queueCfg.ConsumerGroup,
		blockTimeout:      queueCfg.BlockTimeout,
		claimMinIdle:      queueCfg.ClaimMinIdle,
		taskRetentionDays: queueCfg.TaskRetentionDays,
	}

	// Create streams and consumer groups for each priority
	if err := q.initStreams(ctx); err != nil {
		return nil, err
	}

	return q, nil
}

// initStreams creates streams and consumer groups for all priority levels
func (q *RedisQueue) initStreams(ctx context.Context) error {
	priorities := []task.Priority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	for _, p := range priorities {
		streamName := p.StreamName(q.streamPrefix)
		// XGroupCreateMkStream creates both stream and group if they don't exist
		err := q.client.XGroupCreateMkStream(ctx, streamName, q.consumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("failed to create consumer group for %s: %w", streamName, err)
		}
	}

	return nil
}

// Enqueue adds a task to the appropriate priority stream.
// Stores full task data separately for efficient retrieval.
func (q *RedisQueue) Enqueue(ctx context.Context, t *task.Task) error {
	streamName := t.Priority.StreamName(q.streamPrefix)

	// Serialize task to JSON
	taskData, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Store full task data in a separate key (more efficient than embedding in stream)
	taskKey := q.taskKey(t.ID)
	if err := q.client.Set(ctx, taskKey, taskData, 0).Err(); err != nil {
		return fmt.Errorf("failed to store task data: %w", err)
	}

	// Add reference to stream (lightweight message with just ID and type)
	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"task_id": t.ID,
			"type":    t.Type,
		},
	}).Result()

	if err != nil {
		q.client.Del(ctx, taskKey) // Cleanup on failure
		return fmt.Errorf("failed to add task to stream: %w", err)
	}

	return nil
}

// Dequeue fetches the next task, checking priority queues from highest to lowest.
// Non-blocking: returns nil immediately if no tasks available.
func (q *RedisQueue) Dequeue(ctx context.Context, consumerID string) (*task.Task, string, error) {
	// Check queues in priority order: critical -> high -> normal -> low
	priorities := []task.Priority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	for _, p := range priorities {
		streamName := p.StreamName(q.streamPrefix)

		// XReadGroup with Block=0 is non-blocking
		streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.consumerGroup,
			Consumer: consumerID,
			Streams:  []string{streamName, ">"}, // ">" means only new messages
			Count:    1,
			Block:    0,
		}).Result()

		if err == redis.Nil {
			continue // No messages in this priority, try next
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to read from stream %s: %w", streamName, err)
		}

		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			continue
		}

		// Extract task ID from stream message
		msg := streams[0].Messages[0]
		taskID, ok := msg.Values["task_id"].(string)
		if !ok {
			// Invalid message format, acknowledge to remove from pending
			q.client.XAck(ctx, streamName, q.consumerGroup, msg.ID)
			continue
		}

		// Fetch full task data from storage
		t, err := q.GetTask(ctx, taskID)
		if err != nil {
			q.client.XAck(ctx, streamName, q.consumerGroup, msg.ID)
			continue
		}

		return t, msg.ID, nil
	}

	return nil, "", nil // No tasks available in any queue
}

// DequeueBlocking fetches the next task, blocking until one is available.
// Listens to all priority streams simultaneously but returns highest priority first.
func (q *RedisQueue) DequeueBlocking(ctx context.Context, consumerID string) (*task.Task, string, error) {
	priorities := []task.Priority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	// Build streams array: [stream1, stream2, ..., ">", ">", ...]
	streams := make([]string, 0, len(priorities)*2)
	for _, p := range priorities {
		streams = append(streams, p.StreamName(q.streamPrefix))
	}
	for range priorities {
		streams = append(streams, ">")
	}

	// Block until a message arrives on any stream
	result, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.consumerGroup,
		Consumer: consumerID,
		Streams:  streams,
		Count:    1,
		Block:    q.blockTimeout,
	}).Result()

	if err == redis.Nil {
		return nil, "", nil // Timeout, no messages
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to read from streams: %w", err)
	}

	if len(result) == 0 || len(result[0].Messages) == 0 {
		return nil, "", nil
	}

	// Process first received message
	msg := result[0].Messages[0]
	streamName := result[0].Stream
	taskID, ok := msg.Values["task_id"].(string)
	if !ok {
		q.client.XAck(ctx, streamName, q.consumerGroup, msg.ID)
		return nil, "", nil
	}

	t, err := q.GetTask(ctx, taskID)
	if err != nil {
		q.client.XAck(ctx, streamName, q.consumerGroup, msg.ID)
		return nil, "", nil
	}

	return t, msg.ID, nil
}

// Acknowledge marks a message as successfully processed, removing from pending list
func (q *RedisQueue) Acknowledge(ctx context.Context, t *task.Task, messageID string) error {
	streamName := t.Priority.StreamName(q.streamPrefix)
	return q.client.XAck(ctx, streamName, q.consumerGroup, messageID).Err()
}

// GetTask retrieves a task by ID from storage
func (q *RedisQueue) GetTask(ctx context.Context, taskID string) (*task.Task, error) {
	taskKey := q.taskKey(taskID)
	data, err := q.client.Get(ctx, taskKey).Bytes()
	if err == redis.Nil {
		return nil, task.ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var t task.Task
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &t, nil
}

// UpdateTask updates task data in storage
func (q *RedisQueue) UpdateTask(ctx context.Context, t *task.Task) error {
	taskKey := q.taskKey(t.ID)
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// If task is in terminal state and retention is configured, set TTL
	if t.State.IsFinal() && q.taskRetentionDays > 0 {
		ttl := time.Duration(q.taskRetentionDays) * 24 * time.Hour
		return q.client.Set(ctx, taskKey, data, ttl).Err()
	}

	return q.client.Set(ctx, taskKey, data, 0).Err()
}

// UpdateTaskWithTTL updates task data with a specific TTL
func (q *RedisQueue) UpdateTaskWithTTL(ctx context.Context, t *task.Task, ttl time.Duration) error {
	taskKey := q.taskKey(t.ID)
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	return q.client.Set(ctx, taskKey, data, ttl).Err()
}

// GetRetentionTTL returns the configured task retention TTL
func (q *RedisQueue) GetRetentionTTL() time.Duration {
	if q.taskRetentionDays <= 0 {
		return 0
	}
	return time.Duration(q.taskRetentionDays) * 24 * time.Hour
}

// DeleteTask removes task data from storage
func (q *RedisQueue) DeleteTask(ctx context.Context, taskID string) error {
	taskKey := q.taskKey(taskID)
	return q.client.Del(ctx, taskKey).Err()
}

// GetQueueDepth returns pending message count for each priority queue
func (q *RedisQueue) GetQueueDepth(ctx context.Context) (map[task.Priority]int64, error) {
	depths := make(map[task.Priority]int64)
	priorities := []task.Priority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	for _, p := range priorities {
		streamName := p.StreamName(q.streamPrefix)
		// Get consumer group info which includes pending count
		info, err := q.client.XInfoGroups(ctx, streamName).Result()
		if err != nil {
			continue // Stream may not exist yet
		}

		for _, group := range info {
			if group.Name == q.consumerGroup {
				depths[p] = group.Pending
				break
			}
		}
	}

	return depths, nil
}

// ClaimOrphanedTasks claims messages from crashed workers using XCLAIM.
// Messages idle longer than claimMinIdle are considered orphaned.
func (q *RedisQueue) ClaimOrphanedTasks(ctx context.Context, consumerID string) ([]*task.Task, []string, error) {
	var tasks []*task.Task
	var messageIDs []string

	priorities := []task.Priority{
		task.PriorityCritical,
		task.PriorityHigh,
		task.PriorityNormal,
		task.PriorityLow,
	}

	for _, p := range priorities {
		streamName := p.StreamName(q.streamPrefix)

		// Get all pending messages in the consumer group
		pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: streamName,
			Group:  q.consumerGroup,
			Start:  "-",
			End:    "+",
			Count:  100,
		}).Result()

		if err != nil {
			continue
		}

		for _, p := range pending {
			// Only claim messages that have been idle too long
			if p.Idle < q.claimMinIdle {
				continue
			}

			// XCLAIM transfers ownership of the message to this consumer
			claimed, err := q.client.XClaim(ctx, &redis.XClaimArgs{
				Stream:   streamName,
				Group:    q.consumerGroup,
				Consumer: consumerID,
				MinIdle:  q.claimMinIdle,
				Messages: []string{p.ID},
			}).Result()

			if err != nil || len(claimed) == 0 {
				continue
			}

			msg := claimed[0]
			taskID, ok := msg.Values["task_id"].(string)
			if !ok {
				continue
			}

			t, err := q.GetTask(ctx, taskID)
			if err != nil {
				continue
			}

			tasks = append(tasks, t)
			messageIDs = append(messageIDs, msg.ID)
		}
	}

	return tasks, messageIDs, nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// Client returns the underlying Redis client for direct access
func (q *RedisQueue) Client() *redis.Client {
	return q.client
}

// taskKey generates the storage key for a task
func (q *RedisQueue) taskKey(taskID string) string {
	return fmt.Sprintf("task:%s", taskID)
}
