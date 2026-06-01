package todo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// ToolDef 返回 TodoWriteTool 的定义。
// mgr 用于读写 Todo 列表；onUpdate 是回调，用于向 SSE 通道推送 todo_update 事件。
func ToolDef(mgr *Manager, onUpdate func(sessionID string, lst *List)) *base.ToolDef {
	return &base.ToolDef{
		Name:        "todo_write",
		Description: "创建或更新当前对话的任务列表。每次调用全量替换。用于追踪多步任务的执行进度。每个任务包含 content（任务描述）、status（pending/in_progress/completed）、active_form（进行中的动词形式）。",
		Category:    "agent",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"todos": {
					Type:        "array",
					Description: "任务列表，每项包含 {id, content, status, active_form}",
				},
			},
			Required: []string{"todos"},
		},
		Examples: []base.ToolExample{
			{
				Description: "创建3个任务",
				Args: map[string]any{
					"todos": []map[string]any{
						{"id": 1, "content": "检查系统状态", "status": "pending", "active_form": "正在检查系统状态..."},
						{"id": 2, "content": "列出域名记录", "status": "pending", "active_form": "正在查询域名记录..."},
						{"id": 3, "content": "查看Docker容器", "status": "pending", "active_form": "正在获取容器列表..."},
					},
				},
			},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleTodoWrite(mgr, onUpdate, ctx, args)
		},
	}
}

func handleTodoWrite(mgr *Manager, onUpdate func(string, *List), ctx context.Context, args map[string]any) *base.ToolResult {
	// 从 context 获取 sessionID（由上层注入）
	sessionID, _ := ctx.Value(todoSessionKey{}).(string)
	if sessionID == "" {
		sessionID = "default"
	}

	rawTodos, ok := args["todos"]
	if !ok {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "缺少 todos 参数"},
			Summary: "参数错误：缺少 todos",
		}
	}

	var items []Item
	switch v := rawTodos.(type) {
	case []any:
		for _, raw := range v {
			item, err := parseItem(raw)
			if err != nil {
				return &base.ToolResult{
					Status:  base.StatusError,
					Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: err.Error()},
					Summary: "任务格式错误: " + err.Error(),
				}
			}
			items = append(items, item)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &items); err != nil {
			return &base.ToolResult{
				Status:  base.StatusError,
				Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "todos JSON 解析失败"},
				Summary: "参数格式错误",
			}
		}
	default:
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "todos 必须是数组"},
			Summary: "参数类型错误",
		}
	}

	if len(items) == 0 {
		// 空数组 = 清除任务列表
		mgr.Delete(sessionID)
		if onUpdate != nil {
			onUpdate(sessionID, &List{SessionID: sessionID, Items: []Item{}, UpdatedAt: time.Now()})
		}
		return &base.ToolResult{
			Status:  base.StatusSuccess,
			Summary: "任务列表已清除",
		}
	}

	lst := mgr.Update(sessionID, items)
	if onUpdate != nil {
		onUpdate(sessionID, lst)
	}

	pending := 0
	inProgress := 0
	done := 0
	for _, it := range items {
		switch it.Status {
		case StatusPending:
			pending++
		case StatusInProgress:
			inProgress++
		case StatusCompleted:
			done++
		}
	}

	parts := []string{}
	if inProgress > 0 {
		parts = append(parts, fmt.Sprintf("🔄 %d 进行中", inProgress))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("⏳ %d 待处理", pending))
	}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("✅ %d 已完成", done))
	}

	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: "任务列表已更新: " + strings.Join(parts, " | "),
		Data:    lst,
	}
}

func parseItem(raw any) (Item, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return Item{}, fmt.Errorf("每个 todo 必须是对象")
	}
	content, _ := m["content"].(string)
	if content == "" {
		return Item{}, fmt.Errorf("todo.content 不能为空")
	}
	statusStr, _ := m["status"].(string)
	var status Status
	switch statusStr {
	case "in_progress":
		status = StatusInProgress
	case "completed":
		status = StatusCompleted
	default:
		status = StatusPending
	}
	activeForm, _ := m["active_form"].(string)
	id := 0
	switch v := m["id"].(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	}
	return Item{ID: id, Content: content, Status: status, ActiveForm: activeForm}, nil
}

// todoSessionKey 用于在 context 中传递 sessionID
type todoSessionKey struct{}

// WithSessionID 将 sessionID 注入 context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, todoSessionKey{}, sessionID)
}
