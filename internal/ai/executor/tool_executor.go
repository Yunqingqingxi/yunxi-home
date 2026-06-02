// Package executor 提供统一的工具执行能力（确认流、心跳、后台分发）。
package executor

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/middleware"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
)

// ConfirmFunc 确认回调（由上层注入，等待用户确认）
type ConfirmFunc func(toolName string, args map[string]any) (approved bool, fields map[string]string)

// ToolExecutor 统一工具执行器
type ToolExecutor struct {
	chain      *middleware.Chain        // 工具执行链（含 DenyEngine + 超时 + 重试）
	registry   *register.Registry       // 工具注册表
	confirmFn  ConfirmFunc              // 用户确认回调
	emitFn     func(ev base.ChatStreamEvent) // SSE 事件发射
}

// New 创建 ToolExecutor
func New(chain *middleware.Chain, registry *register.Registry) *ToolExecutor {
	return &ToolExecutor{
		chain:    chain,
		registry: registry,
	}
}

// SetConfirmFn 设置用户确认回调
func (t *ToolExecutor) SetConfirmFn(fn ConfirmFunc) {
	t.confirmFn = fn
}

// SetEmitFn 设置 SSE 事件发射回调
func (t *ToolExecutor) SetEmitFn(fn func(ev base.ChatStreamEvent)) {
	t.emitFn = fn
}

// emit 发射 SSE 事件（nil-safe）
func (t *ToolExecutor) emit(ev base.ChatStreamEvent) {
	if t.emitFn != nil {
		t.emitFn(ev)
	}
}

// Execute 执行单个工具调用，返回格式化的观察结果
func (t *ToolExecutor) Execute(ctx context.Context, toolName string, args map[string]any) string {
	tool, ok := t.registry.Get(toolName)

	// 危险工具：需要用户确认
	if ok && tool != nil && tool.RiskLevel == "dangerous" && t.confirmFn != nil {
		approved, _ := t.confirmFn(toolName, args)
		if !approved {
			obs := fmt.Sprintf("[%s 已取消] 用户未确认危险操作，工具未执行", toolName)
			t.emit(base.ChatStreamEvent{Type: "tool_result", Tool: toolName, Content: obs})
			return obs
		}
	}

	// 发射 tool_start 事件
	t.emit(base.ChatStreamEvent{Type: "tool_start", Tool: toolName, Args: formatArgs(args)})

	startTime := time.Now()

	// 后台工具：不阻塞，立即返回占位
	if ok && tool != nil && tool.Background {
		t.emit(base.ChatStreamEvent{
			Type: "tool_result",
			Tool: toolName,
			Content: fmt.Sprintf("[后台执行] 任务 '%s' 已提交，正在后台运行。完成后结果会自动注入对话。", toolName),
		})
		return fmt.Sprintf("[后台执行] 任务 '%s' 已提交，正在后台运行。", toolName)
	}

	// 同步执行（带心跳）
	result := t.executeSync(ctx, toolName, args, startTime)

	// 发射 tool_result 事件
	t.emit(base.ChatStreamEvent{Type: "tool_result", Tool: toolName, Content: formatObservation(toolName, result)})

	return formatObservation(toolName, result)
}

// executeSync 同步执行工具（带心跳）
func (t *ToolExecutor) executeSync(ctx context.Context, toolName string, args map[string]any, startTime time.Time) *base.ToolResult {
	type toolDone struct {
		result *base.ToolResult
	}

	doneCh := make(chan toolDone, 1)
	go func() {
		doneCh <- toolDone{result: t.chain.Execute(ctx, toolName, args)}
	}()

	heartbeat := time.NewTicker(2 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case d := <-doneCh:
			return d.result
		case <-heartbeat.C:
			elapsed := int(time.Since(startTime).Seconds())
			t.emit(base.ChatStreamEvent{
				Type:    "tool_progress",
				Tool:    toolName,
				Content: fmt.Sprintf("执行中... (%ds)", elapsed),
				Args:    fmt.Sprintf("%d", elapsed),
			})
		case <-ctx.Done():
			return &base.ToolResult{
				Status:  base.StatusError,
				Error:   &base.ToolError{Code: base.ErrCodeTimeout, Message: "会话已断开", Retryable: false},
				Summary: fmt.Sprintf("[%s 执行中断] 客户端连接已断开", toolName),
			}
		}
	}
}

// ExecuteBackground 后台执行工具（异步，结果通过回调注入）
func (t *ToolExecutor) ExecuteBackground(ctx context.Context, toolName string, args map[string]any, onComplete func(result *base.ToolResult)) {
	go func() {
		result := t.chain.Execute(ctx, toolName, args)
		if onComplete != nil {
			onComplete(result)
		}
	}()
}

// IsBackground 检查工具是否为后台类型
func (t *ToolExecutor) IsBackground(toolName string) bool {
	tool, ok := t.registry.Get(toolName)
	return ok && tool != nil && tool.Background
}

// IsDangerous 检查工具是否为危险类型
func (t *ToolExecutor) IsDangerous(toolName string) bool {
	tool, ok := t.registry.Get(toolName)
	return ok && tool != nil && tool.RiskLevel == "dangerous"
}

// formatArgs 格式化参数为 JSON 字符串
func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	// 简单字符串拼接，避免引入 encoding/json 依赖
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	result := "{"
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	result += "}"
	return result
}

// formatObservation 格式化工具执行结果为观察文本
func formatObservation(toolName string, result *base.ToolResult) string {
	if result == nil {
		return fmt.Sprintf("[%s 执行失败] 未知错误：结果为空", toolName)
	}

	if result.Status == base.StatusError && result.Error != nil {
		obs := fmt.Sprintf("[%s 执行失败] 错误码: %s 详情: %s", toolName, result.Error.Code, result.Error.Message)
		if result.Error.Retryable {
			obs += fmt.Sprintf(" (可重试: %s)", result.Error.RetryHint)
		}
		if result.Error.Fallback != "" {
			obs += fmt.Sprintf(" 建议降级: %s", result.Error.Fallback)
		}
		log.Warn("工具执行失败", "工具", toolName, "错误", result.Error.Message)
		return obs
	}

	summary := result.Summary
	if summary == "" {
		summary = fmt.Sprintf("[%s 执行成功]", toolName)
	}
	return summary
}
