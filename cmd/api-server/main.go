package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maumercado/task-queue-go/internal/api"
	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger.Init(cfg.LogLevel, os.Getenv("ENV") != "production")

	log := logger.Get()
	log.Info().Msg("Starting API server...")

	// Create Redis queue
	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Queue)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Redis queue")
	}
	defer func() {
		if err := redisQueue.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close Redis queue")
		}
	}()

	// Create DLQ
	dlq := queue.NewDLQ(redisQueue.Client())

	// Create event publisher
	publisher := events.NewRedisPubSub(redisQueue.Client())
	defer func() {
		if err := publisher.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close event publisher")
		}
	}()

	// Create and start scheduler for scheduled tasks
	scheduler := queue.NewScheduler(redisQueue.Client(), redisQueue)

	// Create server
	server := api.NewServer(cfg, redisQueue, dlq, publisher)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      server,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start WebSocket hub
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Start(ctx)

	// Start scheduler
	scheduler.Start(ctx)

	// Start HTTP server
	go func() {
		log.Info().
			Str("addr", httpServer.Addr).
			Msg("HTTP server listening")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop scheduler
	scheduler.Stop()

	// Stop WebSocket hub
	server.Stop()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}

	log.Info().Msg("Server stopped")
}
