<template>
  <div class="agent-bubble" :class="status">
    <div class="agent-bubble-head">
      <AgentStateIcon :state="displayState" :size="16" />
      <span class="agent-label">子 Agent</span>
      <span v-if="round" class="agent-round">R{{ round }}</span>
      <span class="agent-status-text">{{ statusText }}</span>
    </div>
    <div class="agent-bubble-goal">{{ goal }}</div>
    <div v-if="summary" class="agent-bubble-summary">{{ summary }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AgentStateIcon from '../icons/AgentStateIcon.vue'

const props = defineProps<{
  agentId?: string; goal?: string; status?: string; round?: number; summary?: string
}>()

const displayState = computed(() => {
  switch (props.status) {
    case 'running': return 'executing' as const
    case 'done': return 'done' as const
    case 'error': return 'failed' as const
    default: return 'executing' as const
  }
})

const statusText = computed(() => {
  switch (props.status) {
    case 'running': return '执行中'
    case 'done': return '完成'
    case 'error': return '失败'
    default: return '等待中'
  }
})
</script>

<style scoped>
.agent-bubble {
  max-width: 82%; border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px;
  padding: 10px 14px; background: var(--surface-card, #fff);
  animation: agentIn 0.25s ease-out;
}
@keyframes agentIn { from { opacity: 0; transform: translateY(-6px); } to { opacity: 1; transform: translateY(0); } }
.agent-bubble-head { display: flex; align-items: center; gap: 6px; margin-bottom: 4px; }
.agent-label { font-size: 10.5px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; }
.agent-round { font-size: 9.5px; color: var(--text-muted); background: var(--surface-hover); padding: 1px 6px; border-radius: 3px; }
.agent-status-text { font-size: 10px; color: var(--text-muted); margin-left: auto; }
.agent-bubble.running { border-color: rgba(6,182,212,0.25); }
.agent-bubble.done { opacity: 0.7; }
.agent-bubble.error { border-color: rgba(239,68,68,0.2); }
.agent-bubble-goal { font-size: 12.5px; color: var(--text-primary); line-height: 1.4; }
.agent-bubble-summary { margin-top: 6px; padding-top: 6px; border-top: 1px solid var(--border-subtle); font-size: 11px; color: var(--text-secondary); line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
@media (prefers-color-scheme: dark) { .agent-bubble { border-color: #334155; background: #1e293b; } }
</style>
