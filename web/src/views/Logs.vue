<template>
  <div class="logs-page">
    <!-- Tabs -->
    <div class="logs-tabs">
      <button :class="['tab', { active: tab === 'chat' }]" @click="tab = 'chat'">会话日志</button>
      <button :class="['tab', { active: tab === 'system' }]" @click="tab = 'system'">系统日志</button>
    </div>

    <!-- Chat Logs -->
    <div v-if="tab === 'chat'" class="chat-logs-layout">
      <div class="session-list-panel">
        <div class="panel-head">会话列表</div>
        <div class="date-filter" v-if="sessionDates.length > 1">
          <select v-model="selectedDate" class="date-select">
            <option value="">全部日期</option>
            <option v-for="d in sessionDates" :key="d" :value="d">{{ d }}</option>
          </select>
        </div>
        <div v-if="loadingSessions" class="empty-panel">加载中...</div>
        <div v-else-if="!sessions.length" class="empty-panel">暂无日志</div>
        <div v-else class="session-list">
          <template v-for="item in sessionsByDate" :key="item._date || item.session_id">
            <div v-if="item._date" class="session-date-head">{{ item._date }}</div>
            <div v-else
            :class="['session-item', { active: selectedSession === item.session_id }]"
            @click="selectSession(item)">
            <button class="item-delete-btn" title="删除" @click.stop="deleteChatLog(item.session_id)">✕</button>
            <div class="si-title">{{ item.session_id?.slice(0,20) }}...</div>
            <div class="si-meta">
              <span class="si-rounds">{{ item.rounds || '-' }} 轮</span>
              <span class="si-size">{{ fmtSize(item.size) }}</span>
              <span v-if="item.active" class="si-live">● LIVE</span>
            </div>
            <div class="si-date">{{ fmtDate(item.created) }}</div>
          </div>
          </template>
        </div>
      </div>

      <div class="log-view-panel">
        <div class="panel-head">
          <span>日志内容</span>
          <div class="panel-head-actions">
            <button v-if="selectedSession" class="btn-mini" @click="downloadChatLog">↓ 下载</button>
          </div>
        </div>

        <div v-if="!selectedSession" class="empty-panel">← 选择左侧会话查看日志</div>
        <div v-else-if="loadingLog" class="empty-panel">加载中...</div>
        <pre v-else class="log-text-view">{{ logText }}</pre>
      </div>
    </div>

    <!-- System Logs -->
    <div v-if="tab === 'system'" class="system-logs-layout">
      <div class="sys-date-panel">
        <div class="panel-head">日期</div>
        <div v-if="loadingSysFiles" class="empty-panel">加载中...</div>
        <div v-else class="sys-date-list">
          <div v-for="f in sysFiles" :key="f.date"
            :class="['sys-date-item', { active: selectedSysDate === f.date }]"
            @click="selectSysDate(f)">
            <span>{{ f.date }}</span>
            <button class="item-delete-btn" title="删除" @click.stop="deleteSystemLog(f.date)">✕</button>
          </div>
        </div>
      </div>

      <div class="log-view-panel">
        <div class="panel-head">
          <span>{{ selectedSysDate || '选择日期' }}</span>
          <div class="panel-head-actions">
            <button class="btn-mini" @click="toggleSysOrder">{{ sysLogOrder === 'desc' ? '↓ 最新优先' : '↑ 最早优先' }}</button>
            <label v-for="l in sysLevels" :key="l.key" class="filter-cb"><input type="checkbox" v-model="l.on" @change="reloadSysLog" />{{ l.label }}</label>
            <input v-model="sysLogSearch" placeholder="搜索..." class="search-inp" @input="reloadSysLog" />
            <label class="live-toggle"><input type="checkbox" v-model="sysLiveTail" />实时</label>
            <button v-if="selectedSysDate" class="btn-mini" @click="downloadSysLog">↓ 下载</button>
          </div>
        </div>
        <div v-if="!selectedSysDate" class="empty-panel">← 选择左侧日期</div>
        <div v-else-if="loadingSysLog" class="empty-panel">加载中...</div>
        <div v-else class="sys-log-content" ref="sysLogEl">
          <div v-for="(line, i) in sysLines" :key="i"
            :class="['sys-line', 'sys-' + lineLevel(line)]">
            {{ line }}
          </div>
          <div v-if="sysHasMore" class="load-more">
            <button @click="loadMoreSysLog">加载更多...</button>
          </div>
          <div v-if="sysLiveTail" ref="sysLiveAnchor"></div>
        </div>
      </div>
    </div>

    <ConfirmDialog :visible="confirmDialog.visible" :title="confirmDialog.title" :message="confirmDialog.message" :confirm-text="confirmDialog.confirmText" :variant="confirmDialog.variant" icon="warn" @confirm="confirmDialog.visible = false; confirmDialog.resolve(true)" @cancel="confirmDialog.visible = false; confirmDialog.resolve(false)" />
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, reactive, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import api from '../services/api'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import { useToast } from '../composables/useToast.js'

