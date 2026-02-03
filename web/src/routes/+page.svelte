<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '$lib/client';
  import { health } from '$lib/stores/health';
  import QueueBar from '$lib/components/QueueBar.svelte';
  import EventLog from '$lib/components/EventLog.svelte';
  import type { QueueStats, WorkerListResponse } from '@task-queue/client';

  let queueStats = $state<QueueStats | null>(null);
  let workerStats = $state<WorkerListResponse | null>(null);
  let loading = $state(true);

  const QUEUE_COLORS = {
    critical: 'var(--color-critical)',
    high: 'var(--color-high)',
    normal: 'var(--color-normal)',
    low: 'var(--color-low)'
  };

  async function loadStats() {
    try {
      const [queue, workers] = await Promise.all([
        client.getQueueStats(),
        client.listWorkers()
      ]);
      queueStats = queue;
      workerStats = workers;
    } catch (err) {
      console.error('Failed to load stats:', err);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadStats();

    // Refresh stats every 5 seconds
    const interval = setInterval(loadStats, 5000);
    return () => clearInterval(interval);
  });

  function getMaxQueueDepth(depths: QueueStats['queue_depths']): number {
    if (!depths) return 100;
    const max = Math.max(
      depths.critical ?? 0,
      depths.high ?? 0,
      depths.normal ?? 0,
      depths.low ?? 0
    );
    return Math.max(max, 100);
  }

  function getActiveWorkerCount(workers: WorkerListResponse | null): number {
    if (!workers?.workers) return 0;
    return workers.workers.filter(w => w.state !== 'paused' && w.state !== 'shutting_down').length;
  }
</script>

<svelte:head>
  <title>Dashboard - Task Queue</title>
</svelte:head>

<div class="dashboard">
  <h1>Dashboard</h1>

  <div class="stats-grid">
    <!-- Health Status -->
    <div class="card health-card">
      <h3>System Health</h3>
      {#if $health.loading}
        <div class="status-indicator loading">Checking...</div>
      {:else if $health.error}
        <div class="status-indicator unhealthy">Error</div>
        <p class="error-text">{$health.error}</p>
      {:else}
        <div class="status-indicator {$health.status}">
          {$health.status === 'healthy' ? 'Healthy' : 'Unhealthy'}
        </div>
        <div class="health-details">
          <span>Redis: {$health.redis}</span>
        </div>
      {/if}
    </div>

    <!-- Quick Stats -->
    <div class="card quick-stats">
      <h3>Quick Stats</h3>
      {#if loading}
        <p>Loading...</p>
      {:else}
        <div class="stat-row">
          <span class="stat-label">Total Pending</span>
          <span class="stat-value">{queueStats?.total_pending?.toLocaleString() ?? 0}</span>
        </div>
        <div class="stat-row">
          <span class="stat-label">Active Workers</span>
          <span class="stat-value">{getActiveWorkerCount(workerStats)} / {workerStats?.count ?? 0}</span>
        </div>
      {/if}
    </div>
  </div>

  <!-- Queue Depths -->
  <div class="card queue-depths">
    <h3>Queue Depths</h3>
    {#if loading}
      <p>Loading...</p>
    {:else if queueStats?.queue_depths}
      {#each ['critical', 'high', 'normal', 'low'] as priority}
        <QueueBar
          label={priority}
          value={queueStats.queue_depths[priority as keyof typeof queueStats.queue_depths] ?? 0}
          maxValue={getMaxQueueDepth(queueStats.queue_depths)}
          color={QUEUE_COLORS[priority as keyof typeof QUEUE_COLORS]}
        />
      {/each}
    {/if}
  </div>

  <!-- Event Log -->
  <EventLog />
</div>

<style>
  .dashboard {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 1rem;
  }

  .health-card {
    text-align: center;
  }

  .status-indicator {
    font-size: 1.5rem;
    font-weight: 700;
    padding: 0.5rem 1rem;
    border-radius: 8px;
    margin: 0.5rem 0;
  }

  .status-indicator.healthy {
    color: var(--color-success);
    background: rgba(74, 222, 128, 0.1);
  }

  .status-indicator.unhealthy,
  .status-indicator.error {
    color: var(--color-error);
    background: rgba(239, 68, 68, 0.1);
  }

  .status-indicator.loading {
    color: var(--color-text-muted);
    background: rgba(136, 136, 136, 0.1);
  }

  .health-details {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .error-text {
    color: var(--color-error);
    font-size: 0.875rem;
  }

  .quick-stats .stat-row {
    display: flex;
    justify-content: space-between;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--color-primary);
  }

  .quick-stats .stat-row:last-child {
    border-bottom: none;
  }

  .stat-label {
    color: var(--color-text-muted);
  }

  .stat-value {
    font-weight: 600;
    font-family: monospace;
  }

  .queue-depths {
    max-width: 600px;
  }
</style>
