package database

import (
	"context"
	"fmt"

	"github.com/yxd/yunxi-home/internal/database/base"
)

// Ensure interface compliance at compile time.
var _ base.GoalRepository = (*sqliteGoalRepo)(nil)
var _ base.GoalRepository = (*mysqlGoalRepo)(nil)

// ── SQLite goal repo ──────────────────────────────────────────────

type sqliteGoalRepo struct{ db Executor }

func (r *sqliteGoalRepo) Upsert(ctx context.Context, sessionID, goalsJSON string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_goals (id, session_id, title, steps_json, status, created_at, updated_at)
		 VALUES (?, ?, '', ?, 'active', datetime('now'), datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET steps_json=excluded.steps_json, updated_at=datetime('now')`,
		sessionID, sessionID, goalsJSON)
	if err != nil {
		return fmt.Errorf("upsert session_goals: %w", err)
	}
	return nil
}

func (r *sqliteGoalRepo) Get(ctx context.Context, sessionID string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx, "SELECT steps_json FROM session_goals WHERE session_id=?", sessionID).Scan(&data)
	if err != nil {
		return "", nil
	}
	return data, nil
}

func (r *sqliteGoalRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM session_goals WHERE session_id=?", sessionID)
	return err
}

// ── MySQL goal repo ────────────────────────────────────────────────

type mysqlGoalRepo struct{ db Executor }

func (r *mysqlGoalRepo) Upsert(ctx context.Context, sessionID, goalsJSON string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_goals (id, session_id, title, steps_json, status, created_at, updated_at)
		 VALUES (?, ?, '', ?, 'active', NOW(), NOW())
		 ON DUPLICATE KEY UPDATE steps_json=VALUES(steps_json), updated_at=NOW()`,
		sessionID, sessionID, goalsJSON)
	if err != nil {
		return fmt.Errorf("upsert session_goals: %w", err)
	}
	return nil
}

func (r *mysqlGoalRepo) Get(ctx context.Context, sessionID string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx, "SELECT steps_json FROM session_goals WHERE session_id=?", sessionID).Scan(&data)
	if err != nil {
		return "", nil
	}
	return data, nil
}

func (r *mysqlGoalRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM session_goals WHERE session_id=?", sessionID)
	return err
}
