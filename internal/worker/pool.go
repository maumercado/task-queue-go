package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/maumercado/task-queue-go/internal/config"
	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/queue"
	"github.com/maumercado/task-queue-go/internal/task"
)

// State represents the worker pool's current operational state
type State int

const (
	StateIdle         State = iota // Not processing, waiting to start
	StateBusy                      // Actively processing tasks
	StatePaused                    // Temporarily stopped, can resume
	StateShuttingDown              // Gracefully stopping
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateBusy:
		return "busy"
	case StatePaused:
		return "paused"
	case StateShuttingDown:
		return "shutting_down"
	default:
		return "unknown"
	}
}

// Pool manages a pool of concurrent worker goroutines.
// Coordinates task fetching, execution, retry logic, and graceful shutdown.
type Pool struct {
	id             string            // Unique identifier for this worker pool
	queue          *queue.RedisQueue // Queue to fetch tasks from
	dlq            *queue.DLQ        // Dead letter queue for failed tasks
	executor       *Executor         // Executes task handlers
	heartbeat      *Heartbeat        // Sends heartbeats to indicate liveness
	config         *config.WorkerConfig
	state          State
	stateMu        sync.RWMutex
	currentTasks   sync.Map       // Currently running tasks (taskID -> *runningTask)
	wg             sync.WaitGroup // Wait group for graceful shutdown
	stopCh         chan struct{}  // Signal to stop all workers
	pauseCh        chan struct{}  // Signal workers are paused
	resumeCh       chan struct{}  // Signal to resume workers
	concurrencySem chan struct{}  // Semaphore to limit concurrent task execution
}

// runningTask tracks a task currently being processed
type runningTask struct {
	task      *task.Task
	messageID string
	cancel    context.CancelFunc
	startedAt time.Time
}

// NewPool creates a new worker pool with the given configuration
func NewPool(cfg *config.WorkerConfig, q *queue.RedisQueue, dlq *queue.DLQ, handlers map[string]TaskHandler) *Pool {
	// Generate worker ID if not provided
	workerID := cfg.ID
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}

	p := &Pool{
		id:             workerID,
		queue:          q,
		dlq:            dlq,
		config:         cfg,
		state:          StateIdle,
		stopCh:         make(chan struct{}),
		pauseCh:        make(chan struct{}),
		resumeCh:       make(chan struct{}),
		concurrencySem: make(chan struct{}, cfg.Concurrency), // Buffer = max concurrent tasks
	}

	p.executor = NewExecutor(handlers, task.DefaultRetryPolicy())
	p.heartbeat = NewHeartbeat(q.Client(), workerID, cfg.HeartbeatInterval, cfg.HeartbeatTimeout)

	return p
}

// Start begins the worker pool, spawning worker goroutines
func (p *Pool) Start(ctx context.Context) error {
	p.stateMu.Lock()
	p.state = StateBusy
	p.stateMu.Unlock()

	// Start heartbeat to register with Redis
	p.heartbeat.Start(ctx)

	// Spawn worker goroutines (one per concurrency slot)
	for i := 0; i < p.config.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Spawn recovery goroutine to reclaim orphaned tasks
	p.wg.Add(1)
	go p.recoveryLoop(ctx)

	logger.Info().
		Str("worker_id", p.id).
		Int("concurrency", p.config.Concurrency).
		Msg("worker pool started")

	return nil
}

// Stop gracefully stops the worker pool, waiting for in-flight tasks
func (p *Pool) Stop(ctx context.Context) error {
	p.stateMu.Lock()
	p.state = StateShuttingDown
	p.stateMu.Unlock()

	close(p.stopCh) // Signal all workers to stop

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info().Str("worker_id", p.id).Msg("worker pool stopped gracefully")
	case <-time.After(p.config.ShutdownTimeout):
		logger.Warn().Str("worker_id", p.id).Msg("worker pool shutdown timed out")
	case <-ctx.Done():
		logger.Warn().Str("worker_id", p.id).Msg("worker pool shutdown canceled")
	}

	p.heartbeat.Stop()

	return nil
}

// Pause temporarily stops workers from fetching new tasks
func (p *Pool) Pause() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.state == StateBusy {
		p.state = StatePaused
		close(p.pauseCh)
		p.pauseCh = make(chan struct{})
		logger.Info().Str("worker_id", p.id).Msg("worker pool paused")
	}
}

// Resume continues task processing after a pause
func (p *Pool) Resume() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if p.state == StatePaused {
		p.state = StateBusy
		close(p.resumeCh)
		p.resumeCh = make(chan struct{})
		logger.Info().Str("worker_id", p.id).Msg("worker pool resumed")
	}
}

// State returns the current worker pool state
func (p *Pool) State() State {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.state
}

// ID returns the worker pool's unique identifier
func (p *Pool) ID() string {
	return p.id
}

