package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/mcp"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	webmw "github.com/Yunqingqingxi/yunxi-home/internal/web/middleware"
	"github.com/Yunqingqingxi/yunxi-home/internal/toolreg"
)

var log = logger.ForComponent("handlers")

// ChatHandler AI 聊天 Handler
type ChatHandler struct {
	aiService     *ai.Service
	aiEnabledFn   func() bool    // 运行时检查 AI 是否启用
	mcpSvc        mcp.MCPService // MCP 子系统（接口，可 mock）
	mcpConfigPath string         // mcp.json 路径
	skillsDir     string         // 技能目录
}

func NewChatHandler(svc *ai.Service) *ChatHandler {
	return &ChatHandler{aiService: svc, mcpConfigPath: "mcp.json", skillsDir: "skills"}
}

func (h *ChatHandler) SetAIEnabledCheck(fn func() bool)              { h.aiEnabledFn = fn }
func (h *ChatHandler) SetMCPService(svc mcp.MCPService)              { h.mcpSvc = svc }
func (h *ChatHandler) SetMCPConfigPath(path string)                   { h.mcpConfigPath = path }
func (h *ChatHandler) SetSkillsDir(dir string)                        { h.skillsDir = dir }
func (h *ChatHandler) getMCPPath() string                             { if h.mcpConfigPath != "" { return h.mcpConfigPath }; return "mcp.json" }
func (h *ChatHandler) getSkillsDir() string                           { if h.skillsDir != "" { return h.skillsDir }; return "skills" }

func (h *ChatHandler) isAIEnabled() bool {
	if h.aiService == nil { return false }
	if h.aiEnabledFn == nil { return true }
	return h.aiEnabledFn()
}

type ChatRequest struct {
	Message            string `json:"message"`
	SessionID          string `json:"session_id"`
	Model              string `json:"model"`
	PlanMode           bool   `json:"plan_mode"`
	ReasoningIntensity string `json:"reasoning_intensity"` // "low" | "medium" | "high"
}

// Chat SSE 流式聊天  POST /api/chat
func (h *ChatHandler) Chat(c echo.Context) error {
	if !h.isAIEnabled() {
		return c.JSON(http.StatusOK, successResp(map[string]string{
			"hint":    "ai_not_configured",
			"message": "AI 助手尚未配置，请在设置中启用 AI 服务。",
		}))
	}

	var req ChatRequest
	if err := c.Bind(&req); err != nil || req.Message == "" {
		return c.JSON(http.StatusBadRequest, errorResp("请输入消息内容"))
	}
	if req.SessionID == "" {
		req.SessionID = "chat_" + fmt.Sprintf("%d", time.Now().UnixMilli())
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return c.JSON(http.StatusInternalServerError, errorResp("不支持 SSE"))
	}

	ctx := c.Request().Context()
	if req.Model != "" {
		log.Info("Chat request with model override", "model", req.Model, "session", req.SessionID)
		ctx = context.WithValue(ctx, base.ModelOverrideKey{}, req.Model)
	}
	if req.PlanMode {
		ctx = context.WithValue(ctx, base.PlanModeKey{}, true)
	}
	intensity := req.ReasoningIntensity
	if intensity == "" {
		intensity = h.aiService.ReasoningFor(req.Model)
	}
	ctx = context.WithValue(ctx, base.ReasoningIntensityKey{}, intensity)

	// Extract user identity from JWT claims for the adaptation layer
	userID := ""
	if claims := webmw.GetClaims(c); claims != nil {
		userID = fmt.Sprintf("%d", claims.UserID)
	}
	// Immediately emit session_status so frontend knows the session ID
	// before any AI reply arrives — enables sidebar update + URL sync.
	initEv, _ := json.Marshal(base.ChatStreamEvent{
		Type: "session_created",
		Content: fmt.Sprintf(`{"session_id":"%s"}`, req.SessionID),
	})
	fmt.Fprintf(c.Response(), "data: %s\n\n", initEv)
	flusher.Flush()

	stream := h.aiService.StreamChat(ctx, req.SessionID, userID, req.Message, req.Model)

	// Subscribe to agent events via eventBus for events that are NOT
	// also emitted on the main stream channel (agent_progress, agent_result, etc.)
	agentCh := h.aiService.SubscribeSession(req.SessionID)
	defer h.aiService.UnsubscribeSession(req.SessionID, agentCh)

	streamDone := false
	// dedup set: when Go's select picks agentCh before stream, we must not emit duplicates
	emittedSeqs := make(map[int64]bool, 256)
	idleTicks := 0
	const maxIdleTicks = 5
	heartbeatTicker := time.NewTicker(60 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case ev, ok := <-stream:
			if !ok {
				stream = nil
				streamDone = true
				continue
			}
			idleTicks = 0
			if ev.Seq > 0 {
				emittedSeqs[ev.Seq] = true
			}
			data, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(c.Response(), "data: %s\n\n", data); err != nil {
				return nil
			}
			flusher.Flush()
		case ev, ok := <-agentCh:
			if !ok {
				agentCh = nil
				continue
			}
			// Dedup: skip if already emitted via stream channel
			if ev.Seq > 0 && emittedSeqs[ev.Seq] {
				continue
			}
			emittedSeqs[ev.Seq] = true
			idleTicks = 0
			data, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(c.Response(), "data: %s\n\n", data); err != nil {
				return nil
			}
			flusher.Flush()
		case <-heartbeatTicker.C:
			if streamDone {
				hasAgents := h.aiService.HasRunningAgents(req.SessionID)
				if !hasAgents {
					idleTicks++
					if idleTicks >= maxIdleTicks {
						log.Info("SSE closing: stream done, idle timeout", "session", req.SessionID)
						return nil
					}
				} else {
					idleTicks = 0
				}
			}
			if _, err := fmt.Fprintf(c.Response(), ":keepalive\n\n"); err != nil {
				return nil
			}
			flusher.Flush()
		case <-ctx.Done():
			return nil
		}
	}
}

