// ── 日志系统 Pinia Store ────────────────────────────
import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import api from '../services/api'
import type {
  SessionInfo, LogEvent, ChatLogResponse, EventSummary,
  SysFileInfo, AnalyticsSummary, ToolStat, DailySummary,
  SSEStatus, ChatViewMode, LogLevel, EventType,
  SysLogFilter, ChatLogFilter, ParsedLogLine,
} from '../types/logs'

export const useLogsStore = defineStore('logs', () => {
  // ── 分析概览 ──────────────────────────────────────
  const analytics = ref<AnalyticsSummary>({
    total_requests: 0, total_errors: 0, active_sessions: 0,
    total_tokens_in: 0, total_tokens_out: 0, total_cost_usd: 0,
    error_rate: 0,
  })
  const toolStats = ref<ToolStat[]>([])
  const dailySummaries = ref<DailySummary[]>([])
  const analyticsLoading = ref(false)

  // ── 会话日志 ──────────────────────────────────────
  const chatSessions = ref<SessionInfo[]>([])
  const chatSessionsLoading = ref(false)
  const selectedSessionId = ref<string | null>(null)
  const chatEvents = ref<LogEvent[]>([])
  const chatEventsLoading = ref(false)
  const chatEventSummary = ref<EventSummary | null>(null)
  const chatTotal = ref(0)
  const chatOffset = ref(0)
  const chatHasMore = ref(false)
  const chatViewMode = ref<ChatViewMode>('timeline')
  const chatLogText = ref('')
  const chatLogTextLoading = ref(false)
  const chatJSONExpanded = ref<Set<number>>(new Set())

  // 会话筛选
  const chatFilter = ref<ChatLogFilter>({
    eventTypes: new Set(),
    search: '',
    order: 'desc',
    roundFilter: null,
    errorsOnly: false,
  })

  // SSE
  const sseStatus = ref<SSEStatus>('disconnected')
  let sseSource: EventSource | null = null

  // ── 系统日志 ──────────────────────────────────────
  const sysFiles = ref<SysFileInfo[]>([])
  const sysFilesLoading = ref(false)
  const selectedSysDate = ref<string | null>(null)
  const sysLines = ref<string[]>([])
  const sysLoading = ref(false)
  const sysOffset = ref(0)
  const sysHasMore = ref(false)
  const sysLiveTail = ref(false)
  let sysPollTimer: ReturnType<typeof setInterval> | null = null

  // 系统筛选
  const sysFilter = ref<SysLogFilter>({
    levels: new Set(['ERROR', 'WARN', 'INFO', 'DEBUG']),
    components: new Set(),
    search: '',
    order: 'desc',
  })

  // ── Checkbox 辅助（响应式绑定用） ────────────────
  const chatEventChecks = ref<Record<string, boolean>>({})
  const sysLevelChecks = ref<Record<string, boolean>>({ ERROR: true, WARN: true, INFO: true, DEBUG: true })
  const sysComponentChecks = ref<Record<string, boolean>>({})

  // ── 计算属性 ──────────────────────────────────────
  const filteredChatEvents = computed(() => {
    let events = chatEvents.value
    const f = chatFilter.value

    if (f.eventTypes.size > 0) {
      events = events.filter(e => f.eventTypes.has(e.type))
    }
    if (f.search) {
      const lower = f.search.toLowerCase()
      events = events.filter(e =>
        (e.content && e.content.toLowerCase().includes(lower)) ||
        (e.tool_name && e.tool_name.toLowerCase().includes(lower)) ||
        (e.error && e.error.toLowerCase().includes(lower)) ||
        (e.tool_result && e.tool_result.toLowerCase().includes(lower))
      )
    }
    if (f.roundFilter !== null) {
      events = events.filter(e => e.round === f.roundFilter)
    }
    if (f.errorsOnly) {
      events = events.filter(e => e.type === 'error')
    }
    return events
  })

  const groupedEvents = computed(() => {
    const groups: { round: number; events: LogEvent[] }[] = []
    let current: { round: number; events: LogEvent[] } | null = null
    const f = chatFilter.value

    let sorted = [...chatEvents.value]
    if (f.order === 'desc') {
      sorted = sorted.reverse()
    }

    for (const ev of sorted) {
      const r = ev.round || 0
      if (!current || current.round !== r) {
        if (current) groups.push(current)
        current = { round: r, events: [] }
      }
      current.events.push(ev)
    }
    if (current) groups.push(current)
    return groups
  })

  const filteredSysLines = computed(() => {
    let lines = sysLines.value
    const f = sysFilter.value

    const activeLevels = f.levels
    if (activeLevels.size > 0 && activeLevels.size < 4) {
      lines = lines.filter(l => {
        const level = parseLineLevel(l)
        return level ? activeLevels.has(level as LogLevel) : true
      })
    }
    if (f.components.size > 0) {
      lines = lines.filter(l => {
        const comp = parseLineComponent(l)
        return comp ? f.components.has(comp) : true
      })
    }
    if (f.search) {
      const lower = f.search.toLowerCase()
      lines = lines.filter(l => l.toLowerCase().includes(lower))
    }
    return lines
  })

  // ── 会话列表 ──────────────────────────────────────
  async function fetchSessions() {
    chatSessionsLoading.value = true
    try {
      const res = await api.get('/api/logs/chat/sessions')
      chatSessions.value = res.data?.data?.sessions || []
    } catch {
      chatSessions.value = []
    } finally {
      chatSessionsLoading.value = false
    }
  }

  async function selectSession(sessionId: string) {
    disconnectSSE()
    selectedSessionId.value = sessionId
    chatOffset.value = 0
    chatEvents.value = []
    chatLogText.value = ''
    chatEventSummary.value = null
    chatJSONExpanded.value = new Set()

    await _fetchEvents(0)
  }

  async function _fetchEvents(offset: number) {
    chatEventsLoading.value = true
    try {
      const f = chatFilter.value
      const params: Record<string, unknown> = { offset, limit: 500, order: f.order }
      if (f.eventTypes.size > 0) params.type = [...f.eventTypes].join(',')
      if (f.search) params.search = f.search

      const res = await api.get(`/api/logs/chat/${selectedSessionId.value}`, { params })
      const data: ChatLogResponse = res.data?.data || res.data

      if (offset === 0) {
        chatEvents.value = data.events || []
      } else {
        chatEvents.value.push(...(data.events || []))
      }
      chatTotal.value = data.total
      chatEventSummary.value = data.summary || null
      chatOffset.value = offset + (data.events?.length || 0)
      chatHasMore.value = chatOffset.value < data.total
    } catch {
      if (offset === 0) chatEvents.value = []
    } finally {
      chatEventsLoading.value = false
    }
  }

  function loadMoreEvents() {
    if (!chatHasMore.value || chatEventsLoading.value) return
    _fetchEvents(chatOffset.value)
  }

  // ── 会话日志文本视图 ──────────────────────────────
  async function fetchChatText() {
    if (!selectedSessionId.value) return
    chatLogTextLoading.value = true
    try {
      const res = await api.get(`/api/logs/chat/${selectedSessionId.value}/text`)
      chatLogText.value = typeof res.data === 'string' ? res.data : (res.data?.data || '')
    } catch {
      chatLogText.value = '(无法加载日志)'
    } finally {
      chatLogTextLoading.value = false
    }
  }

  // ── SSE 实时推送 ──────────────────────────────────
  function connectSSE() {
    if (!selectedSessionId.value || sseSource) return
    sseStatus.value = 'connecting'

    const token = localStorage.getItem('token')
    const url = `/api/logs/chat/${selectedSessionId.value}/tail?token=${encodeURIComponent(token || '')}`

    sseSource = new EventSource(url)
    sseSource.addEventListener('log', (evt: MessageEvent) => {
      try {
        const event: LogEvent = JSON.parse(evt.data)
        chatEvents.value.unshift(event)
        sseStatus.value = 'connected'
      } catch { /* skip malformed */ }
    })
    sseSource.addEventListener('error', () => {
      if (sseSource?.readyState === EventSource.CLOSED) {
        sseStatus.value = 'disconnected'
      } else {
        sseStatus.value = 'error'
      }
    })
    sseSource.onopen = () => { sseStatus.value = 'connected' }
  }

  function disconnectSSE() {
    if (sseSource) {
      sseSource.close()
      sseSource = null
    }
    sseStatus.value = 'disconnected'
  }

  // ── 删除会话日志 ──────────────────────────────────
  async function deleteChatLog(sessionId: string): Promise<string | null> {
    try {
      await api.delete(`/api/logs/chat/${sessionId}`)
      chatSessions.value = chatSessions.value.filter(s => s.session_id !== sessionId)
      if (selectedSessionId.value === sessionId) {
        selectedSessionId.value = null
        chatEvents.value = []
        chatLogText.value = ''
      }
      return null
    } catch (e: any) {
      return e.response?.status === 409
        ? '会话仍在运行，无法删除'
        : (e.response?.data?.message || '删除失败')
    }
  }

  // ── 下载会话日志 ──────────────────────────────────
  function downloadChatLog() {
    if (!selectedSessionId.value) return
    const token = localStorage.getItem('token')
    const a = document.createElement('a')
    a.href = `/api/logs/chat/${selectedSessionId.value}/download?token=${encodeURIComponent(token || '')}`
    a.click()
  }

  // ── 系统日志 ──────────────────────────────────────
  async function fetchSysFiles() {
    sysFilesLoading.value = true
    try {
      const res = await api.get('/api/logs/system', { params: { order: 'desc' } })
      sysFiles.value = (res.data?.data?.files || []).sort(
        (a: SysFileInfo, b: SysFileInfo) => (b.date || '').localeCompare(a.date || '')
      )
    } catch {
      sysFiles.value = []
    } finally {
      sysFilesLoading.value = false
    }
  }

  function sysLogParams(extra: Record<string, unknown> = {}) {
    const levelOn = [...sysFilter.value.levels]
    const p: Record<string, unknown> = { order: sysFilter.value.order, limit: 500, ...extra }
    if (sysFilter.value.search) p.search = sysFilter.value.search
    if (levelOn.length > 0 && levelOn.length < 4) p.level = levelOn.join(',')
    return p
  }

  async function selectSysDate(date: string) {
    stopSysPoll()
    selectedSysDate.value = date
    sysOffset.value = 0
    sysFilter.value.search = ''
    sysFilter.value.components.clear()
    sysComponentChecks.value = {}
    sysLoading.value = true
    try {
      const res = await api.get(`/api/logs/system/${date}`, { params: sysLogParams({ offset: 0 }) })
      sysLines.value = res.data?.data?.lines || []
      sysHasMore.value = res.data?.data?.has_more || false
      sysOffset.value = (res.data?.data?.lines || []).length
    } catch {
      sysLines.value = []
    } finally {
      sysLoading.value = false
    }
  }

  async function reloadSysLog() {
    if (!selectedSysDate.value) return
    sysOffset.value = 0
    sysLoading.value = true
    try {
      const res = await api.get(`/api/logs/system/${selectedSysDate.value}`, { params: sysLogParams({ offset: 0 }) })
      sysLines.value = res.data?.data?.lines || []
      sysHasMore.value = res.data?.data?.has_more || false
      sysOffset.value = (res.data?.data?.lines || []).length
    } catch {
      sysLines.value = []
    } finally {
      sysLoading.value = false
    }
  }

  async function loadMoreSysLog() {
    if (!selectedSysDate.value) return
    try {
      const res = await api.get(`/api/logs/system/${selectedSysDate.value}`, { params: sysLogParams({ offset: sysOffset.value }) })
      const newLines: string[] = res.data?.data?.lines || []
      sysLines.value.push(...newLines)
      sysHasMore.value = res.data?.data?.has_more || false
      sysOffset.value += newLines.length
    } catch { /* ignore */ }
  }

  function startSysPoll() {
    if (sysPollTimer) return
    sysLiveTail.value = true
    sysPollTimer = setInterval(pollSysTail, 5000)
    pollSysTail()
  }

  function stopSysPoll() {
    sysLiveTail.value = false
    if (sysPollTimer) { clearInterval(sysPollTimer); sysPollTimer = null }
  }

  async function pollSysTail() {
    if (!selectedSysDate.value) return
    try {
      const res = await api.get(`/api/logs/system/${selectedSysDate.value}`, { params: { tail: 200, order: sysFilter.value.order } })
      const tailLines: string[] = res.data?.data?.lines || []
      if (!tailLines.length) return
      const existingSet = new Set(sysLines.value)
      const newLines = tailLines.filter(l => !existingSet.has(l))
      if (newLines.length > 0) {
        sysLines.value = sysFilter.value.order === 'desc'
          ? [...newLines, ...sysLines.value]
          : [...sysLines.value, ...newLines]
      }
    } catch { /* ignore */ }
  }

  async function deleteSystemLog(date: string): Promise<string | null> {
    try {
      await api.delete(`/api/logs/system/${date}`)
      sysFiles.value = sysFiles.value.filter(f => f.date !== date)
      if (selectedSysDate.value === date) {
        selectedSysDate.value = null
        sysLines.value = []
        stopSysPoll()
      }
      return null
    } catch (e: any) {
      return e.response?.data?.message || '删除失败'
    }
  }

  function downloadSysLog() {
    if (!selectedSysDate.value) return
    const token = localStorage.getItem('token')
    const a = document.createElement('a')
    a.href = `/api/logs/system/${selectedSysDate.value}/download?token=${encodeURIComponent(token || '')}`
    a.click()
  }

  // ── 筛选同步 ──────────────────────────────────────

  /** 将 chatEventChecks 同步到 chatFilter.eventTypes */
  function syncChatEventFilter() {
    const types: Set<EventType> = new Set()
    for (const [key, on] of Object.entries(chatEventChecks.value)) {
      if (on) types.add(key as EventType)
    }
    chatFilter.value.eventTypes = types
  }

  /** 将 sysLevelChecks 同步到 sysFilter.levels */
  function syncSysLevelFilter() {
    const levels: Set<LogLevel> = new Set()
    for (const [key, on] of Object.entries(sysLevelChecks.value)) {
      if (on) levels.add(key as LogLevel)
    }
    sysFilter.value.levels = levels
  }

  /** 将 sysComponentChecks 同步到 sysFilter.components */
  function syncSysComponentFilter() {
    const comps: Set<string> = new Set()
    for (const [key, on] of Object.entries(sysComponentChecks.value)) {
      if (on) comps.add(key)
    }
    sysFilter.value.components = comps
  }

  /** 从当前系统日志行中提取组件列表 */
  function extractComponents() {
    const comps = new Set<string>()
    for (const line of sysLines.value) {
      const c = parseLineComponent(line)
      if (c) comps.add(c)
    }
    // 更新组件复选框（保留已有的勾选状态）
    for (const c of comps) {
      if (!(c in sysComponentChecks.value)) {
        sysComponentChecks.value[c] = true
      }
    }
    // 清理不存在的组件
    for (const key of Object.keys(sysComponentChecks.value)) {
      if (!comps.has(key)) delete sysComponentChecks.value[key]
    }
    syncSysComponentFilter()
  }

  // ── 分析数据 ──────────────────────────────────────
  async function fetchAnalytics() {
    analyticsLoading.value = true
    try {
      // 从 status 接口获取 AI 指标
      const res = await api.get('/api/status')
      const ai = res.data?.data?.ai
      if (ai) {
        const requests = ai.requests || 0
        const errors = ai.errors || 0
        analytics.value = {
          total_requests: requests,
          total_errors: errors,
          active_sessions: chatSessions.value.filter(s => s.active).length,
          total_tokens_in: ai.input_tokens || 0,
          total_tokens_out: ai.output_tokens || 0,
          total_cost_usd: ai.cost_usd || 0,
          error_rate: requests > 0 ? (errors / requests * 100) : 0,
        }
        // 工具统计
        const topTools: any[] = ai.top_tools || []
        toolStats.value = topTools.map((t: any) => ({
          tool_name: t.name || t.tool_name,
          calls: t.count || t.calls || 0,
          errors: 0,
          avg_lat_ms: t.avg_ms || t.avg_lat_ms || 0,
          max_lat_ms: 0,
        }))
      }
    } catch { /* ignore */ }
    finally { analyticsLoading.value = false }
  }

  return {
    // Analytics
    analytics, toolStats, dailySummaries, analyticsLoading,
    // Chat
    chatSessions, chatSessionsLoading, selectedSessionId,
    chatEvents, chatEventsLoading, chatEventSummary, chatTotal,
    chatOffset, chatHasMore, chatViewMode, chatLogText, chatLogTextLoading,
    chatJSONExpanded,
    chatFilter, chatEventChecks,
    sseStatus,
    // System
    sysFiles, sysFilesLoading, selectedSysDate,
    sysLines, sysLoading, sysOffset, sysHasMore, sysLiveTail,
    sysFilter, sysLevelChecks, sysComponentChecks,
    // Computed
    filteredChatEvents, groupedEvents, filteredSysLines,
    // Actions
    fetchSessions, selectSession, loadMoreEvents,
    fetchChatText,
    connectSSE, disconnectSSE,
    deleteChatLog, downloadChatLog,
    fetchSysFiles, selectSysDate, reloadSysLog, loadMoreSysLog,
    startSysPoll, stopSysPoll,
    deleteSystemLog, downloadSysLog,
    syncChatEventFilter, syncSysLevelFilter, syncSysComponentFilter,
    extractComponents,
    fetchAnalytics,
  }
})

