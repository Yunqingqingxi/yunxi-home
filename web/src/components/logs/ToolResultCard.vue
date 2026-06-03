<template>
  <div class="tool-result-card">
    <div class="result-main">
      <span class="result-icon" v-html="icons.check"></span>
      <span class="result-tool">{{ event.tool_name }}</span>
      <span class="result-status" :class="event.tool_status === 'error' ? 'bad' : 'good'">
        {{ event.tool_status === 'error' ? '失败' : '完成' }}
      </span>
      <span v-if="event.duration_sec" class="result-dur">{{ event.duration_sec }}s</span>
    </div>
    <div v-if="event.tool_result" class="result-text">{{ truncate(event.tool_result, 200) }}</div>
    <div v-if="event.tool_status === 'error'" class="result-error">⚠ {{ event.error }}</div>
  </div>
</template>

<script setup lang="ts">
import type { LogEvent } from '../../types/logs'
import { ICONS } from '../../stores/logs'
const icons = ICONS

const props = defineProps<{ event: LogEvent }>()

function truncate(s: string, max: number): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '...'
}
</script>

<style scoped>
.tool-result-card { }
.result-main { display: flex; align-items: center; gap: 6px; }
.result-icon { font-size: 13px; }
.result-tool { font-weight: 600; color: #16a34a; font-family: var(--font-mono); font-size: 12px; }
.result-status { font-size: 10px; padding: 0 5px; border-radius: 3px; font-weight: 600; }
.result-status.good { background: #dcfce7; color: #16a34a; }
.result-status.bad { background: #fee2e2; color: #dc2626; }
.result-dur { margin-left: auto; font-size: 10px; font-family: var(--font-mono); color: var(--text-muted); }
.result-text { font-size: 11px; color: var(--text-muted); margin-top: 2px; word-break: break-all; }
.result-error { margin-top: 4px; font-size: 11px; color: var(--color-danger); }
</style>
