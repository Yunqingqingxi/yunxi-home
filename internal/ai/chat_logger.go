package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

// ── ChatLogger: 完整会话追踪日志 ────────────────────────────────
//
// 双层输出：
//   1. JSONL 行 — 结构化，供程序分析（保留所有细节）
//   2. 人类可读行 — 高度可读的文本摘要（策略变更、自然语言结果、决策点突出）
//
// 事件类型：
//   user, thinking, content, tool_call, tool_result,
//   strategy, answer, error, inject, llm_call_done,
//   round_start, round_end, session_start, session_end, session_save

// LogEvent 单条追踪日志条目
type LogEvent struct {
	Timestamp      string         `json:"ts"`
	SessionID      string         `json:"session"`
	Round          int            `json:"round"`
	Type           string         `json:"type"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolArgs       string         `json:"tool_args,omitempty"`
	ToolStatus     string         `json:"tool_status,omitempty"`
	ToolResult     string         `json:"tool_result,omitempty"`
	ToolDurMs      int64          `json:"tool_dur_ms,omitempty"`
	Content        string         `json:"content,omitempty"`
	Model          string         `json:"model,omitempty"`
	PromptTokens   int            `json:"prompt_tokens,omitempty"`
	OutputTokens   int            `json:"output_tokens,omitempty"`
	CacheHit       bool           `json:"cache_hit,omitempty"`
	CacheTokens    int            `json:"cache_tokens,omitempty"`
	Cost           float64        `json:"cost_usd,omitempty"`
	Error          string         `json:"error,omitempty"`
	MsgCount       int            `json:"msg_count,omitempty"`
	ToolCount      int            `json:"tool_count,omitempty"`
	// Strategy fields — 记录策略变更
	Strategy       string         `json:"strategy,omitempty"`
	StrategyReason string         `json:"strategy_reason,omitempty"`
	// TurnStatus — round_end 时标记等待用户/完成/错误
	TurnStatus     string         `json:"turn_status,omitempty"`
	// DurationSec — tool_result 的总耗时（秒），替代冗余的 progress 日志
	DurationSec    float64        `json:"duration_sec,omitempty"`
	// RiskLevel — 工具风险等级: readonly | mutation | dangerous
	RiskLevel      string         `json:"risk_level,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

// turnState 单轮次的状态，用于人类可读格式化
type turnState struct {
	seq           int     // 当前轮次内序号（单调递增）
	pendingCalls  int     // 尚未收到结果的工具调用数
	groupLabel    string  // 当前并行组标签（A/B/C...），空表示非并行
	groupActive   bool    // 是否处于并行组内
	nextGroup     rune    // 下一个并行组标签
	userMessage   string  // 本轮用户消息（用于轮次头）
	roundPrinted  bool    // 是否已打印轮次头
	strategySeq   int     // 当前策略链序号
	strategies    []string // 本轮策略链 ["binary_first", "docker", "source_build"]
	lastProgress  time.Time // 上次 tool_progress 时间（用于去重）
	currentTool   string  // 当前正在执行的工具名（用于进度去重）
}

// ChatLogger 会话追踪日志器
type ChatLogger struct {
	mu          sync.Mutex
	dir         string
	files       map[string]*os.File // sessionID → file handle
	subs        map[string]map[chan LogEvent]struct{} // sessionID → subscriber channels
	turnStates  map[string]*turnState // sessionID → per-session turn state
	enabled     bool
}

// NewChatLogger 创建日志器。dir 为日志目录，空字符串表示禁用。
func NewChatLogger(dir string) *ChatLogger {
	cl := &ChatLogger{
		dir:        dir,
		files:      make(map[string]*os.File),
		subs:       make(map[string]map[chan LogEvent]struct{}),
		turnStates: make(map[string]*turnState),
	}
	if dir != "" {
		cl.enabled = true
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Warn("无法创建聊天日志目录，已禁用", "dir", dir, "error", err)
			cl.enabled = false
		}
	}
	return cl
}

// Enabled reports whether the logger is active.
func (cl *ChatLogger) Enabled() bool { return cl != nil && cl.enabled }

