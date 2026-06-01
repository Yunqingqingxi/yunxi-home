package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// Manager manages chat sessions (CRUD + persistence).
type Manager struct {
	repo     database.ChatSessionRepository
	sessions map[string]*state
	mu       sync.RWMutex
}

type state struct {
	info       models.ChatSession
	history    []base.Message
	turnCount  int
	totalInputTokens  int
	totalOutputTokens int
}

// NewManager creates a new session manager and loads existing sessions from DB.
func NewManager(repo database.ChatSessionRepository) *Manager {
	m := &Manager{
		repo:     repo,
		sessions: make(map[string]*state),
	}
	m.loadFromDB()
	return m
}

// List returns all sessions (with message counts).
func (m *Manager) List() []models.ChatSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]models.ChatSession, 0, len(m.sessions))
	for _, st := range m.sessions {
		info := st.info
		if info.MessageCount == 0 {
			info.MessageCount = len(st.history)
		}
		result = append(result, info)
	}
	return result
}

// Get returns a single session by ID.
func (m *Manager) Get(sessionID string) *models.ChatSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if st, ok := m.sessions[sessionID]; ok {
		return &st.info
	}
	return nil
}

// GetHistory returns the full session with message history.
func (m *Manager) GetHistory(sessionID string) (*models.ChatSessionDetail, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	toolResults := make(map[string]string)
	for _, msg := range st.history {
		if msg.Role == "tool" && msg.ToolCallID != "" {
			toolResults[msg.ToolCallID] = msg.Content
		}
	}

	messages := make([]models.ChatMessage, 0, len(st.history))
	for _, msg := range st.history {
		if msg.Role == "system" {
			continue
		}
		cm := models.ChatMessage{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}
		for _, tc := range msg.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, models.ChatToolCall{
				Name:   tc.Function.Name,
				Args:   tc.Function.Arguments,
				Result: toolResults[tc.ID],
			})
		}
		// 传递 blocks（新格式），tool_result 块合并到同 ID 的 tool_call 块
		if len(msg.Blocks) > 0 {
			for _, b := range msg.Blocks {
				cb := models.ChatBlock{
					Type:     string(b.Type),
					Content:  b.Content,
					ToolName: b.ToolName,
					ToolArgs: b.ToolArgs,
				}
				if b.Type == "tool_result" {
					cb.ToolResult = b.ToolResult
				}
				// 为 tool_call 块填充结果
				if b.Type == "tool_call" && b.ToolCallID != "" {
					if result, ok := toolResults[b.ToolCallID]; ok {
						cb.ToolResult = result
					}
				}
				cm.Blocks = append(cm.Blocks, cb)
			}
		}
		messages = append(messages, cm)
	}
	return &models.ChatSessionDetail{Session: st.info, Messages: messages}, nil
}

// TurnStats 多轮对话统计
type TurnStats struct {
	TurnCount        int `json:"turn_count"`
	TotalInputTokens  int `json:"total_input_tokens"`
	TotalOutputTokens int `json:"total_output_tokens"`
	MessageCount     int `json:"message_count"`
}

// GetTurnStats 返回多轮对话统计
func (m *Manager) GetTurnStats(sessionID string) TurnStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.sessions[sessionID]
	if !ok {
		return TurnStats{}
	}
	return TurnStats{
		TurnCount:        st.turnCount,
		TotalInputTokens:  st.totalInputTokens,
		TotalOutputTokens: st.totalOutputTokens,
		MessageCount:     len(st.history),
	}
}

// RecordTurn 记录一轮对话的 token 使用
func (m *Manager) RecordTurn(sessionID string, inputTokens, outputTokens int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.sessions[sessionID]; ok {
		st.turnCount++
		st.totalInputTokens += inputTokens
		st.totalOutputTokens += outputTokens
		slog.Debug("对话统计", "会话ID", sessionID, "总轮次", st.turnCount, "输入Token", st.totalInputTokens, "输出Token", st.totalOutputTokens)
	}
}

// TurnBoundaryHint 在每个 user 消息之间插入轻量分隔符，帮助模型感知轮次边界。
// 仅在超过 5 轮且未启用计划模式时插入（减少 token 浪费）。
func (m *Manager) TurnBoundaryHint(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.sessions[sessionID]
	if !ok || st.turnCount < 5 {
		return ""
	}
	return fmt.Sprintf("[对话第 %d 轮]", st.turnCount+1)
}