// ── 行解析工具函数 ────────────────────────────────

export function parseLineLevel(line: string): string {
  const m = line.match(/level[=:]\s*(\w+)/i)
  return m ? m[1].toUpperCase() : ''
}

export function parseLineComponent(line: string): string {
  const m = line.match(/component[=:]\s*(\w+)/i)
  return m ? m[1] : ''
}

export function parseLineMessage(line: string): string {
  const m = line.match(/msg[=:]\s*"([^"]*)"/)
  return m ? m[1] : line
}

export function parseLineFields(line: string): Record<string, string> {
  const fields: Record<string, string> = {}
  // 匹配 key=value 或 key="value" 对
  const re = /(\w+)=("([^"]*)"|(\S+))/g
  let m: RegExpExecArray | null
  while ((m = re.exec(line)) !== null) {
    fields[m[1]] = m[3] !== undefined ? m[3] : m[4]
  }
  return fields
}

export function parseLineTimestamp(line: string): string {
  const m = line.match(/time[=:]\s*"([^"]*)"/)
  if (m) {
    const t = m[1]
    // 提取时:分:秒部分
    const timeMatch = t.match(/(\d{2}:\d{2}:\d{2})/)
    return timeMatch ? timeMatch[1] : t.slice(11, 19) || t
  }
  return ''
}