// getFile returns the file handle for a session, creating it if needed.
func (cl *ChatLogger) getFile(sessionID string) (*os.File, error) {
	if f, ok := cl.files[sessionID]; ok {
		return f, nil
	}
	name := fmt.Sprintf("chat-%s-%s.log", time.Now().Format("20060102"), sanitizeSessionID(sessionID))
	f, err := os.OpenFile(filepath.Join(cl.dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	cl.files[sessionID] = f
	return f, nil
}

// Close flushes and closes all log files.
func (cl *ChatLogger) Close() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	for id, f := range cl.files {
		f.Close()
		delete(cl.files, id)
	}
}

const (
	maxLogFieldLen   = 2000  // 单个字段最大字符数（JSONL）
	maxLogFileSize   = 10 * 1024 * 1024 // 单日志文件最大 10MB
	maxLogFileCount     = 5                 // 最多保留 5 个轮转文件
	progressMinInterval = 5 * time.Second   // tool_progress 最小间隔（去重）
)

// write 写入一条 JSONL 日志 + 一行人类可读摘要
func (cl *ChatLogger) write(ev LogEvent) {
	if !cl.enabled {
		return
	}

	// 截断过长的字段，防止日志膨胀
	ev.Content = truncateStr(ev.Content, maxLogFieldLen)
	ev.ToolArgs = truncateStr(ev.ToolArgs, maxLogFieldLen)
	ev.ToolResult = truncateStr(ev.ToolResult, maxLogFieldLen)
	ev.Error = truncateStr(ev.Error, maxLogFieldLen)

	cl.mu.Lock()
	defer cl.mu.Unlock()

	f, err := cl.getFile(ev.SessionID)
	if err != nil {
		return
	}

	// 检查文件大小，超过限制则轮转
	if fi, _ := f.Stat(); fi != nil && fi.Size() > maxLogFileSize {
		f.Close()
		delete(cl.files, ev.SessionID)
		cl.rotateLogs(ev.SessionID)
		f, err = cl.getFile(ev.SessionID)
		if err != nil {
			return
		}
	}

	// JSONL line
	jsonData, _ := json.Marshal(ev)
	line := fmt.Sprintf("%s\n", jsonData)
	f.WriteString(line)

	// Human-readable summary (compact single line)
	humanLine := cl.formatHuman(ev)
	if humanLine != "" {
		f.WriteString(humanLine + "\n")
	}

	// 将关键事件同步到主日志，保持 ChatLogger 自身输出不变
	cl.bridgeToSlog(ev)

	// Broadcast to subscribers
	cl.broadcast(ev)
}

// bridgeToSlog 将关键事件同步到主 slog 日志，便于在主日志中追踪会话生命周期。
// 高频事件（thinking、content、tool_progress）不桥接，避免日志噪音。
func (cl *ChatLogger) bridgeToSlog(ev LogEvent) {
	base := []any{
		slog.String(logger.KeyComponent, "ai"),
		slog.String(logger.KeySessionID, ev.SessionID),
		slog.Int(logger.KeyRound, ev.Round),
	}

	switch ev.Type {
	case "session_start":
		slog.Info("会话开始", append(base, slog.String(logger.KeyEvent, logger.EventSessionStart))...)
	case "session_end":
		slog.Info("会话结束", append(base, slog.String(logger.KeyEvent, logger.EventSessionEnd))...)
	case "error":
		slog.Error("会话错误", append(base,
			slog.String(logger.KeyEvent, "error"),
			slog.String(logger.KeyError, ev.Error),
		)...)
	case "tool_call":
		slog.Info("工具调用", append(base,
			slog.String(logger.KeyEvent, logger.EventToolCall),
			slog.String(logger.KeyTool, ev.ToolName),
			slog.String(logger.KeyToolRisk, ev.RiskLevel),
		)...)
	case "tool_result":
		slog.Info("工具完成", append(base,
			slog.String(logger.KeyEvent, logger.EventToolResult),
			slog.String(logger.KeyTool, ev.ToolName),
			slog.String(logger.KeyToolStatus, ev.ToolStatus),
			slog.Int64("tool_dur_ms", ev.ToolDurMs),
		)...)
	case "llm_call_done":
		slog.Debug("LLM 调用完成", append(base,
			slog.String(logger.KeyEvent, logger.EventLLMCall),
			slog.String(logger.KeyModel, ev.Model),
			slog.Int(logger.KeyTokensIn, ev.PromptTokens),
			slog.Int(logger.KeyTokensOut, ev.OutputTokens),
			slog.Bool(logger.KeyCacheHit, ev.CacheHit),
			slog.Float64("cost_usd", ev.Cost),
		)...)
	case "compaction":
		slog.Info("会话压缩", append(base,
			slog.String(logger.KeyEvent, logger.EventCompaction),
			slog.String("summary", ev.Content),
		)...)
	}
}

// broadcast sends the event to all subscribers for this session.
func (cl *ChatLogger) broadcast(ev LogEvent) {
	subs, ok := cl.subs[ev.SessionID]
	if !ok || len(subs) == 0 {
		return
	}
	for ch := range subs {
		select {
		case ch <- ev:
		default:
			// slow consumer — drop
		}
	}
}

// Subscribe returns a channel that receives live log events for a session.
func (cl *ChatLogger) Subscribe(sessionID string) chan LogEvent {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	ch := make(chan LogEvent, 256)
	if cl.subs[sessionID] == nil {
		cl.subs[sessionID] = make(map[chan LogEvent]struct{})
	}
	if len(cl.subs[sessionID]) >= 10 {
		return nil // max subscribers
	}
	cl.subs[sessionID][ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber channel.
// The caller must pass the same channel returned by Subscribe (cast back to chan LogEvent).
func (cl *ChatLogger) Unsubscribe(sessionID string, ch chan LogEvent) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if subs, ok := cl.subs[sessionID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(cl.subs, sessionID)
		}
	}
}

// RemoveSession closes the file handle for the session and deletes all matching
// log files (including rotated files) from disk. Callers should check IsActive
// first to avoid deleting a session that is still being written to.
func (cl *ChatLogger) RemoveSession(sessionID string) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Close file handle if open
	if f, ok := cl.files[sessionID]; ok {
		f.Close()
		delete(cl.files, sessionID)
	}

	// Clean up subscribers
	if subs, ok := cl.subs[sessionID]; ok {
		for ch := range subs {
			close(ch)
		}
		delete(cl.subs, sessionID)
	}

	// Clean up turn state
	delete(cl.turnStates, sessionID)

	// Delete all matching log files on disk (including rotated files)
	pattern := filepath.Join(cl.dir, fmt.Sprintf("chat-*-%s.log", sanitizeSessionID(sessionID)))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			return err
		}
	}
	return nil
}

