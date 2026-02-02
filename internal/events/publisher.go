package events

import (
	"context"
	"encoding/json"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// Task events
	EventTaskSubmitted EventType = "task.submitted"
	EventTaskStarted   EventType = "task.started"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskRetrying  EventType = "task.retrying"

	// Worker events
	EventWorkerJoined  EventType = "worker.joined"
	EventWorkerLeft    EventType = "worker.left"
	EventWorkerPaused  EventType = "worker.paused"
	EventWorkerResumed EventType = "worker.resumed"

	// System events
	EventQueueDepth    EventType = "queue.depth"
	EventSystemMetrics EventType = "system.metrics"
)

// Event represents a system event
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType, data map[string]interface{}) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
}

// ToJSON serializes the event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes an event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Publisher defines the interface for event publishers
type Publisher interface {
	Publish(ctx context.Context, event *Event) error
	Subscribe(ctx context.Context, eventTypes ...EventType) (<-chan *Event, error)
	Close() error
}

// Subscriber represents an event subscriber
type Subscriber interface {
	OnEvent(event *Event)
	EventTypes() []EventType
}

// TaskEventData creates event data for task events
func TaskEventData(taskID, taskType, priority string, extra map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"task_id":  taskID,
		"type":     taskType,
		"priority": priority,
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// WorkerEventData creates event data for worker events
func WorkerEventData(workerID, state string, extra map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"worker_id": workerID,
		"state":     state,
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// QueueDepthData creates event data for queue depth events
func QueueDepthData(depths map[string]int64) map[string]interface{} {
	return map[string]interface{}{
		"depths": depths,
	}
}
