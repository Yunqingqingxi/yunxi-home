package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yxd/yunxi-home/internal/ai/agent"
	"github.com/yxd/yunxi-home/internal/ai/async"
	"github.com/yxd/yunxi-home/internal/ai/base"
	"github.com/yxd/yunxi-home/internal/ai/coordinator"
	"github.com/yxd/yunxi-home/internal/ai/cron"
	"github.com/yxd/yunxi-home/internal/ai/goal"
	"github.com/yxd/yunxi-home/internal/ai/mcp"
	"github.com/yxd/yunxi-home/internal/ai/middleware"
	"github.com/yxd/yunxi-home/internal/ai/planner"
	"github.com/yxd/yunxi-home/internal/ai/register"
	"github.com/yxd/yunxi-home/internal/ai/session"
	"github.com/yxd/yunxi-home/internal/ai/skill"
	skill_builtin "github.com/yxd/yunxi-home/internal/ai/skill/builtin"
	"github.com/yxd/yunxi-home/internal/ai/todo"
	"github.com/yxd/yunxi-home/internal/database"
	"github.com/yxd/yunxi-home/internal/models"
	"github.com/yxd/yunxi-home/internal/toolreg"
)

// ── Types ──

type Service struct {
	provider        base.AIProvider
	registry        *register.Registry
	sessions        *session.Manager
	chain           *middleware.Chain
	planEngine      *planner.Engine
	budget          *session.BudgetManager
	metrics         *MetricsCollector
	planMode        bool
	goals           *goal.Manager
	coordinator     *coordinator.Coordinator
	injections      map[string]chan InjectedMessage
	asyncExec       *async.Executor
	injectMu        sync.RWMutex
	todoMgr         *todo.Manager
	cronMgr         *cron.Manager
	skillLoader     *skill.Loader
	skillRunner     *skill.Runner
	skillRegistry   *skill.Registry
	skillExecutor   *skill.Executor
	agentMgr        *agent.Manager
	mcpManager      *mcp.Manager
	mcpSubsystem    *mcp.Subsystem // new MCP subsystem (replaces direct manager usage)
	mcpCfgPath      string         // mcp.json 路径（可配置）
	confirmChannels      map[string]chan ConfirmResult
	confirmMu            sync.Mutex
	interactiveChannels  map[string]chan base.InteractiveResponse // 通用交互请求
	interactiveMu        sync.Mutex
	eventBus        *sessionEventBus
	activeStreams   map[string]context.CancelFunc // cancel func for active stream per session
	activeStreamsMu sync.Mutex
	chatLogger      *ChatLogger // 完整会话追踪日志
}

type ConfirmResult struct {
	Approved bool
	Fields   map[string]string
}

type InjectedMessage struct {
	Source    string    `json:"source"`
	Content   string    `json:"content"`
	Priority  string    `json:"priority"`
	Timestamp time.Time `json:"timestamp"`
}

// sessionEventBus buffers events per session and allows late subscribers to reconnect.
type sessionEventBus struct {
	mu       sync.RWMutex
	buffers  map[string]*eventBuffer
}

type eventBuffer struct {
	events   []base.ChatStreamEvent
	subs     map[chan base.ChatStreamEvent]struct{}
	maxBuf   int
}

func newSessionEventBus() *sessionEventBus {
	return &sessionEventBus{buffers: make(map[string]*eventBuffer)}
}

func (b *sessionEventBus) getOrCreate(sessionID string) *eventBuffer {
	b.mu.Lock()
	defer b.mu.Unlock()
	if eb, ok := b.buffers[sessionID]; ok {
		return eb
	}
	eb := &eventBuffer{subs: make(map[chan base.ChatStreamEvent]struct{}), maxBuf: 200}
	b.buffers[sessionID] = eb
	return eb
}

func (b *sessionEventBus) remove(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if eb, ok := b.buffers[sessionID]; ok {
		for ch := range eb.subs {
			close(ch)
		}
		delete(b.buffers, sessionID)
	}
}

// publish sends an event to all subscribers and appends to the ring buffer.
func (eb *eventBuffer) publish(ev base.ChatStreamEvent) {
	for ch := range eb.subs {
		select {
		case ch <- ev:
		default:
			// slow consumer — drop
		}
	}
	eb.events = append(eb.events, ev)
	if len(eb.events) > eb.maxBuf {
		eb.events = eb.events[len(eb.events)-eb.maxBuf:]
	}
}

// subscribe returns a channel that receives all buffered events followed by live events.
func (eb *eventBuffer) subscribe() chan base.ChatStreamEvent {
	ch := make(chan base.ChatStreamEvent, 128)
	eb.subs[ch] = struct{}{}
	// Replay buffered events
	events := make([]base.ChatStreamEvent, len(eb.events))
	copy(events, eb.events)
	go func() {
		for _, ev := range events {
			select {
			case ch <- ev:
			default:
				return
			}
		}
	}()
	return ch
}

func (eb *eventBuffer) unsubscribe(ch chan base.ChatStreamEvent) {
	delete(eb.subs, ch)
}

var _ base.ChatService = (*Service)(nil)

type ServiceConfig struct {
	EnablePlanMode  bool
	MaxTokens       int
	ReserveForReply int
	ReserveForTools int
	Coordinator     *coordinator.Coordinator
	CronRepo        cron.TaskRepository
	SkillsDir       string
	MCPConfigPath   string // mcp.json 路径，空则默认 "mcp.json"
	MetricsSaveFn   func(CounterSnapshot) // 可选：每 30s 持久化计数器快照
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{EnablePlanMode: true, MaxTokens: 128000, ReserveForReply: 4096, ReserveForTools: 16384, MCPConfigPath: "mcp.json"}
}

func NewService(provider base.AIProvider, reg *register.Registry, sessionRepo database.ChatSessionRepository, cfg ServiceConfig) *Service {
	svc := &Service{
		provider:        provider,
		registry:        reg,
		sessions:        session.NewManager(sessionRepo),
		chain:           middleware.NewChain(reg),
		planEngine:      planner.New(reg),
		budget:          session.NewBudgetManager(cfg.MaxTokens, cfg.ReserveForReply, cfg.ReserveForTools),
		metrics:         NewMetricsCollector(nil, cfg.MetricsSaveFn),
		planMode:        cfg.EnablePlanMode,
		goals:           goal.NewManager(),
		coordinator:     cfg.Coordinator,
		injections:      make(map[string]chan InjectedMessage),
		todoMgr:         todo.NewManager(),
		mcpCfgPath:   cfg.MCPConfigPath,
		confirmChannels:     make(map[string]chan ConfirmResult),
		interactiveChannels: make(map[string]chan base.InteractiveResponse),
		eventBus:        newSessionEventBus(),
		activeStreams:   make(map[string]context.CancelFunc),
		chatLogger:      NewChatLogger("log/chat"),
	}
	svc.asyncExec = async.New(3, svc.injectCallback, svc.progressCallback)
	svc.cronMgr = cron.NewManager(func(sessionID, prompt string) { svc.InjectMessage(sessionID, prompt) }, cfg.CronRepo)
	svc.skillRegistry = skill.NewRegistry()
	if cfg.SkillsDir != "" {
		if loader, err := skill.NewLoader(cfg.SkillsDir); err == nil {
			svc.skillLoader = loader
			svc.skillRunner = skill.NewRunner(reg, func(exec *skill.Execution) {})
			// 将 YAML 技能也注册到 Registry
			for _, name := range loader.All() {
				if m := loader.Get(name); m != nil {
					svc.skillRegistry.RegisterYAML(m)
				}
			}
		}
	}
	// 注册内置编程式技能
	skill_builtin.RegisterAll(svc.skillRegistry)
	// 创建 Executor（暂不依赖 MCPContext）
	svc.skillExecutor = skill.NewExecutor(svc.skillRegistry, nil)
	svc.skillExecutor.SetYAMLRunner(svc.skillRunner)
	svc.agentMgr = agent.NewManager(agent.ManagerConfig{
		MaxConcurrent: 5,
		MaxRounds:     100,
		AgentTimeout:  10 * time.Minute,
		Provider:      provider,
		Registry:      reg,
		CompletionFn: func(sessionID string, results []*agent.Result) {
			// 发射 agent_result SSE 事件 → 前端用 AgentBubble 渲染
			if svc.eventBus != nil {
				eb := svc.eventBus.getOrCreate(sessionID)
				for _, r := range results {
					summary := r.Summary
					if r.Status != agent.StatusDone { summary = r.Error }
					ev := base.ChatStreamEvent{
						Type:        "agent_result",
						AgentID:     r.AgentID,
						AgentGoal:   r.Goal,
						AgentStatus: string(r.Status),
						AgentRound:  r.Rounds,
						Content:     summary,
					}
					eb.publish(ev)
					// 写入对话日志
					svc.chatLogger.LogAgentResult(sessionID, r.AgentID, r.Goal, string(r.Status), summary, r.Rounds)
				}
			}
			// 同时注入文本摘要到会话历史
			var parts []string
			ok, fail := 0, 0
			for _, r := range results {
				if r.Status == agent.StatusDone {
					ok++
					parts = append(parts, fmt.Sprintf("✅ %s: %s", r.Goal, r.Summary))
				} else {
					fail++
					parts = append(parts, fmt.Sprintf("❌ %s: %s", r.Goal, r.Error))
				}
			}
			msg := fmt.Sprintf("[后台任务完成] %d/%d 成功\n%s", ok, ok+fail, strings.Join(parts, "\n"))
			svc.InjectWithPriority(sessionID, msg, "info", "agent_completion")
		},
	})
	// Periodic agent cleanup (every 5 min, remove agents older than 30 min)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			svc.agentMgr.Cleanup(30 * time.Minute)
		}
	}()
	return svc
}

