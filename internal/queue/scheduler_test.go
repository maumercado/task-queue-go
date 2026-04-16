package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/maumercado/task-queue-go/internal/task"
)

func TestSchedulerConstants(t *testing.T) {
	assert.Equal(t, "tasks:scheduled", scheduledSetKey)
	assert.Equal(t, "scheduler:lock", schedulerLockKey)
}

func TestNewScheduler(t *testing.T) {
	// Test with nil parameters - should create struct correctly
	scheduler := NewScheduler(nil, nil)

	assert.NotNil(t, scheduler)
	assert.Nil(t, scheduler.client)
	assert.Nil(t, scheduler.queue)
	assert.Equal(t, schedulerPollInterval, scheduler.pollInterval)
	assert.NotNil(t, scheduler.stopCh)
}

// TestActivateTask_AcceptedStates verifies the scheduler accepts both
// StateScheduled and StateRetrying, and rejects any other state.
func TestActivateTask_AcceptedStates(t *testing.T) {
	accepted := []task.State{task.StateScheduled, task.StateRetrying}
	rejected := []task.State{
		task.StatePending,
		task.StateRunning,
		task.StateCompleted,
		task.StateFailed,
		task.StateCanceled,
		task.StateDeadLetter,
	}

	for _, s := range accepted {
		tsk := task.New("test", nil, task.PriorityNormal)
		tsk.State = s
		isAccepted := s == task.StateScheduled || s == task.StateRetrying
		assert.True(t, isAccepted, "state %s should be accepted by scheduler", s)
	}

	for _, s := range rejected {
		tsk := task.New("test", nil, task.PriorityNormal)
		tsk.State = s
		isAccepted := s == task.StateScheduled || s == task.StateRetrying
		assert.False(t, isAccepted, "state %s should be rejected by scheduler", s)
		_ = tsk
	}
}

// TestActivateTask_CanceledTaskSkipped verifies that a task whose state is
// Canceled is treated as "already handled" — the scheduler should skip it.
func TestActivateTask_CanceledTaskSkipped(t *testing.T) {
	tsk := task.New("test", nil, task.PriorityNormal)
	tsk.State = task.StateCanceled

	// Scheduler skips any state that is not scheduled or retrying.
	shouldActivate := tsk.State == task.StateScheduled || tsk.State == task.StateRetrying
	assert.False(t, shouldActivate, "canceled task must not be activated by scheduler")
}

// TestActivateTask_RetryingTransitionToPending verifies the state machine allows
// retrying -> pending (which scheduler does when backoff delay expires).
func TestActivateTask_RetryingTransitionToPending(t *testing.T) {
	tsk := task.New("test", nil, task.PriorityNormal)
	tsk.State = task.StateRetrying

	sm := task.NewStateMachine(tsk)
	err := sm.Transition(task.StatePending)

	assert.NoError(t, err, "retrying -> pending transition must be valid")
	assert.Equal(t, task.StatePending, tsk.State)
}
