package database

import (
	"context"
	"fmt"

	"github.com/yxd/yunxi-home/internal/database/base"
)

// Ensure interface compliance at compile time.
var _ base.TodoRepository = (*sqliteTodoRepo)(nil)
var _ base.TodoRepository = (*mysqlTodoRepo)(nil)

// ── SQLite todo repo ──────────────────────────────────────────────

type sqliteTodoRepo struct{ db Executor }

func (r *sqliteTodoRepo) Upsert(ctx context.Context, sessionID, itemsJSON string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_todos (session_id, items_json, updated_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(session_id) DO UPDATE SET items_json=excluded.items_json, updated_at=datetime('now')`,
		sessionID, itemsJSON)
	if err != nil {
		return fmt.Errorf("upsert session_todos: %w", err)
	}
	return nil
}

func (r *sqliteTodoRepo) Get(ctx context.Context, sessionID string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx, "SELECT items_json FROM session_todos WHERE session_id=?", sessionID).Scan(&data)
	if err != nil {
		return "", nil
	}
	return data, nil
}

func (r *sqliteTodoRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM session_todos WHERE session_id=?", sessionID)
	return err
}

// ── MySQL todo repo ────────────────────────────────────────────────

type mysqlTodoRepo struct{ db Executor }

func (r *mysqlTodoRepo) Upsert(ctx context.Context, sessionID, itemsJSON string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_todos (session_id, items_json, updated_at)
		 VALUES (?, ?, NOW())
		 ON DUPLICATE KEY UPDATE items_json=VALUES(items_json), updated_at=NOW()`,
		sessionID, itemsJSON)
	if err != nil {
		return fmt.Errorf("upsert session_todos: %w", err)
	}
	return nil
}

func (r *mysqlTodoRepo) Get(ctx context.Context, sessionID string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx, "SELECT items_json FROM session_todos WHERE session_id=?", sessionID).Scan(&data)
	if err != nil {
		return "", nil
	}
	return data, nil
}

func (r *mysqlTodoRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM session_todos WHERE session_id=?", sessionID)
	return err
}
