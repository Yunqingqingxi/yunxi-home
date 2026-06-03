<template>
  <!-- 前端兜底过滤：后台执行占位符、静默工具结果不渲染 -->
  <div v-if="!isBackgroundPlaceholder" class="tool-block" :class="{ pending: !result && status !== 'running', running: status === 'running' }">
    <div class="tool-header" @click="result ? showFull = !showFull : null">
      <span v-if="status === 'running'" class="tool-spinner"><span class="spinner-dot"></span></span>
      <code class="tool-name">{{ name }}</code>
      <span v-if="status === 'running'" class="tool-running-label">
        <span class="running-dots">执行中<span class="dot1">.</span><span class="dot2">.</span><span class="dot3">.</span></span>
        <span v-if="elapsed" class="tool-elapsed">{{ elapsed }}</span>
      </span>
      <span v-if="result" class="tool-result-preview">{{ resultPreview }}</span>
      <svg v-if="result" :class="['chevron', { flipped: showFull }]" width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 3.5l2 3 2-3" /></svg>
    </div>
    <div v-if="showFull && args" class="tool-body">
      <code class="tool-json">{{ formattedArgs }}</code>
    </div>
    <div v-if="showFull && result" class="tool-body">
      <code class="tool-result">{{ result }}</code>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
const props = defineProps<{ name?: string; args?: string; result?: string; status?: string; progress?: string; streaming?: boolean }>()
const showFull = ref(false)
const elapsed = ref('')
let timer: ReturnType<typeof setInterval> | null = null

// 静默工具列表（与后端 silentTools 保持一致，前端兜底）
const silentToolNames = new Set(['spawn_agent', 'recall', 'remember', 'request_confirmation', 'activate_specialized_context'])
const isBackgroundPlaceholder = computed(() => {
  if (props.name && silentToolNames.has(props.name)) return true
  if (props.result && (props.result.startsWith('[后台执行]') || props.result.startsWith('[ForceTools'))) return true
  return false
})

watch(() => props.status, (s) => {
  if (s === 'running') {
    const start = Date.now()
    timer = setInterval(() => {
      const sec = Math.floor((Date.now() - start) / 1000)
      elapsed.value = sec >= 60 ? `${Math.floor(sec / 60)}m${sec % 60}s` : `${sec}s`
    }, 1000)
  } else {
    if (timer) { clearInterval(timer); timer = null }
    elapsed.value = ''
  }
})
onUnmounted(() => { if (timer) clearInterval(timer) })

const formattedArgs = computed(() => { if (!props.args) return ''; try { return JSON.stringify(JSON.parse(props.args), null, 2) } catch { return props.args } })
const resultPreview = computed(() => { const r = props.result; if (!r) return ''; const fl = r.split('\n')[0]; return fl.length > 80 ? fl.slice(0, 80) + '…' : fl })
</script>

<style scoped>
.tool-block {
  border: 1px solid rgba(34,197,94,0.2); border-left: 3px solid #22c55e; border-radius: 8px; overflow: hidden;
  margin: 4px 0; font-size: 12px; background: rgba(34,197,94,0.04);
}
.tool-block.running { border-left-color: #22c55e; background: rgba(34,197,94,0.06); animation: toolPulse 2s ease-in-out infinite; }
.tool-block.pending { background: rgba(34,197,94,0.04); }
@keyframes toolPulse {
  0%, 100% { border-left-color: #22c55e; }
  50% { border-left-color: #4ade80; box-shadow: 0 0 8px rgba(34,197,94,0.15); }
}
.tool-header {
  display: flex; align-items: center; gap: 8px; padding: 7px 10px; cursor: pointer; user-select: none;
}
.tool-header:hover { background: rgba(34,197,94,0.06); }
.tool-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 11px; font-weight: 600; color: #166534;
  background: rgba(34,197,94,0.12); padding: 3px 8px; border-radius: 4px;
}
.tool-result-preview { font-size: 12px; color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.tool-running-label { display: flex; align-items: center; gap: 8px; flex: 1; flex-shrink: 0; }
.running-dots { font-size: 11px; color: #16a34a; font-weight: 500; }
.running-dots .dot1 { animation: dotBounce 1.2s infinite; }
.running-dots .dot2 { animation: dotBounce 1.2s 0.2s infinite; }
.running-dots .dot3 { animation: dotBounce 1.2s 0.4s infinite; }
@keyframes dotBounce {
  0%, 100% { opacity: 0.2; }
  50% { opacity: 1; }
}
.tool-elapsed { font-size: 10px; color: var(--text-muted); font-family: var(--font-mono); }
.tool-spinner { display: flex; align-items: center; flex-shrink: 0; }
.spinner-dot {
  width: 8px; height: 8px; border-radius: 50%; background: #22c55e;
  animation: spinnerPulse 1s ease-in-out infinite;
}
@keyframes spinnerPulse {
  0%, 100% { transform: scale(1); opacity: 0.6; }
  50% { transform: scale(1.4); opacity: 1; }
}
.chevron { flex-shrink: 0; transition: transform 0.15s; opacity: 0.5; }
.chevron.flipped { transform: rotate(180deg); }
.tool-body { padding: 0 10px 8px; }
.tool-json, .tool-result {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 11px; color: var(--text-secondary);
  background: rgba(34,197,94,0.04); padding: 8px 10px; border-radius: 4px;
  display: block; max-height: 200px; overflow-y: auto; white-space: pre-wrap; word-break: break-all;
}
@media (prefers-color-scheme: dark) {
  .tool-block { border-color: rgba(34,197,94,0.15); background: rgba(34,197,94,0.05); }
  .tool-block.running { background: rgba(34,197,94,0.08); }
  .tool-block.pending { background: rgba(34,197,94,0.05); }
  .tool-header:hover { background: rgba(34,197,94,0.08); }
  .tool-name { background: rgba(34,197,94,0.15); color: #4ade80; }
  .tool-json, .tool-result { background: rgba(34,197,94,0.05); }
}
</style>
