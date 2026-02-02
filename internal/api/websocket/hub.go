package websocket

import (
	"context"
	"sync"

	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/metrics"
)

// Hub manages WebSocket clients and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *events.Event
	register   chan *Client
	unregister chan *Client
	publisher  *events.RedisPubSub
	mu         sync.RWMutex
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewHub creates a new WebSocket hub
func NewHub(publisher *events.RedisPubSub) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *events.Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		publisher:  publisher,
		stopCh:     make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	// Subscribe to all events from Redis
	eventCh, err := h.publisher.SubscribeAll(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to subscribe to events")
		return
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-h.stopCh:
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				h.broadcast <- event
			}
		}
	}()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		for {
			select {
			case <-ctx.Done():
				h.closeAllClients()
				return
			case <-h.stopCh:
				h.closeAllClients()
				return
			case client := <-h.register:
				h.mu.Lock()
				h.clients[client] = true
				h.mu.Unlock()
				metrics.SetWebSocketConnections(float64(h.ClientCount()))
				logger.Debug().Str("client_id", client.ID).Msg("client registered")

			case client := <-h.unregister:
				h.mu.Lock()
				if _, ok := h.clients[client]; ok {
					delete(h.clients, client)
					close(client.send)
				}
				h.mu.Unlock()
				metrics.SetWebSocketConnections(float64(h.ClientCount()))
				logger.Debug().Str("client_id", client.ID).Msg("client unregistered")

			case event := <-h.broadcast:
				h.broadcastEvent(event)
			}
		}
	}()

	logger.Info().Msg("WebSocket hub started")
}

// Stop stops the hub
func (h *Hub) Stop() {
	close(h.stopCh)
	h.wg.Wait()
	logger.Info().Msg("WebSocket hub stopped")
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister unregisters a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends an event to all connected clients
func (h *Hub) Broadcast(event *events.Event) {
	select {
	case h.broadcast <- event:
	default:
		logger.Warn().Msg("broadcast channel full, dropping event")
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) broadcastEvent(event *events.Event) {
	data, err := event.ToJSON()
	if err != nil {
		logger.Error().Err(err).Msg("failed to serialize event for broadcast")
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// Check if client is subscribed to this event type
		if !client.IsSubscribed(event.Type) {
			continue
		}

		select {
		case client.send <- data:
			metrics.RecordWebSocketMessage(string(event.Type))
		default:
			// Client buffer full, mark for removal
			go func(c *Client) {
				h.unregister <- c
			}(client)
		}
	}
}

func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}
}
