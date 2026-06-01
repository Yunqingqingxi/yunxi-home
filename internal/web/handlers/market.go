package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/toolreg"
)

// ── 技能市场 ────────────────────────────────────────────────────────

// SearchSkills 在线搜索技能市场  POST /api/market/search-skills
func (h *ChatHandler) SearchSkills(c echo.Context) error {
	var req struct{ Query string `json:"query"` }
	c.Bind(&req)
	results, err := toolreg.SearchSkillsOnline(req.Query)
	if err != nil {
		return c.JSON(http.StatusOK, successResp(map[string]any{"items": []toolreg.SkillMarketItem{}, "error": err.Error()}))
	}
	return c.JSON(http.StatusOK, successResp(map[string]any{"items": results}))
}

// InstallSkill 下载并安装技能  POST /api/market/install-skill
func (h *ChatHandler) InstallSkill(c echo.Context) error {
	var req struct {
		DownloadURL string `json:"download_url"`
		SkillsDir   string `json:"skills_dir"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.SkillsDir == "" {
		req.SkillsDir = h.getSkillsDir()
	}
	result, err := toolreg.DownloadAndInstallSkill(req.DownloadURL, req.SkillsDir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	// 热重载技能
	if h.aiService != nil {
		h.aiService.ReloadSkills()
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"result": result}))
}

// ── MCP 安装 ─────────────────────────────────────────────────────────

// InstallMCP 直接安装 MCP 工具（同步，确定性的）
// POST /api/market/install-mcp
func (h *ChatHandler) InstallMCP(c echo.Context) error {
	var req struct {
		Package string            `json:"package"`
		Env     map[string]string `json:"env,omitempty"`
	}
	if err := c.Bind(&req); err != nil || req.Package == "" {
		return c.JSON(http.StatusBadRequest, errorResp("请提供 package 名称"))
	}

	// 参数检测：需要用户填写的返回 need_params
	params, hasParams := toolreg.DetectRequiredParams(req.Package)
	if hasParams && len(req.Env) == 0 && hasRequired(params) {
		return c.JSON(http.StatusOK, successResp(map[string]any{
			"status":      "need_params",
			"package":     req.Package,
			"need_params": params,
		}))
	}

	// 合并默认值
	envVars := make(map[string]string)
	for k, v := range req.Env {
		envVars[k] = v
	}
	if hasParams {
		for _, p := range params {
			if p.Default != "" && envVars[p.Name] == "" {
				envVars[p.Name] = p.Default
			}
		}
	}

	// 执行安装
	result, err := toolreg.InstallMCPWithParams(req.Package, h.getMCPPath(), envVars)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}

	// 热重载 MCP 工具
	if h.aiService != nil {
		if reloadErr := h.aiService.ReloadMCPTools(h.getMCPPath()); reloadErr != nil {
			slog.Warn("MCP reload after install failed", "package", req.Package, "error", reloadErr)
		}
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{
		"success":     result.Success,
		"message":     result.Message,
		"server_name": result.ServerName,
		"package":     result.Package,
	}))
}

func hasRequired(params []toolreg.RequiredParam) bool {
	for _, p := range params {
		if p.Required {
			return true
		}
	}
	return false
}

// InstallMCPStream SSE 流式安装 MCP（实时进度+后端状态记录）
// POST /api/market/install-mcp-stream
func (h *ChatHandler) InstallMCPStream(c echo.Context) error {
	var req struct {
		Package string            `json:"package"`
		Env     map[string]string `json:"env,omitempty"`
		TaskID  string            `json:"task_id"`
	}
	if err := c.Bind(&req); err != nil || req.Package == "" {
		return c.JSON(http.StatusBadRequest, errorResp("请提供 package 名称"))
	}

	// 参数检测
	params, hasParams := toolreg.DetectRequiredParams(req.Package)
	if hasParams && len(req.Env) == 0 && hasRequired(params) {
		return c.JSON(http.StatusOK, successResp(map[string]any{
			"status":      "need_params",
			"package":     req.Package,
			"need_params": params,
		}))
	}

	// 优先使用新 Subsystem（增量安装，不中断已有连接）
	if h.mcpSvc != nil {
		return h.installMCPWithSubsystem(c, req.Package, req.Env)
	}

	// 回退到旧逻辑
	return h.installMCPLegacy(c, req)
}

// installMCPWithSubsystem uses the MCP Subsystem for incremental install.
func (h *ChatHandler) installMCPWithSubsystem(c echo.Context, pkg string, env map[string]string) error {
	ctx := c.Request().Context()
	eventCh, err := h.mcpSvc.Install(ctx, pkg, env)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	flusher, _ := c.Response().Writer.(http.Flusher)

	for ev := range eventCh {
		data, _ := json.Marshal(ev)
		fmt.Fprintf(c.Response(), "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	return nil
}

// installMCPLegacy is the fallback installer when Subsystem is unavailable.
func (h *ChatHandler) installMCPLegacy(c echo.Context, req struct {
	Package string            `json:"package"`
	Env     map[string]string `json:"env,omitempty"`
	TaskID  string            `json:"task_id"`
}) error {
	tracker := toolreg.GetInstallTracker()
	taskID := req.TaskID
	if taskID == "" {
		taskID = "mcp_" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	tracker.CreateTask(taskID, req.Package)

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	flusher, _ := c.Response().Writer.(http.Flusher)
	emit := func(step, status, msg string, pct int) {
		data, _ := json.Marshal(map[string]any{"task_id": taskID, "step": step, "status": status, "message": msg, "progress": pct})
		fmt.Fprintf(c.Response(), "data: %s\n\n", data)
		if flusher != nil { flusher.Flush() }
		tracker.AddStep(taskID, toolreg.InstallStep{Step: step, Status: status, Message: msg})
		newStatus := ""
		if status == "error" { newStatus = "error" }
		if step == "done" { newStatus = "done" }
		tracker.UpdateProgress(taskID, pct, newStatus, "")
	}

	emit("start", "running", fmt.Sprintf("开始安装 %s", req.Package), 5)

	emit("download", "running", "正在下载包...", 10)
	if _, err := exec.LookPath("npm"); err == nil {
		cmd := exec.Command("npm", "install", "-g", req.Package)
		out, installErr := cmd.CombinedOutput()
		if installErr != nil {
			emit("download", "running", fmt.Sprintf("npm install 警告: %s (将使用 npx 自动下载)", string(out)[:min(len(out), 100)]), 20)
		} else {
			emit("download", "done", "包下载完成", 25)
		}
	} else {
		emit("download", "done", "跳过 npm install (将使用 npx 自动下载)", 25)
	}

	emit("config", "running", "写入 mcp.json 配置...", 35)
	result, err := toolreg.InstallMCPWithParams(req.Package, h.getMCPPath(), req.Env)
	if err != nil {
		emit("config", "error", fmt.Sprintf("配置写入失败: %v", err), 35)
		return nil
	}
	emit("config", "done", "配置已写入 mcp.json", 50)

	emit("connect", "running", "正在启动 MCP 服务器并验证连接...", 55)
	serverName := result.ServerName
	verifyResult := verifyMCPWithProgress(serverName, req.Package, emit)
	emit("connect", "done", verifyResult, 85)

	emit("reload", "running", "热重载 MCP 工具...", 90)
	if h.aiService != nil {
		if err := h.aiService.ReloadMCPTools(h.getMCPPath()); err != nil {
			emit("reload", "warning", fmt.Sprintf("重载警告: %v", err), 95)
		} else {
			emit("reload", "done", "MCP 工具已重载", 95)
		}
	}

	emit("done", "success", fmt.Sprintf("✅ %s 安装完成！%s", serverName, verifyResult), 100)
	return nil
}

func verifyMCPWithProgress(serverName, packageName string, emit func(string, string, string, int)) string {
	cmd := exec.Command("npx", "-y", packageName)
	stdin, _ := cmd.StdinPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("❌ 启动失败: %v", err)
	}
	defer cmd.Process.Kill()

	emit("connect", "running", "发送 initialize 请求...", 60)
	stdin.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"yunxi-home","version":"1.0.0"}}}` + "\n"))

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 65536), 65536)

	done := make(chan string, 1)
	go func() {
		time.Sleep(500 * time.Millisecond)
		if stderrBuf.Len() > 0 {
			done <- fmt.Sprintf("⚠️ stderr: %s", strings.TrimSpace(stderrBuf.String())[:100])
			return
		}
		if !scanner.Scan() {
			done <- "❌ 无响应"
			return
		}
		var resp map[string]any
		json.Unmarshal(scanner.Bytes(), &resp)

		stdin.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"))
		stdin.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"))

		if scanner.Scan() {
			var listResp map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &listResp); err == nil {
				if result, ok := listResp["result"].(map[string]any); ok {
					if tools, ok := result["tools"].([]any); ok {
						done <- fmt.Sprintf("✅ 连接成功！发现 %d 个工具", len(tools))
						return
					}
				}
			}
		}
		done <- "⚠️ 服务器已启动但未返回工具列表"
	}()

	select {
	case r := <-done:
		return r
	case <-time.After(20 * time.Second):
		return "⚠️ 连接超时(20s)，服务器可能需要更多时间初始化"
	}
}

