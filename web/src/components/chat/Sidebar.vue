<template>
  <aside
    class="chat-sidebar"
    :class="{ open }"
  >
    <!-- Always-visible toggle button -->
    <button
      class="sidebar-toggle"
      :title="open ? '收起侧栏' : '展开侧栏'"
      @click="open = !open"
    >
      <svg
        width="15"
        height="15"
        viewBox="0 0 15 15"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path
          v-if="open"
          d="M10 4l-5 3.5 5 3.5"
        />
        <path
          v-else
          d="M5 4l5 3.5-5 3.5"
        />
      </svg>
    </button>

    <!-- New chat button -->
    <button
      class="sidebar-new-btn"
      :title="open ? '新建对话' : '新建'"
      @click="$emit('newChat')"
    >
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
      >
        <line x1="8" y1="2" x2="8" y2="14" /><line x1="2" y1="8" x2="14" y2="8" />
      </svg>
    </button>

    <!-- ── Collapsed: mini identifier blocks ── -->
    <div
      v-if="!open"
      class="sidebar-dots"
    >
      <div
        v-for="conv in collapsedItems"
        :key="conv.id"
        :class="['sidebar-dot', dotColorClass(conv), { active: conv.id === activeId }]"
        :title="dotTooltip(conv)"
        @click="$emit('select', conv.id)"
      >
        <span class="dot-char">{{ firstTwoChars(conv.title) }}</span>
      </div>
      <div
        v-if="overflowCount > 0"
        class="sidebar-dot overflow-dot"
        :title="'还有 ' + overflowCount + ' 个会话'"
      >
        <span class="dot-char">+{{ overflowCount }}</span>
      </div>
    </div>

    <!-- ── Expanded: search + time-grouped list ── -->
    <div
      v-else
      class="sidebar-body"
    >
      <!-- Search + actions header -->
      <div class="sidebar-head">
        <div class="sidebar-head-row">
          <span class="sidebar-head-title">对话</span>
          <button
            v-if="conversations.length > 0"
            class="sidebar-clear-btn"
            title="清空全部"
            @click="onClearAll"
          >
            <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M2 4h12M5 4V3a1 1 0 011-1h4a1 1 0 011 1v1M6 7v5M10 7v5M13 4l-1 9.5A1 1 0 0111 14H5a1 1 0 01-1-.5L3 4"/></svg>
          </button>
        </div>
        <div class="sidebar-search-wrap">
          <svg class="search-icon" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="7" cy="7" r="5"/><path d="M11 11l3.5 3.5"/></svg>
          <input
            v-model="searchQuery"
            class="sidebar-search"
            placeholder="搜索会话..."
            type="text"
          />
        </div>
      </div>

      <div class="sidebar-list">
        <!-- Pinned section (outside time groups) -->
        <template v-if="pinnedItems.length > 0">
          <div class="time-group-label">📌 置顶</div>
          <div
            v-for="conv in pinnedItems"
            :key="conv.id"
            :class="['sidebar-item', { active: conv.id === activeId }]"
            @click="$emit('select', conv.id)"
            @mouseenter="hoveredConv = conv.id"
            @mouseleave="hoveredConv = null"
          >
            <div class="sidebar-item-main">
              <!-- Inline rename -->
              <input
                v-if="renamingId === conv.id"
                ref="renameInput"
                v-model="renameValue"
                class="inline-rename-input"
                @keydown.enter="commitRename(conv.id)"
                @keydown.escape="cancelRename"
                @blur="commitRename(conv.id)"
                @click.stop
              />
              <span v-else class="sidebar-item-title">
                <template v-if="conv.isActive"><span class="active-dot" title="活跃中" /></template>
                <template v-if="isQQBot(conv.id)"><svg class="bot-icon" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="5" width="10" height="8" rx="2"/><circle cx="7" cy="9" r="1" fill="currentColor"/><circle cx="10" cy="9" r="1" fill="currentColor"/><line x1="6" y1="12" x2="11" y2="12" stroke="currentColor" stroke-linecap="round"/></svg></template>
                {{ conv.title }}
              </span>
              <span class="sidebar-item-meta">{{ fmtTime(conv.updatedAt) }} · {{ conv.messageCount }} 条</span>
            </div>
            <!-- Context menu button -->
            <button
              v-if="hoveredConv === conv.id"
              class="sidebar-menu-btn"
              title="更多操作"
              @click.stop="openMenu(conv.id, $event)"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="3" r="1.8"/><circle cx="8" cy="8" r="1.8"/><circle cx="8" cy="13" r="1.8"/></svg>
            </button>
            <!-- Sub-agents -->
            <div
              v-if="subAgents[conv.id]?.length"
              class="sidebar-subs"
            >
              <div
                v-for="sa in subAgents[conv.id]"
                :key="sa.id"
                :class="['sub-line', sa.status]"
                :title="sa.goal"
              >
                <StatusDot :status="sa.status" :size="8" class="sub-dot" />
                <span class="sub-goal">{{ sa.goal }}</span>
              </div>
            </div>
          </div>
        </template>

        <!-- Time-grouped items -->
        <template v-for="(group, gIdx) in filteredGroups" :key="gIdx">
          <div class="time-group-label">{{ group.label }}</div>
          <div
            v-for="conv in group.items"
            :key="conv.id"
            :class="['sidebar-item', { active: conv.id === activeId }]"
            @click="$emit('select', conv.id)"
            @mouseenter="hoveredConv = conv.id"
            @mouseleave="hoveredConv = null"
          >
            <div class="sidebar-item-main">
              <input
                v-if="renamingId === conv.id"
                ref="renameInput"
                v-model="renameValue"
                class="inline-rename-input"
                @keydown.enter="commitRename(conv.id)"
                @keydown.escape="cancelRename"
                @blur="commitRename(conv.id)"
                @click.stop
              />
              <span v-else class="sidebar-item-title">
                <template v-if="conv.isActive"><span class="active-dot" title="活跃中" /></template>
                <template v-if="isQQBot(conv.id)"><svg class="bot-icon" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="5" width="10" height="8" rx="2"/><circle cx="7" cy="9" r="1" fill="currentColor"/><circle cx="10" cy="9" r="1" fill="currentColor"/><line x1="6" y1="12" x2="11" y2="12" stroke="currentColor" stroke-linecap="round"/></svg></template>
                {{ conv.title }}
              </span>
              <span class="sidebar-item-meta">{{ fmtTime(conv.updatedAt) }} · {{ conv.messageCount }} 条</span>
            </div>
            <button
              v-if="hoveredConv === conv.id"
              class="sidebar-menu-btn"
              title="更多操作"
              @click.stop="openMenu(conv.id, $event)"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="3" r="1.8"/><circle cx="8" cy="8" r="1.8"/><circle cx="8" cy="13" r="1.8"/></svg>
            </button>
            <div
              v-if="subAgents[conv.id]?.length"
              class="sidebar-subs"
            >
              <div
                v-for="sa in subAgents[conv.id]"
                :key="sa.id"
                :class="['sub-line', sa.status]"
                :title="sa.goal"
              >
                <StatusDot :status="sa.status" :size="8" class="sub-dot" />
                <span class="sub-goal">{{ sa.goal }}</span>
              </div>
            </div>
          </div>
        </template>

        <div
          v-if="!filteredItems.length && !pinnedItems.length"
          class="sidebar-empty"
        >
          {{ searchQuery ? '无匹配会话' : '暂无对话' }}
        </div>
      </div>
    </div>

    <!-- ── Context menu (teleported to body via fixed position) ── -->
    <Teleport to="body">
      <div
        v-if="menuVisible"
        class="sidebar-context-menu"
        :style="{ top: menuY + 'px', left: menuX + 'px' }"
        @click.stop
      >
        <button class="menu-item" @click="startRename">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M11.5 1.5l3 3L5 14H2v-3L11.5 1.5z"/></svg>
          重命名
        </button>
        <button class="menu-item" @click="onTogglePin">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9.5 1.5l5 5L10 11l-5-5 4.5-4.5zM6 10l-4 4"/></svg>
          {{ isMenuConvPinned ? '取消置顶' : '置顶' }}
        </button>
        <div class="menu-divider"></div>
        <button class="menu-item menu-item-danger" @click="onDelete">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4h12M5 4V3a1 1 0 011-1h4a1 1 0 011 1v1M6 7v5M10 7v5M13 4l-1 9.5A1 1 0 0111 14H5a1 1 0 01-1-.5L3 4"/></svg>
          删除
        </button>
      </div>
    </Teleport>
    <!-- Click-away backdrop -->
    <Teleport to="body">
      <div
        v-if="menuVisible"
        class="sidebar-menu-backdrop"
        @click="closeMenu"
      ></div>
    </Teleport>

    <!-- Delete confirm modal -->
    <Teleport to="body">
      <div v-if="showDeleteConfirm" class="sidebar-modal-overlay" @click.self="showDeleteConfirm = false">
        <div class="sidebar-modal">
          <p class="sidebar-modal-text">确定要删除这个会话吗？此操作不可撤销。</p>
          <div class="sidebar-modal-actions">
            <button class="sidebar-modal-cancel" @click="showDeleteConfirm = false">取消</button>
            <button class="sidebar-modal-danger" @click="confirmDelete">删除</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Clear-all confirm modal -->
    <Teleport to="body">
      <div v-if="showClearConfirm" class="sidebar-modal-overlay" @click.self="showClearConfirm = false">
        <div class="sidebar-modal">
          <p class="sidebar-modal-text">确定要清空全部对话吗？此操作不可撤销。</p>
          <div class="sidebar-modal-actions">
            <button class="sidebar-modal-cancel" @click="showClearConfirm = false">取消</button>
            <button class="sidebar-modal-danger" @click="confirmClearAll">清空全部</button>
          </div>
        </div>
      </div>
    </Teleport>
  </aside>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, nextTick } from 'vue'
