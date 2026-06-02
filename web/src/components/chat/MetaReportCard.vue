<template>
  <div v-if="visible" class="meta-card">
    <div class="mc-head" @click="open = !open">
      <span>助手自评</span>
      <svg :class="{ rotated: open }" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4,6 8,10 12,6"/></svg>
    </div>
    <div v-if="open && report" class="mc-body">
      <div class="mc-row"><span class="mc-k">成功率</span><span class="mc-v ok">{{ ((report.success_rate || 0) * 100).toFixed(0) }}%</span></div>
      <div class="mc-row"><span class="mc-k">均延迟</span><span class="mc-v">{{ (report.avg_latency_ms || 0).toFixed(0) }}ms</span></div>
      <div class="mc-row"><span class="mc-k">完成</span><span class="mc-v">{{ report.task_completed || 0 }} / 失败 {{ report.task_failed || 0 }}</span></div>
      <div class="mc-row"><span class="mc-k">冲突</span><span class="mc-v" :class="{ warn: (report.conflict_count || 0) > 3 }">{{ report.conflict_count || 0 }} 次</span></div>
      <div class="mc-load"><span class="bar" :style="{ width: ((report.current_load || 0) * 100).toFixed(0) + '%' }" /></div>
      <div class="mc-role" v-if="report.role">
        <RoleIcon :role="report.role" :size="14" />
        <span>{{ roleLabel(report.role) }}</span>
        <span v-if="report.role_ttl" class="mc-ttl">{{ report.role_ttl }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import RoleIcon from '../icons/RoleIcon.vue'
import type { MetaReport } from '../../types/chat'

const props = defineProps<{ report: MetaReport | null; visible: boolean }>()
const open = ref(true)

function roleLabel(r: string): string {
  switch (r) { case 'executor': return '执行者'; case 'supervisor': return '监督者'; case 'manager': return '管理者'; default: return r }
}
</script>

<style scoped>
.meta-card { border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px; padding: 8px 10px; font-size: 11px; }
.mc-head { display: flex; justify-content: space-between; align-items: center; cursor: pointer; font-weight: 600; color: var(--text-primary); }
.mc-head svg { transition: transform 0.2s; }
.mc-head svg.rotated { transform: rotate(180deg); }
.mc-body { margin-top: 8px; display: flex; flex-direction: column; gap: 4px; }
.mc-row { display: flex; justify-content: space-between; }
.mc-k { color: var(--text-muted); }
.mc-v { color: var(--text-primary); font-variant-numeric: tabular-nums; }
.mc-v.ok { color: #22c55e; }
.mc-v.warn { color: #f59e0b; }
.mc-load { height: 3px; background: var(--surface-hover); border-radius: 2px; margin-top: 4px; }
.mc-load .bar { display: block; height: 100%; background: #3b82f6; border-radius: 2px; transition: width 0.5s; }
.mc-role { display: flex; align-items: center; gap: 4px; margin-top: 4px; }
.mc-ttl { font-size: 10px; color: var(--text-muted); margin-left: auto; }
@media (prefers-color-scheme: dark) { .meta-card { border-color: #334155; } .mc-v.ok { color: #4ade80; } }
</style>
