<template>
  <div class="topo-panel" :class="{ collapsed: !expanded }">
    <div class="tp-head" @click="expanded = !expanded">
      <span class="tp-title">
        任务进度
        <span v-if="topology?.active" class="badge on">进行中</span>
        <span v-else class="badge off">未启用</span>
      </span>
      <svg :class="{ rotated: expanded }" width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4,6 8,10 12,6"/></svg>
    </div>
    <div v-if="expanded && topology" class="tp-body">
      <!-- Warning banner — top priority, always visible first -->
      <div v-if="topology.warning || topology.reject_count >= 2" class="tp-warn" :class="topology.trust_locked ? 'd' : 'w'">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L2 22h20L12 2z"/><line x1="12" y1="10" x2="12" y2="16"/><circle cx="12" cy="19" r="1" fill="currentColor"/></svg>
        <span>{{ topology.warning || `最近 ${topology.reject_count} 次操作失败` }}</span>
        <button v-if="topology.reject_count >= 2" class="tp-btn" @click="overrideNode">放行一次</button>
      </div>

      <!-- Progress + Trust -->
      <div class="tp-progress">
        <span class="tp-label">拓扑完成度</span>
        <span class="tp-val" :class="progressColor">{{ progressPct }}%</span>
        <span class="bar"><i :style="{ width: progressPct + '%', backgroundColor: progressBarColor }" /></span>
      </div>
      <div class="tp-progress secondary">
        <span class="tp-label">操作成功率</span>
        <span class="tp-val small" :class="successRateColor">{{ successRate }}%</span>
        <span class="bar"><i :style="{ width: successRate + '%', backgroundColor: successRateBarColor }" /></span>
      </div>
      <div class="tp-meta">
        <span>偏离 <b :class="deviationLevel">{{ deviationLabel }}</b></span>
        <span>信任 <b :class="trustColor">{{ trustLabel }}</b></span>
        <span v-if="successCount !== null || failCount !== null" class="tp-stats">
          <span class="ok">✓ {{ successCount }}</span>
          <span v-if="failCount" class="ng"> ✗ {{ failCount }}</span>
        </span>
        <button v-if="topology?.trust_locked" class="tp-btn sm" @click="resetTrust" title="手动解锁信任">解锁信任</button>
      </div>

      <!-- Lock competition -->
      <div v-if="lockLeases.length" class="tp-locks">
        <div class="tp-section-title">正在使用的资源</div>
        <div v-for="lk in lockLeases" :key="lk.resource" class="lock-row">
          <span class="lock-res">{{ lk.resource }}</span>
          <span class="lock-type">{{ lk.type }}</span>
          <span class="lock-ttl">剩 {{ lk.ttl }}s</span>
        </div>
      </div>

      <!-- Recent steps table — dual status: topology + tool result -->
      <div v-if="topology.trajectory?.length" class="tp-table">
        <div class="tp-section-title">最近操作</div>
        <div v-for="n in displayNodes.slice(-10).reverse()" :key="n.round" class="tbl-row" :class="n.status">
          <span class="col-step">{{ n.round }}</span>
          <code class="col-tool">{{ n.tool_call || '思考' }}</code>
          <span class="col-result">{{ resultBadge(n.status) }}</span>
          <span class="col-tool-result" :class="toolResultClass(n.tool_result)">{{ toolResultBadge(n.tool_result) }}</span>
        </div>
      </div>

      <!-- Expert mode (collapsed) -->
      <div class="tp-expert">
        <div class="tp-section-title" @click="showExpert = !showExpert">专家模式 {{ showExpert ? '▼' : '▶' }}</div>
        <div v-if="showExpert" class="tp-constraints">
          <label>振幅 A: {{ constraintA.toFixed(1) }}<input type="range" min="0.1" max="1.0" step="0.1" v-model.number="constraintA" @change="updateConstraint" /></label>
          <label>半径 R: {{ constraintR.toFixed(1) }}<input type="range" min="0.5" max="5.0" step="0.1" v-model.number="constraintR" @change="updateConstraint" /></label>
          <label class="chk"><input type="checkbox" v-model="constraintT" @change="updateConstraint" />要求闭环</label>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useChatStore } from '../../stores/chat'
import type { TopologyState } from '../../types/topology'

const chatStore = useChatStore()
const expanded = ref(false)
const showExpert = ref(false)
const constraintA = ref(0.8)
const constraintR = ref(3.0)
const constraintT = ref(false)

const localTopology = ref<TopologyState | null>(null)
const topology = computed(() => (chatStore.topology as any) || localTopology.value)
const lockLeases = ref<any[]>([])

