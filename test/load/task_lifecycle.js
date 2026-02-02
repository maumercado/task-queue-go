// k6 Load Testing Script - Full Task Lifecycle
// Tests: Submit -> Poll until completion -> Verify result
// Run with: k6 run test/load/task_lifecycle.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

// Custom metrics
const taskCompletions = new Counter('task_completions');
const taskFailures = new Counter('task_failures');
const taskTimeouts = new Counter('task_timeouts');
const taskTotalDuration = new Trend('task_total_duration');
const taskPollingAttempts = new Trend('task_polling_attempts');

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const MAX_POLL_ATTEMPTS = 30;  // Max 30 attempts
const POLL_INTERVAL = 1;       // 1 second between polls

export const options = {
  scenarios: {
    lifecycle: {
      executor: 'constant-vus',
      vus: 10,
      duration: '2m',
    },
  },
  thresholds: {
    task_total_duration: ['p(95)<5000'],  // 95% complete within 5s
    http_req_failed: ['rate<0.05'],
  },
};

// Submit a task and wait for completion
export default function() {
  const task = {
    type: 'echo',
    payload: { message: `Lifecycle test ${Date.now()}` },
    priority: 1,
    max_retries: 3,
  };

  const headers = { 'Content-Type': 'application/json' };
  const startTime = Date.now();

  // Submit task
  const submitRes = http.post(
    `${BASE_URL}/api/v1/tasks`,
    JSON.stringify(task),
    { headers }
  );

  const submitted = check(submitRes, {
    'task submitted': (r) => r.status === 201,
  });

  if (!submitted) {
    console.log(`Failed to submit task: ${submitRes.status}`);
    taskFailures.add(1);
    return;
  }

  const taskData = JSON.parse(submitRes.body);
  const taskId = taskData.id;

  // Poll for completion
  let attempts = 0;
  let completed = false;
  let finalState = 'unknown';

  while (attempts < MAX_POLL_ATTEMPTS && !completed) {
    sleep(POLL_INTERVAL);
    attempts++;

    const pollRes = http.get(`${BASE_URL}/api/v1/tasks/${taskId}`);

    if (pollRes.status !== 200) {
      console.log(`Failed to poll task ${taskId}: ${pollRes.status}`);
      continue;
    }

    const pollData = JSON.parse(pollRes.body);
    finalState = pollData.state;

    // Check terminal states
    if (finalState === 'completed' || finalState === 'failed' || finalState === 'dead_letter') {
      completed = true;
    }
  }

  const totalDuration = Date.now() - startTime;
  taskTotalDuration.add(totalDuration);
  taskPollingAttempts.add(attempts);

  if (!completed) {
    console.log(`Task ${taskId} timed out after ${attempts} attempts, state: ${finalState}`);
    taskTimeouts.add(1);
    return;
  }

  if (finalState === 'completed') {
    taskCompletions.add(1);
  } else {
    console.log(`Task ${taskId} ended in state: ${finalState}`);
    taskFailures.add(1);
  }

  check(null, {
    'task completed successfully': () => finalState === 'completed',
    'completed within 5s': () => totalDuration < 5000,
    'polling attempts < 10': () => attempts < 10,
  });
}
