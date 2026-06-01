<template>
  <div
    class="content-block"
    :class="{ streaming }"
  >
    <div
      class="content-body"
      v-html="displayHtml"
    ></div>
    <span
      v-if="streaming"
      class="cursor-blink"
    >|</span>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { computed } from 'vue'
import { renderMarkdown } from '../../stores/chat'

const props = defineProps({
  content: { type: String, default: '' },
  streaming: { type: Boolean, default: false }
})

const displayHtml = computed(() => renderMarkdown(props.content || ''))
</script>

<style scoped>
.content-block { position: relative; }
.content-body :deep(p) { margin: 0 0 6px; }
.content-body :deep(p:last-child) { margin-bottom: 0; }
.content-body :deep(pre) {
  background: #1e293b; color: #e2e8f0;
  padding: 12px 16px; border-radius: 8px; overflow-x: auto;
  margin: 8px 0; font-size: 12px; line-height: 1.6;
  border: 1px solid rgba(255,255,255,0.05);
}
.content-body :deep(pre code) { background: none; padding: 0; color: inherit; font-size: 12px; }
.content-body :deep(code) {
  background: var(--code-bg); color: var(--code-color);
  padding: 2px 5px; border-radius: 4px; font-size: 12px;
  font-family: var(--font-mono);
}
.content-body :deep(h2) { font-size: 15px; font-weight: 700; margin: 10px 0 4px; }
.content-body :deep(h3) { font-size: 14px; font-weight: 650; margin: 8px 0 3px; }
.content-body :deep(ul), .content-body :deep(ol) { margin: 4px 0; padding-left: 18px; }
.content-body :deep(li) { margin: 2px 0; }
.content-body :deep(table) {
  border-collapse: collapse; width: 100%; margin: 8px 0; font-size: 12px;
}
.content-body :deep(th), .content-body :deep(td) {
  border: 1px solid var(--border-default); padding: 5px 8px; text-align: left;
}
.content-body :deep(th) {
  background: var(--surface-hover); font-weight: 600;
  color: var(--text-secondary); font-size: 11px;
  text-transform: uppercase; letter-spacing: 0.5px;
}
.content-body :deep(blockquote) {
  border-left: 3px solid var(--blockquote-border);
  margin: 6px 0; padding: 6px 12px;
  background: var(--blockquote-bg); border-radius: 0 6px 6px 0;
}
.content-body :deep(blockquote p) { margin: 0; }
.content-body :deep(a) { color: var(--brand-600); }
.content-body :deep(hr) { border: none; border-top: 1px solid var(--border-hr); margin: 10px 0; }
.content-body :deep(strong) { font-weight: 650; }
.content-body :deep(img) { max-width: 100%; border-radius: 6px; }

/* 移动端表格溢出滚动 */
@media (max-width: 767px) {
  .content-body :deep(table) {
    display: block; overflow-x: auto; white-space: nowrap;
  }
}
.cursor-blink {
  display: inline-block; width: 2px; height: 15px;
  background: var(--brand-500); margin-left: 2px;
  vertical-align: text-bottom; border-radius: 1px;
  animation: blink 0.7s step-end infinite;
}
@keyframes blink { 0%,100%{opacity:1} 50%{opacity:0} }


</style>