func (s *Service) GetRegistry() *register.Registry      { return s.registry }
func (s *Service) PlanMode() bool                         { return s.planMode }
func (s *Service) GetMetrics() *MetricsCollector          { return s.metrics }
func (s *Service) SetMCPSubsystem(sub *mcp.Subsystem)     { s.mcpSubsystem = sub }
func (s *Service) GetMCPSubsystem() *mcp.Subsystem        { return s.mcpSubsystem }
func (s *Service) ChatLogger() *ChatLogger                  { return s.chatLogger }
func (s *Service) Shutdown()                                 { s.chatLogger.Close(); if s.mcpManager != nil { s.mcpManager.CloseAll() } }

// ── Goal ──

func (s *Service) GetActiveGoal(sessionID string) *base.GoalInfo {
	goals := s.goals.ListBySession(sessionID)
	for _, g := range goals {
		if g.Status == goal.StatusActive || g.Status == goal.StatusPending {
			return goalToInfo(g)
		}
	}
	return nil
}

func (s *Service) CreateGoal(sessionID, title, description string, steps []base.GoalStepInfo) string {
	gSteps := make([]goal.Step, len(steps))
	for i, s := range steps {
		gSteps[i] = goal.Step{ID: s.ID, Title: s.Title, Tool: s.Tool, Status: goal.StepPending}
	}
	id := fmt.Sprintf("goal_%s_%d", sessionID[:min(8, len(sessionID))], time.Now().Unix())
	s.goals.Create(id, title, description, sessionID, gSteps)
	return id
}

func (s *Service) AbandonGoal(goalID string) { s.goals.Abandon(goalID) }

// ── Cross-Session ──

func (s *Service) RegisterSession(sessionID, title string) {
	if s.coordinator != nil { s.coordinator.RegisterSession(sessionID, title) }
}
func (s *Service) UnregisterSession(sessionID string) {
	if s.coordinator != nil { s.coordinator.UnregisterSession(sessionID) }
}

func (s *Service) subscribeCrossSession(sessionID string, ch chan<- base.ChatStreamEvent) {
	if s.coordinator == nil { return }
	evCh := s.coordinator.Subscribe(sessionID, 16)
	go func() {
		for ev := range evCh {
			ch <- base.ChatStreamEvent{Type: "cross_session", CrossSession: &base.CrossSessionEvent{
				Type: ev.Type, SessionID: ev.SessionID, Resource: ev.Path, Message: ev.Message,
			}}
		}
	}()
}

func (s *Service) releaseAllLocks(sessionID string) {
	// locks released automatically via session close
}

// ReasoningFor returns the default reasoning depth for a given model.
func (s *Service) ReasoningFor(model string) string {
	if reg, ok := s.provider.(interface{ ReasoningFor(string) string }); ok {
		return reg.ReasoningFor(model)
	}
	return "medium"
}

// ── StreamChat ──

