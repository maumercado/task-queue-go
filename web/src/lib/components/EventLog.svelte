<script lang="ts">
  import { events, type EventWithId } from '$lib/stores/events';

  function getEventTypeClass(type: string | undefined): string {
    if (!type) return '';
    if (type.startsWith('task.completed')) return 'success';
    if (type.startsWith('task.failed')) return 'error';
    if (type.startsWith('task.')) return 'task';
    if (type.startsWith('worker.')) return 'worker';
    if (type.startsWith('queue.')) return 'queue';
    return '';
  }

  function formatTime(timestamp: string | undefined): string {
    if (!timestamp) return '-';
    return new Date(timestamp).toLocaleTimeString();
  }

  function formatData(data: Record<string, unknown> | undefined): string {
    if (!data) return '';
    const entries = Object.entries(data).slice(0, 3);
    return entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`).join(', ');
  }
</script>

<div class="event-log card">
  <div class="event-log-header">
    <h3>Live Events</h3>
    <button onclick={() => events.clear()}>Clear</button>
  </div>

  <div class="event-list">
    {#each $events as event (event.id)}
      <div class="event-item {getEventTypeClass(event.type)}">
        <span class="event-time">{formatTime(event.timestamp)}</span>
        <span class="event-type">{event.type}</span>
        <span class="event-data">{formatData(event.data)}</span>
      </div>
    {:else}
      <div class="event-empty">Waiting for events...</div>
    {/each}
  </div>
</div>

<style>
  .event-log {
    display: flex;
    flex-direction: column;
    height: 400px;
  }

  .event-log-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .event-log-header h3 {
    margin: 0;
  }

  .event-log-header button {
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
  }

  .event-list {
    flex: 1;
    overflow-y: auto;
    font-size: 0.75rem;
    font-family: monospace;
  }

  .event-item {
    display: grid;
    grid-template-columns: 70px 140px 1fr;
    gap: 0.5rem;
    padding: 0.375rem 0.5rem;
    border-radius: 3px;
    margin-bottom: 0.25rem;
    background: rgba(255, 255, 255, 0.02);
  }

  .event-item.success {
    border-left: 3px solid var(--color-success);
  }

  .event-item.error {
    border-left: 3px solid var(--color-error);
  }

  .event-item.task {
    border-left: 3px solid var(--color-normal);
  }

  .event-item.worker {
    border-left: 3px solid var(--color-warning);
  }

  .event-item.queue {
    border-left: 3px solid var(--color-accent);
  }

  .event-time {
    color: var(--color-text-muted);
  }

  .event-type {
    color: var(--color-accent);
  }

  .event-data {
    color: var(--color-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .event-empty {
    color: var(--color-text-muted);
    text-align: center;
    padding: 2rem;
    font-family: inherit;
  }
</style>
