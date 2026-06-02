<template>
  <div class="dashboard">
    <!-- Stats strip -->
    <div
      v-if="status"
      class="stat-strip"
    >
      <div class="stat-block">
        <span class="stat-val">{{ status.scheduler?.total ?? 0 }}</span><span class="stat-key">域名记录</span>
      </div>
      <div class="stat-block">
        <span class="stat-val">{{ notifyCount }}</span><span class="stat-key">通知渠道</span>
        <span class="notify-dots">
          <span
            :class="['nd', { on: notifyStatus.email_enabled }]"
            title="邮件"
          >✉</span>
          <span
            :class="['nd', { on: notifyStatus.webhook_enabled }]"
            title="Webhook"
          >⤓</span>
          <span
            :class="['nd', { on: notifyStatus.dingtalk_enabled }]"
            title="钉钉"
          >⚡</span>
        </span>
      </div>
      <div class="stat-block">
        <span class="stat-val mono">{{ status.uptime || '-' }}</span><span class="stat-key">运行时间</span>
      </div>
      <div class="stat-block">
        <span class="stat-val mono">{{ status.version || '-' }}</span><span class="stat-key">版本</span>
      </div>
      <div class="stat-block">
        <span
          class="stat-dot"
          :class="{ running: status.scheduler?.running }"
        ></span><span class="stat-key">{{ status.scheduler?.running ? '调度器运行中' : '已停止' }}</span>
      </div>
    </div>

    <!-- AI Usage -->
    <div
      v-if="aiStats"
      class="ai-strip"
    >
      <div class="ai-block">
        <span class="ai-val">{{ fmtNum(aiStats.requests) }}</span><span class="ai-key">AI 请求</span>
      </div>
      <div class="ai-block">
        <span class="ai-val">{{ fmtNum(aiStats.input_tokens + aiStats.output_tokens) }}</span><span class="ai-key">Token 用量</span>
      </div>
      <div class="ai-block">
        <span class="ai-val">¥{{ (aiStats.cost_usd || 0).toFixed(6) }}</span><span class="ai-key">成本</span>
      </div>
      <div class="ai-block">
        <span class="ai-val">{{ aiStats.requests > 0 ? Math.round((1 - aiStats.errors / aiStats.requests) * 100) + '%' : '--' }}</span><span class="ai-key">成功率</span>
      </div>
      <div
        v-if="aiStats.models?.length"
        class="ai-block"
      >
        <span class="ai-key">可用模型</span>
        <span class="ai-models">{{ aiStats.models.join(', ') }}</span>
      </div>
      <div
        v-if="aiStats?.started_at"
        class="ai-since"
      >
        自 {{ fmtSince(aiStats.started_at) }} 起统计
      </div>
    </div>

    <!-- Agent system v2.0 -->
    <div v-if="agentMetrics" class="agent-strip">
      <div class="agent-block"><span class="agent-val ok">{{ agentMetrics.success_rate }}</span><span class="agent-key">助手成功率</span></div>
      <div class="agent-block"><span class="agent-val">{{ agentMetrics.spawned }}</span><span class="agent-key">助手任务</span></div>
      <div class="agent-block"><span class="agent-val warn">{{ agentMetrics.conflicts }}</span><span class="agent-key">资源冲突</span></div>
      <div class="agent-block"><span class="agent-val">{{ agentMetrics.promotions }}</span><span class="agent-key">角色升级</span></div>
      <div class="agent-block"><span class="agent-val">{{ agentMetrics.demotions }}</span><span class="agent-key">角色降级</span></div>
    </div>

    <!-- Gauges -->
    <div
      v-if="status?.system"
      class="gauges-row"
    >
      <div class="gauge-card">
        <div class="gauge-ring">
          <Doughnut
            :data="cpuChartData"
            :options="doughnutOpts"
          /><span class="gauge-val">{{ Math.round(status.system.cpu_usage) }}%</span>
        </div>
        <span class="gauge-label">CPU · {{ status.system.cpu_cores ?? 0 }}核</span>
      </div>
      <div class="gauge-card">
        <div class="gauge-ring">
          <Doughnut
            :data="memChartData"
            :options="doughnutOpts"
          /><span class="gauge-val">{{ Math.round(status.system.mem_usage) }}%</span>
        </div>
        <span class="gauge-label">内存<button
          class="mem-btn"
          :disabled="clearingMem"
          @click="clearMemory"
        >{{ clearingMem ? '释放中' : '释放' }}</button></span>
      </div>
      <div
        v-if="diskInfo"
        class="gauge-card"
      >
        <div class="gauge-ring">
          <Doughnut
            :data="diskChartData"
            :options="doughnutOpts"
          /><span class="gauge-val">{{ Math.round(diskInfo.used_pct) }}%</span>
        </div>
        <span class="gauge-label">磁盘 · {{ fmtBytes(diskInfo.used) }}/{{ fmtBytes(diskInfo.total) }}</span>
      </div>
      <div class="gauge-card net-gauge">
        <div class="net-rates">
          <div class="net-rate down">
            <span class="net-arrow">↓</span><span class="net-val">{{ fmtRate(netRate.rx) }}</span>
          </div>
          <div class="net-rate up">
            <span class="net-arrow">↑</span><span class="net-val">{{ fmtRate(netRate.tx) }}</span>
          </div>
        </div>
        <span class="gauge-label">网络 · 实时速率</span>
      </div>
    </div>

    <!-- MCP + Go Runtime + Process row -->
    <div
      v-if="status"
      class="info-row"
    >
      <div class="info-card">
        <h4>MCP 服务器</h4>
        <div class="info-body">
          <div class="mcp-summary">
            <span class="mcp-big">{{ mcpStats?.connected || 0 }}/{{ mcpStats?.total || 0 }}</span> 已连接 · {{ mcpStats?.tools || 0 }} 工具
          </div>
          <div
            v-if="mcpStats?.servers?.length"
            class="mcp-list"
          >
            <div
              v-for="s in mcpStats.servers"
              :key="s.name"
              class="mcp-item"
            >
              <span :class="['mcp-dot', { on: s.connected }]"></span>
              <span class="mcp-name">{{ s.name }}</span>
              <span class="mcp-tools">{{ s.tools }} tools</span>
            </div>
          </div>
          <div
            v-else
            class="info-empty"
          >
            暂无 MCP 服务器
          </div>
        </div>
      </div>

      <div class="info-card">
        <h4>Go Runtime</h4>
        <div
          v-if="goRuntime"
          class="info-body"
        >
          <div class="info-line">
            <span>协程</span><b>{{ goRuntime.goroutines }}</b>
          </div>
          <div class="info-line">
            <span>堆内存</span><b>{{ goRuntime.heap_alloc_mb }} MB</b>
          </div>
          <div class="info-line">
            <span>GC 次数</span><b>{{ goRuntime.num_gc }}</b>
          </div>
          <div class="info-line">
            <span>最近 GC 暂停</span><b>{{ goRuntime.gc_pause_us }} µs</b>
          </div>
          <div class="info-line">
            <span>Go 版本</span><b class="mono">{{ goRuntime.go_version }}</b>
          </div>
        </div>
      </div>

      <div class="info-card">
        <h4>进程</h4>
        <div
          v-if="processStats"
          class="info-body"
        >
          <div class="info-line">
            <span>RSS</span><b>{{ processStats.rss_kb ? Math.round(processStats.rss_kb / 1024) : '-' }} MB</b>
          </div>
          <div class="info-line">
            <span>线程</span><b>{{ processStats.threads || '-' }}</b>
          </div>
        </div>
      </div>

      <div class="info-card">
        <h4>系统负载</h4>
        <div
          v-if="status?.system?.load_avg"
          class="info-body"
        >
          <div class="load-bars">
            <div
              v-for="(v, key) in parseLoad(status.system.load_avg)"
              :key="key"
              class="load-bar-wrap"
            >
              <span class="load-bar-label">{{ key }}</span>
              <div class="load-bar-track">
                <div
                  class="load-bar-fill"
                  :style="{ width: Math.min(v / cpuCount * 100, 100) + '%' }"
                ></div>
              </div>
              <span class="load-bar-val">{{ v.toFixed(2) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Top 5 Tools -->
    <div
      v-if="aiStats?.top_tools?.length"
      class="chart-card"
    >
      <h4>Top 工具调用</h4>
      <div class="top-tools">
        <div
          v-for="t in aiStats.top_tools"
          :key="t.name"
          class="top-tool-item"
        >
          <span
            class="tt-name"
            :title="t.name"
          >{{ t.name }}</span>
          <span class="tt-count">{{ t.count }}次</span>
          <span class="tt-lat">{{ t.avg_ms?.toFixed(0) || 0 }}ms</span>
          <div class="tt-bar">
            <div
              class="tt-bar-fill"
              :style="{ width: (t.count / aiStats.top_tools[0].count * 100) + '%' }"
            ></div>
          </div>
        </div>
      </div>
    </div>

    <!-- Interfaces -->
    <div
      v-if="ifaceGroups.length"
      class="iface-row"
    >
      <div
        v-for="g in ifaceGroups"
        :key="g.name"
        class="iface-card"
      >
        <span class="iface-name">{{ g.name }}</span>
        <span class="iface-addrs">{{ g.addrs.join(', ') }}</span>
        <span class="iface-rx">↓{{ fmtBytes(g.rx) }}</span>
        <span class="iface-tx">↑{{ fmtBytes(g.tx) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Doughnut, Bar } from 'vue-chartjs'
import { Chart as ChartJS, ArcElement, BarElement, CategoryScale, LinearScale, Filler, Tooltip, Legend } from 'chart.js'
import api from '../services/api'
import { useToast } from '../composables/useToast'

ChartJS.register(ArcElement, BarElement, CategoryScale, LinearScale, Filler, Tooltip, Legend)

const toast = useToast()

const status = ref(null)
const diskInfo = ref(null)
const netRate = ref({ rx: 0, tx: 0 })
const clearingMem = ref(false)
let lastNetBytes = { rx: 0, tx: 0, ts: 0 }
let timers = []
let isActive = true

const doughnutOpts = { cutout: '78%', responsive: true, maintainAspectRatio: true, plugins: { legend: { display: false }, tooltip: { enabled: false } } }

const cpuChartData = computed(() => ({ datasets: [{ data: [status.value?.system?.cpu_usage || 0, 100 - (status.value?.system?.cpu_usage || 0)], backgroundColor: ['#06b6d4', 'rgba(0,0,0,0.05)'], borderWidth: 0 }] }))
const memChartData = computed(() => ({ datasets: [{ data: [status.value?.system?.mem_usage || 0, 100 - (status.value?.system?.mem_usage || 0)], backgroundColor: ['#8b5cf6', 'rgba(0,0,0,0.05)'], borderWidth: 0 }] }))
const diskChartData = computed(() => ({ datasets: [{ data: [diskInfo.value?.used_pct || 0, 100 - (diskInfo.value?.used_pct || 0)], backgroundColor: ['#f59e0b', 'rgba(0,0,0,0.05)'], borderWidth: 0 }] }))

const aiStats = computed(() => status.value?.ai || null)
const mcpStats = computed(() => status.value?.mcp || null)
const goRuntime = computed(() => status.value?.go_runtime || null)
const processStats = computed(() => status.value?.process || null)
const notifyStatus = computed(() => status.value?.notify || { email_enabled: false, webhook_enabled: false, dingtalk_enabled: false })

const agentMetrics = computed(() => {
  const ai = status.value?.ai || {}
  const spawned = ai.sub_agent_spawned || 0
  const success = ai.sub_agent_success || 0
  const failed = ai.sub_agent_failed || 0
  const total = spawned || 1
  return {
    success_rate: Math.round((success / total) * 100) + '%',
    spawned,
    conflicts: ai.lock_conflicts || 0,
    promotions: ai.role_promotions || 0,
    demotions: ai.role_demotions || 0,
  }
})
const notifyCount = computed(() => [notifyStatus.value.email_enabled, notifyStatus.value.webhook_enabled, notifyStatus.value.dingtalk_enabled].filter(Boolean).length)
const cpuCount = computed(() => status.value?.system?.cpu_cores || 1)

const ifaceGroups = computed(() => {
  const raw = status.value?.system?.interfaces || []
  const map = {}
  for (const i of raw) {
    if (!map[i.name]) map[i.name] = { name: i.name, addrs: [], rx: 0, tx: 0 }
    if (i.addr && !map[i.name].addrs.includes(i.addr)) map[i.name].addrs.push(i.addr)
    map[i.name].rx += i.rx_bytes || 0
    map[i.name].tx += i.tx_bytes || 0
  }
  return Object.values(map)
})

function parseLoad(la) {
  if (typeof la === 'string') {
    const parts = la.trim().split(/\s+/)
    return { '1min': parseFloat(parts[0]) || 0, '5min': parseFloat(parts[1]) || 0, '15min': parseFloat(parts[2]) || 0 }
  }
  return { '1min': la?.load1 || 0, '5min': la?.load5 || 0, '15min': la?.load15 || 0 }
}

function updateNetRate() {
  const sys = status.value?.system; if (!sys) return
  let rx = 0, tx = 0
  for (const i of (sys.interfaces || [])) { rx += i.rx_bytes || 0; tx += i.tx_bytes || 0 }
  if (lastNetBytes.ts > 0) {
    const dt = (Date.now() - lastNetBytes.ts) / 1000
    if (dt > 0.5) {
      netRate.value = { rx: Math.max(0, (rx - lastNetBytes.rx) / dt), tx: Math.max(0, (tx - lastNetBytes.tx) / dt) }
    }
  }
  lastNetBytes = { rx, tx, ts: Date.now() }
}

function fmtBytes(n) { if (!n) return '0B'; const u = ['B', 'KB', 'MB', 'GB', 'TB']; const i = Math.floor(Math.log(n) / Math.log(1024)); return (n / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + u[i] }
function fmtRate(bps) { if (!bps || bps < 0) return '0 B/s'; const u = ['B/s', 'KB/s', 'MB/s', 'GB/s']; const i = Math.floor(Math.log(bps) / Math.log(1024)); return (bps / Math.pow(1024, i)).toFixed(1) + ' ' + u[i] }
function fmtNum(n) { if (!n) return '0'; return n >= 1000 ? (n / 1000).toFixed(1) + 'K' : String(n) }
function fmtSince(t) { if (!t) return ''; const d = new Date(t); return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) }

async function loadFast() {
  if (!isActive) return
  try { const r = await api.get('/api/status'); status.value = r.data.data; updateNetRate() } catch (e) { /* ignore */ }
}
async function loadSlow() {
  if (!isActive) return
  try { const r = await api.get('/api/nas/diskinfo', { params: { path: '/' } }); diskInfo.value = r.data.data } catch (e) { /* ignore */ }
}
async function clearMemory() {
  clearingMem.value = true
  try { await api.post('/api/system/gc'); toast.success('内存已释放'); setTimeout(loadFast, 800) }
  catch (e) { toast.error('清理失败') } finally { clearingMem.value = false }
}

onMounted(() => {
  loadFast().then(loadSlow)
  // Layered polling
  timers.push(setInterval(loadFast, 2000))   // system/ai/mcp/go — 2s
  timers.push(setInterval(loadSlow, 30000))  // disk — 30s
})
onUnmounted(() => { isActive = false; timers.forEach(clearInterval) })
</script>

<style scoped>
.dashboard { display: flex; flex-direction: column; gap: 14px; }

/* Stat strip */
.stat-strip { display: grid; grid-template-columns: repeat(5, 1fr); gap: 10px; }
.stat-block { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 12px 8px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 10px; }
.stat-val { font-size: 20px; font-weight: 700; color: var(--text-primary); }
.stat-val.mono { font-family: var(--font-mono); font-size: 13px; }
.stat-key { font-size: 10px; color: var(--text-muted); text-align: center; }
.stat-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--border-default); margin-top: 4px; }
.stat-dot.running { background: #22c55e; box-shadow: 0 0 8px rgba(34,197,94,0.4); }
.notify-dots { display: flex; gap: 6px; margin-top: 2px; }
.nd { font-size: 12px; color: var(--text-muted); opacity: 0.3; transition: opacity 0.2s; }
.nd.on { opacity: 1; color: #22c55e; }

/* AI strip */
.ai-strip { display: grid; grid-template-columns: repeat(5, 1fr); gap: 10px; }
.agent-strip { display: grid; grid-template-columns: repeat(5, 1fr); gap: 10px; margin-top: 10px; }
.agent-block { display: flex; flex-direction: column; align-items: center; gap: 4px; padding: 10px 8px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 10px; }
.agent-val { font-size: 20px; font-weight: 700; font-variant-numeric: tabular-nums; color: var(--text-primary); }
.agent-val.ok { color: #22c55e; }
.agent-val.warn { color: #f59e0b; }
.agent-key { font-size: 11px; color: var(--text-muted); }
.ai-block { display: flex; flex-direction: column; align-items: center; gap: 3px; padding: 10px 8px; background: linear-gradient(135deg, rgba(6,182,212,0.04), rgba(8,145,178,0.02)); border: 1px solid rgba(6,182,212,0.15); border-radius: 10px; }
.ai-val { font-size: 18px; font-weight: 700; color: var(--brand-600); }
.ai-key { font-size: 10px; color: var(--text-muted); }
.ai-models { font-size: 10px; color: var(--text-secondary); font-family: var(--font-mono); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }

/* Gauges */
.gauges-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }
.gauge-card { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 14px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 10px; }
.gauge-ring { position: relative; width: 80px; height: 80px; display: flex; align-items: center; justify-content: center; }
.gauge-ring canvas { position: absolute; }
.gauge-val { position: absolute; font-size: 16px; font-weight: 700; color: var(--text-primary); }
.gauge-label { font-size: 11px; color: var(--text-muted); text-align: center; display: flex; align-items: center; gap: 4px; }
.mem-btn { padding: 1px 6px; border-radius: 4px; border: 1px solid var(--border-default); background: transparent; color: var(--brand-500); font-size: 9px; cursor: pointer; }
.mem-btn:hover { background: var(--brand-50); }
.net-rates { display: flex; gap: 12px; padding: 8px 0; }
.net-rate { display: flex; align-items: center; gap: 3px; font-size: 14px; font-weight: 600; }
.net-arrow { font-size: 12px; }
.net-rate.down .net-arrow, .net-rate.down .net-val { color: #06b6d4; }
.net-rate.up .net-arrow, .net-rate.up .net-val { color: #8b5cf6; }

/* Info cards row */
.info-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }
.info-card { padding: 12px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 10px; }
.info-card h4 { margin: 0 0 8px; font-size: 12px; font-weight: 600; color: var(--text-secondary); }
.info-body { display: flex; flex-direction: column; gap: 6px; }
.info-line { display: flex; justify-content: space-between; font-size: 12px; color: var(--text-muted); }
.info-line b { color: var(--text-primary); font-weight: 600; }
.info-line b.mono { font-family: var(--font-mono); font-size: 11px; }
.info-empty { font-size: 11px; color: var(--text-muted); text-align: center; padding: 8px; }

.mcp-summary { font-size: 12px; color: var(--text-secondary); margin-bottom: 4px; }
.mcp-big { font-size: 16px; font-weight: 700; color: var(--brand-600); }
.mcp-list { display: flex; flex-direction: column; gap: 3px; }
.mcp-item { display: flex; align-items: center; gap: 6px; font-size: 11px; }
.mcp-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--border-default); flex-shrink: 0; }
.mcp-dot.on { background: #22c55e; }
.mcp-name { color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mcp-tools { color: var(--text-muted); margin-left: auto; font-size: 10px; }

.load-bars { display: flex; flex-direction: column; gap: 6px; }
.load-bar-wrap { display: flex; align-items: center; gap: 6px; font-size: 11px; }
.load-bar-label { color: var(--text-muted); min-width: 32px; }
.load-bar-track { flex: 1; height: 4px; background: var(--surface-hover); border-radius: 2px; overflow: hidden; }
.load-bar-fill { height: 100%; background: var(--brand-500); border-radius: 2px; transition: width 0.5s; }
.load-bar-val { color: var(--text-secondary); font-family: var(--font-mono); min-width: 36px; text-align: right; }

/* Top tools */
.chart-card { padding: 14px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 10px; }
.chart-card h4 { margin: 0 0 10px; font-size: 13px; font-weight: 600; color: var(--text-secondary); }
.top-tools { display: flex; flex-direction: column; gap: 8px; }
.top-tool-item { display: grid; grid-template-columns: 1fr 50px 50px; gap: 8px; align-items: center; font-size: 12px; }
.tt-name { color: var(--text-primary); font-family: var(--font-mono); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tt-count { color: var(--brand-500); font-weight: 600; }
.tt-lat { color: var(--text-muted); font-size: 10px; text-align: right; }
.tt-bar { grid-column: 1 / -1; height: 3px; background: var(--surface-hover); border-radius: 2px; overflow: hidden; }
.tt-bar-fill { height: 100%; background: var(--brand-400); border-radius: 2px; }

/* Interfaces */
.iface-row { display: flex; flex-wrap: wrap; gap: 6px; }
.iface-card { display: flex; align-items: center; gap: 10px; padding: 7px 12px; background: var(--surface-card); border: 1px solid var(--border-default); border-radius: 8px; font-size: 11px; }
.iface-name { font-weight: 600; color: var(--text-primary); min-width: 52px; }
.iface-addrs { color: var(--text-muted); font-family: var(--font-mono); font-size: 9px; max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.iface-rx { color: #06b6d4; margin-left: auto; white-space: nowrap; }
.iface-tx { color: #8b5cf6; white-space: nowrap; }

[data-theme="dark"] .stat-block, [data-theme="dark"] .gauge-card, [data-theme="dark"] .chart-card, [data-theme="dark"] .iface-card, [data-theme="dark"] .info-card { background: rgba(255,255,255,0.03); border-color: rgba(255,255,255,0.06); }
[data-theme="dark"] .ai-block { background: rgba(6,182,212,0.06); border-color: rgba(34,211,238,0.12); }
[data-theme="dark"] .mem-btn:hover { background: rgba(6,182,212,0.12); }

@media (max-width: 1023px) {
  .gauges-row, .info-row { grid-template-columns: repeat(2, 1fr); }
  .stat-strip { grid-template-columns: repeat(3, 1fr); }
  .ai-strip { grid-template-columns: repeat(3, 1fr); }
}
@media (max-width: 767px) {
  .stat-strip { grid-template-columns: repeat(2, 1fr); }
  .gauges-row, .info-row { grid-template-columns: 1fr 1fr; }
  .ai-strip { grid-template-columns: repeat(2, 1fr); }
  .gauge-ring { width: 64px; height: 64px; }
  .gauge-val { font-size: 14px; }
}
</style>