// StreamSession allows reconnecting to an active session's event stream.
// GET /api/chat/stream/:id
func (h *ChatHandler) StreamSession(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	id := c.Param("id")
	if id == "" { return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id")) }

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok { return c.JSON(http.StatusInternalServerError, errorResp("不支持 SSE")) }

	ctx := c.Request().Context()
	ch := h.aiService.SubscribeSession(id)
	defer h.aiService.UnsubscribeSession(id, ch)

	// Emit initial session_status so the frontend knows streaming/agent state
	hasStream := h.aiService.HasActiveStream(id)
	hasAgents := h.aiService.HasRunningAgents(id)
	// Collect agent details for reconnection state restore
	agentsJSON := "[]"
	if hasAgents {
		agents := h.aiService.GetSessionAgents(id)
		if len(agents) > 0 {
			data, _ := json.Marshal(agents)
			agentsJSON = string(data)
		}
	}
	log.Info("StreamSession 重连，发送初始状态",
		"session", id,
		"streaming", hasStream,
		"has_agents", hasAgents,
		"agent_count", len(h.aiService.GetSessionAgents(id)),
	)
	initEv, _ := json.Marshal(base.ChatStreamEvent{
		Type: "session_status",
		Content: fmt.Sprintf(
			`{"session_id":"%s","streaming":%v,"has_agents":%v,"agents":%s}`,
			id, hasStream, hasAgents, agentsJSON,
		),
	})
	fmt.Fprintf(c.Response(), "data: %s\n\n", initEv)
	flusher.Flush()

	idleTicks := 0
	const maxIdleTicks = 10 // 10 * 60s = 10min max idle before close
	heartbeatTicker := time.NewTicker(60 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				// Event channel closed — check if agents still running before closing
				if !h.aiService.HasRunningAgents(id) {
					return nil
				}
				ch = nil // Don't break — keep waiting for heartbeat timeout
				continue
			}
			idleTicks = 0
			data, _ := json.Marshal(ev)
			fmt.Fprintf(c.Response(), "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeatTicker.C:
			if ch == nil && !h.aiService.HasRunningAgents(id) {
				idleTicks++
				if idleTicks >= maxIdleTicks {
					log.Info("StreamSession closing: idle timeout", "session", id)
					return nil
				}
			} else if h.aiService.HasRunningAgents(id) {
				idleTicks = 0 // Agents still running, keep alive
			}
			if _, err := fmt.Fprintf(c.Response(), ":keepalive\n\n"); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}

// ClearSession 清除单个会话  POST /api/chat/clear
func (h *ChatHandler) ClearSession(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp(map[string]string{"message": "AI 未启用"})) }
	var req struct{ SessionID string `json:"session_id"` }
	c.Bind(&req)
	if req.SessionID == "" { return c.JSON(http.StatusBadRequest, errorResp("请提供 session_id")) }
	h.aiService.ClearSession(req.SessionID)
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "会话已清除"}))
}

