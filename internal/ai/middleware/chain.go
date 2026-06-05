// Package middleware 提供工具调用链的中间件：校验、重试、并发、错误路由。
package middleware

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
)

var log = logger.ForComponent("middleware")

// ToolResultCallback is called after every tool execution to allow external systems
// (e.g., topology tracker) to observe tool outcomes.
type ToolResultCallback func(ctx context.Context, toolName string, result *base.ToolResult)

// Chain 工具调用中间件链
type Chain struct {
	registry    *register.Registry
	maxRetry    int
	deny        *DenyEngine // 硬边界：不可绕过的安全规则引擎
	hooks       *HookRegistry // 可配置的事件钩子系统（Claude Code 风格）
	onResult    ToolResultCallback
}

// NewChain 创建中间件链
func NewChain(reg *register.Registry) *Chain {
	return &Chain{registry: reg, maxRetry: 1, deny: NewDenyEngine()}
}

// DenyEngine 返回拒绝规则引擎（供外部添加自定义规则）
func (c *Chain) DenyEngine() *DenyEngine { return c.deny }

// SetHookRegistry injects a hook registry for tool lifecycle interception.
func (c *Chain) SetHookRegistry(hr *HookRegistry) { c.hooks = hr }

// HookRegistry returns the current hook registry (may be nil if not set).
func (c *Chain) HookRegistry() *HookRegistry { return c.hooks }

// SetResultCallback registers a callback invoked after each tool execution.
// Used by the topology tracker to observe tool outcomes.
func (c *Chain) SetResultCallback(cb ToolResultCallback) {
	c.onResult = cb
}

