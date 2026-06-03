package database

import (
	"context"
	"fmt"

	"github.com/Yunqingqingxi/yunxi-home/internal/database/base"
)

// Ensure interface compliance at compile time.
var _ base.PromptRepository = (*sqlitePromptRepo)(nil)
var _ base.PromptRepository = (*mysqlPromptRepo)(nil)

// ── SQLite prompt repo ──────────────────────────────────────────────

type sqlitePromptRepo struct{ db Executor }

// NewPromptRepo creates a SQLite-backed PromptRepository.
func NewPromptRepo(db Executor) base.PromptRepository {
	return &sqlitePromptRepo{db: db}
}

func (r *sqlitePromptRepo) GetByCategory(ctx context.Context, category string) ([]base.PromptRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE category=? ORDER BY priority DESC, id",
		category)
	if err != nil {
		return nil, fmt.Errorf("query prompts by category: %w", err)
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *sqlitePromptRepo) GetAll(ctx context.Context) ([]base.PromptRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts ORDER BY category, priority DESC, id")
	if err != nil {
		return nil, fmt.Errorf("query all prompts: %w", err)
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *sqlitePromptRepo) GetByID(ctx context.Context, id string) (*base.PromptRecord, error) {
	var p base.PromptRecord
	err := r.db.QueryRowContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE id=?",
		id).Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, nil // not found
	}
	return &p, nil
}

func (r *sqlitePromptRepo) Upsert(ctx context.Context, p *base.PromptRecord) error {
	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO prompts (id, category, name, content, keywords, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
		 category=excluded.category, name=excluded.name, content=excluded.content,
		 keywords=excluded.keywords, priority=excluded.priority, enabled=excluded.enabled,
		 updated_at=datetime('now')`,
		p.ID, p.Category, p.Name, p.Content, p.Keywords, p.Priority, enabled)
	if err != nil {
		return fmt.Errorf("upsert prompt %s: %w", p.ID, err)
	}
	return nil
}

func (r *sqlitePromptRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM prompts WHERE id=?", id)
	return err
}

func (r *sqlitePromptRepo) InitDefaults(ctx context.Context, prompts []base.PromptRecord) error {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM prompts").Scan(&count); err != nil {
		return fmt.Errorf("count prompts: %w", err)
	}
	if count > 0 {
		return nil
	}
	for _, p := range prompts {
		if err := r.Upsert(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

// ── MySQL prompt repo ────────────────────────────────────────────────

type mysqlPromptRepo struct{ db Executor }

// NewMySQLPromptRepo creates a MySQL-backed PromptRepository.
func NewMySQLPromptRepo(db Executor) base.PromptRepository {
	return &mysqlPromptRepo{db: db}
}

func (r *mysqlPromptRepo) GetByCategory(ctx context.Context, category string) ([]base.PromptRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE category=? ORDER BY priority DESC, id",
		category)
	if err != nil {
		return nil, fmt.Errorf("query prompts by category: %w", err)
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *mysqlPromptRepo) GetAll(ctx context.Context) ([]base.PromptRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts ORDER BY category, priority DESC, id")
	if err != nil {
		return nil, fmt.Errorf("query all prompts: %w", err)
	}
	defer rows.Close()
	return scanPrompts(rows)
}

func (r *mysqlPromptRepo) GetByID(ctx context.Context, id string) (*base.PromptRecord, error) {
	var p base.PromptRecord
	err := r.db.QueryRowContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE id=?",
		id).Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, nil
	}
	return &p, nil
}

func (r *mysqlPromptRepo) Upsert(ctx context.Context, p *base.PromptRecord) error {
	enabled := 0
	if p.Enabled {
		enabled = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO prompts (id, category, name, content, keywords, priority, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		 ON DUPLICATE KEY UPDATE
		 category=VALUES(category), name=VALUES(name), content=VALUES(content),
		 keywords=VALUES(keywords), priority=VALUES(priority), enabled=VALUES(enabled),
		 updated_at=NOW()`,
		p.ID, p.Category, p.Name, p.Content, p.Keywords, p.Priority, enabled)
	if err != nil {
		return fmt.Errorf("upsert prompt %s: %w", p.ID, err)
	}
	return nil
}

func (r *mysqlPromptRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM prompts WHERE id=?", id)
	return err
}

func (r *mysqlPromptRepo) InitDefaults(ctx context.Context, prompts []base.PromptRecord) error {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM prompts").Scan(&count); err != nil {
		return fmt.Errorf("count prompts: %w", err)
	}
	if count > 0 {
		return nil
	}
	for _, p := range prompts {
		if err := r.Upsert(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

// ── Shared helpers ───────────────────────────────────────────────────

type promptRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

func scanPrompts(rows promptRows) ([]base.PromptRecord, error) {
	var result []base.PromptRecord
	for rows.Next() {
		var p base.PromptRecord
		var enabled int
		if err := rows.Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt: %w", err)
		}
		p.Enabled = enabled != 0
		result = append(result, p)
	}
	return result, rows.Err()
}
