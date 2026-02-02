# Distributed Task Queue with Real-time Dashboard - Requirements Document

## Project Overview

A horizontally scalable task queue system built with Go, Redis Streams, and WebSockets. The system demonstrates distributed systems architecture, real-time communication, and operational observability patterns similar to production-grade systems like Celery, Bull, or Temporal.

**Target Audience:** Backend engineering hiring managers and technical interviewers
**Primary Goals:** Showcase Go expertise, distributed systems knowledge, real-time architecture, and production operations thinking

---

## Core Functional Requirements

### 1. Task Management

**FR-1.1** The system shall accept task submissions via REST API with JSON payload
**FR-1.2** Each task shall have a unique ID (UUID), type, payload, priority, and metadata
**FR-1.3** Tasks shall support priority levels: LOW (0), NORMAL (1), HIGH (2), CRITICAL (3)
**FR-1.4** The system shall support task scheduling (delay execution by N seconds/minutes/hours)
**FR-1.5** Tasks shall have configurable retry policies (max retries, backoff strategy)
**FR-1.6** The system shall support task cancellation for pending tasks
**FR-1.7** Tasks shall have timeout configuration (max execution time)
**FR-1.8** The system shall support task dependencies (task B runs after task A completes)

### 2. Task Execution

**FR-2.1** Workers shall pull tasks from the queue using fair distribution
**FR-2.2** The system shall support multiple task types with registered handlers
**FR-2.3** Task handlers shall be Go functions with signature: `func(ctx context.Context, payload []byte) error`
**FR-2.4** Workers shall execute tasks concurrently with configurable concurrency limit
**FR-2.5** The system shall implement at-least-once delivery semantics
**FR-2.6** Workers shall send heartbeats during task execution to prevent timeout
**FR-2.7** Failed tasks shall be retried according to retry policy
**FR-2.8** Tasks exceeding max retries shall be moved to dead letter queue (DLQ)

### 3. Worker Pool Management

**FR-3.1** The system shall support multiple worker instances (horizontal scaling)
**FR-3.2** Workers shall register themselves with the system on startup
**FR-3.3** Workers shall have configurable concurrency (number of concurrent tasks)
**FR-3.4** The system shall detect and remove unhealthy workers (missed heartbeats)
**FR-3.5** Workers shall gracefully shutdown (complete in-flight tasks, reject new tasks)
**FR-3.6** The system shall support worker pause/resume via API
**FR-3.7** Workers shall report their current state (idle, busy, paused, shutting down)
**FR-3.8** The system shall support worker scaling based on queue depth (auto-scaling hints)

### 4. Queue Management

**FR-4.1** The system shall use Redis Streams for durable task storage
**FR-4.2** Each priority level shall have a separate queue
**FR-4.3** Workers shall consume from high-priority queues first
**FR-4.4** The system shall track queue depth (pending tasks) per priority
**FR-4.5** The system shall support queue inspection (peek without consuming)
**FR-4.6** The system shall prevent queue overflow with configurable max queue size
**FR-4.7** The system shall support manual queue purging via admin API
**FR-4.8** Dead letter queue shall store failed tasks with failure metadata

### 5. Task State & History

**FR-5.1** Task states: PENDING, SCHEDULED, RUNNING, COMPLETED, FAILED, CANCELLED, DEAD
**FR-5.2** The system shall persist task state transitions in Redis
**FR-5.3** The system shall store task execution history (attempts, errors, duration)
**FR-5.4** Users shall query task status by task ID via REST API
**FR-5.5** Users shall retrieve task result/error via REST API
**FR-5.6** The system shall maintain task history for configurable retention period (default: 7 days)
**FR-5.7** The system shall support task result pagination
**FR-5.8** Completed tasks shall include execution metadata (worker ID, duration, retry count)

### 6. Real-time Dashboard (WebSocket)

**FR-6.1** The system shall provide WebSocket endpoint for real-time updates
**FR-6.2** Dashboard shall receive live updates for:

- Task submissions (new tasks)
- Task state changes (pending → running → completed/failed)
- Worker status changes (connected, disconnected, paused)
- Queue depth changes
- System metrics updates
**FR-6.3** WebSocket connections shall authenticate using JWT tokens
**FR-6.4** The system shall support multiple concurrent dashboard connections
**FR-6.5** Dashboard updates shall have <100ms latency from event occurrence
**FR-6.6** The system shall use Redis Pub/Sub for cross-worker event distribution
**FR-6.7** WebSocket connections shall auto-reconnect with exponential backoff

### 7. Monitoring & Observability

