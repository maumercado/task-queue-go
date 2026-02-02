package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maumercado/task-queue-go/internal/task"
)

func TestNewExecutor(t *testing.T) {
	// With nil handlers and policy
	executor := NewExecutor(nil, nil)
	assert.NotNil(t, executor)
	assert.NotNil(t, executor.handlers)
	assert.NotNil(t, executor.retryPolicy)

	// With custom handlers
	handlers := map[string]TaskHandler{
		"test": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			return nil, nil
		},
	}
	executor = NewExecutor(handlers, nil)
	assert.Len(t, executor.handlers, 1)
}

func TestExecutor_RegisterHandler(t *testing.T) {
	executor := NewExecutor(nil, nil)

	handler := func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
		return map[string]interface{}{"result": "ok"}, nil
	}

	executor.RegisterHandler("my-type", handler)
	assert.True(t, executor.HasHandler("my-type"))
	assert.False(t, executor.HasHandler("other-type"))
}

func TestExecutor_HandlerTypes(t *testing.T) {
	handlers := map[string]TaskHandler{
		"email":   func(ctx context.Context, t *task.Task) (map[string]interface{}, error) { return nil, nil },
		"compute": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) { return nil, nil },
		"notify":  func(ctx context.Context, t *task.Task) (map[string]interface{}, error) { return nil, nil },
	}

	executor := NewExecutor(handlers, nil)
	types := executor.HandlerTypes()

	assert.Len(t, types, 3)
	assert.Contains(t, types, "email")
	assert.Contains(t, types, "compute")
	assert.Contains(t, types, "notify")
}

func TestExecutor_Execute_Success(t *testing.T) {
	handlers := map[string]TaskHandler{
		"test": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			return map[string]interface{}{
				"echoed": t.Payload,
			}, nil
		},
	}

	executor := NewExecutor(handlers, nil)
	testTask := task.New("test", map[string]interface{}{"key": "value"}, task.PriorityNormal)

	result, err := executor.Execute(context.Background(), testTask)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, testTask.Payload, result["echoed"])
}

func TestExecutor_Execute_Error(t *testing.T) {
	expectedErr := errors.New("task failed")
	handlers := map[string]TaskHandler{
		"fail": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			return nil, expectedErr
		},
	}

	executor := NewExecutor(handlers, nil)
	testTask := task.New("fail", nil, task.PriorityNormal)

	result, err := executor.Execute(context.Background(), testTask)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)
}

func TestExecutor_Execute_HandlerNotFound(t *testing.T) {
	executor := NewExecutor(nil, nil)
	testTask := task.New("unknown", nil, task.PriorityNormal)

	result, err := executor.Execute(context.Background(), testTask)

	assert.Equal(t, ErrHandlerNotFound, err)
	assert.Nil(t, result)
}

func TestExecutor_Execute_Timeout(t *testing.T) {
	handlers := map[string]TaskHandler{
		"slow": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			select {
			case <-time.After(5 * time.Second):
				return map[string]interface{}{"done": true}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	executor := NewExecutor(handlers, nil)
	testTask := task.New("slow", nil, task.PriorityNormal)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := executor.Execute(ctx, testTask)

	assert.Equal(t, ErrTaskTimeout, err)
	assert.Nil(t, result)
}

func TestExecutor_Execute_Canceled(t *testing.T) {
	handlers := map[string]TaskHandler{
		"slow": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			select {
			case <-time.After(5 * time.Second):
				return map[string]interface{}{"done": true}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	executor := NewExecutor(handlers, nil)
	testTask := task.New("slow", nil, task.PriorityNormal)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := executor.Execute(ctx, testTask)

	assert.Equal(t, ErrTaskCanceled, err)
	assert.Nil(t, result)
}

func TestExecutor_Execute_Panic(t *testing.T) {
	handlers := map[string]TaskHandler{
		"panic": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			panic("something went wrong!")
		},
	}

	executor := NewExecutor(handlers, nil)
	testTask := task.New("panic", nil, task.PriorityNormal)

	result, err := executor.Execute(context.Background(), testTask)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler panicked")
	assert.Nil(t, result)
}

func TestExecutor_HasHandler(t *testing.T) {
	handlers := map[string]TaskHandler{
		"exists": func(ctx context.Context, t *task.Task) (map[string]interface{}, error) {
			return nil, nil
		},
	}

	executor := NewExecutor(handlers, nil)

	assert.True(t, executor.HasHandler("exists"))
	assert.False(t, executor.HasHandler("not-exists"))
}

func TestErrorDefinitions(t *testing.T) {
	assert.Equal(t, "handler not found for task type", ErrHandlerNotFound.Error())
	assert.Equal(t, "task execution timed out", ErrTaskTimeout.Error())
	assert.Equal(t, "task execution canceled", ErrTaskCanceled.Error())
}
