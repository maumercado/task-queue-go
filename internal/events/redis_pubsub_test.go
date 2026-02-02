package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRedisPubSub(t *testing.T) {
	// Test with nil client - should create struct correctly even with nil
	// (actual operations would fail but construction should work)
	pubsub := NewRedisPubSub(nil)

	assert.NotNil(t, pubsub)
	assert.Nil(t, pubsub.client)
	assert.NotNil(t, pubsub.subscribers)
	assert.Len(t, pubsub.subscribers, 0)
}

func TestRedisPubSub_channelName(t *testing.T) {
	pubsub := NewRedisPubSub(nil)

	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventTaskSubmitted, "taskqueue:events:task.submitted"},
		{EventTaskStarted, "taskqueue:events:task.started"},
		{EventTaskCompleted, "taskqueue:events:task.completed"},
		{EventTaskFailed, "taskqueue:events:task.failed"},
		{EventTaskRetrying, "taskqueue:events:task.retrying"},
		{EventWorkerJoined, "taskqueue:events:worker.joined"},
		{EventWorkerLeft, "taskqueue:events:worker.left"},
		{EventWorkerPaused, "taskqueue:events:worker.paused"},
		{EventWorkerResumed, "taskqueue:events:worker.resumed"},
		{EventQueueDepth, "taskqueue:events:queue.depth"},
		{EventSystemMetrics, "taskqueue:events:system.metrics"},
	}

	for _, tc := range tests {
		t.Run(string(tc.eventType), func(t *testing.T) {
			channel := pubsub.channelName(tc.eventType)
			assert.Equal(t, tc.expected, channel)
		})
	}
}

func TestRedisPubSub_Close_EmptySubscribers(t *testing.T) {
	pubsub := NewRedisPubSub(nil)

	// Should not panic with empty subscribers
	err := pubsub.Close()
	assert.NoError(t, err)
	assert.Len(t, pubsub.subscribers, 0)
}

func TestChannelPrefix(t *testing.T) {
	assert.Equal(t, "taskqueue:events:", channelPrefix)
}
