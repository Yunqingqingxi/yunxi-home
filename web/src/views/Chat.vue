<template>
  <div class="chat-layout">
    <Sidebar
      :conversations="store.conversations"
      :active-id="store.sessionId"
      :sub-agents="store.subAgents"
      @select="onSidebarSelect"
      @new-chat="onNewChat"
      @rename="onSidebarRename"
      @delete="onSidebarDelete"
      @toggle-pin="onSidebarTogglePin"
    />
    <div class="chat-page">
      <!-- Error boundary -->
      <div
        v-if="renderError"
        class="error-state"
      >
        <p>对话组件渲染异常</p>
        <button
          class="retry-btn"
          @click="resetRender"
        >
          重试
        </button>
      </div>

      <template v-else>
        <!-- HOME STATE -->
        <HomeState
          v-if="!store.sessionId"
          @quick-start="onQuickStart"
        />

        <!-- MESSAGES AREA -->
        <div
          v-else
          ref="msgContainer"
          class="panel-body"
          :style="{ paddingBottom: inputBarHeight + 'px' }"
          @scroll="onPanelScroll"
          @wheel.passive="onPanelWheel"
          @touchmove.passive="onPanelTouchMove"
        >
          <div class="sticky-panels"></div>
          <LockConflictNotice :conflicts="store.lockConflicts" />
          <TodoPanel :items="store.todoList" />
          <template
            v-for="(msg, i) in safeMessages"
            :key="msg?.id || ('_null_' + i)"
          >
            <div
              v-if="msg && i > 0 && memoMinutesBetween(safeMessages[i-1]?.createdAt, msg?.createdAt) > 5"
              class="time-divider"
            >
              <span>{{ msg.createdAt ? memoFormatHM(msg.createdAt) : '' }}</span>
            </div>
            <!-- Message wrapper with v-memo: only re-render when msg identity or _v changes -->
            <div
              v-if="msg"
              v-memo="[msg._v, editingIndex === i]"
              class="msg-wrapper"
              :class="{ 'msg-editing': editingIndex === i, 'msg-fading': fadingNodes && fadingNodes > 0 && i >= editRebaseIndex }"
              @mouseenter="hoveredMsg = i"
              @mouseleave="hoveredMsg = -1"
            >
              <ChatMessage
                :msg="editingIndex === i ? editDraftMsg : msg"
                :show-avatar="i === 0 || safeMessages[i-1]?.role !== msg.role"
                :class="{ 'msg-grouped': i > 0 && safeMessages[i-1]?.role === msg.role }"
                @edit="startEdit(i, $event)"
              />
            </div>
            <!-- Edit bar -->
            <div v-if="editingIndex === i" class="edit-bar">
              <textarea
                ref="editTextarea"
                v-model="editContent"
                class="edit-textarea"
                rows="3"
                @keydown.escape="cancelEdit"
                @keydown.ctrl.enter="saveEdit(i)"
              />
              <div class="edit-actions">
                <span class="edit-hint">Ctrl+Enter 保存 · Esc 取消</span>
                <button class="edit-save-btn" @click="saveEdit(i)">保存</button>
                <button class="edit-cancel-btn" @click="cancelEdit">取消</button>
                <button class="edit-delete-btn" @click="deleteMessage(i)">删除</button>
              </div>
            </div>
          </template>
          <div
            ref="scrollAnchor"
            class="spacer-end"
          ></div>
        </div>
        <button
          v-if="showScrollBtn"
          class="scroll-to-bottom-btn"
          :style="{ bottom: (inputBarHeight + 8) + 'px' }"
          title="滚动到最新"
          @click="scrollToBottom()"
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
          ><path d="M4 6l4 4 4-4" /></svg>
        </button>
      </template>

      <!-- MOBILE TOP BAR -->
      <div class="mobile-topbar" v-if="store.sessionId">
        <button class="mtb-btn" @click="mobileSidebar = !mobileSidebar">☰</button>
        <span class="mtb-title">{{ currentTitle }}</span>
        <button class="mtb-btn" @click="mobileInfo = !mobileInfo">ⓘ</button>
      </div>

      <!-- INPUT BAR -->
      <ChatInputBar ref="inputBarRef" />

      <!-- CONFIRM DIALOG -->
      <ConfirmDialog
        :request="store.confirmRequest"
        :session-id="store.sessionId"
        @done="store.confirmRequest = null"
      />
      <!-- INTERACTIVE DIALOG (表单/输入/选择) -->
      <InteractiveDialog
        :request="store.interactiveRequest"
        :visible="!!store.interactiveRequest"
        @respond="onInteractiveRespond"
        @close="store.interactiveRequest = null"
      />
    </div>

    <!-- Right info panel (desktop) -->
    <aside v-if="store.sessionId" class="info-panel" :class="{ collapsed: infoCollapsed }">
      <button class="ip-toggle" @click="infoCollapsed = !infoCollapsed" :title="infoCollapsed ? '展开面板' : '收起面板'">
        <svg :class="{ rotated: !infoCollapsed }" width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6,4 10,8 6,12"/></svg>
      </button>
      <div v-if="!infoCollapsed" class="ip-body">
        <AgentStatusBar
          :visible="store.isStreaming || store.hasRunningAgents"
          :agents="store.agents"
          :tool-name="store.currentToolName"
        />
        <TopologyPanel />
        <MetaReportCard :report="store.metaReport" :visible="store.agents.some((a: any) => a.role === 'supervisor' || a.role === 'manager')" />
        <div class="ip-section">
          <div class="ip-section-title">活跃助手</div>
          <AgentPanel :agents="store.agents" />
        </div>
      </div>
    </aside>

    <!-- Mobile sidebar drawer -->
    <div v-if="mobileSidebar" class="mobile-overlay" @click="mobileSidebar = false" />
    <aside v-if="mobileSidebar" class="mobile-drawer">
      <Sidebar
        :conversations="store.conversations"
        :active-id="store.sessionId"
        :sub-agents="store.subAgents"
        @select="onSidebarSelect; mobileSidebar = false"
        @new-chat="onNewChat; mobileSidebar = false"
        @rename="onSidebarRename"
        @delete="onSidebarDelete"
        @toggle-pin="onSidebarTogglePin"
      />
    </aside>

    <!-- Mobile info panel (bottom sheet) -->
    <div v-if="mobileInfo" class="mobile-overlay" @click="mobileInfo = false" />
    <div v-if="mobileInfo" class="mobile-sheet">
      <div class="sheet-handle" @click="mobileInfo = false" />
      <TopologyPanel />
      <MetaReportCard :report="store.metaReport" :visible="store.agents.some((a: any) => a.role === 'supervisor' || a.role === 'manager')" />
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, watch, onMounted, onUnmounted, onErrorCaptured, nextTick, getCurrentInstance } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useChatStore } from '../stores/chat'
import ChatMessage from '../components/chat/ChatMessage.vue'
import TodoPanel from '../components/chat/TodoPanel.vue'
import AgentPanel from '../components/chat/AgentPanel.vue'
import ConfirmDialog from '../components/chat/ConfirmDialog.vue'
import InteractiveDialog from '../components/chat/InteractiveDialog.vue'
import Sidebar from '../components/chat/Sidebar.vue'
import HomeState from '../components/chat/HomeState.vue'
import ChatInputBar from '../components/chat/ChatInputBar.vue'
import TopologyPanel from '../components/chat/TopologyPanel.vue'
import AgentStatusBar from '../components/chat/AgentStatusBar.vue'
import LockConflictNotice from '../components/chat/LockConflictNotice.vue'
import MetaReportCard from '../components/chat/MetaReportCard.vue'
import { formatHM, minutesBetween } from '../composables/useFormat'

