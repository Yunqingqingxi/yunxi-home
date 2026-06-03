<template>
  <div class="analytics-overview">
    <div class="stat-card glass-card">
      <div class="stat-label"><span v-html="icons.chart"></span> 总请求</div>
      <div class="stat-value">{{ fmtNum(analytics.total_requests) }}</div>
    </div>
    <div class="stat-card glass-card">
      <div class="stat-label">❌ 错误率</div>
      <div class="stat-value" :class="analytics.error_rate > 10 ? 'bad' : analytics.error_rate > 3 ? 'warn' : ''">
        {{ analytics.error_rate.toFixed(1) }}%
      </div>
    </div>
    <div class="stat-card glass-card">
      <div class="stat-label">🟢 活跃会话</div>
      <div class="stat-value">{{ analytics.active_sessions }}</div>
    </div>
    <div class="stat-card glass-card">
      <div class="stat-label">🪙 Token</div>
      <div class="stat-value">{{ fmtTok(analytics.total_tokens_in + analytics.total_tokens_out) }}</div>
      <div class="stat-sub">📥 {{ fmtTok(analytics.total_tokens_in) }} 📤 {{ fmtTok(analytics.total_tokens_out) }}</div>
    </div>
    <div class="stat-card glass-card">
      <div class="stat-label">💰 费用</div>
      <div class="stat-value">${{ analytics.total_cost_usd.toFixed(4) }}</div>
    </div>
    <div class="stat-card glass-card">
      <div class="stat-label">🔧 工具调用</div>
      <div class="stat-value">{{ toolTotal }}</div>
      <div class="stat-sub">{{ toolSuccessRate }}% 成功</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AnalyticsSummary, ToolStat } from '../../types/logs'
import { fmtTokens, ICONS } from '../../stores/logs'
const icons = ICONS

const props = defineProps<{
  analytics: AnalyticsSummary
  toolStats: ToolStat[]
  loading: boolean
}>()

const toolTotal = computed(() => props.toolStats.reduce((sum, t) => sum + t.calls, 0))
const toolSuccessRate = computed(() => {
  const total = toolTotal.value
  if (!total) return 100
  const errors = props.toolStats.reduce((sum, t) => sum + t.errors, 0)
  return ((1 - errors / total) * 100).toFixed(1)
})

function fmtNum(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

function fmtTok(n: number): string { return fmtTokens(n) }
</script>

<style scoped>
.analytics-overview {
  display: flex; gap: 8px; flex-wrap: wrap;
}
.stat-card {
  flex: 1; min-width: 110px; padding: 10px 14px;
  display: flex; flex-direction: column; gap: 2px;
}
.stat-label { font-size: 11px; color: var(--text-muted); }
.stat-value { font-size: 22px; font-weight: 700; color: var(--text-primary); font-variant-numeric: tabular-nums; }
.stat-value.bad { color: var(--color-danger); }
.stat-value.warn { color: var(--color-warning); }
.stat-sub { font-size: 10px; color: var(--text-muted); }
</style>