import StatusDot from '../icons/StatusDot.vue'

function isQQBot(id: string): boolean { return id?.startsWith('qqbot_') }

const props = defineProps({
  conversations: { type: Array, default: () => [] },
  activeId: { type: String, default: '' },
  subAgents: { type: Object, default: () => ({}) },
})
const emit = defineEmits(['select', 'newChat', 'rename', 'delete', 'togglePin'])

const open = ref(false)
const searchQuery = ref('')
const hoveredConv = ref<string | null>(null)

// ── Context menu ──
const menuVisible = ref(false)
const menuX = ref(0)
const menuY = ref(0)
const menuConvId = ref<string | null>(null)

// ── Inline rename ──
const renamingId = ref<string | null>(null)
const renameValue = ref('')
const renameInput = ref<any>(null)

// ── Clear all ──
const showClearConfirm = ref(false)
const showDeleteConfirm = ref(false)

// ── Collapsed state helpers ──
const MAX_COLLAPSED = 12

const collapsedItems = computed(() => {
  return props.conversations.slice(0, MAX_COLLAPSED)
})

const overflowCount = computed(() => {
  return Math.max(0, props.conversations.length - MAX_COLLAPSED)
})

function firstTwoChars(title: string): string {
  if (!title) return '新'
  // Take first 2 characters (handle CJK + ASCII)
  return title.slice(0, 2)
}

