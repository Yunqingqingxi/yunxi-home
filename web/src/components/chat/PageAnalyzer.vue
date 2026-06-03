<template>
  <div class="page-analyzer">
    <!-- Header -->
    <div class="analyzer-header">
      <svg class="analyzer-head-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4 2c-1.5-1-4-2.5-4-5.5 0-2 1.5-3 3-3.5" />
        <path d="M14.5 22A2.5 2.5 0 0 1 12 19.5v-15a2.5 2.5 0 0 1 4-2c1.5 1 4 2.5 4 5.5 0 2-1.5 3-3 3.5" />
      </svg>
      <span>页面分析</span>
      <span v-if="streaming" class="analyzer-spinner">
        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="8" cy="8" r="6" stroke-opacity="0.2" />
          <path d="M8 2a6 6 0 0 1 6 6" stroke-linecap="round"><animateTransform attributeName="transform" type="rotate" from="0 8 8" to="360 8 8" dur="0.8s" repeatCount="indefinite"/></path>
        </svg>
      </span>
    </div>

    <!-- Page context + snapshot (always shown after loaded) -->
    <div class="analyzer-context">
      <div class="context-page">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <circle cx="12" cy="12" r="3" /><path d="M12 2v4M12 18v4M2 12h4M18 12h4" />
        </svg>
        <span>{{ pageDisplayName }}</span>
      </div>
      <!-- Snapshot data -->
      <div v-if="snapshotData.length > 0 && phase !== 'connecting'" class="snapshot-strip">
        <span v-for="item in snapshotData" :key="item.label" class="snapshot-item">
          {{ item.label }} <strong :style="{ color: item.color }">{{ item.value }}</strong>
        </span>
      </div>
    </div>

    <!-- Streaming / result area -->
    <div class="analyzer-result" ref="resultEl">
      <!-- Loading state -->
      <div v-if="phase === 'connecting'" class="result-connecting">
        <svg width="24" height="24" viewBox="0 0 32 32" fill="none" stroke="var(--brand-500)" stroke-width="2.5">
          <circle cx="16" cy="16" r="13" stroke-opacity="0.12" />
          <path d="M16 3a13 13 0 0 1 13 13" stroke-linecap="round"><animateTransform attributeName="transform" type="rotate" from="0 16 16" to="360 16 16" dur="0.9s" repeatCount="indefinite"/></path>
        </svg>
        <span>正在连接 AI 服务...</span>
      </div>

      <!-- Streaming content -->
      <div v-if="phase === 'streaming' || phase === 'done'" class="result-content">
        <div class="result-stream" v-html="renderedContent"></div>
        <span v-if="phase === 'streaming'" class="stream-cursor">|</span>
      </div>

      <!-- Error -->
      <div v-if="phase === 'error'" class="result-error">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#dc2626" stroke-width="2" stroke-linecap="round">
          <circle cx="12" cy="12" r="10" /><line x1="15" y1="9" x2="9" y2="15" /><line x1="9" y1="9" x2="15" y2="15" />
        </svg>
        <span>{{ errorMsg }}</span>
        <button class="retry-btn" @click="startAnalysis">重试</button>
      </div>
    </div>

    <!-- Actions after done -->
    <div v-if="phase === 'done'" class="analyzer-actions">
      <button class="action-btn primary" @click="continueChat">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        继续对话
      </button>
      <button class="action-btn" @click="refreshAnalysis">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
        重新分析
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { renderMarkdown } from '../../stores/chat'

const props = defineProps<{
  pageName: string
  visible: boolean
}>()

const emit = defineEmits<{ analyze: [text: string]; 'go-chat': [] }>()

type Phase = 'connecting' | 'streaming' | 'done' | 'error'

const phase = ref<Phase>('connecting')
const streaming = ref(false)
const rawContent = ref('')
const errorMsg = ref('')
const sessionId = ref('')
const resultEl = ref<HTMLElement | null>(null)
let abortCtrl: AbortController | null = null

