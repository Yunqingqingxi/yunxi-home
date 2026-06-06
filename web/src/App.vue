<template>
  <div
    v-if="isLoginPage"
    class="login-layout"
  >
    <router-view v-slot="{ Component }">
      <transition
        name="page"
        mode="out-in"
      >
        <component :is="Component" />
      </transition>
    </router-view>
  </div>

  <div
    v-else
    class="app-glass-shell"
    :data-section="sectionName"
  >
    <!-- Floating top navigation -->
    <nav
      class="floating-nav"
      :class="{ compact: !isDesktop }"
    >
      <div
        class="nav-brand"
        title="回到首页"
        @click="navigate('/')"
      >
        <img
          src="/logo.svg"
          alt="云兮之家"
          class="nav-logo"
        />
      </div>
      <div class="nav-pills">
        <button
          v-for="item in navItems"
          :key="item.path"
          :class="['nav-pill', { active: ready && currentRoute === item.path }]"
          :title="!isDesktop ? item.label : ''"
          @click="navigate(item.path)"
        >
          <span
            class="pill-icon"
            v-html="item.icon"
          ></span>
          <span
            v-if="isDesktop"
            class="pill-label"
          >{{ item.label }}</span>
        </button>
      </div>
      <div class="nav-actions">
        <button
          class="nav-action-btn"
          :title="theme.theme === 'dark' ? '切换到亮色模式' : '切换到暗色模式'"
          @click="theme.toggle()"
        >
          <!-- 主题切换：暗色显示月亮，亮色显示齿轮 -->
          <svg
            v-if="theme.theme === 'dark'"
            width="15"
            height="15"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
          >
            <path d="M8 2a6 6 0 1 0 6 5.4A4.5 4.5 0 0 1 8 2Z" />
          </svg>
          <svg
            v-else
            width="15"
            height="15"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
          >
            <circle cx="8" cy="8" r="2.5"/>
            <path d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2M3.5 3.5l1.5 1.5M11 11l1.5 1.5M3.5 12.5l1.5-1.5M11 5l1.5-1.5"/>
          </svg>
        </button>
        <button
          class="nav-action-btn"
          title="退出登录"
          @click="doLogout"
        >
          <svg
            width="15"
            height="15"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
          >
            <path d="M6 3H3a1 1 0 00-1 1v8a1 1 0 001 1h3M10 11l3-3-3-3M13 8H6" />
          </svg>
        </button>
      </div>
    </nav>

    <!-- Mobile bottom dock -->
    <nav
      v-if="false"
      class="mobile-dock"
    >
      <button
        v-for="item in navItems"
        :key="item.path"
        :class="['dock-item', { active: ready && currentRoute === item.path }]"
        @click="navigate(item.path)"
      >
        <span
          class="dock-icon"
          v-html="item.icon"
        ></span>
      </button>
      <button
        class="dock-item"
        title="退出"
        @click="doLogout"
      >
        <span class="dock-icon">
          <svg
            width="18"
            height="18"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.6"
            stroke-linecap="round"
          >
            <path d="M6 3H3a1 1 0 00-1 1v8a1 1 0 001 1h3M10 11l3-3-3-3M13 8H6" />
          </svg>
        </span>
      </button>
    </nav>

    <!-- Main content area -->
    <main
      class="floating-main"
      :class="{ 'has-dock': !isDesktop }"
    >
      <router-view v-slot="{ Component, route }">
        <transition
          name="page"
          mode="out-in"
        >
          <component
            :is="Component"
            :key="route.name"
          />
        </transition>
      </router-view>
    </main>

    <!-- Floating AI page analyzer (hidden on /chat) -->
    <div
      v-if="currentRoute !== '/chat' && !currentRoute.startsWith('/chat/')"
      class="floating-chat-widget"
      :class="{ expanded: analyzerOpen }"
      :style="
        widgetPos.x || widgetPos.y
          ? { left: widgetPos.x + 'px', top: widgetPos.y + 'px', right: 'auto', bottom: 'auto' }
          : {}
      "
    >
      <button
        class="chat-widget-toggle"
        :title="analyzerOpen ? '收起' : 'AI 页面分析'"
        @click="toggleAnalyzer"
        @touchstart="onWidgetDragStart"
        @touchmove="onWidgetDragMove"
        @touchend="onWidgetDragEnd"
      >
        <!-- Brain / AI analyze icon -->
        <svg
          v-if="!analyzerOpen"
          width="22"
          height="22"
          viewBox="0 0 24 24"
          fill="none"
          stroke="var(--brand-400)"
          stroke-width="1.6"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M12 2a4 4 0 0 1 4 4c0 2.5-3 4-4 8-1-4-4-5.5-4-8a4 4 0 0 1 4-4z" />
          <path d="M8 14c-2 1-5 1-5 4 0 2 2.5 3 5 3M16 14c2 1 5 1 5 4 0 2-2.5 3-5 3" opacity="0.6" />
          <circle cx="9" cy="6" r="0.8" fill="var(--brand-500)" opacity="0.8">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2.5s" repeatCount="indefinite" begin="0s" />
          </circle>
          <circle cx="12" cy="5" r="0.8" fill="var(--brand-500)" opacity="0.5">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2.5s" repeatCount="indefinite" begin="0.6s" />
          </circle>
          <circle cx="15" cy="6" r="0.8" fill="var(--brand-500)" opacity="0.5">
            <animate attributeName="opacity" values="0.3;1;0.3" dur="2.5s" repeatCount="indefinite" begin="1.2s" />
          </circle>
        </svg>
        <svg
          v-else
          width="18"
          height="18"
          viewBox="0 0 18 18"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
        >
          <path d="M3 3l12 12M15 3l-12 12" />
        </svg>
      </button>
      <div
        v-if="analyzerOpen"
        class="chat-widget-body"
      >
        <PageAnalyzer
          :page-name="sectionName"
          :visible="analyzerOpen"
          @analyze="analyzePage"
          @go-chat="navigate('/chat')"
        />
      </div>
    </div>

    <!-- Global Upload Bar -->
    <div
      v-if="uploadStore.tasks.length"
      class="floating-upload-bar"
      :class="{ collapsed: gupCollapsed }"
    >
      <div
        class="gup-header"
        @click="gupCollapsed = !gupCollapsed"
      >
        <svg
          :class="['gup-chevron', { open: !gupCollapsed }]"
          width="10"
          height="10"
          viewBox="0 0 10 10"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
        >
          <path d="M3 2l3 3-3 3" />
        </svg>
        <span class="gup-label">{{ uploadStore.hasActive ? '上传中' : '上传完成' }}</span>
        <span class="gup-count">{{
          uploadStore.hasActive
            ? uploadStore.tasks.filter((t) => t.status === 'uploading').length
            : uploadStore.tasks.length
        }}
          files</span>
        <button
          v-if="!uploadStore.hasActive"
          class="gup-dismiss"
          @click.stop="uploadStore.clearDone()"
        >
          <svg
            width="12"
            height="12"
            viewBox="0 0 12 12"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M2 2l8 8M10 2l-8 8" />
          </svg>
        </button>
      </div>
      <div
        v-if="!gupCollapsed"
        class="gup-body"
      >
        <div class="gup-track">
          <div
            class="gup-fill"
            :style="{ width: uploadStore.totalProgress + '%' }"
          ></div>
        </div>
        <div class="gup-list">
          <div
            v-for="t in uploadStore.tasks"
            :key="t.id"
            class="gup-item"
          >
            <span class="gup-name">{{ t.name }}</span>
            <span :class="['gup-status', t.status]">{{
              t.status === 'uploading' ? t.progress + '%' : t.status === 'done' ? '\✓' : '\✗'
            }}</span>
            <button
              v-if="t.status === 'uploading'"
              class="gup-cancel"
              title="取消"
              @click="uploadStore.cancelTask(t.id)"
            >
              ✕
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, onMounted, watch, onUnmounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useThemeStore } from './stores/theme'
import { useUploadStore } from './stores/upload'
import { useSettingsStore } from './stores/settings'
import PageAnalyzer from './components/chat/PageAnalyzer.vue'
import { formatRelativeShort } from './composables/useFormat'

