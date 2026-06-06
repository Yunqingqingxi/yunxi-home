<template>
  <div class="thinking-block" :class="{ streaming }">
    <button class="thinking-toggle" @click="open = !open">
      <span class="thinking-label">{{ streaming ? '思考中...' : '思考过程' }}</span>
      <svg :class="['chevron', { flipped: open }]" width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 4.5l2 3 2-3" /></svg>
    </button>
    <div v-show="open" class="thinking-body" v-html="displayHtml" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { renderMarkdown } from '../../stores/chat'
import { useSettingsStore } from '../../stores/settings'
const props = defineProps<{ reasoning?: string; streaming?: boolean }>()
const open = ref(false)
const displayHtml = computed(() => renderMarkdown(props.reasoning || ''))
const settings = useSettingsStore()
const autoExpand = computed(() => !!settings.aiConfig?.expand_thinking_on_stream)
watch([() => props.streaming, autoExpand], ([s, expand]) => {
  if (expand) open.value = !!s
})
</script>

<style scoped>
.thinking-block {
  border-left: 2px solid var(--border-default, #cbd5e1); padding: 6px 0 6px 10px; margin: 0;
}
.thinking-block.streaming { border-left-color: #3b82f6; }
.thinking-toggle {
  display: flex; align-items: center; gap: 4px; border: none; background: transparent;
  cursor: pointer; font-size: 11px; color: var(--text-muted); font-family: inherit; padding: 0; width: 100%;
}
.thinking-toggle:hover { color: var(--text-secondary); }
.thinking-label { font-weight: 500; }
.chevron { flex-shrink: 0; margin-left: auto; transition: transform 0.15s; opacity: 0.4; }
.chevron.flipped { transform: rotate(180deg); }
.thinking-body {
  font-size: 11px; color: var(--text-muted); line-height: 1.55; margin-top: 4px;
}
.thinking-body :deep(p) { margin: 2px 0; }
.thinking-body :deep(code) { background: var(--surface-hover, #f1f5f9); padding: 1px 4px; border-radius: 3px; font-size: 10px; }
.thinking-body :deep(ul),
.thinking-body :deep(ol) { margin: 4px 0; padding-left: 18px; }
.thinking-body :deep(li) { margin: 2px 0; }
.thinking-body :deep(li)::marker { font-size: 10px; color: var(--text-muted); }
@media (prefers-color-scheme: dark) {
  .thinking-block { border-left-color: #475569; }
  .thinking-block.streaming { border-left-color: #60a5fa; }
}
</style>
