// Package agent 提供子 Agent 派生和并行执行能力。
package agent

import (
	"context"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
)

// Status 子 Agent 状态
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusError   Status = "error"
)

// StatusFromAgentState 将 AgentState 映射为旧版 Status
func StatusFromAgentState(s AgentState) Status {
	switch s {
	case StateStart, StateReasoning, StateExecuting, StateRetry:
		return StatusRunning
	case StateWaitingLock, StateWaitingHuman, StateDelegate:
		return StatusRunning
	case StateSuspended, StateTimeout:
		return StatusPending
	case StateDone:
		return StatusDone
	case StateFailed:
		return StatusError
	case StateCancel:
		return StatusError
	default:
		return StatusPending
	}
}

// SubAgent 统一 Agent 结构 — 主 Agent 和子 Agent 共用。
// 主 Agent: ParentID 为空。
type SubAgent struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Task      string    `json:"task"`
	ParentID  string    `json:"parent_id"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Result    string    `json:"result,omitempty"`
	Timeout   int       `json:"timeout"` // 超时秒数
	Error     string    `json:"error,omitempty"`

	// ── 运行时字段（不序列化）──
	ToolFilter []string       `json:"-"`
	Context    []base.Message `json:"-"`
	Round      int            `json:"-"`
	Progress   string         `json:"-"`
	ProgressPct int           `json:"-"`
	StatusCode  int           `json:"-"`
	StartedAt   time.Time     `json:"-"`
	FinishedAt  time.Time     `json:"-"`
	progressFn  ProgressFunc
	State       *StateMachine `json:"-"`
}

// ToJSON returns the public-facing JSON representation (runtime fields excluded).
func (a *SubAgent) ToJSON() map[string]any {
	parentID := a.ParentID
	return map[string]any{
		"id":         a.ID,
		"task_id":    a.TaskID,
		"task":       a.Task,
		"parent_id":  parentID,
		"status":     string(a.Status),
		"created_at": a.CreatedAt.Format(time.RFC3339),
		"result":     a.Result,
		"timeout":    a.Timeout,
		"error":      a.Error,
	}
}

// Result 子 Agent 完成后返回的结果
type Result struct {
	AgentID string `json:"agent_id"`
	Goal    string `json:"goal"`
	Status  Status `json:"status"`
	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`
	Rounds  int    `json:"rounds"`
}

// agentSessionKey context key for agent session ID
type agentSessionKey struct{}

// WithSessionID injects a session ID into context for agent tools
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, agentSessionKey{}, sessionID)
}

// SessionIDFromCtx extracts the session ID from context
func SessionIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(agentSessionKey{}).(string); ok {
		return v
	}
	return ""
}

// ProgressFunc 进度回调，向父会话推送 SSE 事件
type ProgressFunc func(agent *SubAgent, eventType string)

// CompletionFunc 异步完成回调。当异步派生的所有 Agent 完成后调用。
type CompletionFunc func(sessionID string, results []*Result)

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxConcurrent int             // 最大并发子 Agent 数，默认 5
	MaxRounds     int             // 每个子 Agent 最大轮次，默认 100
	AgentTimeout  time.Duration   // 每个子 Agent 最大墙钟时间，默认 10min
	Provider      base.AIProvider // AI Provider
	Registry      *register.Registry
	ProgressFn    ProgressFunc
	CompletionFn  CompletionFunc // Agent 全部完成时回调（异步模式）
	// AgentLoader: loads YAML-defined agent templates for spawn_agent_name
}

// DefaultConfig 返回默认配置
func DefaultConfig(provider base.AIProvider, reg *register.Registry, progFn ProgressFunc) ManagerConfig {
	return ManagerConfig{
		MaxConcurrent: 5,
		MaxRounds:     100,
		AgentTimeout:  10 * time.Minute,
		Provider:      provider,
		Registry:      reg,
		ProgressFn:    progFn,
	}
}
