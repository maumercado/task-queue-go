<script lang="ts">
  import { events, type EventWithId } from '$lib/stores/events';

  type FilterType = 'all' | 'task' | 'worker' | 'queue' | 'error';
  let activeFilter = $state<FilterType>('all');

  const filters: { label: string; value: FilterType }[] = [
    { label: 'All', value: 'all' },
    { label: 'Tasks', value: 'task' },
    { label: 'Workers', value: 'worker' },
    { label: 'Queue', value: 'queue' },
    { label: 'Errors', value: 'error' },
  ];

  function matchesFilter(event: EventWithId, filter: FilterType): boolean {
    const t = event.type ?? '';
    switch (filter) {
      case 'task':    return t.startsWith('task.');
      case 'worker':  return t.startsWith('worker.');
      case 'queue':   return t.startsWith('queue.');
      case 'error':   return t === 'task.failed' || t === 'task.retrying';
      default:        return true;
    }
  }

  const filtered = $derived($events.filter(e => matchesFilter(e, activeFilter)));

  function getBadgeClass(type: string | undefined): string {
    if (!type) return 'badge-default';
    if (type === 'task.completed')  return 'badge-success';
    if (type === 'task.failed')     return 'badge-error';
    if (type === 'task.retrying')   return 'badge-warn';
    if (type === 'task.started')    return 'badge-info';
    if (type === 'task.submitted')  return 'badge-accent';
    if (type.startsWith('worker.')) return 'badge-worker';
    if (type.startsWith('queue.'))  return 'badge-queue';
    return 'badge-default';
  }

  function formatTime(timestamp: string | undefined): string {
    if (!timestamp) return '-';
    return new Date(timestamp).toLocaleTimeString();
  }

  function formatData(data: Record<string, unknown> | undefined): string {
    if (!data) return '';
    const keys: string[] = [];
    if (data.task_id)      keys.push(`id: ${String(data.task_id).slice(0, 8)}…`);
    if (data.type)         keys.push(`type: ${data.type}`);
    if (data.priority)     keys.push(`pri: ${data.priority}`);
    if (data.attempts !== undefined) keys.push(`#${data.attempts}`);
    if (data.worker_id)    keys.push(`w: ${String(data.worker_id).slice(0, 8)}…`);
    if (data.duration_ms !== undefined) keys.push(`${data.duration_ms}ms`);
    if (data.error)        keys.push(`err: ${String(data.error).slice(0, 30)}`);
    if (data.next_retry_at) {
      const d = new Date(data.next_retry_at as string);
      keys.push(`retry@: ${d.toLocaleTimeString()}`);
    }
    return keys.join('  ·  ');
  }
</script>

<div class="event-log card">
  <div class="event-log-header">
    <h3>Live Events <span class="count">{filtered.length}</span></h3>
    <div class="header-actions">
      <div class="filter-tabs">
        {#each filters as f}
          <button
            class="filter-tab"
            class:active={activeFilter === f.value}
            onclick={() => activeFilter = f.value}
          >{f.label}</button>
        {/each}
      </div>
      <button class="clear-btn" onclick={() => events.clear()}>Clear</button>
    </div>
  </div>

  <div class="event-list">
    {#each filtered as event (event.id)}
      <div class="event-item">
        <span class="event-time">{formatTime(event.timestamp)}</span>
        <span class="event-badge {getBadgeClass(event.type)}">{event.type}</span>
        <span class="event-data">{formatData(event.data)}</span>
      </div>
    {:else}
      <div class="event-empty">Waiting for events…</div>
    {/each}
  </div>
</div>

<style>
  .event-log {
    display: flex;
    flex-direction: column;
    height: 420px;
  }

  .event-log-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .event-log-header h3 {
    margin: 0;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .count {
    font-size: 0.75rem;
    font-weight: 400;
    color: var(--color-text-muted);
    background: rgba(255,255,255,0.05);
    padding: 0.1rem 0.4rem;
    border-radius: 99px;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .filter-tabs {
    display: flex;
    gap: 2px;
  }

  .filter-tab {
    padding: 0.2rem 0.5rem;
    font-size: 0.7rem;
    border-radius: 4px;
    border: 1px solid var(--color-primary);
    background: transparent;
    color: var(--color-text-muted);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .filter-tab.active {
    background: var(--color-accent);
    color: #fff;
    border-color: var(--color-accent);
  }

  .clear-btn {
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
  }

  .event-list {
    flex: 1;
    overflow-y: auto;
    font-size: 0.72rem;
    font-family: monospace;
  }

  .event-item {
    display: grid;
    grid-template-columns: 65px 160px 1fr;
    gap: 0.4rem;
    padding: 0.3rem 0.4rem;
    border-radius: 3px;
    margin-bottom: 2px;
    background: rgba(255, 255, 255, 0.02);
    align-items: center;
  }

  .event-item:hover {
    background: rgba(255, 255, 255, 0.05);
  }

  .event-time {
    color: var(--color-text-muted);
  }

  .event-badge {
    border-radius: 4px;
    padding: 1px 5px;
    font-size: 0.68rem;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .badge-success  { background: rgba(74,222,128,0.15); color: #4ade80; }
  .badge-error    { background: rgba(239,68,68,0.15);  color: #ef4444; }
  .badge-warn     { background: rgba(251,191,36,0.15); color: #fbbf24; }
  .badge-info     { background: rgba(99,179,237,0.15); color: #63b3ed; }
  .badge-accent   { background: rgba(139,92,246,0.15); color: #8b5cf6; }
  .badge-worker   { background: rgba(251,146,60,0.15); color: #fb923c; }
  .badge-queue    { background: rgba(34,211,238,0.15); color: #22d3ee; }
  .badge-default  { background: rgba(148,163,184,0.1); color: #94a3b8; }

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
