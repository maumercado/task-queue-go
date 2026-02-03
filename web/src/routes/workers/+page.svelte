<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '$lib/client';
  import WorkerCard from '$lib/components/WorkerCard.svelte';
  import type { WorkerInfo } from '@task-queue/client';

  let workers = $state<WorkerInfo[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function loadWorkers() {
    try {
      const result = await client.listWorkers();
      workers = result.workers ?? [];
      error = '';
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load workers';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadWorkers();

    // Refresh every 5 seconds
    const interval = setInterval(loadWorkers, 5000);
    return () => clearInterval(interval);
  });
</script>

<svelte:head>
  <title>Workers - Task Queue</title>
</svelte:head>

<div class="workers-page">
  <div class="page-header">
    <h1>Workers</h1>
    <button onclick={loadWorkers} disabled={loading}>
      {loading ? 'Refreshing...' : 'Refresh'}
    </button>
  </div>

  {#if error}
    <div class="error-message card">{error}</div>
  {/if}

  {#if loading && workers.length === 0}
    <div class="card">Loading workers...</div>
  {:else if workers.length === 0}
    <div class="card empty-state">
      <p>No workers are currently registered.</p>
      <p class="hint">Start a worker with <code>make run-worker</code></p>
    </div>
  {:else}
    <div class="workers-grid">
      {#each workers as worker (worker.id)}
        <WorkerCard {worker} onUpdate={loadWorkers} />
      {/each}
    </div>
  {/if}

  <div class="card summary">
    <h3>Summary</h3>
    <div class="summary-stats">
      <div class="summary-stat">
        <span class="label">Total Workers</span>
        <span class="value">{workers.length}</span>
      </div>
      <div class="summary-stat">
        <span class="label">Idle</span>
        <span class="value">{workers.filter(w => w.state === 'idle').length}</span>
      </div>
      <div class="summary-stat">
        <span class="label">Busy</span>
        <span class="value">{workers.filter(w => w.state === 'busy').length}</span>
      </div>
      <div class="summary-stat">
        <span class="label">Paused</span>
        <span class="value">{workers.filter(w => w.state === 'paused').length}</span>
      </div>
    </div>
  </div>
</div>

<style>
  .workers-page {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .error-message {
    color: var(--color-error);
    background: rgba(239, 68, 68, 0.1);
  }

  .empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--color-text-muted);
  }

  .empty-state .hint {
    margin-top: 0.5rem;
    font-size: 0.875rem;
  }

  .empty-state code {
    background: var(--color-primary);
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.875rem;
  }

  .workers-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 1rem;
  }

  .summary {
    max-width: 600px;
  }

  .summary-stats {
    display: flex;
    gap: 2rem;
    flex-wrap: wrap;
  }

  .summary-stat {
    display: flex;
    flex-direction: column;
  }

  .summary-stat .label {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    text-transform: uppercase;
  }

  .summary-stat .value {
    font-size: 1.5rem;
    font-weight: 700;
  }
</style>
