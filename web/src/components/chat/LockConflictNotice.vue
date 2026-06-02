<template>
  <div v-if="conflicts.length > 0" class="lock-notices">
    <div v-for="(c, i) in conflicts" :key="i" class="notice-card" :class="c.decision">
      <div class="notice-head">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="11" width="12" height="9" rx="2"/><path d="M8 11V7a4 4 0 118 0v4"/></svg>
        <span class="notice-title">资源冲突</span>
        <span class="notice-time">{{ new Date().toLocaleTimeString() }}</span>
      </div>
      <div class="notice-body">
        <div class="notice-row"><span class="k">资源</span><span class="v">{{ c.resource_id }}</span></div>
        <div class="notice-row"><span class="k">冲突方</span><span class="v">{{ c.agents.join(' vs ') }}</span></div>
        <div class="notice-row"><span class="k">决策</span><span class="v decision-text">{{ decisionText(c.decision) }}</span></div>
        <div class="notice-row" v-if="c.winner"><span class="k">获得锁</span><span class="v winner">{{ c.winner }}</span></div>
        <div class="notice-reason" v-if="c.reason">{{ c.reason }}</div>
      </div>
      <div v-if="c.decision === 'human_escalate'" class="notice-actions">
        <button class="act-btn deny" @click="$emit('resolve', c, 'deny')">拒绝</button>
        <button class="act-btn allow" @click="$emit('resolve', c, 'allow')">放行</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { LockConflict } from '../../types/chat'

defineProps<{ conflicts: LockConflict[] }>()
defineEmits<{ resolve: [conflict: LockConflict, action: string] }>()

function decisionText(d: string): string {
  switch (d) {
    case 'yield': return '优先级抢占 — 低优先级让出'
    case 'wait': return '等待重试'
    case 'suspend': return '强制暂停低优先级方'
    case 'escalate': return '需要人工决策'
    default: return d
  }
}
</script>

<style scoped>
.lock-notices { display: flex; flex-direction: column; gap: 8px; margin: 4px 0; }
.notice-card { border: 1px solid #f59e0b; border-radius: 8px; padding: 10px 12px; background: rgba(245,158,11,0.04); }
.notice-head { display: flex; align-items: center; gap: 6px; color: #f59e0b; margin-bottom: 6px; }
.notice-title { font-size: 12px; font-weight: 600; }
.notice-time { font-size: 10px; color: var(--text-muted); margin-left: auto; }
.notice-body { display: flex; flex-direction: column; gap: 3px; }
.notice-row { display: flex; gap: 8px; font-size: 11px; }
.k { color: var(--text-muted); min-width: 44px; }
.v { color: var(--text-primary); font-family: monospace; font-size: 10.5px; }
.winner { color: #22c55e; }
.notice-reason { font-size: 10px; color: var(--text-secondary); margin-top: 2px; }
.notice-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 8px; }
.act-btn { padding: 4px 16px; border-radius: 6px; font-size: 12px; cursor: pointer; border: 1px solid; }
.act-btn.deny { color: #ef4444; border-color: #ef4444; background: transparent; }
.act-btn.allow { color: #22c55e; border-color: #22c55e; background: transparent; }
@media (prefers-color-scheme: dark) { .notice-card { background: rgba(245,158,11,0.06); } }
</style>
