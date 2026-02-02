package worker

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/task"
)

// TaskHandler is a function that processes a task
type TaskHandler func(ctx context.Context, t *task.Task) (map[string]interface{}, error)

// Executor executes tasks using registered handlers
type Executor struct {
	handlers    map[string]TaskHandler
	retryPolicy *task.RetryPolicy
}

// NewExecutor creates a new task executor
func NewExecutor(handlers map[string]TaskHandler, retryPolicy *task.RetryPolicy) *Executor {
	if handlers == nil {
		handlers = make(map[string]TaskHandler)
	}
	if retryPolicy == nil {
		retryPolicy = task.DefaultRetryPolicy()
	}
	return &Executor{
		handlers:    handlers,
		retryPolicy: retryPolicy,
	}
}

// RegisterHandler registers a handler for a task type
func (e *Executor) RegisterHandler(taskType string, handler TaskHandler) {
	e.handlers[taskType] = handler
}

// Execute runs the appropriate handler for a task
func (e *Executor) Execute(ctx context.Context, t *task.Task) (result map[string]interface{}, err error) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			logger.Error().
				Str("task_id", t.ID).
				Str("type", t.Type).
				Interface("panic", r).
				Str("stack", string(stack)).
				Msg("task handler panicked")
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()

	handler, ok := e.handlers[t.Type]
	if !ok {
		return nil, ErrHandlerNotFound
	}

	log := logger.WithTask(t.ID)
	log.Debug().
		Str("type", t.Type).
		Int("attempt", t.Attempts).
		Msg("executing task")

	start := time.Now()
	result, err = handler(ctx, t)
	duration := time.Since(start)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn().Dur("duration", duration).Msg("task timed out")
			return nil, ErrTaskTimeout
		}
		if errors.Is(err, context.Canceled) {
			log.Warn().Dur("duration", duration).Msg("task canceled")
			return nil, ErrTaskCanceled
		}
		log.Error().Err(err).Dur("duration", duration).Msg("task failed")
		return nil, err
	}

	log.Debug().Dur("duration", duration).Msg("task executed successfully")
	return result, nil
}

// HasHandler checks if a handler exists for a task type
func (e *Executor) HasHandler(taskType string) bool {
	_, ok := e.handlers[taskType]
	return ok
}

// HandlerTypes returns all registered handler types
func (e *Executor) HandlerTypes() []string {
	types := make([]string, 0, len(e.handlers))
	for t := range e.handlers {
		types = append(types, t)
	}
	return types
}

// Error definitions
var (
	ErrHandlerNotFound = errors.New("handler not found for task type")
	ErrTaskTimeout     = errors.New("task execution timed out")
	ErrTaskCanceled    = errors.New("task execution canceled")
)
