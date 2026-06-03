<template>
  <div class="chat-log-viewer">
    <div class="viewer-toolbar">
      <div class="toolbar-left">
        <button :class="['mode-btn', { active: viewMode === 'timeline' }]" @click="viewMode = 'timeline'">⏱ 时间线</button>
        <button :class="['mode-btn', { active: viewMode === 'text' }]" @click="switchToText">📄 文本</button>
        <button :class="['mode-btn', { active: viewMode === 'json' }]" @click="viewMode = 'json'">{ } JSON</button>
      </div>
      <div class="toolbar-right">
        <button :class="['live-btn', { active: sseStatus === 'connected' }]" @click="toggleLive">
          <span class="live-dot" :class="sseStatus"></span>
          LIVE
        </button>
        <button class="btn-mini" @click="$emit('download')">↓ 下载</button>
      </div>
    </div>

    <!-- Filters + Pagination (timeline mode only) -->
    <ChatEventFilterBar
      v-if="viewMode === 'timeline'"
      :summary="summary"
      :filter="filter"
      :checked="checked"
      @update:filter="emit('update:filter', $event)"
      @update:checked="emit('update:checked', $event)"
    />

    <!-- Pagination bar (timeline mode, top of card) -->
    <div v-if="viewMode === 'timeline' && totalPages > 1" class="pagination-top">
      <span class="pg-label">分页</span>
      <div class="pg-controls">
        <button class="pg-arrow" :disabled="currentPage <= 1" @click="$emit('go-page', 1)" title="首页">«</button>
        <button class="pg-arrow" :disabled="currentPage <= 1" @click="$emit('go-page', currentPage - 1)" title="上一页">‹</button>
        <template v-for="p in pageNumbers" :key="p">
          <span v-if="p === -1" class="pg-dots">…</span>
          <button v-else :class="['pg-num', { active: p === currentPage }]" @click="$emit('go-page', p)">{{ p }}</button>
        </template>
        <button class="pg-arrow" :disabled="currentPage >= totalPages" @click="$emit('go-page', currentPage + 1)" title="下一页">›</button>
        <button class="pg-arrow" :disabled="currentPage >= totalPages" @click="$emit('go-page', totalPages)" title="末页">»</button>
      </div>
      <span class="pg-total">共 {{ totalEvents }} 条</span>
      <select class="pg-size" :value="pageSize" @change="$emit('page-size', Number(($event.target as HTMLSelectElement).value))">
        <option :value="50">50条/页</option>
        <option :value="100">100条/页</option>
        <option :value="200">200条/页</option>
        <option :value="500">500条/页</option>
      </select>
    </div>

    <!-- Timeline -->
    <div v-if="viewMode === 'timeline'" class="viewer-body">
      <div v-if="loading" class="empty-viewer">加载中...</div>
      <div v-else-if="!groupedEvents.length" class="empty-viewer">
        {{ selectedSession ? '该会话暂无事件' : '← 选择左侧会话查看日志' }}
      </div>
      <ChatEventTimeline
        v-else
        :groups="groupedEvents"
        :expandedJSON="expandedJSON"
        @toggle-json="emit('toggle-json', $event)"
        @load-more="$emit('load-more')"
        :has-more="hasMore"
      />
    </div>

    <!-- Text -->
    <div v-else-if="viewMode === 'text'" class="viewer-body">
      <div v-if="textLoading" class="empty-viewer">加载中...</div>
      <div v-else class="log-text-view">
        <template v-for="(line, i) in splitLines" :key="i">
          <LogLineRenderer v-if="line.trim().length > 0" :line="line" />
          <div v-else class="log-text-blank-line"></div>
        </template>
      </div>
    </div>

    <!-- JSON -->
    <div v-else-if="viewMode === 'json'" class="viewer-body">
      <div v-if="loading" class="empty-viewer">加载中...</div>
      <ChatJSONPreview
        v-else
        :events="allEvents"
        :expandedSet="expandedJSON"
        @toggle="emit('toggle-json', $event)"
      />
    </div>

    <!-- Live tail bar -->
    <ChatLiveTailBar
      v-if="viewMode === 'timeline'"
      :status="sseStatus"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { ChatViewMode, ChatLogFilter, EventSummary, LogEvent, SSEStatus } from '../../types/logs'
import ChatEventFilterBar from './ChatEventFilterBar.vue'
import ChatEventTimeline from './ChatEventTimeline.vue'
import ChatJSONPreview from './ChatJSONPreview.vue'
import ChatLiveTailBar from './ChatLiveTailBar.vue'
import LogLineRenderer from './LogLineRenderer.vue'