/** 一次解析所有子项，返回 ParsedLogLine */
export function parseLogLine(line: string): ParsedLogLine {
  return {
    raw: line,
    timestamp: parseLineTimestamp(line),
    level: (parseLineLevel(line) as LogLevel) || '',
    component: parseLineComponent(line),
    message: parseLineMessage(line),
    fields: parseLineFields(line),
  }
}

const S = (d: string) => `<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">${d}</svg>`

/** 通用图标 SVG，供各日志组件使用 */
export const ICONS: Record<string, string> = {
  error: S('<circle cx="8" cy="8" r="6"/><line x1="5.5" y1="5.5" x2="10.5" y2="10.5"/><line x1="10.5" y1="5.5" x2="5.5" y2="10.5"/>'),
  tool: S('<path d="M5.5 2L4 7l1.5 1L7 2zM10.5 2L12 7l-1.5 1L9 2z"/><circle cx="8" cy="11" r="3"/><line x1="8" y1="9" x2="8" y2="11"/>'),
  check: S('<circle cx="8" cy="8" r="6"/><path d="M5 8l2 2 4-4"/>'),
  brain: S('<circle cx="8" cy="5" r="3"/><path d="M2 14c0-3.3 2.7-6 6-6"/><path d="M10 9c2.2 0 4 1.8 4 4"/>'),
  strategy: S('<path d="M3 5h10v9a1.5 1.5 0 01-1.5 1.5h-7A1.5 1.5 0 013 14V5z"/><path d="M5 2h6v3H5z"/>'),
  disk: S('<path d="M3 5h10v6a2 2 0 01-2 2H5a2 2 0 01-2-2V5z"/><circle cx="8" cy="9" r="1.5"/>'),
  zap: S('<polygon points="8,2 10,7 15,8 10,9 8,14 6,9 2,8 6,7"/>'),
}