func (s *Service) StreamChat(ctx context.Context, sessionID, userMessage string, modelOpt ...string) <-chan base.ChatStreamEvent {
	ch := make(chan base.ChatStreamEvent, 64)
	eb := s.eventBus.getOrCreate(sessionID)

	// Guard: only one active StreamChat per session. Duplicate requests become injections.
	s.activeStreamsMu.Lock()
	if _, ok := s.activeStreams[sessionID]; ok {
		s.activeStreamsMu.Unlock()
		slog.Debug("会话已有活跃流，转为注入", "会话", sessionID)
		s.InjectUserMessage(sessionID, userMessage)
		// Return event bus channel so the client can still watch
		subCh := eb.subscribe()
		go func() {
			defer close(ch)
			for ev := range subCh {
				select {
				case ch <- ev:
				default:
				}
			}
		}()
		return ch
	}
	_, streamCancel := context.WithCancel(context.Background())
	s.activeStreams[sessionID] = streamCancel
	s.activeStreamsMu.Unlock()

	go func() {
		defer close(ch)
		defer func() {
			s.activeStreamsMu.Lock()
			delete(s.activeStreams, sessionID)
			s.activeStreamsMu.Unlock()
			streamCancel()
		}()
		defer s.releaseAllLocks(sessionID)
		defer s.closeInjectCh(sessionID)
		defer s.eventBus.remove(sessionID)
			s.chatLogger.LogSessionStart(sessionID)
			defer s.chatLogger.LogSessionEnd(sessionID)

		// emit sends to both the client channel and the event buffer for reconnection
		emit := func(ev base.ChatStreamEvent) {
			select {
			case ch <- ev:
			default:
			}
			eb.publish(ev)
		}
		startTime := time.Now()
		modelOverride := ""
		if len(modelOpt) > 0 { modelOverride = modelOpt[0] }
		bgCtx := context.Background()
		if modelOverride != "" {
			bgCtx = context.WithValue(bgCtx, base.ModelOverrideKey{}, modelOverride)
		}
		injectCh := s.getOrCreateInjectCh(sessionID)
		s.subscribeCrossSession(sessionID, ch)

		// ── Agent progress → SSE routing ──
		oldAgentFn := s.agentMgr.SetProgressFn(func(a *agent.SubAgent, eventType string) {
			ev := base.ChatStreamEvent{
				Type:        eventType,
				AgentID:     a.ID,
				AgentGoal:   a.Goal,
				AgentRound:  a.Round,
				AgentStatus: string(a.Status),
				Content:     a.Summary,
			}
			select {
			case ch <- ev:
			default:
			}
			eb.publish(ev)
		})
		defer s.agentMgr.SetProgressFn(oldAgentFn)
		// ── End agent progress routing ──

		history, st := s.sessions.GetOrCreate(sessionID, models.SessionTypeChat, userMessage)
		if resumePrompt := s.checkGoalResume(sessionID, userMessage); resumePrompt != "" {
		history = append(history, base.Message{Role: "system", Content: resumePrompt})
		}

	// Plan mode: guide LLM to use spawn_agent
	if planMode, _ := bgCtx.Value(base.PlanModeKey{}).(bool); planMode && planner.ShouldPlan(userMessage) {
		history = append(history, base.Message{Role: "system", Content: fmt.Sprintf(
			"## Plan Mode 已启用\n"+
				"请使用 spawn_agent 工具将任务分解为并行的子 Agent。\n"+
				"每个子 Agent 有独立上下文，专注执行其子任务。\n"+
				"- 可并行的独立子任务放入 tasks 数组\n"+
				"- 为每个子任务指定合适的 tool_filter\n"+
				"- 简单单一的问题无需分解，直接回答即可",
		)})
	}

	history = append(history, base.Message{Role: "user", Content: userMessage})
		s.chatLogger.LogUserMessage(sessionID, 0, userMessage)

		// ── 斜杠命令解析 ──
		if cmdName, cmdArgs, ok := parseSlashCommand(userMessage); ok {
			// /compact: AI 摘要 + 静默执行（不调 AI 回复）
			if cmdName == "compact" {
				s.handleCompactCommand(&history, sessionID, cmdArgs, bgCtx, emit, startTime)
				s.sessions.SetState(sessionID, models.SessionStateIdle, "", "")
				s.chatLogger.LogSessionSave(sessionID)
				return
			}
			// 其他内置命令：后台执行 + AI 简短确认
			if builtinCommandSet[cmdName] {
				if result := s.preExecCommand(sessionID, cmdName, cmdArgs); result != "" {
					slog.Info("斜杠命令预执行", "session", sessionID, "command", cmdName, "args", cmdArgs)
					history = append(history, base.Message{Role: "system", Content: result})
					s.chatLogger.LogInject(sessionID, 0, result)
				}
			}
		}

		allTools := s.registry.All()
		type qs struct{ round, maxRounds, loopCount, silentRounds int; lastToolSig string }
		state := qs{maxRounds: 100}
		silenceStart := time.Now()
		for state.round = 0; state.round < state.maxRounds; state.round++ {
			select {
			case msg := <-injectCh:
				if msg.Source == "user" {
					history = append(history, base.Message{Role: "user", Content: strings.TrimPrefix(msg.Content, "[用户消息] ")})
					s.chatLogger.LogUserMessage(sessionID, state.round, strings.TrimPrefix(msg.Content, "[用户消息] "))
				} else {
					history = append(history, base.Message{Role: "system", Content: msg.Content})
				}
				slog.Debug("消息注入", "会话", sessionID, "来源", msg.Source)
				s.chatLogger.LogInject(sessionID, state.round, msg.Content)
			default:
			}
			slog.Debug("调用LLM", "会话", sessionID, "轮次", state.round, "历史消息数", len(history), "工具数", len(allTools))
			s.chatLogger.LogRoundStart(sessionID, state.round, len(history), len(allTools))
			stream, err := s.provider.ChatStream(bgCtx, history, allTools)
			if err != nil {
				s.chatLogger.LogError(sessionID, state.round, "LLM调用失败: "+err.Error())
			slog.Error("LLM调用失败", "会话", sessionID, "轮次", state.round, "错误", err.Error())
				emit(base.ChatStreamEvent{Type: "error", Content: "AI 服务异常: " + err.Error()})
				s.sessions.Save(sessionID, history)
				return
			}
			var contentBuf, reasoningBuf strings.Builder
			var toolCalls []base.ToolCall
			var roundBlocks []base.ContentBlock
			for ev := range stream {
				switch ev.Type {
				case "thinking": reasoningBuf.WriteString(ev.Content); appendBlock(&roundBlocks, base.BlockTypeThinking, ev.Content, "", "", ""); emit(ev)
				case "content": contentBuf.WriteString(ev.Content); appendBlock(&roundBlocks, base.BlockTypeContent, ev.Content, "", "", ""); emit(ev)
				case "tool_call":
					tcID := fmt.Sprintf("call_%d_%d", state.round, len(toolCalls))
					toolCalls = append(toolCalls, base.ToolCall{ID: tcID, Type: "function", Function: base.FunctionCall{Name: ev.Tool, Arguments: ev.Args}})
					roundBlocks = append(roundBlocks, base.ContentBlock{Type: base.BlockTypeToolCall, ToolName: ev.Tool, ToolArgs: ev.Args, ToolCallID: tcID})
					emit(ev)
					toolRisk := ""
				if t, ok := s.registry.Get(ev.Tool); ok { toolRisk = t.RiskLevel }
				s.chatLogger.LogToolCall(sessionID, state.round, ev.Tool, ev.Args, toolRisk)
				case "done":
					if ev.Usage != nil {
						s.metrics.RecordTokens(int64(ev.Usage.PromptTokens), int64(ev.Usage.CompletionTokens), ev.Usage.Cost)
					}
					s.metrics.Record(MetricEvent{
						Timestamp: time.Now(), Type: "llm_request",
						Labels: map[string]string{"model": modelOverride, "session": sessionID, "status": "success"},
					})
				case "error":
					emit(ev)
					s.metrics.Record(MetricEvent{
						Timestamp: time.Now(), Type: "llm_request",
						Labels: map[string]string{"model": modelOverride, "session": sessionID, "status": "error"},
					})
					s.sessions.Save(sessionID, history); return
				}
			}
			// ── 批量记录本轮思考和正文（不在流中逐字记录）──
			if reasoningBuf.Len() > 0 { s.chatLogger.LogThinking(sessionID, state.round, reasoningBuf.String()) }
			if contentBuf.Len() > 0 { s.chatLogger.LogContent(sessionID, state.round, contentBuf.String()) }
			if len(toolCalls) > 0 {
			slog.Debug("检测到工具调用", "会话", sessionID, "轮次", state.round, "工具数", len(toolCalls),
						"AI内容", truncateStr(contentBuf.String(), 200), "思考长度", reasoningBuf.Len())
					for _, tc := range toolCalls {
						slog.Debug("工具调用详情", "会话", sessionID, "轮次", state.round, "工具", tc.Function.Name, "参数", truncateStr(tc.Function.Arguments, 300))
					}
			history = append(history, base.Message{Role: "assistant", Content: contentBuf.String(), ReasoningContent: reasoningBuf.String(), ToolCalls: toolCalls, HasToolCalls: true, Blocks: roundBlocks})
			for _, tc := range toolCalls {
			args, _ := register.ParseArgs(tc.Function.Arguments)
			tool, _ := s.registry.Get(tc.Function.Name)
			if tool != nil && tool.RiskLevel == "dangerous" {
			confirmID := fmt.Sprintf("confirm_%d_%d", state.round, len(toolCalls))
			detail := formatConfirmDetail(tc.Function.Name, args)
			s.sessions.SetState(sessionID, models.SessionStateWaitingUser, "", tc.Function.Name+":"+confirmID)
			emit(base.ChatStreamEvent{Type: "confirm_required", ConfirmRequest: &base.ConfirmRequest{
				ID:      confirmID,
				Tool:    tc.Function.Name,
				Message: detail,
			}})
			approved, _ := s.waitForConfirm(sessionID, confirmID)
			s.sessions.SetState(sessionID, models.SessionStateExecuting, "", "")
			if !approved {
				// 告知 AI 操作被取消，不阻塞后续流程
				obs := fmt.Sprintf("[%s 已取消] 用户未确认危险操作，工具未执行", tc.Function.Name)
				emit(base.ChatStreamEvent{Type: "tool_result", Tool: tc.Function.Name, Content: obs})
				history = append(history, base.Message{Role: "tool", Content: obs, ToolCallID: tc.ID})
				s.sessions.Save(sessionID, history)
				continue
			}
			}

			// Create tool context AFTER confirm (so it's not cancelled)
			toolCtx, toolCancel := context.WithCancel(bgCtx)
			toolCtx = todo.WithSessionID(toolCtx, sessionID)
			toolCtx = cron.WithSessionID(toolCtx, sessionID)
			toolCtx = agent.WithSessionID(toolCtx, sessionID)

			// Send tool_start event so frontend knows what's executing
			emit(base.ChatStreamEvent{Type: "tool_start", Tool: tc.Function.Name, Args: tc.Function.Arguments})
			toolStartTime := time.Now()
			slog.Debug("开始执行工具", "工具", tc.Function.Name)
				s.chatLogger.LogToolStart(sessionID, state.round, tc.Function.Name, tc.Function.Arguments)

			// Execute tool with heartbeat to prevent frontend timeout
			type toolDone struct {
			result *base.ToolResult
			}
			doneCh := make(chan toolDone, 1)
						go func() {
							doneCh <- toolDone{result: s.chain.Execute(toolCtx, tc.Function.Name, args)}
						}()

						// Heartbeat every 2s while tool runs — 附带 Agent 状态防止断连
						var result *base.ToolResult
						heartbeat := time.NewTicker(2 * time.Second)
						waitLoop:
						for {
						select {
						case d := <-doneCh:
						result = d.result
						break waitLoop
						case <-heartbeat.C:
						elapsed := int(time.Since(toolStartTime).Seconds())
						// 上报正在运行的子 Agent 状态
							runningAgents := s.agentMgr.ListAll()
							agentInfo := ""
							if len(runningAgents) > 0 {
								parts := make([]string, 0, len(runningAgents))
								for _, a := range runningAgents {
									if a.Status == "running" {
										parts = append(parts, fmt.Sprintf("%s:R%d", a.ID, a.Round))
									}
								}
								if len(parts) > 0 {
									agentInfo = fmt.Sprintf(" [%d sub-agent running: %s]", len(parts), strings.Join(parts, ", "))
								}
							}
							emit(base.ChatStreamEvent{Type: "tool_progress", Tool: tc.Function.Name, Content: fmt.Sprintf("执行中... (%ds)%s", elapsed, agentInfo), Args: fmt.Sprintf("%d", elapsed)})
							s.chatLogger.LogToolProgress(sessionID, state.round, tc.Function.Name, fmt.Sprintf("执行中... (%ds)", elapsed))
							case <-bgCtx.Done():
								toolCancel()
								result = &base.ToolResult{
									Status: base.StatusError,
									Error:  &base.ToolError{Code: base.ErrCodeTimeout, Message: "会话已断开", Retryable: false},
									Summary: fmt.Sprintf("[%s 执行中断] 客户端连接已断开", tc.Function.Name),
								}
								break waitLoop
							}
						}
						heartbeat.Stop()
						toolCancel()

						obs := formatObservation(tc.Function.Name, result)
					toolRiskLevel := ""
				if tool != nil { toolRiskLevel = tool.RiskLevel }
				s.chatLogger.LogToolResult(sessionID, state.round, tc.Function.Name, string(result.Status), obs, time.Since(toolStartTime).Milliseconds(), toolRiskLevel)
						elapsed := time.Since(toolStartTime).Seconds()
						toolStatus := "success"
						if result.Status == base.StatusError { toolStatus = "error" }
						s.metrics.Record(MetricEvent{
							Timestamp: time.Now(), Type: "tool_call",
							Labels: map[string]string{"tool": tc.Function.Name, "session": sessionID, "status": toolStatus},
							Value: elapsed,
						})
						slog.Debug("工具执行完成", "工具", tc.Function.Name, "状态", string(result.Status), "结果", truncateStr(obs, 200))
						emit(base.ChatStreamEvent{Type: "tool_result", Tool: tc.Function.Name, Content: obs})
						if len(history) > 0 && history[len(history)-1].Role == "assistant" {
							history[len(history)-1].Blocks = append(history[len(history)-1].Blocks, base.ContentBlock{Type: base.BlockTypeToolResult, ToolName: tc.Function.Name, ToolResult: obs, ToolCallID: tc.ID})
						}
						history = append(history, base.Message{Role: "tool", Content: obs, ToolCallID: tc.ID})
						// Save after each tool so crash doesn't lose results
						s.sessions.Save(sessionID, history)
					}
					// 静默检测：无用户可见文本时累计，≥2轮或>30秒自动注入进度
				if contentBuf.Len() == 0 {
					state.silentRounds++
				} else {
					state.silentRounds = 0
					silenceStart = time.Now()
				}
				if state.silentRounds >= 2 || time.Since(silenceStart) > 30*time.Second {
					progressMsg := fmt.Sprintf("[系统] AI 正在后台执行任务（第 %d 轮，已静默 %.0f 秒）。你可以发送「进度」查询状态，或发送新指令打断。",
						state.round, time.Since(silenceStart).Seconds())
					s.InjectWithPriority(sessionID, progressMsg, "info", "silence_detector")
					state.silentRounds = 0
					silenceStart = time.Now()
				}
				s.chatLogger.LogRoundEndSimple(sessionID, state.round, contentBuf.String(), len(roundBlocks))
					continue
				}
			finalContent := contentBuf.String()
			if finalContent == "" { finalContent = reasoningBuf.String() }
			if finalContent == "" { finalContent = "抱歉，我没有生成回复。" }
			history = append(history, base.Message{Role: "assistant", Content: finalContent, ReasoningContent: reasoningBuf.String(), Blocks: roundBlocks})
			s.chatLogger.LogAnswer(sessionID, state.round, finalContent)
			s.chatLogger.LogRoundEnd(sessionID, state.round, finalContent, len(roundBlocks), "completed")
			slog.Debug("对话轮次结束", "会话", sessionID, "总轮次", state.round, "AI回复", truncateStr(finalContent, 300), "块数量", len(roundBlocks))
			if st.Title == "" || st.Title == "新对话" { s.sessions.UpdateTitle(sessionID, truncateStr(userMessage, 40)) }
			s.chatLogger.LogSessionSave(sessionID)
			// 发射总耗时事件（前端显示用）
			totalDur := time.Since(startTime).Milliseconds()
			emit(base.ChatStreamEvent{Type: "done", Content: fmt.Sprintf("%d", totalDur)})
			s.sessions.SetState(sessionID, models.SessionStateIdle, "", "")
			s.sessions.Save(sessionID, history)
			s.sessions.SetState(sessionID, models.SessionStateIdle, "", "")
			return
		}
		s.chatLogger.LogError(sessionID, state.round, "操作超过最大轮次限制")
		emit(base.ChatStreamEvent{Type: "error", Content: "操作超过最大轮次限制"})
		s.sessions.Save(sessionID, history)
		s.chatLogger.LogSessionSave(sessionID)
	}()
	return ch
}

