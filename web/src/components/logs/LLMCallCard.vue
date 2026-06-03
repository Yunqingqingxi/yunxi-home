<template>
  <div class="llm-call-card">
    <div class="llm-main">
      <span class="llm-icon" v-html="icons.brain"></span>
      <span class="llm-model">{{ event.model }}</span>
      <span v-if="event.llm_dur_ms" class="llm-dur">{{ fmtDuration(event.llm_dur_ms) }}</span>
      <span v-if="event.cost_usd" class="llm-cost">${{ event.cost_usd?.toFixed(4) }}</span>
    </div>
    <div class="llm-tokens">
      <span class="tok-item" title="Prompt tokens">📥 {{ fmtTok(event.prompt_tokens) }}</span>
      <span class="tok-item" title="Output tokens">📤 {{ fmtTok(event.output_tokens) }}</span>
      <span v-if="event.cache_tokens" class="tok-item tok-cache" title="Cache tokens"><span v-html="icons.disk"></span> {{ fmtTok(event.cache_tokens) }}</span>
      <span v-if="event.cache_hit" class="cache-hit"><span v-html="icons.zap"></span> 缓存命中</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { LogEvent } from '../../types/logs'
import { ICONS, fmtDuration, fmtTokens } from '../../stores/logs'
const icons = ICONS

defineProps<{ event: LogEvent }>()

function fmtTok(n: number | undefined): string { return fmtTokens(n || 0) }
</script>

<style scoped>
.llm-call-card { }
.llm-main { display: flex; align-items: center; gap: 6px; }
.llm-icon { font-size: 13px; }
.llm-model { font-weight: 600; color: #8b5cf6; font-family: var(--font-mono); font-size: 12px; }
.llm-dur { font-size: 10px; font-family: var(--font-mono); color: var(--text-muted); }
.llm-cost { margin-left: auto; font-size: 10px; font-family: var(--font-mono); color: var(--brand-500); }
.llm-tokens { display: flex; gap: 8px; margin-top: 4px; flex-wrap: wrap; }
.tok-item { font-size: 10px; color: var(--text-muted); font-family: var(--font-mono); }
.tok-cache { color: #06b6d4; }
.cache-hit {
  font-size: 10px; color: #06b6d4; font-weight: 600;
  background: rgba(6,182,212,0.08); padding: 0 5px; border-radius: 3px;
}
</style>