// ── Progress: weighted formula using committed/total nodes, capped at 99% for active sessions ──
const progressPct = computed(() => {
  const nodes = topology.value?.trajectory || []
  const total = nodes.length
  if (total === 0) return 0
  const committed = nodes.filter((n: any) => n.status === 'committed').length
  // Weighted: 70% from committed/total ratio, 30% from X/10
  const xBased = Math.min(((topology.value?.current_coord?.x || 0) / 10) * 100, 100)
  const commitBased = total > 0 ? (committed / total) * 100 : 0
  const weighted = commitBased * 0.7 + xBased * 0.3
  // Cap at 99% for active sessions
  if (topology.value?.active && weighted >= 99) return 99
  return Math.round(weighted)
})
const progressColor = computed(() => topology.value?.reject_count >= 3 ? 'warn' : topology.value?.trust_locked ? 'ng' : 'ok')
const progressBarColor = computed(() => topology.value?.reject_count >= 3 ? '#f59e0b' : topology.value?.trust_locked ? '#ef4444' : '#22c55e')

// ── Success rate: committed / total nodes ──
const successRate = computed(() => {
  const total = topology.value?.total_nodes || (topology.value?.trajectory || []).length
  if (total === 0) return 100
  const committed = topology.value?.committed_count || (topology.value?.trajectory || []).filter((n: any) => n.status === 'committed').length
  return Math.round((committed / total) * 100)
})
const successRateColor = computed(() => successRate.value >= 80 ? 'ok' : successRate.value >= 50 ? 'warn' : 'ng')
const successRateBarColor = computed(() => successRate.value >= 80 ? '#22c55e' : successRate.value >= 50 ? '#f59e0b' : '#ef4444')

const trustLabel = computed(() => {
  const lies = topology.value?.trust_lies || 0
  if (topology.value?.trust_locked) return '已锁定'
  if (lies >= 2) return '需注意'; if (lies >= 1) return '有疑虑'; return '良好'
})
const trustColor = computed(() => topology.value?.trust_locked || (topology.value?.trust_lies || 0) >= 2 ? 'ng' : 'ok')
const deviationLabel = computed(() => { const z = topology.value?.current_coord?.z || 0; const r = topology.value?.constraint?.r || 3; if (z > r * 0.8) return '偏高'; if (z > r * 0.5) return '正常'; return '低' })
const deviationLevel = computed(() => { const z = topology.value?.current_coord?.z || 0; const r = topology.value?.constraint?.r || 3; return z > r * 0.8 ? 'warn' : 'ok' })
const displayNodes = computed(() => (topology.value?.trajectory || []).slice(-30))
const successCount = computed(() => (topology.value?.trajectory || []).filter((n: any) => n.status === 'committed').length)
const failCount = computed(() => (topology.value?.trajectory || []).filter((n: any) => n.status === 'rejected').length)

function resultBadge(s: string): string { switch (s) { case 'committed': return '✓'; case 'rejected': return '✗'; case 'overridden': return '⊡'; default: return s } }
function toolResultBadge(s: string): string { switch (s) { case 'success': return '✓'; case 'error': return '✗'; default: return '—' } }
function toolResultClass(s: string): string { switch (s) { case 'success': return 'ok'; case 'error': return 'ng'; default: return 'none' } }

