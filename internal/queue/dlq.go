package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/maumercado/task-queue-go/internal/task"
)

const (
	dlqStreamName = "tasks:dlq"
	dlqSetName    = "tasks:dlq:set"
)

// DLQ represents a Dead Letter Queue for failed tasks
type DLQ struct {
	client *redis.Client
}

// NewDLQ creates a new Dead Letter Queue
func NewDLQ(client *redis.Client) *DLQ {
	return &DLQ{client: client}
}

// Add moves a task to the dead letter queue
func (d *DLQ) Add(ctx context.Context, t *task.Task, reason string) error {
	// Update task state
	sm := task.NewStateMachine(t)
	if err := sm.MoveToDLQ(); err != nil {
		// Force state if transition is invalid
		t.State = task.StateDeadLetter
		t.UpdatedAt = time.Now().UTC()
	}

	// Store additional DLQ metadata
	dlqEntry := struct {
		Task      *task.Task `json:"task"`
		Reason    string     `json:"reason"`
		AddedAt   time.Time  `json:"added_at"`
		OrigError string     `json:"original_error"`
	}{
		Task:      t,
		Reason:    reason,
		AddedAt:   time.Now().UTC(),
		OrigError: t.Error,
	}

	data, err := json.Marshal(dlqEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ entry: %w", err)
	}

	// Add to DLQ stream
	_, err = d.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqStreamName,
		Values: map[string]interface{}{
			"task_id": t.ID,
			"type":    t.Type,
			"data":    string(data),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add to DLQ stream: %w", err)
	}

	// Add to set for quick lookups
	d.client.SAdd(ctx, dlqSetName, t.ID)

	return nil
}

// DLQEntry represents an entry in the dead letter queue
type DLQEntry struct {
	Task      *task.Task `json:"task"`
	Reason    string     `json:"reason"`
	AddedAt   time.Time  `json:"added_at"`
	OrigError string     `json:"original_error"`
	MessageID string     `json:"message_id"`
}

// List returns tasks in the dead letter queue
func (d *DLQ) List(ctx context.Context, count int64, offset string) ([]DLQEntry, error) {
	if offset == "" {
		offset = "-"
	}

	messages, err := d.client.XRange(ctx, dlqStreamName, offset, "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read DLQ: %w", err)
	}

	entries := make([]DLQEntry, 0, len(messages))
	for i, msg := range messages {
		if int64(i) >= count && count > 0 {
			break
		}

		data, ok := msg.Values["data"].(string)
		if !ok {
			continue
		}

		var entry DLQEntry
		if err := json.Unmarshal([]byte(data), &entry); err != nil {
			continue
		}
		entry.MessageID = msg.ID

		entries = append(entries, entry)
	}

	return entries, nil
}

// Remove removes a task from the dead letter queue
func (d *DLQ) Remove(ctx context.Context, taskID string, messageID string) error {
	// Remove from stream
	if messageID != "" {
		if err := d.client.XDel(ctx, dlqStreamName, messageID).Err(); err != nil {
			return fmt.Errorf("failed to remove from DLQ stream: %w", err)
		}
	}

	// Remove from set
	d.client.SRem(ctx, dlqSetName, taskID)

	return nil
}

// Retry moves a task from DLQ back to the main queue
func (d *DLQ) Retry(ctx context.Context, q *RedisQueue, taskID string, messageID string) error {
	// Find the DLQ entry
	entries, err := d.List(ctx, 0, "")
	if err != nil {
		return err
	}

	var targetEntry *DLQEntry
	for _, entry := range entries {
		if entry.Task.ID == taskID {
			targetEntry = &entry
			break
		}
	}

	if targetEntry == nil {
		return task.ErrTaskNotFound
	}

	// Reset task for reprocessing
	sm := task.NewStateMachine(targetEntry.Task)
	if err := sm.Requeue(); err != nil {
		return fmt.Errorf("failed to requeue task: %w", err)
	}

	// Re-enqueue to main queue
	if err := q.Enqueue(ctx, targetEntry.Task); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	// Remove from DLQ
	return d.Remove(ctx, taskID, targetEntry.MessageID)
}

// RetryAll moves all tasks from DLQ back to the main queue
func (d *DLQ) RetryAll(ctx context.Context, q *RedisQueue) (int, error) {
	entries, err := d.List(ctx, 0, "")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if err := d.Retry(ctx, q, entry.Task.ID, entry.MessageID); err != nil {
			continue // Skip failed retries
		}
		count++
	}

	return count, nil
}

// Size returns the number of tasks in the DLQ
func (d *DLQ) Size(ctx context.Context) (int64, error) {
	return d.client.SCard(ctx, dlqSetName).Result()
}

// Contains checks if a task is in the DLQ
func (d *DLQ) Contains(ctx context.Context, taskID string) (bool, error) {
	return d.client.SIsMember(ctx, dlqSetName, taskID).Result()
}

// Clear removes all tasks from the DLQ
func (d *DLQ) Clear(ctx context.Context) error {
	// Delete stream and set
	if err := d.client.Del(ctx, dlqStreamName).Err(); err != nil {
		return fmt.Errorf("failed to delete DLQ stream: %w", err)
	}

	return d.client.Del(ctx, dlqSetName).Err()
}
