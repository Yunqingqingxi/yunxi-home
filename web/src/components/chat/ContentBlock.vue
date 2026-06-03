<template>
  <div class="content-block">
    <div class="content-body" v-html="displayHtml" />
    <span v-if="streaming" class="cursor-blink">|</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown } from '../../stores/chat'
const props = defineProps<{ content?: string; streaming?: boolean }>()

const token = typeof localStorage !== 'undefined' ? localStorage.getItem('token') || '' : ''
const fileLinkRe = /\[文件:\s*([^\]]+?)\s*\(([^)]+)\)\]/g

const linkedText = computed(() => {
  let text = props.content || ''
  text = text.replace(fileLinkRe, (_m: string, name: string, path: string) => {
    const streamUrl = '/api/nas/files/stream?path=' + encodeURIComponent(path.trim()) + '&token=' + token
    return `[${name.trim()}](${streamUrl})`
  })
  text = text.replace(/\n{3,}/g, '\n\n')
  return text.trim()
})

const displayHtml = computed(() => renderMarkdown(linkedText.value))
</script>

<style scoped>
.content-body :deep(p) { margin: 0 0 4px; }
.content-body :deep(p:last-child) { margin-bottom: 0; }
.content-body :deep(pre) {
  background: #1e293b; color: #e2e8f0; padding: 12px; border-radius: 8px; overflow-x: auto;
  margin: 8px 0; font-size: 12px; line-height: 1.6;
}
.content-body :deep(code) { background: var(--surface-hover, #f1f5f9); padding: 2px 4px; border-radius: 4px; font-size: 11.5px; }
.content-body :deep(pre code) { background: none; padding: 0; font-size: 12px; color: inherit; }
.content-body :deep(ul), .content-body :deep(ol) { margin: 4px 0; padding-left: 20px; }
.content-body :deep(li) { margin: 2px 0; }
.content-body :deep(blockquote) {
  border-left: 2px solid var(--border-default, #cbd5e1); margin: 4px 0; padding: 4px 12px; color: var(--text-muted);
}
.content-body :deep(a) { color: var(--brand-600, #0284c7); }
.content-body :deep(img) { max-width: 100%; border-radius: 4px; }
.content-body :deep(h2) { font-size: 16px; font-weight: 700; margin: 12px 0 4px; }
.content-body :deep(h3) { font-size: 14px; font-weight: 600; margin: 8px 0 4px; }
.content-body :deep(table) { border-collapse: collapse; width: 100%; margin: 8px 0; font-size: 12px; }
.content-body :deep(th) { background: var(--surface-hover, #f1f5f9); padding: 6px 8px; text-align: left; font-weight: 600; border: 1px solid var(--border-subtle, #e2e8f0); }
.content-body :deep(td) { padding: 5px 8px; border: 1px solid var(--border-subtle, #e2e8f0); }
.cursor-blink { display: inline-block; width: 1px; height: 14px; background: var(--text-muted); margin-left: 2px; vertical-align: text-bottom; animation: blink 0.7s step-end infinite; }
@keyframes blink { 0%,100% { opacity: 1; } 50% { opacity: 0; } }
</style>