// GetOrCreate returns the session history, creating a new session if needed.
// sessionType must be one of models.SessionTypeChat or models.SessionTypeQQBot.
func (m *Manager) GetOrCreate(sessionID, sessionType, userMessage string) ([]base.Message, *models.ChatSession) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.sessions[sessionID]
	if !ok {
		if sessionType == "" {
			sessionType = models.SessionTypeChat
		}
		now := time.Now()
		st = &state{
			info: models.ChatSession{
				ID:        sessionID,
				Type:      sessionType,
				Title:     truncateStr(userMessage, 40),
				CreatedAt: now,
				UpdatedAt: now,
			},
			history: []base.Message{{Role: "system", Content: base.BuildSystemPrompt(userMessage, nil)}},
		}
		m.sessions[sessionID] = st
		if m.repo != nil {
			_ = m.repo.Upsert(context.Background(), &st.info)
		}
	} else {
		// Repair incomplete tool calls from previous crash
		st.history = repairIncompleteToolCalls(st.history)
	}
	st.info.UpdatedAt = time.Now()
	return st.history, &st.info
}

// ListByType returns sessions filtered by type.
func (m *Manager) ListByType(sessionType string) []models.ChatSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]models.ChatSession, 0)
	for _, st := range m.sessions {
		if st.info.Type == sessionType {
			result = append(result, st.info)
		}
	}
	return result
}

// DeleteByType removes all sessions of the given type.
func (m *Manager) DeleteByType(sessionType string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, st := range m.sessions {
		if st.info.Type == sessionType {
			delete(m.sessions, id)
			count++
		}
	}
	if m.repo != nil {
		n, _ := m.repo.DeleteByType(context.Background(), sessionType)
		if n > 0 { count = int(n) }
	}
	return count
}

// CleanStale removes sessions of the given type not updated within olderThan.
func (m *Manager) CleanStale(sessionType string, olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	count := 0
	for id, st := range m.sessions {
		if st.info.Type == sessionType && st.info.UpdatedAt.Before(cutoff) {
			delete(m.sessions, id)
			count++
		}
	}
	if m.repo != nil {
		m.repo.DeleteStale(context.Background(), sessionType, olderThan)
	}
	return count
}

// ForkAt 从指定消息索引处分叉，截断后续所有上下文。
// messageIndex 指向要保留的最后一条消息的索引（0-based）。
// newContent 非空时替换该消息；为空时仅截断到该位置（含该消息）。
func (m *Manager) ForkAt(sessionID string, messageIndex int, newContent string) ([]base.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if messageIndex < 0 || messageIndex >= len(st.history) {
		return nil, fmt.Errorf("invalid fork index %d (history has %d messages)", messageIndex, len(st.history))
	}
	if messageIndex == 0 {
		return nil, fmt.Errorf("cannot fork at index 0 (system prompt)")
	}

	// 截断到 fork 点（保留 [0:messageIndex]，丢弃之后所有）
	truncated := len(st.history) - messageIndex
	st.history = st.history[:messageIndex]

	// 替换 fork 点的消息（用户编辑了它）
	if newContent != "" {
		st.history[messageIndex-1] = base.Message{Role: "user", Content: newContent}
	}

	st.info.UpdatedAt = time.Now()

	if m.repo != nil {
		messagesJSON, _ := json.Marshal(st.history)
		st.info.MessagesJSON = string(messagesJSON)
		go func() { _ = m.repo.Upsert(context.Background(), &st.info) }()
	}

	slog.Info("session forked",
		"session", sessionID,
		"at_index", messageIndex,
		"truncated_msgs", truncated,
	)

	cp := make([]base.Message, len(st.history))
	copy(cp, st.history)
	return cp, nil
}

// GetRawHistory 返回原始消息历史（不含 system prompt）
func (m *Manager) GetRawHistory(sessionID string) ([]base.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	cp := make([]base.Message, len(st.history))
	copy(cp, st.history)
	return cp, nil
}

// Save persists the session history.
func (m *Manager) Save(sessionID string, history []base.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	cp := make([]base.Message, len(history))
	copy(cp, history)
	st.history = cp
	st.info.UpdatedAt = time.Now()
	if m.repo != nil {
		messagesJSON, _ := json.Marshal(cp)
		st.info.MessagesJSON = string(messagesJSON)
		slog.Debug("会话保存", "会话ID", sessionID, "消息数", len(cp), "JSON大小", len(st.info.MessagesJSON))
		go func() { _ = m.repo.Upsert(context.Background(), &st.info) }()
	}
}

// UpdateTitle updates the session title.
func (m *Manager) UpdateTitle(sessionID, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.sessions[sessionID]; ok {
		st.info.Title = title
		if m.repo != nil {
			_ = m.repo.Upsert(context.Background(), &st.info)
		}
	}
}