// Execute 执行工具调用，经过 pre → retry → post 处理
func (c *Chain) Execute(ctx context.Context, toolName string, args map[string]any) *base.ToolResult {
	start := time.Now()

	// Extract session ID from context for logging and topology correlation
	sessionID := ""
	if v := ctx.Value(base.SessionIDKey{}); v != nil {
		if sid, ok := v.(string); ok {
			sessionID = sid
		}
	}

	// 0. Deny check — 硬边界：不可绕过的安全规则（Deny > Allow > Ask）
	if denied := c.deny.Check(toolName, args); denied != nil {
		denied.Metadata.DurationMs = time.Since(start).Milliseconds()
		log.Warn("tool denied by security rule",
			"tool", toolName,
			"session", sessionID,
			"reason", denied.Error.Message)
		return denied
	}

	// 0.5 PreToolUse hook — Claude Code style: exit code 2 blocks, 1 warns
	if c.hooks != nil {
		if blocked, warnings := c.hooks.CheckToolPreUse(toolName, args); blocked {
			return &base.ToolResult{
				Status: base.StatusError,
				Error:  &base.ToolError{
					Code:     base.ErrCodePermissionDenied,
					Message:  fmt.Sprintf("Hook blocked: %s", warnings[0]),
					Retryable: false,
				},
				Summary:  fmt.Sprintf("[%s 已被 hook 阻止] %s", toolName, warnings[0]),
				Metadata: base.ToolMetadata{DurationMs: time.Since(start).Milliseconds()},
			}
		} else if len(warnings) > 0 {
			for _, w := range warnings {
				log.Warn("tool pre-use hook warning", "tool", toolName, "warning", w)
			}
		}
	}

	// 1. PreHook: resolve timeout — AI-assigned > tool default
	timeout := resolveTimeout(toolName, args, c.registry)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	tool, ok := c.registry.Get(toolName)
	if !ok {
		return &base.ToolResult{
			Status: base.StatusError,
			Error: &base.ToolError{
				Code:     base.ErrCodeUnknown,
				Message:  "未知工具: " + toolName,
				Fallback: "",
			},
			Summary:  "工具未注册",
			Metadata: base.ToolMetadata{DurationMs: time.Since(start).Milliseconds()},
		}
	}

	// 2. 执行（优先用 V2 处理器）
	log.Info("tool execute start",
		"tool", toolName,
		logger.KeySessionID, sessionID,
		"has_v2", tool.HandlerV2 != nil)
	var result *base.ToolResult
	if tool.HandlerV2 != nil {
		result = tool.HandlerV2(ctx, args)
	} else {
		// 旧版兼容：包装 string + error → ToolResult
		data, err := tool.Handler(ctx, args)
		result = wrapLegacyResult(data, err)
	}

	// 2b. Check if context was cancelled during execution (timeout)
	if ctx.Err() != nil {
		if c.hooks != nil {
			_, _ = c.hooks.CheckToolPostUse(toolName, args, fmt.Sprintf("timeout: %v", ctx.Err()))
		}
		if result.Status != base.StatusError {
			result = &base.ToolResult{
				Status: base.StatusError,
				Error: &base.ToolError{
					Code:      base.ErrCodeTimeout,
					Message:   fmt.Sprintf("工具执行超时或上下文已取消: %v", ctx.Err()),
					Retryable: true,
					RetryHint: "如需重试，请检查操作是否部分完成，避免重复执行副作用",
				},
				Summary: fmt.Sprintf("[%s 执行超时] 操作在完成前被中断，请检查当前状态", toolName),
			}
		} else {
			// Context cancelled AND result already has error — ensure it's marked as timeout
			result.Error.Code = base.ErrCodeTimeout
			result.Error.Retryable = true
			result.Summary = fmt.Sprintf("[%s 执行超时] %s", toolName, result.Error.Message)
		}
	}

	// 3. PostToolUse hook — Claude Code style: inspect result before retry decision
	if c.hooks != nil {
		resultSummary := result.Summary
		if result.Error != nil {
			resultSummary = result.Error.Message
		}
		blocked, warnings := c.hooks.CheckToolPostUse(toolName, args, resultSummary)
		if blocked {
			reason := "unknown"
			if len(warnings) > 0 { reason = warnings[0] }
			result = &base.ToolResult{
				Status:    base.StatusError,
				Error:     &base.ToolError{Code: base.ErrCodePermissionDenied, Message: "Post-use hook blocked: " + reason, Retryable: false},
				Summary:   fmt.Sprintf("[%s 已被 hook 阻止] %s", toolName, reason),
				Metadata:  base.ToolMetadata{DurationMs: time.Since(start).Milliseconds()},
			}
		}
		for _, w := range warnings {
			log.Warn("tool post-use hook warning", "tool", toolName, "warning", w)
		}
	}

	// 4. PostHook: 注入工具名称和耗时
	duration := time.Since(start).Milliseconds()
	result.Metadata.DurationMs = duration
	if result.Error != nil && result.Error.Code == "" {
		result.Error.Code = base.ErrCodeUnknown
	}
	if result.Summary == "" {
		result.Summary = toolName + " 执行完成"
	}

	// 5. 智能重试
	if result.Status == base.StatusError && result.Error != nil && result.Error.Retryable {
		policy := tool.RetryPolicy
		if policy == nil {
			policy = &base.RetryPolicy{MaxRetries: c.maxRetry, Backoff: 2 * time.Second}
		}
		for retry := 0; retry < policy.MaxRetries; retry++ {
			if result.Error.RetryHint != "" {
				log.Info("retrying tool with hint", "tool", toolName, "hint", result.Error.RetryHint)
			}
			// OnRetry hook
			if c.hooks != nil {
				if blocked, _ := c.hooks.CheckToolPreUse(toolName, args); blocked {
					log.Warn("retry blocked by hook", "tool", toolName)
					return result
				}
			}
			select {
			case <-ctx.Done():
				return result
			case <-time.After(policy.Backoff * time.Duration(retry+1)):
			}
			if tool.HandlerV2 != nil {
				result = tool.HandlerV2(ctx, args)
			} else {
				data, err := tool.Handler(ctx, args)
				result = wrapLegacyResult(data, err)
			}
			result.Metadata.DurationMs = time.Since(start).Milliseconds()
			if result.Status != base.StatusError {
				break
			}
		}
	}

	log.Info("tool executed via middleware",
		"tool", toolName,
		logger.KeySessionID, sessionID,
		"status", string(result.Status),
		"duration_ms", result.Metadata.DurationMs,
		"summary", truncate(result.Summary, 200),
		"has_error", result.Error != nil,
	)

	// Notify topology tracker via callback
	if c.onResult != nil {
		c.onResult(ctx, toolName, result)
	}

	return result
}

