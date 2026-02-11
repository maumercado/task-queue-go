package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// EventType represents the type of WebSocket event.
type EventType string

const (
	EventTaskSubmitted EventType = "task.submitted"
	EventTaskStarted   EventType = "task.started"
	EventTaskCompleted EventType = "task.completed"
	EventTaskFailed    EventType = "task.failed"
	EventTaskRetrying  EventType = "task.retrying"
	EventWorkerJoined  EventType = "worker.joined"
	EventWorkerLeft    EventType = "worker.left"
	EventWorkerPaused  EventType = "worker.paused"
	EventWorkerResumed EventType = "worker.resumed"
	EventQueueDepth    EventType = "queue.depth"
	EventSystemMetrics EventType = "system.metrics"
)

// Event represents a WebSocket event from the server.
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// WebSocketClient handles WebSocket connections for real-time events.
type WebSocketClient struct {
	conn      *websocket.Conn
	baseURL   string
	events    chan *Event
	done      chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	connected bool
	apiKey    string
}

// newWebSocketClient creates a new WebSocket client.
func newWebSocketClient(baseURL, apiKey string) *WebSocketClient {
	return &WebSocketClient{
		baseURL: baseURL,
		events:  make(chan *Event, 100),
		done:    make(chan struct{}),
		apiKey:  apiKey,
	}
}

// Connect establishes a WebSocket connection to the server.
func (ws *WebSocketClient) Connect(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.connected {
		return nil
	}

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(ws.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/ws"

	// Set up headers
	headers := make(map[string][]string)
	if ws.apiKey != "" {
		headers["Authorization"] = []string{"Bearer " + ws.apiKey}
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	ws.conn = conn
	ws.connected = true
	ws.done = make(chan struct{})

	// Start reading messages
	go ws.readLoop()

	return nil
}

// readLoop reads messages from the WebSocket connection.
func (ws *WebSocketClient) readLoop() {
	defer func() {
		ws.mu.Lock()
		ws.connected = false
		ws.mu.Unlock()
		close(ws.events)
	}()

	for {
		select {
		case <-ws.done:
			return
		default:
			_, message, err := ws.conn.ReadMessage()
			if err != nil {
				// Expected close errors are ignored; unexpected ones could be logged
				// by the caller via the events channel closing.
				return
			}

			var event Event
			if err := json.Unmarshal(message, &event); err != nil {
				continue // Skip malformed messages
			}

			select {
			case ws.events <- &event:
			case <-ws.done:
				return
			default:
				// Channel full, drop oldest event
				select {
				case <-ws.events:
				default:
				}
				ws.events <- &event
			}
		}
	}
}

// Events returns a channel that receives events from the server.
func (ws *WebSocketClient) Events() <-chan *Event {
	return ws.events
}

// Close closes the WebSocket connection.
func (ws *WebSocketClient) Close() error {
	var err error
	ws.closeOnce.Do(func() {
		close(ws.done)
		ws.mu.Lock()
		defer ws.mu.Unlock()
		if ws.conn != nil {
			err = ws.conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			_ = ws.conn.Close()
		}
	})
	return err
}

// IsConnected returns whether the WebSocket is currently connected.
func (ws *WebSocketClient) IsConnected() bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.connected
}

// Subscribe sends a subscription request for specific event types.
// This is a no-op if the server doesn't support subscription filtering.
func (ws *WebSocketClient) Subscribe(eventTypes ...EventType) error {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if !ws.connected || ws.conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"action": "subscribe",
		"events": eventTypes,
	}

	return ws.conn.WriteJSON(msg)
}

// Unsubscribe sends an unsubscription request for specific event types.
func (ws *WebSocketClient) Unsubscribe(eventTypes ...EventType) error {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if !ws.connected || ws.conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"action": "unsubscribe",
		"events": eventTypes,
	}

	return ws.conn.WriteJSON(msg)
}
