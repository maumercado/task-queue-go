import { writable } from 'svelte/store';
import type { WebSocketEvent } from '@task-queue/client';

export interface EventWithId extends WebSocketEvent {
  id: string;
}

const MAX_EVENTS = 50;

function createEventsStore() {
  const { subscribe, update } = writable<EventWithId[]>([]);

  return {
    subscribe,
    add(event: WebSocketEvent) {
      const eventWithId: EventWithId = {
        ...event,
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
      };
      update(events => {
        const newEvents = [eventWithId, ...events];
        return newEvents.slice(0, MAX_EVENTS);
      });
    },
    clear() {
      update(() => []);
    }
  };
}

export const events = createEventsStore();
