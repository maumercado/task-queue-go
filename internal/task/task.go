package task

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Priority levels for task ordering
type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

func (p Priority) StreamName(prefix string) string {
	return prefix + ":" + p.String()
}

func ParsePriority(s string) Priority {
	switch s {
	case "low":
		return PriorityLow
	case "normal":
		return PriorityNormal
	case "high":
		return PriorityHigh
	case "critical":
		return PriorityCritical
	default:
		return PriorityNormal
	}
}

// PriorityFromInt converts an integer to Priority
func PriorityFromInt(i int) Priority {
	if i < 0 || i > 3 {
		return PriorityNormal
	}
	return Priority(i)
}

// Task represents a unit of work in the queue
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    Priority               `json:"priority"`
	State       State                  `json:"state"`
	Attempts    int                    `json:"attempts"`
	MaxRetries  int                    `json:"max_retries"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	WorkerID    string                 `json:"worker_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	Timeout     time.Duration          `json:"timeout"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// CreateTaskRequest represents the API request for creating a task
type CreateTaskRequest struct {
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    int                    `json:"priority"`
	MaxRetries  int                    `json:"max_retries,omitempty"`
	Timeout     int                    `json:"timeout,omitempty"` // in seconds
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// TaskResponse represents the API response for a task
type TaskResponse struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    string                 `json:"priority"`
	State       string                 `json:"state"`
	Attempts    int                    `json:"attempts"`
	MaxRetries  int                    `json:"max_retries"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	WorkerID    string                 `json:"worker_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// New creates a new Task with default values
func New(taskType string, payload map[string]interface{}, priority Priority) *Task {
	now := time.Now().UTC()
	return &Task{
		ID:         uuid.New().String(),
		Type:       taskType,
		Payload:    payload,
		Priority:   priority,
		State:      StatePending,
		Attempts:   0,
		MaxRetries: 3,
		CreatedAt:  now,
		UpdatedAt:  now,
		Timeout:    5 * time.Minute, // Default timeout
		Metadata:   make(map[string]string),
	}
}

// FromRequest creates a Task from a CreateTaskRequest
func FromRequest(req *CreateTaskRequest) *Task {
	task := New(req.Type, req.Payload, PriorityFromInt(req.Priority))

	if req.MaxRetries > 0 {
		task.MaxRetries = req.MaxRetries
	}
	if req.Timeout > 0 {
		task.Timeout = time.Duration(req.Timeout) * time.Second
	}
	if req.ScheduledAt != nil {
		task.ScheduledAt = req.ScheduledAt
	}
	if req.Metadata != nil {
		task.Metadata = req.Metadata
	}

	return task
}

// ToResponse converts a Task to a TaskResponse
func (t *Task) ToResponse() *TaskResponse {
	return &TaskResponse{
		ID:          t.ID,
		Type:        t.Type,
		Payload:     t.Payload,
		Priority:    t.Priority.String(),
		State:       t.State.String(),
		Attempts:    t.Attempts,
		MaxRetries:  t.MaxRetries,
		Error:       t.Error,
		Result:      t.Result,
		WorkerID:    t.WorkerID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		StartedAt:   t.StartedAt,
		CompletedAt: t.CompletedAt,
		Metadata:    t.Metadata,
	}
}

// ToJSON serializes the task to JSON
func (t *Task) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON deserializes a task from JSON
func FromJSON(data []byte) (*Task, error) {
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ToMap converts the task to a map for Redis storage
func (t *Task) ToMap() map[string]interface{} {
	data, _ := t.ToJSON()
	return map[string]interface{}{
		"data": string(data),
	}
}

// FromMap creates a task from a Redis map
func FromMap(m map[string]interface{}) (*Task, error) {
	data, ok := m["data"].(string)
	if !ok {
		return nil, ErrInvalidTaskData
	}
	return FromJSON([]byte(data))
}

// CanRetry returns true if the task can be retried
func (t *Task) CanRetry() bool {
	return t.Attempts < t.MaxRetries
}

// IncrementAttempts increments the attempt counter
func (t *Task) IncrementAttempts() {
	t.Attempts++
	t.UpdatedAt = time.Now().UTC()
}
