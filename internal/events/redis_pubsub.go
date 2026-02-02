package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/maumercado/task-queue-go/internal/logger"
)

const (
	channelPrefix = "taskqueue:events:"
)

// RedisPubSub implements Publisher using Redis Pub/Sub
type RedisPubSub struct {
	client      *redis.Client
	subscribers map[string]*redis.PubSub
	mu          sync.RWMutex
}

// NewRedisPubSub creates a new Redis Pub/Sub publisher
func NewRedisPubSub(client *redis.Client) *RedisPubSub {
	return &RedisPubSub{
		client:      client,
		subscribers: make(map[string]*redis.PubSub),
	}
}

// Publish publishes an event to Redis
func (r *RedisPubSub) Publish(ctx context.Context, event *Event) error {
	channel := r.channelName(event.Type)
	data, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	if err := r.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	logger.Debug().
		Str("event_type", string(event.Type)).
		Str("channel", channel).
		Msg("event published")

	return nil
}

// Subscribe subscribes to events of the specified types
func (r *RedisPubSub) Subscribe(ctx context.Context, eventTypes ...EventType) (<-chan *Event, error) {
	channels := make([]string, len(eventTypes))
	for i, et := range eventTypes {
		channels[i] = r.channelName(et)
	}

	pubsub := r.client.Subscribe(ctx, channels...)

	// Wait for subscription confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	eventCh := make(chan *Event, 100)

	go func() {
		defer close(eventCh)
		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				event, err := FromJSON([]byte(msg.Payload))
				if err != nil {
					logger.Error().Err(err).Msg("failed to parse event")
					continue
				}

				select {
				case eventCh <- event:
				default:
					// Channel full, drop event
					logger.Warn().
						Str("event_type", string(event.Type)).
						Msg("event channel full, dropping event")
				}
			}
		}
	}()

	return eventCh, nil
}

// SubscribeAll subscribes to all event types
func (r *RedisPubSub) SubscribeAll(ctx context.Context) (<-chan *Event, error) {
	pattern := channelPrefix + "*"
	pubsub := r.client.PSubscribe(ctx, pattern)

	// Wait for subscription confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	eventCh := make(chan *Event, 100)

	go func() {
		defer close(eventCh)
		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				event, err := FromJSON([]byte(msg.Payload))
				if err != nil {
					logger.Error().Err(err).Msg("failed to parse event")
					continue
				}

				select {
				case eventCh <- event:
				default:
					logger.Warn().
						Str("event_type", string(event.Type)).
						Msg("event channel full, dropping event")
				}
			}
		}
	}()

	return eventCh, nil
}

// Close closes all subscriptions
func (r *RedisPubSub) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, pubsub := range r.subscribers {
		pubsub.Close()
	}
	r.subscribers = make(map[string]*redis.PubSub)

	return nil
}

func (r *RedisPubSub) channelName(eventType EventType) string {
	return channelPrefix + string(eventType)
}

// PublishTaskEvent is a helper to publish task-related events
func (r *RedisPubSub) PublishTaskEvent(ctx context.Context, eventType EventType, taskID, taskType, priority string, extra map[string]interface{}) error {
	event := NewEvent(eventType, TaskEventData(taskID, taskType, priority, extra))
	return r.Publish(ctx, event)
}

// PublishWorkerEvent is a helper to publish worker-related events
func (r *RedisPubSub) PublishWorkerEvent(ctx context.Context, eventType EventType, workerID, state string, extra map[string]interface{}) error {
	event := NewEvent(eventType, WorkerEventData(workerID, state, extra))
	return r.Publish(ctx, event)
}
