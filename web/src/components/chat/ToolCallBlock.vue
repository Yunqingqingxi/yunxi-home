<template>
  <div class="tool-block" :class="{ pending: !result && status !== 'running', running: status === 'running' }">
    <div class="tool-header" @click="result ? showFull = !showFull : null">
      <code class="tool-name">{{ name }}</code>
      <span v-if="status === 'running' && progress" class="tool-progress">{{ progress }}</span>
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
import { ref, computed } from 'vue'
const props = defineProps<{ name?: string; args?: string; result?: string; status?: string; progress?: string; streaming?: boolean }>()
const showFull = ref(false)
const formattedArgs = computed(() => { if (!props.args) return ''; try { return JSON.stringify(JSON.parse(props.args), null, 2) } catch { return props.args } })
const resultPreview = computed(() => { const r = props.result; if (!r) return ''; const fl = r.split('\n')[0]; return fl.length > 80 ? fl.slice(0, 80) + '…' : fl })
</script>

<style scoped>
.tool-block {
  border: 1px solid rgba(34,197,94,0.2); border-left: 3px solid #22c55e; border-radius: 8px; overflow: hidden;
  margin: 4px 0; font-size: 12px; background: rgba(34,197,94,0.04);
}
.tool-block.running { border-left-color: #22c55e; background: rgba(34,197,94,0.06); }
.tool-block.pending { background: rgba(34,197,94,0.04); }
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
.tool-progress { font-size: 11px; color: #16a34a; font-weight: 500; flex-shrink: 0; }
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
  .tool-progress { color: #4ade80; }
}
</style>
