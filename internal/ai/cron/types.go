// Package cron 提供 AI 驱动的定时任务调度。
package cron

import "time"

// ScheduledTask 一个由 AI 创建的定时任务
type ScheduledTask struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	CronExpr  string    `json:"cron_expr"`  // 标准 5 字段 cron: "m h dom mon dow"
	Prompt    string    `json:"prompt"`     // 触发时注入给 AI 的提示
	Recurring bool      `json:"recurring"`  // true=重复，false=一次性
	CreatedAt time.Time `json:"created_at"`
	LastRanAt time.Time `json:"last_ran_at,omitempty"`
	NextRunAt time.Time `json:"next_run_at,omitempty"`
}