async function fetchTopo() { const sid = chatStore.sessionId; if (!sid) return; try { const r = await fetch(`/api/chat/sessions/${sid}/topology`, { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } }); const d = await r.json(); if (d.code === 200) localTopology.value = d.data } catch {} }
async function updateConstraint() { const sid = chatStore.sessionId; if (!sid) return; await fetch(`/api/chat/sessions/${sid}/topology/constraint`, { method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${localStorage.getItem('token')}` }, body: JSON.stringify({ a: constraintA.value, r: constraintR.value, t: constraintT.value, force_tools: [] }) }) }
async function overrideNode() { const sid = chatStore.sessionId; if (!sid) return; await fetch(`/api/chat/sessions/${sid}/topology/override`, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${localStorage.getItem('token')}` }, body: JSON.stringify({}) }) }
async function resetTrust() { const sid = chatStore.sessionId; if (!sid) return; await fetch(`/api/chat/sessions/${sid}/topology/trust-reset`, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${localStorage.getItem('token')}` } }) }

watch(() => chatStore.topology, (v: any) => { if (v) { constraintA.value = v.constraint?.a || 0.8; constraintR.value = v.constraint?.r || 3.0; constraintT.value = v.constraint?.t || false } }, { deep: true })
// 切换会话时重新获取拓扑状态
watch(() => chatStore.sessionId, (newSid) => { if (newSid) { fetchTopo() } })
onMounted(() => { fetchTopo() })
</script>

<style scoped>
.topo-panel { border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px; font-size: 11px; overflow: hidden; }
.topo-panel.collapsed .tp-body { display: none; }
.tp-head { padding: 8px 10px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; user-select: none; }
.tp-head svg { transition: transform 0.2s; }
.tp-head svg.rotated { transform: rotate(180deg); }
.tp-title { font-weight: 600; display: flex; align-items: center; gap: 8px; }
.badge { font-size: 10px; padding: 2px 6px; border-radius: 10px; }
.badge.on { background: #22c55e22; color: #22c55e; }
.badge.off { background: #64748b22; color: #64748b; }
.tp-body { padding: 8px 10px; display: flex; flex-direction: column; gap: 8px; }

/* Warning — prominent at top */
.tp-warn { padding: 8px 10px; border-radius: 6px; display: flex; align-items: center; gap: 6px; font-size: 11px; font-weight: 500; }
.tp-warn.w { background: #fef3c7; color: #92400e; border: 1px solid #fcd34d; }
.tp-warn.d { background: #fee2e2; color: #991b1b; border: 1px solid #fca5a5; }
.tp-btn { margin-left: auto; padding: 3px 12px; border-radius: 4px; border: 1px solid; background: #fff; cursor: pointer; font-size: 10px; font-weight: 500; }
	.tp-btn.sm { margin-left: 6px; padding: 2px 8px; font-size: 9px; }

	/* Progress */
.tp-progress { display: flex; align-items: center; gap: 8px; }
.tp-progress.secondary { gap: 8px; }
	.tp-progress.secondary .bar { height: 3px; }
	.tp-progress .bar { flex: 1; height: 4px; background: var(--surface-hover); border-radius: 2px; overflow: hidden; }
.tp-progress .bar i { display: block; height: 100%; border-radius: 2px; transition: width 0.5s, background-color 0.3s; }
.tp-label { color: var(--text-muted); }
.tp-val { font-weight: 700; font-size: 13px; min-width: 36px; font-variant-numeric: tabular-nums; }
.tp-val.small { font-size: 11px; font-weight: 600; }
.tp-val.ok { color: #22c55e; }
.tp-val.warn { color: #f59e0b; }
	.tp-val.ng { color: #ef4444; }

/* Meta row */
.tp-meta { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }
.tp-meta .ok { color: #22c55e; }
.tp-meta .warn { color: #f59e0b; }
.tp-meta .ng { color: #ef4444; font-weight: 600; }
.tp-stats { margin-left: auto; font-variant-numeric: tabular-nums; }
.tp-stats .ok { color: #22c55e; }
.tp-stats .ng { color: #ef4444; }

/* Locks */
.tp-section-title { font-weight: 600; font-size: 10px; color: var(--text-muted); margin-bottom: 4px; cursor: pointer; }
.lock-row { display: flex; gap: 8px; padding: 3px 0; font-size: 10px; font-family: monospace; }
.lock-res { color: var(--text-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; }
.lock-type { color: #f59e0b; }
.lock-ttl { color: var(--text-muted); }

/* Table — redesigned: no "谁" column, full-width tool, row-level tints */
.tp-table { }
.tbl-row { display: grid; grid-template-columns: 24px 1fr 32px 24px; gap: 6px; align-items: center; padding: 4px 6px; border-radius: 4px; margin-bottom: 1px; font-size: 11px; }
.tbl-row.committed { background: rgba(34,197,94,0.06); }
.tbl-row.rejected { background: rgba(239,68,68,0.08); }
.tbl-row.overridden { background: rgba(245,158,11,0.08); }
.col-step { color: var(--text-muted); text-align: right; font-variant-numeric: tabular-nums; }
.col-tool { font-family: monospace; font-size: 10.5px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.col-result { text-align: center; font-weight: 700; font-size: 12px; }
.col-tool-result { text-align: center; font-weight: 700; font-size: 10px; }
.col-tool-result.ok { color: #22c55e; }
.col-tool-result.ng { color: #ef4444; }
	.col-tool-result.none { color: var(--text-muted); }
	.tbl-row.committed .col-result { color: #22c55e; }
	.tbl-row.rejected .col-result { color: #ef4444; }
	.tbl-row.overridden .col-result { color: #f59e0b; }

/* Expert */
.tp-expert { border-top: 1px solid var(--border-subtle); padding-top: 6px; }
.tp-constraints { display: flex; flex-direction: column; gap: 4px; }
.tp-constraints label { display: flex; align-items: center; gap: 6px; font-size: 10px; }
.tp-constraints input[type="range"] { flex: 1; accent-color: #6366f1; }

@media (prefers-color-scheme: dark) {
  .topo-panel { border-color: #334155; }
  .tbl-row.committed { background: rgba(34,197,94,0.08); }
  .tbl-row.rejected { background: rgba(239,68,68,0.10); }
  .tbl-row.overridden { background: rgba(245,158,11,0.10); }
  .tp-warn.w { background: #422006; color: #fbbf24; border-color: #78350f; }
  .tp-warn.d { background: #450a0a; color: #fca5a5; border-color: #7f1d1d; }
  .tp-btn { background: transparent; }
}
</style>