// Memoize time calculations to avoid re-computing in v-for
const _timeCache = new Map<string, number>()
const _fmtCache = new Map<number, string>()
function memoMinutesBetween(a: number | undefined, b: number | undefined): number {
  const k = `${a ?? 0}|${b ?? 0}`
  if (_timeCache.has(k)) return _timeCache.get(k)!
  const v = minutesBetween(a, b)
  _timeCache.set(k, v)
  return v
}
function memoFormatHM(ts: number): string {
  if (_fmtCache.has(ts)) return _fmtCache.get(ts)!
  const v = formatHM(ts)
  _fmtCache.set(ts, v)
  return v
}

const route = useRoute()
const router = useRouter()
const store = useChatStore()
const mobileSidebar = ref(false)
const mobileInfo = ref(false)
const infoCollapsed = ref(false)
const currentTitle = computed(() => store.conversations.find(c => c.id === store.sessionId)?.title || '云兮')

// ── Safer message list: filters null entries that can arise from race conditions ──
const safeMessages = computed(() => store.messages.filter(m => m != null))

// ── Interrupt banner state ──
const msgContainer = ref(null)
const scrollAnchor = ref(null)
const renderError = ref(false)
const inputBarRef = ref(null)
const inputBarHeight = ref(150)
let _observer = null

