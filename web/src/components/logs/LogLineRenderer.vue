<template>
  <div :class="['log-line-renderer', parsed.level ? `lvl-${parsed.level.toLowerCase()}` : '']">
    <span v-if="parsed.timestamp" class="lr-time">{{ parsed.timestamp }}</span>
    <span v-if="parsed.component" :class="['lr-comp', `comp-${parsed.component.toLowerCase()}`]">
      {{ parsed.component }}
    </span>
    <span v-if="parsed.level" :class="['lr-level', `lvl-${parsed.level.toLowerCase()}`]">
      {{ parsed.level }}
    </span>
    <span class="lr-msg" v-html="highlightedMsg"></span>
    <span v-if="showFields && hasExtraFields" class="lr-fields">
      <span
        v-for="(v, k) in extraFields"
        :key="k"
        class="lr-field"
        :title="`${k}=${v}`"
      >{{ k }}={{ v }}</span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { parseLogLine } from '../../stores/logs'

const props = withDefaults(defineProps<{
  line: string
  search?: string
  showFields?: boolean
}>(), {
  search: '',
  showFields: true,
})

const parsed = computed(() => parseLogLine(props.line))

const SYSTEM_KEYS = new Set(['time', 'level', 'msg', 'component'])

const extraFields = computed(() => {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(parsed.value.fields)) {
    if (!SYSTEM_KEYS.has(k)) out[k] = v
  }
  return out
})

const hasExtraFields = computed(() => Object.keys(extraFields.value).length > 0)

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
.log-line-renderer {
  display: flex; gap: 6px; align-items: baseline; flex-wrap: wrap;
  font-family: var(--font-mono); font-size: 11px; line-height: 1.6;
  padding: 3px 10px; margin: 1px 0; border-radius: 3px;
  border-left: 2px solid transparent;
  transition: background 0.1s;
}
.log-line-renderer:hover { background: var(--surface-hover); }

.log-line-renderer.lvl-error {
  border-left-color: var(--color-danger);
  background: rgba(220,38,38,0.03);
}
.log-line-renderer.lvl-warn { border-left-color: #d97706; }
.log-line-renderer.lvl-debug { opacity: 0.55; }

/* ── Timestamp ──────────────────────────────── */
.lr-time {
  color: var(--text-muted); white-space: nowrap; min-width: 78px;
  font-size: 10.5px; user-select: none;
}

/* ── Component badge ────────────────────────── */
.lr-comp {
  font-weight: 600; font-size: 9.5px; padding: 1px 6px; border-radius: 3px;
  white-space: nowrap; letter-spacing: 0.3px;
  transition: opacity 0.15s, transform 0.1s;
}
.lr-comp:hover { opacity: 0.85; transform: scale(1.05); }

.comp-web       { background: rgba(6,182,212,0.12); color: #0891b2; }
.comp-dns       { background: rgba(99,102,241,0.1);  color: #6366f1; }
.comp-ai        { background: rgba(168,85,247,0.1);  color: #8b5cf6; }
.comp-database, .comp-db { background: rgba(16,185,129,0.1); color: #10b981; }
.comp-scheduler { background: rgba(245,158,11,0.1); color: #d97706; }
.comp-bot       { background: rgba(59,130,246,0.1); color: #3b82f6; }
.comp-executor  { background: rgba(239,68,68,0.1);  color: #dc2626; }

/* ── Level badge ────────────────────────────── */
.lr-level {
  font-weight: 700; font-size: 9.5px; padding: 1px 6px; border-radius: 3px;
  white-space: nowrap; letter-spacing: 0.3px;
  transition: opacity 0.15s, transform 0.1s;
}
.lr-level:hover { opacity: 0.85; transform: scale(1.05); }

.lvl-error { background: rgba(220,38,38,0.12);  color: #dc2626; }
.lvl-warn  { background: rgba(217,119,6,0.12);  color: #d97706; }
.lvl-info  { background: rgba(6,182,212,0.1);   color: #0891b2; }
.lvl-debug { background: rgba(148,163,184,0.1); color: #94a3b8; }

/* ── Message ────────────────────────────────── */
.lr-msg {
  color: var(--text-primary); word-break: break-word; flex: 1; min-width: 120px;
}

/* ── Extra fields ───────────────────────────── */
.lr-fields { display: flex; gap: 4px; flex-wrap: wrap; }
.lr-field {
  font-size: 9px; color: var(--text-muted);
  background: var(--border-subtle); padding: 0 5px; border-radius: 3px;
  max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

/* ── Search highlight ───────────────────────── */
:deep(.search-highlight) {
  background: rgba(245,158,11,0.3); color: var(--text-primary); border-radius: 2px;
}

/* ── Dark mode ──────────────────────────────── */
[data-theme="dark"] .log-line-renderer.lvl-error { background: rgba(248,113,113,0.06); }

[data-theme="dark"] .comp-web       { background: rgba(34,211,238,0.18); color: #67e8f9; }
[data-theme="dark"] .comp-dns       { background: rgba(129,140,248,0.15); color: #a5b4fc; }
[data-theme="dark"] .comp-ai        { background: rgba(196,181,253,0.12); color: #c4b5fd; }
[data-theme="dark"] .comp-database,
[data-theme="dark"] .comp-db        { background: rgba(52,211,153,0.15); color: #6ee7b7; }
[data-theme="dark"] .comp-scheduler { background: rgba(251,191,36,0.15); color: #fcd34d; }
[data-theme="dark"] .comp-bot       { background: rgba(96,165,250,0.15); color: #93c5fd; }
[data-theme="dark"] .comp-executor  { background: rgba(248,113,113,0.15); color: #fca5a5; }

[data-theme="dark"] .lvl-error { background: rgba(248,113,113,0.18); color: #fca5a5; }
[data-theme="dark"] .lvl-warn  { background: rgba(251,191,36,0.18); color: #fcd34d; }
[data-theme="dark"] .lvl-info  { background: rgba(34,211,238,0.15);  color: #67e8f9; }
[data-theme="dark"] .lvl-debug { background: rgba(148,163,184,0.15); color: #94a3b8; }
</style>
