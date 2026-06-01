<template>
  <div class="topology-panel" :class="{ collapsed: !expanded }">
    <div class="panel-header" @click="expanded = !expanded">
      <span class="panel-title">
        📐 拓扑约束
        <span v-if="topology?.active" class="badge active">活跃</span>
        <span v-else class="badge inactive">未激活</span>
      </span>
      <span class="collapse-icon">{{ expanded ? '▼' : '▶' }}</span>
    </div>

    <div v-if="expanded" class="panel-body">
      <!-- Warning banner -->
      <div v-if="topology?.warning" class="warning-banner" :class="warningLevel">
        ⚠️ {{ topology?.warning }}
        <button v-if="topology && topology.reject_count >= 5" class="rescue-btn" @click="overrideNode">
          放行一次
        </button>
      </div>

      <!-- Trust status -->
      <div v-if="topology" class="trust-status">
        <span :class="trustClass">
          {{ topology.trust_locked ? '🔒 信任已锁定' : `🔓 信任度: ${3 - topology.trust_lies}/3` }}
        </span>
        <span v-if="topology.closed_loop" class="closed-badge" :class="{ closed: topology.closed_loop }">
          {{ topology.closed_loop ? '✅ 闭环' : '❌ 未闭合' }}
        </span>
      </div>

      <!-- Charts row -->
      <div v-if="topology?.active" class="charts-row">
        <div class="chart-container">
          <canvas ref="xyCanvas"></canvas>
          <div class="chart-label">X-Y 振幅 (A={{ topology?.constraint?.a || 0.8 }})</div>
        </div>
        <div class="chart-container">
          <canvas ref="xzCanvas"></canvas>
          <div class="chart-label">X-Z 偏离 (R={{ topology?.constraint?.r || 3.0 }})</div>
        </div>
      </div>

      <!-- Current coordinates -->
      <div v-if="topology?.active" class="coord-display">
        <span>X: {{ topology?.current_coord?.x?.toFixed(1) || '0.0' }}</span>
        <span>Y: {{ topology?.current_coord?.y?.toFixed(2) || '0.00' }}</span>
        <span>Z: {{ topology?.current_coord?.z?.toFixed(2) || '0.00' }}</span>
      </div>

      <!-- Constraint sliders -->
      <div v-if="topology?.active" class="constraint-sliders">
        <label>
          振幅上限 A: {{ constraintA.toFixed(1) }}
          <input type="range" min="0.1" max="1.0" step="0.1" v-model.number="constraintA" @change="updateConstraint" />
        </label>
        <label>
          半径上限 R: {{ constraintR.toFixed(1) }}
          <input type="range" min="0.5" max="5.0" step="0.1" v-model.number="constraintR" @change="updateConstraint" />
        </label>
        <label class="checkbox-label">
          <input type="checkbox" v-model="constraintT" @change="updateConstraint" />
          闭环要求 (T)
        </label>
      </div>

      <!-- Closed loop indicator -->
      <div v-if="topology?.closed_loop !== undefined" class="loop-indicator" :class="{ closed: topology.closed_loop, open: !topology.closed_loop }">
        <svg width="80" height="40" viewBox="0 0 80 40">
          <path d="M10,35 Q40,-5 70,35" :stroke="topology.closed_loop ? '#22c55e' : '#ef4444'" stroke-width="2" fill="none" />
          <circle cx="10" cy="35" r="4" fill="#6366f1" />
          <circle cx="70" cy="35" r="4" :fill="topology.closed_loop ? '#22c55e' : '#ef4444'" />
          <text x="35" y="15" text-anchor="middle" :fill="topology.closed_loop ? '#22c55e' : '#ef4444'" font-size="10">
            {{ topology.closed_loop ? '✓ 闭合' : `✗ ${topology.closed_distance?.toFixed(1) || '?'}` }}
          </text>
        </svg>
      </div>

      <!-- Trajectory table -->
      <div v-if="topology?.trajectory?.length" class="trajectory-table">
        <div class="table-header">
          <span>轮</span><span>X</span><span>Y</span><span>Z</span><span>工具</span><span>状态</span>
        </div>
        <div v-for="node in topology.trajectory.slice(-20)" :key="node.round" class="table-row" :class="node.status">
          <span>{{ node.round }}</span>
          <span>{{ node.x.toFixed(1) }}</span>
          <span>{{ node.y.toFixed(2) }}</span>
          <span>{{ node.z.toFixed(2) }}</span>
          <span class="tool-name">{{ node.tool_call || '-' }}</span>
          <span>{{ statusIcon(node.status) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { useChatStore } from '../../stores/chat'
import type { TopologyState } from '../../types/topology'

const chatStore = useChatStore()
const expanded = ref(false)
const xyCanvas = ref<HTMLCanvasElement | null>(null)
const xzCanvas = ref<HTMLCanvasElement | null>(null)

const constraintA = ref(0.8)
const constraintR = ref(3.0)
const constraintT = ref(false)

let xyChart: any = null
let xzChart: any = null

// 本地 topology 状态：优先用 store，fallback 到直接 HTTP fetch
const localTopology = ref<TopologyState | null>(null)
const topology = computed<TopologyState | null>(() => (chatStore.topology as any) || localTopology.value)

async function fetchTopologyState() {
  const sessionId = chatStore.sessionId
  if (!sessionId) return
  const token = localStorage.getItem('token')
  try {
    const res = await fetch(`/api/chat/sessions/${sessionId}/topology`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    const data = await res.json()
    if (data.code === 200 && data.data) {
      localTopology.value = data.data as TopologyState
    }
  } catch (e) { /* ignore */ }
}

onMounted(() => {
  fetchTopologyState()
})

const warningLevel = computed(() => {
  if (!topology.value?.warning) return ''
  if (topology.value.warning.includes('锁定')) return 'danger'
  if (topology.value.warning.includes('拒绝')) return 'warning'
  return 'info'
})

const trustClass = computed(() => {
  if (!topology.value) return ''
  if (topology.value.trust_locked) return 'trust-locked'
  if (topology.value.trust_lies >= 2) return 'trust-low'
  if (topology.value.trust_lies >= 1) return 'trust-warn'
  return 'trust-ok'
})

function statusIcon(status: string): string {
  switch (status) {
    case 'committed': return '✅'
    case 'rejected': return '❌'
    case 'pending': return '⏳'
    case 'overridden': return '🔧'
    default: return status
  }
}

function drawCharts() {
  if (!topology.value?.trajectory?.length) return

  const traj = topology.value.trajectory
  const a = topology.value.constraint?.a || 0.8
  const r = topology.value.constraint?.r || 3.0

  // X-Y chart
  if (xyCanvas.value) {
    const ctx = xyCanvas.value.getContext('2d')
    if (ctx) {
      const W = xyCanvas.value.width = xyCanvas.value.clientWidth * 2 || 400
      const H = xyCanvas.value.height = 160
      ctx.scale(2, 2)
      ctx.clearRect(0, 0, W, H)

      const pad = { l: 40, r: 10, t: 10, b: 20 }
      const pw = W / 2 - pad.l - pad.r
      const ph = H - pad.t - pad.b

      // Draw A band
      ctx.fillStyle = 'rgba(99, 102, 241, 0.1)'
      ctx.fillRect(pad.l, pad.t, pw, ph / 2 - (a / 2) * ph)

      // Draw trajectory
      ctx.strokeStyle = '#6366f1'
      ctx.lineWidth = 2
      ctx.beginPath()
      for (let i = 0; i < traj.length; i++) {
        const x = pad.l + (traj[i].x / 10) * pw
        const y = pad.t + ((1 - (traj[i].y + 1) / 2)) * ph
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.stroke()

      // Rejected nodes in red
      for (const n of traj) {
        if (n.status === 'rejected') {
          const x = pad.l + (n.x / 10) * pw
          const y = pad.t + ((1 - (n.y + 1) / 2)) * ph
          ctx.fillStyle = '#ef4444'
          ctx.beginPath()
          ctx.arc(x, y, 3, 0, Math.PI * 2)
          ctx.fill()
        }
      }
    }
  }

  // X-Z chart
  if (xzCanvas.value) {
    const ctx = xzCanvas.value.getContext('2d')
    if (ctx) {
      const W = xzCanvas.value.width = xzCanvas.value.clientWidth * 2 || 400
      const H = xzCanvas.value.height = 160
      ctx.scale(2, 2)
      ctx.clearRect(0, 0, W, H)

      const pad = { l: 40, r: 10, t: 10, b: 20 }
      const pw = W / 2 - pad.l - pad.r
      const ph = H - pad.t - pad.b

      // Draw R red line
      const rY = pad.t + (1 - r / 5) * ph
      ctx.strokeStyle = 'rgba(239, 68, 68, 0.5)'
      ctx.setLineDash([4, 4])
      ctx.beginPath()
      ctx.moveTo(pad.l, rY)
      ctx.lineTo(pad.l + pw, rY)
      ctx.stroke()
      ctx.setLineDash([])

      // Draw trajectory
      ctx.strokeStyle = '#22c55e'
      ctx.lineWidth = 2
      ctx.beginPath()
      for (let i = 0; i < traj.length; i++) {
        const x = pad.l + (traj[i].x / 10) * pw
        const y = pad.t + ((1 - traj[i].z / 5)) * ph
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.stroke()
    }
  }
}

async function updateConstraint() {
  const sessionId = chatStore.sessionId
  if (!sessionId) return
  const token = localStorage.getItem('token')
  try {
    await fetch(`/api/chat/sessions/${sessionId}/topology/constraint`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ a: constraintA.value, r: constraintR.value, t: constraintT.value, force_tools: [] }),
    })
  } catch (e) {
    console.error('Failed to update constraint:', e)
  }
}

async function overrideNode() {
  const sessionId = chatStore.sessionId
  if (!sessionId) return
  const token = localStorage.getItem('token')
  try {
    await fetch(`/api/chat/sessions/${sessionId}/topology/override`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({}),
    })
  } catch (e) {
    console.error('Failed to override:', e)
  }
}

watch(() => chatStore.topology, (newVal) => {
  if (newVal) {
    constraintA.value = (newVal as any)?.constraint?.a || 0.8
    constraintR.value = (newVal as any)?.constraint?.r || 3.0
    constraintT.value = (newVal as any)?.constraint?.t || false
    if (expanded.value) {
      requestAnimationFrame(() => drawCharts())
    }
  }
}, { deep: true })

watch(expanded, (val) => {
  if (val) {
    requestAnimationFrame(() => drawCharts())
  }
})

onMounted(() => {
  if (expanded.value) drawCharts()
})

onUnmounted(() => {
  xyChart = null
  xzChart = null
})
</script>

<style scoped>
.topology-panel {
  background: var(--color-bg-2);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  margin: 8px 0;
  font-size: 12px;
  overflow: hidden;
}

.topology-panel.collapsed .panel-body {
  display: none;
}

.panel-header {
  padding: 8px 12px;
  cursor: pointer;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--color-bg-3);
  user-select: none;
}

