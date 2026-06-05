package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// ToolDef 返回 AgentTool 的定义。
// AI 调用此工具来派生子 Agent 处理并行子任务。
func ToolDef(mgr *Manager) *base.ToolDef {
	return &base.ToolDef{
		Name:        "spawn_agent",
		Description: "派生子Agent并行执行独立子任务。每个子Agent有独立上下文和受限工具，完成后自动汇报结果。\n\ngoal 按此格式写：\n1. 【目标】一句话说清要完成什么\n2. 【范围】限定搜索/操作的具体范围（路径、关键词、时间等）\n3. 【预期产物】明确需要什么样的输出（摘要/列表/文件等）\n4. 【成功标准】什么情况算完成/算找不到/算失败",
		Category:    "agent",
		RiskLevel:   "mutation",
		Background:  true,
		Timeout:     15 * time.Minute,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"tasks": {
					Type:        "array",
					Description: "子任务列表，每项包含 goal(按四段式写) 和 tool_filter(允许的工具列表)",
					Items: &base.ParamProp{
						Type: "object",
						Properties: map[string]base.ParamProp{
							"goal": {
								Type:        "string",
								Description: "子任务目标。四段式：1.目标 2.范围 3.预期产物 4.成功标准。示例：\n1. 搜索 QQ Bot 官方API文档\n2. 搜索词 'QQ机器人 C2C消息 API' 'QQBot file upload'，范围限定官方文档站点\n3. Markdown 摘要：核心API端点、认证方式、消息类型、文件上传限制\n4. 找到关键API信息=200，多次搜索无结果=404，网络不可达=500",
							},
							"tool_filter": {
								Type:        "array",
								Description: "允许的工具列表。信息搜索类给 web_search+file_read，代码类加 run_command+file_write，至少一个",
								Items: &base.ParamProp{
									Type: "string",
								},
							},
						},
						Required: []string{"goal", "tool_filter"},
					},
				},
				"async": {
					Type:        "boolean",
					Description: "设为 true 时后台异步执行，主线程立即返回 agent_id。用于耗时超过 10 秒的长任务，主 Agent 应先回复用户再等待结果。完成后结果自动注入会话。",
				},
			},
			Required: []string{"tasks"},
		},
		Examples: []base.ToolExample{
			{
				Description: "并行检查两个容器和域名",
				Args: map[string]any{
					"tasks": []map[string]any{
						{"goal": "检查 Docker 容器 A 的运行状态和日志", "tool_filter": []string{"docker_list_containers", "docker_get_logs"}},
						{"goal": "查询域名 example.com 的 DNS 记录", "tool_filter": []string{"query_dns_records", "list_cloud_records"}},
					},
				},
			},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleSpawnAgent(mgr, ctx, args)
		},
	}
}

func handleSpawnAgent(mgr *Manager, ctx context.Context, args map[string]any) *base.ToolResult {
	rawTasks, ok := args["tasks"]
	if !ok {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "缺少 tasks 参数"},
			Summary: "参数错误：缺少 tasks",
		}
	}

	var tasks []SpawnTask
	switch v := rawTasks.(type) {
	case []any:
		for _, raw := range v {
			task, err := parseSpawnTask(raw)
			if err != nil {
				return &base.ToolResult{
					Status:  base.StatusError,
					Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: err.Error()},
					Summary: "任务格式错误: " + err.Error(),
				}
			}
			tasks = append(tasks, task)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &tasks); err != nil {
			return &base.ToolResult{
				Status:  base.StatusError,
				Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "tasks JSON 解析失败"},
				Summary: "参数格式错误",
			}
		}
	default:
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "tasks 必须是数组"},
			Summary: "参数类型错误",
		}
	}

	if len(tasks) == 0 {
		return &base.ToolResult{
			Status:  base.StatusSuccess,
			Summary: "没有需要执行的子任务",
		}
	}

	// 异步模式：后台执行，立即返回 agent_id
	async, _ := args["async"].(bool)
	if async {
		sessionID := SessionIDFromCtx(ctx)
		ids := mgr.SpawnAsync(tasks, sessionID)
		return &base.ToolResult{
			Status: base.StatusSuccess,
			Summary: fmt.Sprintf(
				"已启动 %d 个子 Agent 在后台执行（ID: %s）。主线程已释放，可立即回复用户。完成后结果会自动注入会话。",
				len(ids), strings.Join(ids, ", ")),
			Data: ids,
		}
	}

	// 同步模式：阻塞等待全部完成
	results := mgr.SpawnParallel(tasks, "")

	var parts []string
	successCount := 0
	for _, r := range results {
		if r.Status == StatusDone {
			successCount++
			parts = append(parts, fmt.Sprintf("✅ %s: %s", r.Goal, r.Summary))
		} else {
			parts = append(parts, fmt.Sprintf("❌ %s: %s", r.Goal, r.Error))
		}
	}

	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: fmt.Sprintf("并行执行完成 (%d/%d 成功)\n%s", successCount, len(results), strings.Join(parts, "\n")),
		Data:    results,
	}
}

