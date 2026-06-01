package topology

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
)

// ── SQLite Repository ─────────────────────────────────────────

// SQLiteRepo implements Repository against SQLite.
type SQLiteRepo struct {
	db database.Executor
}

// NewSQLiteRepo creates a new SQLite-backed topology repository.
func NewSQLiteRepo(db database.Executor) *SQLiteRepo {
	return &SQLiteRepo{db: db}
}

// EnsureSchema creates the topology tables if they don't exist.
func (r *SQLiteRepo) EnsureSchema(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS agent_sessions (
			id VARCHAR(64) PRIMARY KEY,
			user_id VARCHAR(64) NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			start_coord TEXT NOT NULL DEFAULT '{"x":0,"y":0,"z":0}',
			current_coord TEXT NOT NULL DEFAULT '{"x":0,"y":0,"z":0}',
			constraint_json TEXT NOT NULL DEFAULT '{"a":0.8,"r":3.0,"t":false}',
			trust_lies INTEGER DEFAULT 0,
			trust_locked INTEGER DEFAULT 0,
			reject_count INTEGER DEFAULT 0,
			force_tools_triggered INTEGER DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
			last_active_at DATETIME,
			task_description TEXT DEFAULT '',
			closed_loop INTEGER DEFAULT 0,
			closed_distance REAL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS topology_nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id VARCHAR(64) NOT NULL,
			round INTEGER NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			z REAL NOT NULL,
			tool_call VARCHAR(255) DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'committed',
			reason TEXT DEFAULT '',
			timestamp DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(session_id, round)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_topology_nodes_session ON topology_nodes(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status)`,
	}

	for _, m := range migrations {
		if _, err := r.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("topology schema migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

// ── Session CRUD ──────────────────────────────────────────────

func (r *SQLiteRepo) SaveSession(ctx context.Context, s *SessionRecord) error {
	startJSON, _ := json.Marshal(s.StartCoord)
	currentJSON, _ := json.Marshal(s.CurrentCoord)
	constraintJSON := fmt.Sprintf(`{"a":0.8,"r":3.0,"t":false}`) // Default; real value from tracker

	trustLocked := 0
	if s.TrustLocked {
		trustLocked = 1
	}
	forceTriggered := 0
	if s.ForceToolsTriggered {
		forceTriggered = 1
	}
	closedLoop := 0
	if s.ClosedLoop {
		closedLoop = 1
	}

	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	s.LastActiveAt = now

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (id, user_id, status, start_coord, current_coord, constraint_json,
		 trust_lies, trust_locked, reject_count, force_tools_triggered,
		 created_at, updated_at, last_active_at, closed_loop, closed_distance)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		 status=excluded.status, current_coord=excluded.current_coord,
		 trust_lies=excluded.trust_lies, trust_locked=excluded.trust_locked,
		 reject_count=excluded.reject_count, force_tools_triggered=excluded.force_tools_triggered,
		 updated_at=excluded.updated_at, last_active_at=excluded.last_active_at,
		 closed_loop=excluded.closed_loop, closed_distance=excluded.closed_distance`,
		s.SessionID, "", s.Status, string(startJSON), string(currentJSON), constraintJSON,
		s.TrustLies, trustLocked, s.RejectCount, forceTriggered,
		s.CreatedAt, s.UpdatedAt, s.LastActiveAt,
		closedLoop, s.ClosedDist,
	)
	if err != nil {
		return fmt.Errorf("save agent_session: %w", err)
	}
	return nil
}