// IsActive returns true if the session has an open file handle (being written to).
func (cl *ChatLogger) IsActive(sessionID string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	_, ok := cl.files[sessionID]
	return ok
}

// ListSessions scans the log directory and returns info about each session log file.
func (cl *ChatLogger) ListSessions() []LogSessionInfo {
	if cl.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(cl.dir)
	if err != nil {
		return nil
	}
	var result []LogSessionInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".log" {
			continue
		}
		info, _ := e.Info()
		sessionID := extractSessionID(e.Name())
		if sessionID == "" {
			continue
		}
		size := int64(0)
		created := time.Time{}
		if info != nil {
			size = info.Size()
			created = info.ModTime()
		}
		result = append(result, LogSessionInfo{
			SessionID: sessionID,
			File:      e.Name(),
			Size:      size,
			Created:   created,
			Active:    cl.IsActive(sessionID),
		})
	}
	return result
}

// GetSessionLog reads the full JSONL log for a session and returns parsed events.
func (cl *ChatLogger) GetSessionLog(sessionID string) ([]LogEvent, error) {
	if cl.dir == "" {
		return nil, fmt.Errorf("logger disabled")
	}
	entries, err := os.ReadDir(cl.dir)
	if err != nil {
		return nil, err
	}
	var events []LogEvent
	for _, e := range entries {
		if e.IsDir() || !strings.Contains(e.Name(), sanitizeSessionID(sessionID)) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cl.dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "{") {
				continue
			}
			var ev LogEvent
			if json.Unmarshal([]byte(line), &ev) == nil {
				events = append(events, ev)
			}
		}
	}
	return events, nil
}

// GetSessionLogText 返回会话日志的人类可读文本（仅 formatHuman 输出行，跳过 JSON 行）。
func (cl *ChatLogger) GetSessionLogText(sessionID string) (string, error) {
	if cl.dir == "" {
		return "", fmt.Errorf("logger disabled")
	}
	entries, err := os.ReadDir(cl.dir)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.Contains(e.Name(), sanitizeSessionID(sessionID)) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cl.dir, e.Name()))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" || strings.HasPrefix(line, "{") {
				continue // 跳过 JSON 行和空行
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}

// LogSessionInfo is metadata about a single session log file.
type LogSessionInfo struct {
	SessionID string    `json:"session_id"`
	File      string    `json:"file"`
	Size      int64     `json:"size"`
	Created   time.Time `json:"created"`
	Rounds    int       `json:"rounds"`
	Active    bool      `json:"active"`
}