const router = useRouter()
const route = useRoute()
const theme = useThemeStore()
const settingsStore = useSettingsStore()
const uploadStore = useUploadStore()
const authStore = useAuthStore()
const gupCollapsed = ref(false)
const chatWidgetOpen = ref(false)
const ready = ref(false)

// 移动端悬浮球拖拽
const widgetPos = ref({ x: 0, y: 0 })
let _dragStart = { x: 0, y: 0, elX: 0, elY: 0 }
let _dragged = false
function onWidgetDragStart(e) {
  const t = e.touches[0]
  const el = e.target.closest('.floating-chat-widget')
  _dragStart = { x: t.clientX, y: t.clientY, elX: el.offsetLeft, elY: el.offsetTop }
  _dragged = false
}
function onWidgetDragMove(e) {
  const t = e.touches[0]
  const dx = t.clientX - _dragStart.x,
    dy = t.clientY - _dragStart.y
  if (Math.abs(dx) > 5 || Math.abs(dy) > 5) {
    _dragged = true
    widgetPos.value = { x: _dragStart.elX + dx, y: _dragStart.elY + dy }
  }
}
function onWidgetDragEnd() {
  if (_dragged) _dragged = false
}

let _autoDismiss = null
watch(
  () => uploadStore.hasActive,
  (active) => {
    if (active) {
      clearTimeout(_autoDismiss)
      _autoDismiss = null
      return
    }
    _autoDismiss = setTimeout(() => {
      uploadStore.clearDone()
      _autoDismiss = null
    }, 4000)
  },
)