// Compact compresses session history: keeps system prompt + recent N messages,
// replaces middle messages with a summary. Returns a human-readable result.
func (m *Manager) Compact(sessionID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.sessions[sessionID]
	if !ok {
		return "会话不存在"
	}

	history := st.history
	if len(history) <= 8 {
		return fmt.Sprintf("消息较少（%d 条），无需压缩", len(history))
	}

	// 保留 system prompt（index 0）+ 最近 6 条
	prefixLen := 1 // system prompt
	for i := 1; i < len(history) && i < 3; i++ {
		if history[i].Role == "system" {
			prefixLen = i + 1
		}
	}
	keepRecent := 6
	midStart := prefixLen
	midEnd := len(history) - keepRecent
	if midStart >= midEnd {
		return fmt.Sprintf("消息较少（%d 条），无需压缩", len(history))
	}

	compactedCount := midEnd - midStart
	summaryContent := buildCompactionSummary(history[midStart:midEnd])

	// 构建新的紧凑历史：prefix + summary + recent
	newHistory := make([]base.Message, 0, prefixLen+1+keepRecent)
	newHistory = append(newHistory, history[:prefixLen]...)
	newHistory = append(newHistory, base.Message{
		Role:    "system",
		Content: summaryContent,
	})
	newHistory = append(newHistory, history[midEnd:]...)

	st.history = newHistory
	st.info.UpdatedAt = time.Now()

	if m.repo != nil {
		messagesJSON, _ := json.Marshal(newHistory)
		st.info.MessagesJSON = string(messagesJSON)
		go func() { _ = m.repo.Upsert(context.Background(), &st.info) }()
	}

	slog.Info("会话已压缩",
		"session", sessionID,
		"before", len(history),
		"after", len(newHistory),
		"compacted", compactedCount,
	)

	return fmt.Sprintf("上下文已压缩：%d 条历史消息 → 摘要（保留最近 %d 条）", compactedCount, keepRecent)
}

// CompactWithSummary appends an AI-generated compaction marker to the session history
// without removing any existing messages. The summary is generated externally (by AI).
// keepRecent is the number of recent messages to preserve in the effective context.
// The caller is responsible for updating the in-memory history slice to use the compacted
// version; this method only persists the marker.
func (m *Manager) CompactWithSummary(sessionID string, summary string, keepRecent int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.sessions[sessionID]
	if !ok {
		return "会话不存在"
	}

	history := st.history
	totalMsgs := len(history)

	// Count how many messages are being summarized (exclude system prompt + keepRecent)
	prefixLen := 1
	for i := 1; i < len(history) && i < 3; i++ {
		if history[i].Role == "system" {
			prefixLen = i + 1
		}
	}
	compactedCount := totalMsgs - prefixLen - keepRecent
	if compactedCount < 0 {
		compactedCount = 0
	}
	if compactedCount == 0 {
		return fmt.Sprintf("消息较少（%d 条），无需压缩", totalMsgs)
	}

	// Append compaction marker to full history (immutable append)
	marker := base.Message{
		Role:    "system",
		Content: fmt.Sprintf("[上下文压缩摘要] 以下是对此前 %d 条消息的 AI 摘要：\n%s", compactedCount, summary),
	}
	st.history = append(st.history, marker)
	st.info.UpdatedAt = time.Now()

	if m.repo != nil {
		messagesJSON, _ := json.Marshal(st.history)
		st.info.MessagesJSON = string(messagesJSON)
		go func() { _ = m.repo.Upsert(context.Background(), &st.info) }()
	}

	slog.Info("会话已压缩（追加AI摘要标记）",
		"session", sessionID,
		"total_msgs", totalMsgs,
		"compacted", compactedCount,
		"keep_recent", keepRecent,
		"summary_len", len(summary),
	)

	return fmt.Sprintf("✅ 上下文已压缩：%d 条早期消息 → AI 摘要（保留最近 %d 条）", compactedCount, keepRecent)
}

// buildCompactionSummary 从被压缩的消息中提取关键信息生成摘要
func buildCompactionSummary(messages []base.Message) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[上下文摘要] 此前共 %d 条消息的关键内容：\n", len(messages)))

	added := 0
	for _, msg := range messages {
		if msg.Role == "system" || msg.Content == "" {
			continue
		}
		content := msg.Content
		// 截断过长内容
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("- 用户: %s\n", content))
			added++
		case "assistant":
			sb.WriteString(fmt.Sprintf("- 助手: %s\n", content))
			added++
		case "tool":
			if len([]rune(content)) > 100 {
				content = string([]rune(content)[:100]) + "..."
			}
			sb.WriteString(fmt.Sprintf("- 工具结果(%s): %s\n", msg.ToolCallID, content))
			added++
		}
		if added >= 20 {
			sb.WriteString("- ...（更多内容已省略）\n")
			break
		}
	}
	return sb.String()
}

// Delete removes a session.
func (m *Manager) Delete(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	if m.repo != nil {
		_ = m.repo.Delete(context.Background(), sessionID)
	}
}