const toast = useToast()

const confirmDialog = reactive({ visible: false, title: '', message: '', confirmText: '确定', variant: 'danger', resolve: (_) => {} })
function showConfirm(title, msg, opts = {}) {
  return new Promise(r => {
    Object.assign(confirmDialog, { visible: true, title, message: msg, confirmText: opts.confirmText || '确定', variant: opts.variant || 'danger', resolve: r })
  })
}

const tab = ref('chat')

// ── Chat logs (text view) ──────────────────────────────
const sessions = ref([])
const loadingSessions = ref(false)
const selectedSession = ref('')
const loadingLog = ref(false)
const logText = ref('')
const selectedDate = ref('')

const sortedSessions = computed(() => {
  let list = [...sessions.value].sort((a, b) => {
    if (a.active !== b.active) return a.active ? -1 : 1
    return new Date(b.created) - new Date(a.created)
  })
  if (selectedDate.value) list = list.filter(s => fmtDate(s.created) === selectedDate.value)
  return list
})

const sessionDates = computed(() => {
  const dates = new Set()
  for (const s of sessions.value) dates.add(fmtDate(s.created))
  return [...dates].sort().reverse()
})

const sessionsByDate = computed(() => {
  const groups = []
  let lastDate = ''
  for (const s of sortedSessions.value) {
    const d = fmtDate(s.created)
    if (d !== lastDate) { groups.push({ _date: d }); lastDate = d }
    groups.push(s)
  }
  return groups
})

async function loadSessions() {
  loadingSessions.value = true
  try {
    const res = await api.get('/api/logs/chat/sessions')
    sessions.value = res.data?.data?.sessions || []
  } catch (_) {
    sessions.value = []
  } finally {
    loadingSessions.value = false
  }
}

async function selectSession(s) {
  selectedSession.value = s.session_id
  loadingLog.value = true
  logText.value = ''
  try {
    const res = await api.get('/api/logs/chat/' + s.session_id + '/text')
    logText.value = typeof res.data === 'string' ? res.data : (res.data?.data || '')
  } catch (_) {
    logText.value = '(无法加载日志)'
  } finally {
    loadingLog.value = false
  }
}

function downloadChatLog() {
  const token = localStorage.getItem('token')
  const a = document.createElement('a')
  a.href = `/api/logs/chat/${selectedSession.value}/download?token=${encodeURIComponent(token)}`
  a.click()
}

async function deleteChatLog(sessionId) {
  const ok = await showConfirm('删除会话日志', `确认删除会话日志 ${sessionId.slice(0, 20)}...？`, { confirmText: '删除', variant: 'danger' })
  if (!ok) return
  try {
    await api.delete('/api/logs/chat/' + sessionId)
    sessions.value = sessions.value.filter(s => s.session_id !== sessionId)
    if (selectedSession.value === sessionId) { selectedSession.value = ''; logText.value = '' }
    toast.success('已删除')
  } catch (e) {
    toast.error(e.response?.status === 409 ? '会话仍在运行，无法删除' : (e.response?.data?.message || '删除失败'))
  }
}

