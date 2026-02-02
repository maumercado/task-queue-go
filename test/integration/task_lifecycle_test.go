//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maumercado/task-queue-go/internal/api"
	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/events"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
	"github.com/maumercado/task-queue-go/internal/worker"
)

func init() {
	logger.Init("error", false)
}

func setupTestServer(t *testing.T) (*api.Server, *queue.RedisQueue, func()) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           15, // Use a separate DB for tests
			PoolSize:     10,
			MinIdleConns: 2,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Queue: config.QueueConfig{
			StreamPrefix:        "test_tasks",
			ConsumerGroup:       "test_workers",
			MaxQueueSize:        10000,
			BlockTimeout:        1 * time.Second,
			ClaimMinIdle:        5 * time.Second,
			RecoveryInterval:    5 * time.Second,
			RetryMaxAttempts:    3,
			RetryInitialBackoff: 100 * time.Millisecond,
			RetryMaxBackoff:     1 * time.Second,
			RetryBackoffFactor:  2.0,
		},
		Server: config.ServerConfig{
			Host:         "localhost",
			Port:         8080,
			AdminPort:    8081,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Metrics: config.MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
	}

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Queue)
	require.NoError(t, err)

	dlq := queue.NewDLQ(redisQueue.Client())
	publisher := events.NewRedisPubSub(redisQueue.Client())
	server := api.NewServer(cfg, redisQueue, dlq, publisher)

	cleanup := func() {
		// Clean up test data
		ctx := context.Background()
		redisQueue.Client().FlushDB(ctx)
		redisQueue.Close()
		publisher.Close()
	}

	return server, redisQueue, cleanup
}

func TestTaskLifecycle_CreateAndGet(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a task
	createReq := task.CreateTaskRequest{
		Type:       "test-task",
		Payload:    map[string]interface{}{"key": "value"},
		Priority:   2, // High
		MaxRetries: 5,
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createResp task.TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &createResp)
	require.NoError(t, err)

	assert.NotEmpty(t, createResp.ID)
	assert.Equal(t, "test-task", createResp.Type)
	assert.Equal(t, "high", createResp.Priority)
	assert.Equal(t, "pending", createResp.State)

	// Get the task
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+createResp.ID, nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var getResp task.TaskResponse
	err = json.Unmarshal(w.Body.Bytes(), &getResp)
	require.NoError(t, err)

	assert.Equal(t, createResp.ID, getResp.ID)
	assert.Equal(t, createResp.Type, getResp.Type)
}

func TestTaskLifecycle_Cancel(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a task
	createReq := task.CreateTaskRequest{
		Type:    "cancellable-task",
		Payload: nil,
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp task.TaskResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Cancel the task
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+createResp.ID, nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var cancelResp task.TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &cancelResp)
	require.NoError(t, err)

	assert.Equal(t, "canceled", cancelResp.State)
}

func TestTaskLifecycle_ListQueues(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Create multiple tasks with different priorities
	priorities := []int{0, 1, 2, 3} // low, normal, high, critical
	for _, p := range priorities {
		createReq := task.CreateTaskRequest{
			Type:     fmt.Sprintf("task-priority-%d", p),
			Payload:  nil,
			Priority: p,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List tasks/queue depths
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var listResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err)

	assert.Contains(t, listResp, "queue_depths")
	assert.Contains(t, listResp, "total_pending")
}

func TestTaskLifecycle_GetNotFound(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/nonexistent-id", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAdminEndpoints_Health(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/admin/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp["status"])
	assert.Equal(t, "connected", resp["redis"])
}

func TestAdminEndpoints_ListWorkers(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/admin/workers", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "workers")
	assert.Contains(t, resp, "count")
}

func TestAdminEndpoints_GetQueues(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "queues")
	assert.Contains(t, resp, "total_depth")
}

func TestAdminEndpoints_DLQ(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// List DLQ
	req := httptest.NewRequest(http.MethodGet, "/admin/dlq", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp, "entries")
	assert.Contains(t, resp, "size")
}

func TestWorkerPool_StartStop(t *testing.T) {
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Addr:         "localhost:6379",
			Password:     "",
			DB:           15,
			PoolSize:     10,
			MinIdleConns: 2,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Queue: config.QueueConfig{
			StreamPrefix:    "test_tasks",
			ConsumerGroup:   "test_workers",
			BlockTimeout:    1 * time.Second,
			ClaimMinIdle:    5 * time.Second,
			RecoveryInterval: 5 * time.Second,
		},
		Worker: config.WorkerConfig{
			ID:                "test-worker",
			Concurrency:       2,
			HeartbeatInterval: 1 * time.Second,
			HeartbeatTimeout:  3 * time.Second,
			ShutdownTimeout:   5 * time.Second,
		},
	}

	redisQueue, err := queue.NewRedisQueue(&cfg.Redis, &cfg.Queue)
	require.NoError(t, err)
	defer redisQueue.Close()

	dlq := queue.NewDLQ(redisQueue.Client())

	handlers := map[string]worker.TaskHandler{
		"test": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			return map[string]interface{}{"result": "ok"}, nil
		},
	}

	pool := worker.NewPool(&cfg.Worker, redisQueue, dlq, handlers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pool.Start(ctx)
	require.NoError(t, err)

	assert.Equal(t, worker.StateBusy, pool.State())
	assert.Equal(t, "test-worker", pool.ID())

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the pool
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = pool.Stop(stopCtx)
	require.NoError(t, err)

	// Clean up
	redisQueue.Client().FlushDB(context.Background())
}