// ── Injection ──

func (s *Service) getOrCreateInjectCh(sessionID string) chan InjectedMessage {
	s.injectMu.Lock(); defer s.injectMu.Unlock()
	if ch, ok := s.injections[sessionID]; ok { return ch }
	ch := make(chan InjectedMessage, 32)
	s.injections[sessionID] = ch
	return ch
}

func (s *Service) closeInjectCh(sessionID string) {
	s.injectMu.Lock(); defer s.injectMu.Unlock()
	if ch, ok := s.injections[sessionID]; ok { close(ch); delete(s.injections, sessionID) }
}

func (s *Service) InjectMessage(sessionID, content string) {
	s.injectMu.RLock(); ch, ok := s.injections[sessionID]; s.injectMu.RUnlock()
	if !ok { return }
	select {
	case ch <- InjectedMessage{Content: content, Priority: "info", Timestamp: time.Now()}:
	default:
	}
}

func (s *Service) InjectWithPriority(sessionID, content, priority, source string) {
	s.injectMu.RLock(); ch, ok := s.injections[sessionID]; s.injectMu.RUnlock()
	if !ok { return }
	select {
	case ch <- InjectedMessage{Content: content, Priority: priority, Source: source, Timestamp: time.Now()}:
	default:
	}
}

func (s *Service) InjectUserMessage(sessionID, content string) {
	sessionType := models.SessionTypeChat
	if strings.HasPrefix(sessionID, "qqbot_") {
		sessionType = models.SessionTypeQQBot
	}
	history, _ := s.sessions.GetOrCreate(sessionID, sessionType, content)
	history = append(history, base.Message{Role: "user", Content: content})
	s.sessions.Save(sessionID, history)
	s.InjectWithPriority(sessionID, "[用户消息] "+content, "info", "user")
}

// SaveSystemMessage 直接将系统消息持久化到会话历史（不依赖 StreamChat injectCh）
func (s *Service) SaveSystemMessage(sessionID, content string) {
	history, _ := s.sessions.GetOrCreate(sessionID, models.SessionTypeChat, "")
	if len(history) > 0 && history[len(history)-1].Role == "assistant" && history[len(history)-1].Blocks != nil {
		history[len(history)-1].Blocks = append(history[len(history)-1].Blocks, base.ContentBlock{Type: "content", Content: content})
	}
	history = append(history, base.Message{Role: "system", Content: content})
	s.sessions.Save(sessionID, history)
	// 同时尝试通过 injectCh 推送给活跃流（如果存在的话）
	s.InjectWithPriority(sessionID, content, "info", "command_result")
}

// ── Confirm ──

func (s *Service) waitForConfirm(sessionID, confirmID string) (bool, map[string]string) {
	ch := make(chan ConfirmResult, 1)
	s.confirmMu.Lock(); s.confirmChannels[confirmID] = ch; s.confirmMu.Unlock()
	defer func() { s.confirmMu.Lock(); delete(s.confirmChannels, confirmID); s.confirmMu.Unlock() }()
	select {
	case result := <-ch: return result.Approved, result.Fields
	case <-time.After(60 * time.Second): return false, nil
	}
}

func (s *Service) ConfirmAction(confirmID string, approved bool, fields map[string]string) bool {
	s.confirmMu.Lock(); ch, ok := s.confirmChannels[confirmID]; s.confirmMu.Unlock()
	if !ok { return false }
	select {
	case ch <- ConfirmResult{Approved: approved, Fields: fields}: return true
	default: return false
	}
}

// RespondInteractive 前端响应交互式请求
func (s *Service) RespondInteractive(resp base.InteractiveResponse) bool {
	s.interactiveMu.Lock(); ch, ok := s.interactiveChannels[resp.ID]; s.interactiveMu.Unlock()
	if !ok { return false }
	select {
	case ch <- resp: return true
	default: return false
	}
}

// RequestUserInput 工具：AI 暂停并请求用户输入（弹窗）
func (s *Service) RequestUserInput(ctx context.Context, req base.InteractiveRequest) *base.InteractiveResponse {
	req.ID = fmt.Sprintf("interact_%d", time.Now().UnixNano())
	if req.TimeoutSec <= 0 { req.TimeoutSec = 120 }
	if req.Type == "" { req.Type = "confirm" }

	// 通过 event bus 发送（需要 sessionID）
	sessionID, _ := ctx.Value("session_id").(string)

	ch := make(chan base.InteractiveResponse, 1)
	s.interactiveMu.Lock()
	s.interactiveChannels[req.ID] = ch
	s.interactiveMu.Unlock()
	defer func() {
		s.interactiveMu.Lock()
		delete(s.interactiveChannels, req.ID)
		s.interactiveMu.Unlock()
	}()

	// 发送事件到当前活跃流
	s.activeStreamsMu.Lock()
	_, hasStream := s.activeStreams[sessionID]
	s.activeStreamsMu.Unlock()

	if hasStream {
		eb := s.eventBus.getOrCreate(sessionID)
		eb.publish(base.ChatStreamEvent{
			Type:               "interactive_request",
			InteractiveRequest: &req,
		})
	}

	// 等待响应或超时
	select {
	case resp := <-ch:
		return &resp
	case <-time.After(time.Duration(req.TimeoutSec) * time.Second):
		return &base.InteractiveResponse{ID: req.ID, Approved: false}
	case <-ctx.Done():
		return &base.InteractiveResponse{ID: req.ID, Approved: false}
	}
}

