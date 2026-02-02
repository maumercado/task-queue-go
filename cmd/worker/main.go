package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
	"github.com/maumercado/task-queue-go/internal/worker"
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
	log.Info().Msg("Starting worker...")

	// Create Redis queue
	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Queue)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Redis queue")
	}
	defer redisQueue.Close()

	// Create DLQ
	dlq := queue.NewDLQ(redisQueue.Client())

	// Register task handlers
	handlers := map[string]worker.TaskHandler{
		"echo":    echoHandler,
		"sleep":   sleepHandler,
		"compute": computeHandler,
		"fail":    failHandler,
	}

	// Create worker pool
	pool := worker.NewPool(&cfg.Worker, redisQueue, dlq, handlers)

	// Start worker pool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pool.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start worker pool")
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down worker...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Worker.ShutdownTimeout)
	defer shutdownCancel()

	if err := pool.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Worker shutdown error")
	}

	log.Info().Msg("Worker stopped")
}

// Example task handlers

func echoHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	logger.Info().
		Str("task_id", t.ID).
		Interface("payload", t.Payload).
		Msg("Echo handler processing task")

	return map[string]interface{}{
		"echoed": t.Payload,
	}, nil
}

func sleepHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	duration := 1 * time.Second
	if d, ok := t.Payload["duration"].(float64); ok {
		duration = time.Duration(d) * time.Millisecond
	}

	logger.Info().
		Str("task_id", t.ID).
		Dur("duration", duration).
		Msg("Sleep handler processing task")

	select {
	case <-time.After(duration):
		return map[string]interface{}{
			"slept_for": duration.String(),
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func computeHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	// Simulate some computation
	iterations := 1000000
	if i, ok := t.Payload["iterations"].(float64); ok {
		iterations = int(i)
	}

	logger.Info().
		Str("task_id", t.ID).
		Int("iterations", iterations).
		Msg("Compute handler processing task")

	sum := 0
	for i := 0; i < iterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			sum += i
		}
	}

	return map[string]interface{}{
		"result": sum,
	}, nil
}

func failHandler(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
	logger.Info().
		Str("task_id", t.ID).
		Msg("Fail handler processing task")

	return nil, fmt.Errorf("intentional failure for testing")
}
