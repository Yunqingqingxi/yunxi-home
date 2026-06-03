package base

import (
	"context"
	"fmt"
	"time"
)

// ── 聊天消息类型 ─────────────────────────────────────────────

// ContentBlockType 内容块类型
type ContentBlockType string

const (
	BlockTypeThinking   ContentBlockType = "thinking"
	BlockTypeContent    ContentBlockType = "content"
	BlockTypeToolCall   ContentBlockType = "tool_call"
	BlockTypeToolResult ContentBlockType = "tool_result"
)

// ContentBlock 按时间顺序排列的内容块
// 用于保留 AI 响应中 thinking / content / tool_call / tool_result 的精确交错顺序
type ContentBlock struct {
	Type       ContentBlockType `json:"type"`
	Content    string           `json:"content,omitempty"`
	ToolName   string           `json:"tool_name,omitempty"`
	ToolArgs   string           `json:"tool_args,omitempty"`
	ToolResult string           `json:"tool_result,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

// Message 聊天消息
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	HasToolCalls     bool       `json:"has_tool_calls,omitempty"`
	// Blocks 按时间顺序排列的内容块，仅用于渲染。
	// 旧会话中此字段为空，前端会 fallback 到扁平字段重建。
	Blocks []ContentBlock `json:"blocks,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用详情
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ConfirmRequest 危险操作确认请求
type ConfirmRequest struct {
	ID      string         `json:"id"`
	Tool    string         `json:"tool"`
	Message string         `json:"message"`
	Fields  []ConfirmField `json:"fields"`
}

// ConfirmField 确认弹窗中的字段
type ConfirmField struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Value    string `json:"value,omitempty"`
}

// ── 工具定义 ─────────────────────────────────────────────────

// ToolDef 工具定义
type ToolDef struct {
	Name        string
	Description string
	Parameters  ToolParams
	Handler     ToolHandler
	// HandlerV2 可选，返回结构化结果。设置后优先于 Handler。
	HandlerV2 ToolHandlerV2
	// DependsOn 前置工具名列表，plan 模式用
	DependsOn []string
	// RetryPolicy 重试策略（可选）
	RetryPolicy *RetryPolicy
	// Timeout 超时时间（可选）
	Timeout time.Duration
	// Category 工具分类: "dns"|"file"|"docker"|"system"|"ops"
	Category string
	// RiskLevel 风险等级: "readonly"|"mutation"|"dangerous"
	RiskLevel string
	// Background 标记为可后台执行的长任务（不阻塞对话流）
	Background bool
	// IsConcurrencySafe 标记为可并发执行（只读工具可并行，写工具需串行）
	IsConcurrencySafe bool
	// Examples 工具使用示例（给 LLM 看的 few-shot）
	Examples []ToolExample
}

// ToolExample 工具调用示例
type ToolExample struct {
	Description string         `json:"description"`
	Args        map[string]any `json:"args"`
}

// ToolHandler 工具处理函数（旧版，兼容）
type ToolHandler func(ctx context.Context, args map[string]any) (string, error)

// ToolHandlerV2 工具处理函数（新版，返回结构化结果）
type ToolHandlerV2 func(ctx context.Context, args map[string]any) *ToolResult

