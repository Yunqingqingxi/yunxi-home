package memory

import (
	"context"
	"database/sql"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var repoLog = logger.ForComponent("ai.memory.repo")

// DBRepo persists memories in SQLite.
type DBRepo struct {
	db *sql.DB
}

var _ Repository = (*DBRepo)(nil)

// NewDBRepo creates a new DBRepo.
func NewDBRepo(db *sql.DB) *DBRepo {
	return &DBRepo{db: db}
}

// EnsureSchema creates the memories table if not exists.
func (r *DBRepo) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memories (
			name        TEXT PRIMARY KEY,
			description TEXT NOT NULL DEFAULT '',
			type        TEXT NOT NULL DEFAULT 'reference',
			content     TEXT NOT NULL DEFAULT '',
			source      TEXT NOT NULL DEFAULT 'file',
			created_at  TEXT NOT NULL DEFAULT '',
			updated_at  TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		repoLog.Error("创建 memories 表失败", "error", err)
	}
	return err
}

// GetAll returns all memories.
func (r *DBRepo) GetAll(ctx context.Context) ([]*Memory, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT name, description, type, content, source, created_at, updated_at FROM memories ORDER BY name`)
	if err != nil {
		repoLog.Error("查询全部记忆失败", "error", err)
		return nil, err
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		m := &Memory{}
		var createdAt, updatedAt string
		if err := rows.Scan(&m.Name, &m.Description, &m.Type, &m.Content, &m.Source, &createdAt, &updatedAt); err != nil {
			repoLog.Error("扫描记忆行失败", "error", err)
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		memories = append(memories, m)
	}
	if err := rows.Err(); err != nil {
		repoLog.Error("遍历记忆结果失败", "error", err)
	}
	return memories, err
}

// GetByName returns a single memory by name.
func (r *DBRepo) GetByName(ctx context.Context, name string) (*Memory, error) {
	m := &Memory{}
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT name, description, type, content, source, created_at, updated_at FROM memories WHERE name = ?`,
		name,
	).Scan(&m.Name, &m.Description, &m.Type, &m.Content, &m.Source, &createdAt, &updatedAt)
	if err != nil {
		if err != sql.ErrNoRows {
			repoLog.Error("查询记忆失败", "name", name, "error", err)
		}
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return m, nil
}

// Save inserts or replaces a memory.
func (r *DBRepo) Save(ctx context.Context, m *Memory) error {
	m.UpdatedAt = time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = m.UpdatedAt
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO memories (name, description, type, content, source, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Name, m.Description, string(m.Type), m.Content, m.Source,
		m.CreatedAt.Format(time.RFC3339),
		m.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		repoLog.Error("保存记忆失败", "name", m.Name, "error", err)
	}
	return err
}

// Delete removes a memory by name.
func (r *DBRepo) Delete(ctx context.Context, name string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM memories WHERE name = ?`, name)
	if err != nil {
		repoLog.Error("删除记忆失败", "name", name, "error", err)
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		repoLog.Warn("删除记忆时未找到记录", "name", name)
	}
	return nil
}
