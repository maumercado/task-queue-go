package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_Constants(t *testing.T) {
	// Verify all event types are defined correctly
	assert.Equal(t, EventType("task.submitted"), EventTaskSubmitted)
	assert.Equal(t, EventType("task.started"), EventTaskStarted)
	assert.Equal(t, EventType("task.completed"), EventTaskCompleted)
	assert.Equal(t, EventType("task.failed"), EventTaskFailed)
	assert.Equal(t, EventType("task.retrying"), EventTaskRetrying)
	assert.Equal(t, EventType("worker.joined"), EventWorkerJoined)
	assert.Equal(t, EventType("worker.left"), EventWorkerLeft)
	assert.Equal(t, EventType("worker.paused"), EventWorkerPaused)
	assert.Equal(t, EventType("worker.resumed"), EventWorkerResumed)
	assert.Equal(t, EventType("queue.depth"), EventQueueDepth)
	assert.Equal(t, EventType("system.metrics"), EventSystemMetrics)
}

func TestNewEvent(t *testing.T) {
	data := map[string]interface{}{
		"task_id": "task-123",
		"type":    "email",
	}

	event := NewEvent(EventTaskSubmitted, data)

	assert.Equal(t, EventTaskSubmitted, event.Type)
	assert.Equal(t, data, event.Data)
	assert.False(t, event.Timestamp.IsZero())
	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
}

func TestEvent_ToJSON(t *testing.T) {
	event := &Event{
		Type:      EventTaskCompleted,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Data: map[string]interface{}{
			"task_id": "task-456",
			"result":  "success",
		},
	}

	data, err := event.ToJSON()
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "task.completed", parsed["type"])
	assert.NotEmpty(t, parsed["timestamp"])
	assert.NotNil(t, parsed["data"])
}

func TestFromJSON(t *testing.T) {
	jsonData := `{
		"type": "task.failed",
		"timestamp": "2024-01-15T10:30:00Z",
		"data": {"task_id": "task-789", "error": "timeout"}
	}`

	event, err := FromJSON([]byte(jsonData))
	require.NoError(t, err)

	assert.Equal(t, EventTaskFailed, event.Type)
	assert.Equal(t, "task-789", event.Data["task_id"])
	assert.Equal(t, "timeout", event.Data["error"])
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte("invalid json"))
	assert.Error(t, err)
}

func TestEvent_RoundTrip(t *testing.T) {
	original := NewEvent(EventWorkerJoined, map[string]interface{}{
		"worker_id": "worker-1",
		"state":     "active",
	})

	data, err := original.ToJSON()
	require.NoError(t, err)

	restored, err := FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Data["worker_id"], restored.Data["worker_id"])
	assert.Equal(t, original.Data["state"], restored.Data["state"])
}

func TestTaskEventData(t *testing.T) {
	data := TaskEventData("task-123", "email", "high", map[string]interface{}{
		"attempts": 1,
		"error":    "timeout",
	})

	assert.Equal(t, "task-123", data["task_id"])
	assert.Equal(t, "email", data["type"])
	assert.Equal(t, "high", data["priority"])
	assert.Equal(t, 1, data["attempts"])
	assert.Equal(t, "timeout", data["error"])
}

func TestTaskEventData_NoExtra(t *testing.T) {
	data := TaskEventData("task-456", "compute", "normal", nil)

	assert.Equal(t, "task-456", data["task_id"])
	assert.Equal(t, "compute", data["type"])
	assert.Equal(t, "normal", data["priority"])
	assert.Len(t, data, 3)
}

func TestWorkerEventData(t *testing.T) {
	data := WorkerEventData("worker-1", "active", map[string]interface{}{
		"concurrency":  10,
		"active_tasks": 5,
	})

	assert.Equal(t, "worker-1", data["worker_id"])
	assert.Equal(t, "active", data["state"])
	assert.Equal(t, 10, data["concurrency"])
	assert.Equal(t, 5, data["active_tasks"])
}

func TestWorkerEventData_NoExtra(t *testing.T) {
	data := WorkerEventData("worker-2", "paused", nil)

	assert.Equal(t, "worker-2", data["worker_id"])
	assert.Equal(t, "paused", data["state"])
	assert.Len(t, data, 2)
}

func TestQueueDepthData(t *testing.T) {
	depths := map[string]int64{
		"critical": 10,
		"high":     50,
		"normal":   100,
		"low":      25,
	}

	data := QueueDepthData(depths)

	assert.NotNil(t, data["depths"])
	depthsData := data["depths"].(map[string]int64)
	assert.Equal(t, int64(10), depthsData["critical"])
	assert.Equal(t, int64(50), depthsData["high"])
	assert.Equal(t, int64(100), depthsData["normal"])
	assert.Equal(t, int64(25), depthsData["low"])
}