**FR-7.1** The system shall expose Prometheus metrics endpoint
**FR-7.2** Metrics to track:

- Tasks submitted (counter, by type and priority)
- Tasks completed (counter, by type and status)
- Task execution duration (histogram, by type)
- Queue depth (gauge, by priority)
- Active workers (gauge)
- Worker utilization (gauge, percentage)
- Retry count (counter, by type)
- Dead letter queue size (gauge)
**FR-7.3** The system shall implement structured logging (JSON format)
**FR-7.4** Logs shall include correlation IDs (task ID, worker ID)
**FR-7.5** The system shall expose health check endpoint
**FR-7.6** Health check shall verify Redis connectivity and worker health

### 8. Backpressure & Flow Control

**FR-8.1** The system shall reject task submissions when queue is at max capacity
**FR-8.2** The system shall implement exponential backoff for Redis connection failures
**FR-8.3** Workers shall throttle consumption when system resources are constrained
**FR-8.4** The system shall support rate limiting for task submissions (per client/API key)
**FR-8.5** The system shall emit warnings when queue depth exceeds threshold
**FR-8.6** The system shall pause task consumption when Redis latency exceeds threshold

### 9. Admin API

**FR-9.1** Admin API shall be secured with API key authentication
**FR-9.2** Endpoints for task management:

- Get task status and history
- Cancel pending task
- Retry failed task manually
- Requeue task from DLQ
**FR-9.3** Endpoints for worker management:
- List active workers with status
- Get worker details (current tasks, stats)
- Pause/resume specific worker
- Graceful shutdown worker
**FR-9.4** Endpoints for queue management:
- Get queue statistics (depth, throughput)
- Purge queue (with confirmation)
- Get dead letter queue contents
- Bulk requeue from DLQ
**FR-9.5** Endpoints for system operations:
- Get system-wide metrics
- Health check
- Configuration reload

---

## Non-Functional Requirements

### Performance

**NFR-1.1** The system shall process 10,000 tasks per second across all workers
**NFR-1.2** Task submission latency shall be <10ms (P95)
**NFR-1.3** Task pickup latency shall be <100ms from submission to worker start
**NFR-1.4** WebSocket message delivery latency shall be <100ms
**NFR-1.5** Redis operations shall have <5ms timeout
**NFR-1.6** System shall handle 100 concurrent workers
**NFR-1.7** Memory usage per worker shall be <50MB idle, <200MB under load

### Scalability

**NFR-2.1** The system shall support horizontal worker scaling (add/remove workers dynamically)
**NFR-2.2** The system shall handle 1 million pending tasks
**NFR-2.3** The system shall support 100+ concurrent WebSocket connections
**NFR-2.4** Redis shall be the single source of truth for distributed state
**NFR-2.5** The system shall maintain performance with 50+ task types

### Reliability

**NFR-3.1** The system shall have 99.9% task delivery guarantee (at-least-once)
**NFR-3.2** Worker crashes shall not lose in-flight tasks (tasks auto-retry)
**NFR-3.3** Redis connection loss shall trigger automatic reconnection
**NFR-3.4** The system shall survive Redis restarts (tasks persist in Redis Streams)
**NFR-3.5** Graceful shutdown shall complete within 30 seconds
**NFR-3.6** The system shall handle network partitions without data corruption

### Security

**NFR-4.1** API endpoints shall require authentication (JWT or API key)
**NFR-4.2** WebSocket connections shall require valid JWT tokens
**NFR-4.3** Task payloads shall support encryption at rest (optional)
**NFR-4.4** Admin operations shall require elevated permissions
**NFR-4.5** The system shall prevent task payload injection attacks
**NFR-4.6** Logs shall not include sensitive task payload data

### Maintainability

**NFR-5.1** Code shall follow Go best practices (effective Go, Go proverbs)
**NFR-5.2** Test coverage shall be >80%
**NFR-5.3** All public APIs shall have godoc documentation
**NFR-5.4** The system shall have clear module boundaries
**NFR-5.5** Configuration shall be via environment variables or config file
**NFR-5.6** The system shall include comprehensive README and architecture docs

---

## Technical Architecture

### Stack

- **Language:** Go 1.21+
- **Queue Backend:** Redis 7+ (Redis Streams for queue, Pub/Sub for events)
- **HTTP Framework:** Gin or Chi router
- **WebSocket:** gorilla/websocket
- **Metrics:** Prometheus + prometheus/client_golang
- **Testing:** testify, gomock, httptest
- **Serialization:** JSON (encoding/json)
- **Logging:** zerolog or zap
- **Configuration:** viper or envconfig

