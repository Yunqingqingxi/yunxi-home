package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yxd/yunxi-home/internal/ai/base"
)

// ToolDef 返回 AgentTool 的定义。
// AI 调用此工具来派生子 Agent 处理并行子任务。
func ToolDef(mgr *Manager) *base.ToolDef {
	return &base.ToolDef{
		Name:        "spawn_agent",
		Description: "派生一个或多个子 Agent 来并行处理子任务。当用户的任务可分解为多个独立子任务时（如同时检查多个系统），使用此工具并行执行。每个子 Agent 有独立上下文和受限工具。",
		Category:    "agent",
		RiskLevel:   "mutation",
		Timeout:     15 * time.Minute,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"tasks": {
					Type:        "array",
					Description: "子任务列表，每项包含 goal(任务描述) 和 tool_filter(允许的工具名列表)",
					Items: &base.ParamProp{
						Type: "object",
						Properties: map[string]base.ParamProp{
							"goal": {
								Type:        "string",
								Description: "子任务的详细目标描述，需要具体明确",
							},
							"tool_filter": {
								Type:        "array",
								Description: "允许子 Agent 使用的工具名列表，至少指定一个工具",
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
