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
        <line
          x1="8"
          y1="2"
          x2="8"
          y2="14"
        /><line
          x1="2"
          y1="8"
          x2="14"
          y2="8"
        />
      </svg>
    </button>

    <!-- Collapsed: mini dot indicators -->
    <div
      v-if="!open"
      class="sidebar-dots"
    >
      <div
        v-for="conv in conversations.slice(0, 8)"
        :key="conv.id"
        :class="['sidebar-dot', { active: conv.id === activeId }]"
        :title="conv.title"
        @click="$emit('select', conv.id)"
      >
        <span class="dot-core"></span>
      </div>
    </div>

    <!-- Expanded: full conversation list -->
    <div
      v-else
      class="sidebar-body"
    >
      <div class="sidebar-head">
        对话
      </div>

      <div class="sidebar-list">
        <div
          v-for="conv in conversations"
          :key="conv.id"
          :class="['sidebar-item', { active: conv.id === activeId }]"
          @click="$emit('select', conv.id)"
        >
          <div class="sidebar-item-main">
            <span class="sidebar-item-title">{{ conv.title }}</span>
            <span class="sidebar-item-meta">{{ conv.messageCount }} 条 · {{ fmtTime(conv.updatedAt) }}</span>
          </div>
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
              <span class="sub-icon">{{ sa.status === 'done' ? '✓' : sa.status === 'error' ? '✗' : '⋯' }}</span>
              <span class="sub-goal">{{ sa.goal }}</span>
            </div>
          </div>
        </div>
        <div
          v-if="!conversations.length"
          class="sidebar-empty"
        >
          暂无对话
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref } from 'vue'

defineProps({
  conversations: { type: Array, default: () => [] },
  activeId: { type: String, default: '' },
  subAgents: { type: Object, default: () => ({}) },
})
defineEmits(['select', 'newChat'])

const open = ref(false)

function fmtTime(t) {
  if (!t) return ''
  const d = new Date(t)
  const now = new Date()
  const diff = now - d
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
  width: 260px;
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

/* ── Collapsed dot indicators ── */
.sidebar-dots {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  gap: 12px; padding-top: 10px; overflow-y: auto;
}
.sidebar-dot {
  width: 32px; height: 32px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: all 0.12s;
}
.sidebar-dot:hover { background: var(--surface-hover); }
.sidebar-dot.active { background: var(--brand-50); }
.dot-core {
  width: 8px; height: 8px; border-radius: 50%;
  background: var(--border-default);
  transition: all 0.12s;
}
.sidebar-dot:hover .dot-core { background: var(--brand-400); transform: scale(1.4); }
.sidebar-dot.active .dot-core { background: var(--brand-500); box-shadow: 0 0 6px rgba(6,182,212,0.35); transform: scale(1.2); }

/* ── Expanded body ── */
.sidebar-body {
  flex: 1; display: flex; flex-direction: column;
  padding: 0 10px; overflow: hidden; margin-top: 4px;
}
.sidebar-head {
  font-size: 12px; font-weight: 600; color: var(--text-muted);
  text-transform: uppercase; letter-spacing: 0.8px;
  padding: 6px 10px;
}
.sidebar-list {
  flex: 1; overflow-y: auto; padding-bottom: 8px;
}
.sidebar-list::-webkit-scrollbar { width: 2px; }
.sidebar-list::-webkit-scrollbar-thumb { background: var(--border-default); border-radius: 2px; }

.sidebar-item {
  padding: 8px 10px; border-radius: 8px;
  cursor: pointer; transition: all 0.12s;
  margin-bottom: 2px;
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
}
.sidebar-item.active .sidebar-item-title { font-weight: 600; }
.sidebar-item-meta { font-size: 10.5px; color: var(--text-muted); }

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

@media (max-width: 767px) {
  .chat-sidebar {
    position: fixed; left: 0; top: 0; bottom: 0; z-index: 400;
    width: 0; box-shadow: none;
  }
  .chat-sidebar.open { width: 280px; box-shadow: 6px 0 24px rgba(0,0,0,0.15); }
}
</style>
