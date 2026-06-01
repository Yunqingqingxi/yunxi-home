<template>
  <div v-if="tasks.length" class="upload-panel" :class="{ collapsed }">
    <div class="up-head" @click="collapsed = !collapsed">
      <span>{{ hasActive ? '上传中' : '已完成' }} ({{ tasks.length }})</span>
      <button v-if="!hasActive" class="up-dismiss" @click.stop="$emit('clear')">✕</button>
    </div>
    <div v-if="!collapsed" class="up-body">
      <div v-for="t in tasks" :key="t.id" class="up-item">
        <div class="up-name">{{ t.name }}</div>
        <div class="up-track"><div class="up-fill" :style="{ width: t.progress + '%' }" :class="t.status"></div></div>
        <div class="up-meta">
          <span :class="['up-status', t.status]">{{ t.status === 'uploading' ? t.progress + '%' : t.status === 'done' ? '✓' : '✗' }}</span>
          <button v-if="t.status === 'uploading'" class="up-cancel" @click="$emit('cancel', t.id)">✕</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref } from 'vue'
defineProps({ tasks: Array, hasActive: Boolean })
defineEmits(['cancel', 'clear'])
const collapsed = ref(false)
</script>

<style scoped>
.upload-panel { width: 240px; flex-shrink: 0; border: 1px solid var(--border-default); border-radius: var(--radius-lg); overflow: hidden; background: var(--glass-bg-card); display: flex; flex-direction: column; }
.upload-panel.collapsed { width: auto; }
.up-head { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; font-size: 12px; font-weight: 600; cursor: pointer; border-bottom: 1px solid var(--border-subtle); }
.up-dismiss { border: none; background: none; cursor: pointer; font-size: 14px; color: var(--text-muted); }
.up-body { flex: 1; overflow-y: auto; padding: 8px; }
.up-item { padding: 6px 0; border-bottom: 1px solid var(--border-subtle); }
.up-item:last-child { border-bottom: none; }
.up-name { font-size: 11px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.up-track { height: 3px; border-radius: 2px; background: var(--surface-hover); margin: 3px 0; overflow: hidden; }
.up-fill { height: 100%; background: var(--brand-500); transition: width 0.2s; }
.up-fill.done { background: #22c55e; }
.up-fill.error { background: var(--color-danger); }
.up-meta { display: flex; align-items: center; justify-content: space-between; font-size: 10px; }
.up-status { color: var(--text-muted); }
.up-status.done { color: #22c55e; }
.up-status.error { color: var(--color-danger); }
.up-cancel { border: none; background: none; cursor: pointer; color: var(--text-muted); font-size: 10px; }
</style>
