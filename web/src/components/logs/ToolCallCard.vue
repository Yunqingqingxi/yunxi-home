<template>
  <div class="tool-call-card">
    <div class="tool-main">
      <span class="tool-icon" v-html="icons.tool"></span>
      <span class="tool-name">{{ event.tool_name }}</span>
      <span v-if="event.tool_dur_ms" class="tool-dur">{{ fmtDuration(event.tool_dur_ms) }}</span>
    </div>
    <div v-if="event.tool_args" class="tool-args">{{ truncate(event.tool_args, 150) }}</div>
    <!-- Duration bar -->
    <div v-if="event.tool_dur_ms" class="dur-bar">
      <div class="dur-fill" :style="{ width: durPercent + '%' }"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { LogEvent } from '../../types/logs'
import { ICONS, fmtDuration } from '../../stores/logs'
const icons = ICONS

const props = defineProps<{ event: LogEvent }>()

const maxDur = 30000 // 30s max for bar scaling

const durPercent = computed(() => {
  const d = props.event.tool_dur_ms || 0
  return Math.min((d / maxDur) * 100, 100)
})

function truncate(s: string, max: number): string {
  if (s.length <= max) return s
  return s.slice(0, max) + '...'
}
</script>

<style scoped>
.tool-call-card { }
.tool-main { display: flex; align-items: center; gap: 6px; }
.tool-icon { font-size: 13px; }
.tool-name { font-weight: 600; color: #16a34a; font-family: var(--font-mono); font-size: 12px; }
.tool-dur { margin-left: auto; font-size: 10px; font-family: var(--font-mono); color: var(--text-muted); }
.tool-args { font-size: 11px; color: var(--text-muted); font-family: var(--font-mono); margin-top: 2px; word-break: break-all; }
.dur-bar { height: 3px; background: var(--bg-progress-track); border-radius: 2px; margin-top: 4px; overflow: hidden; }
.dur-fill { height: 100%; background: #10b981; border-radius: 2px; transition: width 0.3s; min-width: 2px; }
</style>
