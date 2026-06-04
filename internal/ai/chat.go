package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/adapt"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/agent"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/async"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/query"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/coordinator"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/cron"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/goal"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/intent"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/memory"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/mcp"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/middleware"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/planner"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/session"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/skill"
	skill_builtin "github.com/Yunqingqingxi/yunxi-home/internal/ai/skill/builtin"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/todo"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/topology"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/toolreg"
)

var log = logger.ForComponent("ai")

// ── Types ──

type Service struct {
	provider            base.AIProvider
	registry            *register.Registry
	sessions            *session.Manager
	chain               *middleware.Chain
	queryClient         *query.Client     // 统一单轮查询客户端
	planEngine          *planner.Engine
	budget              *session.BudgetManager
	metrics             *MetricsCollector
	planMode            bool
	goals               *goal.Manager
	coordinator         *coordinator.Coordinator
	injections          map[string]chan InjectedMessage
	asyncExec           *async.Executor
	injectMu            sync.RWMutex
	todoMgr             *todo.Manager
	cronMgr             *cron.Manager
	skillLoader         *skill.Loader
	skillRunner         *skill.Runner
	skillRegistry       *skill.Registry
	skillExecutor       *skill.Executor
	agentMgr            *agent.Manager
	mcpManager          *mcp.Manager
	mcpSubsystem        *mcp.Subsystem    // new MCP subsystem (replaces direct manager usage)
	mcpCfgPath          string            // mcp.json 路径（可配置）
	tracker             *topology.Tracker // 拓扑约束追踪器
	promptStore         *base.PromptStore // 提示词外置存储+缓存
	memoryManager       *memory.Manager   // 持久记忆管理器
	intentPipeline      *intent.Pipeline  // 意图路由管线（nil=禁用）
	adaptLayer          *adapt.Layer      // 用户适应层（nil=禁用）
	topoActive          map[string]bool   // 会话级拓扑激活状态
	topoMu              sync.RWMutex
	seenResults         map[string]map[uint64]struct{} // sessionID -> set of tool result hashes (info gain dedup)
	seenMu              sync.Mutex
	confirmChannels     map[string]chan ConfirmResult
	confirmMu           sync.Mutex
	interactiveChannels map[string]chan base.InteractiveResponse // 通用交互请求
	interactiveMu       sync.Mutex
	eventBus            *sessionEventBus
	activeStreams       map[string]context.CancelFunc        // cancel func for active stream per session
	activeStreamChs     map[string]chan base.ChatStreamEvent // active SSE channel per session (for tool-originated events)
	activeStreamsMu     sync.Mutex
	chatLogger          *ChatLogger // 完整会话追踪日志
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
	mu      sync.RWMutex
	buffers map[string]*eventBuffer
}

type eventBuffer struct {
	events []base.ChatStreamEvent
	subs   map[chan base.ChatStreamEvent]struct{}
	maxBuf int
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
	MCPConfigPath   string                // mcp.json 路径，空则默认 "mcp.json"
	MetricsSaveFn   func(CounterSnapshot) // 可选：每 30s 持久化计数器快照
	Tracker         *topology.Tracker     // 拓扑约束追踪器（nil=禁用）
	PromptStore     *base.PromptStore     // 提示词外置存储（nil=用 Go 常量）
	IntentPipeline  *intent.Pipeline      // 意图路由管线（nil=禁用）
	MemoryManager   *memory.Manager       // 持久记忆管理器（nil=禁用）
	AdaptLayer      *adapt.Layer          // 用户适应层（nil=禁用）
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{EnablePlanMode: true, MaxTokens: 900000, ReserveForReply: 4096, ReserveForTools: 16384, MCPConfigPath: "mcp.json"}
}

func NewService(provider base.AIProvider, reg *register.Registry, sessionRepo database.ChatSessionRepository, cfg ServiceConfig) *Service {
	svc := &Service{
		provider:            provider,
		registry:            reg,
		sessions:            session.NewManager(sessionRepo),
		chain:               middleware.NewChain(reg),
		queryClient:         query.New(provider),
		planEngine:          planner.New(reg),
		budget:              session.NewBudgetManager(cfg.MaxTokens, cfg.ReserveForReply, cfg.ReserveForTools),
		metrics:             NewMetricsCollector(nil, cfg.MetricsSaveFn),
		planMode:            cfg.EnablePlanMode,
		goals:               goal.NewManager(),
		coordinator:         cfg.Coordinator,
		injections:          make(map[string]chan InjectedMessage),
		todoMgr:             todo.NewManager(),
		mcpCfgPath:          cfg.MCPConfigPath,
		tracker:             cfg.Tracker,
		promptStore:         cfg.PromptStore,
		memoryManager:       cfg.MemoryManager,
		intentPipeline:      cfg.IntentPipeline,
		adaptLayer:          cfg.AdaptLayer,
		topoActive:          make(map[string]bool),
		confirmChannels:     make(map[string]chan ConfirmResult),
		interactiveChannels: make(map[string]chan base.InteractiveResponse),
		eventBus:            newSessionEventBus(),
		activeStreams:       make(map[string]context.CancelFunc),
		activeStreamChs:     make(map[string]chan base.ChatStreamEvent),
		chatLogger:          NewChatLogger("log/chat"),
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
					if r.Status != agent.StatusDone {
						summary = r.Error
					}
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
			// 持久化到会话历史，确保刷新不丢失
			svc.SaveSystemMessage(sessionID, msg)
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
	// Wire all prompts through PromptStore (supports DB hot-reload)
	svc.sessions.SystemPromptFn = func(sessionID, userMessage string, recentToolCalls []string) string {
		var p string
		if svc.promptStore != nil {
			p = svc.promptStore.BuildSystemPrompt(sessionID, userMessage, recentToolCalls)
		}
		// 追加持久记忆摘要
		if svc.memoryManager != nil {
			if summary := svc.memoryManager.Summary(); summary != "" {
				p += "\n\n## 持久记忆\n" + summary
			}
		}
		// 追加用户适应层摘要（从缓存读取）
		if svc.adaptLayer != nil {
			// userID is not available in this callback, skip
		}
		return p
	}
	svc.sessions.QQBotPromptFn = func() string {
		return svc.buildQQBotPrompt()
	}
	// Register callback to feed tool execution results into the topology tracker
	svc.chain.SetResultCallback(func(ctx context.Context, toolName string, result *base.ToolResult) {
		if svc.tracker != nil {
			sessionID := ""
			if v := ctx.Value(base.SessionIDKey{}); v != nil {
				if sid, ok := v.(string); ok {
					sessionID = sid
				}
			}
			if sessionID != "" {
				success := result.Status != base.StatusError
				svc.tracker.RecordToolResult(sessionID, toolName, success, result.Metadata.DurationMs)
			}
		}
	})
	return svc
}

func (s *Service) GetRegistry() *register.Registry { return s.registry }
func (s *Service) PlanMode() bool                  { return s.planMode }
func (s *Service) GetMetrics() *MetricsCollector   { return s.metrics }

// buildQQBotPrompt constructs the QQ Bot prompt entirely from PromptStore (DB only).
func (s *Service) buildQQBotPrompt() string {
	if s.promptStore == nil {
		return ""
	}
	// QQ Bot uses general prompts + QQ-specific rules + memory
	general := s.promptStore.BuildGeneralPrompt()
	qqSpecific := s.promptStore.GetSpecializedPrompt("spec_qqbot")
	result := general
	if qqSpecific != "" {
		result += "\n" + qqSpecific
		log.Warn("QQBot提示词组装", "general_len", len(general), "qq_specific", true, "qq_len", len(qqSpecific))
	} else {
		log.Warn("QQBot提示词组装", "general_len", len(general), "qq_specific", false, "warning", "spec_qqbot 未找到")
	}
	// 追加持久记忆摘要
	if s.memoryManager != nil {
		if summary := s.memoryManager.SummarizeCompact(); summary != "" {
			result += "\n\n" + summary
		}
	}
	return result
}
func (s *Service) SetMCPSubsystem(sub *mcp.Subsystem) { s.mcpSubsystem = sub }
func (s *Service) GetMCPSubsystem() *mcp.Subsystem    { return s.mcpSubsystem }
func (s *Service) ChatLogger() *ChatLogger            { return s.chatLogger }
func (s *Service) Shutdown() {
	// Cancel all active LLM streams to prevent shutdown hang
	s.activeStreamsMu.Lock()
	for sid, cancel := range s.activeStreams {
		cancel()
		delete(s.activeStreams, sid)
	}
	s.activeStreamsMu.Unlock()
	log.Info("已取消所有活跃 LLM 流", "count", len(s.activeStreams))
	s.chatLogger.Close()
	if s.mcpManager != nil {
		s.mcpManager.CloseAll()
	}
}

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
	if s.coordinator != nil {
		s.coordinator.RegisterSession(sessionID, title)
	}
}
func (s *Service) UnregisterSession(sessionID string) {
	if s.coordinator != nil {
		s.coordinator.UnregisterSession(sessionID)
	}
}