// ── Cache ──
const CACHE_KEY_PREFIX = 'yunxi_page_analyzer_v2_'
const CACHE_TTL = 120000 // 2 min

interface CacheEntry {
  content: string
  sessionId: string
  timestamp: number
}

function getCacheKey(): string { return CACHE_KEY_PREFIX + props.pageName }
function getCached(): CacheEntry | null {
  try {
    const raw = sessionStorage.getItem(getCacheKey())
    if (!raw) return null
    const entry = JSON.parse(raw) as CacheEntry
    if (Date.now() - entry.timestamp > CACHE_TTL) { sessionStorage.removeItem(getCacheKey()); return null }
    return entry
  } catch { return null }
}
function setCache(content: string, sid: string) {
  try { sessionStorage.setItem(getCacheKey(), JSON.stringify({ content, sessionId: sid, timestamp: Date.now() })) } catch {}
}

// ── Page info ──
const pageDisplayName = computed(() => {
  const names: Record<string, string> = {
    dashboard: '仪表盘', files: '文件管理', domains: 'DNS 管理',
    market: '技能市场', logs: '日志', system: '系统控制', settings: '设置',
  }
  return names[props.pageName] || '当前页面'
})

const pageDescription = computed(() => {
  const descs: Record<string, string> = {
    dashboard: '系统状态总览面板，展示 CPU/内存/磁盘使用率、AI 用量统计（请求数、Token、费用）、DNS 更新状态、通知摘要。用户可在此页面监控服务器健康状态。',
    files: 'NAS 文件管理器，支持浏览、上传、下载、搜索文件。显示磁盘使用情况。用户可在此管理服务器上的文件资源。',
    domains: 'DNS 域名管理面板，展示所有配置的 DNS 记录及其解析状态。用户可查看和更新 DNS 记录，监控域名解析健康度。',
    logs: '日志查看面板，分为会话日志和系统日志两个视图。会话日志按轮次展示 AI 对话事件（工具调用、LLM 调用、错误等），系统日志支持按级别和组件筛选。',
    market: 'AI 技能市场，展示可安装的 MCP 工具和自定义技能。用户可浏览、安装和管理 AI 扩展能力。',
    system: '系统控制面板，管理 Docker 容器、系统服务启停、进程监控。用户可在此执行运维操作。',
    settings: '系统配置面板，包含 AI 提供商（模型、API Key）、DNS、通知渠道（邮件、Webhook）、NAS 等模块的参数设置。',
  }
  return descs[props.pageName] || '页面功能面板'
})

const renderedContent = computed(() => {
  if (!rawContent.value) return ''
  try { return renderMarkdown(rawContent.value) } catch { return rawContent.value }
})

// ── Snapshot data collected at click time ──
const snapshotData = ref<CollectedItem[]>([])

interface CollectedItem { label: string; value: string; color?: string }