func parseSpawnTask(raw any) (SpawnTask, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		if str, ok := raw.(string); ok {
			var task SpawnTask
			if err := json.Unmarshal([]byte(str), &task); err != nil {
				return SpawnTask{}, fmt.Errorf("任务格式无效")
			}
			return task, nil
		}
		return SpawnTask{}, fmt.Errorf("每个任务必须是对象")
	}

	goal, _ := m["goal"].(string)
	if goal == "" {
		return SpawnTask{}, fmt.Errorf("task.goal 不能为空")
	}

	var toolFilter []string
	if rawFilter, ok := m["tool_filter"]; ok {
		switch fv := rawFilter.(type) {
		case []any:
			for _, item := range fv {
				if s, ok := item.(string); ok {
					toolFilter = append(toolFilter, s)
				}
			}
		case []string:
			toolFilter = fv
		}
	}

	return SpawnTask{Goal: goal, ToolFilter: toolFilter}, nil
}

// ToolDefSpawnAgentName returns a tool that spawns agents by their pre-defined YAML name.
func ToolDefSpawnAgentName(mgr *Manager, loader *AgentLoader) *base.ToolDef {
	// Build agent list for description
	agentNames := ""
	if loader != nil {
		agentNames = strings.Join(loader.Names(), ", ")
	}
	if agentNames == "" {
		agentNames = "(none loaded)"
	}

	return &base.ToolDef{
		Name:        "spawn_agent_name",
		Description: fmt.Sprintf("按预定义名称派生子Agent。可用Agent: %s。选择最匹配的agent名称，传入任务目标即可自动配置工具和超时。", agentNames),
		Category:    "agent",
		RiskLevel:   "mutation",
		Background:  true,
		Timeout:     15 * time.Minute,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name": {
					Type:        "string",
					Description: fmt.Sprintf("预定义的Agent名称。可用: %s", agentNames),
				},
				"goal": {
					Type:        "string",
					Description: "具体任务目标。留空则使用Agent默认系统提示。四段式：1.目标 2.范围 3.预期产物 4.成功标准",
				},
				"async": {
					Type:        "boolean",
					Description: "设为 true 时后台异步执行。耗时超过10秒的长任务应使用异步。",
				},
			},
			Required: []string{"name"},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleSpawnAgentName(mgr, loader, ctx, args)
		},
	}
}

func handleSpawnAgentName(mgr *Manager, loader *AgentLoader, ctx context.Context, args map[string]any) *base.ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "缺少 name 参数"},
			Summary: "参数错误：缺少 agent name",
		}
	}

	if loader == nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeUnknown, Message: "AgentLoader 未初始化"},
			Summary: "系统错误：Agent模板加载器未就绪",
		}
	}

	def := loader.Get(name)
	if def == nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeFileNotFound, Message: fmt.Sprintf("Agent '%s' 不存在", name)},
			Summary: fmt.Sprintf("未找到Agent '%s'。可用: %s", name, strings.Join(loader.Names(), ", ")),
		}
	}

	goal, _ := args["goal"].(string)
	task := def.ToSpawnTask(goal)
	async, _ := args["async"].(bool)

	// Set agent timeout from definition
	if def.ParseTimeout() > 0 {
		// timeout handled by ManagerConfig.AgentTimeout; we can override via the agent's own Timeout field
	}

	if async {
		sessionID := SessionIDFromCtx(ctx)
		agent := mgr.Spawn(task.Goal, task.ToolFilter, sessionID)
		agent.Timeout = int(def.ParseTimeout().Seconds())
		return &base.ToolResult{
			Status: base.StatusSuccess,
			Summary: fmt.Sprintf("已启动 Agent '%s' (ID: %s) 后台执行。完成后结果自动注入会话。", name, agent.ID),
			Data:    []string{agent.ID},
		}
	}

	// Sync mode: block and wait
	results := mgr.SpawnParallel([]SpawnTask{task}, "")

	var parts []string
	for _, r := range results {
		if r.Status == StatusDone {
			parts = append(parts, fmt.Sprintf("✅ %s: %s", r.Goal, r.Summary))
		} else {
			parts = append(parts, fmt.Sprintf("❌ %s: %s", r.Goal, r.Error))
		}
	}

	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: fmt.Sprintf("Agent '%s' 执行完成:\n%s", name, strings.Join(parts, "\n")),
		Data:    results,
	}
}

// ParseArgsString 解析 JSON 参数字符串
func ParseArgsString(raw string) map[string]any {
	if raw == "" || raw == "{}" || raw == "null" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return map[string]any{}
	}
	return args
}
