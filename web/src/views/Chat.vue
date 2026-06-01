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
      <div v-if="renderError" class="error-state">
        <p>对话组件渲染异常</p>
        <button @click="resetRender" class="retry-btn">重试</button>
      </div>

      <template v-else>
        <!-- HOME STATE -->
        <HomeState v-if="!store.sessionId" @quickStart="onQuickStart" />

        <!-- MESSAGES AREA -->
        <div v-else class="panel-body" ref="msgContainer" :style="{ paddingBottom: inputBarHeight + 'px' }" @scroll="onPanelScroll" @wheel.passive="onPanelWheel" @touchmove.passive="onPanelTouchMove">
          <TodoPanel :items="store.todoList" />
          <AgentPanel :agents="store.agents" />
          <template v-for="(msg, i) in store.messages" :key="msg.id">
            <div v-if="i > 0 && timeGap(store.messages[i-1], msg) > 5" class="time-divider">
              <span>{{ fmtMsgTime(msg.createdAt) }}</span>
            </div>
            <ChatMessage
              :msg="msg"
              :show-avatar="i === 0 || store.messages[i-1]?.role !== msg.role"
              :class="{ 'msg-grouped': i > 0 && store.messages[i-1]?.role === msg.role }"
            />
          </template>
          <div ref="scrollAnchor" class="spacer-end"></div>
        </div>
        <button v-if="showScrollBtn" class="scroll-to-bottom-btn" :style="{ bottom: (inputBarHeight + 8) + 'px' }" @click="scrollToBottom(true)" title="滚动到最新">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 6l4 4 4-4"/></svg>
        </button>
      </template>

      <!-- INPUT BAR -->
      <ChatInputBar ref="inputBarRef" />

      <!-- CONFIRM DIALOG -->
      <ConfirmDialog :request="store.confirmRequest" :session-id="store.sessionId" @done="store.confirmRequest = null" />
      <!-- INTERACTIVE DIALOG (表单/输入/选择) -->
      <InteractiveDialog :request="store.interactiveRequest" :visible="!!store.interactiveRequest"
        @respond="onInteractiveRespond" @close="store.interactiveRequest = null" />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted, onErrorCaptured, nextTick } from 'vue'
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

const route = useRoute()
const router = useRouter()
const store = useChatStore()

const msgContainer = ref(null)
const scrollAnchor = ref(null)
const renderError = ref(false)
const inputBarRef = ref(null)
const inputBarHeight = ref(150)
let _observer = null

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
    await fetch('/api/chat/respond', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify(resp)
    })
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

@media (max-width: 767px) {
  .chat-layout { padding-top: 78px; }
  .chat-page { margin: 0; border-radius: 0; }
  .panel-body { padding: 10px 8px 110px; gap: 10px; }
}
</style>