### System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Applications                   │
└────────────┬────────────────────────────────┬───────────────┘
             │                                │
             │ REST API                       │ WebSocket
             │                                │
┌────────────▼────────────┐      ┌───────────▼──────────────┐
│   API Server (Go)       │      │  WebSocket Server (Go)   │
│   - Task submission     │      │  - Real-time updates     │
│   - Task queries        │      │  - Connection management │
│   - Admin operations    │      │  - Event broadcasting    │
└────────────┬────────────┘      └───────────┬──────────────┘
             │                                │
             │                                │
             └──────────┬─────────────────────┘
                        │
             ┌──────────▼──────────┐
             │   Redis Cluster     │
             │   - Streams (queue) │
             │   - Pub/Sub (events)│
             │   - State (hash)    │
             │   - TTL (expiry)    │
             └──────────┬──────────┘
                        │
          ┌─────────────┴─────────────┐
          │                           │
┌─────────▼──────────┐    ┌──────────▼─────────┐
│  Worker Pool 1     │    │  Worker Pool N     │
│  - Task handlers   │    │  - Task handlers   │
│  - Concurrency mgr │    │  - Concurrency mgr │
│  - Heartbeat       │    │  - Heartbeat       │
└────────────────────┘    └────────────────────┘
```

### Module Structure

```
task-queue/
├── cmd/
│   ├── api-server/          # API server entry point
│   ├── worker/              # Worker entry point
│   └── dashboard/           # React dashboard (separate)
├── internal/
│   ├── api/                 # HTTP handlers and routes
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── websocket/
│   ├── worker/              # Worker implementation
│   │   ├── executor.go      # Task execution engine
│   │   ├── pool.go          # Worker pool management
│   │   ├── heartbeat.go     # Heartbeat mechanism
│   │   └── handlers/        # Task type handlers
│   ├── queue/               # Queue abstraction
│   │   ├── redis_streams.go # Redis Streams implementation
│   │   ├── priority.go      # Priority queue logic
│   │   └── dlq.go           # Dead letter queue
│   ├── task/                # Task domain models
│   │   ├── task.go          # Task struct and methods
│   │   ├── state.go         # State machine
│   │   └── retry.go         # Retry policy logic
│   ├── metrics/             # Prometheus metrics
│   ├── events/              # Event publishing/subscribing
│   ├── config/              # Configuration management
│   └── logger/              # Structured logging
├── pkg/
│   └── client/              # Go client library for task submission
├── examples/                # Example task handlers
├── test/
│   ├── integration/         # Integration tests
│   └── load/                # Load testing scripts (k6)
├── deployments/
│   ├── docker/              # Dockerfiles
│   └── kubernetes/          # K8s manifests (optional)
├── docs/
│   ├── architecture.md
│   ├── api.md
│   └── deployment.md
├── docker-compose.yml       # Local development setup
├── Makefile
└── README.md
```

### Data Models

**Task**

```go
type Task struct {
    ID          string                 `json:"id"`           // UUID
    Type        string                 `json:"type"`         // Handler type
    Priority    Priority               `json:"priority"`     // LOW|NORMAL|HIGH|CRITICAL
    Payload     json.RawMessage        `json:"payload"`      // Task-specific data
    State       TaskState              `json:"state"`        // Current state
    RetryPolicy RetryPolicy            `json:"retry_policy"`
    Timeout     time.Duration          `json:"timeout"`
    ScheduledAt *time.Time             `json:"scheduled_at"` // For delayed tasks
    CreatedAt   time.Time              `json:"created_at"`
    StartedAt   *time.Time             `json:"started_at"`
    CompletedAt *time.Time             `json:"completed_at"`
    Result      json.RawMessage        `json:"result"`       // Success result
    Error       string                 `json:"error"`        // Error message
    Attempts    int                    `json:"attempts"`
    WorkerID    string                 `json:"worker_id"`
    Metadata    map[string]interface{} `json:"metadata"`
}

type TaskState string
const (
    StatePending   TaskState = "PENDING"
    StateScheduled TaskState = "SCHEDULED"
    StateRunning   TaskState = "RUNNING"
    StateCompleted TaskState = "COMPLETED"
    StateFailed    TaskState = "FAILED"
    StateCancelled TaskState = "CANCELLED"
    StateDead      TaskState = "DEAD"
)

type Priority int
const (
    PriorityLow      Priority = 0
    PriorityNormal   Priority = 1
    PriorityHigh     Priority = 2
    PriorityCritical Priority = 3
)