// ── Session CRUD ──

func (s *Service) ListSessions() []models.ChatSession                      { return s.sessions.List() }
func (s *Service) ListSessionsByType(sessionType string) []models.ChatSession { return s.sessions.ListByType(sessionType) }
func (s *Service) GetSessionHistory(sessionID string) (*models.ChatSessionDetail, error) { return s.sessions.GetHistory(sessionID) }
func (s *Service) ClearSession(sessionID string)                 { s.sessions.Delete(sessionID) }
func (s *Service) CompactSession(sessionID string) string          { return s.sessions.Compact(sessionID) }
func (s *Service) CompactWithSummary(sessionID, summary string, keepRecent int) string {
	return s.sessions.CompactWithSummary(sessionID, summary, keepRecent)
}
func (s *Service) ClearAllSessions()                             { s.sessions.DeleteAll() }

// ── /compact 命令处理 ──────────────────────────────────────────────

// handleCompactCommand executes the /compact command: generates an AI summary
// of old messages, replaces the in-memory history with the compacted version,
// and emits the result as SSE content events. Does NOT call the main AI loop.
func (s *Service) handleCompactCommand(history *[]base.Message, sessionID, cmdArgs string, bgCtx context.Context, emit func(base.ChatStreamEvent), startTime time.Time) {
	// Save user message first
	s.sessions.Save(sessionID, *history)

	// Parse arguments: /compact [N|hint]
	keepRecent := 6
	hint := ""
	if cmdArgs != "" {
		if n, err := strconv.Atoi(cmdArgs); err == nil && n > 0 && n <= 100 {
			keepRecent = n
		} else {
			hint = cmdArgs
		}
	}

	h := *history
	// Determine split point
	midEnd := len(h) - keepRecent
	if midEnd <= 1 {
		result := fmt.Sprintf("消息较少（%d 条），无需压缩", len(h)-1)
		emit(base.ChatStreamEvent{Type: "content", Content: result})
		emit(base.ChatStreamEvent{Type: "done", Content: fmt.Sprintf("%d", time.Since(startTime).Milliseconds())})
		return
	}

	// Separate old messages (to summarize) from recent ones (to keep)
	oldMsgs := make([]base.Message, midEnd-1)
	copy(oldMsgs, h[1:midEnd]) // skip system prompt (index 0)

	recentMsgs := make([]base.Message, len(h)-midEnd)
	copy(recentMsgs, h[midEnd:])

	// Generate AI summary
	summary, err := s.generateSummary(bgCtx, oldMsgs, hint)
	if err != nil {
		slog.Warn("AI摘要生成失败，使用文本降级方案", "error", err)
		summary = fallbackSummary(oldMsgs)
	}

	// Persist compaction marker to session (append-only)
	result := s.sessions.CompactWithSummary(sessionID, summary, keepRecent)

	// Replace in-memory history with compacted version for subsequent AI calls
	marker := base.Message{Role: "system", Content: "[上下文压缩摘要]\n" + summary}
	newHistory := make([]base.Message, 0, 2+len(recentMsgs))
	newHistory = append(newHistory, h[0]) // system prompt
	newHistory = append(newHistory, marker)
	newHistory = append(newHistory, recentMsgs...)
	*history = newHistory
	s.sessions.Save(sessionID, *history)

	// Log to chat logger
	s.chatLogger.LogCompaction(sessionID, len(oldMsgs), keepRecent, summary)

	// Emit result — no AI reply generated
	slog.Info("/compact 完成", "session", sessionID, "compacted", len(oldMsgs), "keep", keepRecent, "summary_len", len(summary))
	emit(base.ChatStreamEvent{Type: "content", Content: result})
	emit(base.ChatStreamEvent{Type: "done", Content: fmt.Sprintf("%d", time.Since(startTime).Milliseconds())})
}

// compactionSummaryPrompt is the system prompt for the AI summarization call.
const compactionSummaryPrompt = "你是一个对话摘要生成器。请用中文总结以下对话历史的关键信息。" +
	"\n要求：" +
	"\n1. 保留重要的事实、决策、文件路径、配置参数和技术细节" +
	"\n2. 保留用户的偏好和明确指示" +
	"\n3. 忽略问候、闲聊和重复内容" +
	"\n4. 输出 200-500 字的简洁摘要，不用编号列表，用自然段落表达" +
	"\n5. 只输出摘要文本，不要加「以下是摘要」之类的前缀"

// generateSummary calls the AI provider to generate a summary of the given messages.
func (s *Service) generateSummary(ctx context.Context, messages []base.Message, hint string) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("AI provider not available")
	}

	// Build the messages to send to the summarizer
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("用户：%s\n", msg.Content))
		case "assistant":
			content := msg.Content
			if len([]rune(content)) > 300 {
				content = string([]rune(content)[:300]) + "..."
			}
			sb.WriteString(fmt.Sprintf("助手：%s\n", content))
		case "tool":
			content := msg.Content
			if len([]rune(content)) > 150 {
				content = string([]rune(content)[:150]) + "..."
			}
			sb.WriteString(fmt.Sprintf("工具结果：%s\n", content))
		}
	}

	// Build system prompt with optional hint
	sysPrompt := compactionSummaryPrompt
	if hint != "" {
		sysPrompt += fmt.Sprintf("\n6. 用户特别要求：%s", hint)
	}

	summaryMsgs := []base.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: fmt.Sprintf("请总结以下对话历史：\n\n%s", sb.String())},
	}

	// Call AI (no tools, shorter timeout via context)
	summaryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stream, err := s.provider.ChatStream(summaryCtx, summaryMsgs, nil)
	if err != nil {
		return "", fmt.Errorf("summarization call failed: %w", err)
	}

	var summary strings.Builder
	for ev := range stream {
		if ev.Type == "content" {
			summary.WriteString(ev.Content)
		}
		if ev.Type == "error" {
			return "", fmt.Errorf("summarization error: %s", ev.Content)
		}
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		return "", fmt.Errorf("empty summary returned")
	}
	return result, nil
}

// fallbackSummary generates a simple text-based summary when AI summarization fails.
func fallbackSummary(messages []base.Message) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[文本摘要] 此前共 %d 条消息的关键内容：\n", len(messages)))

	added := 0
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		content := msg.Content
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("- 用户: %s\n", content))
			added++
		case "assistant":
			sb.WriteString(fmt.Sprintf("- 助手: %s\n", content))
			added++
		case "tool":
			if len([]rune(content)) > 100 {
				content = string([]rune(content)[:100]) + "..."
			}
			sb.WriteString(fmt.Sprintf("- 工具: %s\n", content))
			added++
		}
		if added >= 20 {
			sb.WriteString("- ...（更多内容已省略）\n")
			break
		}
	}
	return sb.String()
}

// GetHints 生成上下文相关的快捷提示（轻量 LLM 调用）
func (s *Service) GetHints(ctx context.Context, sessionID string) []string {
	fallback := []string{"查看系统状态", "管理文件", "检查 DNS", "Docker 状态"}
	if s.provider == nil {
		return fallback
	}
	tools := s.registry.All()
	toolNames := make([]string, len(tools))
	for i, t := range tools {
		toolNames[i] = t.Name
	}
	toolsList := strings.Join(toolNames, ", ")
	prompt := fmt.Sprintf(
		"[系统指令] 你是一个智能助手。根据以下可用工具列表，生成 4 个简短的自然语言快捷操作建议（每个不超过 10 个字），帮助用户快速上手。直接返回 4 行，不要编号，不要额外解释。\n\n可用工具: %s",
		toolsList,
	)
	messages := []base.Message{{Role: "user", Content: prompt}}
	stream, err := s.provider.ChatStream(ctx, messages, nil)
	if err != nil {
		return fallback
	}
	var content strings.Builder
	for ev := range stream {
		if ev.Type == "content" {
			content.WriteString(ev.Content)
		}
	}
	text := strings.TrimSpace(content.String())
	if text == "" {
		return fallback
	}
	lines := strings.Split(text, "\n")
	result := make([]string, 0, 4)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimLeft(l, "0123456789. )-•·")
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	if len(result) < 3 {
		return fallback
	}
	if len(result) > 5 {
		result = result[:5]
	}
	return result
}
func (s *Service) GetToolsJSON() []byte {
	tools := s.registry.All()
	data, _ := json.Marshal(tools)
	return data
}
func (s *Service) HasActiveSession(sessionID string) bool { return s.sessions.Get(sessionID) != nil }
func (s *Service) SubscribeSession(sessionID string) chan base.ChatStreamEvent {
	return s.eventBus.getOrCreate(sessionID).subscribe()
}
func (s *Service) UnsubscribeSession(sessionID string, ch chan base.ChatStreamEvent) {
	s.eventBus.getOrCreate(sessionID).unsubscribe(ch)
}
func (s *Service) ListActiveSessions() []string {
	var ids []string
	for _, s := range s.sessions.List() { ids = append(ids, s.ID) }
	return ids
}
func (s *Service) GetRawHistory(sessionID string) ([]map[string]any, error) {
	msgs, err := s.sessions.GetRawHistory(sessionID)
	if err != nil { return nil, err }
	return messagesToMaps(msgs), nil
}
func (s *Service) ForkAt(sessionID string, messageIndex int) ([]map[string]any, error) {
	msgs, err := s.sessions.ForkAt(sessionID, messageIndex, "")
	if err != nil { return nil, err }
	return messagesToMaps(msgs), nil
}
func (s *Service) EditMessage(sessionID string, messageIndex int, newContent string) ([]map[string]any, error) {
	msgs, err := s.sessions.ForkAt(sessionID, messageIndex, newContent)
	if err != nil { return nil, err }
	return messagesToMaps(msgs), nil
}

