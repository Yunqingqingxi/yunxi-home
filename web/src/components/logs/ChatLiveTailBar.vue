<template>
  <div class="live-tail-bar">
    <div :class="['status-indicator', status]">
      <span class="status-dot" :class="status"></span>
      <span class="status-text">{{ statusText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SSEStatus } from '../../types/logs'

const props = defineProps<{ status: SSEStatus }>()

const statusText = computed(() => {
  switch (props.status) {
    case 'connected': return '实时连接中... (SSE 已连接)'
    case 'connecting': return '正在连接...'
    case 'error': return '连接异常，正在重试...'
    case 'full': return '订阅者已满 (最多 10 个)'
    default: return '实时未连接'
  }
})
</script>

<style scoped>
.live-tail-bar {
  padding: 4px 10px; border-top: 1px solid var(--border-subtle);
  background: var(--surface-hover); flex-shrink: 0;
}
.status-indicator { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--text-muted); }
.status-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--text-muted); }
.status-dot.connected { background: #16a34a; animation: livePulse 1.5s ease infinite; }
.status-dot.connecting { background: #f59e0b; animation: livePulse 0.5s ease infinite; }
.status-dot.error { background: #dc2626; }
@keyframes livePulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
</style>
