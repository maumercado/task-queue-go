package task

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriority_String(t *testing.T) {
	tests := []struct {
		priority Priority
		expected string
	}{
		{PriorityLow, "low"},
		{PriorityNormal, "normal"},
		{PriorityHigh, "high"},
		{PriorityCritical, "critical"},
		{Priority(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.String())
		})
	}
}

func TestPriority_StreamName(t *testing.T) {
	tests := []struct {
		priority Priority
		prefix   string
		expected string
	}{
		{PriorityLow, "tasks", "tasks:low"},
		{PriorityNormal, "tasks", "tasks:normal"},
		{PriorityHigh, "queue", "queue:high"},
		{PriorityCritical, "jobs", "jobs:critical"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority.StreamName(tt.prefix))
		})
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input    string
		expected Priority
	}{
		{"low", PriorityLow},
		{"normal", PriorityNormal},
		{"high", PriorityHigh},
		{"critical", PriorityCritical},
		{"invalid", PriorityNormal}, // Default
		{"", PriorityNormal},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParsePriority(tt.input))
		})
	}
}

func TestPriorityFromInt(t *testing.T) {
	tests := []struct {
		input    int
		expected Priority
	}{
		{0, PriorityLow},
		{1, PriorityNormal},
		{2, PriorityHigh},
		{3, PriorityCritical},
		{-1, PriorityNormal}, // Out of range
		{4, PriorityNormal},  // Out of range
		{99, PriorityNormal}, // Out of range
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.expected, PriorityFromInt(tt.input))
		})
	}
}

func TestNew(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	task := New("test-type", payload, PriorityHigh)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "test-type", task.Type)
	assert.Equal(t, payload, task.Payload)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Equal(t, StatePending, task.State)
	assert.Equal(t, 0, task.Attempts)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 5*time.Minute, task.Timeout)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
	assert.NotNil(t, task.Metadata)
}

func TestFromRequest(t *testing.T) {
	now := time.Now().UTC()
	req := &CreateTaskRequest{
		Type:        "email",
		Payload:     map[string]interface{}{"to": "user@example.com"},
		Priority:    2, // High
		MaxRetries:  5,
		Timeout:     120, // 2 minutes in seconds
		ScheduledAt: &now,
		Metadata:    map[string]string{"source": "api"},
	}

	task := FromRequest(req)

	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "email", task.Type)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Equal(t, 5, task.MaxRetries)
	assert.Equal(t, 120*time.Second, task.Timeout)
	assert.NotNil(t, task.ScheduledAt)
	assert.Equal(t, "api", task.Metadata["source"])
}

func TestFromRequest_Defaults(t *testing.T) {
	req := &CreateTaskRequest{
		Type:    "simple",
		Payload: nil,
	}

	task := FromRequest(req)

	assert.Equal(t, PriorityLow, task.Priority)
	assert.Equal(t, 3, task.MaxRetries)
	assert.Equal(t, 5*time.Minute, task.Timeout)
	assert.Nil(t, task.ScheduledAt)
}

func TestTask_ToResponse(t *testing.T) {
	now := time.Now().UTC()
	task := &Task{
		ID:          "task-123",
		Type:        "test",
		Payload:     map[string]interface{}{"key": "value"},
		Priority:    PriorityHigh,
		State:       StateRunning,
		Attempts:    1,
		MaxRetries:  3,
		WorkerID:    "worker-1",
		CreatedAt:   now,
		UpdatedAt:   now,
		StartedAt:   &now,
		Metadata:    map[string]string{"key": "value"},
	}

	resp := task.ToResponse()

	assert.Equal(t, "task-123", resp.ID)
	assert.Equal(t, "test", resp.Type)
	assert.Equal(t, "high", resp.Priority)
	assert.Equal(t, "running", resp.State)
	assert.Equal(t, 1, resp.Attempts)
	assert.Equal(t, "worker-1", resp.WorkerID)
}

func TestTask_ToJSON_FromJSON(t *testing.T) {
	original := New("test", map[string]interface{}{"key": "value"}, PriorityNormal)

	data, err := original.ToJSON()
	require.NoError(t, err)

	restored, err := FromJSON(data)
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.Type, restored.Type)
	assert.Equal(t, original.Priority, restored.Priority)
	assert.Equal(t, original.State, restored.State)
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte("invalid json"))
	assert.Error(t, err)
}

func TestTask_ToMap_FromMap(t *testing.T) {
	original := New("test", map[string]interface{}{"key": "value"}, PriorityHigh)

	m := original.ToMap()
	assert.Contains(t, m, "data")

	restored, err := FromMap(m)
	require.NoError(t, err)
	assert.Equal(t, original.ID, restored.ID)
}

func TestFromMap_Invalid(t *testing.T) {
	_, err := FromMap(map[string]interface{}{})
	assert.Equal(t, ErrInvalidTaskData, err)

	_, err = FromMap(map[string]interface{}{"data": 123})
	assert.Equal(t, ErrInvalidTaskData, err)
}

func TestTask_CanRetry(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	task.MaxRetries = 3

	task.Attempts = 0
	assert.True(t, task.CanRetry())

	task.Attempts = 2
	assert.True(t, task.CanRetry())

	task.Attempts = 3
	assert.False(t, task.CanRetry())

	task.Attempts = 5
	assert.False(t, task.CanRetry())
}

func TestTask_IncrementAttempts(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	originalUpdatedAt := task.UpdatedAt

	time.Sleep(time.Millisecond)
	task.IncrementAttempts()

	assert.Equal(t, 1, task.Attempts)
	assert.True(t, task.UpdatedAt.After(originalUpdatedAt))
}

func TestTask_JSONMarshal_Unmarshal(t *testing.T) {
	now := time.Now().UTC()
	task := &Task{
		ID:          "test-id",
		Type:        "email",
		Payload:     map[string]interface{}{"to": "test@example.com"},
		Priority:    PriorityHigh,
		State:       StatePending,
		Attempts:    0,
		MaxRetries:  3,
		CreatedAt:   now,
		UpdatedAt:   now,
		Timeout:     5 * time.Minute,
		Metadata:    map[string]string{"source": "api"},
	}

	data, err := json.Marshal(task)
	require.NoError(t, err)

	var restored Task
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, task.ID, restored.ID)
	assert.Equal(t, task.Type, restored.Type)
	assert.Equal(t, task.Priority, restored.Priority)
}