func extractSessionID(filename string) string {
	// Format: chat-YYYYMMDD-<sessionID>.log
	name := strings.TrimSuffix(filename, ".log")
	parts := strings.SplitN(name, "-", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// Dir returns the log directory path.
func (cl *ChatLogger) Dir() string { return cl.dir }

// rotateLogs removes oldest log files for this session beyond maxLogFileCount.
func (cl *ChatLogger) rotateLogs(sessionID string) {
	pattern := filepath.Join(cl.dir, fmt.Sprintf("chat-*-%s.log", sanitizeSessionID(sessionID)))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) <= maxLogFileCount {
		return
	}
	// Sort by name (which includes date) and remove oldest
	// Files are named chat-YYYYMMDD-sessionID.log so alphabetical = chronological
	for i := 0; i < len(matches)-maxLogFileCount; i++ {
		os.Remove(matches[i])
	}
}

// getTurnState returns the turn state for a session, creating it if needed.
// Must be called with cl.mu held.
func (cl *ChatLogger) getTurnState(sessionID string) *turnState {
	if ts, ok := cl.turnStates[sessionID]; ok {
		return ts
	}
	ts := &turnState{nextGroup: 'A'}
	cl.turnStates[sessionID] = ts
	return ts
}

// smartSummary converts a tool result JSON/blob into a concise human-readable summary.
func smartSummary(toolName, result string) string {
	if result == "" {
		return ""
	}
	if len(result) > 80 {
		// Try to extract key info
		if strings.Contains(result, "count") || strings.Contains(result, "files") {
			return truncateStr(result, 120)
		}
		return truncateStr(result, 100)
	}
	return result
}

// formatHuman produces enriched human-readable log output:
//
//	Round header → strategy → [seq] prefix: content
//	Decision points, strategy switches, and turn status highlighted.
func (cl *ChatLogger) formatHuman(ev LogEvent) string {
	ts := cl.getTurnState(ev.SessionID)

	switch ev.Type {
	case "session_start":
		shortID := ev.SessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		return fmt.Sprintf("══════════════════════════════════════════════════════\nSESSION: %s\n══════════════════════════════════════════════════════", shortID)

	case "session_end":
		delete(cl.turnStates, ev.SessionID)
		return "══════════════════════════════════════════════════════\nSESSION END\n══════════════════════════════════════════════════════"

	case "round_start":
		ts.seq = 0
		ts.pendingCalls = 0
		ts.groupLabel = ""
		ts.groupActive = false
		ts.roundPrinted = false
		ts.strategySeq = 0
		ts.strategies = nil
		ts.lastProgress = time.Time{}
		ts.currentTool = ""
		return ""

	case "user_message":
		ts.userMessage = ev.Content
		return "" // Printed as part of round header

	case "thinking", "content":
		ts.seq++
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] T: %s", ev.Round, ts.seq, truncateStr(ev.Content, 400)))

	case "tool_call":
		ts.seq++
		ts.pendingCalls++
		if ts.pendingCalls == 2 && !ts.groupActive {
			ts.groupActive = true
			ts.groupLabel = string(ts.nextGroup)
			ts.nextGroup++
		}
		groupAnnot := ""
		if ts.groupActive {
			groupAnnot = fmt.Sprintf("  // 并行组%s", ts.groupLabel)
		}
		emoji := toolEmoji(ev.ToolName)
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] %s %s(%s)%s", ev.Round, ts.seq, emoji, ev.ToolName, truncateStr(ev.ToolArgs, 300), groupAnnot))

	case "tool_result":
		ts.seq++
		ts.pendingCalls--
		if ts.pendingCalls == 0 {
			ts.groupActive = false
		}
		status := "OK"
		if ev.ToolStatus == "error" {
			status = "FAIL"
		} else if ev.ToolStatus == "partial" {
			status = "WARN"
		}
		dur := ""
		if ev.ToolDurMs > 0 {
			dur = fmt.Sprintf(" (%.1fs)", float64(ev.ToolDurMs)/1000)
		}
		summary := smartSummary(ev.ToolName, ev.ToolResult)
		if summary == "" {
			summary = status
		}
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] R: %s%s → %s", ev.Round, ts.seq, status, dur, summary))

	case "strategy":
		ts.seq++
		ts.strategySeq++
		ts.strategies = append(ts.strategies, ev.Strategy)
		reason := ""
		if ev.StrategyReason != "" {
			reason = fmt.Sprintf(" — %s", ev.StrategyReason)
		}
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] STRATEGY: %s%s", ev.Round, ts.seq, ev.Strategy, reason))

	case "answer":
		ts.seq++
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] A: %s", ev.Round, ts.seq, truncateStr(ev.Content, 1000)))

	case "error":
		ts.seq++
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] ERROR: %s", ev.Round, ts.seq, ev.Error))

	case "inject":
		ts.seq++
		return cl.formatWithHeader(ev, ts, fmt.Sprintf("[%d:%02d] INJECT: %s", ev.Round, ts.seq, truncateStr(ev.Content, 300)))

	case "round_end":
		// Footer with turn status
		statusLine := ""
		if ev.TurnStatus == "waiting_user" {
			statusLine = fmt.Sprintf("\nWAITING: %s", truncateStr(ev.Content, 200))
		} else if ev.TurnStatus == "error" {
			statusLine = "\nBUILD FAILED"
		}
		return fmt.Sprintf("─── 第 %d 轮结束 (工具 %d 次) ───%s", ev.Round, ev.ToolCount, statusLine)

	case "llm_call_done":
		return "" // JSONL only

	case "tool_start", "tool_progress":
		// Progress dedup: only log every progressMinInterval
		if ev.Type == "tool_progress" {
			now := time.Now()
			if now.Sub(ts.lastProgress) < progressMinInterval && ev.ToolName == ts.currentTool {
				return "" // Suppress redundant progress
			}
			ts.lastProgress = now
			ts.currentTool = ev.ToolName
		}
		return "" // tool_start absorbed into tool_call

	case "agent_result":
		status := "done"
		agentID := ""
		rounds := 0
		if ev.Extra != nil {
			if s, ok := ev.Extra["status"].(string); ok { status = s }
			if id, ok := ev.Extra["agent_id"].(string); ok { agentID = id }
			if r, ok := ev.Extra["rounds"].(float64); ok { rounds = int(r) }
		}
		icon := "✓"
		if status == "error" { icon = "✗" }
		return fmt.Sprintf("  [Agent %s] %s (%d轮) %s: %s", agentID[:min(8, len(agentID))], icon, rounds, status, truncateStr(ev.Content, 300))

	case "session_save":
		return ""
	}
	return ""
}

