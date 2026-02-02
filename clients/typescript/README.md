# Task Queue TypeScript Client

TypeScript/JavaScript client SDK for the Task Queue API.

## Installation

```bash
npm install @task-queue/client
```

## Quick Start

```typescript
import { TaskQueueClient } from '@task-queue/client';

const client = new TaskQueueClient('http://localhost:8080', {
  apiKey: 'your-api-key', // optional
});

// Create a task
const task = await client.createTask({
  type: 'email',
  payload: {
    to: 'user@example.com',
    subject: 'Welcome',
  },
  priority: 1, // 0=low, 1=normal, 2=high, 3=critical
});

console.log('Task created:', task.id);

// Get task status
const status = await client.getTask(task.id!);
console.log('Task state:', status.state);

// Cancel a task
await client.cancelTask(task.id!);
```

## WebSocket Events

Subscribe to real-time task events:

```typescript
// Connect to WebSocket
await client.connectWebSocket();

// Listen for specific event types
const unsubscribe = client.onEvent('task.completed', (event) => {
  console.log('Task completed:', event.data);
});

// Listen for all events
client.onAnyEvent((event) => {
  console.log('Event:', event.type, event.data);
});

// Cleanup
unsubscribe();
client.closeWebSocket();
```

Available event types:
- `task.submitted`
- `task.started`
- `task.completed`
- `task.failed`
- `task.retrying`
- `worker.joined`
- `worker.left`
- `worker.paused`
- `worker.resumed`
- `queue.depth`
- `system.metrics`

## API Reference

### Task Operations

```typescript
// Create a task
await client.createTask({ type: 'email', payload: {...} });

// Get task by ID
await client.getTask(taskId);

// Cancel a task
await client.cancelTask(taskId);

// Get queue statistics
await client.getQueueStats();
```

### Worker Operations

```typescript
// List all workers
await client.listWorkers();

// Get worker details
await client.getWorker(workerId);

// Pause a worker
await client.pauseWorker(workerId);

// Resume a worker
await client.resumeWorker(workerId);
```

### Queue Operations

```typescript
// Get detailed queue stats
await client.getQueues();

// Purge a queue
await client.purgeQueue('normal'); // 'critical' | 'high' | 'normal' | 'low'
```

### Dead Letter Queue

```typescript
// List DLQ entries
await client.listDlq();

// Retry a specific task
await client.retryDlqTask(taskId);

// Retry all DLQ tasks
const count = await client.retryAllDlqTasks();

// Clear the DLQ
await client.clearDlq();
```

### Health & Metrics

```typescript
// Health check
const health = await client.healthCheck();

// Prometheus metrics
const metrics = await client.getMetrics();
```

## Low-Level SDK

For advanced use cases, you can use the generated SDK functions directly:

```typescript
import { sdk, createClient, createConfig } from '@task-queue/client';

const client = createClient(createConfig({
  baseUrl: 'http://localhost:8080',
  headers: {
    'Authorization': 'Bearer your-api-key',
  },
}));

const response = await sdk.createTask({
  client,
  body: { type: 'email', payload: {} },
});
```

## Development

```bash
# Install dependencies
npm install

# Generate client from OpenAPI spec
npm run generate

# Build
npm run build
```

## License

MIT
