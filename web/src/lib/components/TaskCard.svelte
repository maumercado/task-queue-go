<script lang="ts">
  import type { TaskResponse } from '@task-queue/client';

  interface Props {
    task: TaskResponse;
  }

  let { task }: Props = $props();

  function getStateClass(state: string | undefined): string {
    switch (state) {
      case 'completed': return 'success';
      case 'failed':
      case 'dead_letter': return 'error';
      case 'running': return 'warning';
      default: return 'muted';
    }
  }

  function getPriorityClass(priority: string | undefined): string {
    switch (priority) {
      case 'critical': return 'critical';
      case 'high': return 'high';
      case 'normal': return 'normal';
      case 'low': return 'low';
      default: return '';
    }
  }

  function formatDate(date: string | undefined): string {
    if (!date) return '-';
    return new Date(date).toLocaleString();
  }
</script>

<div class="task-card card">
  <div class="task-header">
    <span class="task-id" title={task.id}>{task.id?.slice(0, 8)}...</span>
    <span class="badge {getStateClass(task.state)}">{task.state}</span>
  </div>

  <div class="task-type">{task.type}</div>

  <div class="task-meta">
    <div class="meta-item">
      <span class="meta-label">Priority:</span>
      <span class="priority-badge {getPriorityClass(task.priority)}">{task.priority}</span>
    </div>
    <div class="meta-item">
      <span class="meta-label">Attempts:</span>
      <span>{task.attempts ?? 0} / {task.max_retries ?? 3}</span>
    </div>
    {#if task.worker_id}
      <div class="meta-item">
        <span class="meta-label">Worker:</span>
        <span class="worker-id">{task.worker_id.slice(0, 8)}...</span>
      </div>
    {/if}
  </div>

  {#if task.error}
    <div class="task-error">
      <span class="meta-label">Error:</span>
      <code>{task.error}</code>
    </div>
  {/if}

  <div class="task-times">
    <div class="time-item">
      <span class="meta-label">Created:</span>
      <span>{formatDate(task.created_at)}</span>
    </div>
    {#if task.started_at}
      <div class="time-item">
        <span class="meta-label">Started:</span>
        <span>{formatDate(task.started_at)}</span>
      </div>
    {/if}
    {#if task.completed_at}
      <div class="time-item">
        <span class="meta-label">Completed:</span>
        <span>{formatDate(task.completed_at)}</span>
      </div>
    {/if}
  </div>
</div>

<style>
  .task-card {
    font-size: 0.875rem;
  }

  .task-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
  }

  .task-id {
    font-family: monospace;
    color: var(--color-text-muted);
  }

  .task-type {
    font-weight: 600;
    margin-bottom: 0.75rem;
  }

  .task-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }

  .meta-item {
    display: flex;
    gap: 0.25rem;
    align-items: center;
  }

  .meta-label {
    color: var(--color-text-muted);
  }

  .priority-badge {
    padding: 0.125rem 0.375rem;
    border-radius: 3px;
    font-size: 0.75rem;
    text-transform: uppercase;
  }

  .priority-badge.critical { background: var(--color-critical); }
  .priority-badge.high { background: var(--color-high); }
  .priority-badge.normal { background: var(--color-normal); }
  .priority-badge.low { background: var(--color-low); }

  .worker-id {
    font-family: monospace;
    font-size: 0.75rem;
  }

  .task-error {
    background: rgba(239, 68, 68, 0.1);
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.75rem;
  }

  .task-error code {
    color: var(--color-error);
    font-size: 0.75rem;
    word-break: break-all;
  }

  .task-times {
    border-top: 1px solid var(--color-primary);
    padding-top: 0.5rem;
    font-size: 0.75rem;
  }

  .time-item {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.25rem;
  }
</style>
