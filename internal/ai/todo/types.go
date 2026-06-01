// Package todo 提供会话级别的多步任务进度追踪。
// 对应 claude-code 的 TodoWrite 工具概念 — AI 在对话中创建/更新任务列表。
package todo

import "time"

// Status 任务状态
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

// Item 单个任务项
type Item struct {
	ID         int    `json:"id"`
	Content    string `json:"content"`
	Status     Status `json:"status"`
	ActiveForm string `json:"active_form"` // 进行中的动词形式，如 "查看系统状态..."
}

// List 会话级的 Todo 列表
type List struct {
	SessionID string    `json:"session_id"`
	Items     []Item    `json:"items"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Progress 返回完成百分比 (0-100)
func (l *List) Progress() int {
	if len(l.Items) == 0 {
		return 0
	}
	done := 0
	for _, it := range l.Items {
		if it.Status == StatusCompleted {
			done++
		}
	}
	return done * 100 / len(l.Items)
}

// CompletedCount 返回已完成和总任务数
func (l *List) CompletedCount() (done, total int) {
	total = len(l.Items)
	for _, it := range l.Items {
		if it.Status == StatusCompleted {
			done++
		}
	}
	return
}