// ToolParams JSON Schema 参数定义
type ToolParams struct {
	Type       string               `json:"type"`
	Properties map[string]ParamProp `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

// ParamProp 参数属性
type ParamProp struct {
	Type        string               `json:"type"`
	Description string               `json:"description"`
	Enum        []string             `json:"enum,omitempty"`
	Items       *ParamProp           `json:"items,omitempty"`
	Properties  map[string]ParamProp `json:"properties,omitempty"`
	Required    []string             `json:"required,omitempty"`
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries  int           // 最大重试次数（默认 1）
	Backoff     time.Duration // 退避基础间隔（默认 1s）
	RetryableOn []string      // 可重试的错误码列表
}

// ── 工具执行结果（结构化）──────────────────────────────────

// ToolResult 工具执行结果
type ToolResult struct {
	Status    ToolStatus   `json:"status"`              // success | partial | error
	Data      any          `json:"data"`                // 实际结果数据
	Error     *ToolError   `json:"error,omitempty"`     // 错误详情
	Summary   string       `json:"summary"`             // 给 AI 看的一句话摘要
	NextSteps []string     `json:"next_steps,omitempty"` // 建议后续操作
	Metadata  ToolMetadata `json:"metadata"`            // 执行元信息
}

// ToolStatus 工具执行状态
type ToolStatus string

const (
	StatusSuccess ToolStatus = "success"
	StatusPartial ToolStatus = "partial"
	StatusError   ToolStatus = "error"
)

// ToolError 结构化工具错误
type ToolError struct {
	Code      string `json:"code"`       // DNS_NOT_FOUND | PERMISSION_DENIED | TIMEOUT | EXEC_FAILED | NETWORK_ERROR
	Message   string `json:"message"`    // 人类可读错误消息
	Retryable bool   `json:"retryable"`  // 是否可重试
	RetryHint string `json:"retry_hint"` // 重试建议
	Fallback  string `json:"fallback"`   // 建议的降级工具名
}

// ToolMetadata 工具执行元信息
type ToolMetadata struct {
	DurationMs int64  `json:"duration_ms"`           // 执行耗时（毫秒）
	RowsAffected int  `json:"rows_affected,omitempty"` // 影响行数（数据库操作）
	BytesRead  int64  `json:"bytes_read,omitempty"`   // 读取字节数（文件操作）
	Truncated  bool   `json:"truncated"`              // 结果是否被截断
	OriginalLen int   `json:"original_len,omitempty"` // 截断前长度
}

// ── 错误码分类 ──────────────────────────────────────────────

const (
	ErrCodeDNSNotFound      = "DNS_NOT_FOUND"
	ErrCodeDNSCreateFailed  = "DNS_CREATE_FAILED"
	ErrCodeDNSEmptyResult   = "DNS_EMPTY_RESULT"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeExecFailed       = "EXEC_FAILED"
	ErrCodeNetworkError     = "NETWORK_ERROR"
	ErrCodeFileNotFound     = "FILE_NOT_FOUND"
	ErrCodeInvalidArgs      = "INVALID_ARGS"
	ErrCodeUnknown          = "UNKNOWN_ERROR"
)

// IsRetryable 判断错误码是否可重试
func IsRetryable(code string) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeNetworkError, ErrCodeDNSEmptyResult:
		return true
	default:
		return false
	}
}

// ── Plan 模式类型（Plan 5）─────────────────────────────────

// Plan 多步执行计划
type Plan struct {
	Steps             []PlanStep `json:"steps"`
	RollbackOnFailure bool       `json:"rollback_on_failure"`
	MaxConcurrency    int        `json:"max_concurrency,omitempty"` // 最大并发数
}

// PlanStep 执行步骤
type PlanStep struct {
	ID      int            `json:"id"`
	Tool    string         `json:"tool"`
	Args    map[string]any `json:"args"`
	Depends []int          `json:"depends"` // 依赖的步骤 ID 列表，空=可并发
	Purpose string         `json:"purpose"` // 步骤目的（给 LLM 看）
}

// PlanResult 计划执行结果
type PlanResult struct {
	Steps      []StepResult `json:"steps"`
	TotalSteps int          `json:"total_steps"`
	Successes  int          `json:"successes"`
	Failures   int          `json:"failures"`
	DurationMs int64        `json:"duration_ms"`
}

// StepResult 单步执行结果
type StepResult struct {
	ID     int         `json:"id"`
	Tool   string      `json:"tool"`
	Status ToolStatus  `json:"status"`
	Result *ToolResult `json:"result"`
}

// ── 流式事件 ────────────────────────────────────────────────

// ChatStreamEvent 流式对话事件
type ChatStreamEvent struct {
	Type    string `json:"type"`           // thinking | content | tool_call | tool_result | plan | error | done | todo_update | agent_progress | agent_result | skill_progress | cron_notify
	Content string `json:"content"`
	Tool    string `json:"tool"`
	Args    string `json:"args"`
	// Plan 模式专用
	PlanResult *PlanResult  `json:"plan_result,omitempty"`
	StepResult *StepResult  `json:"step_result,omitempty"`
	// Goal 模式专用
	GoalID      string `json:"goal_id,omitempty"`
	GoalTitle   string `json:"goal_title,omitempty"`
	GoalProgress int   `json:"goal_progress,omitempty"`
	// 跨会话通知
	CrossSession *CrossSessionEvent `json:"cross_session,omitempty"`
	// 后台任务
	TaskID       string `json:"task_id,omitempty"`
	TaskProgress int    `json:"task_progress,omitempty"`
	TaskStatus   string `json:"task_status,omitempty"`
	TaskMessage  string `json:"task_message,omitempty"`
	// Todo 列表更新
	Todos any `json:"todos,omitempty"`
	// 子 Agent 进度
	AgentID    string `json:"agent_id,omitempty"`
	AgentGoal  string `json:"agent_goal,omitempty"`
	AgentRound int    `json:"agent_round,omitempty"`
	AgentStatus string `json:"agent_status,omitempty"`
	// Skill 进度
	SkillName     string `json:"skill_name,omitempty"`
	SkillCurrentStep int `json:"skill_current_step,omitempty"`
	SkillTotalSteps  int `json:"skill_total_steps,omitempty"`
	SkillStepStatus  string         `json:"skill_step_status,omitempty"`
	ConfirmRequest     *ConfirmRequest     `json:"confirm_request,omitempty"`
	InteractiveRequest *InteractiveRequest `json:"interactive_request,omitempty"`
	// Usage is set by providers at stream end with token/cost data.
	Usage *StreamUsage `json:"usage,omitempty"`
	// Topology 拓扑约束事件
	TopologyUpdate *TopologyUpdateEvent `json:"topology_update,omitempty"`
}

// TopologyUpdateEvent is emitted after each topology validation round.
type TopologyUpdateEvent struct {
	SessionID    string     `json:"session_id"`
	Coord        Coordinate `json:"coord"`
	Trajectory   []Coordinate `json:"trajectory"`
	Constraint   TopologyConstraint `json:"constraint"`
	Rejected     bool       `json:"rejected"`
	RejectReason string     `json:"reject_reason,omitempty"`
	RejectCount  int        `json:"reject_count"`
	TrustLies    int        `json:"trust_lies"`
	TrustLocked  bool       `json:"trust_locked"`
	ClosedLoop   bool       `json:"closed_loop"`
	ClosedDist   float64    `json:"closed_distance,omitempty"`
	Warning      string     `json:"warning,omitempty"`
	Oscillation  bool       `json:"oscillation"`
	Override     bool       `json:"override"`
}

// Coordinate is a 3D point in topology space (mirror of topology.Coordinate for SSE).
type Coordinate struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// TopologyConstraint mirrors topology.Constraint for SSE events.
type TopologyConstraint struct {
	A          float64  `json:"a"`
	R          float64  `json:"r"`
	T          bool     `json:"t"`
	ForceTools []string `json:"force_tools,omitempty"`
}

// ── Topology State (for API response) ────────────────────────

// TopologyState is the full topology state returned by the API.
type TopologyState struct {
	SessionID   string             `json:"session_id"`
	CurrentCoord Coordinate        `json:"current_coord"`
	StartCoord  Coordinate         `json:"start_coord"`
	Constraint  TopologyConstraint `json:"constraint"`
	Trajectory  []TopologyNode     `json:"trajectory"`
	RejectCount int                `json:"reject_count"`
	TrustLies   int                `json:"trust_lies"`
	TrustLocked bool               `json:"trust_locked"`
	ClosedLoop  bool               `json:"closed_loop"`
	ClosedDist  float64            `json:"closed_distance,omitempty"`
	Warning     string             `json:"warning,omitempty"`
	Active      bool               `json:"active"`
}

// TopologyNode is a trajectory node for API responses.
type TopologyNode struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Z         float64 `json:"z"`
	Round     int     `json:"round"`
	ToolCall  string  `json:"tool_call"`
	Status    string  `json:"status"`
	Reason    string  `json:"reason,omitempty"`
}

// StreamUsage carries token and cost data from a completed LLM stream.
type StreamUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// ── 交互式请求/响应 ─────────────────────────────────────

// InteractiveRequest AI 发给前端的交互请求（弹窗让用户操作）
type InteractiveRequest struct {
	ID          string             `json:"id"`
	Type        string             `json:"type"` // "confirm" | "input" | "select" | "form"
	Title       string             `json:"title"`
	Message     string             `json:"message"`
	Fields      []InteractiveField `json:"fields,omitempty"`
	Options     []string           `json:"options,omitempty"`
	TimeoutSec  int                `json:"timeout_sec"`
	ConfirmText string             `json:"confirm_text,omitempty"`
	CancelText  string             `json:"cancel_text,omitempty"`
	Variant     string             `json:"variant,omitempty"`
	Pages       []InteractivePage  `json:"pages,omitempty"`   // v3.1 多页向导
	ActionURL   string             `json:"action_url,omitempty"`   // v3.3 操作链接（如 GitHub Token 页面）
	ActionLabel string             `json:"action_label,omitempty"` // v3.3 链接文字
}

// InteractivePage 向导中的一页
type InteractivePage struct {
	Title       string             `json:"title"`
	Description string             `json:"description,omitempty"`
	Fields      []InteractiveField `json:"fields"`
}

// InteractiveField 表单字段
type InteractiveField struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`        // "text" | "password" | "number"
	Placeholder string `json:"placeholder"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
}

// InteractiveResponse 前端返回的用户响应
type InteractiveResponse struct {
	ID       string            `json:"id"`
	Approved bool              `json:"approved"`           // confirm 模式
	Values   map[string]any    `json:"values,omitempty"`  // form/input 模式（any 类型兼容 number）
	Selected string            `json:"selected,omitempty"` // select 模式
}

// CrossSessionEvent 跨会话事件
type CrossSessionEvent struct {
	Type       string `json:"type"` // "lock_conflict" | "resource_modified" | "session_joined" | "session_left"
	SessionID  string `json:"session_id"`
	Resource   string `json:"resource,omitempty"`
	Message    string `json:"message"`
}

// ModelOverrideKey 用于在 context 中传递模型覆盖参数（避免循环 import）
type ModelOverrideKey struct{}

// SessionIDKey is used to pass session ID through context.
type SessionIDKey struct{}

// WithSessionID stores the session ID in the context (for tool middleware).
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey{}, sessionID)
}

// PlanModeKey 用于在 context 中传递 plan 模式开关 (deprecated in v3.1, kept for compat)
type PlanModeKey struct{}

// ReasoningIntensityKey 用于在 context 中传递推理强度
type ReasoningIntensityKey struct{}

// JSONOutputKey 用于在 context 中启用 DeepSeek JSON Output 模式
type JSONOutputKey struct{}

// CmdResultKey 用于在 context 中传递斜杠命令的后台预执行结果。
// 值为 string 类型：命令执行结果摘要，将作为 system 消息注入到 AI 对话历史中。
type CmdResultKey struct{}

// ── AI 服务接口 ─────────────────────────────────────────────

// AIProvider AI 大模型接口（策略模式：每个模型供应商独立实现）
type AIProvider interface {
	ChatStream(ctx context.Context, messages []Message, tools []ToolDef) (<-chan ChatStreamEvent, error)
	TestConnection(ctx context.Context) error
	// Models 返回该 Provider 支持的模型名列表
	Models() []string
	// DefaultReasoning 返回该 Provider 的默认推理深度
	DefaultReasoning() string
}

// ── Provider 工厂注册 ─────────────────────────────────────

// ProviderConfig 创建 Provider 所需的配置
type ProviderConfig struct {
	APIKey string
}

// ProviderFactory 创建 AIProvider 的工厂函数
type ProviderFactory func(cfg ProviderConfig) (AIProvider, error)

var providerFactories = map[string]ProviderFactory{}

// RegisterProvider 注册一个 Provider 工厂（在 init() 中调用）
func RegisterProvider(name string, factory ProviderFactory) {
	providerFactories[name] = factory
}

// CreateProvider 通过工厂创建 Provider 实例
func CreateProvider(name string, cfg ProviderConfig) (AIProvider, error) {
	if f, ok := providerFactories[name]; ok {
		return f(cfg)
	}
	return nil, fmt.Errorf("unknown provider: %s", name)
}

// RegisteredProviders 返回所有已注册的 Provider 名称
func RegisteredProviders() []string {
	names := make([]string, 0, len(providerFactories))
	for name := range providerFactories {
		names = append(names, name)
	}
	return names
}