// ActiveTasks returns the count of currently running tasks
func (p *Pool) ActiveTasks() int {
	count := 0
	p.currentTasks.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// worker is the main loop for each worker goroutine
func (p *Pool) worker(ctx context.Context, workerNum int) {
	defer p.wg.Done()

	log := logger.WithWorker(p.id)
	log.Info().Int("worker_num", workerNum).Msg("worker started")

	for {
		// Check for shutdown signal
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
		}

		// Block if paused locally, wait for resume
		if p.State() == StatePaused {
			select {
			case <-p.resumeCh:
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}

		// Check if paused via admin API (Redis flag)
		if paused, _ := IsWorkerPaused(ctx, p.queue.Client(), p.id); paused {
			// Wait a bit before checking again
			select {
			case <-time.After(1 * time.Second):
				continue
			case <-p.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}

		// Acquire semaphore slot (limits concurrency)
		select {
		case p.concurrencySem <- struct{}{}:
		case <-p.stopCh:
			return
		case <-ctx.Done():
			return
		}

		// Fetch and execute one task
		if err := p.processNextTask(ctx); err != nil {
			log.Error().Err(err).Msg("error processing task")
		}

		// Release semaphore slot
		<-p.concurrencySem
	}
}

// processNextTask fetches and executes a single task
func (p *Pool) processNextTask(ctx context.Context) error {
	// Block waiting for next available task
	t, messageID, err := p.queue.DequeueBlocking(ctx, p.id)
	if err != nil {
		return fmt.Errorf("failed to dequeue: %w", err)
	}

	if t == nil {
		return nil // No task available (timeout)
	}

	// Create timeout context for this task's execution
	taskCtx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()

	// Track this task as running
	rt := &runningTask{
		task:      t,
		messageID: messageID,
		cancel:    cancel,
		startedAt: time.Now(),
	}
	p.currentTasks.Store(t.ID, rt)
	defer p.currentTasks.Delete(t.ID)

	// Transition task to running state
	sm := task.NewStateMachine(t)
	if err := sm.Start(p.id); err != nil {
		logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to start task")
		return err
	}
	if err := p.queue.UpdateTask(ctx, t); err != nil {
		logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to update task state")
	}

	// Execute the task handler
	result, execErr := p.executor.Execute(taskCtx, t)

	// Handle success or failure
	if execErr != nil {
		p.handleTaskFailure(ctx, t, messageID, execErr)
		return nil
	}

	return p.handleTaskSuccess(ctx, t, messageID, result)
}

// handleTaskSuccess marks task as completed and acknowledges the message
func (p *Pool) handleTaskSuccess(ctx context.Context, t *task.Task, messageID string, result map[string]interface{}) error {
	sm := task.NewStateMachine(t)
	if err := sm.Complete(result); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	if err := p.queue.UpdateTask(ctx, t); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Remove from stream's pending list
	if err := p.queue.Acknowledge(ctx, t, messageID); err != nil {
		return fmt.Errorf("failed to acknowledge: %w", err)
	}

	logger.Info().
		Str("task_id", t.ID).
		Str("type", t.Type).
		Int("attempts", t.Attempts).
		Msg("task completed")

	return nil
}

// handleTaskFailure handles retry logic or moves to DLQ
func (p *Pool) handleTaskFailure(ctx context.Context, t *task.Task, messageID string, execErr error) {
	log := logger.WithTask(t.ID)
	log.Error().Err(execErr).Msg("task execution failed")

	sm := task.NewStateMachine(t)

	if t.CanRetry() {
		// Schedule for retry
		if err := sm.Retry(); err != nil {
			log.Error().Err(err).Msg("failed to transition to retry state")
		}
		t.Error = execErr.Error()
		if err := p.queue.UpdateTask(ctx, t); err != nil {
			log.Error().Err(err).Msg("failed to update task")
		}

		// Put back in queue for another attempt
		retryer := task.NewRetryer(task.DefaultRetryPolicy())
		retryer.PrepareForRequeue(t)
		if err := p.queue.Enqueue(ctx, t); err != nil {
			log.Error().Err(err).Msg("failed to re-enqueue task")
		}

		if err := p.queue.Acknowledge(ctx, t, messageID); err != nil {
			log.Error().Err(err).Msg("failed to acknowledge task after retry")
		}
	} else {
		// Max retries exceeded, move to dead letter queue
		if err := sm.Fail(execErr.Error()); err != nil {
			log.Error().Err(err).Msg("failed to mark task as failed")
		}
		if err := p.queue.UpdateTask(ctx, t); err != nil {
			log.Error().Err(err).Msg("failed to update task")
		}
		if err := p.dlq.Add(ctx, t, "max retries exceeded"); err != nil {
			log.Error().Err(err).Msg("failed to add task to DLQ")
		}

		if err := p.queue.Acknowledge(ctx, t, messageID); err != nil {
			log.Error().Err(err).Msg("failed to acknowledge task after DLQ")
		}
	}
}

// recoveryLoop periodically checks for orphaned tasks from crashed workers
func (p *Pool) recoveryLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HeartbeatInterval * 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.recoverOrphanedTasks(ctx)
		}
	}
}

// recoverOrphanedTasks claims and re-queues tasks from dead workers
func (p *Pool) recoverOrphanedTasks(ctx context.Context) {
	// Claim tasks that have been pending too long (worker likely crashed)
	tasks, messageIDs, err := p.queue.ClaimOrphanedTasks(ctx, p.id)
	if err != nil {
		logger.Error().Err(err).Msg("failed to claim orphaned tasks")
		return
	}

	for i, t := range tasks {
		logger.Info().
			Str("task_id", t.ID).
			Str("type", t.Type).
			Msg("recovered orphaned task")

		// Reset and re-enqueue for processing
		retryer := task.NewRetryer(task.DefaultRetryPolicy())
		retryer.PrepareForRequeue(t)

		if err := p.queue.Enqueue(ctx, t); err != nil {
			logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to re-enqueue recovered task")
			continue
		}

		// Acknowledge old message
		if err := p.queue.Acknowledge(ctx, t, messageIDs[i]); err != nil {
			logger.Error().Err(err).Str("task_id", t.ID).Msg("failed to acknowledge recovered task")
		}
	}
}
