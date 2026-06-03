<template>
  <div class="error-card">
    <div class="error-icon" v-html="icons.error"></div>
    <div class="error-text">{{ event.error }}</div>
    <div v-if="event.tool_name" class="error-tool">工具: {{ event.tool_name }}</div>
  </div>
</template>

<script setup lang="ts">
import type { LogEvent } from '../../types/logs'
import { ICONS } from '../../stores/logs'
defineProps<{ event: LogEvent }>()
const icons = ICONS
</script>

<style scoped>
.error-card { }
.error-icon { font-size: 13px; display: inline; margin-right: 6px; }
.error-text { display: inline; color: var(--color-danger); font-weight: 500; font-size: 12px; word-break: break-word; }
.error-tool { margin-top: 2px; font-size: 10px; color: var(--text-muted); }
</style>