// ListInstallTasks 获取当前安装任务列表（刷新不丢失）
// GET /api/market/install-tasks
func (h *ChatHandler) ListInstallTasks(c echo.Context) error {
	tracker := toolreg.GetInstallTracker()
	tasks := tracker.ListTasks()
	result := make([]map[string]any, len(tasks))
	for i, t := range tasks {
		result[i] = map[string]any{
			"id": t.ID, "package": t.Package, "status": t.Status,
			"progress": t.Progress, "steps": t.Steps, "error": t.Error,
			"created_at": t.CreatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(http.StatusOK, successResp(map[string]any{"tasks": result}))
}

// PopularMCP 获取热门 MCP 服务器推荐  GET /api/market/popular-mcp
func (h *ChatHandler) PopularMCP(c echo.Context) error {
	servers := toolreg.GetPopularMCPServers()
	return c.JSON(http.StatusOK, successResp(map[string]any{"items": servers}))
}

// GetInstalled 获取已安装的技能和 MCP 服务器列表  GET /api/market/installed
func (h *ChatHandler) GetInstalled(c echo.Context) error {
	skillsMap := map[string]string{}
	if h.aiService != nil {
		skillsMap = h.aiService.ListSkills()
	}
	skills := make([]map[string]string, 0, len(skillsMap))
	for name, desc := range skillsMap {
		skills = append(skills, map[string]string{"name": name, "description": desc})
	}
	return c.JSON(http.StatusOK, successResp(map[string]any{
		"skills":     skills,
		"skills_dir": h.getSkillsDir(),
	}))
}
