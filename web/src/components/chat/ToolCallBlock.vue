<template>
  <div class="tool-block" :class="{ pending: !result && status !== 'running', running: status === 'running', expanded: showFull }">
    <div class="tool-header" @click="result ? showFull = !showFull : null">
      <span class="tool-icon">{{ result ? '✓' : status === 'running' ? '▶' : '⟳' }}</span>
      <code class="tool-name">{{ name }}</code>
      <code v-if="formattedArgs" class="tool-args" :title="args">{{ formattedArgs }}</code>
      <span v-if="status === 'running' && progress" class="tool-progress">{{ progress }}</span>
      <span v-if="result" class="tool-result-preview">{{ resultPreview }}</span>
      <svg v-if="result" :class="['chevron', { flipped: showFull }]" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 3.5l2 3 2-3"/></svg>
    </div>
    <div v-if="showFull && args" class="tool-body">
      <div class="tool-section-label">参数</div>
      <code class="tool-json">{{ formattedArgs }}</code>
    </div>
    <div v-if="showFull && result" class="tool-body">
      <div class="tool-section-label">结果</div>
      <code class="tool-result">{{ result }}</code>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed } from 'vue'

const props = defineProps({
  name: { type: String, default: '' },
  args: { type: String, default: '' },
  result: { type: String, default: '' },
  status: { type: String, default: '' },
  progress: { type: String, default: '' },
  streaming: { type: Boolean, default: false }
})

// Auto-expand during streaming, collapsed when finalized (historical)
const showFull = ref(props.streaming)

const formattedArgs = computed(() => {
  const a = props.args
  if (!a) return ''
  try {
    const parsed = JSON.parse(a)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return a
  }
})

const resultPreview = computed(() => {
  const r = props.result
  if (!r) return ''
  // 取第一行，去除 JSON 外层的引号和换行
  const firstLine = r.split('\n')[0]
  return firstLine.length > 80 ? firstLine.slice(0, 80) + '…' : firstLine
})
</script>

<style scoped>
.tool-block {
  border: 1px solid var(--border-default);
  border-radius: 8px; overflow: hidden;
  margin-bottom: 6px; background: var(--surface-card);
  transition: border-color 0.3s, box-shadow 0.3s;
}
.tool-block.pending { border-color: var(--border-default); animation: toolPulse 2s ease-in-out infinite; }
.tool-block.pending .tool-icon { color: var(--text-muted); animation: spin 1.4s linear infinite; }
.tool-block.running { border-color: var(--border-default); box-shadow: 0 0 0 1px var(--border-default); }
.tool-block.running .tool-icon { color: var(--brand-500); animation: pulse 1.2s ease-in-out infinite; }
@keyframes toolPulse { 0%,100%{border-color:var(--tool-border)} 50%{border-color:var(--tool-border-pending)} }
.tool-header {
  display: flex; align-items: center; gap: 6px;
  padding: 7px 10px; font-size: 11.5px; cursor: pointer;
  user-select: none;
}
.tool-header:hover { background: rgba(0,0,0,0.02); }
.tool-icon { font-size: 12px; flex-shrink: 0; min-width: 14px; }
.tool-block:not(.pending):not(.running) .tool-icon { color: var(--tool-icon-done); }
@keyframes spin { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }
.tool-name {
  font-family: var(--font-mono); font-size: 11px;
  color: var(--tool-tag-color); background: var(--tool-tag-bg);
  padding: 2px 7px; border-radius: 4px;
  border: 1px solid var(--tool-tag-border); font-weight: 500;
  flex-shrink: 0;
}
.tool-args {
  font-family: var(--font-mono); font-size: 10px;
  color: var(--text-muted); background: var(--surface-card);
  padding: 2px 7px; border-radius: 4px;
  border: 1px solid var(--border-subtle);
  max-width: 200px; overflow: hidden; text-overflow: ellipsis;
  white-space: nowrap; flex-shrink: 1; cursor: help;
}
.tool-result-preview {
  font-size: 10.5px; color: var(--text-muted);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1; min-width: 0;
}
.tool-progress {
  font-size: 10px; color: var(--brand-500);
  font-weight: var(--weight-medium); flex-shrink: 0;
}
.chevron { flex-shrink: 0; transition: transform 0.2s; opacity: 0.4; }
.chevron.flipped { transform: rotate(180deg); opacity: 0.7; }
.tool-body { padding: 0 10px 8px; }
.tool-section-label {
  font-size: 10px; font-weight: var(--weight-semibold);
  color: var(--text-muted); text-transform: uppercase;
  letter-spacing: 0.5px; margin-bottom: 4px; margin-top: 2px;
}
.tool-json,
.tool-result {
  font-family: var(--font-mono); font-size: 10.5px;
  color: var(--text-secondary); background: var(--surface-card);
  padding: 6px 8px; border-radius: 4px; border: 1px solid var(--border-subtle);
  display: block; max-height: 240px; overflow-y: auto;
  white-space: pre-wrap; word-break: break-all;
}
[data-theme="dark"] .tool-block { background: rgba(245,158,11,0.04); }
[data-theme="dark"] .tool-header:hover { background: rgba(255,255,255,0.02); }
[data-theme="dark"] .tool-args { background: rgba(255,255,255,0.05); }
</style>
