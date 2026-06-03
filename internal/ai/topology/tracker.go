package topology

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

var log = logger.ForComponent("topology")

// ── TopologyTracker ───────────────────────────────────────────
//
// Central state machine that tracks AI trajectory, validates coordinates,
// manages trust, detects oscillations, and handles closed-loop enforcement.

// Tracker manages topology state for all active sessions.
type Tracker struct {
	repo   Repository
	states map[string]*sessionTracker
	mu     sync.RWMutex
}

// sessionTracker holds the topology state for a single session.
type sessionTracker struct {
	SessionID          string
	StartCoord         Coordinate
	CurrentCoord       Coordinate
	Constraint         Constraint
	Nodes              []Node
	Trust              TrustState
	RejectCount        int
	ForceToolsTriggered bool
	ClosedLoop         bool
	ClosedDist         float64
	Warning            string
	Active             bool
	OverrideNext       bool        // Skip next validation
	OverrideTarget     *Coordinate // Optional target coordinate for override
	lastCheckpoint     time.Time
	checkpointCount    int // Nodes added since last checkpoint
}

// Repository is the persistence interface for topology data.
type Repository interface {
	SaveSession(ctx context.Context, s *SessionRecord) error
	LoadSession(ctx context.Context, sessionID string) (*SessionRecord, error)
	LoadActiveSessions(ctx context.Context) ([]*SessionRecord, error)
	SaveNode(ctx context.Context, sessionID string, node *Node) error
	LoadNodes(ctx context.Context, sessionID string, limit int) ([]Node, error)
	DeleteNodesFrom(ctx context.Context, sessionID string, fromRound int) (int, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// SessionRecord is the database representation of a topology session.
type SessionRecord struct {
	SessionID          string     `json:"session_id"`
	Status             string     `json:"status"`
	StartCoord         Coordinate `json:"start_coord"`
	CurrentCoord       Coordinate `json:"current_coord"`
	ConstraintJSON     string     `json:"constraint_json"`
	TrustLies          int        `json:"trust_lies"`
	TrustLocked        bool       `json:"trust_locked"`
	RejectCount        int        `json:"reject_count"`
	ForceToolsTriggered bool      `json:"force_tools_triggered"`
	ClosedLoop         bool       `json:"closed_loop"`
	ClosedDist         float64    `json:"closed_distance"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastActiveAt       time.Time  `json:"last_active_at"`
}

// NewTracker creates a new topology tracker.
func NewTracker(repo Repository) *Tracker {
	return &Tracker{
		repo:   repo,
		states: make(map[string]*sessionTracker),
	}
}

// ── Session Lifecycle ─────────────────────────────────────────

// InitSession initializes topology tracking for a new or existing session.
func (t *Tracker) InitSession(sessionID string, constraint Constraint) *sessionTracker {
	t.mu.Lock()
	defer t.mu.Unlock()

	if st, ok := t.states[sessionID]; ok {
		return st
	}

	st := &sessionTracker{
		SessionID:    sessionID,
		StartCoord:   Coordinate{},
		CurrentCoord: Coordinate{},
		Constraint:   constraint,
		Trust:        TrustState{},
		Active:       true,
		lastCheckpoint: time.Now(),
	}
	t.states[sessionID] = st
	return st
}

// GetSession returns the topology state for a session, or nil if not tracked.
func (t *Tracker) GetSession(sessionID string) *sessionTracker {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.states[sessionID]
}

// RemoveSession cleans up topology state for a session.
func (t *Tracker) RemoveSession(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, sessionID)
}

// GetState returns the full SessionState for API consumption.
func (t *Tracker) GetState(sessionID string) *SessionState {
	st := t.GetSession(sessionID)
	if st == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	trajectory := make([]Node, len(st.Nodes))
	copy(trajectory, st.Nodes)

	return &SessionState{
		SessionID:    st.SessionID,
		CurrentCoord: st.CurrentCoord,
		StartCoord:   st.StartCoord,
		Constraint:   st.Constraint,
		Trajectory:   trajectory,
		Trust:        st.Trust,
		RejectCount:  st.RejectCount,
		ClosedLoop:   st.ClosedLoop,
		ClosedDist:   st.ClosedDist,
		Warning:      st.Warning,
		Active:       st.Active,
	}
}

// ── Validation ────────────────────────────────────────────────

// ValidateStep runs all validators on a proposed coordinate.
// Returns true if the step passes, along with the validation result.
func (t *Tracker) ValidateStep(sessionID string, proposed ParseResult, actualTools []string) (bool, string) {
	st := t.GetSession(sessionID)
	if st == nil {
		return true, "" // Not tracked, allow all
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// If override is set, skip validation this round
	if st.OverrideNext {
		st.OverrideNext = false
		if st.OverrideTarget != nil {
			proposed.Coord = *st.OverrideTarget
			st.OverrideTarget = nil
		}
		// Record as overridden
		node := Node{
			Coord:     proposed.Coord,
			Round:     len(st.Nodes),
			Timestamp: time.Now(),
			ToolCall:  fmt.Sprintf("%v", actualTools),
			Status:    NodeOverridden,
			Reason:    "user override",
		}
		st.Nodes = append(st.Nodes, node)
		st.CurrentCoord = proposed.Coord
		st.RejectCount = 0
		st.Warning = ""
		t.checkpointMaybe(sessionID, st, &node)
		return true, ""
	}

	// If trust is locked, use risk-profile midpoint instead of AI self-report
	if st.Trust.Locked && len(actualTools) > 0 {
		profile := MatchRiskProfile(actualTools[0])
		midY := (profile.DeltaYMin + profile.DeltaYMax) / 2
		midZ := (profile.DeltaZMin + profile.DeltaZMax) / 2
		proposed.Coord = Coordinate{
			X: proposed.Coord.X,
			Y: st.CurrentCoord.Y + midY,
			Z: st.CurrentCoord.Z + midZ,
		}
	}

	// 1. Check tool declaration
	declResult := CheckToolDeclared(proposed.Tools, actualTools)
	if !declResult.Passed {
		if len(declResult.DeclaredNotUsed) > 0 && len(actualTools) == 0 {
			// AI declared tools but called none — inject reminder, continue
			return false, declResult.Message
		}
		if len(declResult.UsedNotDeclared) > 0 {
			// Warning: undeclared tools used
			st.Trust.Lies++
			st.Warning = declResult.Message
			if st.Trust.Lies >= MaxLiesBeforeLock {
				st.Trust.Locked = true
				st.Warning += " | 信任已锁定"
			}
		}
	}

	// 2. Check geometry
	geoResult := CheckGeometry(st.CurrentCoord, proposed.Coord, st.Constraint)
	if !geoResult.Passed {
		st.RejectCount++
		node := Node{
			Coord:     proposed.Coord,
			Round:     len(st.Nodes),
			Timestamp: time.Now(),
			ToolCall:  fmt.Sprintf("%v", actualTools),
			Status:    NodeRejected,
			Reason:    geoResult.Reason,
		}
		st.Nodes = append(st.Nodes, node)
		t.checkpointMaybe(sessionID, st, &node)

		if st.RejectCount >= MaxConsecutiveRejects {
			st.Warning = fmt.Sprintf("连续拒绝 %d 次，需要人工干预", st.RejectCount)
		}
		return false, geoResult.Reason
	}

	// 3. Check truthfulness (only if tools were actually called)
	if len(actualTools) > 0 {
		for _, tool := range actualTools {
			truthResult := CheckTruthfulness(tool, st.CurrentCoord, proposed.Coord)
			if !truthResult.Passed {
				st.Trust.Lies++
				st.RejectCount++
				node := Node{
					Coord:     proposed.Coord,
					Round:     len(st.Nodes),
					Timestamp: time.Now(),
					ToolCall:  tool,
					Status:    NodeRejected,
					Reason:    truthResult.Reason,
				}
				st.Nodes = append(st.Nodes, node)
				t.checkpointMaybe(sessionID, st, &node)

				if st.Trust.Lies >= MaxLiesBeforeLock {
					st.Trust.Locked = true
					log.Warn("信任已锁定", "session", sessionID, "lies", st.Trust.Lies)
				}
				return false, truthResult.Reason
			}
		}
	}

	// All validations passed — commit node
	st.RejectCount = 0
	st.Warning = ""
	node := Node{
		Coord:     proposed.Coord,
		Round:     len(st.Nodes),
		Timestamp: time.Now(),
		ToolCall:  fmt.Sprintf("%v", actualTools),
		Status:    NodeCommitted,
	}
	st.Nodes = append(st.Nodes, node)
	st.CurrentCoord = proposed.Coord
	t.checkpointMaybe(sessionID, st, &node)

	return true, ""
}

// ── Closed Loop ───────────────────────────────────────────────

// CheckClosedLoop validates closed-loop requirement when progress X ≥ 10.
// Returns (closed, distance, message).
func (t *Tracker) CheckClosedLoop(sessionID string) (bool, float64, string) {
	st := t.GetSession(sessionID)
	if st == nil || !st.Constraint.T {
		return true, 0, ""
	}

	closed, dist := CheckClosedLoop(st.StartCoord, st.CurrentCoord, st.Constraint)
	if closed {
		t.mu.Lock()
		st.ClosedLoop = true
		st.ClosedDist = dist
		t.mu.Unlock()
		return true, dist, ""
	}

	return false, dist, fmt.Sprintf(
		"任务未闭合(差距=%.2f)。[强制结束] [要求回环]",
		dist,
	)
}

// ── Update Constraint ─────────────────────────────────────────

// UpdateConstraint updates the constraint parameters for a session.
func (t *Tracker) UpdateConstraint(sessionID string, a, r float64, tFlag bool, forceTools []string) error {
	st := t.GetSession(sessionID)
	if st == nil {
		return fmt.Errorf("session not tracked: %s", sessionID)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if a > 0 {
		st.Constraint.A = clampFloat(a, 0.1, 1.0)
	}
	if r > 0 {
		st.Constraint.R = clampFloat(r, 0.5, 5.0)
	}
	st.Constraint.T = tFlag
	if forceTools != nil {
		st.Constraint.ForceTools = forceTools
	}

	log.Info("约束参数已更新", "session", sessionID, "a", st.Constraint.A, "r", st.Constraint.R, "t", st.Constraint.T)
	return nil
}

// ── Trust Unlock ──────────────────────────────────────────────

// ResetTrust resets the trust state (lies counter and lock).
func (t *Tracker) ResetTrust(sessionID string) {
	st := t.GetSession(sessionID)
	if st == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st.Trust.Lies = 0
	st.Trust.Locked = false
	st.Warning = ""
	log.Info("信任状态已重置", "session", sessionID)
}

// ── Override ──────────────────────────────────────────────────

// OverrideNextNode marks the next validation to be skipped (one-shot).
// If targetCoord is provided, forces that coordinate for the next node.
func (t *Tracker) OverrideNextNode(sessionID string, targetCoord *Coordinate) {
	st := t.GetSession(sessionID)
	if st == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	st.OverrideNext = true
	if targetCoord != nil {
		st.OverrideTarget = targetCoord
	}
	st.RejectCount = 0
	st.Warning = ""
	log.Info("节点已放行", "session", sessionID)
}

// ── Rebase ────────────────────────────────────────────────────

// Rebase truncates the trajectory at the given round and resets derived state.
// Returns the number of deleted nodes.
func (t *Tracker) Rebase(sessionID string, atRound int) (int, error) {
	st := t.GetSession(sessionID)
	if st == nil {
		return 0, fmt.Errorf("session not tracked: %s", sessionID)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if atRound < 0 || atRound > len(st.Nodes) {
		return 0, fmt.Errorf("invalid rebase round %d (have %d nodes)", atRound, len(st.Nodes))
	}

	deleted := len(st.Nodes) - atRound
	if deleted <= 0 {
		return 0, nil
	}

	st.Nodes = st.Nodes[:atRound]
	if atRound > 0 {
		st.CurrentCoord = st.Nodes[atRound-1].Coord
	} else {
		st.CurrentCoord = st.StartCoord
	}
	st.RejectCount = 0
	st.ForceToolsTriggered = false
	// Trust state is NOT reset

	// Sync to DB
	if t.repo != nil {
		n, err := t.repo.DeleteNodesFrom(context.Background(), sessionID, atRound)
		if err != nil {
			log.Warn("Rebase DB sync failed", "session", sessionID, "error", err)
		}
		deleted = n
	}

	log.Info("轨迹已rebase", "session", sessionID, "at_round", atRound, "deleted", deleted)
	return deleted, nil
}

// ── ForceTools ────────────────────────────────────────────────

// ShouldForceTools checks if force tools should be triggered.
// Returns the tool name to force, or empty string.
func (t *Tracker) ShouldForceTools(sessionID string, recentHistory []string) string {
	st := t.GetSession(sessionID)
	if st == nil || len(st.Constraint.ForceTools) == 0 {
		return ""
	}

	if st.CurrentCoord.X < ForceToolsProgressThreshold {
		return ""
	}

	// Check if any force tool hasn't been used in recent history
	recentSet := make(map[string]bool, len(recentHistory))
	for _, tool := range recentHistory {
		recentSet[tool] = true
	}

	for _, ft := range st.Constraint.ForceTools {
		if !recentSet[ft] {
			t.mu.Lock()
			st.ForceToolsTriggered = true
			t.mu.Unlock()
			return ft
		}
	}

	return ""
}

// ── Oscillation Detection ─────────────────────────────────────

// DetectOscillation checks the trajectory for back-and-forth patterns.
func (t *Tracker) DetectOscillation(sessionID string) *OscillationState {
	st := t.GetSession(sessionID)
	if st == nil || len(st.Nodes) < 4 {
		return &OscillationState{}
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Check last 4 nodes for oscillation pattern (A→B→A→B)
	n := len(st.Nodes)
	if n < 4 {
		return &OscillationState{}
	}

	last4 := st.Nodes[n-4:]
	// Same tool alternating = oscillation
	if last4[0].ToolCall == last4[2].ToolCall &&
		last4[1].ToolCall == last4[3].ToolCall &&
		last4[0].ToolCall != last4[1].ToolCall {
		return &OscillationState{
			Pattern:  fmt.Sprintf("工具震荡: %s ↔ %s", last4[0].ToolCall, last4[1].ToolCall),
			Detected: true,
			Round:    n,
		}
	}

	return &OscillationState{}
}

// ── Checkpoint ────────────────────────────────────────────────

func (t *Tracker) checkpointMaybe(sessionID string, st *sessionTracker, node *Node) {
	st.checkpointCount++

	// Write to DB if threshold met
	if t.repo != nil && (st.checkpointCount >= CheckpointNodeCount ||
		time.Since(st.lastCheckpoint) >= CheckpointInterval) {
		go func() {
			if err := t.repo.SaveNode(context.Background(), sessionID, node); err != nil {
				log.Warn("保存拓扑节点失败", "session", sessionID, "error", err)
			}
			if err := t.repo.SaveSession(context.Background(), st.toRecord()); err != nil {
				log.Warn("保存拓扑会话失败", "session", sessionID, "error", err)
			}
		}()
		st.checkpointCount = 0
		st.lastCheckpoint = time.Now()
	}
}

// ── Recovery ──────────────────────────────────────────────────

// RecoverActiveSessions loads all active sessions from DB after a restart.
func (t *Tracker) RecoverActiveSessions(ctx context.Context) error {
	if t.repo == nil {
		return nil
	}

	records, err := t.repo.LoadActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("load active sessions: %w", err)
	}

	for _, rec := range records {
		nodes, err := t.repo.LoadNodes(ctx, rec.SessionID, RecoveryLoadNodes)
		if err != nil {
			log.Warn("加载拓扑节点失败", "session", rec.SessionID, "error", err)
		}

		st := &sessionTracker{
			SessionID:           rec.SessionID,
			StartCoord:          rec.StartCoord,
			CurrentCoord:        rec.CurrentCoord,
			Constraint:          DefaultConstraint(),
			Nodes:               nodes,
			Trust:               TrustState{Lies: rec.TrustLies, Locked: rec.TrustLocked},
			RejectCount:         rec.RejectCount,
			ForceToolsTriggered: rec.ForceToolsTriggered,
			ClosedLoop:          rec.ClosedLoop,
			ClosedDist:          rec.ClosedDist,
			Active:              true,
			lastCheckpoint:      time.Now(),
		}

		t.mu.Lock()
		t.states[rec.SessionID] = st
		t.mu.Unlock()

		log.Info("恢复拓扑会话", "session", rec.SessionID, "nodes", len(nodes),
			"coord", rec.CurrentCoord, "lies", rec.TrustLies)
	}

	log.Info("拓扑会话恢复完成", "count", len(records))
	return nil
}

// ── Helpers ───────────────────────────────────────────────────

func (st *sessionTracker) toRecord() *SessionRecord {
	return &SessionRecord{
		SessionID:           st.SessionID,
		Status:              "active",
		StartCoord:          st.StartCoord,
		CurrentCoord:        st.CurrentCoord,
		TrustLies:           st.Trust.Lies,
		TrustLocked:         st.Trust.Locked,
		RejectCount:         st.RejectCount,
		ForceToolsTriggered: st.ForceToolsTriggered,
		ClosedLoop:          st.ClosedLoop,
		ClosedDist:          st.ClosedDist,
	}
}

// ── Shutdown ──────────────────────────────────────────────────

// Shutdown flushes all in-memory state to DB.
func (t *Tracker) Shutdown(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for sessionID, st := range t.states {
		if !st.Active {
			continue
		}
		if t.repo != nil {
			if err := t.repo.SaveSession(ctx, st.toRecord()); err != nil {
				log.Warn("shutdown save session failed", "session", sessionID, "error", err)
			}
		}
	}
	log.Info("拓扑追踪器已关闭", "sessions", len(t.states))
}