// ── System logs ────────────────────────────────────────
const sysFiles = ref([])
const loadingSysFiles = ref(false)
const selectedSysDate = ref('')
const sysLines = ref([])
const loadingSysLog = ref(false)
const sysOffset = ref(0)
const sysHasMore = ref(false)
const sysLogEl = ref(null)
const sysLiveAnchor = ref(null)
const sysLogOrder = ref('desc')
const sysLogSearch = ref('')
const sysLiveTail = ref(false)
const sysLevels = reactive([
  { key: 'ERROR', label: '错误', on: true },
  { key: 'WARN', label: '警告', on: true },
  { key: 'INFO', label: '信息', on: true },
  { key: 'DEBUG', label: '调试', on: true },
])
let sysPollTimer = null

function sysLogParams(extra = {}) {
  const levelOn = sysLevels.filter(l => l.on).map(l => l.key)
  const p = { order: sysLogOrder.value, limit: 500, ...extra }
  if (sysLogSearch.value) p.search = sysLogSearch.value
  if (levelOn.length > 0 && levelOn.length < 4) p.level = levelOn.join(',')
  return p
}

async function loadSysFiles() {
  loadingSysFiles.value = true
  try {
    const res = await api.get('/api/logs/system', { params: { order: 'desc' } })
    sysFiles.value = (res.data?.data?.files || []).sort((a, b) => (b.date || '').localeCompare(a.date || ''))
  } catch (_) { sysFiles.value = [] }
  finally { loadingSysFiles.value = false }
}

function toggleSysOrder() { sysLogOrder.value = sysLogOrder.value === 'desc' ? 'asc' : 'desc'; reloadSysLog() }

function reloadSysLog() {
  if (!selectedSysDate.value) return
  sysOffset.value = 0
  loadingSysLog.value = true
  api.get('/api/logs/system/' + selectedSysDate.value, { params: sysLogParams({ offset: 0 }) })
    .then(res => {
      sysLines.value = res.data?.data?.lines || []
      sysHasMore.value = res.data?.data?.has_more || false
      sysOffset.value = (res.data?.data?.lines || []).length
    })
    .catch(() => { sysLines.value = [] })
    .finally(() => { loadingSysLog.value = false })
}

async function selectSysDate(f) {
  stopSysPoll()
  selectedSysDate.value = f.date
  sysOffset.value = 0; sysLogSearch.value = ''
  loadingSysLog.value = true
  try {
    const res = await api.get('/api/logs/system/' + f.date, { params: sysLogParams({ offset: 0 }) })
    sysLines.value = res.data?.data?.lines || []
    sysHasMore.value = res.data?.data?.has_more || false
    sysOffset.value = (res.data?.data?.lines || []).length
  } catch (_) { sysLines.value = [] }
  finally { loadingSysLog.value = false }
}

async function loadMoreSysLog() {
  try {
    const res = await api.get('/api/logs/system/' + selectedSysDate.value, { params: sysLogParams({ offset: sysOffset.value }) })
    const newLines = res.data?.data?.lines || []
    sysLines.value.push(...newLines)
    sysHasMore.value = res.data?.data?.has_more || false
    sysOffset.value += newLines.length
  } catch (_) {}
}

watch(sysLiveTail, (on) => { if (on) startSysPoll(); else stopSysPoll() })

function startSysPoll() {
  if (sysPollTimer) return
  sysPollTimer = setInterval(pollSysTail, 5000)
  pollSysTail()
}

function stopSysPoll() { if (sysPollTimer) { clearInterval(sysPollTimer); sysPollTimer = null } }

