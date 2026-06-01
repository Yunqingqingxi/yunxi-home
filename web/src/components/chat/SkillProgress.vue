<template>
  <div v-if="summary" class="skill-progress">
    <div class="skill-header">
      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M2 3h3l1 1h4v4H2V3z"/>
      </svg>
      <span class="skill-label">Skill 执行中</span>
    </div>
    <div class="skill-summary" v-html="md(summary)"></div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { renderMarkdown } from '../../stores/chat'

defineProps({
  summary: { type: String, default: '' }
})

function md(text) {
  return renderMarkdown(text)
}
</script>

<style scoped>
.skill-progress {
  border: 1px solid rgba(167,139,250,0.25);
  border-radius: 10px;
  background: rgba(167,139,250,0.06);
  padding: 10px 12px;
  margin: 4px 0;
  animation: skillIn 0.3s var(--ease-out-back);
}
@keyframes skillIn {
  from { opacity: 0; transform: translateY(-4px) scale(0.97); }
  to   { opacity: 1; transform: translateY(0) scale(1); }
}
.skill-header {
  display: flex; align-items: center; gap: 6px;
  font-size: 11px; color: #a78bfa; font-weight: var(--weight-semibold);
  margin-bottom: 6px;
}
.skill-summary {
  font-size: 11.5px; color: var(--text-secondary); line-height: 1.5;
}
.skill-summary :deep(p) { margin: 0; }
.skill-summary :deep(code) {
  background: rgba(167,139,250,0.1); color: #a78bfa;
  padding: 1px 5px; border-radius: 3px; font-size: 10.5px;
}
[data-theme="dark"] .skill-progress {
  background: rgba(167,139,250,0.08);
  border-color: rgba(167,139,250,0.3);
}
</style>
