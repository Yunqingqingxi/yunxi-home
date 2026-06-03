// Package adapt provides a cross-subsystem User Adaptation Layer that learns
// from every interaction and feeds improvements back into memory, prompt, agent,
// topology, and observability subsystems.
package adapt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var log = logger.ForComponent("adapt")

// Repository persists adaptation data (profiles, feedback, session summaries).
type Repository interface {
	EnsureSchema(ctx context.Context) error

	// Profiles
	GetProfile(ctx context.Context, userID string) (*UserProfile, error)
	UpsertProfile(ctx context.Context, p *UserProfile) error

	// Feedback
	RecordFeedback(ctx context.Context, ev *FeedbackEvent) error
	GetRecentFeedback(ctx context.Context, userID string, limit int) ([]FeedbackEvent, error)

	// Session summaries
	SaveSessionSummary(ctx context.Context, s *SessionSummary) error
	GetSessionSummaries(ctx context.Context, userID string, limit int) ([]SessionSummary, error)

	// Prompt effectiveness
	GetPromptEffectiveness(ctx context.Context, promptID, variant string) (*PromptEffectiveness, error)
	UpsertPromptEffectiveness(ctx context.Context, pe *PromptEffectiveness) error
	ListPromptEffectiveness(ctx context.Context) ([]PromptEffectiveness, error)

	// Tool outcomes
	RecordToolOutcome(ctx context.Context, o *ToolOutcome) error
	GetRecentToolOutcomes(ctx context.Context, userID string, limit int) ([]ToolOutcome, error)
}

// SQLiteRepo implements Repository backed by a *sql.DB.
type SQLiteRepo struct {
	db *sql.DB
	mu sync.Mutex
}

func NewSQLiteRepo(db *sql.DB) *SQLiteRepo {
	return &SQLiteRepo{db: db}
}

func (r *SQLiteRepo) EnsureSchema(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	schema := `
	CREATE TABLE IF NOT EXISTS adapt_profiles (
		user_id TEXT PRIMARY KEY,
		profile_json TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS adapt_feedback (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		session_id TEXT NOT NULL,
		type TEXT NOT NULL,
		tool_name TEXT DEFAULT '',
		task_category TEXT DEFAULT '',
		original_msg TEXT DEFAULT '',
		edited_msg TEXT DEFAULT '',
		detail TEXT DEFAULT '',
		timestamp TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_adapt_feedback_user ON adapt_feedback(user_id, timestamp);

	CREATE TABLE IF NOT EXISTS adapt_session_summaries (
		session_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		task_category TEXT DEFAULT '',
		rounds INTEGER DEFAULT 0,
		tool_calls INTEGER DEFAULT 0,
		tool_successes INTEGER DEFAULT 0,
		tool_failures INTEGER DEFAULT 0,
		topo_rejects INTEGER DEFAULT 0,
		tokens_in INTEGER DEFAULT 0,
		tokens_out INTEGER DEFAULT 0,
		trust_locked INTEGER DEFAULT 0,
		completed INTEGER DEFAULT 0,
		duration_sec REAL DEFAULT 0,
		timestamp TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_adapt_sessions_user ON adapt_session_summaries(user_id, timestamp);

	CREATE TABLE IF NOT EXISTS adapt_prompt_effectiveness (
		prompt_id TEXT NOT NULL,
		variant TEXT NOT NULL DEFAULT 'default',
		use_count INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		edit_count INTEGER DEFAULT 0,
		cancel_count INTEGER DEFAULT 0,
		avg_rounds REAL DEFAULT 0,
		PRIMARY KEY (prompt_id, variant)
	);

	CREATE TABLE IF NOT EXISTS adapt_tool_outcomes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		round INTEGER DEFAULT 0,
		tool_name TEXT NOT NULL,
		success INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		topo_passed INTEGER DEFAULT 0,
		topo_rejected INTEGER DEFAULT 0,
		trust_locked INTEGER DEFAULT 0,
		timestamp TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_adapt_outcomes_session ON adapt_tool_outcomes(session_id);
	CREATE INDEX IF NOT EXISTS idx_adapt_outcomes_tool ON adapt_tool_outcomes(tool_name);
	`

	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("adapt schema: %w", err)
	}
	log.Info("adapt schema ensured")
	return nil
}

