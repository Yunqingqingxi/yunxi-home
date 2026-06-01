package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/cron"
)

// CronTaskRepo SQLite 持久化 cron.Manager 的定时任务
type CronTaskRepo struct {
	db *sql.DB
}

var _ cron.TaskRepository = (*CronTaskRepo)(nil)

// NewCronTaskRepo 创建 CronTaskRepo
func NewCronTaskRepo(db *sql.DB) *CronTaskRepo {
	return &CronTaskRepo{db: db}
}

// EnsureSchema 创建 cron_tasks 表（幂等）
func (r *CronTaskRepo) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS cron_tasks (
			id          TEXT PRIMARY KEY,
			session_id  TEXT NOT NULL,
			cron_expr   TEXT NOT NULL DEFAULT '',
			prompt      TEXT NOT NULL DEFAULT '',
			recurring   INTEGER NOT NULL DEFAULT 1,
			created_at  TEXT NOT NULL DEFAULT '',
			last_ran_at TEXT NOT NULL DEFAULT '',
			next_run_at TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
}

// Save 插入或更新定时任务
func (r *CronTaskRepo) Save(task *cron.ScheduledTask) error {
	_, err := r.db.Exec(
		`INSERT OR REPLACE INTO cron_tasks (id, session_id, cron_expr, prompt, recurring, created_at, last_ran_at, next_run_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID,
		task.SessionID,
		task.CronExpr,
		task.Prompt,
		boolToInt(task.Recurring),
		task.CreatedAt.Format(time.RFC3339),
		task.LastRanAt.Format(time.RFC3339),
		task.NextRunAt.Format(time.RFC3339),
	)
	return err
}

// Delete 删除定时任务
func (r *CronTaskRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM cron_tasks WHERE id = ?`, id)
	return err
}

// ListBySession 列出指定会话的任务
func (r *CronTaskRepo) ListBySession(sessionID string) ([]*cron.ScheduledTask, error) {
	rows, err := r.db.Query(
		`SELECT id, session_id, cron_expr, prompt, recurring, created_at, last_ran_at, next_run_at
		 FROM cron_tasks WHERE session_id = ? ORDER BY created_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCronTasks(rows)
}

// ListAll 列出所有任务
func (r *CronTaskRepo) ListAll() ([]*cron.ScheduledTask, error) {
	rows, err := r.db.Query(
		`SELECT id, session_id, cron_expr, prompt, recurring, created_at, last_ran_at, next_run_at
		 FROM cron_tasks ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCronTasks(rows)
}

func scanCronTasks(rows *sql.Rows) ([]*cron.ScheduledTask, error) {
	var tasks []*cron.ScheduledTask
	for rows.Next() {
		var t cron.ScheduledTask
		var recurring int
		var createdAt, lastRanAt, nextRunAt string
		if err := rows.Scan(&t.ID, &t.SessionID, &t.CronExpr, &t.Prompt, &recurring,
			&createdAt, &lastRanAt, &nextRunAt); err != nil {
			return nil, err
		}
		t.Recurring = recurring != 0
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.LastRanAt, _ = time.Parse(time.RFC3339, lastRanAt)
		t.NextRunAt, _ = time.Parse(time.RFC3339, nextRunAt)
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}

