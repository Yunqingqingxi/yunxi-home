package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// cronSessionKey 用于在 context 中传递 sessionID
type cronSessionKey struct{}

// WithSessionID 将 sessionID 注入 context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, cronSessionKey{}, sessionID)
}

// ── CronCreateTool ────────────────────────────────────────

// CreateTool 创建 CronCreateTool 的定义
func CreateTool(mgr *Manager) *base.ToolDef {
	return &base.ToolDef{
		Name:        "cron_create",
		Description: "创建一个定时任务，到指定时间后向当前会话注入提示消息。适用于定时检查、定期备份、定时提醒等场景。cron 表达式为标准 5 字段格式：分 时 日 月 周。如 '*/5 * * * *' 表示每5分钟。",
		Category:    "agent",
		RiskLevel:   "mutation",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"cron_expr": {Type: "string", Description: "标准 5 字段 cron 表达式：min hour dom month dow。例如 '*/5 * * * *' (每5分钟), '0 9 * * *' (每天9点)"},
				"prompt":    {Type: "string", Description: "定时触发时注入的提示消息。例如 '检查系统状态并报告'"},
				"recurring": {Type: "boolean", Description: "是否重复执行。true=重复，false=一次性。默认 true"},
			},
			Required: []string{"cron_expr", "prompt"},
		},
		Examples: []base.ToolExample{
			{
				Description: "每5分钟检查系统状态",
				Args: map[string]any{
					"cron_expr": "*/5 * * * *",
					"prompt":    "请检查当前系统状态并报告",
					"recurring": true,
				},
			},
			{
				Description: "明天9点提醒",
				Args: map[string]any{
					"cron_expr": "0 9 * * *",
					"prompt":    "现在已是上午9点，请执行每日健康检查",
					"recurring": true,
				},
			},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleCreate(mgr, ctx, args)
		},
	}
}

func handleCreate(mgr *Manager, ctx context.Context, args map[string]any) *base.ToolResult {
	sessionID, _ := ctx.Value(cronSessionKey{}).(string)
	if sessionID == "" {
		sessionID = "default"
	}

	cronExpr, _ := args["cron_expr"].(string)
	prompt, _ := args["prompt"].(string)
	recurring := true
	if v, ok := args["recurring"].(bool); ok {
		recurring = v
	}

	if cronExpr == "" || prompt == "" {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "cron_expr 和 prompt 不能为空"},
			Summary: "参数不完整",
		}
	}

	task, err := mgr.Create(sessionID, cronExpr, prompt, recurring)
	if err != nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: err.Error()},
			Summary: "创建定时任务失败: " + err.Error(),
		}
	}

	desc := DescribeCron(cronExpr)
	recurLabel := "重复"
	if !recurring {
		recurLabel = "一次性"
	}

	summary := fmt.Sprintf("✅ 定时任务已创建: %s (%s, %s)", desc, recurLabel, task.ID)
	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Data:    task,
		Summary: summary,
	}
}

// ── CronDeleteTool ────────────────────────────────────────

// DeleteTool 创建 CronDeleteTool 的定义
func DeleteTool(mgr *Manager) *base.ToolDef {
	return &base.ToolDef{
		Name:        "cron_delete",
		Description: "删除一个之前创建的定时任务。",
		Category:    "agent",
		RiskLevel:   "mutation",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"task_id": {Type: "string", Description: "要删除的定时任务 ID"},
			},
			Required: []string{"task_id"},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleDelete(mgr, args)
		},
	}
}

func handleDelete(mgr *Manager, args map[string]any) *base.ToolResult {
	taskID, _ := args["task_id"].(string)
	if taskID == "" {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "task_id 不能为空"},
			Summary: "缺少 task_id",
		}
	}

	if !mgr.Delete(taskID) {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: "NOT_FOUND", Message: "任务不存在: " + taskID},
			Summary: "任务不存在",
		}
	}

	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: "定时任务已删除: " + taskID,
	}
}

// ── CronListTool ──────────────────────────────────────────

// ListTool 创建 CronListTool 的定义
func ListTool(mgr *Manager) *base.ToolDef {
	return &base.ToolDef{
		Name:        "cron_list",
		Description: "列出当前会话的所有定时任务。",
		Category:    "agent",
		RiskLevel:   "readonly",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleList(mgr, ctx)
		},
	}
}

func handleList(mgr *Manager, ctx context.Context) *base.ToolResult {
	sessionID, _ := ctx.Value(cronSessionKey{}).(string)
	if sessionID == "" {
		sessionID = "default"
	}

	tasks := mgr.ListBySession(sessionID)
	data, _ := json.Marshal(tasks)

	if len(tasks) == 0 {
		return &base.ToolResult{
			Status:  base.StatusSuccess,
			Data:    "[]",
			Summary: "当前没有定时任务",
		}
	}

	var parts []string
	for _, t := range tasks {
		desc := DescribeCron(t.CronExpr)
		parts = append(parts, fmt.Sprintf("- %s: %s (ID: %s, 下次: %s)", t.Prompt, desc, t.ID, t.NextRunAt.Format("15:04")))
	}
	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Data:    string(data),
		Summary: "定时任务:\n" + strings.Join(parts, "\n"),
	}
}
