package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/metrics"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
)

// ScheduleTaskFunc is a function type for scheduling tasks
type ScheduleTaskFunc func(ctx context.Context, t *task.Task, scheduledAt time.Time) error

// TaskHandler handles task-related HTTP requests
type TaskHandler struct {
	queue             *queue.RedisQueue
	dlq               *queue.DLQ
	scheduleTask      ScheduleTaskFunc
	maxQueueSize      int64
	defaultMaxRetries int
	publisher         *events.RedisPubSub
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(q *queue.RedisQueue, dlq *queue.DLQ, scheduleTask ScheduleTaskFunc, maxQueueSize int64, defaultMaxRetries int, publisher *events.RedisPubSub) *TaskHandler {
	return &TaskHandler{
		queue:             q,
		dlq:               dlq,
		scheduleTask:      scheduleTask,
		maxQueueSize:      maxQueueSize,
		defaultMaxRetries: defaultMaxRetries,
		publisher:         publisher,
	}
}

// Create handles POST /api/v1/tasks
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req task.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Type == "" {
		h.respondError(w, http.StatusBadRequest, "task type is required")
		return
	}

	// Check queue capacity (backpressure)
	if h.maxQueueSize > 0 {
		depths, err := h.queue.GetQueueDepth(r.Context())
		if err == nil {
			var total int64
			for _, depth := range depths {
				total += depth
			}
			if total >= h.maxQueueSize {
				h.respondError(w, http.StatusServiceUnavailable, "queue at capacity")
				return
			}
		}
	}

	// Create task
	t := task.FromRequest(&req)

	// Apply config default for max_retries when client omits it (request value <= 0).
	// A client sending explicit 0 also gets the default (treat 0 as "use default").
	if req.MaxRetries <= 0 && h.defaultMaxRetries > 0 {
		t.MaxRetries = h.defaultMaxRetries
	}

	// Check if this is a scheduled task
	if req.ScheduledAt != nil && req.ScheduledAt.After(time.Now().UTC()) {
		// Set state to scheduled
		t.State = task.StateScheduled

		// Schedule the task for later
		if err := h.scheduleTask(r.Context(), t, *req.ScheduledAt); err != nil {
			logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to schedule task")
			h.respondError(w, http.StatusInternalServerError, "failed to schedule task")
			return
		}

		logger.Info().
			Str("task_id", t.ID).
			Str("type", t.Type).
			Str("priority", t.Priority.String()).
			Time("scheduled_at", *req.ScheduledAt).
			Msg("task scheduled")

		metrics.RecordTaskSubmission(t.Type, t.Priority.String())
		metrics.RecordScheduledTask()
		h.publishTaskEvent(r.Context(), events.EventTaskSubmitted, t, nil)

		h.respondJSON(w, http.StatusCreated, t.ToResponse())
		return
	}

	// Enqueue task immediately
	if err := h.queue.Enqueue(r.Context(), t); err != nil {
		logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to enqueue task")
		h.respondError(w, http.StatusInternalServerError, "failed to enqueue task")
		return
	}

	logger.Info().
		Str("task_id", t.ID).
		Str("type", t.Type).
		Str("priority", t.Priority.String()).
		Msg("task created")

	metrics.RecordTaskSubmission(t.Type, t.Priority.String())
	h.publishTaskEvent(r.Context(), events.EventTaskSubmitted, t, nil)

	h.respondJSON(w, http.StatusCreated, t.ToResponse())
}

// Get handles GET /api/v1/tasks/{taskID}
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		h.respondError(w, http.StatusBadRequest, "task ID is required")
		return
	}

	t, err := h.queue.GetTask(r.Context(), taskID)
	if err != nil {
		if err == task.ErrTaskNotFound {
			h.respondError(w, http.StatusNotFound, "task not found")
			return
		}
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to get task")
		h.respondError(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	h.respondJSON(w, http.StatusOK, t.ToResponse())
}

// Cancel handles DELETE /api/v1/tasks/{taskID}
func (h *TaskHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		h.respondError(w, http.StatusBadRequest, "task ID is required")
		return
	}

	t, err := h.queue.GetTask(r.Context(), taskID)
	if err != nil {
		if err == task.ErrTaskNotFound {
			h.respondError(w, http.StatusNotFound, "task not found")
			return
		}
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to get task")
		h.respondError(w, http.StatusInternalServerError, "failed to get task")
		return
	}

	// pending, scheduled, and retrying tasks can be canceled.
	// retrying = waiting in backoff delay before next attempt.
	cancellable := t.State == task.StatePending ||
		t.State == task.StateScheduled ||
		t.State == task.StateRetrying

	if !cancellable {
		h.respondError(w, http.StatusConflict, "task cannot be canceled in current state")
		return
	}

	sm := task.NewStateMachine(t)
	if err := sm.Cancel(); err != nil {
		h.respondError(w, http.StatusConflict, "failed to cancel task")
		return
	}

	// Remove from scheduled sorted set so scheduler does not reactivate.
	// Applies to both scheduled (future first run) and retrying (backoff delay).
	if t.State == task.StateScheduled || t.State == task.StateRetrying {
		if err := h.queue.RemoveScheduledTask(r.Context(), taskID); err != nil {
			logger.Warn().Err(err).Str("task_id", taskID).Msg("failed to remove task from scheduled set")
			// Non-fatal: scheduler will skip canceled tasks safely.
		}
	}

	if err := h.queue.UpdateTask(r.Context(), t); err != nil {
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to update task")
		h.respondError(w, http.StatusInternalServerError, "failed to cancel task")
		return
	}

	logger.Info().Str("task_id", taskID).Msg("task canceled")
	h.respondJSON(w, http.StatusOK, t.ToResponse())
}

// ListResponse represents the response for listing tasks
type ListResponse struct {
	Tasks      []*task.TaskResponse `json:"tasks"`
	TotalCount int                  `json:"total_count"`
}

// List handles GET /api/v1/tasks — returns rich queue inspection data.
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	stats, err := h.queue.GetQueueStats(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("failed to get queue stats")
		h.respondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	// Attach DLQ size
	if h.dlq != nil {
		if dlqSize, err := h.dlq.Size(r.Context()); err == nil {
			stats.DLQSize = dlqSize
		}
	}

	// Update Prometheus gauges
	for priority, ps := range stats.Queues {
		metrics.UpdateQueueBacklog(priority, float64(ps.Queued))
		metrics.UpdateQueuePendingUnacked(priority, float64(ps.PendingUnacked))
	}
	metrics.SetScheduledTasksGauge(float64(stats.ScheduledCount))
	metrics.SetDLQSize(float64(stats.DLQSize))

	h.respondJSON(w, http.StatusOK, stats)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *TaskHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (h *TaskHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

// publishTaskEvent fires a task lifecycle event; non-fatal on failure.
func (h *TaskHandler) publishTaskEvent(ctx context.Context, eventType events.EventType, t *task.Task, extra map[string]interface{}) {
	if h.publisher == nil {
		return
	}
	data := map[string]interface{}{
		"task_id":  t.ID,
		"type":     t.Type,
		"priority": t.Priority.String(),
		"state":    t.State.String(),
		"attempts": t.Attempts,
	}
	if t.ScheduledAt != nil {
		data["scheduled_at"] = t.ScheduledAt
	}
	for k, v := range extra {
		data[k] = v
	}
	if err := h.publisher.Publish(ctx, events.NewEvent(eventType, data)); err != nil {
		logger.Warn().Err(err).Str("event", string(eventType)).Str("task_id", t.ID).Msg("failed to publish task event")
	}
}
