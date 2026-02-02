// Example usage of the Task Queue Go client SDK
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maumercado/task-queue-go/pkg/client"
)

func main() {
	// Create client
	baseURL := getEnv("TASKQUEUE_URL", "http://localhost:8080")
	apiKey := os.Getenv("TASKQUEUE_API_KEY")

	opts := []client.Option{}
	if apiKey != "" {
		opts = append(opts, client.WithAPIKey(apiKey))
	}
	opts = append(opts, client.WithTimeout(30*time.Second))

	c, err := client.New(baseURL, opts...)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// Health check
	fmt.Println("=== Health Check ===")
	health, err := c.CheckHealth(ctx)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Printf("Status: %v, Redis: %v\n\n", *health.Status, *health.Redis)

	// Get queue statistics
	fmt.Println("=== Queue Statistics ===")
	stats, err := c.GetQueueStatistics(ctx)
	if err != nil {
		log.Fatalf("Failed to get queue stats: %v", err)
	}
	if stats.QueueDepths != nil {
		fmt.Printf("Queue depths - Critical: %d, High: %d, Normal: %d, Low: %d\n",
			safeInt(stats.QueueDepths.Critical),
			safeInt(stats.QueueDepths.High),
			safeInt(stats.QueueDepths.Normal),
			safeInt(stats.QueueDepths.Low))
	}
	fmt.Printf("Total pending: %d\n\n", safeInt(stats.TotalPending))

	// Create a task
	fmt.Println("=== Create Task ===")
	priority := 1
	maxRetries := 3
	timeout := 60
	task, err := c.SubmitTask(ctx, client.CreateTaskRequest{
		Type: "example",
		Payload: &map[string]interface{}{
			"message": "Hello from Go client!",
			"created": time.Now().Format(time.RFC3339),
		},
		Priority:   &priority,
		MaxRetries: &maxRetries,
		Timeout:    &timeout,
	})
	if err != nil {
		log.Fatalf("Failed to create task: %v", err)
	}
	fmt.Printf("Created task: %s\n", task.Id.String())
	fmt.Printf("State: %v\n", *task.State)
	fmt.Printf("Priority: %v\n\n", *task.Priority)

	// Get task status
	fmt.Println("=== Get Task Status ===")
	taskStatus, err := c.GetTaskByID(ctx, task.Id.String())
	if err != nil {
		log.Printf("Failed to get task: %v", err)
	} else {
		fmt.Printf("Task %s state: %v\n\n", taskStatus.Id.String(), *taskStatus.State)
	}

	// List workers
	fmt.Println("=== Workers ===")
	workers, err := c.ListAllWorkers(ctx)
	if err != nil {
		log.Printf("Failed to list workers: %v", err)
	} else {
		fmt.Printf("Active workers: %d\n", safeInt(workers.Count))
		if workers.Workers != nil {
			for _, w := range *workers.Workers {
				fmt.Printf("  - %s (state: %v, tasks: %d)\n",
					safeString(w.Id), *w.State, safeInt(w.ActiveTasks))
			}
		}
	}
	fmt.Println()

	// Connect to WebSocket for events
	fmt.Println("=== WebSocket Events ===")
	fmt.Println("Connecting to WebSocket...")
	if err := c.ConnectWebSocket(ctx); err != nil {
		log.Printf("Failed to connect WebSocket: %v", err)
	} else {
		fmt.Println("Connected! Listening for events...")

		// Handle shutdown gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Subscribe to task events
		c.SubscribeEvents(
			client.EventTaskSubmitted,
			client.EventTaskStarted,
			client.EventTaskCompleted,
			client.EventTaskFailed,
		)

		// Listen for events with timeout
		timeout := time.After(10 * time.Second)
		eventCount := 0

	eventLoop:
		for {
			select {
			case event, ok := <-c.Events():
				if !ok {
					fmt.Println("WebSocket closed")
					break eventLoop
				}
				fmt.Printf("Event: %s at %v\n", event.Type, event.Timestamp)
				if event.Data != nil {
					fmt.Printf("  Data: %v\n", event.Data)
				}
				eventCount++
				if eventCount >= 5 {
					fmt.Println("Received 5 events, stopping...")
					break eventLoop
				}
			case <-timeout:
				fmt.Println("Timeout waiting for events")
				break eventLoop
			case <-sigChan:
				fmt.Println("Received shutdown signal")
				break eventLoop
			}
		}

		c.CloseWebSocket()
		fmt.Println("WebSocket closed")
	}

	// Cancel the task we created (if still pending)
	fmt.Println("\n=== Cleanup ===")
	_, err = c.CancelTaskByID(ctx, task.Id.String())
	if err != nil {
		fmt.Printf("Could not cancel task (may already be processed): %v\n", err)
	} else {
		fmt.Printf("Cancelled task %s\n", task.Id.String())
	}

	fmt.Println("\nDone!")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func safeInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func safeString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
