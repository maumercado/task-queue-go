# API Reference

Base URL: `http://localhost:8080`

## Authentication

Authentication is disabled by default. When enabled, use either:

- **API Key**: `X-API-Key: your-api-key` header
- **JWT**: `Authorization: Bearer <token>` header

## Task API

### Create Task

```
POST /api/v1/tasks
```

**Request Body:**

```json
{
  "type": "email",
  "payload": {
    "to": "user@example.com",
    "subject": "Welcome",
    "body": "Hello!"
  },
  "priority": 2,
  "max_retries": 5,
  "timeout": 300,
  "metadata": {
    "source": "signup-flow"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | Task type (must match a registered handler) |
| payload | object | No | Data passed to the handler |
| priority | int | No | 0=low, 1=normal, 2=high, 3=critical (default: 0) |
| max_retries | int | No | Max retry attempts (default: 3) |
| timeout | int | No | Execution timeout in seconds (default: 300) |
| metadata | object | No | Arbitrary key-value metadata |

**Response:** `201 Created`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "payload": {"to": "user@example.com", "subject": "Welcome", "body": "Hello!"},
  "priority": "high",
  "state": "pending",
  "attempts": 0,
  "max_retries": 5,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### Get Task

```
GET /api/v1/tasks/{id}
```

**Response:** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "payload": {"to": "user@example.com"},
  "priority": "high",
  "state": "completed",
  "attempts": 1,
  "max_retries": 5,
  "result": {"sent": true, "message_id": "msg-123"},
  "worker_id": "worker-abc123",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:05Z",
  "started_at": "2024-01-15T10:30:01Z",
  "completed_at": "2024-01-15T10:30:05Z"
}
```

**Error:** `404 Not Found`

```json
{
  "error": "Not Found",
  "message": "task not found"
}
```

### Cancel Task

```
DELETE /api/v1/tasks/{id}
```

Only tasks in `pending` or `scheduled` state can be cancelled.

**Response:** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "state": "cancelled",
  ...
}
```

**Error:** `409 Conflict`

```json
{
  "error": "Conflict",
  "message": "task cannot be cancelled in current state"
}
```

### List Queue Depths

```
GET /api/v1/tasks
```

**Response:** `200 OK`

```json
{
  "queue_depths": {
    "critical": 5,
    "high": 23,
    "normal": 142,
    "low": 8
  },
  "total_pending": 178
}
```

## Admin API

### Health Check

```
GET /admin/health
```

**Response:** `200 OK`

```json
{
  "status": "healthy",
  "redis": "connected"
}
```

**Error:** `503 Service Unavailable`

```json
{
  "status": "unhealthy",
  "redis": "disconnected",
  "error": "connection refused"
}
```

### List Workers

```
GET /admin/workers
```

**Response:** `200 OK`

```json
{
  "workers": [
    {
      "id": "worker-abc123",
      "state": "busy",
      "started_at": "2024-01-15T10:00:00Z",
      "last_heartbeat": "2024-01-15T10:30:00Z",
      "active_tasks": 5,
      "concurrency": 10
    }
  ],
  "count": 1
}
```

### Get Queue Statistics

```
GET /admin/queues
```

**Response:** `200 OK`

```json
{
  "queues": {
    "critical": {"depth": 5, "priority": 3},
    "high": {"depth": 23, "priority": 2},
    "normal": {"depth": 142, "priority": 1},
    "low": {"depth": 8, "priority": 0}
  },
  "total_depth": 178
}
```

### List Dead Letter Queue

```
GET /admin/dlq
```

**Response:** `200 OK`

```json
{
  "entries": [
    {
      "task": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "type": "email",
        "state": "dead_letter",
        "attempts": 3,
        "error": "SMTP connection failed"
      },
      "reason": "max retries exceeded",
      "added_at": "2024-01-15T10:35:00Z",
      "message_id": "1705315200000-0"
    }
  ],
  "size": 1
}
```

### Retry DLQ Tasks

```
POST /admin/dlq/retry
```

**Request Body (single task):**

```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Request Body (all tasks):**

```json
{
  "retry_all": true
}
```

**Response:** `200 OK`

```json
{
  "message": "task re-queued",
  "task_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

Or for retry_all:

```json
{
  "message": "tasks re-queued",
  "retried_count": 5
}
```

### Clear DLQ

```
DELETE /admin/dlq
```

**Response:** `200 OK`

```json
{
  "message": "DLQ cleared"
}
```

## WebSocket

### Connect

```
GET /ws
```

Upgrades to WebSocket connection. Events are pushed as JSON messages.

### Event Format

```json
{
  "type": "task.completed",
  "timestamp": "2024-01-15T10:30:05Z",
  "data": {
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "email",
    "priority": "high"
  }
}
```

### Event Types

| Event | Description |
|-------|-------------|
| `task.submitted` | Task added to queue |
| `task.started` | Worker began processing |
| `task.completed` | Task finished successfully |
| `task.failed` | Task execution failed |
| `task.retrying` | Task scheduled for retry |
| `worker.joined` | Worker registered |
| `worker.left` | Worker deregistered |
| `worker.paused` | Worker paused |
| `worker.resumed` | Worker resumed |
| `queue.depth` | Queue depth update |

## Metrics

```
GET /metrics
```

Returns Prometheus-formatted metrics. Key metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `taskqueue_tasks_submitted_total` | counter | type, priority | Tasks submitted |
| `taskqueue_tasks_completed_total` | counter | type, status | Tasks completed |
| `taskqueue_task_duration_seconds` | histogram | type | Execution time |
| `taskqueue_queue_depth` | gauge | priority | Pending tasks |
| `taskqueue_active_workers` | gauge | - | Active workers |
| `taskqueue_dlq_size` | gauge | - | DLQ size |

## Error Responses

All errors return JSON:

```json
{
  "error": "Bad Request",
  "message": "task type is required"
}
```

| Status | Meaning |
|--------|---------|
| 400 | Invalid request body or parameters |
| 401 | Authentication required or invalid |
| 403 | Insufficient permissions |
| 404 | Resource not found |
| 409 | Conflict (e.g., invalid state transition) |
| 500 | Internal server error |
| 503 | Service unavailable (e.g., Redis down) |