const currentRoute = computed(() => router.currentRoute.value.path)
const isLoginPage = computed(() => route.path === '/login')

const sectionName = computed(() => {
  const name = {
    '/': 'files',
    '/dashboard': 'dashboard',
    '/domains': 'domains',
    '/market': 'market',
    '/logs': 'logs',
    '/chat': 'chat',
    '/system': 'system',
    '/terminal': 'terminal',
    '/settings': 'settings',
  }[route.path]
  return name || ''
})

const isDesktop = ref(window.innerWidth >= 768)
const authStore_ = useAuthStore()

function onResize() {
  isDesktop.value = window.innerWidth >= 768
}

const isAdmin = computed(() => authStore_.user?.role === 'admin')

const allNavItems = [
  {
    path: '/',
    label: '文件管理',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M3 3.5h4l1.5 1.5H15a1 1 0 011 1v8a1 1 0 01-1 1H3a1 1 0 01-1-1v-10a1 1 0 011-1z"/></svg>',
  },
  {
    path: '/dashboard',
    label: '仪表盘',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><rect x="1" y="1" width="6" height="7" rx="1.5"/><rect x="11" y="1" width="6" height="4" rx="1.5"/><rect x="1" y="12" width="6" height="5" rx="1.5"/><rect x="11" y="9" width="6" height="8" rx="1.5"/></svg>',
  },
  {
    path: '/domains',
    label: 'DNS 管理',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><circle cx="9" cy="9" r="7.5"/><ellipse cx="9" cy="9" rx="3.5" ry="7.5"/><path d="M1.5 9h15M9 1.5v15"/></svg>',
  },
  {
    path: '/market',
    label: '技能市场',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M9 1.5L11.5 6l5 .5-3.7 3.3 1 5L9 12.2 4.2 14.8l1-5L1.5 6.5l5-.5z"/></svg>',
  },
  {
    path: '/logs',
    label: '日志',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M3 4h12M3 8h8M3 12h10M3 16h6"/></svg>',
  },
  {
    path: '/chat',
    label: 'AI 助手',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M14 10c0 1.1-.9 2-2 2H8l-3.5 3.5V12H3c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2v6z"/><circle cx="5.5" cy="7" r="1" fill="currentColor" opacity="0.6"/><circle cx="9" cy="7" r="1" fill="currentColor" opacity="0.6"/><circle cx="12.5" cy="7" r="1" fill="currentColor" opacity="0.6"/></svg>',
  },
  {
    path: '/system',
    label: '系统控制',
    icon: '<svg width="18" height="18" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><rect x="2" y="3" width="14" height="10" rx="2"/><path d="M6 16h6M9 13v3"/></svg>',
  },
  {
    path: '/settings',
    label: '设置',
    icon: '<svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="8" cy="8" r="4"/><path d="M8 2v1M8 13v1M2 8h1M13 8h1M3.8 3.8l.7.7M11.5 11.5l.7.7M3.8 12.2l.7-.7M11.5 4.5l.7-.7"/></svg>',
  },
]

const navItems = computed(() => allNavItems.filter((item) => item.path !== '/system' && item.path !== '/terminal'))

// Page analyzer for floating widget
const analyzerOpen = ref(false)

