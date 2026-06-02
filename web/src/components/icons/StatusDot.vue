<template>
  <span class="status-dot-wrap" :title="tooltip">
    <span :class="['dot', `dot-${status}`]" :style="{ width: size + 'px', height: size + 'px' }" />
    <slot />
  </span>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  status: string
  size?: number
  tooltip?: string
}>(), { size: 8, tooltip: '' })
</script>

<style scoped>
.status-dot-wrap { display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
.dot { display: inline-block; border-radius: 50%; flex-shrink: 0; }
.dot-running, .dot-reasoning, .dot-executing, .dot-retry { background: #22c55e; }
.dot-waiting_lock, .dot-waiting_human { background: #f59e0b; animation: dot-pulse 1.2s ease-in-out infinite; }
.dot-delegate { background: #06b6d4; }
.dot-suspended, .dot-start { background: #6b7280; }
.dot-timeout { background: #ef4444; }
.dot-done { background: #22c55e; }
.dot-failed, .dot-error { background: #ef4444; }
.dot-cancel { background: #6b7280; }
@media (prefers-color-scheme: dark) {
  .dot-running, .dot-reasoning, .dot-executing, .dot-retry { background: #4ade80; }
  .dot-waiting_lock, .dot-waiting_human { background: #fbbf24; }
  .dot-delegate { background: #22d3ee; }
  .dot-suspended, .dot-start { background: #9ca3af; }
  .dot-timeout { background: #f87171; }
  .dot-done { background: #4ade80; }
  .dot-failed, .dot-error { background: #f87171; }
}
@keyframes dot-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
</style>
