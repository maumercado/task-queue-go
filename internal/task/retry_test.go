package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	assert.Equal(t, 3, policy.MaxAttempts)
	assert.Equal(t, 1*time.Second, policy.InitialBackoff)
	assert.Equal(t, 5*time.Minute, policy.MaxBackoff)
	assert.Equal(t, 2.0, policy.BackoffFactor)
	assert.Equal(t, 0.1, policy.JitterFactor)
}

func TestRetryPolicy_CalculateBackoff(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},            // Initial
		{1, 2 * time.Second},            // 1 * 2^1
		{2, 4 * time.Second},            // 1 * 2^2
		{3, 8 * time.Second},            // 1 * 2^3
		{4, 16 * time.Second},           // 1 * 2^4
		{10, 1 * time.Minute},           // Capped at max
	}

	for _, tt := range tests {
		backoff := policy.CalculateBackoff(tt.attempt)
		assert.Equal(t, tt.expected, backoff, "attempt %d", tt.attempt)
	}
}

func TestRetryPolicy_CalculateBackoff_WithJitter(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0.5,
	}

	// With jitter, result should be within range
	for i := 0; i < 10; i++ {
		backoff := policy.CalculateBackoff(1)
		// Base is 2s, with 50% jitter, range is 1s-3s
		assert.GreaterOrEqual(t, backoff, 1*time.Second)
		assert.LessOrEqual(t, backoff, 3*time.Second)
	}
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts: 3,
	}

	tests := []struct {
		attempts int
		expected bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, false},
		{5, false},
	}

	for _, tt := range tests {
		task := &Task{Attempts: tt.attempts}
		assert.Equal(t, tt.expected, policy.ShouldRetry(task), "attempts: %d", tt.attempts)
	}
}

func TestRetryPolicy_NextRetryTime(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0,
	}

	task := &Task{Attempts: 1}
	before := time.Now().UTC()
	retryTime := policy.NextRetryTime(task)
	after := time.Now().UTC()

	// Should be approximately 2 seconds from now
	expectedMin := before.Add(2 * time.Second)
	expectedMax := after.Add(2 * time.Second)

	assert.True(t, retryTime.After(expectedMin) || retryTime.Equal(expectedMin))
	assert.True(t, retryTime.Before(expectedMax) || retryTime.Equal(expectedMax))
}

func TestRetryPolicy_GetRetryInfo(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0,
	}

	task := &Task{Attempts: 1}
	info := policy.GetRetryInfo(task)

	assert.True(t, info.ShouldRetry)
	assert.Equal(t, 2, info.AttemptsLeft)
	assert.Equal(t, 3, info.TotalAttempts)
	assert.Equal(t, 2*time.Second, info.BackoffDelay)
}

func TestNewRetryer_Default(t *testing.T) {
	retryer := NewRetryer(nil)
	assert.NotNil(t, retryer)
	assert.Equal(t, 3, retryer.policy.MaxAttempts)
}

func TestNewRetryer_CustomPolicy(t *testing.T) {
	policy := &RetryPolicy{MaxAttempts: 5}
	retryer := NewRetryer(policy)
	assert.Equal(t, 5, retryer.policy.MaxAttempts)
}

func TestRetryer_ProcessFailure_ShouldRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0,
	}
	retryer := NewRetryer(policy)

	task := &Task{Attempts: 1}
	shouldRetry, retryAt := retryer.ProcessFailure(task, "error message")

	assert.True(t, shouldRetry)
	assert.False(t, retryAt.IsZero())
	assert.Equal(t, "error message", task.Error)
}

func TestRetryer_ProcessFailure_NoRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts: 2,
	}
	retryer := NewRetryer(policy)

	task := &Task{Attempts: 3}
	shouldRetry, retryAt := retryer.ProcessFailure(task, "error message")

	assert.False(t, shouldRetry)
	assert.True(t, retryAt.IsZero())
}

func TestRetryer_ScheduleRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     1 * time.Minute,
		BackoffFactor:  2.0,
		JitterFactor:   0,
	}
	retryer := NewRetryer(policy)

	task := &Task{
		State:      StateRunning,
		Attempts:   1,
		MaxRetries: 3,
	}

	// First transition to failed
	sm := NewStateMachine(task)
	err := sm.Fail("error")
	require.NoError(t, err)

	// Then schedule retry
	result, err := retryer.ScheduleRetry(task)
	require.NoError(t, err)

	assert.Equal(t, StateRetrying, result.State)
	assert.NotNil(t, result.ScheduledAt)
}

func TestRetryer_PrepareForRequeue(t *testing.T) {
	policy := DefaultRetryPolicy()
	retryer := NewRetryer(policy)

	task := &Task{
		State:       StateRunning,
		Attempts:    1,
		ScheduledAt: func() *time.Time { t := time.Now(); return &t }(),
	}

	// Transition to failed first
	sm := NewStateMachine(task)
	sm.Fail("error")

	retryer.PrepareForRequeue(task)

	assert.Equal(t, StatePending, task.State)
	assert.Nil(t, task.ScheduledAt)
}
