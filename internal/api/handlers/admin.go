package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
	"github.com/maumercado/task-queue-go/internal/worker"
)

// AdminHandler handles admin API requests
type AdminHandler struct {
	queue *queue.RedisQueue
	dlq   *queue.DLQ
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(q *queue.RedisQueue, dlq *queue.DLQ) *AdminHandler {
	return &AdminHandler{
		queue: q,
		dlq:   dlq,
	}
}

// ListWorkers handles GET /admin/workers
func (h *AdminHandler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := worker.GetActiveWorkers(r.Context(), h.queue.Client())
	if err != nil {
		logger.Error().Err(err).Msg("failed to get active workers")
		h.respondError(w, http.StatusInternalServerError, "failed to get workers")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"workers": workers,
		"count":   len(workers),
	})
}

// GetWorker handles GET /admin/workers/{workerID}
func (h *AdminHandler) GetWorker(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "workerID")
	if workerID == "" {
		h.respondError(w, http.StatusBadRequest, "worker ID is required")
		return
	}

	alive, err := worker.IsWorkerAlive(r.Context(), h.queue.Client(), workerID)
	if err != nil {
		logger.Error().Err(err).Str("worker_id", workerID).Msg("failed to check worker status")
		h.respondError(w, http.StatusInternalServerError, "failed to get worker")
		return
	}

	if !alive {
		h.respondError(w, http.StatusNotFound, "worker not found or not active")
		return
	}

	// Get detailed worker info
	workers, err := worker.GetActiveWorkers(r.Context(), h.queue.Client())
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get worker details")
		return
	}

	for _, wk := range workers {
		if wk.ID == workerID {
			h.respondJSON(w, http.StatusOK, wk)
			return
		}
	}

	h.respondError(w, http.StatusNotFound, "worker not found")
}

// GetQueues handles GET /admin/queues
func (h *AdminHandler) GetQueues(w http.ResponseWriter, r *http.Request) {
	depths, err := h.queue.GetQueueDepth(r.Context())
	if err != nil {
		logger.Error().Err(err).Msg("failed to get queue depths")
		h.respondError(w, http.StatusInternalServerError, "failed to get queue statistics")
		return
	}

	var total int64
	queueStats := make(map[string]interface{})
	for priority, depth := range depths {
		queueStats[priority.String()] = map[string]interface{}{
			"depth":    depth,
			"priority": int(priority),
		}
		total += depth
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"queues":      queueStats,
		"total_depth": total,
	})
}

// ListDLQ handles GET /admin/dlq
func (h *AdminHandler) ListDLQ(w http.ResponseWriter, r *http.Request) {
	entries, err := h.dlq.List(r.Context(), 100, "")
	if err != nil {
		logger.Error().Err(err).Msg("failed to list DLQ")
		h.respondError(w, http.StatusInternalServerError, "failed to list DLQ")
		return
	}

	size, _ := h.dlq.Size(r.Context())

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"size":    size,
	})
}

