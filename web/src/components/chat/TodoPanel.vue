<template>
  <div v-if="items.length > 0" class="todo-panel" :class="{ collapsed: collapsed }">
    <div class="todo-header" @click="collapsed = !collapsed">
      <div class="todo-header-left">
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5">
          <rect x="2" y="2" width="10" height="10" rx="2"/>
          <path d="M4.5 7l2 2 3-4" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span class="todo-title">任务进度</span>
        <span class="todo-count">{{ doneCount }}/{{ items.length }}</span>
      </div>
      <div class="todo-header-right">
        <!-- Mini progress ring -->
        <svg width="20" height="20" viewBox="0 0 20 20" class="progress-ring">
          <circle cx="10" cy="10" r="8" fill="none" stroke="var(--border-default)" stroke-width="2"/>
          <circle
            cx="10" cy="10" r="8"
            fill="none"
            stroke="var(--brand-500)"
            stroke-width="2"
            stroke-linecap="round"
            :stroke-dasharray="circumference"
            :stroke-dashoffset="dashOffset"
            transform="rotate(-90 10 10)"
            class="progress-ring-fill"
          />
        </svg>
        <svg :class="['chevron', { flipped: !collapsed }]" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M3 3.5l2 3 2-3"/>
        </svg>
      </div>
    </div>

    <div v-if="!collapsed" class="todo-body">
      <div
        v-for="item in items"
        :key="item.id"
        class="todo-item"
        :class="item.status"
      >
        <span class="todo-status-icon">{{ statusIcon(item.status) }}</span>
        <span class="todo-content">{{ item.active_form || item.content }}</span>
        <span v-if="item.status === 'in_progress'" class="todo-pulse"></span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  items: { type: Array, default: () => [] }
})

const collapsed = ref(false)
const circumference = 2 * Math.PI * 8 // ~50.26

const doneCount = computed(() => props.items.filter(i => i.status === 'completed').length)
const progressPercent = computed(() => {
  if (props.items.length === 0) return 0
  return (doneCount.value / props.items.length) * 100
})
const dashOffset = computed(() => {
  return circumference - (progressPercent.value / 100) * circumference
})

function statusIcon(status) {
  switch (status) {
    case 'in_progress': return '🔄'
    case 'completed':  return '✅'
    default:           return '⏳'
  }
}
</script>

<style scoped>
.todo-panel {
  background: var(--surface-raised);
  -webkit-border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
  margin-bottom: 10px;
  overflow: hidden;
  animation: todoSlideIn 0.35s var(--ease-out-back);
  flex-shrink: 0;
}

@keyframes todoSlideIn {
  from { opacity: 0; transform: translateY(-8px) scale(0.97); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}

.todo-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 12px; cursor: pointer; user-select: none;
  transition: background 0.15s;
}
.todo-header:hover { background: var(--surface-hover); }

.todo-header-left {
  display: flex; align-items: center; gap: 6px;
  color: var(--brand-600);
}
[data-theme="dark"] .todo-header-left { color: var(--brand-400); }

.todo-title {
  font-size: 12px; font-weight: var(--weight-semibold);
}
.todo-count {
  font-size: 10px; color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

.todo-header-right {
  display: flex; align-items: center; gap: 6px;
}

.progress-ring-fill {
  transition: stroke-dashoffset 0.5s var(--ease-out-expo);
}

.chevron {
  color: var(--text-muted); transition: transform 0.2s;
  flex-shrink: 0;
}
.chevron.flipped { transform: rotate(180deg); }

.todo-body {
  padding: 0 12px 10px;
  display: flex; flex-direction: column; gap: 2px;
  animation: bodyIn 0.25s var(--ease-out-expo);
}
@keyframes bodyIn {
  from { opacity: 0; max-height: 0; }
  to   { opacity: 1; max-height: 500px; }
}

.todo-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 8px; border-radius: 8px;
  font-size: 12px; color: var(--text-secondary);
  transition: all 0.3s var(--ease-out-expo);
  border: 1px solid transparent;
}

.todo-item.in_progress {
  color: var(--text-primary);
  background: rgba(6,182,212,0.06);
  border-color: rgba(6,182,212,0.18);
  animation: itemPulse 2.2s ease-in-out infinite;
}
[data-theme="dark"] .todo-item.in_progress {
  background: rgba(34,211,238,0.08);
  border-color: rgba(34,211,238,0.22);
}

@keyframes itemPulse {
  0%, 100% { box-shadow: 0 0 0 0 rgba(6,182,212,0.12); }
  50%      { box-shadow: 0 0 0 4px rgba(6,182,212,0); }
}

.todo-item.completed {
  color: var(--text-muted);
  text-decoration: none;
  opacity: 0.75;
}

.todo-status-icon { font-size: 13px; flex-shrink: 0; }

.todo-content {
  flex: 1; line-height: 1.4;
}

.todo-pulse {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--brand-500);
  animation: dotPulse 1.4s ease-in-out infinite;
  flex-shrink: 0;
}
@keyframes dotPulse {
  0%, 100% { opacity: 0.3; transform: scale(0.8); }
  50%      { opacity: 1;   transform: scale(1.2); }
}

/* Collapsed state */
.todo-panel.collapsed { opacity: 0.85; }
.todo-panel.collapsed:hover { opacity: 1; }

/* Dark theme polish */
[data-theme="dark"] .todo-panel {
  background: rgba(18,26,44,0.55);
  border-color: rgba(255,255,255,0.07);
}

/* Mobile */
@media (max-width: 767px) {
  .todo-header { padding: 6px 10px; }
  .todo-title { font-size: 11px; }
  .todo-item { font-size: 11px; padding: 5px 6px; }
}
</style>