// ListSessions 列出会话  GET /api/chat/sessions?type=chat
func (h *ChatHandler) ListSessions(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp([]any{})) }
	sessionType := c.QueryParam("type")
	var sessions []models.ChatSession
	if sessionType != "" {
		sessions = h.aiService.ListSessionsByType(sessionType)
	} else {
		sessions = h.aiService.ListSessions()
	}
	// Fill IsActive for each session
	for i := range sessions {
		sessions[i].IsActive = h.aiService.HasActiveStream(sessions[i].ID) || h.aiService.HasRunningAgents(sessions[i].ID)
	}
	return c.JSON(http.StatusOK, successResp(sessions))
}

// GetSessionAgents 返回指定会话的所有子 Agent
// GET /api/chat/sessions/:id/agents
func (h *ChatHandler) GetSessionAgents(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp([]any{})) }
	id := c.Param("id")
	if id == "" { return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id")) }
	agents := h.aiService.GetSessionAgents(id)
	result := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		result = append(result, a.ToJSON())
	}
	return c.JSON(http.StatusOK, successResp(result))
}

// ClearAllSessions 清除全部会话  POST /api/chat/clear-all
func (h *ChatHandler) ClearAllSessions(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp(map[string]string{"message": "无会话"})) }
	h.aiService.ClearAllSessions()
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "所有会话已清除"}))
}

// Hints 返回 AI 生成的上下文快捷提示  GET /api/chat/hints
func (h *ChatHandler) Hints(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusOK, successResp([]string{"查看系统状态", "管理文件", "检查 DNS", "Docker 状态"}))
	}
	sessionID := c.QueryParam("session_id")
	hints := h.aiService.GetHints(c.Request().Context(), sessionID)
	return c.JSON(http.StatusOK, successResp(hints))
}

// Tools 返回已注册的工具列表  GET /api/chat/tools
func (h *ChatHandler) Tools(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusOK, successResp([]any{}))
	}
	var tools []any
	rawJSON := h.aiService.GetToolsJSON()
	if len(rawJSON) > 0 {
		json.Unmarshal(rawJSON, &tools)
	}
	return c.JSON(http.StatusOK, successResp(map[string]any{
		"tools": tools,
	}))
}

// GetSessionDetail returns session metadata and message history.
// GET /api/chat/sessions/:id
func (h *ChatHandler) GetSessionDetail(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	id := c.Param("id")
	if id == "" { return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id")) }
	detail, err := h.aiService.GetSessionHistory(id)
	if err != nil { return c.JSON(http.StatusNotFound, errorResp("会话不存在")) }
	return c.JSON(http.StatusOK, successResp(detail))
}

// DeleteSession removes a single chat session.
// DELETE /api/chat/sessions/:id
func (h *ChatHandler) DeleteSession(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已删除"})) }
	id := c.Param("id")
	if id == "" { return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id")) }
	h.aiService.ClearSession(id)
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "会话已删除"}))
}

