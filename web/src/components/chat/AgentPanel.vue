<template>
  <div v-if="agents.length > 0" class="agent-panel">
    <div
      v-for="agent in visibleAgents"
      :key="agent.agent_id"
      class="agent-card"
      :class="agent.status"
    >
      <div class="agent-header">
        <span class="agent-status-icon">{{ statusIcon(agent.status || agent.agent_status) }}</span>
        <span class="agent-goal">{{ truncate(agent.goal || agent.agent_goal, 60) }}</span>
        <span v-if="agent.agent_round" class="agent-round">R{{ agent.agent_round }}</span>
      </div>
      <div v-if="agent.summary || agent.content" class="agent-result">
        {{ truncate(agent.summary || agent.content, 120) }}
      </div>
    </div>
    <div v-if="hiddenCount > 0" class="agent-more">
      +{{ hiddenCount }} 个更多子任务
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { computed } from 'vue'

const props = defineProps({
  agents: { type: Array, default: () => [] }
})

const maxVisible = 5

const visibleAgents = computed(() => props.agents.slice(0, maxVisible))
const hiddenCount = computed(() => Math.max(0, props.agents.length - maxVisible))

function statusIcon(status) {
  switch (status) {
    case 'running':
    case 'pending': return '⟳'
    case 'done':    return '✅'
    case 'error':   return '❌'
    default:        return '⏳'
  }
}

function truncate(s, n) {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}
</script>

<style scoped>
.agent-panel {
  display: flex; flex-direction: column; gap: 4px;
  margin: 4px 0; flex-shrink: 0;
}
.agent-card {
  border: 1px solid var(--border-default);
  border-radius: 8px; padding: 8px 10px;
  background: var(--surface-card);
  transition: all 0.3s var(--ease-out-expo);
  animation: agentIn 0.3s var(--ease-out-back);
}
@keyframes agentIn {
  from { opacity: 0; transform: translateX(-8px) scale(0.96); }
  to   { opacity: 1; transform: translateX(0) scale(1); }
}
.agent-card.running, .agent-card.pending {
  border-color: rgba(6,182,212,0.25);
  background: rgba(6,182,212,0.04);
  animation: agentPulse 2.2s ease-in-out infinite;
}
@keyframes agentPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(6,182,212,0.08); }
  50%      { box-shadow: 0 0 0 5px rgba(6,182,212,0); }
}
.agent-card.done {
  border-color: rgba(34,197,94,0.25);
  background: rgba(34,197,94,0.04);
}
.agent-card.error {
  border-color: rgba(239,68,68,0.25);
  background: rgba(239,68,68,0.04);
}

.agent-header {
  display: flex; align-items: center; gap: 6px;
}
.agent-status-icon { font-size: 12px; flex-shrink: 0; }
.agent-card.running .agent-status-icon { animation: spin 1.4s linear infinite; }
.agent-card.pending .agent-status-icon { animation: spin 1.4s linear infinite; }
@keyframes spin { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }

.agent-goal {
  font-size: 11.5px; color: var(--text-primary);
  flex: 1; line-height: 1.4;
}
.agent-round {
  font-size: 9px; color: var(--text-muted);
  background: var(--surface-hover);
  padding: 1px 6px; border-radius: 4px;
  font-variant-numeric: tabular-nums;
}
.agent-result {
  margin-top: 4px; padding-top: 4px;
  border-top: 1px solid var(--border-subtle);
  font-size: 10.5px; color: var(--text-secondary);
  line-height: 1.4;
}
.agent-more {
  font-size: 10px; color: var(--text-muted);
  text-align: center; padding: 4px;
}

[data-theme="dark"] .agent-card.running {
  background: rgba(34,211,238,0.06);
}
[data-theme="dark"] .agent-card.done {
  background: rgba(34,197,94,0.06);
}
</style>
