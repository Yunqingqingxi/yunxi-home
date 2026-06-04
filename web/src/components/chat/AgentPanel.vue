<template>
  <div v-if="agents.length > 0" class="agent-panel">
    <div v-for="agent in visibleAgents" :key="agent.agent_id" class="agent-card" :class="[agent.status, agent.state]">
      <div class="agent-header">
        <AgentStateIcon :state="agent.state || agent.agent_status || 'running'" :size="16" />
        <span class="agent-id">{{ agent.agent_id }}</span>
        <RoleIcon v-if="agent.role && agent.role !== 'executor'" :role="agent.role" :size="14" />
        <span class="agent-round" v-if="agent.agent_round">R{{ agent.agent_round }}</span>
        <span class="agent-status-label" :class="agent.state || agent.agent_status">{{ stateLabel(agent) }}</span>
      </div>
      <div class="agent-goal">{{ truncate(agent.goal || agent.agent_goal || agent.task, 80) }}</div>
      <!-- Progress bar -->
      <div class="agent-progress" v-if="agent.agent_round">
        <span class="progress-bar" :style="{ width: progressPct(agent) + '%', backgroundColor: progressColor(agent) }" />
      </div>
      <!-- State transitions mini timeline -->
      <AgentStateTimeline v-if="agent._transitions && agent._transitions.length > 1" :transitions="agent._transitions" />
      <!-- Waiting indicators -->
      <div v-if="isWaiting(agent)" class="agent-waiting" :class="agent.state">
        <span class="pulse-dot" /> {{ waitMessage(agent) }}
      </div>
      <!-- Result preview -->
      <div v-if="agent.summary || agent.content" class="agent-result">
        <span class="result-icon">{{ agent.status === 'done' ? '✅' : agent.status === 'error' ? '❌' : '📋' }}</span>
        {{ truncate(agent.summary || agent.content, 140) }}
      </div>
      <!-- Error display -->
      <div v-if="agent.error" class="agent-error">{{ truncate(agent.error, 100) }}</div>
    </div>
    <div v-if="hiddenCount > 0" class="agent-more" @click="showAll = !showAll">
      {{ showAll ? '收起' : `+${hiddenCount} 个更多 Agent` }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import AgentStateIcon from '../icons/AgentStateIcon.vue'
import RoleIcon from '../icons/RoleIcon.vue'
import AgentStateTimeline from './AgentStateTimeline.vue'
import type { AgentInfo } from '../../types/chat'

const props = defineProps<{ agents: AgentInfo[] }>()

const showAll = ref(false)
const maxVisible = computed(() => showAll.value ? props.agents.length : 5)
const visibleAgents = computed(() => props.agents.slice(0, maxVisible.value))
const hiddenCount = computed(() => Math.max(0, props.agents.length - maxVisible.value))

function stateLabel(agent: AgentInfo): string {
  const s = agent.state || agent.agent_status || ''
  switch (s) {
    case 'start': return '初始化'
    case 'reasoning': return '思考中'
    case 'executing': return '执行中'
    case 'waiting_lock': return '等待锁'
    case 'waiting_human': return '等待确认'
    case 'delegate': return '委派中'
    case 'suspended': return '已暂停'
    case 'timeout': return '超时'
    case 'retry': return '重试中'
    case 'done': return '✅ 完成'
    case 'failed': return '❌ 失败'
    case 'cancel': return '已取消'
    case 'running': return '执行中'
    case 'pending': return '等待中'
    default: return s || '运行中'
  }
}

function isWaiting(agent: AgentInfo): boolean {
  const s = agent.state || agent.agent_status || ''
  return s === 'waiting_lock' || s === 'waiting_human' || s === 'suspended' || s === 'delegate'
}

function waitMessage(agent: AgentInfo): string {
  const s = agent.state || agent.agent_status || ''
  switch (s) {
    case 'waiting_lock': return '等待资源释放...'
    case 'waiting_human': return '等待用户确认...'
    case 'suspended': return '已暂停，发送消息可恢复'
    case 'delegate': return '等待子任务完成...'
    default: return '等待中...'
  }
}

function progressPct(agent: AgentInfo): number {
  if (agent.state === 'done') return 100
  if (agent.state === 'failed' || agent.state === 'cancel') return 100
  return Math.min(95, (agent.agent_round || 0) * 8)
}

function progressColor(agent: AgentInfo): string {
  if (agent.state === 'failed' || agent.state === 'cancel') return '#ef4444'
  if (agent.state === 'done') return '#22c55e'
  if (agent.state === 'waiting_lock' || agent.state === 'suspended') return '#f59e0b'
  return '#3b82f6'
}

function truncate(s: string | undefined, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}
</script>

<style scoped>
.agent-panel { display: flex; flex-direction: column; gap: 6px; margin: 4px 0; flex-shrink: 0; }
.agent-card {
  border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px;
  padding: 8px 10px; background: var(--surface-card, #fff); transition: all 0.3s;
}
.agent-card.done, .agent-card.failed, .agent-card.cancel { opacity: 0.65; }
.agent-card.executing, .agent-card.running { border-left: 3px solid #22c55e; }
.agent-card.reasoning { border-left: 3px solid #3b82f6; }
.agent-card.waiting_lock, .agent-card.suspended, .agent-card.timeout { border-left: 3px solid #f59e0b; }
.agent-card.waiting_human { border-left: 3px solid #8b5cf6; }
.agent-card.failed, .agent-card.cancel { border-left: 3px solid #ef4444; }
.agent-header { display: flex; align-items: center; gap: 6px; }
.agent-id { font-size: 11px; font-weight: 600; color: var(--text-primary); font-family: monospace; }
.agent-round { font-size: 10px; color: var(--text-muted); background: var(--surface-hover); padding: 1px 6px; border-radius: 4px; }
.agent-status-label { font-size: 10px; color: var(--text-muted); margin-left: auto; text-transform: uppercase; }
.agent-status-label.executing, .agent-status-label.running { color: #22c55e; }
.agent-status-label.reasoning { color: #3b82f6; }
.agent-status-label.failed, .agent-status-label.cancel { color: #ef4444; }
.agent-goal { font-size: 11.5px; color: var(--text-primary); margin-top: 4px; line-height: 1.4; }
.agent-progress { height: 4px; background: var(--surface-hover); border-radius: 2px; margin-top: 6px; overflow: hidden; }
.progress-bar { display: block; height: 100%; border-radius: 2px; transition: width 0.4s ease, background-color 0.3s; }
.agent-waiting { font-size: 10.5px; color: #f59e0b; margin-top: 4px; display: flex; align-items: center; gap: 6px; }
.agent-waiting.waiting_human { color: #8b5cf6; }
.pulse-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; animation: pulse 1.5s ease-in-out infinite; }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.3; } }
.agent-result { margin-top: 4px; padding-top: 4px; border-top: 1px solid var(--border-subtle); font-size: 10.5px; color: var(--text-secondary); line-height: 1.4; display: flex; gap: 4px; align-items: flex-start; }
.result-icon { flex-shrink: 0; }
.agent-error { margin-top: 4px; padding: 4px 6px; background: rgba(239,68,68,0.08); border-radius: 4px; font-size: 10px; color: #ef4444; }
.agent-more { font-size: 10.5px; color: var(--text-muted); text-align: center; padding: 6px; cursor: pointer; border-radius: 6px; }
.agent-more:hover { background: var(--surface-hover); color: var(--text-primary); }

@media (prefers-color-scheme: dark) {
  .agent-card { border-color: #334155; background: #1e293b; }
  .agent-card.done, .agent-card.failed, .agent-card.cancel { opacity: 0.55; }
  .agent-status-label.executing, .agent-status-label.running { color: #4ade80; }
  .agent-status-label.reasoning { color: #60a5fa; }
  .agent-status-label.failed, .agent-status-label.cancel { color: #f87171; }
  .agent-error { background: rgba(239,68,68,0.12); }
}
</style>
