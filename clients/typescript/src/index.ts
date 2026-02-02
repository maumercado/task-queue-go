// Main client
export { TaskQueueClient, type TaskQueueClientOptions } from './client';

// WebSocket client
export {
  WebSocketClient,
  type WebSocketClientOptions,
  type EventCallback,
  type EventType,
} from './websocket';

// Re-export all generated types
export type {
  CreateTaskRequest,
  TaskResponse,
  ErrorResponse,
  HealthResponse,
  QueueStats,
  QueueDetailedStats,
  WorkerInfo,
  WorkerListResponse,
  DlqEntry,
  DlqListResponse,
  RetryDlqRequest,
  RetryDlqSingleResponse,
  RetryDlqAllResponse,
  WebSocketEvent,
  TaskId,
  WorkerId,
  Priority,
} from './generated/types.gen';

// Re-export low-level SDK functions for advanced use
export * as sdk from './generated/sdk.gen';

// Re-export client creation utilities for custom clients
export { createClient, createConfig } from './generated/client';