func (s *Service) subscribeCrossSession(sessionID string, ch chan<- base.ChatStreamEvent) {
	if s.coordinator == nil {
		return
	}
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

func (s *Service) StreamChat(ctx context.Context, sessionID, userID, userMessage string, modelOpt ...string) <-chan base.ChatStreamEvent {
	ch := make(chan base.ChatStreamEvent, 64)
	eb := s.eventBus.getOrCreate(sessionID)

	// Guard: only one active StreamChat per session. Duplicate requests become injections.
	s.activeStreamsMu.Lock()
	if _, ok := s.activeStreams[sessionID]; ok {
		s.activeStreamsMu.Unlock()
		log.Debug("会话已有活跃流，转为注入", "会话", sessionID)
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
	s.activeStreamChs[sessionID] = ch
	s.activeStreamsMu.Unlock()

	go func() {
		defer close(ch)
		defer func() {
			s.activeStreamsMu.Lock()
			delete(s.activeStreams, sessionID)
			delete(s.activeStreamChs, sessionID)
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
		if len(modelOpt) > 0 {
			modelOverride = modelOpt[0]
		}
		bgCtx := context.Background()
		if modelOverride != "" {
			bgCtx = context.WithValue(bgCtx, base.ModelOverrideKey{}, modelOverride)
		}
		s.getOrCreateInjectCh(sessionID) // ensure injectCh exists before draining
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
			// Safety: ch may be closed when main stream ends before agent finishes.
			func() {
				defer func() { recover() }()
				select {
				case ch <- ev:
				default:
				}
			}()
			eb.publish(ev)

			// Persist agent result + auto-resume main agent
			if eventType == "agent_result" {
			status := "完成"
			if a.Status == agent.StatusError {
			status = "失败"
			}
			resultMsg := fmt.Sprintf("[子Agent %s] %s | 目标: %s | 结果: %s",
			a.ID, status, a.Goal, a.Summary)
			s.SaveSystemMessage(sessionID, resultMsg)
			 // 自动恢复主Agent：子Agent完成后注入消息触发AI继续处理
					if !s.HasActiveStream(sessionID) {
						log.Info("子Agent完成，自动恢复主Agent", "session", sessionID, "agent", a.ID)
						s.InjectMessage(sessionID, fmt.Sprintf("[系统] 子Agent %s 已完成，请继续处理。%s", a.ID, resultMsg))
					}
				}
		})
		defer s.agentMgr.SetProgressFn(oldAgentFn)
		// ── End agent progress routing ──

		sessionType := models.SessionTypeChat
		if strings.HasPrefix(sessionID, "qqbot_") {
			sessionType = models.SessionTypeQQBot
		}
		history, st := s.sessions.GetOrCreate(sessionID, sessionType, userMessage)
		if resumePrompt := s.checkGoalResume(sessionID, userMessage); resumePrompt != "" {
			history = append(history, base.Message{Role: "system", Content: resumePrompt})
		}

		// Plan mode: guide LLM to use spawn_agent
		if planMode, _ := bgCtx.Value(base.PlanModeKey{}).(bool); planMode && planner.ShouldPlan(userMessage) {
			history = append(history, base.Message{Role: "system", Content: fmt.Sprintf(
				"## Plan Mode 已启用\n" +
					"请使用 spawn_agent 工具将任务分解为并行的子 Agent。\n" +
					"每个子 Agent 有独立上下文，专注执行其子任务。\n" +
					"- 可并行的独立子任务放入 tasks 数组\n" +
					"- 为每个子任务指定合适的 tool_filter\n" +
					"- 简单单一的问题无需分解，直接回答即可",
			)})
		}

		history = append(history, base.Message{Role: "user", Content: userMessage})
		s.chatLogger.LogUserMessage(sessionID, 0, userMessage)

		// ── 意图路由（v3.2）──
		if s.intentPipeline != nil {
			result := s.intentPipeline.Route(bgCtx, userMessage)
			switch result.Stage {
			case "rule":
				injectMsg := fmt.Sprintf(
					"[系统指令] 用户意图已识别为工具 '%s'（匹配: %s, 置信度: %.0f%%）。"+
						"请先调用此工具处理用户请求。",
					result.Tool, result.Pattern, result.Strength*100,
				)
				history = append(history, base.Message{Role: "system", Content: injectMsg})
				log.Info("意图路由(规则)", "会话", sessionID, "工具", result.Tool,
					"模式", result.Pattern)
			case "llm":
				injectMsg := fmt.Sprintf(
					"[建议] 用户意图可能对应工具 '%s'，请考虑使用。如果判断不匹配可忽略。",
					result.Tool,
				)
				history = append(history, base.Message{Role: "system", Content: injectMsg})
				log.Info("意图路由(LLM)", "会话", sessionID, "工具", result.Tool)
			}
		}

		// ── 记忆匹配注入（v3.3）──
		// ── 专用提示词自动激活（关键词匹配）──
		if s.promptStore != nil {
			activated := s.promptStore.TryAutoActivate(sessionID, userMessage)
			if len(activated) > 0 {
				log.Warn("专用提示词已激活", "session", sessionID, "count", len(activated), "ids", strings.Join(activated, ","))
			}
		}

		if s.memoryManager != nil {
			matched := s.memoryManager.Match(userMessage)
			for _, mem := range matched {
				history = append(history, base.Message{
					Role:    "system",
					Content: fmt.Sprintf("[相关记忆: %s]\n%s", mem.Name, mem.Content),
				})
				log.Info("记忆匹配注入", "会话", sessionID, "记忆", mem.Name)
			}
		}

		// ── 用户适应层：首次用户消息时注入 profile 摘要 ──
		if s.adaptLayer != nil && userID != "" && len(history) <= 2 {
			profileSummary := s.adaptLayer.OnSessionStart(bgCtx, userID, userMessage)
			if profileSummary != "" {
				history = append(history, base.Message{
					Role:    "system",
					Content: profileSummary,
				})
				log.Info("适应层注入", "会话", sessionID, "用户", userID)
			}
			// Detect re-ask / correction
			if ev := s.adaptLayer.OnUserMessage(sessionID, userID, userMessage); ev != nil {
				log.Info("隐式反馈检测", "会话", sessionID, "类型", string(ev.Type))
				// Inject a hint for corrections
				if ev.Type == adapt.FeedbackCorrection {
					history = append(history, base.Message{
						Role:    "system",
						Content: "[系统提示] 用户指出之前的回复有误。请重新理解用户需求，给出正确的回答。",
					})
				}
			}
		}

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
					log.Info("斜杠命令预执行", "session", sessionID, "command", cmdName, "args", cmdArgs)
					history = append(history, base.Message{Role: "system", Content: result})
					s.chatLogger.LogInject(sessionID, 0, result)
				}
			}
		}

		// ── 主动激活拓扑追踪（不再依赖前端 API 惰性初始化）──
		if s.tracker != nil {
			s.topoMu.Lock()
			if !s.topoActive[sessionID] {
				s.tracker.InitSession(sessionID, topology.DefaultConstraint())
				s.topoActive[sessionID] = true
			}
			s.topoMu.Unlock()
		}

		allTools := s.registry.All()
		type qs struct {
			round, maxRounds, loopCount, silentRounds int
			lastToolSig                               string
			taskAbandonCount                          int
			taskStallCount                            int
			consecutiveEmptyResults                   int        // rounds where all tool results were empty/error
			noProgressToolRounds                      int        // rounds where topology X didn't increase
			consecutiveToolFailures                   int        // consecutive tool failures (reset on success)
			sameToolRepetitions                       int        // same tool repeated without progress
			lastToolName                              string     // last tool called
			infoGainHistory                           [5]float64 // rolling window of per-round avg info gain
			infoGainPtr                               int
			lastCoordX                                float64 // topology X from previous round
		}
		state := qs{maxRounds: 1000}
		_ = time.Now                                // silenceStart placeholder
		topoActive := s.isTopologyActive(sessionID) // 一次性读取，每轮复用
		for state.round = 0; state.round < state.maxRounds; state.round++ {
			// ── 优先级批量消费 injectCh（支持中断信号）──
			msgs, interrupted := s.drainInjectChNonBlocking(sessionID)
			for _, msg := range msgs {
				if msg.Priority == "interrupt" && msg.Source == "cancel_session" {
					continue // handled by interrupted flag below
				}
				if msg.Source == "user" {
					history = append(history, base.Message{Role: "user", Content: strings.TrimPrefix(msg.Content, "[用户消息] ")})
					s.chatLogger.LogUserMessage(sessionID, state.round, strings.TrimPrefix(msg.Content, "[用户消息] "))
				} else {
					history = append(history, base.Message{Role: "system", Content: msg.Content})
				}
				log.Debug("消息注入", "会话", sessionID, "来源", msg.Source, "优先级", msg.Priority)
				s.chatLogger.LogInject(sessionID, state.round, msg.Content)
			}
			if interrupted {
				snapshot := s.buildSnapshot(sessionID)
				s.saveSnapshot(sessionID)
				emit(base.ChatStreamEvent{
					Type:    "interrupted",
					Content: fmt.Sprintf("进度 %d%%，最后执行：%s", snapshot.Progress, snapshot.LastTask),
				})
				s.sessions.Save(sessionID, history)
				s.chatLogger.LogSessionSave(sessionID)
				return
			}

			// ── ForceTools 检查（拓扑激活时）──
			// 仅在 X 坐标确实停滞 2+ 轮时才触发，避免 AI 正常使用同工具（如分块读文件）被误打断
			log.Info("ForceTools guard check", "round", state.round, "noProgress", state.noProgressToolRounds, "topoActive", topoActive, "hasTracker", s.tracker != nil)
			if topoActive && s.tracker != nil && state.noProgressToolRounds >= 2 {
				if forcedTool := s.tracker.ShouldForceTools(sessionID, s.recentToolNames(history, 10), state.consecutiveToolFailures + state.sameToolRepetitions); forcedTool != "" {
					log.Info("ForceTools 触发", "会话", sessionID, "工具", forcedTool, "轮次", state.round)
					s.chatLogger.LogRoundStart(sessionID, state.round, len(history), len(allTools))
					// 直接执行强制工具，跳过 LLM 调用
					s.executeForcedTool(sessionID, forcedTool, bgCtx, emit, &history, state.round, &state.round)
					s.chatLogger.LogRoundEndSimple(sessionID, state.round, "[ForceTools: "+forcedTool+"]", 0)
					continue
				}
			}

			// ── 分层注入：拓扑状态 message[1] 与静态 prompt message[0] 分离 ──
			if topoActive && s.tracker != nil {
				history = s.ensureTopologyState(history, sessionID)
			}

			// ── ForceTools：进度过半且关键工具缺失时跳过 LLM ──
			if topoActive && s.tracker != nil && state.noProgressToolRounds >= 2 {
				recentTools := s.extractRecentToolNames(history, 10)
				if forceTool := s.tracker.ShouldForceTools(sessionID, recentTools, state.consecutiveToolFailures + state.sameToolRepetitions); forceTool != "" {
					trackerSt := s.tracker.GetState(sessionID)
					progress := 0.0
					if trackerSt != nil {
						progress = trackerSt.CurrentCoord.X
					}
					injectMsg := fmt.Sprintf(
						"[系统指令] 进度已达 %.1f/10，必须立即调用工具 '%s'。调用后给用户一句话说明执行了什么。",
						progress, forceTool,
					)
					history = append(history, base.Message{Role: "system", Content: injectMsg})
					log.Info("ForceTools触发", "会话", sessionID, "工具", forceTool,
						"进度", fmt.Sprintf("%.1f", progress))
				}
			}

			log.Debug("调用LLM", "会话", sessionID, "轮次", state.round, "历史消息数", len(history), "工具数", len(allTools))
			s.chatLogger.LogRoundStart(sessionID, state.round, len(history), len(allTools))
			stream, err := s.provider.ChatStream(bgCtx, history, allTools)
			if err != nil {
				s.chatLogger.LogError(sessionID, state.round, "LLM调用失败: "+err.Error())
				log.Error("LLM调用失败", "会话", sessionID, "轮次", state.round, "错误", err.Error())
				emit(base.ChatStreamEvent{Type: "error", Content: "AI 服务异常: " + err.Error()})
				s.sessions.Save(sessionID, history)
				return
			}
			var contentBuf, reasoningBuf strings.Builder
			var toolCalls []base.ToolCall
			var roundBlocks []base.ContentBlock
			for ev := range stream {
				switch ev.Type {
				case "thinking":
					reasoningBuf.WriteString(ev.Content)
					appendBlock(&roundBlocks, base.BlockTypeThinking, ev.Content, "", "", "")
					emit(ev)
					log.Debug("AI块输出", "session", sessionID, "round", state.round, "type", "thinking", "len", len([]rune(ev.Content)))
				case "content":
				contentBuf.WriteString(ev.Content)
				appendBlock(&roundBlocks, base.BlockTypeContent, ev.Content, "", "", "")
				emit(ev)
					log.Debug("AI块输出", "session", sessionID, "round", state.round, "type", "content", "len", len([]rune(ev.Content)))
						// 流式持久化：每 500 字符保存一次，防止刷新丢失 AI 回复内容
						contentRunes := []rune(contentBuf.String())
						if len(contentRunes)%500 < len([]rune(ev.Content)) {
							cp := make([]base.Message, len(history))
							copy(cp, history)
							cp = append(cp, base.Message{
								Role: "assistant", Content: contentBuf.String(),
								ReasoningContent: reasoningBuf.String(),
								HasToolCalls: false, Blocks: roundBlocks,
							})
							s.sessions.Save(sessionID, cp)
						}
				case "tool_call":
				tcID := fmt.Sprintf("call_%d_%d", state.round, len(toolCalls))
				toolCalls = append(toolCalls, base.ToolCall{ID: tcID, Type: "function", Function: base.FunctionCall{Name: ev.Tool, Arguments: ev.Args}})
				// Silent tools: don't record in blocks or emit to frontend
						if !isSilentTool(ev.Tool) {
							roundBlocks = append(roundBlocks, base.ContentBlock{Type: base.BlockTypeToolCall, ToolName: ev.Tool, ToolArgs: ev.Args, ToolCallID: tcID})
							emit(ev)
						}
					toolRisk := ""
					if t, ok := s.registry.Get(ev.Tool); ok {
						toolRisk = t.RiskLevel
					}
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
					s.sessions.Save(sessionID, history)
					return
				}
			}
			// ── 批量记录本轮思考和正文（不在流中逐字记录）──
			// ── 拓扑标签解析 + 过滤（从 AI 输出中提取 <topology> 并剥离）──
			parsedTopo := topology.ParseResult{}
			if topoActive {
				rawContent := contentBuf.String()
				parsedTopo = topology.ParseTopology(rawContent)
				if parsedTopo.Parsed {
					// 过滤：从内容中移除 <topology> 标签
					stripped := topology.StripTopologyTag(rawContent)
					contentBuf.Reset()
					contentBuf.WriteString(stripped)
					// 从 roundBlocks 中也清理 topology 标签
					s.stripTopoFromBlocks(&roundBlocks)
					log.Debug("拓扑标签已解析", "会话", sessionID, "坐标", fmt.Sprintf("(%.1f,%.2f,%.2f)", parsedTopo.Coord.X, parsedTopo.Coord.Y, parsedTopo.Coord.Z), "工具", parsedTopo.Tools)
				}
			}
			if reasoningBuf.Len() > 0 {
				s.chatLogger.LogThinking(sessionID, state.round, reasoningBuf.String())
			}
			if contentBuf.Len() > 0 {
				s.chatLogger.LogContent(sessionID, state.round, contentBuf.String())
			}
			if len(toolCalls) > 0 {
				log.Debug("检测到工具调用", "会话", sessionID, "轮次", state.round, "工具数", len(toolCalls),
					"AI内容", truncateStr(contentBuf.String(), 200), "思考长度", reasoningBuf.Len())
				for _, tc := range toolCalls {
					log.Debug("工具调用详情", "会话", sessionID, "轮次", state.round, "工具", tc.Function.Name, "参数", truncateStr(tc.Function.Arguments, 300))
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
							if !isSilentTool(tc.Function.Name) {
							 emit(base.ChatStreamEvent{Type: "tool_result", Tool: tc.Function.Name, Content: obs})
							}
						history = append(history, base.Message{Role: "tool", Content: obs, ToolCallID: tc.ID})
						s.sessions.Save(sessionID, history)
							continue
						}
					}

					// Create tool context AFTER confirm (so it's not cancelled)
					toolCtx, toolCancel := context.WithCancel(bgCtx)
					toolCtx = base.WithSessionID(toolCtx, sessionID)
					toolCtx = todo.WithSessionID(toolCtx, sessionID)
					toolCtx = cron.WithSessionID(toolCtx, sessionID)
					toolCtx = agent.WithSessionID(toolCtx, sessionID)

					// Send tool_start event so frontend knows what's executing
					emit(base.ChatStreamEvent{Type: "tool_start", Tool: tc.Function.Name, Args: tc.Function.Arguments})
					toolStartTime := time.Now()
					log.Debug("开始执行工具", "工具", tc.Function.Name)
					s.chatLogger.LogToolStart(sessionID, state.round, tc.Function.Name, tc.Function.Arguments)

					// ── 后台异步执行：长时间任务不阻塞对话流 ──
					isBg := false
					if tool, ok := s.registry.Get(tc.Function.Name); ok {
						if bg, _ := args["background"].(bool); bg {
							isBg = true
						}
						if tool.Background {
							isBg = true
						}
					}
					if isBg {
						tool, _ := s.registry.Get(tc.Function.Name)
						if tool != nil {
							s.dispatchBackground(tool, tc.Function.Name, args, sessionID, ch)
						}
						placeholder := fmt.Sprintf("[后台执行] 任务 '%s' 已提交，正在后台运行。完成后结果会自动注入对话。", tc.Function.Name)
						history = append(history, base.Message{Role: "tool", Content: placeholder, ToolCallID: tc.ID})
						s.sessions.Save(sessionID, history)
						state.round++
						continue
					}

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
								Status:  base.StatusError,
								Error:   &base.ToolError{Code: base.ErrCodeTimeout, Message: "会话已断开", Retryable: false},
								Summary: fmt.Sprintf("[%s 执行中断] 客户端连接已断开", tc.Function.Name),
							}
							break waitLoop
						}
					}
					heartbeat.Stop()
					toolCancel()

					obs := formatObservation(tc.Function.Name, result)
					toolRiskLevel := ""
					if tool != nil {
						toolRiskLevel = tool.RiskLevel
					}
					s.chatLogger.LogToolResult(sessionID, state.round, tc.Function.Name, string(result.Status), obs, time.Since(toolStartTime).Milliseconds(), toolRiskLevel)
					elapsed := time.Since(toolStartTime).Seconds()
					toolStatus := "success"
					if result.Status == base.StatusError {
						toolStatus = "error"
					}
					s.metrics.Record(MetricEvent{
						Timestamp: time.Now(), Type: "tool_call",
						Labels: map[string]string{"tool": tc.Function.Name, "session": sessionID, "status": toolStatus},
						Value:  elapsed,
					})
					// ── 用户适应层：记录工具执行结果 ──
					if s.adaptLayer != nil && userID != "" {
						success := result.Status != base.StatusError
						s.adaptLayer.OnToolResult(bgCtx, sessionID, userID, tc.Function.Name, success, result.Metadata.DurationMs, state.round)
					}
					log.Debug("工具执行完成", "工具", tc.Function.Name, "状态", string(result.Status), "结果", truncateStr(obs, 200))
					if !isSilentTool(tc.Function.Name) {
					emit(base.ChatStreamEvent{Type: "tool_result", Tool: tc.Function.Name, Content: obs})
					   if len(history) > 0 && history[len(history)-1].Role == "assistant" {
					   history[len(history)-1].Blocks = append(history[len(history)-1].Blocks, base.ContentBlock{Type: base.BlockTypeToolResult, ToolName: tc.Function.Name, ToolResult: obs, ToolCallID: tc.ID})
					 }
					}
					history = append(history, base.Message{Role: "tool", Content: obs, ToolCallID: tc.ID})
					// 追踪信息增益：累积本轮所有工具结果的增益值
					gain := s.computeInfoGain(obs, sessionID)
					state.infoGainHistory[state.infoGainPtr%5] = gain
					state.infoGainPtr++
					// Save after each tool so crash doesn't lose results
					s.sessions.Save(sessionID, history)
					// ── 拓扑计数器更新 ──
					if result.Status == base.StatusError {
						state.consecutiveToolFailures++
					} else {
						state.consecutiveToolFailures = 0
					}
					// 同工具重复检测：只计非 readonly 工具（连续读文件是正常行为）
					toolRisk := ""
					if t, _ := s.registry.Get(tc.Function.Name); t != nil {
						toolRisk = t.RiskLevel
					}
					if toolRisk != "readonly" && tc.Function.Name == state.lastToolName {
						state.sameToolRepetitions++
					} else {
						state.sameToolRepetitions = 0
					}
					state.lastToolName = tc.Function.Name
				}
				// 信息增益总结：计算本轮平均增益 + 更新连续空结果计数
				var roundAvgGain float64
				gainCount := 0
				for _, g := range state.infoGainHistory {
					if g > 0 || gainCount > 0 {
						roundAvgGain += g
						gainCount++
					}
				}
				if gainCount > 0 {
					roundAvgGain /= float64(gainCount)
				}
				if roundAvgGain < 0.2 && state.round > 0 {
					state.consecutiveEmptyResults++
				} else if roundAvgGain >= 0.2 {
					state.consecutiveEmptyResults = 0
				}
				// 静默检测：仅累计，不单独告警（纯工具会话是正常场景）
				if contentBuf.Len() == 0 {
					state.silentRounds++
				} else {
					state.silentRounds = 0
				}
				// ── 卡住检测：连续低信息增益 + 有工具调用 = 无效忙碌 ──
				if state.consecutiveEmptyResults >= 3 && len(toolCalls) > 0 {
					history = append(history, base.Message{Role: "system", Content: "[系统指令] 你已经连续多轮调用工具但没有获得新的有效信息。可能缺少关键前置信息（如URL、路径、权限）。" +
						"如果缺少只有用户才能提供的信息，请直接告知用户你遇到了什么障碍，并询问必要的信息。"})
					state.consecutiveEmptyResults = 0
					log.Warn("卡住检测触发", "会话", sessionID, "连续空结果轮数", 3)
				}
				// ── 拓扑验证 + SSE 事件发射 ──
				s.emitTopologyUpdate(sessionID, topoActive, parsedTopo, toolCalls, contentBuf.String(), state.round, emit)
				// 追踪拓扑进度速度：检测无进展的工具调用轮次
				if topoActive {
					// Track progress using estimated coordinate (works even without <topology> tag)
					currentX := parsedTopo.Coord.X
					if currentX == 0 {
						currentX = float64(state.round) * 0.5 // fallback estimate
					}
					if math.Abs(currentX-state.lastCoordX) < 0.01 {
						state.noProgressToolRounds++
					} else {
						state.noProgressToolRounds = 0
					}
					state.lastCoordX = currentX
				}
				// ── 拓扑反馈注入：告知 AI 当前状态 ──
			if topoActive {
				var hints []string
				if state.sameToolRepetitions >= 8 {
					hints = append(hints, fmt.Sprintf("【严重】同一工具'%s'已连续使用%d次！必须立即换用其他工具（如 file_list/file_read/web_search），并给用户文字反馈", state.lastToolName, state.sameToolRepetitions))
				} else if state.sameToolRepetitions >= 5 {
					hints = append(hints, fmt.Sprintf("同一工具已连续使用%d次，请换策略并输出文字告知用户进度", state.sameToolRepetitions))
				}
				if state.noProgressToolRounds >= 3 {
					hints = append(hints, fmt.Sprintf("进度停滞(X=%.1f)已%d轮，尝试不同方法", parsedTopo.Coord.X, state.noProgressToolRounds))
				}
				if state.consecutiveToolFailures >= 3 {
					hints = append(hints, fmt.Sprintf("连续%d次工具失败，检查参数或换工具", state.consecutiveToolFailures))
				}
				// 静默提示仅在叠加其他问题时才注入（纯工具会话是正常的）
				if state.silentRounds >= 5 && len(hints) > 0 {
					hints = append(hints, fmt.Sprintf("连续%d轮无文字输出", state.silentRounds))
				}
				// ── X 值修正提示（注入AI系统消息，不渲染到前端）──
				if s.tracker != nil {
					trackerSt := s.tracker.GetSession(sessionID)
					if trackerSt != nil && trackerSt.XCorrected && trackerSt.XCorrectedMsg != "" {
						history = append(history, base.Message{Role: "system", Content: "[拓扑反馈] " + trackerSt.XCorrectedMsg})
						trackerSt.XCorrected = false
						trackerSt.XCorrectedMsg = ""
						log.Info("X值修正已注入", "session", sessionID)
					}
				}
				if len(hints) > 0 {
					feedback := "[拓扑反馈] " + strings.Join(hints, "；") + "。"
					history = append(history, base.Message{Role: "system", Content: feedback})
					log.Info("拓扑反馈注入", "session", sessionID, "hints", strings.Join(hints, " | "))
				}
			}
			s.chatLogger.LogRoundEndSimple(sessionID, state.round, contentBuf.String(), len(roundBlocks))
			continue
		}
			finalContent := contentBuf.String()
			if finalContent == "" {
				finalContent = reasoningBuf.String()
			}
			if finalContent == "" {
				finalContent = "抱歉，我没有生成回复。"
			}

			// ── 任务完成度检测 ──
			// 仅当 AI 没有任何工具调用且输出为空时才拦截
			shouldIntercept := finalContent == "" && len(toolCalls) == 0
			isStuckWithTools := state.noProgressToolRounds >= 3 && len(toolCalls) > 0
			if completed, dist := s.checkCompletionDistance(sessionID); !completed &&
				(len(toolCalls) == 0 || isStuckWithTools) && state.taskAbandonCount < 3 &&
				shouldIntercept {
				state.taskAbandonCount++
				retryMsg := fmt.Sprintf(
					"[系统指令] 任务未完成(差距=%.2f，目标X=10 Y=0 Z=0，当前X=%.1f Y=%.2f Z=%.2f)。"+
						"你调用了工具但没有推进进度。换参数、换工具、换搜索词，或者——"+
						"如果缺少用户才能提供的信息，直接告知用户你遇到了什么障碍并请求帮助。第 %d/3 次机会。",
					dist, parsedTopo.Coord.X, parsedTopo.Coord.Y, parsedTopo.Coord.Z,
					state.taskAbandonCount,
				)
				history = append(history, base.Message{Role: "system", Content: retryMsg})
				log.Warn("弃疗拦截(几何)", "会话", sessionID, "距离", fmt.Sprintf("%.3f", dist),
					"次数", state.taskAbandonCount)
				s.chatLogger.LogRoundEnd(sessionID, state.round, finalContent+" [拦截]", len(roundBlocks), "abandon_blocked")
				state.round++
				continue
			}

			// ── 最终输出前距离检查 ──
			// 拓扑完成度信息不再追加到用户可见内容中，改为系统消息注入
			if topoActive && s.tracker != nil {
			if completed, dist := s.checkCompletionDistance(sessionID); !completed {
			history = append(history, base.Message{Role: "system", Content: fmt.Sprintf(
			"[拓扑反馈] 任务未完成(完成度 %.1f%%，差距 %.2f)。继续执行或告知用户当前状态。",
			 parsedTopo.Coord.X*10, dist,
			 )})
			 }
				}

			history = append(history, base.Message{Role: "assistant", Content: finalContent, ReasoningContent: reasoningBuf.String(), Blocks: roundBlocks})
			// ── 拓扑验证 + 闭环检查（最后一轮纯文本回复）──
			s.emitTopologyUpdate(sessionID, topoActive, parsedTopo, nil, finalContent, state.round, emit)
			s.chatLogger.LogAnswer(sessionID, state.round, finalContent)
			s.chatLogger.LogRoundEnd(sessionID, state.round, finalContent, len(roundBlocks), "completed")
			log.Debug("对话轮次结束", "会话", sessionID, "总轮次", state.round, "AI回复", truncateStr(finalContent, 300), "块数量", len(roundBlocks))
			if st.Title == "" || st.Title == "新对话" {
				// Fallback: first 15 chars of user message
				title := userMessage
				if runes := []rune(title); len(runes) > 15 {
					title = string(runes[:15])
				}
				s.sessions.UpdateTitle(sessionID, title)
				// Async: generate AI-summarized title (max 15 chars)
				go s.generateTitleAsync(sessionID, userMessage)
			}
			s.chatLogger.LogSessionSave(sessionID)
			// 发射总耗时事件（前端显示用）
			totalDur := time.Since(startTime).Milliseconds()
			emit(base.ChatStreamEvent{Type: "done", Content: fmt.Sprintf("%d", totalDur)})

			// ── 用户适应层：记录会话结束 ──
			// TODO: 实时上下文余量 — 接入 budget.NeedsCompact() + 精准 token 计数
			if s.adaptLayer != nil {
				uid := userID
				if uid == "" { uid = sessionID }
				taskCat := s.adaptLayer.InferTaskCategory(uid, userMessage)
				toolSuccesses := state.round - state.consecutiveToolFailures
				if toolSuccesses < 0 { toolSuccesses = 0 }
				trustLocked := false
				if s.tracker != nil {
					if trackerSt := s.tracker.GetState(sessionID); trackerSt != nil {
						trustLocked = trackerSt.Trust.Locked
					}
				}
				// TODO: budget.NeedsCompact() → context_window_used_pct
				summary := &adapt.SessionSummary{
					SessionID: sessionID, UserID: uid, TaskCategory: taskCat,
					Rounds: state.round, ToolCalls: state.round,
					ToolSuccesses: toolSuccesses, ToolFailures: state.consecutiveToolFailures,
					TokensIn: 0, TokensOut: 0, // TODO: 从 session state 取实际值
					TrustLocked: trustLocked, Completed: true,
					DurationSec: time.Since(startTime).Seconds(),
					Timestamp: time.Now().UTC(),
				}
				s.adaptLayer.OnSessionEnd(bgCtx, summary)
			}

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
	s.injectMu.Lock()
	defer s.injectMu.Unlock()
	if ch, ok := s.injections[sessionID]; ok {
		return ch
	}
	ch := make(chan InjectedMessage, 32)
	s.injections[sessionID] = ch
	return ch
}

