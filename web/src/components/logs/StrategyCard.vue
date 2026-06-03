<template>
  <div class="strategy-card">
    <span class="strategy-icon" v-html="icons.strategy"></span>
    <span class="strategy-name">{{ event.strategy }}</span>
    <span v-if="event.strategy_reason" class="strategy-reason">
      — {{ event.strategy_reason }}
    </span>
  </div>
</template>

<script setup lang="ts">
import type { LogEvent } from '../../types/logs'
import { ICONS } from '../../stores/logs'
const icons = ICONS
defineProps<{ event: LogEvent }>()
</script>

<style scoped>
.strategy-card { display: flex; align-items: baseline; gap: 6px; flex-wrap: wrap; }
.strategy-icon { font-size: 13px; }
.strategy-name { font-weight: 600; color: #f59e0b; font-family: var(--font-mono); font-size: 12px; }
.strategy-reason { font-size: 11px; color: var(--text-muted); }
</style>
