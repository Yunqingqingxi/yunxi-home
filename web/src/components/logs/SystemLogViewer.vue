<template>
  <div class="system-log-viewer">
    <div class="sys-toolbar">
      <div class="sys-toolbar-row">
        <button class="btn-mini" @click="$emit('toggle-order')">
          {{ order === 'desc' ? '↓ 最新优先' : '↑ 最早优先' }}
        </button>
        <label v-for="l in levelOptions" :key="l.key" class="filter-cb">
          <input type="checkbox" :checked="levelChecks[l.key]" @change="toggleLevel(l.key)" />
          {{ l.label }}
        </label>
        <span class="sep">|</span>
        <label v-for="c in compOptions" :key="c" class="filter-cb">
          <input type="checkbox" :checked="compChecks[c]" @change="toggleComp(c)" />
          {{ c }}
        </label>
        <input
          :value="search"
          @input="emit('update:search', ($event.target as HTMLInputElement).value)"
          placeholder="搜索..."
          class="search-inp"
        />
        <label class="live-toggle">
          <input type="checkbox" :checked="liveTail" @change="$emit('toggle-live')" />实时
        </label>
        <button v-if="selectedDate" class="btn-mini" @click="$emit('download')">↓ 下载</button>
      </div>
    </div>

    <div v-if="!selectedDate" class="empty-viewer">← 选择左侧日期</div>
    <div v-else-if="loading" class="empty-viewer">加载中...</div>
    <div v-else ref="sysContent" class="sys-log-content">
      <SysLogLine
        v-for="(line, i) in lines"
        :key="i"
        :line="line"
        :search="search"
        :index="i"
      />
      <div v-if="hasMore" class="load-more">
        <button @click="$emit('load-more')">加载更多...</button>
      </div>
      <div v-if="liveTail" ref="anchorEl"></div>
    </div>

    <div v-if="liveTail" class="live-indicator">
      <span class="live-dot"></span> 实时跟踪中 (每 5 秒轮询)
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, ref, nextTick } from 'vue'
import type { LogLevel } from '../../types/logs'
import SysLogLine from './SysLogLine.vue'

const props = defineProps<{
  selectedDate: string | null
  lines: string[]
  loading: boolean
  hasMore: boolean
  liveTail: boolean
  order: string
  search: string
  levelChecks: Record<string, boolean>
  compChecks: Record<string, boolean>
}>()

const emit = defineEmits<{
  'toggle-order': []
  'update:levelChecks': [value: Record<string, boolean>]
  'update:compChecks': [value: Record<string, boolean>]
  'update:search': [value: string]
  'toggle-live': []
  'download': []
  'load-more': []
}>()

const levelOptions: { key: LogLevel; label: string }[] = [
  { key: 'ERROR', label: '错误' },
  { key: 'WARN', label: '警告' },
  { key: 'INFO', label: '信息' },
  { key: 'DEBUG', label: '调试' },
]

const compOptions = computed(() => Object.keys(props.compChecks).sort())

const sysContent = ref<HTMLElement | null>(null)
const anchorEl = ref<HTMLElement | null>(null)

// Auto-scroll in live mode
watch(() => props.lines.length, () => {
  if (props.liveTail && anchorEl.value) {
    nextTick(() => anchorEl.value?.scrollIntoView({ behavior: 'smooth' }))
  }
})

function toggleLevel(key: string) {
  const updated = { ...props.levelChecks }
  updated[key] = !updated[key]
  emit('update:levelChecks', updated)
}

function toggleComp(key: string) {
  const updated = { ...props.compChecks }
  updated[key] = !updated[key]
  emit('update:compChecks', updated)
}
</script>

<style scoped>
.system-log-viewer { display: flex; flex-direction: column; flex: 1; min-height: 0; }

.sys-toolbar { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); flex-shrink: 0; }
.sys-toolbar-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }

.btn-mini {
  padding: 3px 8px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 11px; font-family: inherit;
}
.btn-mini:hover { background: var(--surface-hover); color: var(--text-primary); }
.filter-cb { font-size: 10px; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; gap: 2px; white-space: nowrap; }
.filter-cb input { cursor: pointer; }
.sep { color: var(--border-default); font-size: 10px; }
.search-inp {
  width: 100px; padding: 3px 6px; border: 1px solid var(--border-default);
  border-radius: 4px; background: transparent; color: var(--text-primary);
  font-size: 11px; font-family: inherit; outline: none;
}
.live-toggle { font-size: 11px; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; gap: 4px; }
.live-toggle input { cursor: pointer; }

.empty-viewer {
  flex: 1; display: flex; align-items: center; justify-content: center;
  color: var(--text-muted); font-size: 13px; padding: 60px 20px;
}
.sys-log-content { flex: 1; overflow-y: auto; padding: 8px 10px; }
.load-more { text-align: center; padding: 10px; }
.load-more button {
  border: 1px solid var(--border-default); background: transparent;
  padding: 6px 16px; border-radius: 6px; cursor: pointer;
  font-family: inherit; font-size: 12px; color: var(--text-secondary);
}

.live-indicator {
  display: flex; align-items: center; gap: 6px; padding: 4px 10px;
  font-size: 11px; color: var(--text-muted);
  border-top: 1px solid var(--border-subtle); background: var(--surface-hover); flex-shrink: 0;
}
.live-dot { width: 7px; height: 7px; border-radius: 50%; background: #16a34a; animation: livePulse 1.5s ease infinite; }
@keyframes livePulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
</style>
