<template>
  <div v-if="transitions.length > 1" class="timeline" :title="tooltip">
    <span v-for="(t, i) in displayTransitions" :key="i" class="dot" :class="t.className" :style="{ backgroundColor: t.color }" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ transitions: { to: string }[] }>()

const stateColors: Record<string, string> = {
  start: '#94a3b8', reasoning: '#3b82f6', executing: '#22c55e', waiting_lock: '#f59e0b',
  waiting_human: '#8b5cf6', delegate: '#06b6d4', suspended: '#6b7280', timeout: '#ef4444',
  retry: '#f59e0b', done: '#22c55e', failed: '#ef4444', cancel: '#6b7280',
}

const displayTransitions = computed(() => {
  const max = 8
  const list = props.transitions.slice(-max)
  return list.map(t => ({ className: `st-${t.to}`, color: stateColors[t.to] || '#94a3b8' }))
})

const tooltip = computed(() =>
  props.transitions.slice(-8).map(t => t.to).join(' → ')
)
</script>

<style scoped>
.timeline { display: flex; gap: 3px; margin-top: 4px; }
.dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; opacity: 0.7; }
.dot:last-child { opacity: 1; width: 7px; height: 7px; }
</style>