function toggleAnalyzer() {
  analyzerOpen.value = !analyzerOpen.value
}

function analyzePage(prompt: string) {
  analyzerOpen.value = false
  const context = sectionName.value
  const encoded = encodeURIComponent(prompt)
  router.push({ path: '/chat', query: { prompt: encoded, context } })
}

// Legacy session list functions (keep for compatibility)
const chatSessions = ref([])
const activeSessionId = ref('')

async function loadChatSessions() {
  try {
    const token = localStorage.getItem('token')
    if (!token) return
    const res = await fetch('/api/chat/sessions', { headers: { Authorization: 'Bearer ' + token } })
    if (res.status === 401) return
    const data = await res.json()
    if (data.code === 200) {
      chatSessions.value = (data.data || []).sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
    }
  } catch (e) {}
}

function navigateChat(sessionId) {
  activeSessionId.value = sessionId
  router.push({ path: '/chat/' + sessionId })
}

function sessionTime(t) {
  return formatRelativeShort(t)
}

async function deleteSession(id) {
  try {
    const token = localStorage.getItem('token')
    await fetch('/api/chat/sessions/' + id, { method: 'DELETE', headers: { Authorization: 'Bearer ' + token } })
    if (activeSessionId.value === id) {
      activeSessionId.value = ''
      router.push({ path: '/chat' })
    }
    loadChatSessions()
  } catch (e) {}
}

onMounted(() => {
  loadChatSessions()
  uploadStore.resumeFromStorage()
  settingsStore.load()
  window.addEventListener('resize', onResize)
  nextTick(() => {
    ready.value = true
  })
})

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
})

function navigate(path) {
  router.push(path)
}

function doLogout() {
  authStore_.logout()
  router.push('/login')
}

// Auto-collapse analyzer when navigating to chat page
watch(
  () => route.path,
  (newPath) => {
    if (newPath === '/chat' || newPath.startsWith('/chat/')) {
      analyzerOpen.value = false
    }
  },
)
</script>

<style>
@import './styles/tokens.css';
@import './styles/dark.css';
@import './styles/base.css';
@import './styles/arco.css';
</style>

<style scoped>
/* ============================================
   App Glass Shell
   ============================================ */
.app-glass-shell {
  position: relative;
  width: 100%;
  height: 100vh;
  overflow: hidden;
}

/* ============================================
   Floating Navigation
   ============================================ */
.floating-nav {
  position: fixed;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 200;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 8px;
  max-width: 820px;
  width: calc(100% - 24px);
  background: var(--glass-bg-nav);
  backdrop-filter: blur(var(--glass-blur-nav));
  -webkit-backdrop-filter: blur(var(--glass-blur-nav));
  border: 1px solid var(--glass-border-strong);
  border-radius: 16px;
  box-shadow: var(--glass-shadow-nav);
  transition: all 0.2s var(--ease-out-expo);
}

.nav-brand {
  display: flex;
  align-items: center;
  gap: 7px;
  padding-right: 10px;
  flex-shrink: 0;
  cursor: pointer;
}

.nav-logo {
  height: 20px;
}

.nav-pills {
  display: flex;
  align-items: center;
  gap: 1px;
  flex: 1;
  justify-content: center;
  overflow-x: auto;
  scrollbar-width: none;
}
.nav-pills::-webkit-scrollbar {
  display: none;
}

.nav-pill {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 9px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-family: var(--font-sans);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  white-space: nowrap;
  transition: all 0.15s var(--ease-out-expo);
  position: relative;
  flex-shrink: 0;
}

.nav-pill:hover {
  background: rgba(255, 255, 255, 0.35);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  color: var(--text-primary);
}

.nav-pill.active {
  background: rgba(14, 165, 233, 0.12);
  color: var(--brand-600);
  font-weight: var(--weight-semibold);
}

.nav-pill.active::after {
  content: '';
  position: absolute;
  bottom: 1px;
  left: 50%;
  transform: translateX(-50%);
  width: 16px;
  height: 2px;
  background: var(--brand-500);
  border-radius: 1px;
}

.pill-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  color: inherit;
}

.pill-label {
  font-size: var(--text-sm);
}

[data-theme='dark'] .nav-pill.active {
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
}
[data-theme='dark'] .nav-pill.active::after {
  background: #38bdf8;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 1px;
  padding-left: 10px;
  flex-shrink: 0;
}

