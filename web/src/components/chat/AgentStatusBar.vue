<template>
  <div v-if="visible" class="agent-status-bar">
    <span class="bar-label">助手 · {{ total }}</span>
    <span class="bar-segments">
      <span v-for="(entry, i) in segments" :key="i"
        class="bar-seg" :style="{ width: entry.width + '%', backgroundColor: entry.color }"
        :title="entry.label"
      />
    </span>
    <span class="bar-tool" v-if="toolName">{{ toolName }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  visible: boolean
  agents?: any[]
  toolName?: string
}>()

const total = computed(() => (props.agents || []).length)

const stateColor: Record<string, string> = { reasoning: '#3b82f6', executing: '#22c55e', waiting_lock: '#f59e0b', waiting_human: '#8b5cf6', delegate: '#06b6d4', done: '#22c55e', failed: '#ef4444', retry: '#f59e0b', running: '#22c55e' }

const segments = computed(() => {
  const agents = props.agents || []
  if (!agents.length) return []
  const counts: Record<string, number> = {}
  for (const a of agents) { const s = a.state || a.agent_status || 'running'; counts[s] = (counts[s] || 0) + 1 }
  return Object.entries(counts).map(([s, c]) => ({
    width: (c / agents.length) * 100,
    color: stateColor[s] || '#94a3b8',
    label: `${s}: ${c}`
  }))
})
</script>

<style scoped>
.agent-status-bar {
  display: flex; align-items: center; gap: 8px; padding: 5px 12px; margin: 0 16px 4px;
  background: var(--surface-card, #fff); border: 1px solid var(--border-subtle, #e2e8f0);
  border-radius: 8px; font-size: 11px; color: var(--text-secondary);
}
.bar-label { font-weight: 600; white-space: nowrap; }
.bar-segments { display: flex; height: 5px; border-radius: 3px; overflow: hidden; flex: 1; gap: 1px; }
.bar-seg { border-radius: 1px; transition: width 0.5s ease; }
.bar-tool { font-family: monospace; font-size: 10px; color: var(--text-muted); margin-left: auto; white-space: nowrap; }
@media (prefers-color-scheme: dark) { .agent-status-bar { border-color: #334155; background: #1e293b; } }
</style>
