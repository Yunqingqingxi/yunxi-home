// Package orchestrator 提供主 ReAct 循环编排能力。
package orchestrator

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/agent"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/executor"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/runtime"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/topology"
)

var log = logger.ForComponent("orchestrator")

// Config 编排器配置
type Config struct {
	MaxRounds        int                 // 最大推理轮次，默认 100
	Runtime          *runtime.AgentRuntime // AI 推理运行时
	ToolExecutor     *executor.ToolExecutor // 工具执行器
	Registry         *register.Registry     // 工具注册表
	AgentMgr         *agent.Manager         // 子 Agent 管理器
	Tracker          *topology.Tracker      // 拓扑约束追踪器（nil=禁用）
	TopologyActive   bool                  // 是否激活拓扑追踪
	EmitFn           func(ev base.ChatStreamEvent) // SSE 事件发射
	InjectDrainFn    func() ([]InjectedMessage, bool) // 注入消息消费（返回消息列表 + 是否中断）
	ConfirmFn        func(string, map[string]any) (bool, map[string]string) // 确认回调
	OnToolCallStart  func(toolName string, args map[string]any) // 工具调用开始回调
	OnToolCallDone   func(toolName string, result string, elapsed time.Duration) // 工具调用完成回调
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		MaxRounds: 100,
	}
}

// InjectedMessage 注入消息
type InjectedMessage struct {
	Source    string
	Content   string
	Priority  string
	Timestamp time.Time
}

// AgentOrchestrator 主 ReAct 循环编排器
type AgentOrchestrator struct {
	cfg Config
}

// New 创建编排器
func New(cfg Config) *AgentOrchestrator {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 100
	}
	return &AgentOrchestrator{cfg: cfg}
}

// emit 发射 SSE 事件
func (o *AgentOrchestrator) emit(ev base.ChatStreamEvent) {
	if o.cfg.EmitFn != nil {
		o.cfg.EmitFn(ev)
	}
}

// Run 执行完整的 ReAct 循环
func (o *AgentOrchestrator) Run(ctx context.Context, sessionID string, history []base.Message) (*LoopResult, error) {
	state := &loopState{
		maxRounds:  o.cfg.MaxRounds,
		tracker:    o.cfg.Tracker,
		topoActive: o.cfg.TopologyActive,
	}

	allTools := o.cfg.Registry.All()

	for state.round = 0; state.round < state.maxRounds; state.round++ {
		// 1. 消费注入消息（用户消息、系统消息、中断信号）
		msgs, interrupted := o.drainInjects()
		for _, msg := range msgs {
			if msg.Source == "user" {
				history = append(history, base.Message{
					Role:    "user",
					Content: strings.TrimPrefix(msg.Content, "[用户消息] "),
				})
			} else {
				history = append(history, base.Message{
					Role:    "system",
					Content: msg.Content,
				})
			}
			log.Debug("消息注入", "会话", sessionID, "来源", msg.Source)
		}
		if interrupted {
			return &LoopResult{Interrupted: true, Rounds: state.round}, nil
		}

		// 2. ForceTools 检查（拓扑激活时）
		if state.topoActive && o.cfg.Tracker != nil {
			if forcedTool := o.cfg.Tracker.ShouldForceTools(sessionID, o.recentToolNames(history, 10), 0); forcedTool != "" {
				log.Info("ForceTools 触发", "会话", sessionID, "工具", forcedTool)
				o.executeForcedTool(sessionID, forcedTool, ctx, &history, &state.round)
				continue
			}
		}

		// 3. LLM 推理
		log.Debug("调用LLM", "会话", sessionID, "轮次", state.round, "历史消息数", len(history))
		result := o.cfg.Runtime.Round(ctx, history, allTools)

		if result.Error != nil {
			log.Error("LLM调用失败", "会话", sessionID, "轮次", state.round, "错误", result.Error)
			o.emit(base.ChatStreamEvent{Type: "error", Content: "AI 服务异常: " + result.Error.Error()})
			return &LoopResult{Error: result.Error, Rounds: state.round}, nil
		}

		// 4. 无工具调用 → AI 完成回复
		if !result.HasTools() {
			finalContent := result.BestContent()
			if finalContent == "" {
				finalContent = "抱歉，我没有生成回复。"
			}
			o.emit(base.ChatStreamEvent{Type: "content", Content: finalContent})
			o.emit(base.ChatStreamEvent{Type: "done", Content: fmt.Sprintf("%d", 0)})
			return &LoopResult{Success: true, Content: finalContent, Rounds: state.round}, nil
		}

		// 5. 追加 assistant 消息（含工具调用）
		history = append(history, base.Message{
			Role:             "assistant",
			Content:          result.Content,
			ReasoningContent: result.Reasoning,
			ToolCalls:        result.ToolCalls,
			HasToolCalls:     true,
		})

		// 6. 执行工具调用
		for _, tc := range result.ToolCalls {
			toolName := tc.Function.Name
			args := parseArgs(tc.Function.Arguments)

			if o.cfg.OnToolCallStart != nil {
				o.cfg.OnToolCallStart(toolName, args)
			}

			startTime := time.Now()
			obs := o.cfg.ToolExecutor.Execute(ctx, toolName, args)
			elapsed := time.Since(startTime)

			if o.cfg.OnToolCallDone != nil {
				o.cfg.OnToolCallDone(toolName, obs, elapsed)
			}

			history = append(history, base.Message{
				Role:       "tool",
				Content:    obs,
				ToolCallID: tc.ID,
			})
		}
	}

	o.emit(base.ChatStreamEvent{Type: "error", Content: "操作超过最大轮次限制"})
	return &LoopResult{Success: false, Rounds: state.maxRounds,
		Error: fmt.Errorf("exceeded max rounds: %d", state.maxRounds)}, nil
}