// ExecuteParallel 并发执行多个工具调用
func (c *Chain) ExecuteParallel(ctx context.Context, calls []ToolCall) []*base.ToolResult {
	results := make([]*base.ToolResult, len(calls))
	type indexedResult struct {
		idx int
		res *base.ToolResult
	}
	ch := make(chan indexedResult, len(calls))

	for i, call := range calls {
		go func(idx int, tc ToolCall) {
			ch <- indexedResult{idx, c.Execute(ctx, tc.Name, tc.Args)}
		}(i, call)
	}

	for range calls {
		r := <-ch
		results[r.idx] = r.res
	}
	return results
}

// ToolCall 表示一次工具调用
type ToolCall struct {
	Name string
	Args map[string]any
}

// wrapLegacyResult 将旧版 (string, error) 包装为 ToolResult
func wrapLegacyResult(data string, err error) *base.ToolResult {
	if err != nil {
		code := classifyError(err)
		retryable := base.IsRetryable(code)
		hint := ""
		if retryable {
			switch code {
			case base.ErrCodeTimeout:
				hint = "请检查命令是否可拆分或使用更短的超时时间"
			case base.ErrCodeNetworkError:
				hint = "请检查网络连接后重试"
			default:
				hint = "请检查命令是否正确"
			}
		}
		// Include data in error message only if not already present (prevent duplication)
		msg := err.Error()
		trimmed := strings.TrimSpace(data)
		if trimmed != "" && !strings.Contains(msg, trimmed) {
			msg = msg + "\n输出: " + trimmed
		}
		return &base.ToolResult{
			Status: base.StatusError,
			Error: &base.ToolError{
				Code:      code,
				Message:   msg,
				Retryable: retryable,
				RetryHint: hint,
			},
			Summary: fmt.Sprintf("命令执行失败 (%s): %s", code, msg),
		}
	}
	status := base.StatusSuccess
	if data == "" || data == "[]" || data == "null" {
		status = base.StatusPartial
		data = "(命令执行成功，无输出)"
	}
	return &base.ToolResult{
		Status:  status,
		Data:    data,
		Summary: data,
	}
}

// classifyError 根据错误消息推断错误码
func classifyError(err error) string {
	msg := err.Error()
	switch {
	case containsAny(msg, "timeout", "deadline", "context"):
		return base.ErrCodeTimeout
	case containsAny(msg, "permission", "denied", "forbidden", "access"):
		return base.ErrCodePermissionDenied
	case containsAny(msg, "network", "connection", "refused", "unreachable"):
		return base.ErrCodeNetworkError
	case containsAny(msg, "not found", "no such", "不存在", "未找到"):
		return base.ErrCodeFileNotFound
	case containsAny(msg, "invalid", "required", "参数", "格式"):
		return base.ErrCodeInvalidArgs
	default:
		return base.ErrCodeExecFailed
	}
}

func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// resolveTimeout determines the effective timeout for a tool call.
// Priority: AI-assigned timeout in args > tool's registered Timeout > 0 (none).
// AI-assigned values are clamped to [3s, 120s] to prevent abuse.
func resolveTimeout(toolName string, args map[string]any, reg *register.Registry) time.Duration {
	// Check for AI-assigned timeout in args
	if v, ok := args["timeout"]; ok {
		secs := toInt(v)
		if secs > 0 {
			if secs < 3 {
				secs = 3
			}
			if secs > 120 {
				secs = 120
			}
			return time.Duration(secs) * time.Second
		}
	}
	// Fall back to tool's registered timeout
	if td, ok := reg.Get(toolName); ok && td.Timeout > 0 {
		return td.Timeout
	}
	return 0
}

// toInt converts various numeric types to int.
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case float32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	case string:
		// JSON numbers might come as strings
		var f float64
		if _, err := fmt.Sscanf(n, "%f", &f); err == nil {
			return int(f)
		}
	}
	return 0
}