// ── Budget ──

func (s *Service) budgetCompactInPlace(history *[]base.Message) {
	if s.budget == nil || len(*history) <= 15 {
		return
	}
	wrapped := make([]session.MessageWithContent, len(*history))
	for i := range *history {
		wrapped[i] = session.MessageWithContent{
			Role:    (*history)[i].Role,
			Content: (*history)[i].Content + (*history)[i].ReasoningContent,
		}
	}
	if !s.budget.NeedsCompact(wrapped) {
		return
	}
	compacted := s.budget.CompactHistory(wrapped)
	newHistory := make([]base.Message, len(compacted))
	for i := range compacted {
		newHistory[i] = base.Message{
			Role:    compacted[i].Role,
			Content: compacted[i].Content,
		}
	}
	*history = newHistory
	slog.Debug("会话已压缩", "原消息数", len(wrapped), "压缩后", len(newHistory))
}

// ── Background ──

func (s *Service) injectCallback(sessionID, content, priority string) {
	s.InjectWithPriority(sessionID, content, priority, "async_executor")
}
func (s *Service) progressCallback(sessionID string, ev async.ProgressEvent) {}

func (s *Service) SubmitBackground(tool, sessionID string, args map[string]any, runner func(ctx *async.TaskContext) (string, error)) string {
	return s.asyncExec.Submit(tool, sessionID, args, runner)
}
func (s *Service) ListBackgroundTasks(sessionID string) []*async.Task { return s.asyncExec.ListBySession(sessionID) }
func (s *Service) CancelBackgroundTask(taskID string) error           { return s.asyncExec.Cancel(taskID) }
func (s *Service) ListCronTasks(sessionID string) []*cron.ScheduledTask { return s.cronMgr.ListBySession(sessionID) }
func (s *Service) DeleteCronTask(taskID string) bool                   { return s.cronMgr.Delete(taskID) }
// RegisterAgentTools 注册子 Agent、Todo、Cron、Skill 工具。
// 必须在 NewService 之后、StreamChat 之前调用。
func (s *Service) RegisterAgentTools() {
	// spawn_agent — 派生并行子 Agent
	s.registry.Register(agent.ToolDef(s.agentMgr))

	// todo_write — 创建/更新任务列表
	s.registry.Register(todo.ToolDef(s.todoMgr, func(sessionID string, lst *todo.List) {
		s.InjectMessage(sessionID, fmt.Sprintf("todos updated: %d items", len(lst.Items)))
	}))

	// cron_create / cron_delete / cron_list
	s.registry.Register(cron.CreateTool(s.cronMgr))
	s.registry.Register(cron.DeleteTool(s.cronMgr))
	s.registry.Register(cron.ListTool(s.cronMgr))

	// Skill 工具（支持编程式 + YAML）
	s.registry.Register(skill.ToolDefWithExecutor(s.skillLoader, s.skillRunner, s.skillExecutor))
	s.registry.Register(skill.ListToolDefWithExecutor(s.skillLoader, s.skillExecutor))

	// reload_mcp — AI 可调用此工具重载 MCP
	s.registry.Register(&base.ToolDef{
		Name:        "reload_mcp",
		Description: "重载 mcp.json 配置，重新连接所有 MCP 服务器并注册其工具。安装新 MCP 服务器后必须调用。",
		Category:    "system",
		RiskLevel:   "readonly",
		Parameters:  base.ToolParams{Type: "object", Properties: map[string]base.ParamProp{}},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			if err := s.ReloadMCPTools(s.mcpConfigPath()); err != nil {
				return "", fmt.Errorf("MCP 重载失败: %w", err)
			}
			return "MCP 工具已重载", nil
		},
	})

	// get_mcp_status — 查询所有 MCP 服务器的连接状态
	s.registry.Register(&base.ToolDef{
		Name:        "get_mcp_status",
		Description: "查询所有 MCP 服务器的连接状态。返回每个服务器的名称、连接状态（已连接/未连接）、可用工具数和错误信息。进程存在 ≠ 已连接：只有完成协议握手的服务器才标记为已连接。",
		Category:    "system",
		RiskLevel:   "readonly",
		Timeout:     5 * time.Second,
		Parameters:  base.ToolParams{Type: "object", Properties: map[string]base.ParamProp{}},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			sub := s.GetMCPSubsystem()
			if sub == nil {
				return "MCP 子系统未初始化", nil
			}
			sub.HealthCheck()
			servers := sub.List()
			if len(servers) == 0 {
				return "没有配置任何 MCP 服务器。使用 /get-mcp <关键词> 搜索可安装的服务器。", nil
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("MCP 服务器状态 (%d 个):\n\n", len(servers)))
			for _, st := range servers {
				icon := "❌"
				if st.Connected {
					icon = "✅"
				}
				sb.WriteString(fmt.Sprintf("%s %s (包: %s)\n", icon, st.Name, st.Package))
				if st.Connected {
					sb.WriteString(fmt.Sprintf("   状态: 已连接 | 工具: %d 个\n", st.Tools))
				} else {
					errInfo := st.Error
					if errInfo == "" {
						errInfo = "未完成协议握手（进程可能未启动或启动失败）"
					}
					sb.WriteString(fmt.Sprintf("   状态: 未连接 — %s\n", errInfo))
				}
			}
			return sb.String(), nil
		},
	})

	// request_confirmation — AI 请求用户确认危险操作
	s.registry.Register(&base.ToolDef{
		Name:        "request_confirmation",
		Description: "在执行高风险操作前请求用户确认。高风险操作包括：修改系统配置文件（/etc/、/opt/下文件，包括 /opt/yunxi-home/mcp.json）、使用 sudo 命令、安装/卸载系统软件包、修改数据库结构、删除重要数据。调用后系统弹出确认弹窗，用户可选择批准或拒绝。批准后方可继续执行后续操作。⚠️ 禁止猜测用户凭据——需要密码/密钥时先用此工具询问用户。",
		Category:    "system",
		RiskLevel:   "mutation",
		Timeout:     120 * time.Second,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"title":       {Type: "string", Description: "确认弹窗标题，例如 '修改系统配置文件'"},
				"message":     {Type: "string", Description: "详细说明要执行的操作及影响范围"},
				"details":     {Type: "string", Description: "操作详情（可选），例如要执行的完整命令、要修改的文件路径等"},
				"variant":     {Type: "string", Description: "弹窗变体", Enum: []string{"danger", "warning", "info"}},
				"timeout_sec": {Type: "integer", Description: "等待用户确认的超时秒数，默认 120"},
			},
			Required: []string{"title", "message"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			title, _ := args["title"].(string)
			message, _ := args["message"].(string)
			details, _ := args["details"].(string)
			variant, _ := args["variant"].(string)
			timeoutSec := toolreg.GetInt(args, "timeout_sec", 120)

			if variant == "" {
				variant = "warning"
			}

			fullMsg := message
			if details != "" {
				fullMsg = message + "\n\n" + details
			}

			resp := s.RequestUserInput(ctx, base.InteractiveRequest{
				Type:       "confirm",
				Title:      title,
				Message:    fullMsg,
				TimeoutSec: timeoutSec,
				Variant:    variant,
			})
			if resp == nil || !resp.Approved {
				return "❌ 用户拒绝了确认请求。操作已取消。请勿重试，改为询问用户是否愿意更改方案或降低风险。", nil
			}
			return "✅ 用户已确认。你可以继续执行操作。注意：后续的 run_command 或 file_write 等操作可能会单独触发弹窗确认，这是正常的。", nil
		},
	})

	slog.Info("agent/cron/todo/skill tools registered")
}

// LoadMCPTools 从 mcp.json 加载 MCP 服务器配置，连接并注册其工具。
// ReloadSkills 热重载技能：重新读取 YAML 并刷新工具注册
func (s *Service) ReloadSkills() error {
	if s.skillLoader == nil {
		return fmt.Errorf("skill loader not initialized")
	}
	if err := s.skillLoader.Reload(); err != nil {
		return err
	}
	if s.skillRunner != nil {
		s.registry.Register(skill.ToolDef(s.skillLoader, s.skillRunner))
		s.registry.Register(skill.ListToolDef(s.skillLoader))
	}
	slog.Info("技能已热重载", "count", len(s.skillLoader.All()))
	return nil
}