type RetryPolicy struct {
    MaxRetries     int           `json:"max_retries"`
    InitialBackoff time.Duration `json:"initial_backoff"`
    MaxBackoff     time.Duration `json:"max_backoff"`
    BackoffFactor  float64       `json:"backoff_factor"` // Exponential factor
}
```

**Worker**

```go
type Worker struct {
    ID           string        `json:"id"`
    State        WorkerState   `json:"state"`
    Concurrency  int           `json:"concurrency"`
    CurrentTasks []string      `json:"current_tasks"`
    ProcessedCount int64       `json:"processed_count"`
    FailedCount  int64         `json:"failed_count"`
    StartedAt    time.Time     `json:"started_at"`
    LastHeartbeat time.Time    `json:"last_heartbeat"`
}

type WorkerState string
const (
    WorkerIdle       WorkerState = "IDLE"
    WorkerBusy       WorkerState = "BUSY"
    WorkerPaused     WorkerState = "PAUSED"
    WorkerShutdown   WorkerState = "SHUTTING_DOWN"
)
```

**Event (WebSocket/Pub-Sub)**

```go
type Event struct {
    Type      EventType   `json:"type"`
    Timestamp time.Time   `json:"timestamp"`
    Data      interface{} `json:"data"`
}

type EventType string
const (
    EventTaskSubmitted  EventType = "task.submitted"
    EventTaskStarted    EventType = "task.started"
    EventTaskCompleted  EventType = "task.completed"
    EventTaskFailed     EventType = "task.failed"
    EventWorkerJoined   EventType = "worker.joined"
    EventWorkerLeft     EventType = "worker.left"
    EventQueueDepth     EventType = "queue.depth"
    EventSystemMetrics  EventType = "system.metrics"
)
```

---

## API Endpoints

### Public API (Port 8080)

**Task Management**

```
POST   /api/v1/tasks
  Body: { type, payload, priority?, retry_policy?, timeout?, scheduled_at? }
  Response: { task_id, state }

GET    /api/v1/tasks/:id
  Response: Task object with full details

DELETE /api/v1/tasks/:id
  Description: Cancel pending task
  Response: { cancelled: true }

GET    /api/v1/tasks
  Query: ?state=COMPLETED&type=email&limit=50&offset=0
  Response: { tasks: [], total, limit, offset }
```

**WebSocket**

```
GET    /ws
  Headers: Authorization: Bearer <jwt>
  Description: Real-time event stream
```

**Health & Metrics**

```
GET    /health
  Response: { status: "healthy", redis: "connected", workers: 5 }

GET    /metrics
  Description: Prometheus metrics
```

### Admin API (Port 8081)

**Worker Management**

```
GET    /admin/workers
  Response: { workers: [Worker] }

GET    /admin/workers/:id
  Response: Worker object with stats

POST   /admin/workers/:id/pause
POST   /admin/workers/:id/resume
POST   /admin/workers/:id/shutdown
```

**Queue Management**

```
GET    /admin/queues
  Response: { queues: [{ priority, depth, throughput }] }

GET    /admin/queues/:priority
  Response: Queue details

DELETE /admin/queues/:priority
  Description: Purge queue

GET    /admin/dlq
  Response: Dead letter queue contents

POST   /admin/dlq/retry
  Body: { task_ids: [] }
  Description: Retry tasks from DLQ
```

**Task Management**

```
POST   /admin/tasks/:id/retry
  Description: Manual retry of failed task
```

---

## Implementation Phases

### Phase 1: Core Queue (Week 1)

- Project setup (Go modules, directory structure)
- Redis Streams integration
- Basic task model and state machine
- Priority queue implementation
- Simple worker executor (single-threaded)
- Task submission and consumption

### Phase 2: Worker Pool (Week 2)

- Worker pool management
- Concurrent task execution
- Worker heartbeat mechanism
- Graceful shutdown
- Worker state tracking in Redis
- Health detection and removal

### Phase 3: Reliability (Week 3)

- Retry policy implementation
- Dead letter queue
- Task timeout handling
- At-least-once delivery guarantee
- Task cancellation
- Error handling and recovery

### Phase 4: Observability (Week 4)

- Prometheus metrics integration
- Structured logging with correlation IDs
- Admin API implementation
- Queue statistics and monitoring
- Worker performance metrics

### Phase 5: Real-time Dashboard (Week 5)

- WebSocket server implementation
- Redis Pub/Sub event distribution
- Event streaming to clients
- JWT authentication for WebSocket
- Connection lifecycle management
- React dashboard UI (basic)

### Phase 6: Polish (Week 6)

- Comprehensive test suite
- Load testing with k6 or vegeta
- Documentation (godoc, README, architecture)
- Docker Compose setup
- Example task handlers
- Performance benchmarking report

---

## Testing Strategy

### Unit Tests

- Task state machine transitions
- Retry policy calculations (backoff timing)
- Priority queue ordering
- Worker pool concurrency limits
- Event publishing/subscribing
- Task timeout detection

### Integration Tests

- Full task lifecycle (submit → execute → complete)
- Worker registration and heartbeat
- Dead letter queue flow
- Redis connection failure recovery
- Graceful shutdown with in-flight tasks
- WebSocket event delivery

### Load Tests

- 10k tasks/sec submission and processing
- 100 concurrent workers
- Memory leak detection (run for 1 hour)
- Redis connection pool exhaustion
- WebSocket connection stability (100+ clients)
- Queue depth with 1M tasks

### Chaos Tests (Optional)

- Redis connection failures during task execution
- Worker crashes mid-task
- Network partitions
- Redis restart with pending tasks

### Test Coverage Goals

- Statements: 80%+
- Functions: 85%+
- Branches: 75%+

---

## Configuration

**Environment Variables**

```bash
# Redis
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=100

