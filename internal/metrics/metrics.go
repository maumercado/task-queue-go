package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Task metrics
	TasksSubmitted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_tasks_submitted_total",
			Help: "Total number of tasks submitted",
		},
		[]string{"type", "priority"},
	)

	TasksCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_tasks_completed_total",
			Help: "Total number of tasks completed",
		},
		[]string{"type", "status"},
	)

	TaskDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "taskqueue_task_duration_seconds",
			Help:    "Task execution duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
		},
		[]string{"type"},
	)

	TaskRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_task_retries_total",
			Help: "Total number of task retries",
		},
		[]string{"type"},
	)

	// Queue metrics
	QueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "taskqueue_queue_depth",
			Help: "Current number of tasks in queue",
		},
		[]string{"priority"},
	)

	QueueLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "taskqueue_queue_latency_seconds",
			Help:    "Time spent in queue before processing",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
		},
		[]string{"priority"},
	)

	// Worker metrics
	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "taskqueue_active_workers",
			Help: "Current number of active workers",
		},
	)

	WorkerBusyTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_worker_busy_seconds_total",
			Help: "Total time workers spent processing tasks",
		},
		[]string{"worker_id"},
	)

	WorkerIdleTime = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_worker_idle_seconds_total",
			Help: "Total time workers spent idle",
		},
		[]string{"worker_id"},
	)

	// DLQ metrics
	DLQSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "taskqueue_dlq_size",
			Help: "Current number of tasks in dead letter queue",
		},
	)

	DLQAdded = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "taskqueue_dlq_added_total",
			Help: "Total number of tasks added to dead letter queue",
		},
	)

	// HTTP metrics
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "taskqueue_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// Redis metrics
	RedisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "taskqueue_redis_operation_duration_seconds",
			Help:    "Redis operation duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12), // 0.1ms to ~200ms
		},
		[]string{"operation"},
	)

	RedisErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_redis_errors_total",
			Help: "Total number of Redis errors",
		},
		[]string{"operation"},
	)

	// Retry / scheduling metrics
	TasksRetrying = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_tasks_retrying_total",
			Help: "Total number of automatic task retries scheduled",
		},
		[]string{"type"},
	)

	TasksScheduled = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "taskqueue_tasks_scheduled_total",
			Help: "Total number of tasks submitted with a future scheduled_at",
		},
	)

	RetryDelay = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "taskqueue_retry_delay_seconds",
			Help:    "Backoff delay applied before retrying a failed task",
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 10), // 0.5s to ~256s
		},
		[]string{"type"},
	)

	ScheduledTasksGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "taskqueue_scheduled_tasks",
			Help: "Current number of tasks waiting in the scheduled sorted set",
		},
	)

	OrphanClaims = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "taskqueue_orphan_claims_total",
			Help: "Total number of orphaned task messages claimed via XCLAIM",
		},
	)

	QueueBacklog = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "taskqueue_queue_backlog",
			Help: "Total messages in each priority stream (backlog including unacked)",
		},
		[]string{"priority"},
	)

	QueuePendingUnacked = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "taskqueue_queue_pending_unacked",
			Help: "Messages delivered to consumers but not yet ACKed (PEL)",
		},
		[]string{"priority"},
	)

	// WebSocket metrics
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "taskqueue_websocket_connections",
			Help: "Current number of WebSocket connections",
		},
	)

	WebSocketMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "taskqueue_websocket_messages_total",
			Help: "Total number of WebSocket messages sent",
		},
		[]string{"type"},
	)
)

// RecordTaskSubmission records a task submission
func RecordTaskSubmission(taskType, priority string) {
	TasksSubmitted.WithLabelValues(taskType, priority).Inc()
}

// RecordTaskCompletion records a task completion
func RecordTaskCompletion(taskType, status string, duration float64) {
	TasksCompleted.WithLabelValues(taskType, status).Inc()
	TaskDuration.WithLabelValues(taskType).Observe(duration)
}

// RecordTaskRetry records a task retry (legacy counter kept for compatibility)
func RecordTaskRetry(taskType string) {
	TaskRetries.WithLabelValues(taskType).Inc()
}

// RecordTaskRetrying records an automatic delayed retry being scheduled.
func RecordTaskRetrying(taskType string, delaySeconds float64) {
	TasksRetrying.WithLabelValues(taskType).Inc()
	RetryDelay.WithLabelValues(taskType).Observe(delaySeconds)
}

// RecordScheduledTask records a task submitted with a future scheduled_at.
func RecordScheduledTask() {
	TasksScheduled.Inc()
}

// SetScheduledTasksGauge sets the current scheduled sorted-set size.
func SetScheduledTasksGauge(count float64) {
	ScheduledTasksGauge.Set(count)
}

// RecordOrphanClaim records one orphaned message being claimed.
func RecordOrphanClaim() {
	OrphanClaims.Inc()
}

// UpdateQueueBacklog sets the backlog gauge for a priority stream.
func UpdateQueueBacklog(priority string, count float64) {
	QueueBacklog.WithLabelValues(priority).Set(count)
}

// UpdateQueuePendingUnacked sets the PEL gauge for a priority stream.
func UpdateQueuePendingUnacked(priority string, count float64) {
	QueuePendingUnacked.WithLabelValues(priority).Set(count)
}

// UpdateQueueDepth updates the queue depth gauge
func UpdateQueueDepth(priority string, depth float64) {
	QueueDepth.WithLabelValues(priority).Set(depth)
}

// RecordQueueLatency records the time a task spent in queue
func RecordQueueLatency(priority string, latency float64) {
	QueueLatency.WithLabelValues(priority).Observe(latency)
}

// SetActiveWorkers sets the active workers gauge
func SetActiveWorkers(count float64) {
	ActiveWorkers.Set(count)
}

// RecordWorkerBusyTime records time spent processing
func RecordWorkerBusyTime(workerID string, duration float64) {
	WorkerBusyTime.WithLabelValues(workerID).Add(duration)
}

// SetDLQSize sets the DLQ size gauge
func SetDLQSize(size float64) {
	DLQSize.Set(size)
}

// IncrementDLQAdded increments the DLQ added counter
func IncrementDLQAdded() {
	DLQAdded.Inc()
}

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(method, path, status string, duration float64) {
	HTTPRequestDuration.WithLabelValues(method, path, status).Observe(duration)
	HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordRedisOperation records a Redis operation
func RecordRedisOperation(operation string, duration float64) {
	RedisOperationDuration.WithLabelValues(operation).Observe(duration)
}

// RecordRedisError records a Redis error
func RecordRedisError(operation string) {
	RedisErrors.WithLabelValues(operation).Inc()
}

// SetWebSocketConnections sets the WebSocket connections gauge
func SetWebSocketConnections(count float64) {
	WebSocketConnections.Set(count)
}

// RecordWebSocketMessage records a WebSocket message
func RecordWebSocketMessage(msgType string) {
	WebSocketMessages.WithLabelValues(msgType).Inc()
}
