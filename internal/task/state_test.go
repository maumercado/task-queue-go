package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StatePending, "pending"},
		{StateScheduled, "scheduled"},
		{StateRunning, "running"},
		{StateCompleted, "completed"},
		{StateFailed, "failed"},
		{StateRetrying, "retrying"},
		{StateCancelled, "cancelled"},
		{StateDeadLetter, "dead_letter"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		input    string
		expected State
	}{
		{"pending", StatePending},
		{"scheduled", StateScheduled},
		{"running", StateRunning},
		{"completed", StateCompleted},
		{"failed", StateFailed},
		{"retrying", StateRetrying},
		{"cancelled", StateCancelled},
		{"dead_letter", StateDeadLetter},
		{"invalid", StatePending}, // Default
		{"", StatePending},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseState(tt.input))
		})
	}
}

func TestState_IsFinal(t *testing.T) {
	finalStates := []State{StateCompleted, StateFailed, StateCancelled, StateDeadLetter}
	nonFinalStates := []State{StatePending, StateScheduled, StateRunning, StateRetrying}

	for _, state := range finalStates {
		assert.True(t, state.IsFinal(), "Expected %s to be final", state)
	}

	for _, state := range nonFinalStates {
		assert.False(t, state.IsFinal(), "Expected %s to not be final", state)
	}
}

func TestState_IsActive(t *testing.T) {
	activeStates := []State{StateRunning, StateRetrying}
	inactiveStates := []State{StatePending, StateScheduled, StateCompleted, StateFailed, StateCancelled, StateDeadLetter}

	for _, state := range activeStates {
		assert.True(t, state.IsActive(), "Expected %s to be active", state)
	}

	for _, state := range inactiveStates {
		assert.False(t, state.IsActive(), "Expected %s to not be active", state)
	}
}

func TestState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from    State
		to      State
		allowed bool
	}{
		// From Pending
		{StatePending, StateScheduled, true},
		{StatePending, StateRunning, true},
		{StatePending, StateCancelled, true},
		{StatePending, StateCompleted, false},
		{StatePending, StateFailed, false},

		// From Running
		{StateRunning, StateCompleted, true},
		{StateRunning, StateFailed, true},
		{StateRunning, StateRetrying, true},
		{StateRunning, StateCancelled, true},
		{StateRunning, StatePending, false},

		// From Failed
		{StateFailed, StateRetrying, true},
		{StateFailed, StateDeadLetter, true},
		{StateFailed, StatePending, true},
		{StateFailed, StateCompleted, false},

		// From Completed (terminal)
		{StateCompleted, StatePending, false},
		{StateCompleted, StateRunning, false},

		// From Cancelled (terminal)
		{StateCancelled, StatePending, false},

		// From DeadLetter
		{StateDeadLetter, StatePending, true},
		{StateDeadLetter, StateRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.from.String()+"->"+tt.to.String(), func(t *testing.T) {
			assert.Equal(t, tt.allowed, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func TestStateMachine_Transition(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	// Valid transition
	err := sm.Transition(StateRunning)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, task.State)
	assert.NotNil(t, task.StartedAt)

	// Valid transition to completed
	err = sm.Transition(StateCompleted)
	require.NoError(t, err)
	assert.Equal(t, StateCompleted, task.State)
	assert.NotNil(t, task.CompletedAt)
}

func TestStateMachine_Transition_Invalid(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	// Invalid transition from pending to completed
	err := sm.Transition(StateCompleted)
	assert.Equal(t, ErrInvalidTransition, err)
	assert.Equal(t, StatePending, task.State)
}

func TestStateMachine_Start(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	err := sm.Start("worker-123")
	require.NoError(t, err)

	assert.Equal(t, StateRunning, task.State)
	assert.Equal(t, "worker-123", task.WorkerID)
	assert.Equal(t, 1, task.Attempts)
	assert.NotNil(t, task.StartedAt)
}

func TestStateMachine_Complete(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	// First start the task
	err := sm.Start("worker-123")
	require.NoError(t, err)

	// Then complete it
	result := map[string]interface{}{"output": "success"}
	err = sm.Complete(result)
	require.NoError(t, err)

	assert.Equal(t, StateCompleted, task.State)
	assert.Equal(t, result, task.Result)
	assert.Empty(t, task.Error)
	assert.NotNil(t, task.CompletedAt)
}

func TestStateMachine_Fail(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	// First start the task
	err := sm.Start("worker-123")
	require.NoError(t, err)

	// Then fail it
	err = sm.Fail("something went wrong")
	require.NoError(t, err)

	assert.Equal(t, StateFailed, task.State)
	assert.Equal(t, "something went wrong", task.Error)
}

func TestStateMachine_Retry_WithRetriesLeft(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	task.MaxRetries = 3
	task.Attempts = 1
	sm := NewStateMachine(task)

	// Start then fail
	sm.Start("worker-123")
	sm.Fail("error")

	// Should transition to retrying
	err := sm.Retry()
	require.NoError(t, err)
	assert.Equal(t, StateRetrying, task.State)
}

func TestStateMachine_Retry_NoRetriesLeft(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	task.MaxRetries = 1
	task.Attempts = 2
	sm := NewStateMachine(task)

	// Start then fail
	sm.Start("worker-123")
	sm.Fail("error")

	// Should transition to dead letter
	err := sm.Retry()
	require.NoError(t, err)
	assert.Equal(t, StateDeadLetter, task.State)
}

func TestStateMachine_Cancel(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	err := sm.Cancel()
	require.NoError(t, err)
	assert.Equal(t, StateCancelled, task.State)
}

func TestStateMachine_MoveToDLQ(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	sm := NewStateMachine(task)

	// Start -> Fail -> DLQ
	sm.Start("worker")
	sm.Fail("error")

	err := sm.MoveToDLQ()
	require.NoError(t, err)
	assert.Equal(t, StateDeadLetter, task.State)
}

func TestStateMachine_Requeue(t *testing.T) {
	task := New("test", nil, PriorityNormal)
	task.State = StateDeadLetter
	task.WorkerID = "old-worker"
	task.Attempts = 5
	task.Error = "previous error"
	now := time.Now()
	task.StartedAt = &now
	task.CompletedAt = &now

	sm := NewStateMachine(task)
	err := sm.Requeue()
	require.NoError(t, err)

	assert.Equal(t, StatePending, task.State)
	assert.Empty(t, task.WorkerID)
	assert.Equal(t, 0, task.Attempts)
	assert.Empty(t, task.Error)
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
}
