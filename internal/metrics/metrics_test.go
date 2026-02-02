package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are registered without panic
	// promauto already registers them, so we just verify they exist

	// Task metrics
	assert.NotNil(t, TasksSubmitted)
	assert.NotNil(t, TasksCompleted)
	assert.NotNil(t, TaskDuration)
	assert.NotNil(t, TaskRetries)

	// Queue metrics
	assert.NotNil(t, QueueDepth)
	assert.NotNil(t, QueueLatency)

	// Worker metrics
	assert.NotNil(t, ActiveWorkers)
	assert.NotNil(t, WorkerBusyTime)
	assert.NotNil(t, WorkerIdleTime)

	// DLQ metrics
	assert.NotNil(t, DLQSize)
	assert.NotNil(t, DLQAdded)

	// HTTP metrics
	assert.NotNil(t, HTTPRequestDuration)
	assert.NotNil(t, HTTPRequestsTotal)

	// Redis metrics
	assert.NotNil(t, RedisOperationDuration)
	assert.NotNil(t, RedisErrors)

	// WebSocket metrics
	assert.NotNil(t, WebSocketConnections)
	assert.NotNil(t, WebSocketMessages)
}

func TestRecordTaskSubmission(t *testing.T) {
	// Reset for test
	TasksSubmitted.Reset()

	RecordTaskSubmission("email", "high")
	RecordTaskSubmission("email", "high")
	RecordTaskSubmission("compute", "normal")

	// Verify counter incremented (we can't easily get the value without a registry scrape)
	// Just ensure no panic
}

func TestRecordTaskCompletion(t *testing.T) {
	TasksCompleted.Reset()
	TaskDuration.Reset()

	RecordTaskCompletion("email", "success", 1.5)
	RecordTaskCompletion("email", "failed", 0.5)

	// Just ensure no panic
}

func TestRecordTaskRetry(t *testing.T) {
	TaskRetries.Reset()

	RecordTaskRetry("email")
	RecordTaskRetry("email")

	// Just ensure no panic
}

func TestUpdateQueueDepth(t *testing.T) {
	QueueDepth.Reset()

	UpdateQueueDepth("high", 100)
	UpdateQueueDepth("normal", 500)
	UpdateQueueDepth("low", 50)

	// Just ensure no panic
}

func TestRecordQueueLatency(t *testing.T) {
	QueueLatency.Reset()

	RecordQueueLatency("high", 0.001)
	RecordQueueLatency("normal", 0.5)

	// Just ensure no panic
}

func TestSetActiveWorkers(t *testing.T) {
	SetActiveWorkers(5)
	SetActiveWorkers(10)
	SetActiveWorkers(0)

	// Just ensure no panic
}

func TestRecordWorkerBusyTime(t *testing.T) {
	WorkerBusyTime.Reset()

	RecordWorkerBusyTime("worker-1", 10.5)
	RecordWorkerBusyTime("worker-2", 5.0)

	// Just ensure no panic
}

func TestSetDLQSize(t *testing.T) {
	SetDLQSize(0)
	SetDLQSize(10)
	SetDLQSize(100)

	// Just ensure no panic
}

func TestIncrementDLQAdded(t *testing.T) {
	DLQAdded = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_dlq_added_total",
		Help: "Test counter",
	})

	IncrementDLQAdded()
	IncrementDLQAdded()

	// Just ensure no panic
}

func TestRecordHTTPRequest(t *testing.T) {
	HTTPRequestDuration.Reset()
	HTTPRequestsTotal.Reset()

	RecordHTTPRequest("GET", "/api/v1/tasks", "200", 0.05)
	RecordHTTPRequest("POST", "/api/v1/tasks", "201", 0.1)
	RecordHTTPRequest("GET", "/api/v1/tasks/123", "404", 0.01)

	// Just ensure no panic
}

func TestRecordRedisOperation(t *testing.T) {
	RedisOperationDuration.Reset()

	RecordRedisOperation("XADD", 0.001)
	RecordRedisOperation("XREAD", 0.005)
	RecordRedisOperation("GET", 0.0001)

	// Just ensure no panic
}

func TestRecordRedisError(t *testing.T) {
	RedisErrors.Reset()

	RecordRedisError("XADD")
	RecordRedisError("GET")

	// Just ensure no panic
}

func TestSetWebSocketConnections(t *testing.T) {
	SetWebSocketConnections(0)
	SetWebSocketConnections(10)
	SetWebSocketConnections(5)

	// Just ensure no panic
}

func TestRecordWebSocketMessage(t *testing.T) {
	WebSocketMessages.Reset()

	RecordWebSocketMessage("task.submitted")
	RecordWebSocketMessage("task.completed")
	RecordWebSocketMessage("worker.joined")

	// Just ensure no panic
}
