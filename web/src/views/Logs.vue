<template>
  <div class="logs-page">
    <!-- Analytics Overview -->
    <LogsAnalyticsOverview
      :analytics="store.analytics"
      :tool-stats="store.toolStats"
      :loading="store.analyticsLoading"
    />

    <!-- Tabs -->
    <div class="logs-tabs">
      <button :class="['tab', { active: tab === 'chat' }]" @click="switchTab('chat')">会话日志</button>
      <button :class="['tab', { active: tab === 'system' }]" @click="switchTab('system')">系统日志</button>
    </div>

    <!-- ── Chat Logs ── -->
    <div v-if="tab === 'chat'" class="chat-logs-layout">
      <ChatSessionList
        :sessions="store.chatSessions"
        :selected="store.selectedSessionId"
        :loading="store.chatSessionsLoading"
        @select="onSelectSession"
        @delete="onDeleteChat"
      />

      <div class="log-view-panel glass-card">
        <ChatLogViewer
          :view-mode="store.chatViewMode"
          :loading="store.chatEventsLoading"
          :text-loading="store.chatLogTextLoading"
          :text="store.chatLogText"
          :all-events="store.chatEvents"
          :grouped-events="store.groupedEvents"
          :summary="store.chatEventSummary"
          :filter="store.chatFilter"
          :checked="store.chatEventChecks"
          :sse-status="store.sseStatus"
          :selected-session="store.selectedSessionId"
          :has-more="store.chatHasMore"
          :expandedJSON="store.chatJSONExpanded"
          @update:view-mode="store.chatViewMode = $event"
          @update:filter="onChatFilterChange"
          @update:checked="store.chatEventChecks = $event"
          @download="store.downloadChatLog()"
          @toggle-live="onToggleSSE"
          @toggle-json="onToggleJSON"
          @load-more="store.loadMoreEvents()"
          @fetch-text="store.fetchChatText()"
        />
      </div>
    </div>

    <!-- ── System Logs ── -->
    <div v-if="tab === 'system'" class="system-logs-layout">
      <!-- Date panel -->
      <div class="sys-date-panel glass-card">
        <div class="panel-head">日期</div>
        <div v-if="store.sysFilesLoading" class="empty-panel">加载中...</div>
        <div v-else class="sys-date-list">
          <div
            v-for="f in store.sysFiles"
            :key="f.date"
            :class="['sys-date-item', { active: store.selectedSysDate === f.date }]"
            @click="store.selectSysDate(f.date)"
          >
            <span>{{ f.date }}</span>
            <button class="item-delete-btn" title="删除" @click.stop="onDeleteSysLog(f.date)">✕</button>
          </div>
        </div>
      </div>

      <!-- Viewer -->
      <div class="log-view-panel glass-card">
        <div class="panel-head">
          <span>{{ store.selectedSysDate || '选择日期' }}</span>
          <button
            v-if="store.selectedSysDate"
            class="btn-mini"
            @click="onDeleteSysLog(store.selectedSysDate!)"
          >🗑 删除</button>
        </div>
        <SystemLogViewer
          :selected-date="store.selectedSysDate"
          :lines="store.filteredSysLines"
          :loading="store.sysLoading"
          :has-more="store.sysHasMore"
          :live-tail="store.sysLiveTail"
          :order="store.sysFilter.order"
          :search="store.sysFilter.search"
          :level-checks="store.sysLevelChecks"
          :comp-checks="store.sysComponentChecks"
          @toggle-order="onToggleSysOrder"
          @update:level-checks="onLevelChecksChange"
          @update:comp-checks="onCompChecksChange"
          @update:search="onSysSearchChange"
          @toggle-live="onToggleSysLive"
          @download="store.downloadSysLog()"
          @load-more="store.loadMoreSysLog()"
        />
      </div>
    </div>

    <!-- Confirm Dialog -->
    <ConfirmDialog
      :visible="confirmDialog.visible"
      :title="confirmDialog.title"
      :message="confirmDialog.message"
      :confirm-text="confirmDialog.confirmText"
      :variant="confirmDialog.variant"
      icon="warn"
      @confirm="confirmDialog.visible = false; confirmDialog.resolve!(true)"
      @cancel="confirmDialog.visible = false; confirmDialog.resolve!(false)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount, watch } from 'vue'
import { useLogsStore } from '../stores/logs'
import type { ChatLogFilter, LogLevel } from '../types/logs'
import api from '../services/api'
import LogsAnalyticsOverview from '../components/logs/LogsAnalyticsOverview.vue'
import ChatSessionList from '../components/logs/ChatSessionList.vue'
import ChatLogViewer from '../components/logs/ChatLogViewer.vue'
import SystemLogViewer from '../components/logs/SystemLogViewer.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'
import { useToast } from '../composables/useToast'

const toast = useToast()
const store = useLogsStore()
const tab = ref<'chat' | 'system'>('chat')

// ── Confirm dialog ──────────────────────────────────
const confirmDialog = reactive({
  visible: false, title: '', message: '', confirmText: '确定', variant: 'danger' as string,
  resolve: null as ((v: boolean) => void) | null,
})

function showConfirm(title: string, msg: string, opts: { confirmText?: string; variant?: string } = {}): Promise<boolean> {
  return new Promise(r => {
    Object.assign(confirmDialog, {
      visible: true, title, message: msg,
      confirmText: opts.confirmText || '确定',
      variant: opts.variant || 'danger',
      resolve: r,
    })
  })
}

