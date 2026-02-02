package task

import (
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines the retry behavior for failed tasks
type RetryPolicy struct {
	MaxAttempts    int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	BackoffFactor  float64       // Multiplier for exponential backoff
	JitterFactor   float64       // Random jitter factor (0.0 to 1.0)
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0.1,
	}
}

// CalculateBackoff calculates the backoff duration for a given attempt number
func (p *RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return p.InitialBackoff
	}

	// Calculate exponential backoff: initial * factor^attempt
	backoff := float64(p.InitialBackoff) * math.Pow(p.BackoffFactor, float64(attempt))

	// Cap at max backoff
	if backoff > float64(p.MaxBackoff) {
		backoff = float64(p.MaxBackoff)
	}

	// Add jitter
	if p.JitterFactor > 0 {
		jitter := backoff * p.JitterFactor * (rand.Float64()*2 - 1) // -jitter to +jitter
		backoff += jitter
	}

	// Ensure non-negative
	if backoff < 0 {
		backoff = float64(p.InitialBackoff)
	}

	return time.Duration(backoff)
}

// ShouldRetry determines if a task should be retried based on the policy
func (p *RetryPolicy) ShouldRetry(t *Task) bool {
	return t.Attempts < p.MaxAttempts
}

// NextRetryTime calculates when the task should be retried
func (p *RetryPolicy) NextRetryTime(t *Task) time.Time {
	backoff := p.CalculateBackoff(t.Attempts)
	return time.Now().UTC().Add(backoff)
}

// RetryInfo contains information about retry scheduling
type RetryInfo struct {
	ShouldRetry   bool
	NextRetryAt   time.Time
	BackoffDelay  time.Duration
	AttemptsLeft  int
	TotalAttempts int
}

// GetRetryInfo returns comprehensive retry information for a task
func (p *RetryPolicy) GetRetryInfo(t *Task) *RetryInfo {
	shouldRetry := p.ShouldRetry(t)
	backoff := p.CalculateBackoff(t.Attempts)

	return &RetryInfo{
		ShouldRetry:   shouldRetry,
		NextRetryAt:   time.Now().UTC().Add(backoff),
		BackoffDelay:  backoff,
		AttemptsLeft:  p.MaxAttempts - t.Attempts,
		TotalAttempts: p.MaxAttempts,
	}
}

// Retryer handles retry logic for tasks
type Retryer struct {
	policy *RetryPolicy
}

// NewRetryer creates a new Retryer with the given policy
func NewRetryer(policy *RetryPolicy) *Retryer {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}
	return &Retryer{policy: policy}
}

// ProcessFailure handles a task failure and determines the next action
func (r *Retryer) ProcessFailure(t *Task, errMsg string) (shouldRetry bool, retryAt time.Time) {
	t.Error = errMsg
	t.UpdatedAt = time.Now().UTC()

	if r.policy.ShouldRetry(t) {
		return true, r.policy.NextRetryTime(t)
	}

	return false, time.Time{}
}

// ScheduleRetry prepares a task for retry
func (r *Retryer) ScheduleRetry(t *Task) (*Task, error) {
	sm := NewStateMachine(t)
	if err := sm.Retry(); err != nil {
		return nil, err
	}

	// Set scheduled retry time
	retryAt := r.policy.NextRetryTime(t)
	t.ScheduledAt = &retryAt

	return t, nil
}

// PrepareForRequeue prepares a task to be placed back in the queue
func (r *Retryer) PrepareForRequeue(t *Task) {
	// Transition to pending via retrying
	sm := NewStateMachine(t)
	_ = sm.Retry() // Ignore error, will be in retrying state

	// Then back to pending for immediate requeue
	t.State = StatePending
	t.ScheduledAt = nil
	t.UpdatedAt = time.Now().UTC()
}
