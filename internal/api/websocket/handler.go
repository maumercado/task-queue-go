package websocket

import (
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/maumercado/task-queue-go/internal/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, implement proper origin checking
		return true
	},
}

// Handler handles WebSocket connections
type Handler struct {
	hub *Hub
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

// ServeWS handles WebSocket upgrade requests
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to upgrade WebSocket connection")
		return
	}

	client := NewClient(h.hub, conn)

	// Subscribe to all events by default
	client.SubscribeAll()

	h.hub.Register(client)

	// Start pumps in goroutines
	go client.WritePump()
	go client.ReadPump()

	logger.Info().
		Str("client_id", client.ID).
		Str("remote_addr", r.RemoteAddr).
		Msg("WebSocket client connected")
}
