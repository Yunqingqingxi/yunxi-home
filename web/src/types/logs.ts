// ── 日志系统 TypeScript 类型定义 ────────────────────────────
// 对应 Go 后端 LogEvent 结构体 (internal/ai/chat_logger.go)

/** 会话日志事件类型 (18种) */
export type EventType =
  | 'session_start' | 'session_end' | 'session_save'
  | 'round_start' | 'round_end'
  | 'user_message' | 'inject'
  | 'thinking' | 'content' | 'answer'
  | 'tool_call' | 'tool_start' | 'tool_progress' | 'tool_result'
  | 'strategy'
  | 'llm_call_done'
  | 'error'
  | 'agent_result'
  | 'compaction'

/** 工具风险等级 */
export type RiskLevel = 'readonly' | 'mutation' | 'dangerous'

/** 轮次状态 */
export type TurnStatus = 'waiting_user' | 'done' | 'error'

/** Go LogEvent 结构体完整映射 */
export interface LogEvent {
  ts: string
  session: string
  round: number
  type: EventType
  tool_name?: string
  tool_args?: string
  tool_status?: string
  tool_result?: string
  tool_dur_ms?: number
  content?: string
  model?: string
  prompt_tokens?: number
  output_tokens?: number
  cache_hit?: boolean
  cache_tokens?: number
  cost_usd?: number
  error?: string
  msg_count?: number
  tool_count?: number
  strategy?: string
  strategy_reason?: string
  turn_status?: TurnStatus
  duration_sec?: number
  round_dur_ms?: number
  llm_dur_ms?: number
  risk_level?: RiskLevel
  extra?: Record<string, unknown>
}

/** GET /api/logs/chat/sessions 响应中的会话信息 */
export interface SessionInfo {
  session_id: string
  file: string
  size: number
  created: string
  rounds: number
  active: boolean
}

/** GET /api/logs/chat/:id 响应中的事件摘要 */
export interface EventSummary {
  errors: number
  tool_calls: number
  rounds: number
  types: Record<string, number>
}

/** 会话日志 API 响应 */
export interface ChatLogResponse {
  events: LogEvent[]
  total: number
  filtered: number
  summary: EventSummary
}

/** 系统日志文件信息 */
export interface SysFileInfo {
  date: string
  path: string
  size: number
}

/** 分析概览 */
export interface AnalyticsSummary {
  total_requests: number
  total_errors: number
  active_sessions: number
  total_tokens_in: number
  total_tokens_out: number
  total_cost_usd: number
  error_rate: number
}

/** 工具使用统计 */
export interface ToolStat {
  tool_name: string
  calls: number
  errors: number
  avg_lat_ms: number
  max_lat_ms: number
}

/** 每日汇总 */
export interface DailySummary {
  date: string
  requests: number
  errors: number
  tokens_in: number
  tokens_out: number
  cost_usd: number
}

/** SSE 连接状态 */
export type SSEStatus = 'disconnected' | 'connecting' | 'connected' | 'error' | 'full'

/** 会话日志查看模式 */
export type ChatViewMode = 'timeline' | 'text' | 'json'

/** 日志级别 */
export type LogLevel = 'ERROR' | 'WARN' | 'INFO' | 'DEBUG'

/** 系统日志筛选条件 */
export interface SysLogFilter {
  levels: Set<LogLevel>
  components: Set<string>
  search: string
  order: 'asc' | 'desc'
}

/** 会话日志筛选条件 */
export interface ChatLogFilter {
  eventTypes: Set<EventType>
  search: string
  order: 'asc' | 'desc'
  roundFilter: number | null
  errorsOnly: boolean
}

/** 系统日志结构化的行解析结果 */
export interface ParsedLogLine {
  raw: string
  timestamp: string
  level: LogLevel | ''
  component: string
  message: string
  fields: Record<string, string>
}