// GetMCPServer 搜索 MCP 服务器并返回安装指引（实际安装由 AI 执行）
func (s *Service) GetMCPServer(ctx context.Context, query string) string {
	if query == "" || query == "help" {
		return "用法: /get-mcp <关键词>\n\n" +
			"在 npm 市场搜索 MCP 服务器。\n" +
			"示例: /get-mcp filesystem | github | postgres | puppeteer"
	}
	results, err := toolreg.SearchMCPMarket(query)
	if err != nil {
		return err.Error()
	}

	// 精确名称匹配 → 返回安装指引
	for _, r := range results {
		if strings.EqualFold(r.Name, query) || results[0].Score >= 90 {
			// 检查参数
			params, hasParams := toolreg.DetectRequiredParams(r.Name)
			if hasParams {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("📦 %s\n\n安装步骤：\n1. npm install -g %s\n2. 在 mcp.json 添加配置：\n", r.Name, r.Name))
				sb.WriteString(fmt.Sprintf(`  "servers": {"%s": {"command": "npx", "args": ["-y", "%s"]`, strings.TrimPrefix(strings.TrimPrefix(r.Name, "@modelcontextprotocol/"), "server-"), r.Name))
				hasEnv := false
				for _, p := range params {
					if p.Required || p.Default != "" {
						if !hasEnv {
							sb.WriteString(", \"env\": {")
							hasEnv = true
						}
						val := p.Default
						if val == "" { val = "<你的" + p.Label + ">" }
						sb.WriteString(fmt.Sprintf("\"%s\": \"%s\"", p.Name, val))
					}
				}
				if hasEnv { sb.WriteString("}") }
				sb.WriteString("}}\n3. /reload-mcp\n\n需要配置的参数：\n")
				for _, p := range params {
					sb.WriteString(fmt.Sprintf("  • %s=%s — %s\n", p.Name, p.Default, p.Description))
				}
				return sb.String()
			}
			return fmt.Sprintf("📦 %s\n\n用以下命令安装：\n1. npm install -g %s\n2. 添加到 mcp.json\n3. /reload-mcp", r.Name, r.Name)
		}
	}

	return toolreg.FormatMCPSearchResults(results, query)
}

// mcpConfigPath returns the configured MCP config path, or the default.
func (s *Service) mcpConfigPath() string {
	if s.mcpCfgPath != "" {
		return s.mcpCfgPath
	}
	return "mcp.json"
}

// ReloadMCPTools 热重载 MCP 工具：重新读取 mcp.json，重新连接并注册工具
func (s *Service) ReloadMCPTools(configPath string) error {
	if configPath == "" {
		configPath = s.mcpConfigPath()
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取 mcp.json 失败: %w", err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析 mcp.json 失败: %w", err)
	}
	if s.mcpManager != nil {
		s.mcpManager.CloseAll()
	}
	s.mcpManager = mcp.NewManager()
	for name, svr := range cfg.MCPServers {
		s.mcpManager.AddServer(mcp.ServerConfig{Name: name, Command: svr.Command, Args: svr.Args, Env: svr.Env})
	}
	if err := s.mcpManager.ConnectAll(); err != nil {
		return err
	}
	mcp.RegisterTools(s.mcpManager, MCPRegistryAdapter{s.registry})
	slog.Info("MCP 工具已热重载", "count", len(cfg.MCPServers))
	return nil
}

// ListSkills 返回已加载的技能名→简介映射
func (s *Service) ListSkills() map[string]string {
	// 优先从 Registry 获取（包含编程式 + YAML）
	if s.skillRegistry != nil {
		return s.skillRegistry.ListAll()
	}
	// 回退到旧 Loader
	if s.skillLoader == nil {
		return nil
	}
	m := make(map[string]string)
	for _, name := range s.skillLoader.All() {
		sk := s.skillLoader.Get(name)
		if sk != nil {
			m[name] = sk.Description
		}
	}
	return m
}

// RunSkill 执行指定技能，返回结果文本
func (s *Service) RunSkill(ctx context.Context, name string) string {
	// 1. 尝试 Executor（编程式技能优先）
	if s.skillExecutor != nil && s.skillRegistry.Has(name) {
		result, err := s.skillExecutor.Run(ctx, name, nil)
		if err != nil {
			return fmt.Sprintf("技能 '%s' 执行失败: %v", name, err)
		}
		return fmt.Sprintf("[%s]\n%v", name, result)
	}
	// 2. 回退到 YAML Loader
	if s.skillLoader == nil || s.skillRunner == nil {
		return fmt.Sprintf("技能 '%s' 不存在。发送 /help 查看可用指令或 /list-skills 查看技能列表", name)
	}
	sk := s.skillLoader.Get(name)
	if sk == nil {
		return fmt.Sprintf("技能 '%s' 不存在。发送 /help 查看可用指令或 /list-skills 查看技能列表", name)
	}
	exec := s.skillRunner.Execute(ctx, sk)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【%s】\n", sk.Name))
	for i, step := range exec.Steps {
		status := "✅"
		if step.Error != "" {
			status = fmt.Sprintf("❌ %s", step.Error)
		}
		sb.WriteString(fmt.Sprintf("  %d. %s %s\n", i+1, step.Purpose, status))
	}
	return sb.String()
}

// CreateSkill 使用 AI 根据描述自动生成技能 YAML 并保存
func (s *Service) CreateSkill(ctx context.Context, description string) (string, error) {
	if s.provider == nil || s.skillLoader == nil {
		return "", fmt.Errorf("AI 或技能系统未启用")
	}
	existing := ""
	for name, desc := range s.ListSkills() {
		existing += fmt.Sprintf("- %s: %s\n", name, desc)
	}
	prompt := fmt.Sprintf("根据以下描述生成一个技能 YAML。直接输出 YAML，不要代码块标记。\n现有技能（避免重名）：\n%s\n描述：%s\n格式：\nname: xxx\ndescription: xxx\ncategory: ops\nrisk: readonly\nsteps:\n  - id: 1\n    tool: tool_name\n    purpose: 步骤说明\n    args:\n      key: value\n可用工具：%s",
		existing, description, strings.Join(s.toolNames(), ", "))

	messages := []base.Message{{Role: "user", Content: prompt}}
	stream, err := s.provider.ChatStream(ctx, messages, nil)
	if err != nil {
		return "", fmt.Errorf("AI 调用失败: %w", err)
	}
	var yamlBuf strings.Builder
	for ev := range stream {
		if ev.Type == "content" {
			yamlBuf.WriteString(ev.Content)
		}
	}
	yamlContent := strings.TrimSpace(yamlBuf.String())
	if yamlContent == "" {
		return "", fmt.Errorf("AI 未生成有效内容")
	}
	name := extractYAMLField(yamlContent, "name")
	if name == "" {
		return "", fmt.Errorf("生成的 YAML 缺少 name 字段")
	}
	filePath := fmt.Sprintf("skills/%s.yaml", name)
	if err := os.WriteFile(filePath, []byte(yamlContent+"\n"), 0644); err != nil {
		return "", fmt.Errorf("保存失败: %w", err)
	}
	if err := s.ReloadSkills(); err != nil {
		return "", fmt.Errorf("已保存但重载失败: %w", err)
	}
	return fmt.Sprintf("技能 '%s' 已创建。使用 /%s 执行。\n文件: %s", name, name, filePath), nil
}

func (s *Service) toolNames() []string {
	tools := s.registry.All()
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}

func extractYAMLField(yaml, key string) string {
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

func (s *Service) LoadMCPTools(configPath string) error {
	if configPath == "" {
		configPath = s.mcpConfigPath()
	}
	s.mcpManager = mcp.NewManager()

	// 读取 mcp.json
	data, err := os.ReadFile(configPath)
	if err != nil {
		slog.Info("MCP config not found, skipping", "path", configPath)
		return nil
	}

	var mcpConfig struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &mcpConfig); err != nil {
		return fmt.Errorf("parse mcp.json: %w", err)
	}

	for name, svr := range mcpConfig.MCPServers {
		s.mcpManager.AddServer(mcp.ServerConfig{Name: name, Command: svr.Command, Args: svr.Args})
	}
	if err := s.mcpManager.ConnectAll(); err != nil {
		slog.Warn("MCP connect warning", "error", err)
	}

	// 将 MCP 工具注册到 AI 注册表
	mcp.RegisterTools(s.mcpManager, MCPRegistryAdapter{s.registry})
	slog.Info("MCP tools loaded", "servers", len(mcpConfig.MCPServers))
	return nil
}

// MCPRegistryAdapter adapts register.Registry to mcp.ToolRegistry interface.
type MCPRegistryAdapter struct{ Reg *register.Registry }

