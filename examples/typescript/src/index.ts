// Example usage of the Task Queue TypeScript client SDK
import { TaskQueueClient, type TaskResponse, type EventType } from '@task-queue/client';

const TASKQUEUE_URL = process.env.TASKQUEUE_URL || 'http://localhost:8080';
const TASKQUEUE_API_KEY = process.env.TASKQUEUE_API_KEY;

async function main() {
  // Create client
  const client = new TaskQueueClient(TASKQUEUE_URL, {
    apiKey: TASKQUEUE_API_KEY,
  });

  try {
    // Health check
    console.log('=== Health Check ===');
    const health = await client.healthCheck();
    console.log(`Status: ${health.status}, Redis: ${health.redis}\n`);

    // Get queue statistics
    console.log('=== Queue Statistics ===');
    const stats = await client.getQueueStats();
    console.log('Queue depths:', stats.queue_depths);
    console.log(`Total pending: ${stats.total_pending}\n`);

    // Create a task
    console.log('=== Create Task ===');
    const task = await client.createTask({
      type: 'example',
      payload: {
        message: 'Hello from TypeScript client!',
        created: new Date().toISOString(),
      },
      priority: 1, // normal
      max_retries: 3,
      timeout: 60,
    });
    console.log(`Created task: ${task.id}`);
    console.log(`State: ${task.state}`);
    console.log(`Priority: ${task.priority}\n`);

    // Get task status
    console.log('=== Get Task Status ===');
    const taskStatus = await client.getTask(task.id!);
    console.log(`Task ${taskStatus.id} state: ${taskStatus.state}\n`);

    // List workers
    console.log('=== Workers ===');
    const workers = await client.listWorkers();
    console.log(`Active workers: ${workers.count}`);
    if (workers.workers) {
      for (const w of workers.workers) {
        console.log(`  - ${w.id} (state: ${w.state}, tasks: ${w.active_tasks})`);
      }
    }
    console.log();

    // Connect to WebSocket for events
    console.log('=== WebSocket Events ===');
    console.log('Connecting to WebSocket...');

    await client.connectWebSocket({
      reconnect: false,
    });
    console.log('Connected! Listening for events...');

    // Track received events
    let eventCount = 0;
    const maxEvents = 5;

    // Listen for all events
    const unsubscribe = client.onAnyEvent((event) => {
      console.log(`Event: ${event.type} at ${event.timestamp}`);
      if (event.data) {
        console.log('  Data:', JSON.stringify(event.data, null, 2));
      }
      eventCount++;
    });

    // Wait for events with timeout
    await new Promise<void>((resolve) => {
      const timeout = setTimeout(() => {
        console.log('Timeout waiting for events');
        resolve();
      }, 10000);

      const checkEvents = setInterval(() => {
        if (eventCount >= maxEvents) {
          console.log(`Received ${maxEvents} events, stopping...`);
          clearTimeout(timeout);
          clearInterval(checkEvents);
          resolve();
        }
      }, 100);
    });

    unsubscribe();
    client.closeWebSocket();
    console.log('WebSocket closed');

    // Cancel the task we created (if still pending)
    console.log('\n=== Cleanup ===');
    try {
      await client.cancelTask(task.id!);
      console.log(`Cancelled task ${task.id}`);
    } catch (err) {
      console.log(`Could not cancel task (may already be processed): ${err}`);
    }

    console.log('\nDone!');
  } catch (err) {
    console.error('Error:', err);
    process.exit(1);
  }
}

// Schedule a task for the future
async function scheduleTaskExample() {
  const client = new TaskQueueClient(TASKQUEUE_URL);

  // Schedule task to run in 5 minutes
  const scheduledTime = new Date(Date.now() + 5 * 60 * 1000);

  const task = await client.createTask({
    type: 'scheduled-job',
    payload: { action: 'cleanup' },
    scheduled_at: scheduledTime.toISOString(),
  });

  console.log(`Scheduled task ${task.id} for ${scheduledTime.toISOString()}`);
}

// Example with different priorities
async function priorityExample() {
  const client = new TaskQueueClient(TASKQUEUE_URL);

  const priorities: Array<{ name: string; value: number }> = [
    { name: 'low', value: 0 },
    { name: 'normal', value: 1 },
    { name: 'high', value: 2 },
    { name: 'critical', value: 3 },
  ];

  for (const { name, value } of priorities) {
    const task = await client.createTask({
      type: 'priority-test',
      payload: { priority: name },
      priority: value,
    });
    console.log(`Created ${name} priority task: ${task.id}`);
  }
}

// Run the main example
main().catch(console.error);