function collectSnapshot(d: any): CollectedItem[] {
  if (!d) return []
  const sys = d.system
  const ai = d.ai
  const items: CollectedItem[] = []

  switch (props.pageName) {
    case 'dashboard':
      if (sys?.cpu_usage != null) items.push({ label: 'CPU 使用率', value: sys.cpu_usage.toFixed(1) + '%', color: cpuColor(sys.cpu_usage) })
      if (sys?.mem_percent != null) items.push({ label: '内存使用率', value: sys.mem_percent.toFixed(1) + '%', color: memColor(sys.mem_percent) })
      if (sys?.disk_percent != null) items.push({ label: '磁盘使用率', value: sys.disk_percent.toFixed(1) + '%', color: diskColor(sys.disk_percent) })
      if (ai?.requests != null) items.push({ label: 'AI 总请求', value: fmtNum(ai.requests) })
      if (ai?.errors != null && ai?.requests > 0) items.push({ label: 'AI 错误率', value: (ai.errors / ai.requests * 100).toFixed(1) + '%', color: errColor(ai) })
      if (ai?.input_tokens != null) items.push({ label: 'AI Token 用量', value: fmtNum((ai.input_tokens || 0) + (ai.output_tokens || 0)) })
      if (ai?.tool_calls != null) items.push({ label: '工具调用次数', value: String(ai.tool_calls) })
      if (d.scheduler?.status) items.push({ label: 'DNS 调度器', value: d.scheduler.status })
      break
    case 'logs':
      if (ai?.requests != null) items.push({ label: 'AI 请求', value: fmtNum(ai.requests) })
      if (ai?.errors != null) items.push({ label: 'AI 错误数', value: String(ai.errors), color: ai.errors > 0 ? '#dc2626' : undefined })
      if (ai?.tool_calls != null) items.push({ label: '工具调用', value: String(ai.tool_calls) })
      if (ai?.input_tokens != null) items.push({ label: 'Token 用量', value: fmtNum((ai.input_tokens || 0) + (ai.output_tokens || 0)) })
      break
    case 'system':
      if (sys?.cpu_usage != null) items.push({ label: 'CPU 使用率', value: sys.cpu_usage.toFixed(1) + '%', color: cpuColor(sys.cpu_usage) })
      if (sys?.mem_percent != null) items.push({ label: '内存使用率', value: sys.mem_percent.toFixed(1) + '%', color: memColor(sys.mem_percent) })
      if (d.goroutines != null) items.push({ label: 'Goroutines', value: String(d.goroutines) })
      if (d.uptime) items.push({ label: '运行时长', value: d.uptime })
      break
    case 'files':
      if (sys?.disk_percent != null) items.push({ label: '磁盘使用率', value: sys.disk_percent.toFixed(1) + '%', color: diskColor(sys.disk_percent) })
      if (sys?.cpu_usage != null) items.push({ label: 'CPU 使用率', value: sys.cpu_usage.toFixed(1) + '%', color: cpuColor(sys.cpu_usage) })
      if (sys?.mem_percent != null) items.push({ label: '内存使用率', value: sys.mem_percent.toFixed(1) + '%', color: memColor(sys.mem_percent) })
      break
    default:
      if (sys?.cpu_usage != null) items.push({ label: 'CPU 使用率', value: sys.cpu_usage.toFixed(1) + '%', color: cpuColor(sys.cpu_usage) })
      if (sys?.mem_percent != null) items.push({ label: '内存使用率', value: sys.mem_percent.toFixed(1) + '%', color: memColor(sys.mem_percent) })
      break
  }
  return items
}

