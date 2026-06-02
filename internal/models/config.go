package models

import "time"

// ConfigEntry represents a single config section stored in the database.
type ConfigEntry struct {
	Section   string    `json:"section"`
	Data      string    `json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatSession represents a persisted AI chat session.
// Session type constants.
const (
	SessionTypeChat  = "chat"
	SessionTypeQQBot = "qqbot"
)

type ChatSession struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	MessagesJSON string    `json:"-" db:"messages"`
	MessageCount int       `json:"message_count"`
	// Pinned 是否置顶
	Pinned bool `json:"pinned"`
	// IsActive 是否有活跃的流或 Agent
	IsActive bool `json:"is_active"`
	// State 会话运行状态: idle | waiting_user | executing_tool | interrupted
	State      string `json:"state"`
	// CurrentGoal 当前任务目标
	CurrentGoal string `json:"current_goal,omitempty"`
	// WaitingFor 等待的具体描述
	WaitingFor  string `json:"waiting_for,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Session state constants
const (
	SessionStateIdle        = "idle"
	SessionStateWaitingUser = "waiting_user"
	SessionStateExecuting   = "executing_tool"
	SessionStateInterrupted = "interrupted"
)

// ChatSessionDetail includes session metadata and message history.
type ChatSessionDetail struct {
	Session  ChatSession   `json:"session"`
	Messages []ChatMessage `json:"messages"`
}

// ChatBlock is a frontend-facing content block in chronological order.
type ChatBlock struct {
	Type       string `json:"type"`
	Content    string `json:"content,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolArgs   string `json:"tool_args,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
}

// ChatMessage is a simplified view of ai.Message for the frontend.
type ChatMessage struct {
	Role             string         `json:"role"`
	Content          string         `json:"content"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []ChatToolCall `json:"tool_calls,omitempty"`
	// Blocks 按时间顺序排列的内容块。新格式优先，旧会话无此字段。
	Blocks []ChatBlock `json:"blocks,omitempty"`
}

// ChatToolCall represents a single tool invocation for the frontend.
type ChatToolCall struct {
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result"`
}