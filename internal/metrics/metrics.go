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

// RecordTaskRetry records a task retry
func RecordTaskRetry(taskType string) {
	TaskRetries.WithLabelValues(taskType).Inc()
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
