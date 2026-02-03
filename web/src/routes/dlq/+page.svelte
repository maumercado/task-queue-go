<script lang="ts">
  import { onMount } from 'svelte';
  import { client } from '$lib/client';
  import type { DlqEntry } from '@task-queue/client';

  let entries = $state<DlqEntry[]>([]);
  let loading = $state(true);
  let error = $state('');
  let actionLoading = $state(false);

  async function loadDlq() {
    try {
      const result = await client.listDlq();
      entries = result.entries ?? [];
      error = '';
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load DLQ';
    } finally {
      loading = false;
    }
  }

  async function retryEntry(taskId: string) {
    actionLoading = true;
    try {
      await client.retryDlqTask(taskId);
      await loadDlq();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to retry task';
    } finally {
      actionLoading = false;
    }
  }

  async function retryAll() {
    if (!confirm(`Are you sure you want to retry all ${entries.length} entries?`)) return;

    actionLoading = true;
    try {
      const count = await client.retryAllDlqTasks();
      alert(`Retried ${count} tasks`);
      await loadDlq();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to retry all tasks';
    } finally {
      actionLoading = false;
    }
  }

  async function clearDlq() {
    if (!confirm('Are you sure you want to clear the entire DLQ? This cannot be undone.')) return;

    actionLoading = true;
    try {
      await client.clearDlq();
      await loadDlq();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to clear DLQ';
    } finally {
      actionLoading = false;
    }
  }

  function formatDate(date: string | undefined): string {
    if (!date) return '-';
    return new Date(date).toLocaleString();
  }

  onMount(() => {
    loadDlq();

    // Refresh every 10 seconds
    const interval = setInterval(loadDlq, 10000);
    return () => clearInterval(interval);
  });
</script>

<svelte:head>
  <title>Dead Letter Queue - Task Queue</title>
</svelte:head>

<div class="dlq-page">
  <div class="page-header">
    <h1>Dead Letter Queue</h1>
    <div class="header-actions">
      <button onclick={loadDlq} disabled={loading || actionLoading}>
        Refresh
      </button>
      {#if entries.length > 0}
        <button onclick={retryAll} disabled={actionLoading} class="success">
          Retry All ({entries.length})
        </button>
        <button onclick={clearDlq} disabled={actionLoading} class="danger">
          Clear DLQ
        </button>
      {/if}
    </div>
  </div>

  {#if error}
    <div class="error-message card">{error}</div>
  {/if}

  {#if loading && entries.length === 0}
    <div class="card">Loading DLQ entries...</div>
  {:else if entries.length === 0}
    <div class="card empty-state">
      <p>The Dead Letter Queue is empty.</p>
      <p class="hint">Tasks that fail all retries will appear here.</p>
    </div>
  {:else}
    <div class="dlq-list">
      {#each entries as entry (entry.task_id)}
        <div class="dlq-entry card">
          <div class="entry-header">
            <span class="task-id" title={entry.task_id}>
              {entry.task_id?.slice(0, 12)}...
            </span>
            <span class="task-type">{entry.task_type}</span>
          </div>

          <div class="entry-reason">
            <span class="label">Reason:</span>
            <code>{entry.reason}</code>
          </div>

          <div class="entry-footer">
            <span class="added-at">Added: {formatDate(entry.added_at)}</span>
            <button
              onclick={() => entry.task_id && retryEntry(entry.task_id)}
              disabled={actionLoading}
            >
              Retry
            </button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <div class="card info">
    <h3>About Dead Letter Queue</h3>
    <p>
      The DLQ contains tasks that have exhausted all retry attempts. You can:
    </p>
    <ul>
      <li><strong>Retry</strong> individual tasks to re-queue them</li>
      <li><strong>Retry All</strong> to bulk re-queue all failed tasks</li>
      <li><strong>Clear DLQ</strong> to permanently remove all entries</li>
    </ul>
  </div>
</div>

<style>
  .dlq-page {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }

  .header-actions {
    display: flex;
    gap: 0.5rem;
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

  .dlq-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .dlq-entry {
    font-size: 0.875rem;
  }

  .entry-header {
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
  }

  .entry-reason {
    background: rgba(239, 68, 68, 0.1);
    padding: 0.5rem;
    border-radius: 4px;
    margin-bottom: 0.75rem;
  }

  .entry-reason .label {
    color: var(--color-text-muted);
    margin-right: 0.5rem;
  }

  .entry-reason code {
    color: var(--color-error);
    font-size: 0.75rem;
    word-break: break-all;
  }

  .entry-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-top: 1px solid var(--color-primary);
    padding-top: 0.5rem;
  }

  .added-at {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .info {
    max-width: 600px;
    font-size: 0.875rem;
  }

  .info ul {
    margin-top: 0.5rem;
    padding-left: 1.5rem;
  }

  .info li {
    margin-bottom: 0.25rem;
  }
</style>