.nav-action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s;
}

.nav-action-btn:hover {
  background: rgba(255, 255, 255, 0.35);
  color: var(--text-primary);
}

/* Compact (mobile) */
.floating-nav.compact {
  padding: 6px 8px;
  max-width: calc(100% - 16px);
  border-radius: 14px;
}
.floating-nav.compact .nav-brand {
  padding-right: 6px;
}
.floating-nav.compact .nav-pill {
  padding: 7px 9px;
  gap: 0;
}
.floating-nav.compact .nav-actions {
  padding-left: 4px;
}

/* ============================================
   Mobile Bottom Dock
   ============================================ */
.mobile-dock {
  position: fixed;
  bottom: max(12px, var(--safe-area-bottom, 12px));
  left: 50%;
  transform: translateX(-50%);
  z-index: 200;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 5px 6px;
  background: var(--glass-bg-nav);
  backdrop-filter: blur(var(--glass-blur-nav));
  -webkit-backdrop-filter: blur(var(--glass-blur-nav));
  border: 1px solid var(--glass-border-strong);
  border-radius: 16px;
  box-shadow: var(--glass-shadow-nav);
}

.dock-item {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border: none;
  border-radius: 12px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.15s var(--ease-out-expo);
}

.dock-item:hover,
.dock-item.active {
  background: rgba(255, 255, 255, 0.35);
  color: var(--brand-600);
}

.dock-item.active::after {
  content: '';
  position: absolute;
  bottom: 2px;
  width: 16px;
  height: 2px;
  background: var(--brand-500);
  border-radius: 1px;
}

.dock-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  position: relative;
}

[data-theme='dark'] .dock-item.active {
  color: #38bdf8;
}

/* ============================================
   Main Content
   ============================================ */
.floating-main {
  width: 100%;
  height: 100vh;
  padding: 72px 24px 24px;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
  z-index: 1;
}

.floating-main.has-dock {
  padding-bottom: 84px;
}

/* ============================================
   Floating Chat Widget
   ============================================ */
.floating-chat-widget {
  position: fixed;
  bottom: 20px;
  right: 20px;
  z-index: 190;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.chat-widget-toggle {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  border: 1px solid var(--glass-border-strong);
  background: var(--glass-bg-widget);
  backdrop-filter: blur(var(--glass-blur-nav));
  -webkit-backdrop-filter: blur(var(--glass-blur-nav));
  box-shadow: var(--glass-shadow-nav);
  color: var(--brand-500);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s var(--ease-out-expo);
  flex-shrink: 0;
}

.chat-widget-toggle:hover {
  transform: scale(1.08);
  box-shadow: var(--glass-shadow-elevated);
}

.chat-widget-body {
  width: 340px;
  max-height: 460px;
  background: var(--glass-bg-widget);
  backdrop-filter: blur(var(--glass-blur-elevated));
  -webkit-backdrop-filter: blur(var(--glass-blur-elevated));
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-xl);
  box-shadow: var(--glass-shadow-elevated);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: widgetSlideIn 0.2s var(--ease-out-expo);
}

