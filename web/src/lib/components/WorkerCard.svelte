<script lang="ts">
  import type { WorkerInfo } from '@task-queue/client';
  import { client } from '$lib/client';

  interface Props {
    worker: WorkerInfo;
    onUpdate?: () => void;
  }

  let { worker, onUpdate }: Props = $props();
  let loading = $state(false);

  function getStateClass(state: string | undefined): string {
    switch (state) {
      case 'busy': return 'warning';
      case 'paused': return 'error';
      case 'shutting_down': return 'muted';
      default: return 'success';
    }
  }

  function formatDate(date: string | undefined): string {
    if (!date) return '-';
    return new Date(date).toLocaleString();
  }

  function formatRelative(date: string | undefined): string {
    if (!date) return '-';
    const now = Date.now();
    const then = new Date(date).getTime();
    const diff = now - then;

    if (diff < 1000) return 'just now';
    if (diff < 60000) return `${Math.floor(diff / 1000)}s ago`;
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    return `${Math.floor(diff / 3600000)}h ago`;
  }

  async function handlePause() {
    if (!worker.id) return;
    loading = true;
    try {
      await client.pauseWorker(worker.id);
      onUpdate?.();
    } catch (err) {
      console.error('Failed to pause worker:', err);
    } finally {
      loading = false;
    }
  }

  async function handleResume() {
    if (!worker.id) return;
    loading = true;
    try {
      await client.resumeWorker(worker.id);
      onUpdate?.();
    } catch (err) {
      console.error('Failed to resume worker:', err);
    } finally {
      loading = false;
    }
  }
</script>

<div class="worker-card card">
  <div class="worker-header">
    <span class="worker-id" title={worker.id}>{worker.id?.slice(0, 12)}...</span>
    <span class="badge {getStateClass(worker.state)}">{worker.state}</span>
  </div>

  <div class="worker-stats">
    <div class="stat">
      <span class="stat-label">Active Tasks</span>
      <span class="stat-value">{worker.active_tasks ?? 0} / {worker.concurrency ?? 0}</span>
    </div>
    <div class="stat">
      <span class="stat-label">Last Heartbeat</span>
      <span class="stat-value">{formatRelative(worker.last_heartbeat)}</span>
    </div>
  </div>

  <div class="worker-meta">
    <div class="meta-item">
      <span class="meta-label">Started:</span>
      <span>{formatDate(worker.started_at)}</span>
    </div>
    {#if worker.version}
      <div class="meta-item">
        <span class="meta-label">Version:</span>
        <span>{worker.version}</span>
      </div>
    {/if}
  </div>

  <div class="worker-actions">
    {#if worker.state === 'paused'}
      <button onclick={handleResume} disabled={loading} class="success">
        {loading ? 'Resuming...' : 'Resume'}
      </button>
    {:else if worker.state !== 'shutting_down'}
      <button onclick={handlePause} disabled={loading} class="danger">
        {loading ? 'Pausing...' : 'Pause'}
      </button>
    {/if}
  </div>
</div>

<style>
  .worker-card {
    font-size: 0.875rem;
  }

  .worker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }

  .worker-id {
    font-family: monospace;
    font-weight: 600;
  }

  .worker-stats {
    display: flex;
    gap: 2rem;
    margin-bottom: 0.75rem;
  }

  .stat {
    display: flex;
    flex-direction: column;
  }

  .stat-label {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .stat-value {
    font-size: 1rem;
    font-weight: 600;
  }

  .worker-meta {
    border-top: 1px solid var(--color-primary);
    padding-top: 0.5rem;
    margin-bottom: 0.75rem;
    font-size: 0.75rem;
  }

  .meta-item {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.25rem;
  }

  .meta-label {
    color: var(--color-text-muted);
  }

  .worker-actions {
    display: flex;
    gap: 0.5rem;
  }

  .worker-actions button {
    flex: 1;
  }
</style>