func (s *Service) closeInjectCh(sessionID string) {
	s.injectMu.Lock()
	defer s.injectMu.Unlock()
	if ch, ok := s.injections[sessionID]; ok {
		close(ch)
		delete(s.injections, sessionID)
	}
}

func (s *Service) InjectMessage(sessionID, content string) {
	s.injectMu.RLock()
	ch, ok := s.injections[sessionID]
	s.injectMu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- InjectedMessage{Content: content, Priority: "system", Source: "inject", Timestamp: time.Now()}:
	default:
	}
}

func (s *Service) InjectWithPriority(sessionID, content, priority, source string) {
	s.injectMu.RLock()
	ch, ok := s.injections[sessionID]
	s.injectMu.RUnlock()
	if !ok {
		return
	}
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
	s.InjectWithPriority(sessionID, "[用户消息] "+content, "user", "user")
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
	s.confirmMu.Lock()
	s.confirmChannels[confirmID] = ch
	s.confirmMu.Unlock()
	defer func() { s.confirmMu.Lock(); delete(s.confirmChannels, confirmID); s.confirmMu.Unlock() }()
	select {
	case result := <-ch:
		return result.Approved, result.Fields
	case <-time.After(60 * time.Second):
		return false, nil
	}
}

func (s *Service) ConfirmAction(confirmID string, approved bool, fields map[string]string) bool {
	s.confirmMu.Lock()
	ch, ok := s.confirmChannels[confirmID]
	s.confirmMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- ConfirmResult{Approved: approved, Fields: fields}:
		return true
	default:
		return false
	}
}

