package websocket

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/logger"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Send buffer size
	sendBufferSize = 256
)

// Client represents a WebSocket client connection
type Client struct {
	ID            string
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[events.EventType]bool
	subMu         sync.RWMutex
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		ID:            uuid.New().String()[:8],
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, sendBufferSize),
		subscriptions: make(map[events.EventType]bool),
	}
}

// Subscribe subscribes the client to an event type
func (c *Client) Subscribe(eventType events.EventType) {
	c.subMu.Lock()
	c.subscriptions[eventType] = true
	c.subMu.Unlock()
}

// Unsubscribe unsubscribes the client from an event type
func (c *Client) Unsubscribe(eventType events.EventType) {
	c.subMu.Lock()
	delete(c.subscriptions, eventType)
	c.subMu.Unlock()
}

// SubscribeAll subscribes the client to all events
func (c *Client) SubscribeAll() {
	c.subMu.Lock()
	c.subscriptions[events.EventTaskSubmitted] = true
	c.subscriptions[events.EventTaskStarted] = true
	c.subscriptions[events.EventTaskCompleted] = true
	c.subscriptions[events.EventTaskFailed] = true
	c.subscriptions[events.EventTaskRetrying] = true
	c.subscriptions[events.EventWorkerJoined] = true
	c.subscriptions[events.EventWorkerLeft] = true
	c.subscriptions[events.EventWorkerPaused] = true
	c.subscriptions[events.EventWorkerResumed] = true
	c.subscriptions[events.EventQueueDepth] = true
	c.subscriptions[events.EventSystemMetrics] = true
	c.subMu.Unlock()
}

// IsSubscribed checks if the client is subscribed to an event type
func (c *Client) IsSubscribed(eventType events.EventType) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()

	// If no subscriptions, receive all events
	if len(c.subscriptions) == 0 {
		return true
	}

	return c.subscriptions[eventType]
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error().Err(err).Str("client_id", c.ID).Msg("WebSocket read error")
			}
			break
		}

		// Handle client messages (e.g., subscription commands)
		c.handleMessage(message)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued messages to current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ClientMessage represents a message from the client
type ClientMessage struct {
	Action     string   `json:"action"`
	EventTypes []string `json:"event_types,omitempty"`
}

func (c *Client) handleMessage(message []byte) {
	// Parse message and handle subscription commands
	// For now, we just log it
	logger.Debug().
		Str("client_id", c.ID).
		Str("message", string(message)).
		Msg("received client message")
}
