package logger

// ──────────────────────────────────────────────
// 标准日志字段 Key 常量
//
// 所有包统一使用这些常量，替代裸字符串 key，
// 确保日志字段命名一致、可检索。
// ──────────────────────────────────────────────

const (
	// ── 核心标识 ──
	KeyComponent = "component" // 哪个子系统：dns / ai / web / db / bot / scheduler ...
	KeyEvent     = "event"     // 什么操作：http_request / file_write / bot_message / tool_call ...
	KeyTraceID   = "trace_id"  // 分布式追踪 ID
	KeySpanID    = "span_id"   // 当前 span ID

	// ── HTTP ──
	KeyHTTPMethod = "http_method"
	KeyHTTPURI    = "http_uri"
	KeyHTTPStatus = "http_status"
	KeyLatency    = "latency_ms"

	// ── DNS / 调度 ──
	KeyDomain   = "domain"
	KeyRR       = "rr"
	KeyRecordID = "record_id"
	KeyIP       = "ip"
	KeyOldIP    = "old_ip"
	KeyNewIP    = "new_ip"

	// ── AI / 工具 ──
	KeyTool       = "tool"
	KeyToolStatus = "tool_status"
	KeyToolRisk   = "tool_risk"
	KeyModel      = "model"
	KeyRound      = "round"
	KeyTokensIn   = "tokens_in"
	KeyTokensOut  = "tokens_out"
	KeyCacheHit   = "cache_hit"
	KeySessionID  = "session_id"

	// ── 文件操作 ──
	KeyFilePath = "file_path"
	KeyFileSize = "file_size"

	// ── 通用 ──
	KeyError  = "error"
	KeyCount  = "count"
	KeyUserID = "user_id"
	KeySource = "source"
	KeyPath   = "path"
)

// ──────────────────────────────────────────────
// 事件类型常量（KeyEvent 的标准取值）
// ──────────────────────────────────────────────

const (
	// ── Web 层 ──
	EventHTTPRequest = "http_request"

	// ── 文件操作 ──
	EventFileRead   = "file_read"
	EventFileWrite  = "file_write"
	EventFileDelete = "file_delete"
	EventFileUpload = "file_upload"

	// ── Bot 消息 ──
	EventBotMessage = "bot_message"
	EventBotCommand = "bot_command"
	EventBotEvent   = "bot_event"

	// ── DNS ──
	EventDNSUpdate = "dns_update"
	EventDNSCheck  = "dns_check"

	// ── AI / Chat ──
	EventSessionStart = "session_start"
	EventSessionEnd   = "session_end"
	EventToolCall     = "tool_call"
	EventToolResult   = "tool_result"
	EventLLMCall      = "llm_call"
	EventIntentRoute  = "intent_route"
	EventCompaction   = "compaction"

	// ── 数据库 ──
	EventDBMigration = "db_migration"
	EventDBSync      = "db_sync"

	// ── 调度器 ──
	EventSchedulerTick = "scheduler_tick"
	EventCronTrigger   = "cron_trigger"

	// ── 系统 ──
	EventStartup  = "startup"
	EventShutdown = "shutdown"
	EventConfig   = "config_change"
)
