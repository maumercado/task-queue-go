import { writable } from 'svelte/store';
import type { HealthResponse } from '@task-queue/client';

export interface HealthState extends HealthResponse {
  loading: boolean;
  error?: string;
}

function createHealthStore() {
  const { subscribe, set, update } = writable<HealthState>({
    status: undefined,
    redis: undefined,
    loading: true
  });

  return {
    subscribe,
    setLoading(loading: boolean) {
      update(state => ({ ...state, loading }));
    },
    setHealth(health: HealthResponse) {
      set({
        ...health,
        loading: false,
        error: undefined
      });
    },
    setError(error: string) {
      update(state => ({
        ...state,
        loading: false,
        error
      }));
    }
  };
}

export const health = createHealthStore();