async function pollSysTail() {
  if (!selectedSysDate.value) return
  try {
    const res = await api.get('/api/logs/system/' + selectedSysDate.value, { params: { tail: 200, order: sysLogOrder.value } })
    const tailLines = res.data?.data?.lines || []
    if (!tailLines.length) return
    const existingSet = new Set(sysLines.value)
    const newLines = tailLines.filter(l => !existingSet.has(l))
    if (newLines.length > 0) {
      sysLines.value = sysLogOrder.value === 'desc' ? [...newLines, ...sysLines.value] : [...sysLines.value, ...newLines]
      nextTick(() => { if (sysLiveAnchor.value) sysLiveAnchor.value.scrollIntoView({ behavior: 'smooth' }) })
    }
  } catch (_) {}
}

function downloadSysLog() {
  const token = localStorage.getItem('token')
  const a = document.createElement('a')
  a.href = `/api/logs/system/${selectedSysDate.value}/download?token=${encodeURIComponent(token)}`
  a.click()
}

async function deleteSystemLog(date) {
  const ok = await showConfirm('删除系统日志', `确认删除 ${date} 的系统日志？`, { confirmText: '删除', variant: 'danger' })
  if (!ok) return
  try {
    await api.delete('/api/logs/system/' + date)
    sysFiles.value = sysFiles.value.filter(f => f.date !== date)
    if (selectedSysDate.value === date) { selectedSysDate.value = ''; sysLines.value = []; stopSysPoll() }
    toast.success('已删除')
  } catch (e) { toast.error(e.response?.data?.message || '删除失败') }
}

// ── Helpers ────────────────────────────────────────────
function lineLevel(line) {
  if (line.includes('ERROR') || line.includes('level=ERROR')) return 'error'
  if (line.includes('WARN')) return 'warn'
  if (line.includes('DEBUG')) return 'debug'
  return 'info'
}

function fmtSize(bytes) {
  if (!bytes) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(1)) + ' ' + u[i]
}

function fmtDate(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

onBeforeUnmount(() => stopSysPoll())

// ── Init ───────────────────────────────────────────────
loadSessions()
loadSysFiles()
</script>

<style scoped>
.logs-page { display: flex; flex-direction: column; flex: 1; min-height: 0; gap: 12px; }

/* Tabs */
.logs-tabs { display: flex; gap: 4px; }
.tab { padding: 8px 20px; border: 1px solid var(--border-default); border-radius: 8px 8px 0 0; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 13px; font-family: inherit; border-bottom: none; }
.tab.active { background: var(--glass-bg-card); color: var(--text-primary); font-weight: 600; border-color: var(--border-default); }

/* Layout */
.chat-logs-layout, .system-logs-layout { display: flex; flex: 1; min-height: 0; gap: 12px; }
.session-list-panel, .sys-date-panel { width: 220px; flex-shrink: 0; display: flex; flex-direction: column; border: 1px solid var(--border-default); border-radius: var(--radius-lg); overflow: hidden; background: var(--glass-bg-card); }
.log-view-panel { flex: 1; display: flex; flex-direction: column; min-width: 0; border: 1px solid var(--border-default); border-radius: var(--radius-lg); overflow: hidden; background: var(--glass-bg-card); }
.panel-head { display: flex; align-items: center; justify-content: space-between; padding: 10px 14px; font-size: 13px; font-weight: 600; color: var(--text-primary); border-bottom: 1px solid var(--border-subtle); flex-shrink: 0; gap: 8px; }
.panel-head-actions { display: flex; align-items: center; gap: 8px; }
.empty-panel { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text-muted); font-size: 13px; padding: 40px; }