// RespondInteractive 前端响应交互式请求
func (s *Service) RespondInteractive(resp base.InteractiveResponse) bool {
	s.interactiveMu.Lock()
	ch, ok := s.interactiveChannels[resp.ID]
	s.interactiveMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- resp:
		return true
	default:
		return false
	}
}

// RequestUserInput 工具：AI 暂停并请求用户输入（弹窗）
func (s *Service) RequestUserInput(ctx context.Context, req base.InteractiveRequest) *base.InteractiveResponse {
	req.ID = fmt.Sprintf("interact_%d", time.Now().UnixNano())
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = 120
	}
	if req.Type == "" {
		req.Type = "confirm"
	}

	// 通过 event bus 发送（需要 sessionID）
	sessionID, _ := ctx.Value(base.SessionIDKey{}).(string)

	ch := make(chan base.InteractiveResponse, 1)
	s.interactiveMu.Lock()
	s.interactiveChannels[req.ID] = ch
	s.interactiveMu.Unlock()
	defer func() {
		s.interactiveMu.Lock()
		delete(s.interactiveChannels, req.ID)
		s.interactiveMu.Unlock()
	}()

	// 发送事件到当前活跃 SSE 流（主通道 + event bus 重连通道）
	s.activeStreamsMu.Lock()
	streamCh, hasStream := s.activeStreamChs[sessionID]
	s.activeStreamsMu.Unlock()

	log.Info("interactive_request 发送", "sessionID", sessionID, "hasStream", hasStream, "reqID", req.ID,
			"type", req.Type, "hasActionURL", req.ActionURL != "", "actionURL", req.ActionURL)
	if !hasStream || streamCh == nil {
		log.Warn("interactive_request: 无活跃 SSE 通道，尝试 event bus fallback", "sessionID", sessionID)
		// fallback: 仅发送到 event bus
		if sessionID != "" {
			eb := s.eventBus.getOrCreate(sessionID)
			eb.publish(base.ChatStreamEvent{
				Type:               "interactive_request",
				InteractiveRequest: &req,
			})
		}
	} else {
		ev := base.ChatStreamEvent{
			Type:               "interactive_request",
			InteractiveRequest: &req,
		}
		// 主 SSE 通道（优先，直接到达前端）
		select {
		case streamCh <- ev:
			log.Info("interactive_request: 已发送到主 SSE 通道", "reqID", req.ID)
		default:
			log.Warn("interactive_request: 主 SSE 通道已满", "reqID", req.ID)
		}
		// event bus（重连订阅者也能收到）
		eb := s.eventBus.getOrCreate(sessionID)
		eb.publish(ev)
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

func (s *Service) ListSessions() []models.ChatSession { return s.sessions.List() }
func (s *Service) ListSessionsByType(sessionType string) []models.ChatSession {
	return s.sessions.ListByType(sessionType)
}
func (s *Service) GetSessionHistory(sessionID string) (*models.ChatSessionDetail, error) {
	return s.sessions.GetHistory(sessionID)
}
func (s *Service) ClearSession(sessionID string) { s.sessions.Delete(sessionID) }
func (s *Service) UpdateSessionMeta(sessionID string, title *string, pinned *bool) error {
	return s.sessions.UpdateMeta(sessionID, title, pinned)
}
func (s *Service) CompactSession(sessionID string) string { return s.sessions.Compact(sessionID) }
func (s *Service) CompactWithSummary(sessionID, summary string, keepRecent int) string {
	return s.sessions.CompactWithSummary(sessionID, summary, keepRecent)
}
func (s *Service) ClearAllSessions() { s.sessions.DeleteAll() }

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
		log.Warn("AI摘要生成失败，使用文本降级方案", "error", err)
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
	log.Info("/compact 完成", "session", sessionID, "compacted", len(oldMsgs), "keep", keepRecent, "summary_len", len(summary))
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

	// Call AI via unified query client (no tools, 30s timeout)
	result := s.queryClient.Chat(ctx, summaryMsgs, query.WithTimeout(30*time.Second))
	if result.Err != nil {
		return "", fmt.Errorf("summarization failed: %w", result.Err)
	}
	if result.Content == "" {
		return "", fmt.Errorf("empty summary returned")
	}
	// Track token usage
	if result.Usage != nil {
		s.metrics.RecordTokens(int64(result.Usage.PromptTokens), int64(result.Usage.CompletionTokens), result.Usage.Cost)
	}
	return result.Content, nil
}

// GenerateTitle generates a short conversation title from the first user message.
// Uses a lightweight prompt and returns a title of max 20 characters.
func (s *Service) GenerateTitle(ctx context.Context, sessionID, userMessage string) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("AI provider not available")
	}

	titleMsgs := []base.Message{
		{Role: "system", Content: "你是一个标题生成器。根据用户的第一条消息生成一个简短的对话标题。" +
			"规则：1) 不超过20个字 2) 只返回标题本身，不要引号、不要解释、不要标点 3) 提取核心话题"},
		{Role: "user", Content: userMessage},
	}

	result := s.queryClient.Chat(ctx, titleMsgs, query.WithTimeout(15*time.Second))
	if result.Err != nil {
		return "", fmt.Errorf("title generation failed: %w", result.Err)
	}

	title := strings.TrimSpace(result.Content)
	// Track token usage
	if result.Usage != nil {
		s.metrics.RecordTokens(int64(result.Usage.PromptTokens), int64(result.Usage.CompletionTokens), result.Usage.Cost)
	}
	// Strip quotes and excessive punctuation
	title = strings.Trim(title, "\"'「」\"\"''，。.！!？?：:；;、")
	if len([]rune(title)) > 20 {
		title = string([]rune(title)[:20])
	}
	if title == "" {
		title = "新对话"
	}

	// Update session title in DB
	s.sessions.UpdateTitle(sessionID, title)

	return title, nil
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
	result := s.queryClient.Chat(ctx, messages, query.WithTimeout(10*time.Second))
	if result.Err != nil || result.Content == "" {
		return fallback
	}
	// Track token usage
	if result.Usage != nil {
		s.metrics.RecordTokens(int64(result.Usage.PromptTokens), int64(result.Usage.CompletionTokens), result.Usage.Cost)
	}
	text := strings.TrimSpace(result.Content)
	if text == "" {
		return fallback
	}
	lines := strings.Split(text, "\n")
	hints := make([]string, 0, 4)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		l = strings.TrimLeft(l, "0123456789. )-•·")
		l = strings.TrimSpace(l)
		if l != "" {
			hints = append(hints, l)
		}
	}
	if len(hints) < 3 {
		return fallback
	}
	if len(hints) > 5 {
		hints = hints[:5]
	}
	return hints
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
	for _, s := range s.sessions.List() {
		ids = append(ids, s.ID)
	}
	return ids
}
func (s *Service) GetRawHistory(sessionID string) ([]map[string]any, error) {
	msgs, err := s.sessions.GetRawHistory(sessionID)
	if err != nil {
		return nil, err
	}
	return messagesToMaps(msgs), nil
}
func (s *Service) ForkAt(sessionID string, messageIndex int) ([]map[string]any, error) {
	msgs, err := s.sessions.ForkAt(sessionID, messageIndex, "")
	if err != nil {
		return nil, err
	}
	return messagesToMaps(msgs), nil
}
func (s *Service) EditMessage(sessionID string, messageIndex int, newContent string) ([]map[string]any, error) {
	msgs, err := s.sessions.ForkAt(sessionID, messageIndex, newContent)
	if err != nil {
		return nil, err
	}
	return messagesToMaps(msgs), nil
}