// UpdateSessionMeta 更新会话元数据（标题、置顶）
// PATCH /api/chat/sessions/:id
func (h *ChatHandler) UpdateSessionMeta(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	id := c.Param("id")
	if id == "" { return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id")) }
	var req struct {
		Title  *string `json:"title"`
		Pinned *bool   `json:"pinned"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Title == nil && req.Pinned == nil {
		return c.JSON(http.StatusBadRequest, errorResp("至少需要 title 或 pinned"))
	}
	if err := h.aiService.UpdateSessionMeta(id, req.Title, req.Pinned); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("更新失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "updated"}))
}

// GenerateTitle 异步为会话生成标题  POST /api/chat/title
func (h *ChatHandler) GenerateTitle(c echo.Context) error {
	if !h.isAIEnabled() {
		return c.JSON(http.StatusOK, successResp(map[string]string{"status": "skipped"}))
	}
	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.SessionID == "" || req.Message == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id 或 message"))
	}

	// Fire-and-forget: return immediately, generate title in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		title, err := h.aiService.GenerateTitle(ctx, req.SessionID, req.Message)
		if err != nil {
			log.Warn("异步标题生成失败", "session", req.SessionID, "error", err)
			return
		}
		log.Info("异步标题已生成", "session", req.SessionID, "title", title)
	}()

	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "processing"}))
}

// ConfirmAction 处理危险操作确认
// POST /api/chat/confirm
func (h *ChatHandler) ConfirmAction(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	var req struct {
		ConfirmID string            `json:"confirm_id"`
		Approved  bool              `json:"approved"`
		Fields    map[string]string `json:"fields"`
	}
	if err := c.Bind(&req); err != nil { return c.JSON(http.StatusBadRequest, errorResp("参数错误")) }
	if !h.aiService.ConfirmAction(req.ConfirmID, req.Approved, req.Fields) {
		return c.JSON(http.StatusNotFound, errorResp("确认已过期或不存在"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "ok"}))
}

// RespondInteractive 处理前端对交互式请求的响应
// POST /api/chat/respond
func (h *ChatHandler) RespondInteractive(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	var req base.InteractiveResponse
	if err := c.Bind(&req); err != nil {
		body, _ := io.ReadAll(c.Request().Body)
		log.Warn("RespondInteractive bind failed", "error", err, "body", string(body))
		return c.JSON(http.StatusBadRequest, errorResp("参数错误: "+err.Error()))
	}
	if !h.aiService.RespondInteractive(req) {
		return c.JSON(http.StatusNotFound, errorResp("请求已过期或不存在"))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "ok"}))
}

// InjectMessage 在流式输出中注入用户消息（不中断当前流）
// POST /api/chat/inject
func (h *ChatHandler) InjectMessage(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusOK, successResp(map[string]string{"status": "injected"})) }
	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
	}
	if err := c.Bind(&req); err != nil || req.SessionID == "" || req.Message == "" {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	h.aiService.InjectMessage(req.SessionID, req.Message)
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "injected"}))
}

// RunCommand 执行 / 指令（与 QQ Bot 一致的直接分发，不经过 AI）
// POST /api/chat/command
func (h *ChatHandler) RunCommand(c echo.Context) error {
	if h.aiService == nil { return c.JSON(http.StatusNotFound, errorResp("AI 未启用")) }
	var req struct {
		SessionID string `json:"session_id"`
		Command   string `json:"command"`
	}
	if err := c.Bind(&req); err != nil || req.Command == "" {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	cmdStr := strings.TrimSpace(req.Command)
	if !strings.HasPrefix(cmdStr, "/") {
		return c.JSON(http.StatusBadRequest, errorResp("命令必须以 / 开头"))
	}
	cmdStr = strings.TrimPrefix(cmdStr, "/")

	parts := strings.Fields(cmdStr)
	cmdName := strings.ToLower(parts[0])
	cmdArg := ""
	if len(parts) > 1 {
		cmdArg = strings.Join(parts[1:], " ")
	}

	var result string

	switch {
	case cmdName == "reload-skills":
		if err := h.aiService.ReloadSkills(); err != nil {
			result = "重载失败: " + err.Error()
		} else {
			result = "技能已重新加载"
		}

	case cmdName == "reload-mcp":
		if err := h.aiService.ReloadMCPTools(h.getMCPPath()); err != nil {
			result = "重载失败: " + err.Error()
		} else {
			result = "MCP 工具已重新加载"
		}

	case cmdName == "help" || cmdName == "list-skills":
		skills := h.aiService.ListSkills()
		var sb strings.Builder
		sb.WriteString("可用指令:\n  /help /clear /compact /get-mcp /reload-skills /reload-mcp\n")
		if len(skills) > 0 {
			sb.WriteString("\n可用技能:\n")
			for name, desc := range skills {
				sb.WriteString(fmt.Sprintf("  /%s  %s\n", name, desc))
			}
		}
		result = sb.String()

	case cmdName == "clear":
		if req.SessionID != "" {
			h.aiService.ClearSession(req.SessionID)
		}
		result = "会话已清空"

	case cmdName == "compact":
		result = "上下文压缩已由 v3.1 拓扑约束系统自动管理，无需手动执行"

	case cmdName == "get-mcp":
		if cmdArg == "" || cmdArg == "help" {
			result = h.aiService.GetMCPServer(c.Request().Context(), cmdArg)
		} else {
			results, searchErr := toolreg.SearchMCPMarket(cmdArg)
			if searchErr != nil {
				result = "搜索失败: " + searchErr.Error()
			} else if len(results) > 0 && results[0].Score >= 100 {
				pkg := results[0].Name
				params, hasParams := toolreg.DetectRequiredParams(pkg)
				if hasParams && hasRequired(params) {
					result = fmt.Sprintf("📦 %s\n需要配置参数才能安装。输入 /get-mcp %s 查看详情。", pkg, pkg)
				} else {
					result = fmt.Sprintf("📦 %s\n安装: npm install -g %s 或 npx -y %s\n然后添加到 mcp.json 并执行 /reload-mcp", pkg, pkg, pkg)
				}
			} else {
				result = toolreg.FormatMCPSearchResults(results, cmdArg)
			}
		}

	default:
		// 尝试作为技能名执行
		result = h.aiService.RunSkill(c.Request().Context(), cmdName)
	}

	// 将命令原文作为用户消息写入会话历史
	if req.SessionID != "" {
		h.aiService.InjectUserMessage(req.SessionID, "/"+cmdStr)
	}
	// 直接持久化命令结果到会话历史（不依赖 StreamChat injectCh）
	if req.SessionID != "" && result != "" {
		msg := "[指令 /" + cmdName
		if cmdArg != "" {
			msg += " " + cmdArg
		}
		msg += "]\n" + result
		h.aiService.SaveSystemMessage(req.SessionID, msg)
	}

	return c.JSON(http.StatusOK, successResp(map[string]string{"command": cmdName, "result": result}))
}

// ── 统一命令列表 ────────────────────────────────────────────────────────

// CommandInfo 命令信息
type CommandInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // "builtin" | "skill" | "mcp"
	Description string `json:"description"`
	Usage       string `json:"usage"`
	SkillName   string `json:"skill_name,omitempty"` // 仅 type=skill 时有值
}

// GetCommands 返回所有可用命令（内置 + 技能）
// GET /api/commands
func (h *ChatHandler) GetCommands(c echo.Context) error {
	cmds := []CommandInfo{
		{Name: "help", Type: "builtin", Description: "显示可用指令", Usage: "/help"},
		{Name: "clear", Type: "builtin", Description: "清空当前对话", Usage: "/clear"},
		{Name: "compact", Type: "builtin", Description: "压缩对话上下文", Usage: "/compact"},
		{Name: "topology", Type: "builtin", Description: "拓扑约束管理 (unlock/status)", Usage: "/topology [unlock|status]"},
		{Name: "get-mcp", Type: "builtin", Description: "搜索并安装 MCP 工具", Usage: "/get-mcp <关键词>"},
		{Name: "reload-skills", Type: "builtin", Description: "重新加载技能", Usage: "/reload-skills"},
		{Name: "reload-mcp", Type: "builtin", Description: "重新加载 MCP 工具", Usage: "/reload-mcp"},
	}

	// 动态技能列表（从技能注册中心）
	if h.aiService != nil {
		skills := h.aiService.ListSkills()
		for name, desc := range skills {
			cmds = append(cmds, CommandInfo{
				Name:        name,
				Type:        "skill",
				Description: desc,
				Usage:       "/" + name,
				SkillName:   name,
			})
		}
	}

	// MCP 安装命令（热门包）
	if h.mcpSvc != nil {
		mcpServers := h.mcpSvc.List()
		for _, s := range mcpServers {
			cmds = append(cmds, CommandInfo{
				Name:        "mcp-" + s.Name,
				Type:        "mcp",
				Description: "已安装 MCP: " + s.Package + " (" + statusText(s.Connected) + ", " + strconv.Itoa(s.Tools) + " 工具)",
				Usage:       "/mcp-" + s.Name,
			})
		}
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{"commands": cmds}))
}

func statusText(connected bool) string {
	if connected { return "已连接" }
	return "未连接"
}

// ── Prompt Management API ─────────────────────────────────────

// GetPrompts returns all prompt sections with effective values and source.
// GET /api/config/prompts
func (h *ChatHandler) GetPrompts(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusOK, successResp(map[string]any{"sections": nil}))
	}
	sections := h.aiService.GetAllPromptSections()
	return c.JSON(http.StatusOK, successResp(map[string]any{"sections": sections}))
}

// UpdatePrompt updates a single prompt section and triggers hot reload.
// PUT /api/config/prompts/:section
func (h *ChatHandler) UpdatePrompt(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	section := c.Param("section")
	if section == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 section"))
	}
	var req struct {
		Data string `json:"data"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if err := h.aiService.UpdatePromptSection(c.Request().Context(), section, req.Data); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("更新失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "updated", "section": section}))
}

// ResetPrompt removes a DB override, falling back to Go default.
// POST /api/config/prompts/:section/reset
func (h *ChatHandler) ResetPrompt(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	section := c.Param("section")
	if section == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 section"))
	}
	if err := h.aiService.ResetPromptSection(c.Request().Context(), section); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("重置失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "reset", "section": section}))
}

// ── Topology API ──────────────────────────────────────────────

// GetTopology returns the current topology state for a session.
// GET /api/chat/sessions/:id/topology
func (h *ChatHandler) GetTopology(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	state := h.aiService.GetTopologyState(id)
	return c.JSON(http.StatusOK, successResp(state))
}

// UpdateConstraint updates topology constraint parameters for a session.
// PUT /api/chat/sessions/:id/topology/constraint
func (h *ChatHandler) UpdateConstraint(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	var req struct {
		A          float64  `json:"a"`
		R          float64  `json:"r"`
		T          bool     `json:"t"`
		ForceTools []string `json:"force_tools"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if err := h.aiService.UpdateConstraint(id, req.A, req.R, req.T, req.ForceTools); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("更新约束失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "constraint_updated"}))
}

// OverrideNode forces acceptance of the next topology check.
// POST /api/chat/sessions/:id/topology/override
func (h *ChatHandler) OverrideNode(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	var req struct {
		TargetCoord *base.Coordinate `json:"target_coord"`
	}
	c.Bind(&req)
	h.aiService.OverrideNextNode(id, req.TargetCoord)
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "override_accepted"}))
}

// ResetTrust resets the topology trust state (unlocks trust and resets lie counter).
// POST /api/chat/sessions/:id/topology/trust-reset
func (h *ChatHandler) ResetTrust(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	h.aiService.ResetTrust(id)
	return c.JSON(http.StatusOK, successResp(map[string]string{"status": "trust_reset"}))
}

// EditMessage edits or inserts a message at the given index.
// PUT /api/chat/sessions/:id/messages/:messageIndex
func (h *ChatHandler) EditMessage(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	var req struct {
		Content    string `json:"content"`
		InsertMode bool   `json:"insert_mode"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	result, err := h.aiService.EditMessageTopology(id, c.Param("messageIndex"), req.Content, req.InsertMode)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("编辑失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(result))
}

// DeleteMessage deletes a message at the given index.
// DELETE /api/chat/sessions/:id/messages/:messageIndex
func (h *ChatHandler) DeleteMessage(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	result, err := h.aiService.DeleteMessageTopology(id, c.Param("messageIndex"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("删除失败: "+err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(result))
}

// InterruptSession 中断会话的活跃流
// POST /api/chat/sessions/:id/interrupt
func (h *ChatHandler) InterruptSession(c echo.Context) error {
	if h.aiService == nil {
		return c.JSON(http.StatusNotFound, errorResp("AI 未启用"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session_id"))
	}
	var req struct {
		Mode string `json:"mode"`
	}
	c.Bind(&req)
	if req.Mode == "" {
		req.Mode = "soft"
	}
	log.Info("InterruptSession API 调用", "session", id, "mode", req.Mode)
	snapshot, err := h.aiService.CancelSession(id, req.Mode)
	if err != nil {
		log.Error("InterruptSession 失败", "session", id, "error", err)
		return c.JSON(http.StatusInternalServerError, errorResp("中断失败: "+err.Error()))
	}
	log.Info("InterruptSession 成功", "session", id, "snapshot", snapshot)
	return c.JSON(http.StatusOK, successResp(snapshot))
}
