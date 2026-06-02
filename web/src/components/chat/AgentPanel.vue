<template>
  <div v-if="agents.length > 0" class="agent-panel">
    <div v-for="agent in visibleAgents" :key="agent.agent_id" class="agent-card" :class="agent.status">
      <div class="agent-header">
        <AgentStateIcon :state="agent.state || 'executing'" :size="16" />
        <span class="agent-id">{{ agent.agent_id }}</span>
        <RoleIcon v-if="agent.role && agent.role !== 'executor'" :role="agent.role" :size="14" />
        <span class="agent-round" v-if="agent.agent_round">R{{ agent.agent_round }}</span>
        <span class="agent-status-label" :class="agent.state">{{ stateLabel(agent) }}</span>
      </div>
      <div class="agent-goal">{{ truncate(agent.goal || agent.agent_goal, 64) }}</div>
      <div class="agent-progress" v-if="agent.agent_round">
        <span class="progress-bar" :style="{ width: Math.min(100, (agent.agent_round || 0) * 10) + '%' }" />
      </div>
      <AgentStateTimeline v-if="agent._transitions" :transitions="agent._transitions" />
      <div v-if="agent.state === 'waiting_lock' || agent.state === 'waiting_human'" class="agent-waiting">
        {{ agent.state === 'waiting_lock' ? '等待资源释放...' : '等待用户确认...' }}
      </div>
      <div v-if="agent.summary || agent.content" class="agent-result">
        {{ truncate(agent.summary || agent.content, 120) }}
      </div>
    </div>
    <div v-if="hiddenCount > 0" class="agent-more">+{{ hiddenCount }} 个更多</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AgentStateIcon from '../icons/AgentStateIcon.vue'
import RoleIcon from '../icons/RoleIcon.vue'
import AgentStateTimeline from './AgentStateTimeline.vue'

const props = defineProps<{ agents: any[] }>()

const maxVisible = 5
const visibleAgents = computed(() => props.agents.slice(0, maxVisible))
const hiddenCount = computed(() => Math.max(0, props.agents.length - maxVisible))

function stateLabel(agent: any): string {
  const s = agent.state || agent.agent_status || ''
  switch (s) {
    case 'reasoning': return '思考中'
    case 'executing': return '执行中'
    case 'waiting_lock': return '等待锁'
    case 'waiting_human': return '等待确认'
    case 'delegate': return '委派中'
    case 'suspended': return '已暂停'
    case 'done': return '完成'
    case 'failed': return '失败'
    case 'cancel': return '已取消'
    case 'running': return '执行中'
    default: return s || '运行中'
  }
}

function truncate(s: string, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}
</script>

<style scoped>
.agent-panel { display: flex; flex-direction: column; gap: 6px; margin: 4px 0; flex-shrink: 0; }
.agent-card { border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px; padding: 8px 10px; background: var(--surface-card, #fff); transition: opacity 0.2s; }
.agent-card.done { opacity: 0.7; }
.agent-header { display: flex; align-items: center; gap: 6px; }
.agent-id { font-size: 11px; font-weight: 600; color: var(--text-primary); font-family: monospace; }
.agent-round { font-size: 10px; color: var(--text-muted); background: var(--surface-hover); padding: 1px 6px; border-radius: 4px; }
.agent-status-label { font-size: 10px; color: var(--text-muted); margin-left: auto; }
.agent-status-label.executing, .agent-status-label.running { color: #22c55e; }
.agent-status-label.reasoning { color: #3b82f6; }
.agent-status-label.failed, .agent-status-label.cancel { color: #ef4444; }
.agent-goal { font-size: 11.5px; color: var(--text-primary); margin-top: 4px; line-height: 1.4; }
.agent-progress { height: 3px; background: var(--surface-hover); border-radius: 2px; margin-top: 6px; overflow: hidden; }
.progress-bar { display: block; height: 100%; background: #3b82f6; border-radius: 2px; transition: width 0.5s ease; }
.agent-waiting { font-size: 10px; color: #f59e0b; margin-top: 4px; }
.agent-result { margin-top: 4px; padding-top: 4px; border-top: 1px solid var(--border-subtle); font-size: 10.5px; color: var(--text-secondary); line-height: 1.4; }
.agent-more { font-size: 10px; color: var(--text-muted); text-align: center; padding: 4px; }

@media (prefers-color-scheme: dark) {
  .agent-card { border-color: #334155; background: #1e293b; }
  .agent-status-label.executing, .agent-status-label.running { color: #4ade80; }
  .agent-status-label.reasoning { color: #60a5fa; }
  .agent-status-label.failed, .agent-status-label.cancel { color: #f87171; }
}
</style>
