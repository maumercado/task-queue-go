<script lang="ts">
  import { client } from '$lib/client';
  import TaskForm from '$lib/components/TaskForm.svelte';
  import TaskCard from '$lib/components/TaskCard.svelte';
  import type { TaskResponse } from '@task-queue/client';

  let taskId = $state('');
  let task = $state<TaskResponse | null>(null);
  let loading = $state(false);
  let error = $state('');

  async function lookupTask() {
    if (!taskId.trim()) return;

    loading = true;
    error = '';
    task = null;

    try {
      task = await client.getTask(taskId.trim());
    } catch (err) {
      error = err instanceof Error ? err.message : 'Task not found';
    } finally {
      loading = false;
    }
  }

  async function cancelTask() {
    if (!task?.id) return;

    loading = true;
    error = '';

    try {
      task = await client.cancelTask(task.id);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to cancel task';
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      lookupTask();
    }
  }
</script>

<svelte:head>
  <title>Tasks - Task Queue</title>
</svelte:head>

<div class="tasks-page">
  <h1>Tasks</h1>

  <div class="tasks-grid">
    <!-- Create Task Form -->
    <div class="create-section">
      <TaskForm />
    </div>

    <!-- Task Lookup -->
    <div class="lookup-section">
      <div class="card">
        <h2>Lookup Task</h2>

        <div class="lookup-form">
          <input
            type="text"
            placeholder="Enter task ID..."
            bind:value={taskId}
            onkeydown={handleKeydown}
          />
          <button onclick={lookupTask} disabled={loading || !taskId.trim()}>
            {loading ? 'Loading...' : 'Lookup'}
          </button>
        </div>

        {#if error}
          <div class="error-message">{error}</div>
        {/if}
      </div>

      {#if task}
        <TaskCard {task} />
        {#if task.state === 'pending' || task.state === 'scheduled' || task.state === 'running'}
          <button class="danger" onclick={cancelTask} disabled={loading}>
            Cancel Task
          </button>
        {/if}
      {/if}
    </div>
  </div>
</div>

<style>
  .tasks-page {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .tasks-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
    gap: 2rem;
  }

  .lookup-form {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }

  .lookup-form input {
    flex: 1;
  }

  .lookup-form button {
    white-space: nowrap;
  }

  .error-message {
    color: var(--color-error);
    font-size: 0.875rem;
    padding: 0.5rem;
    background: rgba(239, 68, 68, 0.1);
    border-radius: 4px;
  }

  .lookup-section button.danger {
    width: 100%;
    margin-top: 0.5rem;
  }
</style>
