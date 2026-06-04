<template>
  <div class="chat-event-timeline" @scroll="onScroll">
    <div v-for="group in groups" :key="group.round" class="round-group">
      <div class="round-header glass-card" @click="toggleRound(group.round)">
        <span class="round-toggle">{{ collapsed.has(group.round) ? '▶' : '▼' }}</span>
        <span class="round-label">Round {{ group.round }}</span>
        <span class="round-meta">{{ group.events.length }} 事件</span>
        <span v-if="roundDuration(group)" class="round-meta">· {{ roundDuration(group) }}</span>
        <span class="round-tool-count">{{ toolCount(group) }} 工具</span>
      </div>
      <div v-if="!collapsed.has(group.round)" class="round-events">
        <EventCard
          v-for="(ev, i) in group.events"
          :key="i"
          :event="ev"
          :index="i"
          :expanded="expandedJSON.has(i)"
          @toggle-json="$emit('toggle-json', i)"
        />
      </div>
    </div>
    <div v-if="hasMore" class="load-more">
      <button @click="$emit('load-more')">加载更多...</button>
    </div>
    <div v-if="!groups.length" class="empty-timeline">暂无匹配事件</div>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'
import type { LogEvent } from '../../types/logs'
import { formatDuration } from '../../composables/useFormat'
import EventCard from './EventCard.vue'

const props = defineProps<{
  groups: { round: number; events: LogEvent[] }[]
  expandedJSON: Set<number>
  hasMore: boolean
}>()

const emit = defineEmits<{ 'toggle-json': [index: number]; 'load-more': [] }>()

const collapsed = reactive(new Set<number>())

function toggleRound(round: number) {
  if (collapsed.has(round)) collapsed.delete(round)
  else collapsed.add(round)
}

function roundDuration(group: { events: LogEvent[] }): string {
  const end = group.events.find(e => e.type === 'round_end')
  if (end?.round_dur_ms) return formatDuration(end.round_dur_ms)
  return ''
}

function toolCount(group: { events: LogEvent[] }): number {
  return group.events.filter(e => e.type === 'tool_call' || e.type === 'tool_start').length
}

function onScroll(e: Event) {
  const el = e.target as HTMLElement
  if (!el) return
  const { scrollTop, scrollHeight, clientHeight } = el
  if (scrollHeight - scrollTop - clientHeight < 100) {
    emit('load-more')
  }
}
</script>

<style scoped>
.chat-event-timeline { flex: 1; overflow-y: auto; padding: 8px 12px 8px 24px; min-height: 0; }
.round-group { margin-bottom: 6px; }
.round-header {
  display: flex; align-items: center; gap: 8px; padding: 6px 12px;
  cursor: pointer; font-size: 12px; color: var(--text-secondary);
  border-radius: var(--radius-sm); user-select: none;
}
.round-header:hover { background: var(--surface-hover); }
.round-toggle { font-size: 9px; width: 12px; color: var(--text-muted); }
.round-label { font-weight: 600; color: var(--text-primary); }
.round-meta { font-size: 11px; color: var(--text-muted); }
.round-tool-count {
  margin-left: auto; font-size: 10px; color: var(--brand-500);
  background: color-mix(in srgb, var(--brand-500) 10%, transparent);
  padding: 1px 6px; border-radius: 8px;
}
.round-events { padding-left: 4px; }
.load-more { text-align: center; padding: 12px; }
.load-more button {
  border: 1px solid var(--border-default); background: transparent;
  padding: 6px 16px; border-radius: 6px; cursor: pointer;
  font-family: inherit; font-size: 12px; color: var(--text-secondary);
}
.empty-timeline {
  text-align: center; padding: 40px; color: var(--text-muted); font-size: 13px;
}
</style>
