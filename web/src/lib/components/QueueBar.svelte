<script lang="ts">
  interface Props {
    label: string;
    value: number;
    maxValue: number;
    color: string;
  }

  let { label, value, maxValue, color }: Props = $props();

  function getPercentage(val: number, max: number): number {
    if (max === 0) return 0;
    return Math.min((val / max) * 100, 100);
  }
</script>

<div class="queue-bar">
  <div class="queue-bar-header">
    <span class="queue-bar-label">{label}</span>
    <span class="queue-bar-value">{value.toLocaleString()}</span>
  </div>
  <div class="queue-bar-track">
    <div
      class="queue-bar-fill"
      style="width: {getPercentage(value, maxValue)}%; background: {color};"
    ></div>
  </div>
</div>

<style>
  .queue-bar {
    margin-bottom: 0.75rem;
  }

  .queue-bar-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 0.25rem;
    font-size: 0.875rem;
  }

  .queue-bar-label {
    text-transform: capitalize;
    font-weight: 500;
  }

  .queue-bar-value {
    font-family: monospace;
    color: var(--color-text-muted);
  }

  .queue-bar-track {
    height: 8px;
    background: var(--color-primary);
    border-radius: 4px;
    overflow: hidden;
  }

  .queue-bar-fill {
    height: 100%;
    border-radius: 4px;
    transition: width 0.3s ease;
  }
</style>
