<template>
  <div :class="['sys-log-line', `level-${parsed.level.toLowerCase()}`]" @click="expanded = !expanded">
    <span class="line-time">{{ parsed.timestamp }}</span>
    <span v-if="parsed.component" :class="['line-comp', `comp-${parsed.component.toLowerCase()}`]">
      {{ parsed.component }}
    </span>
    <span v-if="parsed.level" :class="['line-level', `lvl-${parsed.level.toLowerCase()}`]">{{ parsed.level }}</span>
    <span class="line-msg" v-html="highlightedMsg"></span>
    <span class="line-tags">
      <span v-for="(v, k) in parsed.fields" :key="k" class="line-tag" v-show="k !== 'time' && k !== 'level' && k !== 'msg' && k !== 'component'" :title="`${k}=${v}`">{{ k }}={{ v }}</span>
    </span>

    <!-- Expanded detail -->
    <div v-if="expanded" class="line-detail">
      <div class="detail-grid">
        <div v-for="(v, k) in parsed.fields" :key="k" class="detail-row">
          <span class="detail-key">{{ k }}</span>
          <span class="detail-val">{{ v }}</span>
        </div>
      </div>
      <pre class="detail-raw">{{ line }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ParsedLogLine } from '../../types/logs'
import { parseLogLine } from '../../stores/logs'

const props = defineProps<{ line: string; search: string; index: number }>()
const expanded = ref(false)

const parsed = computed<ParsedLogLine>(() => parseLogLine(props.line))

const highlightedMsg = computed(() => {
  let msg = parsed.value.message || props.line
  // Escape HTML
  const div = document.createElement('div')
  div.textContent = msg
  msg = div.innerHTML
  if (props.search) {
    const re = new RegExp(`(${escapeRegex(props.search)})`, 'gi')
    msg = msg.replace(re, '<mark class="search-highlight">$1</mark>')
  }
  return msg
})

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
</script>

<style scoped>
.sys-log-line {
  padding: 5px 8px; margin: 1px 0; border-radius: var(--radius-xs);
  font-family: var(--font-mono); font-size: 11px; line-height: 1.5;
  display: flex; gap: 6px; align-items: baseline; flex-wrap: wrap;
  cursor: pointer; transition: background 0.1s;
  border-left: 2px solid transparent;
}
.sys-log-line:hover { background: var(--surface-hover); }

.sys-log-line.level-error { border-left-color: var(--color-danger); background: rgba(220,38,38,0.03); }
.sys-log-line.level-warn  { border-left-color: #d97706; }
.sys-log-line.level-debug { opacity: 0.65; }
.sys-log-line.level-info  { border-left-color: transparent; }

.line-time { color: var(--text-muted); white-space: nowrap; min-width: 65px; }
.line-comp {
  font-weight: 600; font-size: 10px; padding: 0 5px; border-radius: 3px;
  white-space: nowrap; min-width: 32px; text-align: center;
  transition: opacity 0.15s, transform 0.1s;
}
.line-comp:hover { opacity: 0.85; transform: scale(1.05); }
.comp-web       { background: rgba(6,182,212,0.12); color: #0891b2; }
.comp-dns       { background: rgba(99,102,241,0.1); color: #6366f1; }
.comp-ai        { background: rgba(168,85,247,0.1); color: #8b5cf6; }
.comp-database, .comp-db { background: rgba(16,185,129,0.1); color: #10b981; }
.comp-scheduler { background: rgba(245,158,11,0.1); color: #d97706; }
.comp-bot       { background: rgba(59,130,246,0.1); color: #3b82f6; }
.comp-executor  { background: rgba(239,68,68,0.1); color: #dc2626; }
.line-level {
  font-weight: 600; font-size: 10px; padding: 0 4px; border-radius: 3px; white-space: nowrap;
  transition: opacity 0.15s, transform 0.1s;
}
.line-level:hover { opacity: 0.85; transform: scale(1.05); }
.lvl-error { background: rgba(220,38,38,0.12); color: #dc2626; }
.lvl-warn  { background: rgba(217,119,6,0.12); color: #d97706; }
.lvl-info  { background: rgba(6,182,212,0.1); color: #0891b2; }
.lvl-debug { background: rgba(148,163,184,0.1); color: #94a3b8; }
.line-msg { color: var(--text-primary); word-break: break-word; }
.line-tags { display: flex; gap: 3px; flex-wrap: wrap; }
.line-tag {
  font-size: 9px; color: var(--text-muted);
  background: var(--border-subtle); padding: 0 4px; border-radius: 3px;
  max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

:deep(.search-highlight) {
  background: rgba(245,158,11,0.3); color: var(--text-primary); border-radius: 2px;
}

.line-detail {
  width: 100%; margin-top: 6px; padding-top: 6px;
  border-top: 1px solid var(--border-subtle);
}
.detail-grid { display: grid; grid-template-columns: auto 1fr; gap: 2px 8px; margin-bottom: 6px; }
.detail-row { display: contents; font-size: 11px; }
.detail-key { color: var(--text-muted); white-space: nowrap; }
.detail-val { color: var(--text-primary); font-family: var(--font-mono); word-break: break-all; }
.detail-raw {
  margin: 0; padding: 6px 8px; font-family: var(--font-mono); font-size: 10px;
  line-height: 1.4; white-space: pre-wrap; word-break: break-all;
  background: var(--surface-hover); border-radius: var(--radius-xs);
  color: var(--text-secondary); max-height: 150px; overflow-y: auto;
}

[data-theme="dark"] .sys-log-line.level-error { background: rgba(248,113,113,0.06); }

/* Dark mode: component badges */
[data-theme="dark"] .comp-web       { background: rgba(34,211,238,0.18); color: #67e8f9; }
[data-theme="dark"] .comp-dns       { background: rgba(129,140,248,0.15); color: #a5b4fc; }
[data-theme="dark"] .comp-ai        { background: rgba(196,181,253,0.12); color: #c4b5fd; }
[data-theme="dark"] .comp-database,
[data-theme="dark"] .comp-db        { background: rgba(52,211,153,0.15); color: #6ee7b7; }
[data-theme="dark"] .comp-scheduler { background: rgba(251,191,36,0.15); color: #fcd34d; }
[data-theme="dark"] .comp-bot       { background: rgba(96,165,250,0.15); color: #93c5fd; }
[data-theme="dark"] .comp-executor  { background: rgba(248,113,113,0.15); color: #fca5a5; }

/* Dark mode: level badges */
[data-theme="dark"] .lvl-error { background: rgba(248,113,113,0.18); color: #fca5a5; }
[data-theme="dark"] .lvl-warn  { background: rgba(251,191,36,0.18); color: #fcd34d; }
[data-theme="dark"] .lvl-info  { background: rgba(34,211,238,0.15);  color: #67e8f9; }
[data-theme="dark"] .lvl-debug { background: rgba(148,163,184,0.15); color: #94a3b8; }
</style>
