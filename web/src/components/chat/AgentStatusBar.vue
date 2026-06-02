<template>
  <div v-if="visible" class="agent-status-bar">
    <span class="status-dot" :class="{ pulse: isRunning }"></span>
    <span class="status-label">{{ label }}</span>
    <span v-if="toolName" class="status-tool">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 3h4l1 5H3l1-5z"/><circle cx="12" cy="12" r="2"/></svg>
      {{ toolName }}
    </span>
    <span v-if="progress" class="status-progress">{{ progress }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  visible: boolean
  isRunning?: boolean
  toolName?: string
  progress?: string
}>()

const label = computed(() => (props.isRunning ? '执行中...' : '已完成'))
</script>

<style scoped>
.agent-status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  margin: 0 16px 4px;
  background: var(--surface-card);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  font-size: 12px;
  color: var(--text-secondary);
  animation: fadeIn 0.2s ease;
}
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--brand-500);
  flex-shrink: 0;
}
.status-dot.pulse {
  animation: pulse 1.5s ease infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 rgba(6,182,212,0.4); }
  50% { opacity: 0.7; box-shadow: 0 0 0 4px rgba(6,182,212,0); }
}
.status-label {
  font-weight: 500;
}
.status-tool {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--surface-hover);
  font-family: monospace;
  font-size: 11px;
}
.status-progress {
  margin-left: auto;
  font-size: 11px;
  color: var(--text-muted);
}
</style>
