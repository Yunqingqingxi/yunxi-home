<template>
  <div class="agent-bubble" :class="status">
    <div class="agent-bubble-head">
      <span class="agent-icon">{{ icon }}</span>
      <span class="agent-label">子 Agent</span>
      <span v-if="round" class="agent-round">R{{ round }}</span>
      <span class="agent-status-text">{{ statusText }}</span>
    </div>
    <div class="agent-bubble-goal">{{ goal }}</div>
    <div v-if="summary" class="agent-bubble-summary">{{ summary }}</div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { computed } from 'vue'

const props = defineProps({
  agentId: { type: String, default: '' },
  goal: { type: String, default: '' },
  status: { type: String, default: 'running' },
  round: { type: Number, default: 0 },
  summary: { type: String, default: '' },
})

const icon = computed(() => {
  switch (props.status) {
    case 'running': return '⟳'
    case 'done': return '✓'
    case 'error': return '✗'
    default: return '⋯'
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
  max-width: 82%;
  border: 1px solid var(--border-default);
  border-radius: 10px; padding: 10px 14px;
  background: var(--surface-card);
  transition: all 0.3s;
  animation: agentIn 0.25s ease-out;
}
@keyframes agentIn {
  from { opacity: 0; transform: translateY(-6px); }
  to { opacity: 1; transform: translateY(0); }
}

.agent-bubble-head {
  display: flex; align-items: center; gap: 6px; margin-bottom: 4px;
}
.agent-icon { font-size: 13px; width: 16px; text-align: center; flex-shrink: 0; }
.agent-label {
  font-size: 10.5px; font-weight: 600; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.5px;
}
.agent-round {
  font-size: 9.5px; color: var(--text-muted); background: var(--surface-hover);
  padding: 1px 6px; border-radius: 3px;
}
.agent-status-text {
  font-size: 10px; color: var(--text-muted); margin-left: auto;
}

.agent-bubble.running {
  border-color: rgba(6,182,212,0.25);
  background: rgba(6,182,212,0.03);
}
.agent-bubble.running .agent-icon { color: var(--brand-500); animation: spin 1.4s linear infinite; }
.agent-bubble.running .agent-status-text { color: var(--brand-500); }

.agent-bubble.done {
  border-color: rgba(34,197,94,0.2);
  background: rgba(34,197,94,0.03);
}
.agent-bubble.done .agent-icon { color: #22c55e; }
.agent-bubble.done .agent-status-text { color: #22c55e; }

.agent-bubble.error {
  border-color: rgba(239,68,68,0.2);
  background: rgba(239,68,68,0.03);
}
.agent-bubble.error .agent-icon { color: var(--color-danger); }
.agent-bubble.error .agent-status-text { color: var(--color-danger); }

.agent-bubble-goal {
  font-size: 12.5px; color: var(--text-primary); line-height: 1.4;
}
.agent-bubble-summary {
  margin-top: 6px; padding-top: 6px;
  border-top: 1px solid var(--border-subtle);
  font-size: 11px; color: var(--text-secondary); line-height: 1.5;
  white-space: pre-wrap; word-break: break-word;
}

@keyframes spin { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }

[data-theme="dark"] .agent-bubble.running { background: rgba(34,211,238,0.06); }
[data-theme="dark"] .agent-bubble.done { background: rgba(34,197,94,0.06); }
[data-theme="dark"] .agent-bubble.error { background: rgba(239,68,68,0.06); }
</style>
