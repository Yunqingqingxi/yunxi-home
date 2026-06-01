<template>
  <div
    v-if="tasks.length"
    class="mip-bar"
    :class="{ collapsed }"
  >
    <div
      class="mip-head"
      @click="collapsed = !collapsed"
    >
      <span class="mip-chevron">{{ collapsed ? '▶' : '▼' }}</span>
      <span class="mip-title">MCP 安装</span>
      <span class="mip-summary">{{ runningCount }} 进行中 · {{ doneCount }} 完成</span>
      <button
        v-if="!runningCount"
        class="mip-dismiss"
        @click.stop="$emit('clear')"
      >
        ✕
      </button>
    </div>
    <div
      v-if="!collapsed"
      class="mip-body"
    >
      <div
        v-for="t in tasks"
        :key="t.id"
        class="mip-task"
        :class="t.status"
      >
        <div class="mip-task-head">
          <span class="mip-task-name">{{ t.package }}</span>
          <span class="mip-task-pct">{{ t.progress }}%</span>
        </div>
        <div class="mip-track">
          <div
            class="mip-fill"
            :class="t.status"
            :style="{ width: t.progress + '%' }"
          ></div>
        </div>
        <div class="mip-steps">
          <div
            v-for="s in t.steps"
            :key="s.step"
            class="mip-step"
            :class="s.status"
          >
            <span class="mip-step-icon">{{ s.status === 'done' ? '✅' : s.status === 'error' ? '❌' : s.status === 'running' ? '⏳' : '○' }}</span>
            <span class="mip-step-msg">{{ s.message }}</span>
          </div>
        </div>
        <div
          v-if="t.error"
          class="mip-error"
        >
          {{ t.error }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed } from 'vue'

const props = defineProps({
  tasks: { type: Array, default: () => [] }
})
defineEmits(['clear'])

const collapsed = ref(false)
const runningCount = computed(() => props.tasks.filter(t => t.status === 'running').length)
const doneCount = computed(() => props.tasks.filter(t => t.status === 'done' || t.status === 'error').length)
</script>

<style scoped>
.mip-bar { position: fixed; bottom: 20px; right: 20px; z-index: 300; width: 380px; max-width: calc(100vw - 40px); background: var(--glass-bg-elevated); border: 1px solid var(--glass-border-strong); border-radius: 14px; box-shadow: var(--glass-shadow-elevated); overflow: hidden; transition: all 0.2s; }
.mip-bar.collapsed { width: auto; }
.mip-head { display: flex; align-items: center; gap: 8px; padding: 10px 14px; cursor: pointer; user-select: none; font-size: 13px; }
.mip-chevron { font-size: 10px; color: var(--text-muted); }
.mip-title { font-weight: 700; color: var(--text-primary); }
.mip-summary { font-size: 11px; color: var(--text-muted); margin-left: auto; }
.mip-dismiss { width: 22px; height: 22px; border: none; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; }
.mip-dismiss:hover { background: rgba(220,38,38,0.08); color: var(--color-danger); }

.mip-body { padding: 0 14px 10px; max-height: 400px; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; }
.mip-task { display: flex; flex-direction: column; gap: 6px; padding: 8px 10px; border-radius: 8px; background: rgba(0,0,0,0.02); }
.mip-task.done { opacity: 0.7; }
.mip-task-head { display: flex; justify-content: space-between; font-size: 12px; }
.mip-task-name { font-weight: 600; color: var(--text-primary); }
.mip-task-pct { font-size: 11px; color: var(--brand-500); font-weight: 600; }
.mip-track { height: 3px; border-radius: 2px; background: var(--bg-progress-track); overflow: hidden; }
.mip-fill { height: 100%; border-radius: 2px; background: var(--brand-500); transition: width 0.3s; }
.mip-fill.error { background: var(--color-danger); }
.mip-fill.done { background: var(--color-success); }

.mip-steps { display: flex; flex-direction: column; gap: 3px; }
.mip-step { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--text-muted); }
.mip-step.running { color: var(--brand-600); font-weight: 500; }
.mip-step.done { color: var(--color-success); }
.mip-step.error { color: var(--color-danger); }
.mip-step-icon { font-size: 11px; width: 16px; text-align: center; }
.mip-step-msg { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mip-error { font-size: 10px; color: var(--color-danger); padding: 4px 8px; background: rgba(220,38,38,0.06); border-radius: 4px; white-space: pre-wrap; word-break: break-all; }
</style>