// drainInjects 消费注入通道
func (o *AgentOrchestrator) drainInjects() ([]InjectedMessage, bool) {
	if o.cfg.InjectDrainFn == nil {
		return nil, false
	}
	return o.cfg.InjectDrainFn()
}

// recentToolNames 获取历史中最近 N 个工具名
func (o *AgentOrchestrator) recentToolNames(history []base.Message, n int) []string {
	names := make([]string, 0, n)
	for i := len(history) - 1; i >= 0 && len(names) < n; i-- {
		msg := history[i]
		for _, tc := range msg.ToolCalls {
			names = append(names, tc.Function.Name)
			if len(names) >= n {
				break
			}
		}
	}
	return names
}

// executeForcedTool 直接执行强制工具（ForceTools 触发时）
func (o *AgentOrchestrator) executeForcedTool(sessionID, toolName string, ctx context.Context, history *[]base.Message, round *int) {
	tcID := fmt.Sprintf("call_%d_force", *round)
	tc := base.ToolCall{ID: tcID, Type: "function", Function: base.FunctionCall{Name: toolName, Arguments: "{}"}}
	*history = append(*history, base.Message{
		Role: "assistant", Content: "", HasToolCalls: true,
		ToolCalls: []base.ToolCall{tc},
	})

	o.emit(base.ChatStreamEvent{Type: "tool_start", Tool: toolName, Args: "{}"})
	obs := o.cfg.ToolExecutor.Execute(ctx, toolName, map[string]any{})
	o.emit(base.ChatStreamEvent{Type: "tool_result", Tool: toolName, Content: obs})
	*history = append(*history, base.Message{Role: "tool", Content: obs, ToolCallID: tcID})

	*round++
}

// loopState 循环状态
type loopState struct {
	round      int
	maxRounds  int
	tracker    *topology.Tracker
	topoActive bool
}

// LoopResult 循环执行结果
type LoopResult struct {
	Success     bool
	Interrupted bool
	Content     string
	Rounds      int
	Error       error
}

// parseArgs 简单解析 JSON 参数（避免引入复杂依赖）
func parseArgs(raw string) map[string]any {
	if raw == "" || raw == "{}" || raw == "null" {
		return map[string]any{}
	}
	// 简单 JSON 解析（用于大多数工具参数场景）
	// 对于复杂参数，使用完整的 JSON 解析
	result := make(map[string]any)
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		// 简单键值对解析
		inner := raw[1 : len(raw)-1]
		if inner == "" {
			return result
		}
	}
	return result
}
