<template>
  <div class="chat-event-filter-bar">
    <div class="filter-row">
      <div class="filter-chips">
        <label
          v-for="entry in eventEntries"
          :key="entry.type"
          class="filter-chip"
          :style="{ '--chip-color': entry.color }"
        >
          <input
            type="checkbox"
            :checked="checked[entry.type]"
            @change="toggle(entry.type)"
          />
          <span class="chip-label"><span v-html="entry.icon"></span> {{ entry.label }}</span>
          <span class="chip-count">{{ entry.count }}</span>
        </label>
      </div>
    </div>
    <div class="filter-row">
      <input
        :value="filter.search"
        @input="updateSearch(($event.target as HTMLInputElement).value)"
        placeholder="搜索事件内容、工具名、错误..."
        class="search-input"
      />
      <button
        :class="['quick-btn', { active: filter.errorsOnly }]"
        @click="toggleErrorsOnly"
      >仅错误</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import type { ChatLogFilter, EventType, EventSummary } from '../../types/logs'
import { EVENT_TYPE_CONFIG } from '../../stores/logs'

const props = defineProps<{
  summary: EventSummary | null
  filter: ChatLogFilter
  checked: Record<string, boolean>
}>()

const emit = defineEmits<{
  'update:filter': [value: ChatLogFilter]
  'update:checked': [value: Record<string, boolean>]
}>()

// 构建事件类型列表，带计数
const eventEntries = computed(() => {
  const types = props.summary?.types || {}
  return Object.entries(EVENT_TYPE_CONFIG)
    .filter(([type]) => type in types || types[type] !== undefined)
    .map(([type, cfg]) => ({
      type: type as EventType,
      label: cfg.label,
      color: cfg.color,
      icon: cfg.icon,
      count: types[type] || 0,
    }))
})

function toggle(type: EventType) {
  const updated = { ...props.checked }
  if (type in updated) {
    updated[type] = !updated[type]
  } else {
    updated[type] = false
  }
  emit('update:checked', updated)
}

function updateSearch(val: string) {
  emit('update:filter', { ...props.filter, search: val })
}

function toggleErrorsOnly() {
  emit('update:filter', { ...props.filter, errorsOnly: !props.filter.errorsOnly })
}

// 首次有 summary 时初始化 checked
watch(() => props.summary, (s) => {
  if (!s) return
  const types = s.types || {}
  const updated: Record<string, boolean> = {}
  for (const type of Object.keys(EVENT_TYPE_CONFIG)) {
    updated[type] = type in types
  }
  emit('update:checked', updated)
}, { immediate: true })
</script>

<style scoped>
.chat-event-filter-bar {
  padding: 6px 10px; border-bottom: 1px solid var(--border-subtle);
  display: flex; flex-direction: column; gap: 5px; flex-shrink: 0;
}
.filter-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.filter-chips { display: flex; gap: 3px; flex-wrap: wrap; }
.filter-chip {
  display: flex; align-items: center; gap: 2px; padding: 2px 7px;
  border: 1px solid color-mix(in srgb, var(--chip-color, #94a3b8) 30%, transparent);
  border-radius: 12px; font-size: 10px; cursor: pointer; white-space: nowrap;
  color: var(--text-secondary); transition: background 0.12s;
}
.filter-chip:hover { background: color-mix(in srgb, var(--chip-color, #94a3b8) 6%, transparent); }
.filter-chip input { display: none; }
.filter-chip input:checked + .chip-label { color: var(--text-primary); font-weight: 600; }
.chip-count { font-size: 9px; color: var(--text-muted); background: var(--border-subtle); padding: 0 4px; border-radius: 8px; min-width: 14px; text-align: center; }
.chip-label { font-size: 10px; }
.search-input {
  flex: 1; min-width: 140px; padding: 3px 8px; border: 1px solid var(--border-default);
  border-radius: 4px; background: transparent; color: var(--text-primary);
  font-size: 11px; font-family: inherit; outline: none;
}
.quick-btn {
  padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 11px; font-family: inherit; white-space: nowrap;
}
.quick-btn.active { background: rgba(220,38,38,0.1); color: var(--color-danger); border-color: rgba(220,38,38,0.3); }
</style>