function dotColorClass(conv: any): string {
  if (conv.id === props.activeId && conv.isActive) return 'dot-active-stream'
  if (conv.pinned) return 'dot-pinned'
  if (conv.isActive) return 'dot-active'
  return 'dot-normal'
}

function dotTooltip(conv: any): string {
  const t = fmtTime(conv.updatedAt)
  return conv.title + '\n' + conv.messageCount + ' 条 · ' + t
}

// ── Time grouping ──
interface TimeGroup {
  label: string
  items: any[]
}

const groupedConversations = computed<TimeGroup[]>(() => {
  const now = new Date()
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterdayStart = new Date(todayStart.getTime() - 86400000)
  const weekStart = new Date(todayStart.getTime() - 6 * 86400000)

  const groups: TimeGroup[] = [
    { label: '今天', items: [] },
    { label: '昨天', items: [] },
    { label: '本周', items: [] },
    { label: '更早', items: [] },
  ]

  for (const conv of props.conversations) {
    // Skip pinned — they display in separate section
    if ((conv as any).pinned) continue

    // Apply search filter
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase()
      const matchTitle = (conv as any).title?.toLowerCase().includes(q)
      const matchSubs = (props.subAgents[(conv as any).id] || []).some(
        (s: any) => s.goal?.toLowerCase().includes(q)
      )
      if (!matchTitle && !matchSubs) continue
    }

    const d = new Date((conv as any).updatedAt)
    if (d >= todayStart) {
      groups[0].items.push(conv)
    } else if (d >= yesterdayStart) {
      groups[1].items.push(conv)
    } else if (d >= weekStart) {
      groups[2].items.push(conv)
    } else {
      groups[3].items.push(conv)
    }
  }

  return groups.filter(g => g.items.length > 0)
})

// Pinned items with optional search filter
const pinnedItems = computed(() => {
  let items = props.conversations.filter((c: any) => c.pinned)
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    items = items.filter((c: any) =>
      c.title?.toLowerCase().includes(q) ||
      (props.subAgents[c.id] || []).some((s: any) => s.goal?.toLowerCase().includes(q))
    )
  }
  return items
})

