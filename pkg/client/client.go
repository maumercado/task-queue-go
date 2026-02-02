package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// TaskQueueClient wraps the generated client with additional functionality.
type TaskQueueClient struct {
	*ClientWithResponses
	baseURL string
	opts    *options
	ws      *WebSocketClient
}

// New creates a new TaskQueueClient.
func New(baseURL string, opts ...Option) (*TaskQueueClient, error) {
	// Ensure URL doesn't have trailing slash for consistency
	baseURL = strings.TrimSuffix(baseURL, "/")

	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	// Create the generated client with our options
	genClient, err := NewClientWithResponses(
		baseURL,
		WithHTTPClient(o.httpClient),
		WithRequestEditorFn(o.applyHeaders()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &TaskQueueClient{
		ClientWithResponses: genClient,
		baseURL:             baseURL,
		opts:                o,
	}, nil
}

// ConnectWebSocket establishes a WebSocket connection for real-time events.
func (c *TaskQueueClient) ConnectWebSocket(ctx context.Context) error {
	if c.ws != nil && c.ws.IsConnected() {
		return nil
	}
	c.ws = newWebSocketClient(c.baseURL, c.opts.apiKey)
	return c.ws.Connect(ctx)
}

// Events returns a channel that receives WebSocket events.
// Must call ConnectWebSocket first.
func (c *TaskQueueClient) Events() <-chan *Event {
	if c.ws == nil {
		ch := make(chan *Event)
		close(ch)
		return ch
	}
	return c.ws.Events()
}

// CloseWebSocket closes the WebSocket connection.
func (c *TaskQueueClient) CloseWebSocket() error {
	if c.ws == nil {
		return nil
	}
	return c.ws.Close()
}

// SubscribeEvents subscribes to specific event types.
func (c *TaskQueueClient) SubscribeEvents(eventTypes ...EventType) error {
	if c.ws == nil {
		return fmt.Errorf("websocket not connected")
	}
	return c.ws.Subscribe(eventTypes...)
}

// Helper methods that provide a cleaner interface

// SubmitTask creates a new task and returns the created task.
func (c *TaskQueueClient) SubmitTask(ctx context.Context, req CreateTaskRequest) (*TaskResponse, error) {
	resp, err := c.CreateTaskWithResponse(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.JSON201 != nil {
		return resp.JSON201, nil
	}

	if resp.JSON400 != nil {
		return nil, fmt.Errorf("bad request: %s", safeString(resp.JSON400.Message))
	}
	if resp.JSON429 != nil {
		return nil, fmt.Errorf("rate limited: %s", safeString(resp.JSON429.Message))
	}
	if resp.JSON503 != nil {
		return nil, fmt.Errorf("service unavailable: %s", safeString(resp.JSON503.Message))
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// GetTaskByID retrieves a task by its ID.
func (c *TaskQueueClient) GetTaskByID(ctx context.Context, taskID string) (*TaskResponse, error) {
	id, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	resp, err := c.GetTaskWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	if resp.JSON404 != nil {
		return nil, fmt.Errorf("task not found: %s", safeString(resp.JSON404.Message))
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// CancelTaskByID cancels a task by its ID.
func (c *TaskQueueClient) CancelTaskByID(ctx context.Context, taskID string) (*TaskResponse, error) {
	id, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	resp, err := c.CancelTaskWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	if resp.JSON404 != nil {
		return nil, fmt.Errorf("task not found: %s", safeString(resp.JSON404.Message))
	}
	if resp.JSON409 != nil {
		return nil, fmt.Errorf("cannot cancel task: %s", safeString(resp.JSON409.Message))
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// GetQueueStatistics returns the current queue depths.
func (c *TaskQueueClient) GetQueueStatistics(ctx context.Context) (*QueueStats, error) {
	resp, err := c.ListTasksWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// CheckHealth checks the health of the API server.
func (c *TaskQueueClient) CheckHealth(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.HealthCheckWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	if resp.JSON503 != nil {
		return resp.JSON503, nil
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// ListAllWorkers returns all active workers.
func (c *TaskQueueClient) ListAllWorkers(ctx context.Context) (*WorkerListResponse, error) {
	resp, err := c.ListWorkersWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// PauseWorkerByID pauses a worker.
func (c *TaskQueueClient) PauseWorkerByID(ctx context.Context, workerID string) error {
	resp, err := c.PauseWorkerWithResponse(ctx, workerID)
	if err != nil {
		return err
	}

	if resp.JSON200 != nil {
		return nil
	}
	if resp.JSON404 != nil {
		return fmt.Errorf("worker not found: %s", safeString(resp.JSON404.Message))
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// ResumeWorkerByID resumes a paused worker.
func (c *TaskQueueClient) ResumeWorkerByID(ctx context.Context, workerID string) error {
	resp, err := c.ResumeWorkerWithResponse(ctx, workerID)
	if err != nil {
		return err
	}

	if resp.JSON200 != nil {
		return nil
	}
	if resp.JSON404 != nil {
		return fmt.Errorf("worker not found: %s", safeString(resp.JSON404.Message))
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// GetDLQEntries returns all entries in the dead letter queue.
func (c *TaskQueueClient) GetDLQEntries(ctx context.Context) (*DLQListResponse, error) {
	resp, err := c.ListDLQWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// RetryDLQTask retries a specific task from the DLQ.
func (c *TaskQueueClient) RetryDLQTask(ctx context.Context, taskID string) error {
	resp, err := c.RetryDLQWithResponse(ctx, RetryDLQRequest{
		TaskId: &taskID,
	})
	if err != nil {
		return err
	}

	if resp.StatusCode() == 200 {
		return nil
	}
	if resp.JSON404 != nil {
		return fmt.Errorf("task not found in DLQ: %s", safeString(resp.JSON404.Message))
	}
	if resp.JSON400 != nil {
		return fmt.Errorf("bad request: %s", safeString(resp.JSON400.Message))
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// RetryAllDLQTasks retries all tasks in the DLQ.
func (c *TaskQueueClient) RetryAllDLQTasks(ctx context.Context) (int, error) {
	retryAll := true
	resp, err := c.RetryDLQWithResponse(ctx, RetryDLQRequest{
		RetryAll: &retryAll,
	})
	if err != nil {
		return 0, err
	}

	if resp.StatusCode() == 200 && resp.Body != nil {
		// The response might be RetryDLQAllResponse
		return 0, nil // Count not easily extractable from union type
	}
	if resp.JSON400 != nil {
		return 0, fmt.Errorf("bad request: %s", safeString(resp.JSON400.Message))
	}

	return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// ClearDLQAll clears all entries from the dead letter queue.
func (c *TaskQueueClient) ClearDLQAll(ctx context.Context) error {
	resp, err := c.ClearDLQWithResponse(ctx)
	if err != nil {
		return err
	}

	if resp.JSON200 != nil {
		return nil
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// safeString safely dereferences a string pointer.
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
