package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/maumercado/task-queue-go/internal/api/handlers"
	apiMiddleware "github.com/maumercado/task-queue-go/internal/api/middleware"
	"github.com/maumercado/task-queue-go/internal/api/websocket"
	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/queue"
)

// Server represents the HTTP server
type Server struct {
	router       *chi.Mux
	queue        *queue.RedisQueue
	dlq          *queue.DLQ
	config       *config.Config
	taskHandler  *handlers.TaskHandler
	adminHandler *handlers.AdminHandler
	wsHub        *websocket.Hub
	wsHandler    *websocket.Handler
	publisher    *events.RedisPubSub
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, q *queue.RedisQueue, dlq *queue.DLQ, publisher *events.RedisPubSub) *Server {
	wsHub := websocket.NewHub(publisher)

	// Create schedule task function
	scheduleTask := queue.ScheduleTaskFunc(q.Client())

	s := &Server{
		router:       chi.NewRouter(),
		queue:        q,
		dlq:          dlq,
		config:       cfg,
		taskHandler:  handlers.NewTaskHandler(q, scheduleTask, cfg.Queue.MaxQueueSize),
		adminHandler: handlers.NewAdminHandler(q, dlq),
		wsHub:        wsHub,
		wsHandler:    websocket.NewHandler(wsHub),
		publisher:    publisher,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// Logging
	s.router.Use(apiMiddleware.RequestLogger())

	// Recoverer
	s.router.Use(middleware.Recoverer)

	// Heartbeat endpoint for load balancers
	s.router.Use(middleware.Heartbeat("/health"))
}

func (s *Server) setupRoutes() {
	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Content type for API routes
		r.Use(middleware.AllowContentType("application/json"))

		// Rate limiting for API routes
		if s.config.Queue.RateLimitRPS > 0 {
			r.Use(apiMiddleware.ClientRateLimit(s.config.Queue.RateLimitRPS))
		}

		// Task routes
		r.Route("/tasks", func(r chi.Router) {
			r.Post("/", s.taskHandler.Create)
			r.Get("/{taskID}", s.taskHandler.Get)
			r.Delete("/{taskID}", s.taskHandler.Cancel)
			r.Get("/", s.taskHandler.List)
		})
	})

	// Admin routes
	s.router.Route("/admin", func(r chi.Router) {
		r.Use(middleware.AllowContentType("application/json"))

		r.Get("/health", s.adminHandler.HealthCheck)

		// Worker management
		r.Get("/workers", s.adminHandler.ListWorkers)
		r.Get("/workers/{workerID}", s.adminHandler.GetWorker)
		r.Post("/workers/{workerID}/pause", s.adminHandler.PauseWorker)
		r.Post("/workers/{workerID}/resume", s.adminHandler.ResumeWorker)

		// Queue management
		r.Get("/queues", s.adminHandler.GetQueues)
		r.Delete("/queues/{priority}", s.adminHandler.PurgeQueue)

		// Task management
		r.Post("/tasks/{taskID}/retry", s.adminHandler.RetryTask)

		// DLQ management
		r.Get("/dlq", s.adminHandler.ListDLQ)
		r.Post("/dlq/retry", s.adminHandler.RetryDLQ)
		r.Delete("/dlq", s.adminHandler.ClearDLQ)
	})

	// WebSocket endpoint
	s.router.Get("/ws", s.wsHandler.ServeWS)

	// Metrics endpoint
	if s.config.Metrics.Enabled {
		s.router.Handle(s.config.Metrics.Path, promhttp.Handler())
	}
}

// Start starts the WebSocket hub
func (s *Server) Start(ctx context.Context) {
	go s.wsHub.Run(ctx)
}

// Stop stops the WebSocket hub
func (s *Server) Stop() {
	s.wsHub.Stop()
}

// Router returns the chi router
func (s *Server) Router() *chi.Mux {
	return s.router
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Publisher returns the event publisher
func (s *Server) Publisher() *events.RedisPubSub {
	return s.publisher
}