// ── Budget ──

func (s *Service) budgetCompactInPlace(history *[]base.Message) {
	if s.budget == nil || len(*history) <= 100 {
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
	origIdx := 0
	for i := range compacted {
		// 找到对应的原始消息，保留 ReasoningContent / ToolCalls / Blocks
		var orig base.Message
		if origIdx < len(*history) && (*history)[origIdx].Role == compacted[i].Role {
			orig = (*history)[origIdx]
			origIdx++
		}
		newHistory[i] = base.Message{
			Role:             compacted[i].Role,
			Content:          compacted[i].Content,
			ReasoningContent: orig.ReasoningContent,
			ToolCalls:        orig.ToolCalls,
			HasToolCalls:     orig.HasToolCalls,
			ToolCallID:       orig.ToolCallID,
			Blocks:           orig.Blocks,
		}
	}
	*history = newHistory
	log.Debug("会话已压缩", "原消息数", len(wrapped), "压缩后", len(newHistory))
}

// ── Background ──

func (s *Service) injectCallback(sessionID, content, priority string) {
	s.InjectWithPriority(sessionID, content, priority, "async_executor")
}
func (s *Service) progressCallback(sessionID string, ev async.ProgressEvent) {}

func (s *Service) SubmitBackground(tool, sessionID string, args map[string]any, runner func(ctx *async.TaskContext) (string, error)) string {
	return s.asyncExec.Submit(tool, sessionID, args, runner)
}
func (s *Service) ListBackgroundTasks(sessionID string) []*async.Task {
	return s.asyncExec.ListBySession(sessionID)
}
func (s *Service) CancelBackgroundTask(taskID string) error { return s.asyncExec.Cancel(taskID) }
func (s *Service) ListCronTasks(sessionID string) []*cron.ScheduledTask {
	return s.cronMgr.ListBySession(sessionID)
}
func (s *Service) DeleteCronTask(taskID string) bool { return s.cronMgr.Delete(taskID) }

// ── v3.1 拓扑约束 + 提示词管理 ──

// GetAllPromptSections 返回所有提示词（兼容旧 API 格式）
func (s *Service) GetAllPromptSections() map[string]interface{} {
	if s.promptStore == nil {
		return nil
	}
	prompts := s.promptStore.GetAllPrompts()
	result := make(map[string]interface{}, len(prompts))
	for _, p := range prompts {
		result[p.ID] = p
	}
	return result
}

// UpdatePromptSection 更新提示词并热重载。保留已有元数据（category/name/keywords/priority）。
func (s *Service) UpdatePromptSection(ctx context.Context, id, data string) error {
	if s.promptStore == nil {
		return fmt.Errorf("promptStore not initialized")
	}
	rec := database.PromptRecord{
		ID:       id,
		Content:  data,
		Enabled:  true,
		Category: "general",
		Priority: 5,
	}
	// 保留已有元数据
	if old := s.promptStore.GetPrompt(id); old != nil {
		rec.Category = old.Category
		rec.Name = old.Name
		rec.Keywords = old.Keywords
		rec.Priority = old.Priority
	}
	return s.promptStore.UpdatePrompt(ctx, rec)
}

// ResetPromptSection 重置提示词（删除 DB 记录，下次 seed 恢复）
func (s *Service) ResetPromptSection(ctx context.Context, id string) error {
	if s.promptStore == nil {
		return fmt.Errorf("promptStore not initialized")
	}
	return s.promptStore.DeletePrompt(ctx, id)
}

// GetTopologyState 返回会话的完整拓扑状态（始终返回有效默认值，即使 tracker 未初始化）
func (s *Service) GetTopologyState(sessionID string) interface{} {
	def := base.TopologyState{
		SessionID:    sessionID,
		CurrentCoord: base.Coordinate{X: 0, Y: 0, Z: 0},
		StartCoord:   base.Coordinate{X: 0, Y: 0, Z: 0},
		Constraint:   base.TopologyConstraint{A: 0.8, R: 3.0, T: false},
		Active:       true,
	}
	if s.tracker == nil {
		return &def
	}
	st := s.tracker.GetState(sessionID)
	if st == nil {
		// 惰性初始化：首次查询时用默认约束激活拓扑
		s.tracker.InitSession(sessionID, topology.DefaultConstraint())
		s.topoMu.Lock()
		s.topoActive[sessionID] = true
		s.topoMu.Unlock()
		st = s.tracker.GetState(sessionID)
		if st == nil {
			return &def
		}
	}
	// 转换 topology.SessionState → base.TopologyState
	traj := make([]base.TopologyNode, len(st.Trajectory))
	for i, n := range st.Trajectory {
		traj[i] = base.TopologyNode{
			X: n.Coord.X, Y: n.Coord.Y, Z: n.Coord.Z,
			Round: n.Round, ToolCall: n.ToolCall,
			Status: string(n.Status), Reason: n.Reason,
		}
	}
	return &base.TopologyState{
		SessionID:    st.SessionID,
		CurrentCoord: base.Coordinate{X: st.CurrentCoord.X, Y: st.CurrentCoord.Y, Z: st.CurrentCoord.Z},
		StartCoord:   base.Coordinate{X: st.StartCoord.X, Y: st.StartCoord.Y, Z: st.StartCoord.Z},
		Constraint: base.TopologyConstraint{
			A: st.Constraint.A, R: st.Constraint.R, T: st.Constraint.T,
			ForceTools: st.Constraint.ForceTools,
		},
		Trajectory:  traj,
		RejectCount: st.RejectCount,
		TrustLies:   st.Trust.Lies,
		TrustLocked: st.Trust.Locked,
		ClosedLoop:  st.ClosedLoop,
		ClosedDist:  st.ClosedDist,
		Warning:     st.Warning,
		Active:      st.Active,
	}
}

// UpdateConstraint 更新拓扑约束参数，同时激活该会话的拓扑追踪
func (s *Service) UpdateConstraint(sessionID string, a, r float64, t bool, forceTools []string) error {
	if s.tracker == nil {
		return fmt.Errorf("topology tracker not initialized")
	}
	// 首次约束更新 → 激活拓扑追踪
	s.activateTopology(sessionID)
	return s.tracker.UpdateConstraint(sessionID, a, r, t, forceTools)
}

// OverrideNextNode 放行下一轮拓扑校验
func (s *Service) OverrideNextNode(sessionID string, targetCoord interface{}) {
	if s.tracker == nil {
		return
	}
	var tc *topology.Coordinate
	if c, ok := targetCoord.(*base.Coordinate); ok && c != nil {
		tc = &topology.Coordinate{X: c.X, Y: c.Y, Z: c.Z}
	}
	s.tracker.OverrideNextNode(sessionID, tc)
}

// ResetTrust 重置信任状态（手动解锁）
func (s *Service) ResetTrust(sessionID string) {
	if s.tracker == nil {
		return
	}
	s.tracker.ResetTrust(sessionID)
}

// EditMessageTopology 编辑/插入消息并同步拓扑轨迹
func (s *Service) EditMessageTopology(sessionID, messageIndexStr, content string, insertMode bool) (interface{}, error) {
	msgIdx, err := strconv.Atoi(messageIndexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid message index: %s", messageIndexStr)
	}
	if insertMode {
		// 插入模式：在指定位置插入消息，保留后续上下文
		_, err := s.sessions.InsertAt(sessionID, msgIdx, content)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"message": "inserted"}, nil
	}
	// 编辑模式：从指定位置分叉，截断后续上下文
	msgs, forkErr := s.sessions.ForkAt(sessionID, msgIdx, content)
	if forkErr != nil {
		return nil, forkErr
	}
	deletedNodes := 0
	if s.tracker != nil {
		// 估算拓扑 round（每个 assistant+tool 轮次 ≈ 1 个拓扑节点）
		topoRound := estimateTopoRound(msgs)
		if n, err := s.tracker.Rebase(sessionID, topoRound); err == nil {
			deletedNodes = n
		}
	}
	return map[string]interface{}{"deleted_nodes": deletedNodes, "message": "ok"}, nil
}

// DeleteMessageTopology 删除消息并同步拓扑轨迹
func (s *Service) DeleteMessageTopology(sessionID, messageIndexStr string) (interface{}, error) {
	msgIdx, err := strconv.Atoi(messageIndexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid message index: %s", messageIndexStr)
	}
	msgs, forkErr := s.sessions.ForkAt(sessionID, msgIdx+1, "")
	if forkErr != nil {
		return nil, forkErr
	}
	deletedNodes := 0
	if s.tracker != nil {
		topoRound := estimateTopoRound(msgs)
		if n, err := s.tracker.Rebase(sessionID, topoRound); err == nil {
			deletedNodes = n
		}
	}
	return map[string]interface{}{"deleted_nodes": deletedNodes, "message": "ok"}, nil
}

// ── 拓扑辅助 ──

// activateTopology 激活会话的拓扑追踪（幂等）
func (s *Service) activateTopology(sessionID string) {
	s.topoMu.Lock()
	defer s.topoMu.Unlock()
	if s.topoActive[sessionID] {
		return
	}
	s.topoActive[sessionID] = true
	if s.tracker != nil {
		s.tracker.InitSession(sessionID, topology.DefaultConstraint())
	}
}

// isTopologyActive 查询会话拓扑是否激活
func (s *Service) isTopologyActive(sessionID string) bool {
	s.topoMu.RLock()
	defer s.topoMu.RUnlock()
	return s.topoActive[sessionID]
}

// estimateTopoRound 根据消息历史估算当前拓扑轮次
func estimateTopoRound(msgs []base.Message) int {
	round := 0
	for _, m := range msgs {
		if m.Role == "assistant" {
			round++
		}
	}
	return round
}