// ── v3.1 Message editing ──
const hoveredMsg = ref(-1)
const editingIndex = ref(-1)
const editContent = ref('')
const editDraftMsg = ref<any>(null)
const editTextarea = ref<any>(null)
const fadingNodes = ref(0)
const editRebaseIndex = ref(-1)
let _fadeTimer: ReturnType<typeof setTimeout> | null = null

function startEdit(i: number, msg: any) {
  editingIndex.value = i
  editContent.value = msg.content || ''
  editDraftMsg.value = { ...msg, content: msg.content }
  nextTick(() => {
    const ta = editTextarea.value
    if (ta) {
      if (Array.isArray(ta)) ta[0]?.focus()
      else ta.focus()
    }
  })
}

function cancelEdit() {
  editingIndex.value = -1
  editContent.value = ''
  editDraftMsg.value = null
}

async function saveEdit(i: number) {
  const sid = store.sessionId
  const token = localStorage.getItem('token')
  if (!sid || !token || !editContent.value.trim()) return

  const isInsert = !editDraftMsg.value
  const msgIndex = i + 1 // +1 for system prompt

  console.log('[chat] saveEdit: start',
    '| sid:', sid,
    '| msgIndex:', msgIndex,
    '| isInsert:', isInsert,
    '| streaming:', store.isStreaming,
    '| msgs before:', store.messages.length)

  try {
    const resp = await fetch(`/api/chat/sessions/${sid}/messages/${msgIndex}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ content: editContent.value, insert_mode: isInsert }),
    })
    const data = await resp.json()
    console.log('[chat] saveEdit: PUT response', data)
    const deletedNodes = data?.data?.deleted_nodes || 0

    if (deletedNodes > 0) {
      editRebaseIndex.value = i
      fadingNodes.value = deletedNodes
      if (_fadeTimer) clearTimeout(_fadeTimer)
      _fadeTimer = setTimeout(() => { fadingNodes.value = 0; editRebaseIndex.value = -1 }, 1500)
      console.log('[chat] saveEdit: fading', deletedNodes, 'nodes from index', i)
    }

    // ④ 编辑后中断（fire-and-forget，仅当 Agent 活跃时）
    if (store.isStreaming || store.hasRunningAgents) {
      console.log('[chat] saveEdit: interrupting active session')
      store.setLifecycle(sid, 'idle')
      store.sessionAgents[sid] = []
      fetch(`/api/chat/sessions/${sid}/interrupt`, {
        method: 'POST',
        headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode: 'soft' }),
      }).catch(() => {})
    }

    // Reload the session
    console.log('[chat] saveEdit: reloading session messages')
    await store.fetchSessionMessages(sid)
    console.log('[chat] saveEdit: reload done, msgs:', store.messages.length)
  } catch (e) {
    console.error('[chat] saveEdit: failed', e)
  }

  cancelEdit()
  console.log('[chat] saveEdit: done, editingIndex:', editingIndex.value)
}

async function deleteMessage(i: number) {
  const sid = store.sessionId
  const token = localStorage.getItem('token')
  if (!sid || !token) return

  const msgIndex = i + 1
  try {
    const resp = await fetch(`/api/chat/sessions/${sid}/messages/${msgIndex}`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer ' + token },
    })
    const data = await resp.json()
    const deletedNodes = data?.data?.deleted_nodes || 0

    if (deletedNodes > 0) {
      editRebaseIndex.value = i
      fadingNodes.value = deletedNodes
      if (_fadeTimer) clearTimeout(_fadeTimer)
      _fadeTimer = setTimeout(() => { fadingNodes.value = 0; editRebaseIndex.value = -1 }, 1500)
    }

    store.fetchSessionMessages(sid)
  } catch (e) {
    console.error('Delete failed:', e)
  }

  cancelEdit()
}

onErrorCaptured((e, instance, info) => { console.error('[ChatView]', e, info); renderError.value = true; return false })
function resetRender() { renderError.value = false }

function onQuickStart(text) {
  nextTick(() => {
    inputBarRef.value?.setInput(text)
    startObserve()
  })
}

async function onInteractiveRespond(resp) {
  const token = localStorage.getItem('token')
  try {
    const r = await fetch('/api/chat/respond', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify(resp)
    })
    if (!r.ok) {
      const txt = await r.text()
      console.error('respond failed:', r.status, txt)
    }
  } catch (e) { console.error('respond failed', e) }
}

function startObserve() {
  _observer?.disconnect()
  if (!store.sessionId) return
  const comp = inputBarRef.value
  // ChatInputBar 多根节点（fragment注释→SVG→input-bar），沿着DOM树找输入栏
  let el = comp?.$el
  while (el && !el.classList?.contains('input-bar-floating') && !el.classList?.contains('input-bar-bottom')) {
    el = el.nextElementSibling
  }
  if (!el || !(el instanceof Element)) return
  _observer = new ResizeObserver(([entry]) => {
    if (store.sessionId) {
      const h = entry.borderBoxSize?.[0]?.blockSize ?? entry.contentRect.height
      inputBarHeight.value = h + 40
    }
  })
  _observer.observe(el)
}

// ── Scroll behavior ──
const scrolledUp = ref(false)
const showScrollBtn = ref(false)
let _userScrolled = false

function isNearBottom(el) { return el.scrollHeight - el.scrollTop - el.clientHeight < 120 }
function scrollToBottom() {
  scrollAnchor.value?.scrollIntoView({ block: 'end', behavior: 'smooth' })
  _userScrolled = false; scrolledUp.value = false; showScrollBtn.value = false
}
function onPanelScroll() {
  const el = msgContainer.value; if (!el) return
  const near = isNearBottom(el)
  if (!near && _userScrolled) { scrolledUp.value = true; showScrollBtn.value = true }
  else if (near) { scrolledUp.value = false; showScrollBtn.value = false }
}
function onPanelWheel(e) { if (e.deltaY < 0) _userScrolled = true }
function onPanelTouchMove(e) { _userScrolled = true }

function scrollToEnd() {
  scrollAnchor.value?.scrollIntoView({ block: 'end', behavior: 'instant' })
}

// Content growth watcher: auto-scroll unless user scrolled up
watch([() => store.messages.length, () => store.isStreaming, () => {
  const msgs = store.messages; const last = msgs.length > 0 ? msgs[msgs.length - 1] : null
  return last?._v ?? 0
}], ([len, streaming], [oldLen]) => {
  if (!streaming) { _userScrolled = false }
  if (!_userScrolled && !scrolledUp.value) {
    scrollToEnd()
  }
})
watch(() => store.sessionId, (sid, oldSid) => {
  console.log('[Chat] sessionId changed:', (oldSid||'').slice(-8), '→', (sid||'').slice(-8), 'route:', route.path)
  _observer?.disconnect()
  if (sid) {
    inputBarHeight.value = 120; nextTick(startObserve)
    if (!oldSid && route.path !== '/chat/' + sid) {
      console.log('[Chat] syncing URL to /chat/' + sid)
      router.replace('/chat/' + sid)
    }
  }
  else { inputBarHeight.value = 0 }
})
onMounted(() => { if (store.sessionId && store.messages.length) { nextTick(() => scrollToEnd()); setTimeout(() => scrollToEnd(), 200) } })

// ── Route / session ──
async function loadSessionFromRoute(sid) {
	if (!sid || sid === 'default') { store.clearCurrent(); router.replace('/chat'); return }
	// Already viewing this session — just ensure stream is connected
	if (store.sessionId === sid && store.messages.length > 0) {
		store.connectStream(sid)
		nextTick(() => { scrollToEnd() })
		return
	}
	// Keep old session's SSE alive — just swap displayed session
	store.resetStreaming()
	store.sessionId = sid
	store.activeConversationId = sid
	const ok = await store.fetchSessionMessages(sid)
	if (!ok) { store.clearCurrent(); return }
	_userScrolled = false; scrolledUp.value = false; showScrollBtn.value = false
	// Always start persistent SSE for the newly loaded session
	store.connectStream(sid)
	nextTick(() => { scrollToEnd() })
}

watch(() => route.params.sessionId, loadSessionFromRoute, { immediate: true })
watch(() => route.query.session, loadSessionFromRoute)

// ── Auto-send prompt from URL query parameter (page analyzer) ──
let _autoPromptSent = false
watch(
  () => route.query.prompt,
  (prompt) => {
    if (!prompt || _autoPromptSent) return
    const text = typeof prompt === 'string' ? decodeURIComponent(prompt) : String(prompt)
    if (!text) return
    _autoPromptSent = true
    // Build context message from URL query
    const context = route.query.context
    const fullMessage = context
      ? `[页面上下文: 用户正在查看"${context}"页面]\n${text}`
      : text
    // Wait for store to be ready, then send
    const trySend = () => {
      if (store.loading) { setTimeout(trySend, 300); return }
      nextTick(() => {
        store.sendMessage(fullMessage).then(() => {
          // Clean up query params after send
          router.replace({ path: '/chat/' + (store.sessionId || ''), query: {} })
        })
      })
    }
    // If no session yet, wait for the page to initialize
    if (!store.sessionId) {
      setTimeout(trySend, 500)
    } else {
      trySend()
    }
  },
  { immediate: true },
)

function onSidebarSelect(id) {
  if (id === store.sessionId) return
  store.switchConversation(id).then(ok => {
    if (ok) {
      _userScrolled = false; scrolledUp.value = false; showScrollBtn.value = false
      nextTick(() => { scrollToEnd() })
      router.replace('/chat/' + id)
    }
  })
}
function onNewChat() { console.log('[Chat] onNewChat'); store.clearCurrent(); router.replace('/chat') }

async function onSidebarRename(id: string, title: string) {
  await store.renameConversation(id, title)
}

async function onSidebarDelete(id: string) {
  await store.deleteConversation(id)
  // If deleted the current conversation, go home
  if (id === store.sessionId || !store.sessionId) {
    router.replace('/chat')
  }
}

async function onSidebarTogglePin(id: string, pinned: boolean) {
  await store.togglePin(id, pinned)
}

// Bug 1-3 fix: beforeunload handler — best-effort abort on tab close
let _beforeUnload: (() => void) | null = null

onMounted(() => {
  store.loadConversations()
  _beforeUnload = () => { store.cleanupAllStreams() }
  window.addEventListener('beforeunload', _beforeUnload)
})

onUnmounted(() => {
  console.log('[Chat] ⚠️ onUnmounted — component destroyed')
  _observer?.disconnect()
  if (_beforeUnload) {
    window.removeEventListener('beforeunload', _beforeUnload)
    _beforeUnload = null
  }
  store.cleanupAllStreams()
  store.forceClearSending()
})

// Only cleanup streams when actually leaving chat page (not session switching)
watch(() => route.path, (newPath, oldPath) => {
  if (oldPath && oldPath.startsWith('/chat/') && !newPath.startsWith('/chat')) {
    console.log('[Chat] leaving chat route, cleaning up all streams')
    store.cleanupAllStreams()
    store.forceClearSending()
  }
})
</script>

<style scoped>
.chat-layout {
  position: fixed; inset: 0;
  display: flex; padding: 72px 0 0 0;
  overflow: hidden; z-index: 1;
  min-width: 1200px;
}
.chat-page {
  flex: 1; display: flex; flex-direction: column;
  min-height: 0; overflow: hidden; position: relative;
  margin: 0 24px 24px 0;
}

/* Sticky wrapper for interrupt banner + agent status bar */
.sticky-panels {
  position: sticky;
  top: 0;
  z-index: 10;
}

.panel-body {
  flex: 1; overflow-y: auto; padding: 12px 20px 120px;
  display: flex; flex-direction: column; gap: 14px;
}

@media (max-width: 1199px) {
  .chat-layout { min-width: 100vw; }
  .panel-body { padding: 12px 16px 120px; }
}

@media (max-width: 767px) {
  .chat-layout {
    min-width: 100vw;
    padding: 56px 0 0 0;
  }
  .chat-page {
    margin: 0 0 0 0;
  }
  .panel-body {
    padding: 8px 12px 180px; /* 手机端留足空间给输入栏+安全区 */
  }
  .scroll-to-bottom-btn {
    bottom: 180px !important;
  }
}

/* Shared */
.panel-body::-webkit-scrollbar { width: 3px; }
.panel-body::-webkit-scrollbar-thumb { background: var(--border-default); border-radius: 3px; }
.spacer-end { height: 8px; flex-shrink: 0; user-select: none; pointer-events: none; }
.error-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--text-muted); }
.retry-btn { padding: 6px 18px; border-radius: 8px; border: 1px solid var(--border-default); background: var(--surface-card); color: var(--text-primary); cursor: pointer; font-size: 13px; }

/* Scroll-to-bottom button */
.scroll-to-bottom-btn {
  position: absolute; left: 50%; transform: translateX(-50%);
  z-index: 10; width: 36px; height: 36px; border-radius: 50%;
  border: 1px solid var(--glass-border-strong); background: var(--glass-bg-nav);
  backdrop-filter: blur(var(--glass-blur-nav));
  -webkit-backdrop-filter: blur(var(--glass-blur-nav));
  box-shadow: var(--glass-shadow-nav); color: var(--brand-500);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  animation: scrollBtnIn 0.25s var(--ease-out-back);
  transition: transform 0.15s, box-shadow 0.15s;
}
.scroll-to-bottom-btn:hover { transform: translateX(-50%) scale(1.1); box-shadow: var(--glass-shadow-elevated); }
@keyframes scrollBtnIn { from { opacity: 0; transform: translateX(-50%) translateY(8px); } to { opacity: 1; transform: translateX(-50%) translateY(0); } }

/* Time divider */
.time-divider {
  display: flex; align-items: center; gap: 12px;
  padding: 6px 0; flex-shrink: 0;
}
.time-divider::before, .time-divider::after {
  content: ''; flex: 1; height: 1px;
  background: var(--border-subtle);
}
.time-divider span {
  font-size: 10px; color: var(--text-muted);
  font-family: var(--font-mono); flex-shrink: 0;
}

/* Message grouping — tighter spacing for consecutive same-role */
.msg-grouped { margin-top: 4px; }

/* Home ↔ Chat transition */
.panel-body, .home-state {
  animation: chatFadeIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes chatFadeIn {
  from { opacity: 0; transform: translateY(8px); }
  to   { opacity: 1; transform: translateY(0); }
}

/* ── v3.1 Message editing ── */
.msg-wrapper {
  position: relative;
  content-visibility: auto;
  contain-intrinsic-size: auto 120px;
}

.edit-bar {
  margin: 4px 0 8px 48px;
  padding: 12px;
  border-radius: 10px;
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  box-shadow: var(--shadow-xs, 0 1px 2px rgba(0,0,0,0.04));
  animation: editBarIn 0.2s var(--ease-out-expo, ease-out);
}

@keyframes editBarIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.edit-textarea {
  width: 100%;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--surface-input, var(--surface-card));
  color: var(--text-primary);
  font-size: 13px;
  font-family: inherit;
  line-height: 1.5;
  resize: vertical;
  outline: none;
  transition: border-color 0.15s;
}

.edit-textarea:focus {
  border-color: var(--brand-400, #22d3ee);
  box-shadow: 0 0 0 3px rgba(6, 182, 212, 0.08);
}

.edit-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.edit-hint {
  font-size: 11px;
  color: var(--text-muted);
  flex: 1;
}

.edit-save-btn,
.edit-cancel-btn,
.edit-delete-btn {
  padding: 5px 14px;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.15s;
  line-height: 1.4;
}

.edit-save-btn {
  background: var(--brand-500);
  color: #fff;
  border-color: transparent;
  font-weight: 500;
}

.edit-save-btn:hover { background: var(--brand-600); }

.edit-cancel-btn {
  background: transparent;
  color: var(--text-secondary);
}

.edit-cancel-btn:hover {
  background: var(--surface-hover);
  color: var(--text-primary);
}

.edit-delete-btn {
  background: transparent;
  color: var(--color-danger, #f85149);
  border-color: transparent;
  margin-left: auto;
}

.edit-delete-btn:hover {
  background: rgba(248, 81, 73, 0.1);
}

/* ── v3.1 Topology fade-out animation ── */
.msg-fading {
  animation: nodeFadeOut 1.5s ease forwards;
}

@keyframes nodeFadeOut {
  0% { opacity: 1; filter: blur(0); }
  50% { opacity: 0.5; filter: blur(2px); }
  100% { opacity: 1; filter: blur(0); }
}

/* ── Three-column layout ── */
.chat-layout { display: flex; height: 100vh; overflow: hidden; }

/* ── Right info panel ── */
.info-panel {
  width: 320px; min-width: 320px; border-left: 1px solid var(--border-subtle, #e2e8f0);
  background: var(--surface-card, #fff); display: flex; flex-direction: column;
  transition: width 0.25s ease, min-width 0.25s ease;
}
.info-panel.collapsed { width: 40px; min-width: 40px; }
.ip-toggle {
  width: 100%; padding: 10px 0; border: none; background: transparent; cursor: pointer;
  color: var(--text-muted); display: flex; justify-content: center;
}
.ip-toggle svg { transition: transform 0.2s; }
.ip-toggle svg.rotated { transform: rotate(180deg); }
.ip-body {
  flex: 1; overflow-y: auto; padding: 0 12px 12px; display: flex; flex-direction: column; gap: 8px;
}
.ip-section { margin-top: 4px; }
.ip-section-title { font-size: 10px; font-weight: 600; color: var(--text-muted); margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px; }

/* Hide right panel on tablet/mobile */
@media (max-width: 1279px) {
  .info-panel { display: none; }
}

/* ── Mobile: top bar ── */
.mobile-topbar { display: none; }
@media (max-width: 767px) {
  .mobile-topbar {
    display: flex; align-items: center; gap: 4px;
    padding: 3px 6px; background: var(--surface-card);
    border-bottom: 1px solid var(--border-subtle);
    position: sticky; top: 0; z-index: 20;
  }
  .mtb-btn { background: none; border: none; font-size: 15px; cursor: pointer; padding: 3px 6px; color: var(--text-primary); }
  .mtb-title { flex: 1; font-size: 12px; font-weight: 600; text-align: center; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .chat-layout .sidebar { display: none; }
  .chat-page { margin: 0; border-radius: 0; }
  .panel-body { padding: 6px 6px 96px; }
}

/* ── Mobile: sidebar drawer ── */
.mobile-overlay { display: none; }
@media (max-width: 767px) {
  .mobile-overlay { display: block; position: fixed; inset: 0; background: rgba(0,0,0,0.3); z-index: 50; }
  .mobile-drawer {
    position: fixed; top: 0; left: 0; bottom: 0; width: 72vw; max-width: 260px;
    z-index: 55; background: var(--surface-card); box-shadow: 2px 0 10px rgba(0,0,0,0.08);
    overflow-y: auto; animation: slideIn 0.2s ease;
  }
  @keyframes slideIn { from { transform: translateX(-100%); } to { transform: translateX(0); } }

  .mobile-sheet {
    position: fixed; bottom: 0; left: 0; right: 0; max-height: 50vh;
    z-index: 55; background: var(--surface-card); border-radius: 12px 12px 0 0;
    box-shadow: 0 -2px 10px rgba(0,0,0,0.06); overflow-y: auto; padding: 10px 8px;
    animation: sheetUp 0.25s ease;
  }
  @keyframes sheetUp { from { transform: translateY(100%); } to { transform: translateY(0); } }
  .sheet-handle { width: 28px; height: 3px; background: var(--border-default); border-radius: 2px; margin: 0 auto 8px; cursor: pointer; }
}
</style>
