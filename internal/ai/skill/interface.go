// Package skill 提供可编程技能系统。
// 技能分为两类：YAML 声明式技能（Manifest）和 Go 编程式技能（ProgrammaticSkill）。
package skill

import (
	"context"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"net/http"
	"time"
)

// ── MCPContext ──────────────────────────────────────────────────────────
// 暴露给 Skill 的安全上下文接口。Skill 通过此接口使用系统能力，
// 不直接访问全局状态。

// MCPContext 是 MCP 系统暴露给 Skill 的安全接口。
type MCPContext interface {
	// HTTPClient 返回带超时和重试的 HTTP 客户端
	HTTPClient() *http.Client
	// Logger 返回结构化日志器
	Logger() *slog.Logger
	// Cache 操作
	GetCache(key string) ([]byte, error)
	SetCache(key string, value []byte, ttl time.Duration) error
}

// ── Skill 接口 ──────────────────────────────────────────────────────────

// ProgrammaticSkill 是 Go 代码实现的技能接口。
// 与 YAML 声明的 Manifest 技能互补——前者适合复杂逻辑，后者适合简单流程编排。
type ProgrammaticSkill interface {
	// Name 返回技能唯一名称（用于注册和调用）
	Name() string
	// Description 返回人类可读描述
	Description() string
	// Category 返回分类：ops | file | dns | system | general
	Category() string
	// RiskLevel 返回风险等级：readonly | mutation | dangerous
	RiskLevel() string

	// Run 执行技能。ctx 支持超时和取消。
	// params 为调用方传入的已解析参数。
	// 返回结果可以是任意类型，由调用方负责格式化。
	Run(ctx context.Context, params map[string]any, mcp MCPContext) (any, error)
}

// ── SkillWithSchema ─────────────────────────────────────────────────────
// 可选接口：技能实现此接口可声明参数 Schema，Executor 会在执行前校验。

// ParamSchema 简化的 JSON Schema 参数定义
type ParamSchema struct {
	Type       string                  `json:"type"`
	Properties map[string]ParamProp    `json:"properties,omitempty"`
	Required   []string                `json:"required,omitempty"`
}

// ParamProp 参数属性
type ParamProp struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

// SkillWithSchema 声明参数 Schema 的技能（可选实现）。
type SkillWithSchema interface {
	ProgrammaticSkill
	// Parameters 返回参数的 JSON Schema
	Parameters() ParamSchema
}

// ── SkillWithExamples ───────────────────────────────────────────────────
// 可选接口：为 AI 提供 few-shot 示例。

// ToolExample 工具调用示例
type ToolExample struct {
	Description string         `json:"description"`
	Args        map[string]any `json:"args"`
}

// SkillWithExamples 提供 AI 示例的技能（可选实现）。
type SkillWithExamples interface {
	ProgrammaticSkill
	// Examples 返回 few-shot 示例
	Examples() []ToolExample
}