// recentToolNames 提取历史中最近 N 个工具名（用于 ForceTools 检查）
func (s *Service) recentToolNames(history []base.Message, n int) []string {
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

// hasRecentBackgroundTool checks if any of the last N assistant messages included
// a background/async tool (spawn_agent, run_command with Background:true, etc.).
// Used to skip interception when work is being handled asynchronously.
func (s *Service) hasRecentBackgroundTool(history []base.Message, lookback int) bool {
	count := 0
	for i := len(history) - 1; i >= 0 && count < lookback; i-- {
		if history[i].Role == "assistant" && history[i].HasToolCalls {
			count++
			for _, tc := range history[i].ToolCalls {
				if tool, ok := s.registry.Get(tc.Function.Name); ok && tool.Background {
					return true
				}
			}
		}
	}
	return false
}

// computeInfoGain returns a heuristic score 0.0-1.0 indicating how much new
// information a tool result provides compared to prior results in this session.
// 0.0 = error/empty, 0.1 = too short, 0.3 = duplicate, 1.0 = novel useful info.
func (s *Service) computeInfoGain(obs string, sessionID string) float64 {
	if obs == "" {
		return 0.0
	}
	// Check for error results: pattern "[xxx 执行失败]" or similar
	if strings.Contains(obs, "执行失败") || strings.Contains(obs, "[error]") ||
		strings.Contains(obs, "Error:") || strings.Contains(obs, "exit status") {
		return 0.0
	}
	// Check for very short/empty results
	if len(obs) < 20 {
		return 0.1
	}
	// Deduplicate: if we've seen this exact result hash before, it's low gain
	h := fnv.New64a()
	h.Write([]byte(obs))
	hash := h.Sum64()
	s.seenMu.Lock()
	if s.seenResults == nil {
		s.seenResults = make(map[string]map[uint64]struct{})
	}
	if s.seenResults[sessionID] == nil {
		s.seenResults[sessionID] = make(map[uint64]struct{})
	}
	_, seen := s.seenResults[sessionID][hash]
	if !seen {
		s.seenResults[sessionID][hash] = struct{}{}
	}
	s.seenMu.Unlock()
	if seen {
		return 0.3
	}
	return 1.0
}

// checkCompletionDistance returns whether the session has completed (dist < 0.05 from target).
// Target is (X=10, Y=0, Z=0) — perfect task completion.
func (s *Service) checkCompletionDistance(sessionID string) (completed bool, dist float64) {
	if s.tracker == nil {
		return true, 0
	}
	st := s.tracker.GetState(sessionID)
	if st == nil {
		return true, 0
	}
	target := topology.Coordinate{X: 10, Y: 0, Z: 0}
	dist = topology.Distance(st.CurrentCoord, target)
	return dist < 0.05, dist
}

// extractRecentToolNames extracts tool names from the last N assistant messages in history.
func (s *Service) extractRecentToolNames(history []base.Message, n int) []string {
	seen := make(map[string]bool)
	var names []string
	count := 0
	for i := len(history) - 1; i >= 0 && count < n; i-- {
		msg := history[i]
		if msg.Role == "assistant" && msg.HasToolCalls {
			count++
			for _, tc := range msg.ToolCalls {
				name := tc.Function.Name
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// executeForcedTool 直接执行强制工具（ForceTools 触发时）
func (s *Service) executeForcedTool(sessionID, toolName string, bgCtx context.Context, emit func(base.ChatStreamEvent), history *[]base.Message, round int, stateRound *int) {
	tool, ok := s.registry.Get(toolName)
	if !ok || tool == nil {
		log.Warn("ForceTools: 工具未注册，跳过并标记已尝试", "工具", toolName)
		// 将失败的强制工具附加到 history，避免 ShouldForceTools 反复选同一工具
		tcID := fmt.Sprintf("call_%d_force_miss", round)
		missTC := base.ToolCall{ID: tcID, Type: "function", Function: base.FunctionCall{Name: toolName, Arguments: "{}"}}
		*history = append(*history, base.Message{Role: "assistant", Content: "", ReasoningContent: "[ForceTools]", HasToolCalls: true, ToolCalls: []base.ToolCall{missTC}})
		*history = append(*history, base.Message{Role: "tool", Content: "[ForceTools 跳过] 工具未注册: " + toolName, ToolCallID: tcID})
		*stateRound++
		return
	}
	tcID := fmt.Sprintf("call_%d_force", round)
	forceArgs := s.forceToolArgs(toolName)
	tc := base.ToolCall{ID: tcID, Type: "function", Function: base.FunctionCall{Name: toolName, Arguments: forceArgs}}
	*history = append(*history, base.Message{Role: "assistant", Content: "", ReasoningContent: "[ForceTools]", HasToolCalls: true, ToolCalls: []base.ToolCall{tc}})

	// 执行工具
	toolCtx, toolCancel := context.WithCancel(bgCtx)
	defer toolCancel()
	toolCtx = context.WithValue(toolCtx, base.SessionIDKey{}, sessionID)

	emit(base.ChatStreamEvent{Type: "tool_start", Tool: toolName, Args: forceArgs})
	parsedArgs, _ := register.ParseArgs(forceArgs)
	result := s.chain.Execute(toolCtx, toolName, parsedArgs)
	obs := formatObservation(toolName, result)
	if !isSilentTool(toolName) {
		emit(base.ChatStreamEvent{Type: "tool_result", Tool: toolName, Content: obs})
	}
	*history = append(*history, base.Message{Role: "tool", Content: obs, ToolCallID: tcID})

	// 记录为被覆盖的拓扑节点
	if s.tracker != nil && s.isTopologyActive(sessionID) {
		parsed := topology.ParseResult{Coord: topology.Coordinate{}, Tools: []string{toolName}, Parsed: true}
		s.tracker.ValidateStep(sessionID, &parsed, []string{toolName})
	}
	*stateRound++
}

// forceToolArgs 返回 ForceTools 触发时工具的默认参数（避免空参数导致失败）
func (s *Service) forceToolArgs(toolName string) string {
	switch toolName {
	case "web_search":
		return `{"query":"yunxi-home"}`
	case "file_search":
		return `{"query":"main.go","path":"/","recursive":true,"max_depth":3}`
	case "file_list":
		return `{"path":"/"}`
	case "recall":
		return `{"query":"project"}`
	default:
		return "{}"
	}
}

// estimateFromRiskProfile 当 AI 未自报 <topology> 标签时，用 RiskProfile 系统估算坐标。
// 这套估算确保即使用不报标签的模型，任务完成度检测也能正常工作。
func (s *Service) estimateFromRiskProfile(sessionID string, toolNames []string) topoParseResult {
	st := s.tracker.GetState(sessionID)
	if st == nil {
		return topoParseResult{Parsed: false}
	}

	// 匹配每个工具的 RiskProfile，取平均 delta
	var totalDY, totalDZ float64
	for _, name := range toolNames {
		profile := topology.MatchRiskProfile(name)
		totalDY += (profile.DeltaYMin + profile.DeltaYMax) / 2
		totalDZ += (profile.DeltaZMin + profile.DeltaZMax) / 2
	}
	avgDY := totalDY / float64(len(toolNames))
	avgDZ := totalDZ / float64(len(toolNames))

	// 估算 X 进度：读工具 +1，写工具 +2，其他 +1.5
	var deltaX float64
	for _, name := range toolNames {
		deltaX += topology.EstimateProgressDelta(name)
	}

	newX := st.CurrentCoord.X + deltaX
	if newX > 10.0 {
		newX = 10.0
	}
	newY := st.CurrentCoord.Y + avgDY
	newZ := st.CurrentCoord.Z + avgDZ

	// Clamp to constraint boundaries to avoid validation rejection on long sessions
	if newY > st.Constraint.A {
		newY = st.Constraint.A
	} else if newY < -st.Constraint.A {
		newY = -st.Constraint.A
	}
	if newZ > st.Constraint.R {
		newZ = st.Constraint.R
	}

	return topoParseResult{
		Coord:  topology.Coordinate{X: newX, Y: newY, Z: newZ},
		Tools:  toolNames,
		Parsed: true, // 标记为系统估算
	}
}

// emitTopologyUpdate 验证拓扑坐标 + 闭环检查 + 发射 SSE 事件
func (s *Service) emitTopologyUpdate(sessionID string, topoActive bool, parsed topoParseResult, toolCalls []base.ToolCall, content string, round int, emit func(base.ChatStreamEvent)) {
	if !topoActive || s.tracker == nil {
		return
	}

	// 提取实际调用的工具名
	actualTools := make([]string, len(toolCalls))
	for i, tc := range toolCalls {
		actualTools[i] = tc.Function.Name
	}

	// 如果 AI 未自报 <topology> 标签，用 RiskProfile 系统估算坐标
	if !parsed.Parsed && len(actualTools) > 0 {
		parsed = s.estimateFromRiskProfile(sessionID, actualTools)
		log.Debug("拓扑估算(无标签)", "会话", sessionID,
			"工具", actualTools, "估算坐标", fmt.Sprintf("(%.1f,%.2f,%.2f)", parsed.Coord.X, parsed.Coord.Y, parsed.Coord.Z))
	}
	if !parsed.Parsed {
		return // 没有工具调用也没有标签，无法估算
	}

	// 运行拓扑验证
	passed, reason := s.tracker.ValidateStep(sessionID, &parsed, actualTools)

	// 闭环检查（T=true 且进度接近完成）
	if parsed.Coord.X >= 9.5 {
		closed, dist, msg := s.tracker.CheckClosedLoop(sessionID)
		if !closed && msg != "" {
			emit(base.ChatStreamEvent{Type: "content", Content: "\n\n" + msg})
			log.Warn("闭环未达成", "会话", sessionID, "距离", dist)
		}
	}

	// 获取最新状态用于 SSE 事件
	st := s.tracker.GetState(sessionID)
	if st == nil {
		return
	}

	// 转换轨迹坐标 (including tool result status)
	traj := make([]base.TrajectoryNode, len(st.Trajectory))
	for i, n := range st.Trajectory {
		traj[i] = base.TrajectoryNode{
			X:          n.Coord.X,
			Y:          n.Coord.Y,
			Z:          n.Coord.Z,
			ToolCall:   n.ToolCall,
			Status:     string(n.Status),
			ToolResult: string(n.ToolResult),
			Reason:     n.Reason,
		}
	}

	// 发射 topology_update SSE 事件
	event := base.ChatStreamEvent{
		Type: "topology_update",
		TopologyUpdate: &base.TopologyUpdateEvent{
			SessionID:  sessionID,
			Coord:      base.Coordinate{X: parsed.Coord.X, Y: parsed.Coord.Y, Z: parsed.Coord.Z},
			Trajectory: traj,
			Constraint: base.TopologyConstraint{
				A: st.Constraint.A, R: st.Constraint.R, T: st.Constraint.T,
				ForceTools: st.Constraint.ForceTools,
			},
			Rejected:       !passed,
			RejectReason:   reason,
			RejectCount:    st.RejectCount,
			TrustLies:      st.Trust.Lies,
			TrustLocked:    st.Trust.Locked,
			ClosedLoop:     st.ClosedLoop,
			ClosedDist:     st.ClosedDist,
			Warning:        st.Warning,
			CommittedCount: st.CommittedCount,
			TotalNodes:     st.TotalNodes,
		},
	}
	emit(event)

	// ── Structured topology event for system log ──
	// Log each validated step so the System Log Viewer can correlate topology events
	// with middleware tool execution events by session_id.
	topoLogger := logger.ForComponent("topology")
	toolList := strings.Join(actualTools, ",")
	topoStatus := "committed"
	if !passed {
		topoStatus = "rejected"
	}
	toolStatus := "none"
	if len(actualTools) > 0 {
		// Use the most recent result for the first actual tool
		results := s.tracker.GetRecentResults(sessionID)
		for i := len(results) - 1; i >= 0; i-- {
			for _, tool := range actualTools {
				if results[i].ToolName == tool {
					if results[i].Success {
						toolStatus = "success"
					} else {
						toolStatus = "error"
					}
					break
				}
			}
			if toolStatus != "none" {
				break
			}
		}
	}
	topoLogger.Info("拓扑验证",
		logger.KeySessionID, sessionID,
		logger.KeyRound, round,
		"tool", toolList,
		"topo_status", topoStatus,
		"tool_status", toolStatus,
		"x", fmt.Sprintf("%.2f", parsed.Coord.X),
		"y", fmt.Sprintf("%.2f", parsed.Coord.Y),
		"z", fmt.Sprintf("%.2f", parsed.Coord.Z),
		"rejected", !passed,
		"reason", reason,
	)
}

// stripTopoFromBlocks 从 content blocks 中移除 <topology> 标签
func (s *Service) stripTopoFromBlocks(blocks *[]base.ContentBlock) {
	for i := range *blocks {
		if (*blocks)[i].Type == base.BlockTypeContent {
			(*blocks)[i].Content = topology.StripTopologyTag((*blocks)[i].Content)
		}
	}
}

// ensureTopologyState 维护 history[1] 的紧凑拓扑状态（分层注入）。
// 坐标变化时替换 message[1]，不追加；首次激活时插入。
// 格式: <t:x,y,z|A:a,R:r,T:t>  (~15 tokens)
func (s *Service) ensureTopologyState(history []base.Message, sessionID string) []base.Message {
	st := s.tracker.GetState(sessionID)
	if st == nil {
		return history
	}
	content := base.BuildTopologyMessage(
		st.CurrentCoord.X, st.CurrentCoord.Y, st.CurrentCoord.Z,
		st.Constraint.A, st.Constraint.R, st.Constraint.T,
		false, // acked
	)
	// 如果 history[1] 已经是拓扑状态消息，就地替换
	if len(history) > 1 && history[1].Role == "system" && strings.HasPrefix(history[1].Content, base.TopologyMsgPrefix) {
		// 节流：坐标变化很小时不更新
		if !topology.ShouldUpdateCoord(
			topology.Coordinate{X: st.CurrentCoord.X, Y: st.CurrentCoord.Y, Z: st.CurrentCoord.Z},
			topology.Coordinate{X: st.CurrentCoord.X, Y: st.CurrentCoord.Y, Z: st.CurrentCoord.Z},
		) {
			// 实际上需要比较新旧坐标，但当前坐标就是最新的，所以这里应该比较 history[1] 中保存的旧坐标
			// 简化处理：直接替换（紧凑格式只有 ~15 tokens，影响很小）
		}
		history[1].Content = content
		return history
	}
	// 首次激活：在 message[0] 之后插入 message[1]
	if len(history) >= 1 {
		newHistory := make([]base.Message, 0, len(history)+1)
		newHistory = append(newHistory, history[0])
		newHistory = append(newHistory, base.Message{Role: "system", Content: content})
		newHistory = append(newHistory, history[1:]...)
		return newHistory
	}
	return history
}

// topoParseResult is used to pass parsed topology through the loop (avoids import cycle).
// Mirrors topology.ParseResult without importing the package in StreamChat signatures.
type topoParseResult = topology.ParseResult

func strVal(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func boolVal(m map[string]any, key string) bool  { b, _ := m[key].(bool); return b }

// RegisterAgentTools 注册子 Agent、Todo、Cron、Skill 工具。
// 必须在 NewService 之后、StreamChat 之前调用。
func (s *Service) RegisterAgentTools() {
	// spawn_agent — 派生并行子 Agent
	s.registry.Register(agent.ToolDef(s.agentMgr))

	// agent_report — 子Agent完成任务后汇报状态（结构化参数保证稳定性）
	s.registry.Register(&base.ToolDef{
		Name:        "agent_report",
		Description: "子Agent任务完成时调用此工具汇报状态。status: 200=成功, 404=未找到目标, 500=执行失败。progress: 0-100 完成百分比。message: 任务结果摘要。**每个子Agent结束时必须调用**",
		Category:    "agent",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"status":   {Type: "integer", Description: "HTTP风格状态码: 200=成功, 404=目标不存在, 500=执行错误"},
				"progress": {Type: "integer", Description: "任务完成进度 0-100"},
				"message":  {Type: "string", Description: "任务结果摘要（包含具体发现、文件列表等）"},
			},
			Required: []string{"status", "progress", "message"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			status := 200
			if v, ok := args["status"]; ok {
				switch n := v.(type) {
				case float64: status = int(n)
				case int: status = n
				}
			}
			progress := 100
			if v, ok := args["progress"]; ok {
				switch n := v.(type) {
				case float64: progress = int(n)
				case int: progress = n
				}
			}
			message := ""
			if v, ok := args["message"].(string); ok { message = v }
			agentID, _ := ctx.Value(agent.AgentIDKey{}).(string)
			if agentID != "" {
				s.agentMgr.ReportStatus(agentID, status, progress, message)
			}
			statusText := "成功"
			switch {
			case status >= 500: statusText = "失败"
			case status >= 400: statusText = "未找到"
			}
			return fmt.Sprintf("[%d %s] 进度=%d%% | %s", status, statusText, progress, message), nil
		},
	})

	// _install_skill — 静默安装 skill 到 /opt/yunxi-home/skills/
	s.registry.Register(&base.ToolDef{
		Name: "_install_skill", Category: "system", RiskLevel: "mutation",
		Description: "将技能 YAML 安装到 /opt/yunxi-home/skills/{name}.yaml。name=技能名, content=YAML内容。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name":    {Type: "string", Description: "技能文件名（不含.yaml后缀）"},
				"content": {Type: "string", Description: "技能的完整 YAML 内容"},
			},
			Required: []string{"name", "content"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			content, _ := args["content"].(string)
			if name == "" || content == "" { return "", fmt.Errorf("name和content不能为空") }
			// 安全检查：只允许字母数字连字符下划线
			for _, c := range name {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
					return "", fmt.Errorf("非法技能名: %s", name)
				}
			}
			path := filepath.Join("/opt/yunxi-home/skills", name, name+".yml")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return "", err }
			if err := os.WriteFile(path, []byte(content), 0644); err != nil { return "", err }
			s.ReloadSkills()
			return fmt.Sprintf("技能已安装: %s.yml → /opt/yunxi-home/skills/%s/", name, name), nil
		},
	})
	// _install_mcp — 静默安装 MCP 到 /opt/yunxi-home/mcp.json
	s.registry.Register(&base.ToolDef{
		Name: "_install_mcp", Category: "system", RiskLevel: "mutation",
		Description: "安装 MCP 服务器到 /opt/yunxi-home/mcp.json。server_name=MCP名, command=启动命令, args=参数数组JSON, env=环境变量JSON(可选)。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"server_name": {Type: "string", Description: "MCP 服务器名称"},
				"command":     {Type: "string", Description: "启动命令（如 npx, python3, uvx）"},
				"args":        {Type: "string", Description: "JSON 数组字符串，如 [\"-y\",\"pkg\"]"},
				"env":         {Type: "string", Description: "JSON 对象字符串，如 {\"KEY\":\"val\"}（可选）"},
			},
			Required: []string{"server_name", "command", "args"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			serverName, _ := args["server_name"].(string)
			command, _ := args["command"].(string)
			argsStr, _ := args["args"].(string)
			envStr, _ := args["env"].(string)
			if serverName == "" || command == "" { return "", fmt.Errorf("参数不完整") }
			// 读取现有 mcp.json
			mcpPath := "/opt/yunxi-home/mcp.json"
			data, err := os.ReadFile(mcpPath)
			if err != nil && !os.IsNotExist(err) { return "", err }
			mcpConfig := make(map[string]any)
			if len(data) > 0 { json.Unmarshal(data, &mcpConfig) }
			servers, _ := mcpConfig["mcpServers"].(map[string]any)
			if servers == nil { servers = make(map[string]any); mcpConfig["mcpServers"] = servers }
			// 解析 args
			var parsedArgs []string
			if argsStr != "" { json.Unmarshal([]byte(argsStr), &parsedArgs) }
			entry := map[string]any{"command": command, "args": parsedArgs}
			if envStr != "" {
				var parsedEnv map[string]string
				if json.Unmarshal([]byte(envStr), &parsedEnv) == nil { entry["env"] = parsedEnv }
			}
			servers[serverName] = entry
			out, _ := json.MarshalIndent(mcpConfig, "", "  ")
			if err := os.WriteFile(mcpPath, out, 0644); err != nil { return "", err }
			s.ReloadMCPTools(mcpPath)
			return fmt.Sprintf("MCP已安装: %s → %s", serverName, mcpPath), nil
		},
	})

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

	// Memory 工具（remember / recall）
	if s.memoryManager != nil {
		s.registry.Register(s.memoryManager.RememberTool())
		s.registry.Register(s.memoryManager.RecallTool())
	}

	// system — 技能系统内部工具，用于返回 SKILL.md 正文供 AI 阅读执行
	s.registry.Register(&base.ToolDef{
		Name:        "system",
		Description: "内部工具：返回技能说明文档正文。AI 调用此工具后应阅读返回内容并按其中指令执行。",
		Category:    "system",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{Type: "object", Properties: map[string]base.ParamProp{
			"skill_body": {Type: "string", Description: "技能的 SKILL.md 完整正文"},
		}},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			if body, ok := args["skill_body"].(string); ok && body != "" {
				return body, nil
			}
			return "（无技能说明）", nil
		},
	})

	// web_search — 通过 DuckDuckGo 搜索网页（免费，无需 API Key）
	s.registry.Register(&base.ToolDef{
		Name:        "web_search",
		Description: "搜索互联网获取实时信息。当需要最新数据、新闻、事实时使用。背后使用 DuckDuckGo，无需 API Key。",
		Category:    "search",
		RiskLevel:   "readonly",
		Timeout:     15 * time.Second,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"query": {Type: "string", Description: "搜索查询词"},
			},
			Required: []string{"query"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			query, _ := args["query"].(string)
			if query == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			return bingSearch(ctx, query)
		},
	})

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

	// request_confirmation — AI 请求用户交互（确认/表单/输入/向导）
	s.registry.Register(&base.ToolDef{
		Name:        "request_confirmation",
		Description: "请求用户交互。支持四种模式：\n1) confirm: 简单确认弹窗（默认）。适合询问是否执行操作。\n2) input: 单页表单。适合收集少量字段（如 host/port）。\n3) form: 同 input，带更多字段。\n4) wizard: 多页向导。用 pages 参数定义多页，每页有独立 fields。\n\n高风险操作（修改配置文件、sudo、安装软件包、改数据库、删数据）必须先调此工具。⚠️ 禁止猜测用户凭据——需要密码/密钥时用此工具询问。",
		Category:    "system",
		RiskLevel:   "mutation",
		Timeout:     120 * time.Second,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"title":       {Type: "string", Description: "弹窗标题"},
				"message":     {Type: "string", Description: "主说明文字"},
				"details":     {Type: "string", Description: "操作详情（可选），完整命令或文件路径"},
				"type":        {Type: "string", Description: "弹窗类型", Enum: []string{"confirm", "input", "form", "wizard"}},
				"variant":     {Type: "string", Description: "视觉风格", Enum: []string{"danger", "warning", "info"}},
				"timeout_sec": {Type: "integer", Description: "超时秒数，默认 120"},
				"action_url": {Type: "string", Description: "操作链接（可选），如 GitHub Token 页面 https://github.com/settings/tokens。前端渲染为可点击的链接"},
				"action_label": {Type: "string", Description: "链接文字（可选），如「前往 GitHub 创建 Token」"},
				"fields": {Type: "array", Description: "表单字段列表（type=input/form 时使用）", Items: &base.ParamProp{Type: "object", Properties: map[string]base.ParamProp{
					"name":        {Type: "string", Description: "字段名（英文）"},
					"label":       {Type: "string", Description: "字段标签（中文）"},
					"type":        {Type: "string", Description: "输入类型: text | password | number"},
					"placeholder": {Type: "string", Description: "占位提示"},
					"default":     {Type: "string", Description: "默认值"},
					"required":    {Type: "boolean", Description: "是否必填"},
				}, Required: []string{"name", "label"}}},
				"pages": {Type: "array", Description: "多页向导（type=wizard 时使用）", Items: &base.ParamProp{Type: "object", Properties: map[string]base.ParamProp{
					"title":       {Type: "string", Description: "本页标题"},
					"description": {Type: "string", Description: "本页说明"},
					"fields": {Type: "array", Items: &base.ParamProp{Type: "object", Properties: map[string]base.ParamProp{
						"name": {Type: "string", Description: "字段名"}, "label": {Type: "string", Description: "字段标签"},
						"type": {Type: "string", Description: "text|password|number"}, "placeholder": {Type: "string"},
						"default": {Type: "string"}, "required": {Type: "boolean"},
					}, Required: []string{"name", "label"}}},
				}, Required: []string{"title", "fields"}}},
			},
			Required: []string{"title", "message"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			title, _ := args["title"].(string)
			message, _ := args["message"].(string)
			details, _ := args["details"].(string)
			variant, _ := args["variant"].(string)
			reqType, _ := args["type"].(string)
			timeoutSec := toolreg.GetInt(args, "timeout_sec", 120)

			if variant == "" {
				variant = "warning"
			}
			if reqType == "" {
				reqType = "confirm"
			}

			fullMsg := message
			if details != "" {
				fullMsg = message + "\n\n" + details
			}

			// 解析 fields
			var fields []base.InteractiveField
			if rawFields, ok := args["fields"]; ok {
				if arr, ok := rawFields.([]any); ok {
					for _, item := range arr {
						if m, ok := item.(map[string]any); ok {
							f := base.InteractiveField{
								Name:        strVal(m, "name"),
								Label:       strVal(m, "label"),
								Type:        strVal(m, "type"),
								Placeholder: strVal(m, "placeholder"),
								Default:     strVal(m, "default"),
								Required:    boolVal(m, "required"),
							}
							if f.Type == "" {
								f.Type = "text"
							}
							fields = append(fields, f)
						}
					}
				}
			}

			// 解析 pages (wizard)
			var pages []base.InteractivePage
			if rawPages, ok := args["pages"]; ok {
				if arr, ok := rawPages.([]any); ok {
					for _, item := range arr {
						if m, ok := item.(map[string]any); ok {
							p := base.InteractivePage{
								Title:       strVal(m, "title"),
								Description: strVal(m, "description"),
							}
							if rawF, ok := m["fields"]; ok {
								if farr, ok := rawF.([]any); ok {
									for _, fi := range farr {
										if fm, ok := fi.(map[string]any); ok {
											f := base.InteractiveField{
												Name: strVal(fm, "name"), Label: strVal(fm, "label"),
												Type: strVal(fm, "type"), Placeholder: strVal(fm, "placeholder"),
												Default: strVal(fm, "default"), Required: boolVal(fm, "required"),
											}
											if f.Type == "" {
												f.Type = "text"
											}
											p.Fields = append(p.Fields, f)
										}
									}
								}
							}
							pages = append(pages, p)
						}
					}
				}
			}

			resp := s.RequestUserInput(ctx, base.InteractiveRequest{
				Type:        reqType,
				Title:       title,
				Message:     fullMsg,
				Fields:      fields,
				Pages:       pages,
				TimeoutSec:  timeoutSec,
				Variant:     variant,
				ActionURL:   strVal(args, "action_url"),
				ActionLabel: strVal(args, "action_label"),
			})
			if resp == nil || !resp.Approved {
				return "❌ 用户拒绝了确认请求。操作已取消。请勿重试，改为询问用户是否愿意更改方案或降低风险。", nil
			}
			// 包含用户填写的值
			extra := ""
			if len(resp.Values) > 0 {
				parts := make([]string, 0, len(resp.Values))
				for k, v := range resp.Values {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
				extra = fmt.Sprintf(" 用户提交的值: %s。", strings.Join(parts, ", "))
			}
			return "✅ 用户已确认。" + extra + "你可以继续执行操作。注意：后续的 run_command 或 file_write 等操作可能会单独触发弹窗确认，这是正常的。", nil
		},
	})

	// activate_specialized_context — AI 自主激活专用提示词上下文
	s.registry.Register(s.activateSpecializedContextTool())

	log.Info("agent/cron/todo/skill tools registered")
}

