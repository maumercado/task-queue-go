<script lang="ts">
  import { page } from '$app/stores';

  const navItems = [
    { href: '/', label: 'Dashboard' },
    { href: '/tasks', label: 'Tasks' },
    { href: '/workers', label: 'Workers' },
    { href: '/dlq', label: 'Dead Letter Queue' }
  ];
</script>

<nav class="nav">
  <div class="nav-brand">Task Queue</div>
  <ul class="nav-links">
    {#each navItems as item}
      <li>
        <a
          href={item.href}
          class:active={$page.url.pathname === item.href}
        >
          {item.label}
        </a>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .nav {
    background: var(--color-surface);
    padding: 1rem 2rem;
    display: flex;
    align-items: center;
    gap: 2rem;
    border-bottom: 1px solid var(--color-primary);
  }

  .nav-brand {
    font-weight: 700;
    font-size: 1.25rem;
    color: var(--color-accent);
  }

  .nav-links {
    display: flex;
    list-style: none;
    gap: 1rem;
  }

  .nav-links a {
    color: var(--color-text-muted);
    padding: 0.5rem 1rem;
    border-radius: 4px;
    transition: background 0.2s, color 0.2s;
  }

  .nav-links a:hover {
    color: var(--color-text);
    text-decoration: none;
    background: var(--color-primary);
  }

  .nav-links a.active {
    color: var(--color-text);
    background: var(--color-primary);
  }
</style>
