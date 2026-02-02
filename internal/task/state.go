package task

import (
	"errors"
	"time"
)

// State represents the current state of a task
type State int

const (
	StatePending State = iota
	StateScheduled
	StateRunning
	StateCompleted
	StateFailed
	StateRetrying
	StateCancelled
	StateDeadLetter
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateScheduled:
		return "scheduled"
	case StateRunning:
		return "running"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateRetrying:
		return "retrying"
	case StateCancelled:
		return "cancelled"
	case StateDeadLetter:
		return "dead_letter"
	default:
		return "unknown"
	}
}

func ParseState(s string) State {
	switch s {
	case "pending":
		return StatePending
	case "scheduled":
		return StateScheduled
	case "running":
		return StateRunning
	case "completed":
		return StateCompleted
	case "failed":
		return StateFailed
	case "retrying":
		return StateRetrying
	case "cancelled":
		return StateCancelled
	case "dead_letter":
		return StateDeadLetter
	default:
		return StatePending
	}
}

// IsFinal returns true if the state is a terminal state
func (s State) IsFinal() bool {
	return s == StateCompleted || s == StateFailed || s == StateCancelled || s == StateDeadLetter
}

// IsActive returns true if the task is actively being processed
func (s State) IsActive() bool {
	return s == StateRunning || s == StateRetrying
}

// Error definitions
var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrInvalidTaskData   = errors.New("invalid task data")
	ErrTaskNotFound      = errors.New("task not found")
	ErrTaskAlreadyExists = errors.New("task already exists")
)

// ValidTransitions defines the allowed state transitions
var ValidTransitions = map[State][]State{
	StatePending:    {StateScheduled, StateRunning, StateCancelled},
	StateScheduled:  {StatePending, StateRunning, StateCancelled},
	StateRunning:    {StateCompleted, StateFailed, StateRetrying, StateCancelled},
	StateRetrying:   {StateRunning, StateFailed, StateDeadLetter, StateCancelled},
	StateFailed:     {StateRetrying, StateDeadLetter, StatePending}, // Can retry or move to DLQ
	StateCompleted:  {},                                             // Terminal state
	StateCancelled:  {},                                             // Terminal state
	StateDeadLetter: {StatePending},                                 // Can be re-queued
}

// CanTransitionTo checks if a transition from current state to target state is valid
func (s State) CanTransitionTo(target State) bool {
	validTargets, ok := ValidTransitions[s]
	if !ok {
		return false
	}
	for _, v := range validTargets {
		if v == target {
			return true
		}
	}
	return false
}

// StateMachine handles task state transitions
type StateMachine struct {
	task *Task
}

// NewStateMachine creates a new state machine for a task
func NewStateMachine(task *Task) *StateMachine {
	return &StateMachine{task: task}
}

// Transition attempts to transition the task to a new state
func (sm *StateMachine) Transition(target State) error {
	if !sm.task.State.CanTransitionTo(target) {
		return ErrInvalidTransition
	}

	now := time.Now().UTC()
	sm.task.State = target
	sm.task.UpdatedAt = now

	// Handle state-specific updates
	switch target {
	case StateRunning:
		sm.task.StartedAt = &now
	case StateCompleted, StateFailed, StateCancelled, StateDeadLetter:
		sm.task.CompletedAt = &now
	}

	return nil
}

// Start transitions the task to running state
func (sm *StateMachine) Start(workerID string) error {
	if err := sm.Transition(StateRunning); err != nil {
		return err
	}
	sm.task.WorkerID = workerID
	sm.task.IncrementAttempts()
	return nil
}

// Complete transitions the task to completed state
func (sm *StateMachine) Complete(result map[string]interface{}) error {
	if err := sm.Transition(StateCompleted); err != nil {
		return err
	}
	sm.task.Result = result
	sm.task.Error = ""
	return nil
}

// Fail transitions the task to failed state
func (sm *StateMachine) Fail(errMsg string) error {
	if err := sm.Transition(StateFailed); err != nil {
		return err
	}
	sm.task.Error = errMsg
	return nil
}

// Retry transitions the task to retrying state
func (sm *StateMachine) Retry() error {
	if !sm.task.CanRetry() {
		// Move to dead letter queue if max retries exceeded
		return sm.Transition(StateDeadLetter)
	}
	return sm.Transition(StateRetrying)
}

// Cancel transitions the task to cancelled state
func (sm *StateMachine) Cancel() error {
	return sm.Transition(StateCancelled)
}

// MoveToDLQ transitions the task to dead letter queue
func (sm *StateMachine) MoveToDLQ() error {
	return sm.Transition(StateDeadLetter)
}

// Requeue transitions the task back to pending state
func (sm *StateMachine) Requeue() error {
	// Reset for reprocessing
	sm.task.WorkerID = ""
	sm.task.Attempts = 0
	sm.task.Error = ""
	sm.task.StartedAt = nil
	sm.task.CompletedAt = nil
	return sm.Transition(StatePending)
}