// RetryDLQRequest represents a request to retry DLQ tasks
type RetryDLQRequest struct {
	TaskID    string `json:"task_id,omitempty"`
	RetryAll  bool   `json:"retry_all,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// RetryDLQ handles POST /admin/dlq/retry
func (h *AdminHandler) RetryDLQ(w http.ResponseWriter, r *http.Request) {
	var req RetryDLQRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RetryAll {
		count, err := h.dlq.RetryAll(r.Context(), h.queue)
		if err != nil {
			logger.Error().Err(err).Msg("failed to retry all DLQ tasks")
			h.respondError(w, http.StatusInternalServerError, "failed to retry DLQ tasks")
			return
		}

		h.respondJSON(w, http.StatusOK, map[string]interface{}{
			"message":       "tasks re-queued",
			"retried_count": count,
		})
		return
	}

	if req.TaskID == "" {
		h.respondError(w, http.StatusBadRequest, "task_id or retry_all is required")
		return
	}

	if err := h.dlq.Retry(r.Context(), h.queue, req.TaskID, req.MessageID); err != nil {
		if err == task.ErrTaskNotFound {
			h.respondError(w, http.StatusNotFound, "task not found in DLQ")
			return
		}
		logger.Error().Err(err).Str("task_id", req.TaskID).Msg("failed to retry DLQ task")
		h.respondError(w, http.StatusInternalServerError, "failed to retry task")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "task re-queued",
		"task_id": req.TaskID,
	})
}

// ClearDLQ handles DELETE /admin/dlq
func (h *AdminHandler) ClearDLQ(w http.ResponseWriter, r *http.Request) {
	if err := h.dlq.Clear(r.Context()); err != nil {
		logger.Error().Err(err).Msg("failed to clear DLQ")
		h.respondError(w, http.StatusInternalServerError, "failed to clear DLQ")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "DLQ cleared",
	})
}

// HealthCheck handles GET /admin/health
func (h *AdminHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check Redis connection
	if err := h.queue.Client().Ping(r.Context()).Err(); err != nil {
		h.respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "unhealthy",
			"redis":  "disconnected",
			"error":  err.Error(),
		})
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"redis":  "connected",
	})
}

// RetryTask handles POST /admin/tasks/{taskID}/retry
func (h *AdminHandler) RetryTask(w http.ResponseWriter, r *http.Request) {
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

	// Only failed or dead_letter tasks can be retried
	if t.State != task.StateFailed && t.State != task.StateDeadLetter {
		h.respondError(w, http.StatusConflict, "only failed or dead_letter tasks can be retried")
		return
	}

	// Reset task for retry
	sm := task.NewStateMachine(t)
	if err := sm.Requeue(); err != nil {
		h.respondError(w, http.StatusConflict, "failed to requeue task")
		return
	}

	// Update task in storage
	if err := h.queue.UpdateTask(r.Context(), t); err != nil {
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to update task")
		h.respondError(w, http.StatusInternalServerError, "failed to retry task")
		return
	}

	// Re-enqueue task
	if err := h.queue.Enqueue(r.Context(), t); err != nil {
		logger.Error().Err(err).Str("task_id", taskID).Msg("failed to enqueue task")
		h.respondError(w, http.StatusInternalServerError, "failed to retry task")
		return
	}

	logger.Info().Str("task_id", taskID).Msg("task retried manually")
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "task re-queued",
		"task_id": taskID,
	})
}

// PauseWorker handles POST /admin/workers/{workerID}/pause
func (h *AdminHandler) PauseWorker(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "workerID")
	if workerID == "" {
		h.respondError(w, http.StatusBadRequest, "worker ID is required")
		return
	}

	// Check if worker exists
	alive, err := worker.IsWorkerAlive(r.Context(), h.queue.Client(), workerID)
	if err != nil {
		logger.Error().Err(err).Str("worker_id", workerID).Msg("failed to check worker status")
		h.respondError(w, http.StatusInternalServerError, "failed to check worker status")
		return
	}

	if !alive {
		h.respondError(w, http.StatusNotFound, "worker not found or not active")
		return
	}

	// Set pause flag in Redis
	pauseKey := "worker:" + workerID + ":paused"
	if err := h.queue.Client().Set(r.Context(), pauseKey, "1", 0).Err(); err != nil {
		logger.Error().Err(err).Str("worker_id", workerID).Msg("failed to pause worker")
		h.respondError(w, http.StatusInternalServerError, "failed to pause worker")
		return
	}

	logger.Info().Str("worker_id", workerID).Msg("worker paused")
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "worker paused",
		"worker_id": workerID,
	})
}

// ResumeWorker handles POST /admin/workers/{workerID}/resume
func (h *AdminHandler) ResumeWorker(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "workerID")
	if workerID == "" {
		h.respondError(w, http.StatusBadRequest, "worker ID is required")
		return
	}

	// Check if worker exists
	alive, err := worker.IsWorkerAlive(r.Context(), h.queue.Client(), workerID)
	if err != nil {
		logger.Error().Err(err).Str("worker_id", workerID).Msg("failed to check worker status")
		h.respondError(w, http.StatusInternalServerError, "failed to check worker status")
		return
	}

	if !alive {
		h.respondError(w, http.StatusNotFound, "worker not found or not active")
		return
	}

	// Remove pause flag from Redis
	pauseKey := "worker:" + workerID + ":paused"
	if err := h.queue.Client().Del(r.Context(), pauseKey).Err(); err != nil {
		logger.Error().Err(err).Str("worker_id", workerID).Msg("failed to resume worker")
		h.respondError(w, http.StatusInternalServerError, "failed to resume worker")
		return
	}

	logger.Info().Str("worker_id", workerID).Msg("worker resumed")
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "worker resumed",
		"worker_id": workerID,
	})
}

// PurgeQueue handles DELETE /admin/queues/{priority}
func (h *AdminHandler) PurgeQueue(w http.ResponseWriter, r *http.Request) {
	priority := chi.URLParam(r, "priority")
	if priority == "" {
		h.respondError(w, http.StatusBadRequest, "priority is required")
		return
	}

	// Validate priority
	p := task.ParsePriority(priority)
	if priority != p.String() {
		h.respondError(w, http.StatusBadRequest, "invalid priority: must be critical, high, normal, or low")
		return
	}

	// Get stream name
	streamName := "tasks:" + priority

	// Delete the stream (removes all messages)
	if err := h.queue.Client().Del(r.Context(), streamName).Err(); err != nil {
		logger.Error().Err(err).Str("priority", priority).Msg("failed to purge queue")
		h.respondError(w, http.StatusInternalServerError, "failed to purge queue")
		return
	}

	// Recreate the stream with consumer group
	err := h.queue.Client().XGroupCreateMkStream(r.Context(), streamName, "workers", "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		logger.Error().Err(err).Str("priority", priority).Msg("failed to recreate queue")
		// Don't return error - stream was still purged
	}

	logger.Info().Str("priority", priority).Msg("queue purged")
	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "queue purged",
		"priority": priority,
	})
}

func (h *AdminHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (h *AdminHandler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]interface{}{
		"error":   http.StatusText(status),
		"message": message,
	})
}