.panel-title {
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 10px;
}

.badge.active { background: #22c55e22; color: #22c55e; }
.badge.inactive { background: #64748b22; color: #64748b; }

.panel-body {
  padding: 12px;
}

.warning-banner {
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.warning-banner.warning { background: #f59e0b22; color: #f59e0b; }
.warning-banner.danger { background: #ef444422; color: #ef4444; }
.warning-banner.info { background: #3b82f622; color: #3b82f6; }

.rescue-btn {
  margin-left: auto;
  padding: 4px 12px;
  border-radius: 4px;
  border: 1px solid currentColor;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 11px;
}

.charts-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.chart-container {
  flex: 1;
  min-width: 0;
}

.chart-container canvas {
  width: 100%;
  height: 80px;
  border: 1px solid var(--color-border);
  border-radius: 4px;
  background: var(--color-bg-1);
}

.chart-label {
  text-align: center;
  font-size: 10px;
  color: var(--color-text-3);
  margin-top: 2px;
}

.coord-display {
  display: flex;
  gap: 16px;
  padding: 6px 0;
  font-family: monospace;
  font-size: 11px;
  color: var(--color-text-2);
}

.trust-status {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
  margin-bottom: 8px;
}

.trust-ok { color: #22c55e; }
.trust-warn { color: #f59e0b; }
.trust-low { color: #ef4444; }
.trust-locked { color: #ef4444; font-weight: bold; }

.closed-badge { font-size: 11px; padding: 2px 8px; border-radius: 10px; }
.closed-badge.closed { background: #22c55e22; color: #22c55e; }

.constraint-sliders {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0;
}

.constraint-sliders label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--color-text-2);
}

.constraint-sliders input[type="range"] {
  flex: 1;
  accent-color: #6366f1;
}

.checkbox-label {
  cursor: pointer;
}

.loop-indicator {
  text-align: center;
  margin: 8px 0;
}

.trajectory-table {
  margin-top: 8px;
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: 4px;
}

.table-header, .table-row {
  display: grid;
  grid-template-columns: 30px 40px 45px 45px 1fr 30px;
  gap: 4px;
  padding: 4px 8px;
  font-size: 11px;
}

.table-header {
  font-weight: 600;
  background: var(--color-bg-3);
  position: sticky;
  top: 0;
}

.table-row.rejected { background: #ef444411; }
.table-row.overridden { background: #f59e0b11; }

.tool-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.collapse-icon { font-size: 10px; color: var(--color-text-3); }
</style>