// DeleteAll removes all sessions.
func (m *Manager) DeleteAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*state)
	if m.repo != nil {
		_ = m.repo.DeleteAll(context.Background())
	}
}

func (m *Manager) loadFromDB() {
	if m.repo == nil {
		return
	}
	ctx := context.Background()
	dbSessions, err := m.repo.List(ctx)
	if err != nil {
		slog.Warn("failed to load chat sessions from DB", "error", err)
		return
	}
	for _, dbs := range dbSessions {
		history := []base.Message{{Role: "system", Content: base.CorePrompt}}
		if dbs.MessagesJSON != "" && dbs.MessagesJSON != "[]" {
			var msgs []base.Message
			if err := json.Unmarshal([]byte(dbs.MessagesJSON), &msgs); err == nil {
				history = msgs
			}
		}
		m.sessions[dbs.ID] = &state{info: dbs, history: history}
	}
	slog.Info("loaded chat sessions from DB", "count", len(dbSessions))
}

// repairIncompleteToolCalls 修复因服务崩溃导致的不完整工具调用。
// 如果 assistant 消息有 HasToolCalls=true 但缺少对应的 tool 结果消息，
// 插入合成错误结果，使对话历史格式有效，避免后续 LLM API 调用被拒绝。
func repairIncompleteToolCalls(history []base.Message) []base.Message {
	if len(history) == 0 {
		return history
	}
	// Work on a copy
	fixed := make([]base.Message, 0, len(history)+4)
	for i, msg := range history {
		fixed = append(fixed, msg)
		if msg.Role != "assistant" || !msg.HasToolCalls || len(msg.ToolCalls) == 0 {
			continue
		}
		// Collect expected tool_call IDs
		expectedIDs := make(map[string]bool, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			expectedIDs[tc.ID] = true
		}
		// Scan forward to find tool result messages matching these IDs
		for j := i + 1; j < len(history); j++ {
			if history[j].Role == "tool" && expectedIDs[history[j].ToolCallID] {
				delete(expectedIDs, history[j].ToolCallID)
			}
			// Stop at the next user or assistant message (end of tool results scope)
			if history[j].Role == "user" || history[j].Role == "assistant" {
				break
			}
		}
		// Insert synthetic error results for missing IDs
		for _, tc := range msg.ToolCalls {
			if expectedIDs[tc.ID] {
				fixed = append(fixed, base.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("[%s 执行中断] 服务异常重启，工具调用未完成", tc.Function.Name),
				})
			}
		}
	}
	return fixed
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// ── Session State Management ──────────────────────────────────────────

// SetState updates the session state and optionally the goal/waiting_for fields.
func (m *Manager) SetState(sessionID, state, goal, waitingFor string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.sessions[sessionID]
	if !ok { return }
	st.info.State = state
	if goal != "" { st.info.CurrentGoal = goal }
	st.info.WaitingFor = waitingFor
	st.info.UpdatedAt = time.Now()
}

// GetState returns the current session state.
func (m *Manager) GetState(sessionID string) (state, goal, waitingFor string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.sessions[sessionID]
	if !ok { return models.SessionStateIdle, "", "" }
	return st.info.State, st.info.CurrentGoal, st.info.WaitingFor
}

// BuildResumePrompt generates a context-aware resume prompt based on session state.
func (m *Manager) BuildResumePrompt(sessionID, userMessage string) string {
	state, goal, waitingFor := m.GetState(sessionID)
	switch state {
	case models.SessionStateWaitingUser:
		return fmt.Sprintf(
			"你之前正在等待用户确认操作「%s」。用户现在回复了：「%s」。请根据用户的回复（同意/拒绝/修改）继续执行。"+
				"\n如果用户表达了新意图，以新意图为准，放弃之前的等待。",
			waitingFor, userMessage)
	case models.SessionStateInterrupted:
		return fmt.Sprintf(
			"你之前正在执行操作，但被用户打断。用户的新消息是：「%s」。请评估当前状态，决定是继续还是重新开始。"+
				"\n如果之前的工具结果仍然有效，可以复用；否则重新执行。",
			userMessage)
	case models.SessionStateExecuting:
		return fmt.Sprintf(
			"你之前正在执行工具操作，尚未完成。用户现在说：「%s」。如果用户想打断或改变方向，请优先响应用户。",
			userMessage)
	default:
		if goal != "" {
			return fmt.Sprintf(
				"你有一个未完成的目标「%s」。用户说：「%s」。如果这与目标相关，请继续执行；如果是新任务，以新任务为准。",
				goal, userMessage)
		}
		return "你有一个未完成的目标需要继续。请回顾以上历史记录，从上次中断的地方继续执行。\n如果目标已完成，请给出总结。"
	}
}
