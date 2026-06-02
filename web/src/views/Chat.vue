<template>
  <div class="chat-layout">
    <Sidebar
      :conversations="store.conversations"
      :active-id="store.sessionId"
      :sub-agents="store.subAgents"
      @select="onSidebarSelect"
      @new-chat="onNewChat"
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
          <InterruptBanner
            :visible="interruptBannerVisible && !!store.interruptSnapshot"
            :progress="store.interruptSnapshot?.progress || 0"
            :last-task="store.interruptSnapshot?.last_task || ''"
            @continue="onInterruptContinue"
            @retry="onInterruptRetry"
            @dismiss="onInterruptNewTask"
          />
          <AgentStatusBar
            v-if="store.currentToolName"
            :visible="store.isStreaming || store.hasRunningAgents"
            :is-running="store.isStreaming"
            :tool-name="store.currentToolName"
            :progress="store.currentToolProgress"
          />
          <TopologyPanel />
          <TodoPanel :items="store.todoList" />
          <AgentPanel :agents="store.agents" />
          <template
            v-for="(msg, i) in store.messages"
            :key="msg.id"
          >
            <!-- Insert button between messages -->
            <div
              v-if="i > 0 && msg.role === 'user'"
              class="insert-between"
              @click="startInsert(i)"
              title="在此插入消息"
            >
              <span class="insert-line"></span>
              <span class="insert-plus">+</span>
            </div>
            <div
              v-if="i > 0 && timeGap(store.messages[i-1], msg) > 5"
              class="time-divider"
            >
              <span>{{ fmtMsgTime(msg.createdAt) }}</span>
            </div>
            <!-- Message wrapper with edit controls -->
            <div
              class="msg-wrapper"
              :class="{ 'msg-editing': editingIndex === i, 'msg-fading': fadingNodes && fadingNodes > 0 && i >= editRebaseIndex }"
              @mouseenter="hoveredMsg = i"
              @mouseleave="hoveredMsg = -1"
            >
              <ChatMessage
                :msg="editingIndex === i ? editDraftMsg : msg"
                :show-avatar="i === 0 || store.messages[i-1]?.role !== msg.role"
                :class="{ 'msg-grouped': i > 0 && store.messages[i-1]?.role === msg.role }"
              />
              <!-- Edit pencil on user messages -->
              <button
                v-if="msg.role === 'user' && hoveredMsg === i && editingIndex !== i"
                class="edit-pencil"
                title="编辑消息"
                @click="startEdit(i, msg)"
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
                  <path d="M11.5 1.5l3 3L5 14H2v-3L11.5 1.5z"/>
                </svg>
              </button>
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
          @click="scrollToBottom(true)"
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
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, watch, onMounted, onUnmounted, onErrorCaptured, nextTick, getCurrentInstance } from 'vue'
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
import InterruptBanner from '../components/chat/InterruptBanner.vue'
import AgentStatusBar from '../components/chat/AgentStatusBar.vue'

const route = useRoute()
const router = useRouter()
const store = useChatStore()

// ── Interrupt banner state ──
const interruptBannerVisible = ref(false)
let _interruptTimer: ReturnType<typeof setTimeout> | null = null

// Watch store interruptSnapshot to show banner
watch(() => store.interruptSnapshot, (snap) => {
  if (snap) {
    interruptBannerVisible.value = true
    if (_interruptTimer) clearTimeout(_interruptTimer)
    _interruptTimer = setTimeout(() => { interruptBannerVisible.value = false }, 30000)
  }
}, { deep: true })

function dismissBanner() {
  interruptBannerVisible.value = false
  if (_interruptTimer) { clearTimeout(_interruptTimer); _interruptTimer = null }
}

function onInterruptContinue() {
  dismissBanner()
  store.sendMessage('继续', '', {})
}

function onInterruptRetry() {
  dismissBanner()
  store.sendMessage('换个方式重新执行刚才的任务', '', {})
}

function onInterruptNewTask() {
  dismissBanner()
}

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

