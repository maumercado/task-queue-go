package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
)

// ScheduleTaskFunc is a function type for scheduling tasks
type ScheduleTaskFunc func(ctx context.Context, t *task.Task, scheduledAt time.Time) error

// TaskHandler handles task-related HTTP requests
type TaskHandler struct {
	queue        *queue.RedisQueue
	scheduleTask ScheduleTaskFunc
	maxQueueSize int64
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(q *queue.RedisQueue, scheduleTask ScheduleTaskFunc, maxQueueSize int64) *TaskHandler {
	return &TaskHandler{
		queue:        q,
		scheduleTask: scheduleTask,
		maxQueueSize: maxQueueSize,
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

	// Only pending or scheduled tasks can be cancelled
	if t.State != task.StatePending && t.State != task.StateScheduled {
		h.respondError(w, http.StatusConflict, "task cannot be cancelled in current state")
		return
	}

	sm := task.NewStateMachine(t)
	if err := sm.Cancel(); err != nil {
		h.respondError(w, http.StatusConflict, "failed to cancel task")
		return
	}

	if err := h.queue.UpdateTask(r.Context(), t); err != nil {
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to update task")
		h.respondError(w, http.StatusInternalServerError, "failed to cancel task")
		return
	}

	logger.Info().Str("task_id", taskID).Msg("task cancelled")
	h.respondJSON(w, http.StatusOK, t.ToResponse())
}

// ListResponse represents the response for listing tasks
type ListResponse struct {
	Tasks      []*task.TaskResponse `json:"tasks"`
	TotalCount int                  `json:"total_count"`
}

// List handles GET /api/v1/tasks
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	// Get queue depths for now (full listing would require additional Redis data structures)
	depths, err := h.queue.GetQueueDepth(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("failed to get queue depths")
		h.respondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	// Calculate total count from depths
	var total int64
	for _, depth := range depths {
		total += depth
	}

	response := map[string]interface{}{
		"queue_depths": map[string]int64{
			"critical": depths[task.PriorityCritical],
			"high":     depths[task.PriorityHigh],
			"normal":   depths[task.PriorityNormal],
			"low":      depths[task.PriorityLow],
		},
		"total_pending": total,
	}

	h.respondJSON(w, http.StatusOK, response)
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
