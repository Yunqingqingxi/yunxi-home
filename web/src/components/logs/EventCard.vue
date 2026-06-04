<template>
  <div
    :class="['event-card', `event-${event.type}`]"
    @click="expanded = !expanded"
  >
    <div class="event-header">
      <span class="event-icon" v-html="config.icon"></span>
      <span class="event-type">{{ config.label }}</span>
      <span class="event-time">{{ formatHMS(event.ts) }}</span>
      <span v-if="event.round" class="event-round">R{{ event.round }}</span>
      <span v-if="event.tool_dur_ms" class="event-dur">{{ formatDuration(event.tool_dur_ms) }}</span>
      <span v-if="event.risk_level" :class="['risk-badge', `risk-${event.risk_level}`]">
        {{ riskLabel(event.risk_level) }}
      </span>
    </div>
    <div class="event-body">
      <!-- Tool Call / Start -->
      <template v-if="event.type === 'tool_call' || event.type === 'tool_start'">
        <ToolCallCard :event="event" />
      </template>
      <!-- Tool Result -->
      <template v-else-if="event.type === 'tool_result'">
        <ToolResultCard :event="event" />
      </template>
      <!-- LLM Call -->
      <template v-else-if="event.type === 'llm_call_done'">
        <LLMCallCard :event="event" />
      </template>
      <!-- Error -->
      <template v-else-if="event.type === 'error'">
        <ErrorCard :event="event" />
      </template>
      <!-- Strategy -->
      <template v-else-if="event.type === 'strategy'">
        <StrategyCard :event="event" />
      </template>
      <!-- User message -->
      <template v-else-if="event.type === 'user_message'">
        <div class="content-text">{{ event.content }}</div>
      </template>
      <!-- Answer / Content / Thinking -->
      <template v-else-if="event.type === 'answer' || event.type === 'content' || event.type === 'thinking'">
        <div class="content-text">{{ truncateText(event.content, 300) }}</div>
      </template>
      <!-- Generic -->
      <template v-else>
        <div v-if="event.content" class="content-text">{{ truncateText(event.content, 200) }}</div>
        <div v-if="event.tool_name" class="meta-row">工具: {{ event.tool_name }}</div>
        <div v-if="event.model" class="meta-row">模型: {{ event.model }}</div>
        <div v-if="event.error" class="meta-row error-text">{{ event.error }}</div>
      </template>
    </div>
    <!-- Expand detail -->
    <div v-if="expanded" class="event-detail">
      <div class="detail-section" v-if="event.tool_args">
        <div class="detail-label">参数:</div>
        <pre class="detail-pre">{{ formatJSON(event.tool_args) }}</pre>
      </div>
      <div class="detail-section" v-if="event.tool_result && event.type !== 'tool_result'">
        <div class="detail-label">结果:</div>
        <pre class="detail-pre">{{ truncateText(event.tool_result, 1000) }}</pre>
      </div>
      <div class="detail-section" v-if="event.content && hasLongContent">
        <div class="detail-label">完整内容:</div>
        <div class="detail-text">{{ event.content }}</div>
      </div>
      <div class="detail-section">
        <button class="raw-btn" @click.stop="$emit('toggle-json')">
          { } {{ expandedProp ? '收起 JSON' : '查看原始 JSON' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { LogEvent, RiskLevel } from '../../types/logs'
import { EVENT_TYPE_CONFIG, RISK_CONFIG } from '../../stores/logs'
import { formatDuration, formatHMS } from '../../composables/useFormat'
import ToolCallCard from './ToolCallCard.vue'
import ToolResultCard from './ToolResultCard.vue'
import LLMCallCard from './LLMCallCard.vue'
import ErrorCard from './ErrorCard.vue'
import StrategyCard from './StrategyCard.vue'

const props = defineProps<{ event: LogEvent; index: number; expanded: boolean }>()
defineEmits<{ 'toggle-json': [] }>()

const expanded = ref(false)
const expandedProp = computed(() => props.expanded)

const config = computed(() => EVENT_TYPE_CONFIG[props.event.type] || { icon: '•', label: props.event.type, color: '#94a3b8' })
const hasLongContent = computed(() => (props.event.content?.length || 0) > 300)

function formatTime(ts: string): string {
  return formatHMS(ts)
}

function truncateText(text: string | undefined, max: number): string {
  if (!text) return ''
  if (text.length <= max) return text
  return text.slice(0, max) + '...'
}

function formatJSON(raw: string): string {
  try { return JSON.stringify(JSON.parse(raw), null, 2) }
  catch { return raw }
}

function riskLabel(level: string): string {
  return RISK_CONFIG[level]?.label || level
}
</script>

<style scoped>
.event-card {
  margin: 2px 0; padding: 8px 12px;
  border-left: 3px solid var(--event-color, #94a3b8);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  background: var(--glass-bg-card); cursor: pointer;
  transition: background 0.1s; position: relative;
}
.event-card:hover { background: var(--surface-hover); }

/* 事件类型边框颜色 */
.event-tool_call, .event-tool_start, .event-tool_progress, .event-tool_result { --event-color: #10b981; }
.event-llm_call_done { --event-color: #8b5cf6; }
.event-error { --event-color: #dc2626; background: rgba(220,38,38,0.03); }
.event-thinking { --event-color: #6366f1; }
.event-content, .event-answer { --event-color: #06b6d4; }
.event-user_message { --event-color: #3b82f6; }
.event-strategy { --event-color: #f59e0b; }
.event-agent_result { --event-color: #0ea5e9; }
.event-session_start, .event-session_end, .event-session_save, .event-compaction { --event-color: #64748b; }

.event-header {
  display: flex; align-items: center; gap: 6px; margin-bottom: 4px;
  font-size: 10px; color: var(--text-muted);
}
.event-icon { font-size: 11px; }
.event-type { font-weight: 600; color: var(--text-secondary); }
.event-time { font-family: var(--font-mono); }
.event-round { margin-left: auto; background: var(--border-subtle); padding: 0 5px; border-radius: 3px; font-size: 9px; }
.event-dur { font-family: var(--font-mono); color: var(--text-muted); }
.risk-badge { padding: 0 5px; border-radius: 3px; font-size: 9px; font-weight: 600; }
.risk-readonly { background: #dcfce7; color: #16a34a; }
.risk-mutation { background: #fef3c7; color: #d97706; }
.risk-dangerous { background: #fee2e2; color: #dc2626; }

.event-body { font-size: 12px; }
.content-text { color: var(--text-primary); line-height: 1.5; word-break: break-word; }
.meta-row { font-size: 11px; color: var(--text-muted); }
.error-text { color: var(--color-danger); }

.event-detail { margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border-subtle); }
.detail-section { margin-bottom: 6px; }
.detail-label { font-size: 10px; color: var(--text-muted); font-weight: 600; margin-bottom: 2px; text-transform: uppercase; }
.detail-pre {
  margin: 0; padding: 6px 8px; font-family: var(--font-mono); font-size: 11px;
  line-height: 1.4; white-space: pre-wrap; word-break: break-all;
  background: var(--surface-hover); border-radius: var(--radius-xs);
  color: var(--text-secondary); max-height: 200px; overflow-y: auto;
}
.detail-text { font-size: 12px; line-height: 1.5; color: var(--text-primary); white-space: pre-wrap; word-break: break-word; }
.raw-btn {
  padding: 3px 10px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; cursor: pointer; font-size: 11px; font-family: inherit;
  color: var(--text-muted);
}
.raw-btn:hover { background: var(--surface-hover); color: var(--text-primary); }

[data-theme="dark"] .event-error { background: rgba(248,113,113,0.06); }
</style>