function cpuColor(v: number) { return v > 80 ? '#dc2626' : v > 50 ? '#d97706' : '#16a34a' }
function memColor(v: number) { return v > 85 ? '#dc2626' : v > 60 ? '#d97706' : '#16a34a' }
function diskColor(v: number) { return v > 85 ? '#dc2626' : v > 70 ? '#d97706' : '#16a34a' }
function errColor(ai: any) { const r = ai.requests ? (ai.errors / ai.requests * 100) : 0; return r > 10 ? '#dc2626' : r > 3 ? '#d97706' : undefined }
function fmtNum(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

// ── Build context prompt with frozen snapshot ──
function buildContext(items: CollectedItem[]): string {
  let ctx = `[页面快照分析请求]
点击分析时用户正在浏览「${pageDisplayName.value}」页面。
页面功能: ${pageDescription.value}`

  if (items.length > 0) {
    ctx += '\n\n点击时的实时数据快照:'
    for (const item of items) {
      ctx += `\n- ${item.label}: ${item.value}`
    }
  }

  ctx += '\n\n请基于以上页面功能描述和此刻的实时数据，分析当前系统状态、指出值得关注的问题、并给出实用的优化建议。用简洁的要点形式呈现。'
  return ctx
}

// ── Main analysis flow ──
async function startAnalysis() {
  // Clean up previous
  if (abortCtrl) { abortCtrl.abort(); abortCtrl = null }
  rawContent.value = ''
  errorMsg.value = ''

  // Check cache
  const cached = getCached()
  if (cached) {
    rawContent.value = cached.content
    sessionId.value = cached.sessionId
    phase.value = 'done'
    streaming.value = false
    return
  }

  phase.value = 'connecting'

  const token = localStorage.getItem('token') || ''
  abortCtrl = new AbortController()
  let streamTimeout: ReturnType<typeof setTimeout> | null = null

  try {
    // 1. Take snapshot of current page data
    let items: CollectedItem[] = []
    try {
      const snapRes = await fetch('/api/status', {
        headers: { 'Authorization': 'Bearer ' + token },
        signal: abortCtrl.signal,
      })
      if (snapRes.ok) {
        const snapData = await snapRes.json()
        const d = snapData.data || snapData
        items = collectSnapshot(d)
        snapshotData.value = items
      }
    } catch { /* snapshot failed, continue without data */ }

    // 2. Build context with frozen snapshot and send to AI
    const context = buildContext(items)

    const res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
      body: JSON.stringify({ message: context, session_id: '' }),
      signal: abortCtrl.signal,
    })

    if (!res.ok) throw new Error('AI 服务返回错误: ' + res.status)

    // Check if response is SSE or JSON error
    const contentType = res.headers.get('content-type') || ''
    if (contentType.includes('application/json') || !contentType.includes('text/event-stream')) {
      // JSON response — likely AI not configured
      const body = await res.text()
      try {
        const json = JSON.parse(body)
        if (json.hint === 'ai_not_configured' || json.data?.hint === 'ai_not_configured') {
          errorMsg.value = 'AI 助手尚未配置，请在设置中启用 AI 服务'
        } else {
          errorMsg.value = (json.data?.message || json.message) || 'AI 服务未就绪'
        }
      } catch { errorMsg.value = body || 'AI 服务返回了非预期的响应格式' }
      phase.value = 'error'
      return
    }

    // SSE stream with timeout guard
    phase.value = 'streaming'
    streaming.value = true

    // 10s timeout for the entire analysis
    const streamTimeout = setTimeout(() => {
      if (abortCtrl) abortCtrl.abort()
      if (phase.value === 'streaming' || phase.value === 'connecting') {
        if (!rawContent.value) {
          errorMsg.value = 'AI 响应超时，请检查网络或 AI 服务状态'
          phase.value = 'error'
        } else {
          phase.value = 'done'
        }
      }
      streaming.value = false
    }, 30000)

    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data) continue
        try {
          const ev = JSON.parse(data)
          if (ev.type === 'content' && ev.content) {
            rawContent.value += ev.content
            nextTick(() => {
              if (resultEl.value) resultEl.value.scrollTop = resultEl.value.scrollHeight
            })
          } else if (ev.type === 'session_status' && ev.session_id) {
            sessionId.value = ev.session_id
          } else if (ev.type === 'done') {
            // done
          } else if (ev.type === 'error') {
            errorMsg.value = ev.content || 'AI 分析出错'
            phase.value = 'error'
            streaming.value = false
            return
          }
        } catch { /* skip malformed */ }
      }
    }

    // Complete
    clearTimeout(streamTimeout)
    if (rawContent.value) {
      setCache(rawContent.value, sessionId.value)
    }
    phase.value = 'done'
    streaming.value = false
  } catch (e: any) {
    clearTimeout(streamTimeout!)
    if (e.name === 'AbortError') return
    errorMsg.value = e.message || '网络请求失败'
    phase.value = 'error'
    streaming.value = false
  }
}

function continueChat() {
  if (sessionId.value) {
    emit('go-chat')
    setTimeout(() => {
      window.location.hash = '#/chat/' + sessionId.value
    }, 50)
  } else {
    emit('go-chat')
  }
}

