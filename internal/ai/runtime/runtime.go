// Package runtime 封装单轮 AI 推理调用（Provider + Stream 处理）。
package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// RoundResult 单轮推理结果
type RoundResult struct {
	Content    string          // AI 文本回复
	Reasoning  string          // 思考过程
	ToolCalls  []base.ToolCall // 工具调用列表
	Usage      *base.StreamUsage // Token 用量
	Error      error           // 调用错误
}

// AgentRuntime 封装 AI Provider 调用
type AgentRuntime struct {
	provider base.AIProvider
}

// New 创建 AgentRuntime
func New(provider base.AIProvider) *AgentRuntime {
	return &AgentRuntime{provider: provider}
}

// Round 执行单轮推理：发送消息给 LLM，收集流式响应
func (r *AgentRuntime) Round(ctx context.Context, messages []base.Message, tools []base.ToolDef) *RoundResult {
	stream, err := r.provider.ChatStream(ctx, messages, tools)
	if err != nil {
		return &RoundResult{Error: fmt.Errorf("LLM call failed: %w", err)}
	}

	var contentBuf, reasoningBuf strings.Builder
	var toolCalls []base.ToolCall

	for ev := range stream {
		switch ev.Type {
		case "thinking":
			reasoningBuf.WriteString(ev.Content)
		case "content":
			contentBuf.WriteString(ev.Content)
		case "tool_call":
			toolCalls = append(toolCalls, base.ToolCall{
				ID:   fmt.Sprintf("call_%d", len(toolCalls)),
				Type: "function",
				Function: base.FunctionCall{
					Name:      ev.Tool,
					Arguments: ev.Args,
				},
			})
		case "error":
			return &RoundResult{Error: fmt.Errorf("LLM error: %s", ev.Content)}
		case "done":
			if ev.Usage != nil {
				return &RoundResult{
					Content:   contentBuf.String(),
					Reasoning: reasoningBuf.String(),
					ToolCalls: toolCalls,
					Usage:     ev.Usage,
				}
			}
		}
	}

	return &RoundResult{
		Content:   contentBuf.String(),
		Reasoning: reasoningBuf.String(),
		ToolCalls: toolCalls,
		Usage:     nil, // no usage info in stream
	}
}

// HasTools 检查是否有工具调用
func (rr *RoundResult) HasTools() bool {
	return len(rr.ToolCalls) > 0
}

// HasContent 检查是否有文本回复
func (rr *RoundResult) HasContent() bool {
	return len(rr.Content) > 0
}

// BestContent 返回最佳文本内容（优先 content，其次 reasoning）
func (rr *RoundResult) BestContent() string {
	if rr.Content != "" {
		return rr.Content
	}
	return rr.Reasoning
}