// ── Profile CRUD ──────────────────────────────────────────────────────────────

func (r *SQLiteRepo) GetProfile(ctx context.Context, userID string) (*UserProfile, error) {
	var jsonStr string
	err := r.db.QueryRowContext(ctx,
		`SELECT profile_json FROM adapt_profiles WHERE user_id = ?`,
		userID,
	).Scan(&jsonStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	var p UserProfile
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}
	return &p, nil
}

func (r *SQLiteRepo) UpsertProfile(ctx context.Context, p *UserProfile) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	p.UpdatedAt = time.Now().UTC()
	// Re-marshal after timestamps updated
	data, _ = json.Marshal(p)

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO adapt_profiles (user_id, profile_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		 profile_json=excluded.profile_json, updated_at=excluded.updated_at`,
		p.UserID, string(data), p.CreatedAt.Format(time.RFC3339), now,
	)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

// ── Feedback ──────────────────────────────────────────────────────────────────

func (r *SQLiteRepo) RecordFeedback(ctx context.Context, ev *FeedbackEvent) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO adapt_feedback (id, user_id, session_id, type, tool_name, task_category, original_msg, edited_msg, detail, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, ev.UserID, ev.SessionID, string(ev.Type),
		ev.ToolName, ev.TaskCategory, ev.OriginalMsg, ev.EditedMsg, ev.Detail,
		ev.Timestamp.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *SQLiteRepo) GetRecentFeedback(ctx context.Context, userID string, limit int) ([]FeedbackEvent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, session_id, type, tool_name, task_category, original_msg, edited_msg, detail, timestamp
		 FROM adapt_feedback WHERE user_id = ? ORDER BY timestamp DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FeedbackEvent
	for rows.Next() {
		var ev FeedbackEvent
		var ts string
		if err := rows.Scan(&ev.ID, &ev.UserID, &ev.SessionID, &ev.Type, &ev.ToolName,
			&ev.TaskCategory, &ev.OriginalMsg, &ev.EditedMsg, &ev.Detail, &ts); err != nil {
			return nil, err
		}
		ev.Timestamp, _ = time.Parse(time.RFC3339, ts)
		result = append(result, ev)
	}
	return result, nil
}

// ── Session Summaries ─────────────────────────────────────────────────────────

func (r *SQLiteRepo) SaveSessionSummary(ctx context.Context, s *SessionSummary) error {
	trust := 0
	if s.TrustLocked {
		trust = 1
	}
	completed := 0
	if s.Completed {
		completed = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO adapt_session_summaries
		 (session_id, user_id, task_category, rounds, tool_calls, tool_successes, tool_failures,
		  topo_rejects, tokens_in, tokens_out, trust_locked, completed, duration_sec, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		 rounds=excluded.rounds, tool_calls=excluded.tool_calls, tool_successes=excluded.tool_successes,
		 tool_failures=excluded.tool_failures, topo_rejects=excluded.topo_rejects,
		 tokens_in=excluded.tokens_in, tokens_out=excluded.tokens_out,
		 trust_locked=excluded.trust_locked, completed=excluded.completed,
		 duration_sec=excluded.duration_sec`,
		s.SessionID, s.UserID, s.TaskCategory, s.Rounds, s.ToolCalls,
		s.ToolSuccesses, s.ToolFailures, s.TopoRejects,
		s.TokensIn, s.TokensOut, trust, completed, s.DurationSec,
		s.Timestamp.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *SQLiteRepo) GetSessionSummaries(ctx context.Context, userID string, limit int) ([]SessionSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT session_id, user_id, task_category, rounds, tool_calls, tool_successes, tool_failures,
		 topo_rejects, tokens_in, tokens_out, trust_locked, completed, duration_sec, timestamp
		 FROM adapt_session_summaries WHERE user_id = ? ORDER BY timestamp DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SessionSummary
	for rows.Next() {
		var s SessionSummary
		var trust, completed int
		var ts string
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.TaskCategory, &s.Rounds, &s.ToolCalls,
			&s.ToolSuccesses, &s.ToolFailures, &s.TopoRejects, &s.TokensIn, &s.TokensOut,
			&trust, &completed, &s.DurationSec, &ts); err != nil {
			return nil, err
		}
		s.TrustLocked = trust != 0
		s.Completed = completed != 0
		s.Timestamp, _ = time.Parse(time.RFC3339, ts)
		result = append(result, s)
	}
	return result, nil
}

