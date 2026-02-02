import { createClient, createConfig } from './generated/client';
import type { Client } from './generated/client';
import * as sdk from './generated/sdk.gen';
import type {
  CreateTaskRequest,
  TaskResponse,
  QueueStats,
  HealthResponse,
  WorkerListResponse,
  WorkerInfo,
  DlqListResponse,
  QueueDetailedStats,
  Priority,
} from './generated/types.gen';
import { WebSocketClient, type WebSocketClientOptions, type EventCallback, type EventType } from './websocket';

export interface TaskQueueClientOptions {
  apiKey?: string;
  headers?: Record<string, string>;
  timeout?: number;
}

export class TaskQueueClient {
  private client: Client;
  private baseUrl: string;
  private options: TaskQueueClientOptions;
  private ws: WebSocketClient | null = null;

  constructor(baseUrl: string, options: TaskQueueClientOptions = {}) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.options = options;

    const headers: Record<string, string> = {
      ...options.headers,
    };

    if (options.apiKey) {
      headers['Authorization'] = `Bearer ${options.apiKey}`;
    }

    this.client = createClient(
      createConfig({
        baseUrl: this.baseUrl,
        headers,
      })
    );
  }

  // Task Operations

  async createTask(request: CreateTaskRequest): Promise<TaskResponse> {
    const response = await sdk.createTask({
      client: this.client,
      body: request,
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to create task');
    }

    return response.data as TaskResponse;
  }

  async getTask(taskId: string): Promise<TaskResponse> {
    const response = await sdk.getTask({
      client: this.client,
      path: { taskId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Task not found');
    }

    return response.data as TaskResponse;
  }

  async cancelTask(taskId: string): Promise<TaskResponse> {
    const response = await sdk.cancelTask({
      client: this.client,
      path: { taskId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to cancel task');
    }

    return response.data as TaskResponse;
  }

  async getQueueStats(): Promise<QueueStats> {
    const response = await sdk.listTasks({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to get queue stats');
    }

    return response.data as QueueStats;
  }

  // Health Operations

  async healthCheck(): Promise<HealthResponse> {
    const response = await sdk.healthCheck({
      client: this.client,
    });

    // Health check can return 503 for unhealthy status
    return response.data as HealthResponse;
  }

  // Worker Operations

  async listWorkers(): Promise<WorkerListResponse> {
    const response = await sdk.listWorkers({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to list workers');
    }

    return response.data as WorkerListResponse;
  }

  async getWorker(workerId: string): Promise<WorkerInfo> {
    const response = await sdk.getWorker({
      client: this.client,
      path: { workerId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Worker not found');
    }

    return response.data as WorkerInfo;
  }

  async pauseWorker(workerId: string): Promise<void> {
    const response = await sdk.pauseWorker({
      client: this.client,
      path: { workerId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to pause worker');
    }
  }

  async resumeWorker(workerId: string): Promise<void> {
    const response = await sdk.resumeWorker({
      client: this.client,
      path: { workerId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to resume worker');
    }
  }

  // Queue Operations

  async getQueues(): Promise<QueueDetailedStats> {
    const response = await sdk.getQueues({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to get queue details');
    }

    return response.data as QueueDetailedStats;
  }

  async purgeQueue(priority: Priority): Promise<void> {
    const response = await sdk.purgeQueue({
      client: this.client,
      path: { priority },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to purge queue');
    }
  }

  // Admin Task Operations

  async retryTask(taskId: string): Promise<void> {
    const response = await sdk.retryTask({
      client: this.client,
      path: { taskId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to retry task');
    }
  }

  // DLQ Operations

  async listDlq(): Promise<DlqListResponse> {
    const response = await sdk.listDlq({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to list DLQ entries');
    }

    return response.data as DlqListResponse;
  }

  async clearDlq(): Promise<void> {
    const response = await sdk.clearDlq({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to clear DLQ');
    }
  }

  async retryDlqTask(taskId: string): Promise<void> {
    const response = await sdk.retryDlq({
      client: this.client,
      body: { task_id: taskId },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to retry DLQ task');
    }
  }

  async retryAllDlqTasks(): Promise<number> {
    const response = await sdk.retryDlq({
      client: this.client,
      body: { retry_all: true },
    });

    if (response.error) {
      throw new Error(response.error.message || 'Failed to retry DLQ tasks');
    }

    const data = response.data as { retried_count?: number };
    return data.retried_count ?? 0;
  }

  // Metrics

  async getMetrics(): Promise<string> {
    const response = await sdk.getMetrics({
      client: this.client,
    });

    if (response.error) {
      throw new Error('Failed to get metrics');
    }

    return response.data as string;
  }

  // WebSocket Operations

  async connectWebSocket(options?: WebSocketClientOptions): Promise<void> {
    if (this.ws?.isConnected) {
      return;
    }

    this.ws = new WebSocketClient(this.baseUrl, {
      apiKey: this.options.apiKey,
      ...options,
    });

    await this.ws.connect();
  }

  onEvent(eventType: EventType, callback: EventCallback): () => void {
    if (!this.ws) {
      throw new Error('WebSocket not connected. Call connectWebSocket() first.');
    }
    return this.ws.on(eventType, callback);
  }

  onAnyEvent(callback: EventCallback): () => void {
    if (!this.ws) {
      throw new Error('WebSocket not connected. Call connectWebSocket() first.');
    }
    return this.ws.onAny(callback);
  }

  subscribeToEvents(eventTypes: EventType[]): void {
    if (!this.ws) {
      throw new Error('WebSocket not connected. Call connectWebSocket() first.');
    }
    this.ws.subscribe(eventTypes);
  }

  unsubscribeFromEvents(eventTypes: EventType[]): void {
    if (!this.ws) {
      throw new Error('WebSocket not connected. Call connectWebSocket() first.');
    }
    this.ws.unsubscribe(eventTypes);
  }

  closeWebSocket(): void {
    this.ws?.close();
    this.ws = null;
  }

  get isWebSocketConnected(): boolean {
    return this.ws?.isConnected ?? false;
  }
}
