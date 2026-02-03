<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { browser } from '$app/environment';
  import '../app.css';
  import Nav from '$lib/components/Nav.svelte';
  import { client } from '$lib/client';
  import { events } from '$lib/stores/events';
  import { health } from '$lib/stores/health';

  let wsConnected = $state(false);
  let healthInterval: ReturnType<typeof setInterval> | undefined;

  async function checkHealth() {
    try {
      const result = await client.healthCheck();
      health.setHealth(result);
    } catch (err) {
      health.setError(err instanceof Error ? err.message : 'Health check failed');
    }
  }

  onMount(() => {
    if (!browser) return;

    // Initial health check
    checkHealth();

    // Poll health every 30 seconds
    healthInterval = setInterval(checkHealth, 30000);

    // Connect WebSocket
    client.connectWebSocket()
      .then(() => {
        wsConnected = true;

        client.onAnyEvent((event) => {
          events.add(event);

          // Refresh health on certain events
          if (event.type?.startsWith('queue.') || event.type?.startsWith('worker.')) {
            checkHealth();
          }
        });
      })
      .catch((err) => {
        console.error('WebSocket connection failed:', err);
      });
  });

  onDestroy(() => {
    if (browser) {
      if (healthInterval) clearInterval(healthInterval);
      client.closeWebSocket();
    }
  });

  interface Props {
    children?: import('svelte').Snippet;
  }

  let { children }: Props = $props();
</script>

<div class="app">
  <Nav />
  <main class="main">
    {@render children?.()}
  </main>
  <footer class="footer">
    <span>Task Queue Dashboard</span>
    <span class="ws-status" class:connected={wsConnected}>
      WS: {wsConnected ? 'Connected' : 'Disconnected'}
    </span>
  </footer>
</div>

<style>
  .app {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .main {
    flex: 1;
    padding: 1.5rem 2rem;
    max-width: 1400px;
    width: 100%;
    margin: 0 auto;
  }

  .footer {
    background: var(--color-surface);
    padding: 0.75rem 2rem;
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-primary);
  }

  .ws-status {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .ws-status::before {
    content: '';
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-error);
  }

  .ws-status.connected::before {
    background: var(--color-success);
  }
</style>
