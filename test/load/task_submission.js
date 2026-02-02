// k6 Load Testing Script for Task Queue
// Run with: k6 run test/load/task_submission.js
// With custom options: k6 run --vus 50 --duration 30s test/load/task_submission.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics
const taskSubmissions = new Counter('task_submissions');
const taskSubmissionErrors = new Counter('task_submission_errors');
const taskSubmissionRate = new Rate('task_submission_success_rate');
const taskSubmissionDuration = new Trend('task_submission_duration');

// Configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || '';

// Test options
export const options = {
  scenarios: {
    // Smoke test - verify system works
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
      startTime: '0s',
      tags: { test_type: 'smoke' },
    },
    // Load test - normal load
    load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 20 },  // Ramp up
        { duration: '1m', target: 20 },   // Stay at 20
        { duration: '30s', target: 50 },  // Ramp to 50
        { duration: '1m', target: 50 },   // Stay at 50
        { duration: '30s', target: 0 },   // Ramp down
      ],
      startTime: '15s',
      tags: { test_type: 'load' },
    },
    // Stress test - find breaking point
    stress: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 200,
      stages: [
        { duration: '30s', target: 100 },  // Ramp to 100 req/s
        { duration: '1m', target: 100 },   // Stay at 100 req/s
        { duration: '30s', target: 200 },  // Ramp to 200 req/s
        { duration: '1m', target: 200 },   // Stay at 200 req/s
        { duration: '30s', target: 0 },    // Ramp down
      ],
      startTime: '5m',
      tags: { test_type: 'stress' },
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'],  // 95% < 100ms, 99% < 200ms
    task_submission_success_rate: ['rate>0.99'],    // 99% success rate
    http_req_failed: ['rate<0.01'],                  // Less than 1% errors
  },
};

// Task types for variety
const TASK_TYPES = ['echo', 'sleep', 'compute', 'webhook', 'email'];

// Priority distribution (weighted random)
const PRIORITIES = [
  { value: 0, weight: 40 },  // low - 40%
  { value: 1, weight: 35 },  // normal - 35%
  { value: 2, weight: 20 },  // high - 20%
  { value: 3, weight: 5 },   // critical - 5%
];

// Weighted random selection
function weightedRandom(items) {
  const totalWeight = items.reduce((sum, item) => sum + item.weight, 0);
  let random = Math.random() * totalWeight;

  for (const item of items) {
    random -= item.weight;
    if (random <= 0) {
      return item.value;
    }
  }
  return items[0].value;
}

// Generate random task payload
function generateTask() {
  const taskType = TASK_TYPES[Math.floor(Math.random() * TASK_TYPES.length)];
  const priority = weightedRandom(PRIORITIES);

  const payloads = {
    echo: { message: `Test message ${Date.now()}` },
    sleep: { duration: Math.floor(Math.random() * 1000) + 100 },
    compute: { iterations: Math.floor(Math.random() * 1000) + 100 },
    webhook: { url: 'https://httpbin.org/post', method: 'POST' },
    email: { to: 'test@example.com', subject: 'Load Test', body: 'Test email' },
  };

  return {
    type: taskType,
    payload: payloads[taskType],
    priority: priority,
    max_retries: 3,
    timeout: 60,
    metadata: {
      source: 'k6-load-test',
      iteration: __ITER,
      vu: __VU,
    },
  };
}

// Setup - runs once before test
export function setup() {
  // Verify API is reachable
  const healthRes = http.get(`${BASE_URL}/admin/health`);

  if (healthRes.status !== 200) {
    throw new Error(`API not healthy: ${healthRes.status}`);
  }

  console.log('API is healthy, starting load test');
  return { startTime: Date.now() };
}

// Main test function
export default function() {
  const task = generateTask();

  const headers = {
    'Content-Type': 'application/json',
  };

  if (API_KEY) {
    headers['X-API-Key'] = API_KEY;
  }

  const startTime = Date.now();

  const res = http.post(
    `${BASE_URL}/api/v1/tasks`,
    JSON.stringify(task),
    { headers }
  );

  const duration = Date.now() - startTime;
  taskSubmissionDuration.add(duration);

  const success = check(res, {
    'status is 201': (r) => r.status === 201,
    'has task id': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.id && body.id.length > 0;
      } catch (e) {
        return false;
      }
    },
    'state is pending': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.state === 'pending';
      } catch (e) {
        return false;
      }
    },
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  if (success) {
    taskSubmissions.add(1);
    taskSubmissionRate.add(1);
  } else {
    taskSubmissionErrors.add(1);
    taskSubmissionRate.add(0);
    console.log(`Failed request: ${res.status} - ${res.body}`);
  }

  // Small random sleep between requests (0-100ms)
  sleep(Math.random() * 0.1);
}

// Teardown - runs once after test
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000;
  console.log(`Test completed in ${duration.toFixed(2)}s`);

  // Get final queue stats
  const res = http.get(`${BASE_URL}/admin/queues`);
  if (res.status === 200) {
    console.log(`Final queue stats: ${res.body}`);
  }
}