// ── Prompt Effectiveness ──────────────────────────────────────────────────────

func (r *SQLiteRepo) GetPromptEffectiveness(ctx context.Context, promptID, variant string) (*PromptEffectiveness, error) {
	if variant == "" {
		variant = "default"
	}
	var pe PromptEffectiveness
	err := r.db.QueryRowContext(ctx,
		`SELECT prompt_id, variant, use_count, success_count, edit_count, cancel_count, avg_rounds
		 FROM adapt_prompt_effectiveness WHERE prompt_id = ? AND variant = ?`,
		promptID, variant,
	).Scan(&pe.PromptID, &pe.Variant, &pe.UseCount, &pe.SuccessCount, &pe.EditCount, &pe.CancelCount, &pe.AvgRounds)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pe, nil
}

func (r *SQLiteRepo) UpsertPromptEffectiveness(ctx context.Context, pe *PromptEffectiveness) error {
	if pe.Variant == "" {
		pe.Variant = "default"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO adapt_prompt_effectiveness (prompt_id, variant, use_count, success_count, edit_count, cancel_count, avg_rounds)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(prompt_id, variant) DO UPDATE SET
		 use_count=excluded.use_count, success_count=excluded.success_count,
		 edit_count=excluded.edit_count, cancel_count=excluded.cancel_count,
		 avg_rounds=excluded.avg_rounds`,
		pe.PromptID, pe.Variant, pe.UseCount, pe.SuccessCount, pe.EditCount, pe.CancelCount, pe.AvgRounds,
	)
	return err
}

func (r *SQLiteRepo) ListPromptEffectiveness(ctx context.Context) ([]PromptEffectiveness, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT prompt_id, variant, use_count, success_count, edit_count, cancel_count, avg_rounds
		 FROM adapt_prompt_effectiveness ORDER BY success_count DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PromptEffectiveness
	for rows.Next() {
		var pe PromptEffectiveness
		if err := rows.Scan(&pe.PromptID, &pe.Variant, &pe.UseCount, &pe.SuccessCount,
			&pe.EditCount, &pe.CancelCount, &pe.AvgRounds); err != nil {
			return nil, err
		}
		result = append(result, pe)
	}
	return result, nil
}

// ── Tool Outcomes ─────────────────────────────────────────────────────────────

func (r *SQLiteRepo) RecordToolOutcome(ctx context.Context, o *ToolOutcome) error {
	success := 0
	if o.Success {
		success = 1
	}
	topoPassed := 0
	if o.TopoPassed {
		topoPassed = 1
	}
	topoRejected := 0
	if o.TopoRejected {
		topoRejected = 1
	}
	trustLocked := 0
	if o.TrustLocked {
		trustLocked = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO adapt_tool_outcomes (session_id, round, tool_name, success, duration_ms, topo_passed, topo_rejected, trust_locked, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.SessionID, o.Round, o.ToolName, success, o.DurationMs,
		topoPassed, topoRejected, trustLocked,
		o.Timestamp.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *SQLiteRepo) GetRecentToolOutcomes(ctx context.Context, userID string, limit int) ([]ToolOutcome, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT t.session_id, t.round, t.tool_name, t.success, t.duration_ms,
		 t.topo_passed, t.topo_rejected, t.trust_locked, t.timestamp
		 FROM adapt_tool_outcomes t
		 INNER JOIN adapt_session_summaries s ON t.session_id = s.session_id
		 WHERE s.user_id = ?
		 ORDER BY t.timestamp DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ToolOutcome
	for rows.Next() {
		var o ToolOutcome
		var success, topoPassed, topoRejected, trustLocked int
		var ts string
		if err := rows.Scan(&o.SessionID, &o.Round, &o.ToolName, &success, &o.DurationMs,
			&topoPassed, &topoRejected, &trustLocked, &ts); err != nil {
			return nil, err
		}
		o.Success = success != 0
		o.TopoPassed = topoPassed != 0
		o.TopoRejected = topoRejected != 0
		o.TrustLocked = trustLocked != 0
		o.Timestamp, _ = time.Parse(time.RFC3339, ts)
		result = append(result, o)
	}
	return result, nil
}