/* Session list */
.session-list { flex: 1; overflow-y: auto; }
.session-item { padding: 10px 14px; border-bottom: 1px solid var(--border-subtle); cursor: pointer; transition: background 0.1s; position: relative; }
.session-item .item-delete-btn { position: absolute; top: 6px; right: 6px; width: 20px; height: 20px; border: none; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 13px; border-radius: 4px; display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s; }
.session-item:hover .item-delete-btn { opacity: 1; }
.session-item .item-delete-btn:hover { color: var(--color-danger); background: rgba(239,68,68,0.1); }
.session-item:hover { background: var(--surface-hover); }
.session-item.active { background: color-mix(in srgb, var(--brand-500) 8%, transparent); }
.si-title { font-size: 12px; font-family: var(--font-mono); color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.si-meta { display: flex; gap: 8px; margin-top: 4px; font-size: 11px; color: var(--text-muted); }
.si-live { color: var(--brand-500); font-weight: 600; }
.si-date { font-size: 10px; color: var(--text-muted); margin-top: 2px; }

/* Chat log text view */
.log-text-view {
  flex: 1; overflow: auto; margin: 0; padding: 12px 16px;
  font-family: var(--font-mono); font-size: 12px; line-height: 1.6;
  color: var(--text-primary); white-space: pre-wrap; word-break: break-all;
  background: transparent; border: none; tab-size: 2;
}

/* System logs */
.sys-date-list { flex: 1; overflow-y: auto; }
.sys-date-item { padding: 8px 14px; border-bottom: 1px solid var(--border-subtle); cursor: pointer; font-size: 12px; color: var(--text-secondary); font-family: var(--font-mono); position: relative; display: flex; align-items: center; justify-content: space-between; }
.sys-date-item .item-delete-btn { width: 20px; height: 20px; border: none; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 13px; border-radius: 4px; display: flex; align-items: center; justify-content: center; opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s; flex-shrink: 0; }
.sys-date-item:hover .item-delete-btn { opacity: 1; }
.sys-date-item .item-delete-btn:hover { color: var(--color-danger); background: rgba(239,68,68,0.1); }
.sys-date-item:hover { background: var(--surface-hover); }
.sys-date-item.active { color: var(--brand-600); font-weight: 600; background: color-mix(in srgb, var(--brand-500) 8%, transparent); }

.sys-log-content { flex: 1; overflow-y: auto; padding: 10px 14px; font-family: var(--font-mono); font-size: 12px; line-height: 1.5; }
.sys-line { white-space: pre-wrap; word-break: break-all; }
.sys-error { color: var(--color-danger); background: rgba(220,38,38,0.04); }
.sys-warn { color: #d97706; }
.sys-debug { color: var(--text-muted); opacity: 0.7; }
.sys-info { color: var(--text-secondary); }
.load-more { text-align: center; padding: 10px; }
.load-more button { border: 1px solid var(--border-default); background: transparent; padding: 6px 16px; border-radius: 6px; cursor: pointer; font-family: inherit; font-size: 12px; color: var(--text-secondary); }

/* UI */
.live-toggle { font-size: 12px; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; gap: 4px; }
.live-toggle input { cursor: pointer; }
.btn-mini { padding: 4px 10px; border: 1px solid var(--border-default); border-radius: 5px; background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 11px; font-family: inherit; }
.btn-mini:hover { background: var(--surface-hover); color: var(--text-primary); }
.filter-cb { font-size: 10px; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; gap: 2px; white-space: nowrap; }
.filter-cb input { cursor: pointer; }
.search-inp { width: 100px; padding: 3px 6px; border: 1px solid var(--border-default); border-radius: 4px; background: transparent; color: var(--text-primary); font-size: 11px; font-family: inherit; outline: none; }

/* Date grouping */
.session-date-head { padding: 5px 14px; font-size: 10.5px; font-weight: 600; color: var(--brand-600); background: var(--brand-50); border-bottom: 1px solid var(--border-subtle); position: sticky; top: 0; z-index: 1; }
.date-filter { padding: 5px 10px; border-bottom: 1px solid var(--border-subtle); }
.date-select { width: 100%; padding: 3px 6px; border: 1px solid var(--border-default); border-radius: 4px; background: transparent; color: var(--text-primary); font-size: 10.5px; font-family: inherit; outline: none; }

[data-theme="dark"] .sys-error { background: rgba(248,113,113,0.08); }
[data-theme="dark"] .session-date-head { background: rgba(34,211,238,0.08); color: #67e8f9; }

/* Responsive */
@media (max-width: 900px) {
  .chat-logs-layout, .system-logs-layout { flex-direction: column; }
  .session-list-panel, .sys-date-panel { width: 100%; max-height: 200px; }
}
</style>