/** 事件类型的显示标签和颜色映射 */
export const EVENT_TYPE_CONFIG: Record<string, { label: string; color: string; icon: string }> = {
  session_start:  { label: '会话开始', color: '#64748b', icon: S('<polygon points="5,3 14,8 5,13"/>') },
  session_end:    { label: '会话结束', color: '#64748b', icon: S('<rect x="3" y="3" width="10" height="10" rx="1"/>') },
  session_save:   { label: '会话保存', color: '#64748b', icon: S('<path d="M3 14v-3l4-4 2 2 4-4v9"/><path d="M3 2h10v6"/>') },
  round_start:    { label: '轮次开始', color: '#94a3b8', icon: S('<polygon points="6,4 12,8 6,12"/>') },
  round_end:      { label: '轮次结束', color: '#94a3b8', icon: S('<polygon points="10,4 4,8 10,12"/>') },
  user_message:   { label: '用户消息', color: '#3b82f6', icon: S('<circle cx="8" cy="5" r="3"/><path d="M3 14c0-2.8 2.2-5 5-5s5 2.2 5 5"/>') },
  inject:         { label: '系统注入', color: '#8b5cf6', icon: S('<line x1="8" y1="2" x2="8" y2="14"/><path d="M3 5l5-3 5 3"/><line x1="5" y1="8" x2="11" y2="8"/>') },
  thinking:       { label: '思考',    color: '#6366f1', icon: S('<circle cx="5" cy="8" r="1.5"/><circle cx="11" cy="8" r="1.5"/><circle cx="8" cy="11" r="1.5"/>') },
  content:        { label: '内容',    color: '#06b6d4', icon: S('<path d="M3 5h10M3 8h8M3 11h6"/>') },
  answer:         { label: 'AI 回复', color: '#06b6d4', icon: S('<rect x="2" y="3" width="12" height="10" rx="2"/><circle cx="6" cy="7" r="1"/><circle cx="10" cy="7" r="1"/><path d="M5 10h6"/>') },
  tool_call:      { label: '工具调用', color: '#10b981', icon: S('<path d="M5.5 2L4 7l1.5 1L7 2zM10.5 2L12 7l-1.5 1L9 2z"/><circle cx="8" cy="11" r="3"/><line x1="8" y1="9" x2="8" y2="11"/>') },
  tool_start:     { label: '工具开始', color: '#10b981', icon: S('<polygon points="8,2 10,7 15,8 10,9 8,14 6,9 2,8 6,7"/>') },
  tool_progress:  { label: '工具进度', color: '#10b981', icon: S('<circle cx="8" cy="8" r="6"/><path d="M8 3v5l3 2"/>') },
  tool_result:    { label: '工具结果', color: '#10b981', icon: S('<circle cx="8" cy="8" r="6"/><path d="M5 8l2 2 4-4"/>') },
  strategy:       { label: '策略变更', color: '#f59e0b', icon: S('<path d="M3 5h10v9a1.5 1.5 0 01-1.5 1.5h-7A1.5 1.5 0 013 14V5z"/><path d="M5 2h6v3H5z"/>') },
  llm_call_done:  { label: 'LLM 调用', color: '#8b5cf6', icon: S('<circle cx="8" cy="5" r="3"/><path d="M2 14c0-3.3 2.7-6 6-6"/><path d="M10 9c2.2 0 4 1.8 4 4"/>') },
  error:          { label: '错误',    color: '#dc2626', icon: S('<circle cx="8" cy="8" r="6"/><line x1="5.5" y1="5.5" x2="10.5" y2="10.5"/><line x1="10.5" y1="5.5" x2="5.5" y2="10.5"/>') },
  agent_result:   { label: 'Agent 结果', color: '#0ea5e9', icon: S('<circle cx="8" cy="8" r="3"/><circle cx="8" cy="8" r="6.5"/>') },
  compaction:     { label: '上下文压缩', color: '#94a3b8', icon: S('<path d="M3 4h10v8a2 2 0 01-2 2H5a2 2 0 01-2-2V4z"/><line x1="7" y1="2" x2="9" y2="2"/><line x1="6" y1="8" x2="10" y2="8"/><line x1="6" y1="10" x2="8" y2="10"/>') },
}

/** 风险等级配置 */
export const RISK_CONFIG: Record<string, { label: string; color: string; bg: string }> = {
  readonly:  { label: '只读', color: '#16a34a', bg: '#dcfce7' },
  mutation:  { label: '变更', color: '#d97706', bg: '#fef3c7' },
  dangerous: { label: '危险', color: '#dc2626', bg: '#fee2e2' },
}

/** 格式化字节 */
export function fmtSize(bytes: number): string {
  if (!bytes) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(1)) + ' ' + u[i]
}

/** 格式化日期 */
export function fmtDate(t: string): string {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

/** 格式化毫秒 */
export function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

/** 格式化 Token 数量 */
export function fmtTokens(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return String(n)
}