func (a MCPRegistryAdapter) Register(name string, td *base.ToolDef) {
	td.Name = name
	a.Reg.Register(td)
}

func (s *Service) dispatchBackground(tool *base.ToolDef, funcName string, args map[string]any, sessionID string, ch chan<- base.ChatStreamEvent) bool {
	if !tool.Background { return false }
	runner := func(ctx *async.TaskContext) (string, error) { return tool.Handler(context.Background(), args) }
	id := s.SubmitBackground(funcName, sessionID, args, runner)
	ch <- base.ChatStreamEvent{Type: "background_task", TaskID: id, TaskStatus: "submitted", TaskMessage: "后台任务已提交: " + funcName}
	return true
}

// ── Helpers ──

func appendBlock(blocks *[]base.ContentBlock, typ base.ContentBlockType, content, toolName, toolArgs, toolCallID string) {
	if len(*blocks) > 0 && (*blocks)[len(*blocks)-1].Type == typ {
		(*blocks)[len(*blocks)-1].Content += content
		return
	}
	*blocks = append(*blocks, base.ContentBlock{Type: typ, Content: content, ToolName: toolName, ToolArgs: toolArgs, ToolCallID: toolCallID})
}

func messagesToMaps(msgs []base.Message) []map[string]any {
	result := make([]map[string]any, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]any{"role": m.Role, "content": m.Content, "reasoning_content": m.ReasoningContent, "tool_calls": m.ToolCalls, "has_tool_calls": m.HasToolCalls, "blocks": m.Blocks}
	}
	return result
}

func formatObservation(toolName string, result *base.ToolResult) string {
	if result.Status == base.StatusError && result.Error != nil {
		e := result.Error
		sb := strings.Builder{}
		sb.WriteString(fmt.Sprintf("[%s 执行失败]", toolName))
		sb.WriteString(fmt.Sprintf(" 错误码: %s", e.Code))
		sb.WriteString(fmt.Sprintf(" 详情: %s", e.Message))
		if e.Retryable {
			sb.WriteString(fmt.Sprintf(" (可重试: %s)", e.RetryHint))
		}
		if e.Fallback != "" {
			sb.WriteString(fmt.Sprintf(" 建议降级工具: %s", e.Fallback))
		}
		return sb.String()
	}
	summary := result.Summary
	if summary == "" {
		summary = fmt.Sprintf("[%s 执行成功]", toolName)
	}
	// Attach any error info even for partial status
	if result.Status == base.StatusPartial && result.Error != nil {
		summary += fmt.Sprintf(" (部分成功: %s)", result.Error.Message)
	}
	return summary
}

// ── Slash Command Pre-Execution ──────────────────────────────────────────

// parseSlashCommand parses a slash command from a message.
// Format: /<commandName> <arguments...>
// <commandName> must be [a-z0-9-]+ only.
// After <commandName>, must be space or end-of-string (e.g., /compactabc is NOT a command).
// Returns (commandName, args, isCommand).
func parseSlashCommand(message string) (name, args string, isCommand bool) {
	if !strings.HasPrefix(message, "/") {
		return "", "", false
	}
	rest := message[1:]
	cmdEnd := 0
	for cmdEnd < len(rest) && (isCmdChar(rest[cmdEnd])) {
		cmdEnd++
	}
	if cmdEnd == 0 {
		return "", "", false
	}
	name = strings.ToLower(rest[:cmdEnd])
	if cmdEnd < len(rest) {
		if rest[cmdEnd] != ' ' {
			return "", "", false
		}
		args = strings.TrimSpace(rest[cmdEnd+1:])
	}
	return name, args, true
}

func isCmdChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-'
}

// builtinCommandSet is the set of registered built-in slash command names.
var builtinCommandSet = map[string]bool{
	"help": true, "clear": true,
	"get-mcp": true, "reload-skills": true, "reload-mcp": true,
}

// preExecCommand executes a built-in slash command directly (bypassing AI)
// and returns a system message to inject into the conversation history.
func (s *Service) preExecCommand(sessionID, cmdName, cmdArgs string) string {
	switch cmdName {
	case "clear":
		if sessionID != "" {
			s.ClearSession(sessionID)
		}
		return "[系统] 用户执行了 /clear。会话已清空。请简短确认（如「会话已清空，有什么可以帮你的？」），不要追问或展开其他话题。"

	case "help":
		skills := s.ListSkills()
		var sb strings.Builder
		sb.WriteString("可用命令:\n  /help — 显示此帮助\n  /clear — 清空当前对话\n  /compact [提示] — 压缩对话上下文\n  /get-mcp <关键词> — 搜索 MCP 工具\n  /reload-skills — 重载技能\n  /reload-mcp — 重载 MCP 工具\n")
		if len(skills) > 0 {
			sb.WriteString("\n可用技能:\n")
			for name, desc := range skills {
				sb.WriteString(fmt.Sprintf("  /%s — %s\n", name, desc))
			}
		}
		return fmt.Sprintf("[系统] 用户执行了 /help。以下是可用命令和技能列表：\n\n%s\n\n请基于以上列表简短回复用户，告知可以使用这些命令和技能。不需要重复列出所有命令。", sb.String())

	case "reload-skills":
		if err := s.ReloadSkills(); err != nil {
			return fmt.Sprintf("[系统] 用户执行了 /reload-skills。重载失败：%v。请告知用户错误信息。", err)
		}
		return "[系统] 用户执行了 /reload-skills。技能已成功重新加载。请简短确认。"

	case "reload-mcp":
		if err := s.ReloadMCPTools(s.mcpConfigPath()); err != nil {
			return fmt.Sprintf("[系统] 用户执行了 /reload-mcp。重载失败：%v。请告知用户错误信息。", err)
		}
		return "[系统] 用户执行了 /reload-mcp。MCP 工具已成功重新加载。请简短确认。"

	case "get-mcp":
		if cmdArgs == "" {
			return "[系统] 用户执行了 /get-mcp 无参数。请提示用户提供搜索关键词，例如 /get-mcp filesystem 或 /get-mcp mysql。"
		}
		result := s.GetMCPServer(context.Background(), cmdArgs)
		return fmt.Sprintf("[系统] 用户执行了 /get-mcp %s。搜索结果：\n\n%s\n\n请基于以上结果简短回复用户，说明如何安装配置。", cmdArgs, result)

	default:
		return ""
	}
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen { return s }
	// Smart truncation: keep head 60% + [truncated] + tail 30%
	headLen := maxLen * 60 / 100
	tailLen := maxLen * 30 / 100
	if headLen+tailLen+20 > maxLen {
		headLen = maxLen - 20
		tailLen = 0
	}
	if tailLen > 0 {
		return string(runes[:headLen]) + fmt.Sprintf("...[truncated %d chars]...", len(runes)-maxLen) + string(runes[len(runes)-tailLen:])
	}
	return string(runes[:headLen]) + "..."
}

func (s *Service) checkGoalResume(sessionID, userMessage string) string {
	// Use the new state-aware resume prompt builder
	state, _, _ := s.sessions.GetState(sessionID)
	if state == models.SessionStateIdle { return "" }
	// For any non-idle state (waiting_user, executing_tool, interrupted),
	// generate a context-aware resume prompt
	return s.sessions.BuildResumePrompt(sessionID, userMessage)
}

func goalToInfo(g *goal.Goal) *base.GoalInfo {
	steps := make([]base.GoalStepInfo, len(g.Steps))
	for i, s := range g.Steps {
		steps[i] = base.GoalStepInfo{ID: s.ID, Title: s.Title, Tool: s.Tool, Status: string(s.Status)}
	}
	return &base.GoalInfo{ID: g.ID, Title: g.Title, Description: g.Description, Steps: steps, Status: string(g.Status)}
}

// formatConfirmDetail builds a human-readable description of a dangerous operation.
func formatConfirmDetail(toolName string, args map[string]any) string {
	switch toolName {
	case "file_delete":
		if p, ok := args["path"].(string); ok && p != "" {
			return fmt.Sprintf("删除文件: %s", p)
		}
		if paths, ok := args["paths"].([]any); ok && len(paths) > 0 {
			parts := make([]string, len(paths))
			for i, v := range paths {
				parts[i] = fmt.Sprintf("%v", v)
			}
			return fmt.Sprintf("批量删除 %d 个文件: %s", len(paths), strings.Join(parts, ", "))
		}
	case "run_command":
		if cmd, ok := args["command"].(string); ok && cmd != "" {
			return fmt.Sprintf("执行命令: %s", cmd)
		}
	case "docker_control_container":
		if name, ok := args["name"].(string); ok && name != "" {
			action, _ := args["action"].(string)
			return fmt.Sprintf("Docker 容器操作: %s → %s", name, action)
		}
	}
	return fmt.Sprintf("即将执行危险操作: %s", toolName)
}