function startInsert(i: number) {
  editingIndex.value = i
  editContent.value = ''
  editDraftMsg.value = null
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

  try {
    const resp = await fetch(`/api/chat/sessions/${sid}/messages/${msgIndex}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ content: editContent.value, insert_mode: isInsert }),
    })
    const data = await resp.json()
    const deletedNodes = data?.data?.deleted_nodes || 0

    if (deletedNodes > 0) {
      editRebaseIndex.value = i
      fadingNodes.value = deletedNodes
      if (_fadeTimer) clearTimeout(_fadeTimer)
      _fadeTimer = setTimeout(() => { fadingNodes.value = 0; editRebaseIndex.value = -1 }, 1500)
    }

    // ④ 编辑后中断（fire-and-forget，仅当 Agent 活跃时）
    if (store.isStreaming || store.hasRunningAgents) {
      fetch(`/api/chat/sessions/${sid}/interrupt`, {
        method: 'POST',
        headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode: 'soft' }),
      }).catch(() => {})
    }

    // Reload the session
    store.fetchSessionMessages(sid)
  } catch (e) {
    console.error('Edit failed:', e)
  }

  cancelEdit()
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

function timeGap(a, b) {
  if (!a?.createdAt || !b?.createdAt) return 0
  return Math.abs(b.createdAt - a.createdAt) / 60000
}

function fmtMsgTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return d.getHours().toString().padStart(2, '0') + ':' + d.getMinutes().toString().padStart(2, '0')
}

// ── Scroll behavior ──
const scrolledUp = ref(false)
const showScrollBtn = ref(false)
let _userScrolled = false
let _scrollRaf = 0

function isNearBottom(el) { return el.scrollHeight - el.scrollTop - el.clientHeight < 120 }
function scrollToBottom(smooth = false) {
  cancelAnimationFrame(_scrollRaf)
  const el = msgContainer.value; if (!el) return
  el.style.scrollBehavior = smooth ? 'smooth' : 'auto'
  el.scrollTop = el.scrollHeight
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

let _lastSmartScroll = 0
function smartScrollToBottom() {
  if (scrolledUp.value) return
  const now = Date.now()
  if (now - _lastSmartScroll < 200) return
  _lastSmartScroll = now
  scrollToEnd(false)
}
function scrollToEnd(smooth = true) {
  const el = msgContainer.value
  if (el) { el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'instant' }) }
}
function forceScrollToBottom(retries = 30) {
  if (scrollAnchor.value) { scrollAnchor.value.scrollIntoView({ block: 'end', behavior: 'instant' }) }
  else if (retries > 0) { setTimeout(() => forceScrollToBottom(retries - 1), 100) }
}

watch(() => store.messages.length, () => nextTick(() => smartScrollToBottom()))
watch(() => store.isStreaming, (v) => { if (!v) { _userScrolled = false; nextTick(() => smartScrollToBottom()) } })
// 流式时内容增长（_v 递增）自动滚动到底部
watch(() => {
  const msgs = store.messages
  const last = msgs.length > 0 ? msgs[msgs.length - 1] : null
  return last?._v ?? 0
}, () => { if (store.isStreaming) nextTick(() => smartScrollToBottom()) })
watch(() => store.sessionId, (sid) => {
  _observer?.disconnect()
  if (sid) { inputBarHeight.value = 120; nextTick(startObserve) }
  else { inputBarHeight.value = 0 }
})
onMounted(() => { if (store.sessionId && store.messages.length) { nextTick(() => scrollToEnd()); setTimeout(() => scrollToEnd(), 200) } })

// ── Route / session ──
function loadSessionFromRoute(sid) {
  if (!sid || sid === 'default') { store.clearCurrent(); router.replace('/chat'); return }
  if (store.sessionId === sid && store.isStreaming && store.messages.length > 0) return
  store.disconnectStream()
  store.resetStreaming(); store.sessionId = sid
  store.fetchSessionMessages(sid).then(ok => {
    if (!ok) { store.clearCurrent(); return }
    _userScrolled = false; scrolledUp.value = false; showScrollBtn.value = false
    const msgs = store.messages
    const last = msgs.length > 0 ? msgs[msgs.length - 1] : null
    const hasPendingTools = last && last.role === 'assistant' && (
      (last.tools && last.tools.some(t => !t.result)) ||
      (last.blocks && last.blocks.some(b => b.type === 'tool' && !b.result))
    )
    if (hasPendingTools) { store.connectStream(sid) }
    // Scroll to bottom — instant first, then smooth re-check
    nextTick(() => { scrollToEnd(false) })
    setTimeout(() => { scrollToEnd(true) }, 150)
    setTimeout(() => { scrollToEnd(true) }, 400)
  })
}

watch(() => route.params.sessionId, loadSessionFromRoute, { immediate: true })
watch(() => route.query.session, loadSessionFromRoute)

function onSidebarSelect(id) {
  if (id === store.sessionId) return
  store.switchConversation(id).then(ok => { if (ok) router.replace('/chat/' + id) })
}
function onNewChat() { store.clearCurrent(); router.replace('/chat') }

onMounted(() => { store.loadConversations() })
onUnmounted(() => { _observer?.disconnect(); store.disconnectStream() })
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
}

.edit-pencil {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: var(--color-bg-3);
  color: var(--color-text-3);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s, background 0.15s;
  z-index: 2;
}

.msg-wrapper:hover .edit-pencil {
  opacity: 1;
}

.edit-pencil:hover {
  background: var(--color-primary-light, #6366f122);
  color: var(--color-primary, #6366f1);
}

.insert-between {
  display: flex;
  align-items: center;
  height: 20px;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s;
  margin: -4px 0;
}

.insert-between:hover {
  opacity: 1;
}

.insert-line {
  flex: 1;
  height: 1px;
  background: var(--color-border);
}

.insert-plus {
  margin: 0 12px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--color-bg-3);
  border: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  color: var(--color-text-3);
  transition: all 0.15s;
}

.insert-between:hover .insert-plus {
  background: var(--color-primary, #6366f1);
  color: #fff;
  border-color: var(--color-primary, #6366f1);
}

.edit-bar {
  margin: 4px 0 8px 48px;
  padding: 12px;
  border-radius: 8px;
  background: var(--color-bg-2);
  border: 1px solid var(--color-primary, #6366f1);
  animation: editBarIn 0.15s ease;
}

@keyframes editBarIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}

.edit-textarea {
  width: 100%;
  padding: 10px;
  border-radius: 6px;
  border: 1px solid var(--color-border);
  background: var(--color-bg-1);
  color: var(--color-text-1);
  font-size: 14px;
  font-family: inherit;
  resize: vertical;
  outline: none;
}

.edit-textarea:focus {
  border-color: var(--color-primary, #6366f1);
}

.edit-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.edit-hint {
  font-size: 11px;
  color: var(--color-text-3);
  flex: 1;
}

.edit-save-btn,
.edit-cancel-btn,
.edit-delete-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: none;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s;
}

.edit-save-btn {
  background: var(--color-primary, #6366f1);
  color: #fff;
}

.edit-save-btn:hover { filter: brightness(1.1); }

.edit-cancel-btn {
  background: var(--color-bg-3);
  color: var(--color-text-2);
}

.edit-cancel-btn:hover { background: var(--color-bg-4); }

.edit-delete-btn {
  background: #ef444422;
  color: #ef4444;
  margin-left: auto;
}

.edit-delete-btn:hover { background: #ef444433; }

/* ── v3.1 Topology fade-out animation ── */
.msg-fading {
  animation: nodeFadeOut 1.5s ease forwards;
}

@keyframes nodeFadeOut {
  0% { opacity: 1; filter: blur(0); }
  50% { opacity: 0.5; filter: blur(2px); }
  100% { opacity: 1; filter: blur(0); }
}

@media (max-width: 767px) {
  .chat-layout { padding-top: 78px; }
  .chat-page { margin: 0; border-radius: 0; }
  .panel-body { padding: 10px 8px 110px; gap: 10px; }
  .edit-bar { margin-left: 12px; }
}
</style>