// activateSpecializedContextTool returns a tool definition that lets the AI activate
// specialized prompt contexts. When the AI calls this tool, the corresponding specialized
// prompt is injected into the system message for the current session.
func (s *Service) activateSpecializedContextTool() *base.ToolDef {
	return &base.ToolDef{
		Name:        "activate_specialized_context",
		Description: "激活特定领域的上下文规则。当你需要文件系统操作、项目运行、代码审查、MCP开发或文件发送等场景的详细规则时调用。可选上下文将在工具的context参数中列出。",
		Category:    "system",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"context": {Type: "string", Description: "要激活的上下文ID"},
			},
			Required: []string{"context"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			contextID, _ := args["context"].(string)
			if contextID == "" {
				return "请指定要激活的上下文名称", nil
			}
			sessionID, _ := ctx.Value(base.SessionIDKey{}).(string)
			if sessionID == "" {
				return "无法获取当前会话", nil
			}
			if s.promptStore == nil {
				return "提示词存储未初始化", nil
			}
			s.promptStore.ActivateContext(sessionID, contextID)
			return "已激活 " + contextID + " 上下文规则", nil
		},
	}
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
	log.Info("技能已热重载", "count", len(s.skillLoader.All()))
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
						if val == "" {
							val = "<你的" + p.Label + ">"
						}
						sb.WriteString(fmt.Sprintf("\"%s\": \"%s\"", p.Name, val))
					}
				}
				if hasEnv {
					sb.WriteString("}")
				}
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
	log.Info("MCP 工具已热重载", "count", len(cfg.MCPServers))
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
	result := s.queryClient.Chat(ctx, messages, query.WithTimeout(60*time.Second))
	if result.Err != nil {
		return "", fmt.Errorf("AI 调用失败: %w", result.Err)
	}
	// Track token usage
	if result.Usage != nil {
		s.metrics.RecordTokens(int64(result.Usage.PromptTokens), int64(result.Usage.CompletionTokens), result.Usage.Cost)
	}
	yamlContent := strings.TrimSpace(result.Content)
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
		log.Info("MCP config not found, skipping", "path", configPath)
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
		log.Warn("MCP connect warning", "error", err)
	}

	// 将 MCP 工具注册到 AI 注册表
	mcp.RegisterTools(s.mcpManager, MCPRegistryAdapter{s.registry})
	log.Info("MCP tools loaded", "servers", len(mcpConfig.MCPServers))
	return nil
}

