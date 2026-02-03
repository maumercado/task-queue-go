import { TaskQueueClient } from '@task-queue/client';
import { browser } from '$app/environment';

const baseUrl = browser
  ? (import.meta.env.VITE_API_URL || 'http://localhost:8080')
  : 'http://localhost:8080';

export const client = new TaskQueueClient(baseUrl);