// ── Tab switch ──────────────────────────────────────
function switchTab(t: 'chat' | 'system') {
  tab.value = t
  if (t === 'chat') {
    store.fetchSessions()
  } else {
    store.fetchSysFiles()
  }
}

// ── Chat handlers ───────────────────────────────────
async function onSelectSession(s: any) {
  store.selectedSessionId = s.session_id
  store.chatLogText = ''
  store.chatJSONExpanded = new Set()
  await store.selectSession(s.session_id)
}

async function onDeleteChat(sessionId: string) {
  const ok = await showConfirm('删除会话日志', `确认删除会话日志 ${sessionId.slice(0, 20)}...？`, { confirmText: '删除', variant: 'danger' })
  if (!ok) return
  const err = await store.deleteChatLog(sessionId)
  if (err) toast.error(err)
  else toast.success('已删除')
}

function onChatFilterChange(f: ChatLogFilter) {
  store.chatFilter = f
  store.syncChatEventFilter()
}

function onToggleSSE() {
  if (store.sseStatus === 'connected') {
    store.disconnectSSE()
  } else {
    store.connectSSE()
  }
}

function onToggleJSON(index: number) {
  const set = new Set(store.chatJSONExpanded)
  if (set.has(index)) set.delete(index)
  else set.add(index)
  store.chatJSONExpanded = set
}

// ── System handlers ─────────────────────────────────
function onToggleSysOrder() {
  store.sysFilter.order = store.sysFilter.order === 'desc' ? 'asc' : 'desc'
  store.reloadSysLog()
}

function onLevelChecksChange(checks: Record<string, boolean>) {
  store.sysLevelChecks = checks
  store.syncSysLevelFilter()
  store.reloadSysLog()
}

function onCompChecksChange(checks: Record<string, boolean>) {
  store.sysComponentChecks = checks
  store.syncSysComponentFilter()
  store.reloadSysLog()
}

function onSysSearchChange(val: string) {
  store.sysFilter.search = val
  store.reloadSysLog()
}

function onToggleSysLive() {
  if (store.sysLiveTail) store.stopSysPoll()
  else store.startSysPoll()
}

async function onDeleteSysLog(date: string) {
  const ok = await showConfirm('删除系统日志', `确认删除 ${date} 的系统日志？`, { confirmText: '删除', variant: 'danger' })
  if (!ok) return
  const err = await store.deleteSystemLog(date)
  if (err) toast.error(err)
  else toast.success('已删除')
}

// ── Lifecycle ───────────────────────────────────────
onMounted(() => {
  store.fetchSessions()
  store.fetchSysFiles()
  store.fetchAnalytics()
})

onBeforeUnmount(() => {
  store.disconnectSSE()
  store.stopSysPoll()
})
</script>

<style scoped>
.logs-page { display: flex; flex-direction: column; flex: 1; min-height: 0; gap: 10px; }

/* Tabs */
.logs-tabs { display: flex; gap: 4px; flex-shrink: 0; }
.tab {
  padding: 8px 20px; border: 1px solid var(--border-default);
  border-radius: 8px 8px 0 0; background: transparent;
  color: var(--text-muted); cursor: pointer; font-size: 13px;
  font-family: inherit; border-bottom: none;
}
.tab.active { background: var(--glass-bg-card); color: var(--text-primary); font-weight: 600; }

/* Layout */
.chat-logs-layout, .system-logs-layout { display: flex; flex: 1; min-height: 0; gap: 12px; }
.log-view-panel { flex: 1; display: flex; flex-direction: column; min-width: 0; border-radius: var(--radius-lg); overflow: hidden; }

/* System date panel */
.sys-date-panel { width: 200px; flex-shrink: 0; display: flex; flex-direction: column; border-radius: var(--radius-lg); overflow: hidden; }
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; font-size: 13px; font-weight: 600;
  color: var(--text-primary); border-bottom: 1px solid var(--border-subtle); flex-shrink: 0;
}
.empty-panel {
  flex: 1; display: flex; align-items: center; justify-content: center;
  color: var(--text-muted); font-size: 13px; padding: 40px;
}
.sys-date-list { flex: 1; overflow-y: auto; }
.sys-date-item {
  padding: 8px 14px; border-bottom: 1px solid var(--border-subtle);
  cursor: pointer; font-size: 12px; color: var(--text-secondary);
  font-family: var(--font-mono); position: relative;
  display: flex; align-items: center; justify-content: space-between;
}
.sys-date-item .item-delete-btn {
  width: 20px; height: 20px; border: none; background: transparent;
  color: var(--text-muted); cursor: pointer; font-size: 13px; border-radius: 4px;
  display: flex; align-items: center; justify-content: center;
  opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s; flex-shrink: 0;
}
.sys-date-item:hover .item-delete-btn { opacity: 1; }
.sys-date-item .item-delete-btn:hover { color: var(--color-danger); background: rgba(239,68,68,0.1); }
.sys-date-item:hover { background: var(--surface-hover); }
.sys-date-item.active { color: var(--brand-600); font-weight: 600; background: color-mix(in srgb, var(--brand-500) 8%, transparent); }

.btn-mini {
  padding: 4px 10px; border: 1px solid var(--border-default); border-radius: 5px;
  background: transparent; color: var(--text-secondary); cursor: pointer;
  font-size: 11px; font-family: inherit;
}
.btn-mini:hover { background: var(--surface-hover); color: var(--text-primary); }

/* Responsive */
@media (max-width: 900px) {
  .chat-logs-layout, .system-logs-layout { flex-direction: column; }
  .sys-date-panel { width: 100%; max-height: 200px; }
}
</style>
