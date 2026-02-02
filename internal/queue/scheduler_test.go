package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