// Flat filtered items (for empty state check)
const filteredItems = computed(() => {
  let items = props.conversations
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    items = items.filter((c: any) =>
      c.title?.toLowerCase().includes(q) ||
      (props.subAgents[c.id] || []).some((s: any) => s.goal?.toLowerCase().includes(q))
    )
  }
  return items
})

// Filtered time groups (reactive to searchQuery)
const filteredGroups = computed(() => groupedConversations.value)

// ── Context menu logic ──
function openMenu(convId: string, event?: MouseEvent) {
  menuConvId.value = convId
  // Position near the mouse if event is provided, otherwise fallback
  if (event) {
    menuX.value = event.clientX
    menuY.value = event.clientY
  }
  menuVisible.value = true
  // Fallback positioning if no event
  if (!event) {
    menuX.value = 200
    menuY.value = 200
  }
}

function closeMenu() {
  menuVisible.value = false
  menuConvId.value = null
}

const isMenuConvPinned = computed(() => {
  if (!menuConvId.value) return false
  const conv = props.conversations.find((c: any) => c.id === menuConvId.value)
  return !!(conv as any)?.pinned
})

function startRename() {
  if (!menuConvId.value) return
  const conv = props.conversations.find((c: any) => c.id === menuConvId.value)
  renamingId.value = menuConvId.value
  renameValue.value = (conv as any)?.title || ''
  closeMenu()
  nextTick(() => {
    const inputs = document.querySelectorAll('.inline-rename-input')
    const last = inputs[inputs.length - 1] as HTMLInputElement
    if (last) { last.focus(); last.select() }
  })
}

function cancelRename() {
  renamingId.value = null
  renameValue.value = ''
}

async function commitRename(id: string) {
  if (!renamingId.value) return
  const val = renameValue.value.trim()
  renamingId.value = null
  renameValue.value = ''
  if (val && val !== (props.conversations.find((c: any) => c.id === id) as any)?.title) {
    emit('rename', id, val)
  }
}

function onTogglePin() {
  if (!menuConvId.value) return
  emit('togglePin', menuConvId.value, !isMenuConvPinned.value)
  closeMenu()
}

function onDelete() {
  if (!menuConvId.value) return
  closeMenu()
  showDeleteConfirm.value = true
}
function confirmDelete() {
  showDeleteConfirm.value = false
  emit('delete', menuConvId.value)
}

function onClearAll() {
  showClearConfirm.value = true
}

async function confirmClearAll() {
  showClearConfirm.value = false
  const token = localStorage.getItem('token')
  try {
    await fetch('/api/chat/clear-all', {
      method: 'POST',
      headers: { Authorization: 'Bearer ' + token },
    })
  } catch (e) { /* ignore */ }
  emit('newChat')
}

// ── Formatting ──
function fmtTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前'
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前'
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}
</script>

<style scoped>
.chat-sidebar {
  position: relative;
  width: 56px; flex-shrink: 0;
  background: var(--glass-bg-nav);
  backdrop-filter: blur(var(--glass-blur-nav));
  -webkit-backdrop-filter: blur(var(--glass-blur-nav));
  border-right: 1px solid var(--glass-border-strong);
  display: flex; flex-direction: column; align-items: center;
  padding: 14px 0;
  transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  z-index: 10;
}
.chat-sidebar.open {
  width: 280px;
  align-items: stretch;
}

/* ── Toggle ── */
.sidebar-toggle {
  width: 34px; height: 34px; border-radius: 8px; border: none;
  background: transparent; color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all 0.12s; flex-shrink: 0;
  margin: 0 auto;
}
.sidebar-toggle:hover { background: var(--surface-hover); color: var(--brand-500); }
.chat-sidebar.open .sidebar-toggle { margin: 0 12px; }

/* ── New chat ── */
.sidebar-new-btn {
  width: 34px; height: 34px; border-radius: 8px; border: none;
  background: transparent; color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all 0.12s; flex-shrink: 0; margin: 6px auto;
}
.sidebar-new-btn:hover { background: var(--brand-50); color: var(--brand-500); }
.chat-sidebar.open .sidebar-new-btn { margin: 6px 12px; width: calc(100% - 24px); justify-content: flex-start; gap: 8px; padding-left: 10px; }
.chat-sidebar.open .sidebar-new-btn::after { content: '新建对话'; font-size: 12px; font-weight: 500; font-family: inherit; }

