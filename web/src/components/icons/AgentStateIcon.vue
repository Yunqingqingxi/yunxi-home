<template>
  <svg
    :class="['agent-state-icon', `state-${state}`, { 'icon-spin': isSpinning, 'icon-pulse': isPulsing, 'icon-blink': isBlinking }]"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <template v-if="state === 'start'">
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" />
    </template>
    <template v-else-if="state === 'reasoning'">
      <path d="M12 4C10 4 8.5 5.5 8.5 7.5c0 1 .5 2 1 2.5C8 10.5 7 12 7 13c0 2 2 3 5 3s5-1 5-3c0-1-1-2.5-2.5-3 .5-.5 1-1.5 1-2.5C15.5 5.5 14 4 12 4z" stroke="currentColor" stroke-width="1.5" fill="none" />
      <path d="M10 11c-.5.8-1 1.5-1 2 0 1 1.2 2 3 2s3-1 3-2c0-.5-.5-1.2-1-2" stroke="currentColor" stroke-width="1.2" fill="none" />
    </template>
    <template v-else-if="state === 'executing'">
      <path d="M12 2l1.5 4.5L18 5l-2 3.5L20 10l-4 1.5L17 16l-3.5-2L10 16l1-4.5L7 10l4-1.5L9 5l4.5 1.5L12 2z" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linejoin="round" />
      <circle cx="12" cy="12" r="3" stroke="currentColor" stroke-width="1.5" fill="none" />
    </template>
    <template v-else-if="state === 'waiting_lock'">
      <path d="M8 11V7a4 4 0 118 0v4" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
      <rect x="6" y="11" width="12" height="9" rx="2" stroke="currentColor" stroke-width="2" fill="none" />
      <circle cx="12" cy="15.5" r="1.2" fill="currentColor" />
    </template>
    <template v-else-if="state === 'waiting_human'">
      <circle cx="12" cy="7" r="3.5" stroke="currentColor" stroke-width="2" fill="none" />
      <path d="M5 21v-2c0-3 3-5.5 7-5.5s7 2.5 7 5.5v2" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
      <circle cx="19" cy="5" r="3.5" fill="var(--bg, #fff)" stroke="currentColor" stroke-width="1.2" />
      <text x="19" y="6.5" text-anchor="middle" font-size="5" font-family="sans-serif" fill="currentColor">?</text>
    </template>
    <template v-else-if="state === 'delegate'">
      <circle cx="6" cy="12" r="2.5" stroke="currentColor" stroke-width="1.5" fill="none" />
      <circle cx="12" cy="5" r="2.5" stroke="currentColor" stroke-width="1.5" fill="none" />
      <circle cx="18" cy="12" r="2.5" stroke="currentColor" stroke-width="1.5" fill="none" />
      <line x1="8.5" y1="12" x2="9.5" y2="6" stroke="currentColor" stroke-width="1.2" />
      <line x1="14.5" y1="6" x2="15.5" y2="12" stroke="currentColor" stroke-width="1.2" />
    </template>
    <template v-else-if="state === 'suspended'">
      <rect x="7" y="5" width="3" height="14" rx="1" fill="currentColor" />
      <rect x="14" y="5" width="3" height="14" rx="1" fill="currentColor" />
    </template>
    <template v-else-if="state === 'timeout'">
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" />
      <line x1="12" y1="12" x2="12" y2="7" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
      <line x1="12" y1="12" x2="16" y2="12" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
    </template>
    <template v-else-if="state === 'retry'">
      <path d="M4 12a8 8 0 0114.5-4.5M20 12a8 8 0 01-14.5 4.5" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" />
      <polyline points="16,4 20,4 20,0" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
    </template>
    <template v-else-if="state === 'done'">
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" />
      <polyline points="7,12 10.5,15.5 17,8" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round" />
    </template>
    <template v-else-if="state === 'failed'">
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" />
      <line x1="8" y1="8" x2="16" y2="16" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
      <line x1="16" y1="8" x2="8" y2="16" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
    </template>
    <template v-else-if="state === 'cancel'">
      <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" />
      <line x1="5" y1="5" x2="19" y2="19" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
    </template>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AgentState } from '../../types/chat'

const props = withDefaults(defineProps<{ state: AgentState; size?: number }>(), { size: 20 })

const isSpinning = computed(() => props.state === 'executing' || props.state === 'retry')
const isPulsing = computed(() => props.state === 'reasoning')
const isBlinking = computed(() => props.state === 'waiting_human')
</script>

<style scoped>
.agent-state-icon { flex-shrink: 0; color: #64748b; }
.state-executing { color: #22c55e; }
.state-reasoning { color: #3b82f6; }
.state-waiting_lock { color: #f59e0b; }
.state-waiting_human { color: #8b5cf6; }
.state-delegate { color: #06b6d4; }
.state-suspended { color: #6b7280; }
.state-timeout { color: #ef4444; }
.state-retry { color: #f59e0b; }
.state-done { color: #22c55e; }
.state-failed { color: #ef4444; }
.state-cancel { color: #6b7280; }
.state-start { color: #94a3b8; }
.icon-spin { animation: spin 2s linear infinite; }
.icon-pulse { animation: pulse 1.5s ease-in-out infinite; }
.icon-blink { animation: blink 1.2s ease-in-out infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.5; } }
@keyframes blink { 0%,100% { opacity: 1; } 50% { opacity: 0.35; } }

@media (prefers-color-scheme: dark) {
  .agent-state-icon { color: #94a3b8; }
  .state-executing { color: #4ade80; }
  .state-reasoning { color: #60a5fa; }
  .state-waiting_lock { color: #fbbf24; }
  .state-waiting_human { color: #a78bfa; }
  .state-delegate { color: #22d3ee; }
  .state-suspended { color: #9ca3af; }
  .state-timeout { color: #f87171; }
  .state-retry { color: #fbbf24; }
  .state-done { color: #4ade80; }
  .state-failed { color: #f87171; }
  .state-cancel { color: #9ca3af; }
}
</style>