# API Server
API_PORT=8080
ADMIN_PORT=8081
JWT_SECRET=changeme
API_KEY=admin-secret

# Worker
WORKER_ID=worker-1
WORKER_CONCURRENCY=10
WORKER_HEARTBEAT_INTERVAL=5s
WORKER_SHUTDOWN_TIMEOUT=30s

# Queue
MAX_QUEUE_SIZE=1000000
TASK_RETENTION_DAYS=7
DEFAULT_TASK_TIMEOUT=5m
MAX_RETRIES=3

# Metrics
METRICS_ENABLED=true
LOG_LEVEL=info
```

---

## Deliverables

1. **Source Code:** Complete Go implementation with modular architecture
2. **Documentation:**
   - README with quickstart (running with Docker Compose)
   - Architecture overview with diagrams
   - API documentation (OpenAPI/Swagger)
   - Deployment guide
3. **Tests:** Comprehensive test suite (unit + integration)
4. **Docker Setup:** Docker Compose with Redis and all services
5. **Dashboard:** React app showing real-time task flow and metrics
6. **Load Test Results:** Benchmark report with graphs
7. **Blog Post:** Technical writeup on distributed task queue design

---

## Success Criteria

- [ ] System processes 10k tasks/sec with 100 workers
- [ ] At-least-once delivery verified (no task loss)
- [ ] WebSocket updates <100ms latency
- [ ] Worker crashes auto-recover tasks
- [ ] Graceful shutdown completes in <30s
- [ ] 80%+ test coverage
- [ ] Redis restart doesn't lose tasks
- [ ] Dashboard shows live updates with <100ms delay
- [ ] Clean Go code following best practices
- [ ] Comprehensive documentation

---

## Out of Scope (Future Enhancements)

- Task dependencies (DAG execution)
- Distributed tracing (OpenTelemetry)
- Task result caching
- Webhook callbacks on completion
- Multi-region deployment
- Task prioritization based on SLA
- Worker auto-scaling (Kubernetes HPA)
- Task scheduling with cron expressions
- Multi-tenant support

---

## Example Task Handlers

```go
// Email sending task
func SendEmailHandler(ctx context.Context, payload []byte) error {
    var email EmailPayload
    if err := json.Unmarshal(payload, &email); err != nil {
        return err
    }
    // Send email logic
    return smtp.Send(email.To, email.Subject, email.Body)
}

// Image processing task
func ProcessImageHandler(ctx context.Context, payload []byte) error {
    var img ImagePayload
    if err := json.Unmarshal(payload, &img); err != nil {
        return err
    }
    // Download, resize, upload
    return processImage(ctx, img.URL, img.Size)
}

// Data export task (long-running)
func ExportDataHandler(ctx context.Context, payload []byte) error {
    var export ExportPayload
    if err := json.Unmarshal(payload, &export); err != nil {
        return err
    }
    // Generate CSV/JSON, upload to S3
    return exportToStorage(ctx, export.Query, export.Format)
}
```

---

## References & Resources

- [Redis Streams Documentation](https://redis.io/docs/data-types/streams/)
- [Bull Queue Architecture](https://github.com/OptimalBits/bull)
- [Temporal Workflow Engine](https://temporal.io/)
- [Goroutine Best Practices](https://golang.org/doc/effective_go)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [WebSocket Protocol RFC 6455](https://tools.ietf.org/html/rfc6455)

---

**Document Version:** 1.0
**Last Updated:** 2026-01-30
**Author:** Backend Engineering Portfolio Project