// MCPRegistryAdapter adapts register.Registry to mcp.ToolRegistry interface.
type MCPRegistryAdapter struct{ Reg *register.Registry }

func (a MCPRegistryAdapter) Register(name string, td *base.ToolDef) {
	td.Name = name
	a.Reg.Register(td)
}

func (s *Service) dispatchBackground(tool *base.ToolDef, funcName string, args map[string]any, sessionID string, ch chan<- base.ChatStreamEvent) bool {
	if !tool.Background {
		return false
	}
	log.Info("后台任务分发", "tool", funcName, "session", sessionID)
	runner := func(ctx *async.TaskContext) (string, error) {
		if tool.HandlerV2 != nil {
			res := tool.HandlerV2(context.Background(), args)
			if res.Status == base.StatusError && res.Error != nil {
				return "", fmt.Errorf("%s: %s", res.Error.Code, res.Error.Message)
			}
			return res.Summary, nil
		}
		if tool.Handler != nil {
			return tool.Handler(context.Background(), args)
		}
		return "", fmt.Errorf("tool %s has no handler", funcName)
	}
	id := s.SubmitBackground(funcName, sessionID, args, runner)
	ch <- base.ChatStreamEvent{Type: "background_task", TaskID: id, TaskStatus: "submitted", TaskMessage: "后台任务已提交: " + funcName}
	return true
}

// ── Helpers ──

func appendBlock(blocks *[]base.ContentBlock, typ base.ContentBlockType, content, toolName, toolArgs, toolCallID string) {
	if len(*blocks) > 0 && (*blocks)[len(*blocks)-1].Type == typ {
		// 替换为新对象，确保前端 Vue 响应式检测到变化
		prev := (*blocks)[len(*blocks)-1]
		(*blocks)[len(*blocks)-1] = base.ContentBlock{
			Type: typ, Content: prev.Content + content,
			ToolName: prev.ToolName, ToolArgs: prev.ToolArgs, ToolCallID: prev.ToolCallID,
		}
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

// bingSearch 使用 Bing 搜索（国内可访问，无需 API Key）。
func bingSearch(ctx context.Context, query string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://www.bing.com/search?"+url.Values{"q": {query}, "setlang": {"zh-cn"}, "count": {"15"}}.Encode(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("搜索请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	html := string(body)

	// 解析 Bing 搜索结果: <li class="b_algo"><h2><a href="URL">Title</a></h2><p>Desc</p>
	algoRe := regexp.MustCompile(`<li class="b_algo">(.*?)</li>`)
	linkRe := regexp.MustCompile(`<a[^>]*href="(https?://[^"]*)"[^>]*>([^<]*)</a>`)
	descRe := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)

	algos := algoRe.FindAllStringSubmatch(html, -1)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索: %s\n\n", query))

	count := 0
	for _, algo := range algos {
		if count >= 10 {
			break
		}
		block := algo[1]
		linkMatch := linkRe.FindStringSubmatch(block)
		if linkMatch == nil {
			continue
		}
		urlStr := linkMatch[1]
		title := cleanHTML(linkMatch[2])
		if urlStr == "" || title == "" {
			continue
		}
		count++
		sb.WriteString(fmt.Sprintf("[%d] %s\n%s\n", count, title, urlStr))
		descMatch := descRe.FindStringSubmatch(block)
		if descMatch != nil {
			desc := cleanHTML(descMatch[1])
			sb.WriteString(truncateStr(strings.TrimSpace(desc), 300))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if count == 0 {
		return fmt.Sprintf("未找到 '%s' 的搜索结果", query), nil
	}
	return sb.String(), nil
}

// cleanHTML 去除 HTML 标签
func cleanHTML(s string) string {
	return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, "")
}

// silentTools lists tools whose events are never emitted to the frontend.
var silentTools = map[string]bool{
	"recall":                       true,
	"remember":                     true,
	"request_confirmation":         true,
	"spawn_agent":                  true,
	"agent_report":                 true,
	"_install_skill":               true,
	"_install_mcp":                 true,
	"activate_specialized_context": true,
}

func isSilentTool(name string) bool { return silentTools[name] }

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
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

// generateTitleAsync 异步调用 AI 生成会话标题摘要（不超过15字）。
// 失败时保留之前已设置的 fallback 标题（取自用户消息前15字）。
func (s *Service) generateTitleAsync(sessionID, userMessage string) {
	ctx := context.Background()

	msgs := []base.Message{
		{Role: "system", Content: "你是一个标题生成器。根据用户的第一条请求，生成一个不超过15个字的简短标题，概括核心请求。只输出标题本身，不要解释、不要引号、不要标点。"},
		{Role: "user", Content: userMessage},
	}

	result := s.queryClient.Chat(ctx, msgs, query.WithTimeout(15*time.Second))
	if result.Err != nil || result.Content == "" {
		log.Debug("标题生成失败", "会话", sessionID, "error", result.Err)
		return
	}

	title := strings.TrimSpace(result.Content)
	// 限制标题为15个 rune（汉字/字符）
	runes := []rune(title)
	if len(runes) > 15 {
		title = string(runes[:15])
	}

	log.Info("AI 生成标题", "会话", sessionID, "标题", title)
	s.sessions.UpdateTitle(sessionID, title)
}

func (s *Service) checkGoalResume(sessionID, userMessage string) string {
	// Use the new state-aware resume prompt builder
	state, _, _ := s.sessions.GetState(sessionID)
	if state == models.SessionStateIdle {
		return ""
	}
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
