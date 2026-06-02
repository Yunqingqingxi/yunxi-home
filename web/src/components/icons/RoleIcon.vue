<template>
  <svg :class="['role-icon', `role-${role}`]" :width="size" :height="size" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <!-- executor: simple person -->
    <template v-if="role === 'executor'">
      <circle cx="12" cy="5" r="4" stroke="currentColor" stroke-width="1.8" fill="none" />
      <path d="M4 21v-2c0-3.5 3.5-6 8-6s8 2.5 8 6v2" stroke="currentColor" stroke-width="1.8" fill="none" stroke-linecap="round" />
    </template>
    <!-- supervisor: person + magnifying glass -->
    <template v-else-if="role === 'supervisor'">
      <circle cx="12" cy="5" r="4" stroke="currentColor" stroke-width="1.8" fill="none" />
      <path d="M4 21v-2c0-3.5 3.5-6 8-6s8 2.5 8 6v2" stroke="currentColor" stroke-width="1.8" fill="none" stroke-linecap="round" />
      <circle cx="17" cy="17" r="4" stroke="currentColor" stroke-width="1.5" fill="none" />
      <line x1="20" y1="20" x2="22" y2="22" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
    </template>
    <!-- manager: person + star -->
    <template v-else-if="role === 'manager'">
      <circle cx="12" cy="5" r="4" stroke="currentColor" stroke-width="1.8" fill="none" />
      <path d="M4 21v-2c0-3.5 3.5-6 8-6s8 2.5 8 6v2" stroke="currentColor" stroke-width="1.8" fill="none" stroke-linecap="round" />
      <polygon points="19,7 19.8,9.5 22.5,9.5 20.3,11.2 21.1,13.8 19,12.2 16.9,13.8 17.7,11.2 15.5,9.5 18.2,9.5" fill="currentColor" />
    </template>
  </svg>
</template>

<script setup lang="ts">
import type { AgentRole } from '../../types/chat'
withDefaults(defineProps<{ role: AgentRole; size?: number }>(), { size: 18 })
</script>

<style scoped>
.role-icon { flex-shrink: 0; }
.role-executor { color: #94a3b8; }
.role-supervisor { color: #3b82f6; }
.role-manager { color: #eab308; }
@media (prefers-color-scheme: dark) {
  .role-executor { color: #9ca3af; }
  .role-supervisor { color: #60a5fa; }
  .role-manager { color: #facc15; }
}
</style>
