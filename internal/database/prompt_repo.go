package database

import (
	"context"
	"fmt"
	"time"

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

// EnsureSchema creates the prompts table if not exists.
func (r *sqlitePromptRepo) EnsureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS prompts (
			id         TEXT PRIMARY KEY,
			category   TEXT NOT NULL DEFAULT 'general',
			name       TEXT NOT NULL DEFAULT '',
			content    TEXT NOT NULL DEFAULT '',
			keywords   TEXT NOT NULL DEFAULT '',
			priority   INTEGER NOT NULL DEFAULT 0,
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
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
	var enabled int
	var createdAtStr, updatedAtStr string
	err := r.db.QueryRowContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE id=?",
		id).Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &enabled, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, nil // not found
	}
	p.Enabled = enabled != 0
	p.CreatedAt = parseTime(createdAtStr)
	p.UpdatedAt = parseTime(updatedAtStr)
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
	// 确保表存在
	if _, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS prompts (
			id         TEXT PRIMARY KEY,
			category   TEXT NOT NULL DEFAULT 'general',
			name       TEXT NOT NULL DEFAULT '',
			content    TEXT NOT NULL DEFAULT '',
			keywords   TEXT NOT NULL DEFAULT '',
			priority   INTEGER NOT NULL DEFAULT 0,
			enabled    INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)
	`); err != nil {
		return fmt.Errorf("ensure prompts table: %w", err)
	}

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
	var enabled int
	var createdAtStr, updatedAtStr string
	err := r.db.QueryRowContext(ctx,
		"SELECT id, category, name, content, keywords, priority, enabled, created_at, updated_at FROM prompts WHERE id=?",
		id).Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &enabled, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, nil
	}
	p.Enabled = enabled != 0
	p.CreatedAt = parseTime(createdAtStr)
	p.UpdatedAt = parseTime(updatedAtStr)
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
	if _, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS prompts (
			id         VARCHAR(255) PRIMARY KEY,
			category   VARCHAR(50) NOT NULL DEFAULT 'general',
			name       VARCHAR(255) NOT NULL DEFAULT '',
			content    TEXT NOT NULL,
			keywords   VARCHAR(500) NOT NULL DEFAULT '',
			priority   INT NOT NULL DEFAULT 0,
			enabled    TINYINT NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT NOW(),
			updated_at DATETIME NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure prompts table: %w", err)
	}

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
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&p.ID, &p.Category, &p.Name, &p.Content, &p.Keywords, &p.Priority, &enabled, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan prompt: %w", err)
		}
		p.Enabled = enabled != 0
		p.CreatedAt = parseTime(createdAtStr)
		p.UpdatedAt = parseTime(updatedAtStr)
		result = append(result, p)
	}
	return result, rows.Err()
}

// parseTime parses a time string from DB. Supports SQLite datetime() and RFC3339 formats.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// SQLite datetime('now') / MySQL NOW() format: "2006-01-02 15:04:05"
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	// RFC3339 format: "2006-01-02T15:04:05Z"
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}