// printRoundHeader returns the round header block once per turn.
func (cl *ChatLogger) printRoundHeader(ev LogEvent, ts *turnState) string {
	if ts.roundPrinted {
		return ""
	}
	ts.roundPrinted = true

	strategyLine := ""
	if len(ts.strategies) > 0 {
		strategyLine = fmt.Sprintf("\nSTRATEGY: %s", strings.Join(ts.strategies, " → "))
	}

	userLine := ""
	if ts.userMessage != "" {
		userLine = fmt.Sprintf("\nUSER: %s", truncateStr(ts.userMessage, 200))
	}

	return fmt.Sprintf(
		"\n───────────────────────────────────────────────────────\nTURN %d 轮%s%s\n───────────────────────────────────────────────────────",
		ev.Round, userLine, strategyLine,
	)
}

// formatWithHeader prepends the round header (first call only) to the event line.
func (cl *ChatLogger) formatWithHeader(ev LogEvent, ts *turnState, line string) string {
	header := cl.printRoundHeader(ev, ts)
	if header != "" {
		return header + "\n" + line
	}
	return line
}

// toolEmoji returns a short text label for common tool types.
func toolEmoji(name string) string {
	switch {
	case strings.Contains(name, "file_read") || strings.Contains(name, "read"):
		return "READ"
	case strings.Contains(name, "file_list") || strings.Contains(name, "list") || strings.Contains(name, "find"):
		return "LIST"
	case strings.Contains(name, "file_write") || strings.Contains(name, "write") || strings.Contains(name, "edit"):
		return "WRITE"
	case strings.Contains(name, "file_delete") || strings.Contains(name, "delete") || strings.Contains(name, "rm"):
		return "DELETE"
	case strings.Contains(name, "run_command") || strings.Contains(name, "exec"):
		return "EXEC"
	case strings.Contains(name, "docker"):
		return "DOCKER"
	case strings.Contains(name, "get_system") || strings.Contains(name, "status"):
		return "STATUS"
	case strings.Contains(name, "search") || strings.Contains(name, "grep"):
		return "SEARCH"
	case strings.Contains(name, "build") || strings.Contains(name, "compile"):
		return "BUILD"
	case strings.Contains(name, "deploy"):
		return "DEPLOY"
	default:
		return "TOOL"
	}
}

