<script lang="ts">
  import { client } from '$lib/client';

  let taskType = $state('example');
  let payload = $state('{}');
  let priority = $state(1);
  let scheduledAt = $state('');
  let loading = $state(false);
  let error = $state('');
  let success = $state('');

  async function handleSubmit(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';
    success = '';

    try {
      let parsedPayload: Record<string, unknown> = {};
      if (payload.trim()) {
        parsedPayload = JSON.parse(payload);
      }

      const request: Parameters<typeof client.createTask>[0] = {
        type: taskType,
        payload: parsedPayload,
        priority
      };

      if (scheduledAt) {
        request.scheduled_at = new Date(scheduledAt).toISOString();
      }

      const task = await client.createTask(request);
      success = `Task created: ${task.id}`;

      // Reset form
      taskType = 'example';
      payload = '{}';
      priority = 1;
      scheduledAt = '';
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to create task';
    } finally {
      loading = false;
    }
  }
</script>

<form class="task-form card" onsubmit={handleSubmit}>
  <h2>Create Task</h2>

  <div class="form-group">
    <label for="type">Type</label>
    <input
      id="type"
      type="text"
      bind:value={taskType}
      placeholder="e.g., email, process-image"
      required
    />
  </div>

  <div class="form-group">
    <label for="payload">Payload (JSON)</label>
    <textarea
      id="payload"
      bind:value={payload}
      rows="4"
      placeholder={'{"key": "value"}'}
    ></textarea>
  </div>

  <div class="form-group">
    <label for="priority">Priority</label>
    <select id="priority" bind:value={priority}>
      <option value={0}>Low</option>
      <option value={1}>Normal</option>
      <option value={2}>High</option>
      <option value={3}>Critical</option>
    </select>
  </div>

  <div class="form-group">
    <label for="scheduled">Schedule (optional)</label>
    <input
      id="scheduled"
      type="datetime-local"
      bind:value={scheduledAt}
    />
  </div>

  <button type="submit" disabled={loading}>
    {loading ? 'Creating...' : 'Create Task'}
  </button>

  {#if error}
    <div class="message error">{error}</div>
  {/if}

  {#if success}
    <div class="message success">{success}</div>
  {/if}
</form>

<style>
  .task-form {
    max-width: 500px;
  }

  .form-group {
    margin-bottom: 1rem;
  }

  label {
    display: block;
    margin-bottom: 0.25rem;
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  textarea {
    font-family: monospace;
    resize: vertical;
  }

  .message {
    margin-top: 1rem;
    padding: 0.5rem;
    border-radius: 4px;
    font-size: 0.875rem;
  }

  .message.error {
    background: rgba(239, 68, 68, 0.2);
    color: var(--color-error);
  }

  .message.success {
    background: rgba(74, 222, 128, 0.2);
    color: var(--color-success);
  }
</style>
