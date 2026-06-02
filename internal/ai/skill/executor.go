package skill

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"net/http"
	"time"
)

// ── SkillExecutor ───────────────────────────────────────────────────────

// Executor 技能执行器。统一管理编程式和 YAML 声明式技能的查找和执行。
type Executor struct {
	registry   *Registry
	mcp        MCPContext    // 全局 MCP 上下文（可为 nil）
	yamlRunner *Runner       // 兼容旧 YAML 技能的执行器（可为 nil）
}

// NewExecutor 创建技能执行器
func NewExecutor(registry *Registry, mcp MCPContext) *Executor {
	return &Executor{registry: registry, mcp: mcp}
}

// SetYAMLRunner 设置 YAML 技能执行器（兼容旧系统）
func (e *Executor) SetYAMLRunner(runner *Runner) {
	e.yamlRunner = runner
}

// Registry 返回底层的技能注册中心
func (e *Executor) Registry() *Registry { return e.registry }

// ── 执行 ──────────────────────────────────────────────────────────────

// Run 执行指定技能。优先查找编程式技能，再回退到 YAML 技能。
func (e *Executor) Run(ctx context.Context, name string, params map[string]any) (any, error) {
	// 1. 尝试编程式技能
	if s, ok := e.registry.Get(name); ok {
		return e.runProgrammatic(ctx, s, params)
	}
	// 2. 回退到 YAML 技能
	if m, ok := e.registry.GetYAML(name); ok {
		return e.runYAML(ctx, m, params)
	}
	return nil, fmt.Errorf("skill not found: %s", name)
}

// runProgrammatic 执行编程式技能
func (e *Executor) runProgrammatic(ctx context.Context, s ProgrammaticSkill, params map[string]any) (any, error) {
	// 参数校验
	if sws, ok := s.(SkillWithSchema); ok {
		schema := sws.Parameters()
		if err := validateParams(params, schema); err != nil {
			return nil, fmt.Errorf("参数校验失败: %w", err)
		}
	}

	logger := e.getLogger()
	logger.Debug("executing programmatic skill", "skill", s.Name(), "params", params)

	start := time.Now()
	result, err := s.Run(ctx, params, e.mcp)
	if err != nil {
		logger.Error("skill failed", "skill", s.Name(), "error", err, "duration_ms", time.Since(start).Milliseconds())
		return nil, err
	}

	logger.Debug("skill completed", "skill", s.Name(), "duration_ms", time.Since(start).Milliseconds())
	return result, nil
}

// runYAML 执行 YAML 声明式技能
func (e *Executor) runYAML(ctx context.Context, m *Manifest, _ map[string]any) (any, error) {
	if e.yamlRunner == nil {
		return nil, fmt.Errorf("YAML runner not configured")
	}
	exec := e.yamlRunner.Execute(ctx, m)
	// 收集结果
	var parts []string
	for _, step := range exec.Steps {
		status := "✅"
		if step.Error != "" {
			status = fmt.Sprintf("❌ %s", step.Error)
		}
		parts = append(parts, fmt.Sprintf("  %d. %s %s", step.ID, step.Purpose, status))
	}
	return parts, nil
}

// ── 辅助 ──────────────────────────────────────────────────────────────

func (e *Executor) getLogger() *slog.Logger {
	if e.mcp != nil {
		return e.mcp.Logger()
	}
	return slog.Default()
}

// validateParams 根据 JSON Schema 校验参数
func validateParams(params map[string]any, schema ParamSchema) error {
	if schema.Required != nil {
		for _, req := range schema.Required {
			if _, ok := params[req]; !ok {
				return fmt.Errorf("缺少必填参数: %s", req)
			}
		}
	}
	// 类型校验（简化版）
	for name, prop := range schema.Properties {
		val, ok := params[name]
		if !ok {
			continue
		}
		switch prop.Type {
		case "string":
			if _, ok := val.(string); !ok {
				return fmt.Errorf("参数 %s 应为字符串", name)
			}
		case "number", "integer":
			switch val.(type) {
			case float64, int, int64, float32:
				// OK
			default:
				return fmt.Errorf("参数 %s 应为数字", name)
			}
		case "boolean":
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("参数 %s 应为布尔值", name)
			}
		case "array":
			if _, ok := val.([]any); !ok {
				return fmt.Errorf("参数 %s 应为数组", name)
			}
		}
	}
	return nil
}

// ── SimpleMCPContext ────────────────────────────────────────────────────
// 不依赖 MCP 系统的轻量上下文实现

// SimpleMCPContext 一个基础的 MCPContext 实现
type SimpleMCPContext struct {
	Log       *slog.Logger
	cache     map[string][]byte
}

// NewSimpleMCPContext 创建基础 MCP 上下文
func NewSimpleMCPContext(logger *slog.Logger) MCPContext {
	return &SimpleMCPContext{
		Log:   logger,
		cache: make(map[string][]byte),
	}
}

func (c *SimpleMCPContext) HTTPClient() *http.Client { return http.DefaultClient }
func (c *SimpleMCPContext) Logger() *slog.Logger      { return c.Log }
func (c *SimpleMCPContext) GetCache(key string) ([]byte, error) {
	if v, ok := c.cache[key]; ok { return v, nil }
	return nil, fmt.Errorf("cache miss: %s", key)
}
func (c *SimpleMCPContext) SetCache(key string, value []byte, ttl time.Duration) error {
	c.cache[key] = value
	return nil
}
