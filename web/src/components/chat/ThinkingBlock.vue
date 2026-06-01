<template>
  <div class="thinking-block" :class="{ streaming, collapsed: !open }">
    <button class="thinking-toggle" @click="open = !open">
      <span class="thinking-dot" :class="{ pulse: streaming }"></span>
      <span class="thinking-label">{{ streaming ? '思考中' : '思考过程' }}</span>
      <svg :class="['chevron', { flipped: open }]" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 4.5l2 3 2-3"/></svg>
    </button>
    <div v-show="open" class="thinking-body">{{ reasoning }}</div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, watch } from 'vue'

const props = defineProps({
  reasoning: { type: String, default: '' },
  streaming: { type: Boolean, default: false }
})

const open = ref(false)

// Auto-expand during streaming
watch(() => props.streaming, (s) => { if (s) { open.value = true } }, { immediate: true })
</script>

<style scoped>
.thinking-block {
  background: var(--thinking-bg);
  border: 1px solid var(--thinking-border); border-radius: 10px; overflow: hidden;
  margin-bottom: 6px;
}
.thinking-block.streaming { border-color: rgba(6,182,212,0.20); border-left-color: var(--brand-400); }
.thinking-toggle {
  display: flex; align-items: center; gap: 6px; width: 100%;
  padding: 8px 12px; border: none; background: transparent;
  cursor: pointer; font-size: 12px; color: var(--text-muted);
  font-family: inherit;
}
.thinking-toggle:hover { color: var(--text-secondary); }
.thinking-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: var(--thinking-color); opacity: 0.5; flex-shrink: 0;
}
.thinking-dot.pulse { animation: thinkPulse 1.4s ease-in-out infinite; }
@keyframes thinkPulse {
  0%,100%{opacity:.35;transform:scale(.9)} 50%{opacity:1;transform:scale(1.2)}
}
.thinking-label { font-weight: 500; flex: 1; text-align: left; }
.chevron { flex-shrink: 0; transition: transform 0.2s; opacity: 0.4; }
.chevron.flipped { transform: rotate(180deg); opacity: 0.7; }
.thinking-body {
  padding: 0 12px 10px; font-size: 11.5px;
  color: var(--thinking-text); line-height: 1.55;
  white-space: pre-wrap; font-style: italic;
}


/* Extra polish */
.thinking-body {
  font-size: 11.5px; line-height: 1.6;
}
[data-theme="dark"] .thinking-block {
  background: var(--thinking-bg);
  border-color: var(--thinking-border);
}

</style>
/* Dark mode */
[data-theme="dark"] .thinking-block {
  background: rgba(34,211,238,0.04);
  border-color: rgba(34,211,238,0.12);
  border-left-color: rgba(34,211,238,0.30);
}
[data-theme="dark"] .thinking-block.streaming {
  border-color: rgba(34,211,238,0.18);
  border-left-color: rgba(34,211,238,0.45);
}