// ── Convenience methods used by StreamChat ──

func (cl *ChatLogger) LogSessionStart(sessionID string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Type: "session_start"})
}

func (cl *ChatLogger) LogSessionEnd(sessionID string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Type: "session_end"})
}

func (cl *ChatLogger) LogRoundStart(sessionID string, round, msgCount, toolCount int) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "round_start", MsgCount: msgCount, ToolCount: toolCount})
}

func (cl *ChatLogger) LogThinking(sessionID string, round int, content string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "thinking", Content: content})
}

func (cl *ChatLogger) LogContent(sessionID string, round int, content string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "content", Content: content})
}

func (cl *ChatLogger) LogToolCall(sessionID string, round int, name, args, riskLevel string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "tool_call", ToolName: name, ToolArgs: args, RiskLevel: riskLevel})
}

func (cl *ChatLogger) LogToolStart(sessionID string, round int, name, args string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "tool_start", ToolName: name, ToolArgs: args})
}

func (cl *ChatLogger) LogToolProgress(sessionID string, round int, name, msg string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "tool_progress", ToolName: name, Content: msg})
}

func (cl *ChatLogger) LogToolResult(sessionID string, round int, name, status, result string, durMs int64, riskLevel string) {
	cl.write(LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round,
		Type: "tool_result", ToolName: name, ToolStatus: status, ToolResult: result,
		ToolDurMs: durMs, DurationSec: float64(durMs) / 1000, RiskLevel: riskLevel,
	})
}

func (cl *ChatLogger) LogStrategy(sessionID string, round int, strategy, reason string) {
	cl.write(LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round,
		Type: "strategy", Strategy: strategy, StrategyReason: reason,
	})
}

func (cl *ChatLogger) LogLLMCallDone(sessionID string, round int, model string, promptTok, outputTok, cacheTok int, cacheHit bool, cost float64) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "llm_call_done", Model: model, PromptTokens: promptTok, OutputTokens: outputTok, CacheTokens: cacheTok, CacheHit: cacheHit, Cost: cost})
}

func (cl *ChatLogger) LogUserMessage(sessionID string, round int, content string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "user_message", Content: content})
}

func (cl *ChatLogger) LogInject(sessionID string, round int, content string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "inject", Content: content})
}

func (cl *ChatLogger) LogError(sessionID string, round int, errMsg string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "error", Error: errMsg})
}

func (cl *ChatLogger) LogRoundEnd(sessionID string, round int, content string, blockCount int, turnStatus string) {
	cl.write(LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round,
		Type: "round_end", Content: content, ToolCount: blockCount, TurnStatus: turnStatus,
	})
}

// LogRoundEndSimple is a backward-compatible shorthand without turn status.
func (cl *ChatLogger) LogRoundEndSimple(sessionID string, round int, content string, blockCount int) {
	cl.LogRoundEnd(sessionID, round, content, blockCount, "")
}

func (cl *ChatLogger) LogAnswer(sessionID string, round int, content string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Round: round, Type: "answer", Content: content})
}

// LogAgentResult 记录异步 Agent 完成事件
func (cl *ChatLogger) LogAgentResult(sessionID string, agentID, goal, status, summary string, rounds int) {
	cl.write(LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID,
		Type: "agent_result", Content: summary,
		Extra: map[string]any{
			"agent_id":  agentID,
			"goal":      goal,
			"status":    status,
			"rounds":    rounds,
		},
	})
}

func (cl *ChatLogger) LogSessionSave(sessionID string) {
	cl.write(LogEvent{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), SessionID: sessionID, Type: "session_save"})
}

// LogCompaction records a /compact operation in the chat log.
func (cl *ChatLogger) LogCompaction(sessionID string, compactedCount, keepRecent int, summary string) {
	cl.write(LogEvent{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		SessionID:  sessionID,
		Type:       "compaction",
		Content:    fmt.Sprintf("compacted=%d→summary keep=%d summary_len=%d", compactedCount, keepRecent, len(summary)),
		TurnStatus: "completed",
	})
}

// sanitizeSessionID replaces risky chars for use in a filename.
func sanitizeSessionID(id string) string {
	b := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b = append(b, c)
		} else {
			b = append(b, '_')
		}
	}
	if len(b) > 32 {
		b = b[:32]
	}
	return string(b)
}
