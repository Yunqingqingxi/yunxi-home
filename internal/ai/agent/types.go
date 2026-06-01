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

// SubAgent 一个 AI 派生的子 Agent
type SubAgent struct {
	ID          string          `json:"agent_id"`
	Goal        string          `json:"goal"`         // 单一子任务描述
	ToolFilter  []string        `json:"tool_filter"`  // 允许的工具列表（空=全部，["*"]=全部）
	Context     []base.Message  `json:"-"`            // 独立上下文
	Status      Status          `json:"status"`
	Summary     string          `json:"summary"`      // 完成后的结果摘要
	Error       string          `json:"error,omitempty"`
	Round       int             `json:"round"`        // 已完成轮次
	Progress    string          `json:"progress"`     // 当前进度描述
	StartedAt   time.Time       `json:"started_at"`
	FinishedAt  time.Time       `json:"finished_at,omitempty"`
	progressFn  ProgressFunc    // 创建时捕获的进度回调（nil-safe）
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
// sessionID 是父会话 ID，results 是所有 Agent 的结果。
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
