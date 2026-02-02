import type { WebSocketEvent } from './generated/types.gen';

export type EventType = NonNullable<WebSocketEvent['type']>;

export type EventCallback = (event: WebSocketEvent) => void;

export interface WebSocketClientOptions {
  apiKey?: string;
  reconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

export class WebSocketClient {
  private socket: WebSocket | null = null;
  private baseUrl: string;
  private options: WebSocketClientOptions;
  private listeners: Map<string, Set<EventCallback>> = new Map();
  private globalListeners: Set<EventCallback> = new Set();
  private reconnectAttempts = 0;
  private shouldReconnect = true;

  constructor(baseUrl: string, options: WebSocketClientOptions = {}) {
    this.baseUrl = baseUrl;
    this.options = {
      reconnect: true,
      reconnectInterval: 5000,
      maxReconnectAttempts: 10,
      ...options,
    };
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const wsUrl = this.getWebSocketUrl();

      this.socket = new WebSocket(wsUrl);

      this.socket.onopen = () => {
        this.reconnectAttempts = 0;
        resolve();
      };

      this.socket.onerror = (error) => {
        reject(new Error(`WebSocket connection failed: ${error}`));
      };

      this.socket.onclose = (event) => {
        if (this.shouldReconnect && this.options.reconnect) {
          this.attemptReconnect();
        }
      };

      this.socket.onmessage = (event) => {
        this.handleMessage(event.data);
      };
    });
  }

  private getWebSocketUrl(): string {
    const url = new URL(this.baseUrl);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    url.pathname = '/ws';

    if (this.options.apiKey) {
      url.searchParams.set('token', this.options.apiKey);
    }

    return url.toString();
  }

  private handleMessage(data: string): void {
    try {
      const event = JSON.parse(data) as WebSocketEvent;

      // Notify type-specific listeners
      if (event.type) {
        const typeListeners = this.listeners.get(event.type);
        if (typeListeners) {
          typeListeners.forEach((callback) => callback(event));
        }
      }

      // Notify global listeners
      this.globalListeners.forEach((callback) => callback(event));
    } catch {
      // Ignore malformed messages
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= (this.options.maxReconnectAttempts ?? 10)) {
      return;
    }

    this.reconnectAttempts++;
    setTimeout(() => {
      this.connect().catch(() => {
        // Reconnect failed, will retry
      });
    }, this.options.reconnectInterval);
  }

  on(eventType: EventType, callback: EventCallback): () => void {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set());
    }
    this.listeners.get(eventType)!.add(callback);

    // Return unsubscribe function
    return () => {
      this.listeners.get(eventType)?.delete(callback);
    };
  }

  onAny(callback: EventCallback): () => void {
    this.globalListeners.add(callback);
    return () => {
      this.globalListeners.delete(callback);
    };
  }

  off(eventType: EventType, callback: EventCallback): void {
    this.listeners.get(eventType)?.delete(callback);
  }

  offAll(eventType?: EventType): void {
    if (eventType) {
      this.listeners.delete(eventType);
    } else {
      this.listeners.clear();
      this.globalListeners.clear();
    }
  }

  subscribe(eventTypes: EventType[]): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    this.socket.send(
      JSON.stringify({
        action: 'subscribe',
        events: eventTypes,
      })
    );
  }

  unsubscribe(eventTypes: EventType[]): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket not connected');
    }

    this.socket.send(
      JSON.stringify({
        action: 'unsubscribe',
        events: eventTypes,
      })
    );
  }

  close(): void {
    this.shouldReconnect = false;
    if (this.socket) {
      this.socket.close(1000, 'Client closing');
      this.socket = null;
    }
  }

  get isConnected(): boolean {
    return this.socket?.readyState === WebSocket.OPEN;
  }
}