@keyframes widgetSlideIn {
  from {
    opacity: 0;
    transform: translateY(10px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.chat-widget-header {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border-subtle);
  color: var(--text-primary);
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  flex-shrink: 0;
}

.chat-widget-badge {
  margin-left: auto;
  font-size: 10px;
  font-weight: var(--weight-semibold);
  background: rgba(14, 165, 233, 0.12);
  color: var(--brand-600);
  padding: 1px 7px;
  border-radius: 8px;
  min-width: 18px;
  text-align: center;
}
[data-theme='dark'] .chat-widget-badge {
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
}

.chat-widget-sessions {
  flex: 1;
  overflow-y: auto;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 320px;
}

.chat-widget-empty {
  padding: 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.chat-widget-session {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
  font-family: var(--font-sans);
  text-align: left;
  transition: all 0.12s;
  width: 100%;
}

.chat-widget-session:hover {
  background: rgba(255, 255, 255, 0.35);
}

.chat-widget-session.active {
  background: rgba(14, 165, 233, 0.1);
}

.cws-name {
  flex: 1;
  font-size: var(--text-sm);
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cws-time {
  font-size: 10px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.cws-del {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0;
  transition: all 0.1s;
  flex-shrink: 0;
}

.chat-widget-session:hover .cws-del {
  opacity: 0.7;
}
.cws-del:hover {
  background: rgba(220, 38, 38, 0.1);
  color: var(--color-danger);
  opacity: 1;
}

.chat-widget-footer {
  padding: 8px 12px;
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.chat-widget-new-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  padding: 8px;
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: rgba(255, 255, 255, 0.3);
  color: var(--brand-600);
  font-family: var(--font-sans);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  cursor: pointer;
  transition: all 0.15s;
}

.chat-widget-new-btn:hover {
  background: rgba(14, 165, 233, 0.1);
  border-color: rgba(14, 165, 233, 0.2);
}


/* ============================================
   Global Upload Bar (floating center bottom)
   ============================================ */
.floating-upload-bar {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 180;
  width: 420px;
  max-width: calc(100vw - 32px);
  background: var(--glass-bg-elevated);
  backdrop-filter: blur(var(--glass-blur-elevated));
  -webkit-backdrop-filter: blur(var(--glass-blur-elevated));
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-lg);
  box-shadow: var(--glass-shadow-elevated);
  overflow: hidden;
  transition: all 0.2s var(--ease-out-expo);
}

.floating-upload-bar.collapsed {
  width: auto;
}

.gup-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
  transition: background 0.1s;
}
.gup-header:hover {
  background: rgba(0, 0, 0, 0.02);
}
.gup-chevron {
  flex-shrink: 0;
  transition: transform 0.2s var(--ease-out-expo);
  color: var(--text-muted);
}
.gup-chevron.open {
  transform: rotate(90deg);
}
.gup-label {
  font-size: var(--text-sm);
  font-weight: var(--weight-semibold);
  color: var(--text-primary);
}
.gup-count {
  font-size: var(--text-xs);
  color: var(--text-muted);
  margin-left: auto;
}
.gup-dismiss {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.1s;
}
.gup-dismiss:hover {
  background: rgba(220, 38, 38, 0.08);
  color: var(--color-danger);
}

.gup-body {
  padding: 0 12px 10px;
}
.gup-track {
  height: 3px;
  border-radius: 2px;
  background: var(--bg-progress-track);
  overflow: hidden;
  margin-bottom: 6px;
}
.gup-fill {
  height: 100%;
  background: var(--brand-500);
  border-radius: 2px;
  transition: width 0.3s var(--ease-out-expo);
}
.gup-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 140px;
  overflow-y: auto;
}
.gup-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--text-xs);
}
.gup-name {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-secondary);
}
.gup-status {
  font-size: 10px;
  font-weight: var(--weight-semibold);
  min-width: 28px;
  text-align: right;
}
.gup-status.uploading {
  color: var(--brand-500);
}
.gup-status.done {
  color: var(--color-success);
}
.gup-status.error {
  color: var(--color-danger);
}
.gup-cancel {
  width: 20px;
  height: 20px;
  border: none;
  background: rgba(220, 38, 38, 0.08);
  color: var(--color-danger);
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  padding: 0;
}
.gup-cancel:hover {
  background: rgba(220, 38, 38, 0.18);
}

/* ============================================
   Responsive
   ============================================ */
@media (min-width: 768px) {
  .floating-main {
    padding: 80px 28px 28px;
  }
}

@media (max-width: 767px) {
  .floating-nav.compact {
    top: 6px;
    padding: 4px 6px;
    border-radius: 14px;
  }
  .floating-nav.compact .nav-pill {
    padding: 6px 8px;
  }

  .floating-main {
    padding: 78px 10px 10px;
  }

  .floating-main.has-dock {
    padding-bottom: 90px;
  }

  .floating-chat-widget {
    bottom: 16px;
    right: 8px;
    z-index: 50;
    opacity: 0.85;
  }
  .chat-widget-body {
    width: calc(100vw - 32px);
    max-width: 360px;
    max-height: 400px;
  }

  .floating-upload-bar {
    bottom: max(80px, calc(var(--safe-area-bottom, 0px) + 80px));
    left: 8px;
    right: 8px;
    width: auto;
    max-width: none;
    transform: none;
  }
}
</style>

<style>
/* 全局修复：移动端 Arco Switch 不变形 */
@media (max-width: 767px) {
  .arco-switch {
    flex-shrink: 0 !important;
  }
}
</style>