func (r *SQLiteRepo) LoadSession(ctx context.Context, sessionID string) (*SessionRecord, error) {
	var rec SessionRecord
	var startJSON, currentJSON string
	var trustLocked, forceTriggered, closedLoop int

	err := r.db.QueryRowContext(ctx,
		`SELECT id, status, COALESCE(start_coord,'{}'), COALESCE(current_coord,'{}'),
		 trust_lies, trust_locked, reject_count, force_tools_triggered,
		 created_at, updated_at, COALESCE(last_active_at, created_at),
		 closed_loop, closed_distance
		 FROM agent_sessions WHERE id = ?`, sessionID,
	).Scan(&rec.SessionID, &rec.Status, &startJSON, &currentJSON,
		&rec.TrustLies, &trustLocked, &rec.RejectCount, &forceTriggered,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.LastActiveAt,
		&closedLoop, &rec.ClosedDist)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load agent_session: %w", err)
	}

	json.Unmarshal([]byte(startJSON), &rec.StartCoord)
	json.Unmarshal([]byte(currentJSON), &rec.CurrentCoord)
	rec.TrustLocked = trustLocked == 1
	rec.ForceToolsTriggered = forceTriggered == 1
	rec.ClosedLoop = closedLoop == 1

	return &rec, nil
}

func (r *SQLiteRepo) LoadActiveSessions(ctx context.Context) ([]*SessionRecord, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, status, COALESCE(start_coord,'{}'), COALESCE(current_coord,'{}'),
		 trust_lies, trust_locked, reject_count, force_tools_triggered,
		 created_at, updated_at, COALESCE(last_active_at, created_at),
		 closed_loop, closed_distance
		 FROM agent_sessions WHERE status IN ('active','interrupted')
		 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list active agent_sessions: %w", err)
	}
	defer rows.Close()

	var records []*SessionRecord
	for rows.Next() {
		var rec SessionRecord
		var startJSON, currentJSON string
		var trustLocked, forceTriggered, closedLoop int

		if err := rows.Scan(&rec.SessionID, &rec.Status, &startJSON, &currentJSON,
			&rec.TrustLies, &trustLocked, &rec.RejectCount, &forceTriggered,
			&rec.CreatedAt, &rec.UpdatedAt, &rec.LastActiveAt,
			&closedLoop, &rec.ClosedDist); err != nil {
			return nil, fmt.Errorf("scan agent_session: %w", err)
		}

		json.Unmarshal([]byte(startJSON), &rec.StartCoord)
		json.Unmarshal([]byte(currentJSON), &rec.CurrentCoord)
		rec.TrustLocked = trustLocked == 1
		rec.ForceToolsTriggered = forceTriggered == 1
		rec.ClosedLoop = closedLoop == 1

		records = append(records, &rec)
	}
	return records, rows.Err()
}

// ── Node CRUD ─────────────────────────────────────────────────

func (r *SQLiteRepo) SaveNode(ctx context.Context, sessionID string, node *Node) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO topology_nodes (session_id, round, x, y, z, tool_call, status, reason, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, node.Round, node.Coord.X, node.Coord.Y, node.Coord.Z,
		node.ToolCall, string(node.Status), node.Reason, node.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("save topology_node: %w", err)
	}
	return nil
}

func (r *SQLiteRepo) LoadNodes(ctx context.Context, sessionID string, limit int) ([]Node, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT round, x, y, z, tool_call, status, COALESCE(reason,''), timestamp
		 FROM topology_nodes WHERE session_id = ? ORDER BY round ASC LIMIT ?`,
		sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("load topology_nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		var status string
		if err := rows.Scan(&node.Round, &node.Coord.X, &node.Coord.Y, &node.Coord.Z,
			&node.ToolCall, &status, &node.Reason, &node.Timestamp); err != nil {
			return nil, fmt.Errorf("scan topology_node: %w", err)
		}
		node.Status = NodeStatus(status)
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (r *SQLiteRepo) DeleteNodesFrom(ctx context.Context, sessionID string, fromRound int) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM topology_nodes WHERE session_id = ? AND round >= ?`,
		sessionID, fromRound)
	if err != nil {
		return 0, fmt.Errorf("delete topology_nodes: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

func (r *SQLiteRepo) DeleteSession(ctx context.Context, sessionID string) error {
	// Delete nodes first
	_, _ = r.db.ExecContext(ctx, `DELETE FROM topology_nodes WHERE session_id = ?`, sessionID)
	_, err := r.db.ExecContext(ctx, `DELETE FROM agent_sessions WHERE id = ?`, sessionID)
	return err
}