function refreshAnalysis() {
  sessionStorage.removeItem(getCacheKey())
  rawContent.value = ''
  startAnalysis()
}

// Watch visible
watch(() => props.visible, (v) => {
  if (v) startAnalysis()
  else {
    if (abortCtrl) { abortCtrl.abort(); abortCtrl = null }
    streaming.value = false
  }
})
</script>

<style scoped>
.page-analyzer { display: flex; flex-direction: column; }

/* Header */
.analyzer-header {
  display: flex; align-items: center; gap: 7px;
  padding: 10px 14px; border-bottom: 1px solid var(--border-subtle);
  color: var(--text-primary); font-size: 13px; font-weight: 600; flex-shrink: 0;
}
.analyzer-head-icon { color: var(--brand-500); flex-shrink: 0; }
.analyzer-spinner { margin-left: auto; color: var(--brand-500); display: flex; align-items: center; }

/* Context */
.analyzer-context {
  padding: 8px 14px; border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-hover); flex-shrink: 0;
}
.context-page {
  display: flex; align-items: center; gap: 5px;
  font-size: 11.5px; font-weight: 600; color: var(--brand-500);
}
.context-desc { font-size: 10.5px; color: var(--text-muted); margin-top: 2px; line-height: 1.35; }
.snapshot-strip {
  display: flex; flex-wrap: wrap; gap: 4px 8px; margin-top: 5px;
  font-size: 10px; color: var(--text-muted);
}
.snapshot-item strong {
  font-weight: 600; font-family: var(--font-mono); font-size: 10px;
}

/* Result area */
.analyzer-result {
  flex: 1; min-height: 120px; max-height: 360px; overflow-y: auto;
  padding: 10px 14px;
}

/* Connecting */
.result-connecting {
  display: flex; flex-direction: column; align-items: center; gap: 10px;
  padding: 30px 0; color: var(--text-muted); font-size: 12px;
}

/* Streaming content */
.result-content { font-size: 12.5px; line-height: 1.65; color: var(--text-primary); }
.result-stream :deep(p) { margin: 0 0 6px; }
.result-stream :deep(ul), .result-stream :deep(ol) { margin: 4px 0; padding-left: 18px; }
.result-stream :deep(li) { margin: 2px 0; }
.result-stream :deep(strong) { color: var(--brand-600); }
.result-stream :deep(code) {
  font-family: var(--font-mono); font-size: 11px;
  background: var(--code-bg); color: var(--code-color);
  padding: 1px 5px; border-radius: 3px;
}
.stream-cursor {
  display: inline-block; color: var(--brand-500); font-weight: 300;
  animation: blink 0.8s step-end infinite;
}
@keyframes blink { 0%,100% { opacity: 1; } 50% { opacity: 0; } }

/* Error */
.result-error {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 20px 0; font-size: 12px; color: var(--text-secondary);
}
.retry-btn {
  padding: 5px 16px; border: 1px solid var(--border-default); border-radius: 5px;
  background: transparent; color: var(--text-primary); font-family: inherit;
  font-size: 11px; cursor: pointer;
}
.retry-btn:hover { background: var(--surface-hover); }

/* Actions */
.analyzer-actions {
  display: flex; gap: 8px; padding: 8px 14px 10px;
  border-top: 1px solid var(--border-subtle); flex-shrink: 0;
}
.action-btn {
  flex: 1; display: flex; align-items: center; justify-content: center; gap: 5px;
  padding: 7px; border: 1px solid var(--border-default); border-radius: var(--radius-sm);
  background: transparent; color: var(--text-secondary); font-family: inherit;
  font-size: 12px; cursor: pointer; transition: all 0.12s;
}
.action-btn:hover { background: var(--surface-hover); color: var(--text-primary); }
.action-btn.primary {
  background: var(--gradient-brand); color: #fff; border-color: transparent;
  font-weight: 500;
}
.action-btn.primary:hover { background: var(--gradient-brand-hover); }
</style>