const props = defineProps<{
  viewMode: ChatViewMode
  loading: boolean
  textLoading: boolean
  text: string
  allEvents: LogEvent[]
  groupedEvents: { round: number; events: LogEvent[] }[]
  summary: EventSummary | null
  filter: ChatLogFilter
  checked: Record<string, boolean>
  sseStatus: SSEStatus
  selectedSession: string | null
  hasMore: boolean
  expandedJSON: Set<number>
	currentPage: number
	totalPages: number
	totalEvents: number
	pageSize: number
}>()

const emit = defineEmits<{
  'update:viewMode': [value: ChatViewMode]
  'update:filter': [value: ChatLogFilter]
  'update:checked': [value: Record<string, boolean>]
  'download': []
  'toggle-live': []
  'toggle-json': [index: number]
  'load-more': []
  'fetch-text': []
	'go-page': [page: number]
	'page-size': [size: number]
}>()

const viewMode = ref<ChatViewMode>(props.viewMode)
watch(viewMode, v => emit('update:viewMode', v))

// Generate page number buttons: show first, last, current±2, with ellipsis
const pageNumbers = computed(() => {
  const total = props.totalPages
  const cur = props.currentPage
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: number[] = [1]
  if (cur > 3) pages.push(-1)
  for (let p = Math.max(2, cur - 1); p <= Math.min(total - 1, cur + 1); p++) pages.push(p)
  if (cur < total - 2) pages.push(-1)
  pages.push(total)
  return pages
})
watch(() => props.viewMode, v => { viewMode.value = v })

const splitLines = computed(() => props.text.split('\n'))

function switchToText() {
  viewMode.value = 'text'
  emit('fetch-text')
}

function toggleLive() {
  emit('toggle-live')
}
</script>

<style scoped>
.chat-log-viewer { display: flex; flex-direction: column; flex: 1; min-height: 0; }

.viewer-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 6px 10px; border-bottom: 1px solid var(--border-subtle);
  gap: 8px; flex-shrink: 0;
}
.toolbar-left, .toolbar-right { display: flex; align-items: center; gap: 4px; }
.mode-btn {
  padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 11px; font-family: inherit;
}
.mode-btn.active { background: var(--brand-500); color: #fff; border-color: var(--brand-500); }
.live-btn {
  display: flex; align-items: center; gap: 4px;
  padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 11px; font-family: inherit;
}
.live-btn.active { background: rgba(16,185,129,0.12); color: #16a34a; border-color: rgba(16,185,129,0.3); }
.live-dot { width: 7px; height: 7px; border-radius: 50%; background: var(--text-muted); display: inline-block; }
.live-dot.connected { background: #16a34a; animation: livePulse 1.5s ease infinite; }
.live-dot.connecting { background: #f59e0b; animation: livePulse 0.5s ease infinite; }
.live-dot.error { background: #dc2626; }

@keyframes livePulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.viewer-body { flex: 1; overflow-y: auto; min-height: 0; }
.empty-viewer {
  display: flex; align-items: center; justify-content: center;
  color: var(--text-muted); font-size: 13px; padding: 60px 20px;
}
.log-text-view {
  padding: 6px 4px; background: transparent;
}
.log-text-blank-line { height: 1.2em; }
.btn-mini {
  padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 11px; font-family: inherit;
}
.btn-mini:hover { background: var(--surface-hover); color: var(--text-primary); }

/* ── Pagination ── */
.pagination-top {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; margin: 4px 0;
  background: var(--surface-card); border: 1px solid var(--border-subtle);
  border-radius: 8px; font-size: 12px;
}
.pg-label { font-weight: 600; color: var(--text-muted); font-size: 11px; margin-right: 4px; }
.pg-controls { display: flex; align-items: center; gap: 2px; }
.pg-arrow, .pg-num {
  min-width: 28px; height: 28px; display: flex; align-items: center; justify-content: center;
  border: 1px solid var(--border-default); border-radius: 5px;
  background: transparent; color: var(--text-secondary); font-size: 12px;
  font-family: inherit; cursor: pointer; transition: all 0.15s;
}
.pg-arrow:hover:not(:disabled), .pg-num:hover { background: var(--surface-hover); color: var(--text-primary); border-color: var(--border-strong); }
.pg-arrow:disabled { opacity: 0.3; cursor: default; }
.pg-num.active { background: var(--brand-500); color: #fff; border-color: transparent; font-weight: 600; }
.pg-dots { width: 28px; text-align: center; color: var(--text-muted); }
.pg-total { color: var(--text-muted); font-size: 11px; margin-left: 4px; }
.pg-size {
  margin-left: auto; padding: 4px 6px; border: 1px solid var(--border-default);
  border-radius: 5px; background: transparent; color: var(--text-secondary);
  font-size: 11px; font-family: inherit; cursor: pointer;
}
</style>
