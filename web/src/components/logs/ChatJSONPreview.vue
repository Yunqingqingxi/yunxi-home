<template>
  <div class="json-preview">
    <div
      v-for="(ev, i) in events"
      :key="i"
      class="json-event"
    >
      <div class="json-header" @click="toggle(i)">
        <span class="json-toggle">{{ expandedSet.has(i) ? '▼' : '▶' }}</span>
        <span class="json-type">{{ ev.type }}</span>
        <span class="json-time">{{ ev.ts }}</span>
        <span class="json-round" v-if="ev.round">Round {{ ev.round }}</span>
      </div>
      <pre v-if="expandedSet.has(i)" class="json-body">{{ formatJSON(ev) }}</pre>
    </div>
    <div v-if="!events.length" class="empty-json">暂无事件</div>
  </div>
</template>

<script setup lang="ts">
import type { LogEvent } from '../../types/logs'

defineProps<{
  events: LogEvent[]
  expandedSet: Set<number>
}>()

const emit = defineEmits<{ toggle: [index: number] }>()

function toggle(i: number) { emit('toggle', i) }

function formatJSON(ev: LogEvent): string {
  return JSON.stringify(ev, null, 2)
}
</script>

<style scoped>
.json-preview { padding: 8px 12px; }
.json-event { margin-bottom: 4px; }
.json-header {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 8px; cursor: pointer; border-radius: var(--radius-xs);
  font-size: 11px; color: var(--text-muted); font-family: var(--font-mono);
}
.json-header:hover { background: var(--surface-hover); }
.json-toggle { font-size: 9px; width: 12px; }
.json-type { font-weight: 600; color: var(--text-secondary); }
.json-time { color: var(--text-muted); }
.json-round { margin-left: auto; font-size: 10px; }
.json-body {
  margin: 0 0 4px 18px; padding: 8px 12px;
  font-family: var(--font-mono); font-size: 11px; line-height: 1.5;
  white-space: pre-wrap; word-break: break-all;
  background: var(--surface-hover); border-radius: var(--radius-sm);
  color: var(--text-secondary); max-height: 300px; overflow-y: auto;
}
.empty-json { text-align: center; padding: 40px; color: var(--text-muted); font-size: 13px; }
</style>