/* ── Collapsed: colored dots with first 2 chars ── */
.sidebar-dots {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  gap: 6px; padding-top: 10px; overflow-y: auto;
}
.sidebar-dot {
  width: 32px; height: 32px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: all 0.15s;
  position: relative;
}
.sidebar-dot:hover { background: var(--surface-hover); transform: scale(1.08); }
.sidebar-dot.active { background: var(--brand-50); }
.dot-char {
  font-size: 11px; font-weight: 600;
  color: var(--text-secondary);
  line-height: 1;
}

/* Color states for collapsed dots */
.sidebar-dot.dot-normal .dot-char { color: var(--text-muted); }
.sidebar-dot.dot-active .dot-char { color: var(--brand-500); }
.sidebar-dot.dot-active { box-shadow: inset 0 0 0 2px var(--brand-500); }
.sidebar-dot.dot-active-stream { animation: dotPulse 1.5s ease-in-out infinite; }
.sidebar-dot.dot-active-stream .dot-char { color: #f59e0b; }
.sidebar-dot.dot-pinned { box-shadow: inset 0 0 0 2px #eab308; }
.sidebar-dot.dot-pinned .dot-char { color: #ca8a04; }
.sidebar-dot.overflow-dot { background: var(--surface-hover); }
.sidebar-dot.overflow-dot .dot-char { color: var(--text-muted); font-size: 10px; }

@keyframes dotPulse {
  0%, 100% { box-shadow: inset 0 0 0 2px rgba(245, 158, 11, 0.3); }
  50% { box-shadow: inset 0 0 0 2px rgba(245, 158, 11, 0.8); }
}

/* ── Expanded body ── */
.sidebar-body {
  flex: 1; display: flex; flex-direction: column;
  padding: 0 10px; overflow: hidden; margin-top: 4px;
}

/* Header row */
.sidebar-head {
  padding: 6px 0 8px;
}
.sidebar-head-row {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 10px; margin-bottom: 6px;
}
.sidebar-head-title {
  font-size: 12px; font-weight: 600; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.8px;
}
.sidebar-clear-btn {
  width: 26px; height: 26px; border-radius: 6px; border: none;
  background: transparent; color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all 0.12s;
}
.sidebar-clear-btn:hover { background: rgba(248, 81, 73, 0.1); color: var(--color-danger, #f85149); }

/* Search */
.sidebar-search-wrap {
  display: flex; align-items: center; gap: 6px;
  padding: 4px 10px; margin: 0 2px;
  border-radius: 8px;
  background: var(--surface-input, rgba(255,255,255,0.04));
  border: 1px solid var(--border-subtle);
  transition: border-color 0.15s;
}
.sidebar-search-wrap:focus-within {
  border-color: var(--brand-400);
  box-shadow: 0 0 0 3px rgba(6, 182, 212, 0.08);
}
.search-icon { flex-shrink: 0; color: var(--text-muted); }
.sidebar-search {
  flex: 1; border: none; background: transparent;
  font-size: 12px; color: var(--text-primary);
  outline: none; font-family: inherit;
}
.sidebar-search::placeholder { color: var(--text-muted); }

/* Time group labels */
.time-group-label {
  font-size: 10.5px; font-weight: 600; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.6px;
  padding: 10px 10px 4px; margin-top: 2px;
}

/* List */
.sidebar-list {
  flex: 1; overflow-y: auto; padding-bottom: 8px;
}
.sidebar-list::-webkit-scrollbar { width: 2px; }
.sidebar-list::-webkit-scrollbar-thumb { background: var(--border-default); border-radius: 2px; }

.sidebar-item {
  padding: 8px 10px; border-radius: 8px;
  cursor: pointer; transition: all 0.12s;
  margin-bottom: 2px; position: relative;
}
.sidebar-item:hover { background: var(--surface-hover); }
.sidebar-item.active {
  background: var(--brand-50);
  box-shadow: inset 3px 0 0 var(--brand-500);
}
[data-theme="dark"] .sidebar-item.active {
  background: rgba(6,182,212,0.08);
  box-shadow: inset 3px 0 0 #22d3ee;
}
.sidebar-item-main { display: flex; flex-direction: column; gap: 1px; }
.sidebar-item-title {
  font-size: 12.5px; font-weight: 500;
  color: var(--text-primary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  display: flex; align-items: center; gap: 6px;
}
.sidebar-item.active .sidebar-item-title { font-weight: 600; }
.sidebar-item-meta { font-size: 10.5px; color: var(--text-muted); }

/* Active indicator dot */
.active-dot {
  width: 7px; height: 7px; border-radius: 50%;
  background: var(--brand-500);
  flex-shrink: 0;
  animation: activePulse 2s ease-in-out infinite;
  box-shadow: 0 0 4px rgba(6, 182, 212, 0.5);
}
@keyframes activePulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.85); }
}

/* Context menu button */
.sidebar-menu-btn {
  position: absolute; right: 8px; top: 50%; transform: translateY(-50%);
  width: 24px; height: 24px; border-radius: 6px; border: none;
  background: var(--surface-card); color: var(--text-muted);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all 0.12s; z-index: 2;
}
.sidebar-menu-btn:hover { background: var(--surface-hover); color: var(--text-primary); }

/* Inline rename */
.inline-rename-input {
  width: 100%; padding: 3px 6px; border-radius: 4px;
  border: 1px solid var(--brand-400);
  background: var(--surface-card);
  color: var(--text-primary);
  font-size: 12.5px; font-weight: 500; font-family: inherit;
  outline: none;
}

/* Sub-agents */
.sidebar-subs {
  margin-top: 4px; padding-left: 10px;
  border-left: 2px solid var(--border-subtle);
  display: flex; flex-direction: column; gap: 2px;
}
.sub-line { display: flex; align-items: center; gap: 4px; font-size: 10.5px; padding: 1px 0; }
.sub-icon { font-size: 9px; flex-shrink: 0; width: 11px; text-align: center; }
.sub-line.running .sub-icon { color: var(--brand-500); animation: pulse 1.2s ease-in-out infinite; }
.sub-line.done .sub-icon { color: #22c55e; }
.sub-line.error .sub-icon { color: var(--color-danger); }
.sub-goal { color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.sidebar-empty { padding: 20px 10px; text-align: center; color: var(--text-muted); font-size: 12px; }

@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }

/* ── Context menu (global) ── */
.sidebar-context-menu {
  position: fixed; z-index: 9999;
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  box-shadow: 0 8px 30px rgba(0,0,0,0.15);
  padding: 6px; min-width: 160px;
  animation: menuIn 0.15s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes menuIn {
  from { opacity: 0; transform: scale(0.95) translateY(-4px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}
.menu-item {
  display: flex; align-items: center; gap: 8px;
  width: 100%; padding: 8px 10px; border: none; border-radius: 6px;
  background: transparent; color: var(--text-primary);
  font-size: 12.5px; font-family: inherit; cursor: pointer;
  transition: background 0.1s;
}
.menu-item:hover { background: var(--surface-hover); }
.menu-item-danger { color: var(--color-danger, #f85149); }
.menu-item-danger:hover { background: rgba(248, 81, 73, 0.08); }
.menu-divider { height: 1px; background: var(--border-subtle); margin: 4px 6px; }

.sidebar-menu-backdrop {
  position: fixed; inset: 0; z-index: 9998;
}

/* ── Clear-all modal ── */
.sidebar-modal-overlay {
  position: fixed; inset: 0; z-index: 10000;
  background: rgba(0,0,0,0.35);
  display: flex; align-items: center; justify-content: center;
  animation: fadeIn 0.15s;
}
.sidebar-modal {
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  padding: 24px; max-width: 360px; width: 90%;
  box-shadow: 0 12px 40px rgba(0,0,0,0.2);
}
.sidebar-modal-text { font-size: 14px; color: var(--text-primary); margin: 0 0 20px; line-height: 1.5; }
.sidebar-modal-actions { display: flex; gap: 10px; justify-content: flex-end; }
.sidebar-modal-cancel {
  padding: 7px 18px; border-radius: 8px; border: 1px solid var(--border-default);
  background: transparent; color: var(--text-secondary); font-size: 13px;
  font-family: inherit; cursor: pointer;
}
.sidebar-modal-cancel:hover { background: var(--surface-hover); }
.sidebar-modal-danger {
  padding: 7px 18px; border-radius: 8px; border: none;
  background: var(--color-danger, #f85149); color: #fff;
  font-size: 13px; font-family: inherit; cursor: pointer; font-weight: 500;
}
.sidebar-modal-danger:hover { opacity: 0.9; }

@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

/* ── Responsive ── */
@media (max-width: 767px) {
  .chat-sidebar {
    position: fixed; left: 0; top: 0; bottom: 0; z-index: 400;
    width: 0; box-shadow: none;
  }
  .chat-sidebar.open { width: 280px; box-shadow: 6px 0 24px rgba(0,0,0,0.15); }
}
</style>
