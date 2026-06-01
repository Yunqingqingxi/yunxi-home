// Package topology implements the geometric constraint system for AI agent navigation.
//
// Every AI step is mapped to a 3D coordinate space:
//   X = progress (0-10)
//   Y = complexity delta (-1.0 to 1.0)
//   Z = deviation from origin (0 to R)
//
// Validators enforce geometric constraints and truthfulness of AI self-reported coordinates.
package topology

import (
	"time"
)

// ── Types ─────────────────────────────────────────────────────

// Coordinate is a 3D point in the topology space.
type Coordinate struct {
	X float64 `json:"x"` // Progress 0-10
	Y float64 `json:"y"` // Complexity delta -1.0 to 1.0
	Z float64 `json:"z"` // Deviation from origin 0 to R
}

// Constraint holds the topology constraint parameters.
type Constraint struct {
	A          float64  `json:"a"`                    // Amplitude cap 0.1-1.0, default 0.8
	R          float64  `json:"r"`                    // Radius cap 0.5-5.0, default 3.0
	T          bool     `json:"t"`                    // Closed-loop requirement, default false
	ForceTools []string `json:"force_tools,omitempty"` // Tools to force when progress threshold met
}

// DefaultConstraint returns the default constraint parameters.
func DefaultConstraint() Constraint {
	return Constraint{A: 0.8, R: 3.0, T: false}
}

// NodeStatus is the status of a topology node.
type NodeStatus string

const (
	NodeCommitted  NodeStatus = "committed"
	NodeRejected   NodeStatus = "rejected"
	NodePending    NodeStatus = "pending"
	NodeOverridden NodeStatus = "overridden"
)

// Node is a single point in the topology trajectory.
type Node struct {
	Coord     Coordinate `json:"coord"`
	Round     int        `json:"round"`
	Timestamp time.Time  `json:"timestamp"`
	ToolCall  string     `json:"tool_call"`
	Status    NodeStatus `json:"status"`
	Reason    string     `json:"reason,omitempty"`
}

// ── Parse Result ──────────────────────────────────────────────

// ParseResult holds the parsed topology tag data from AI output.
type ParseResult struct {
	Coord       Coordinate `json:"coord"`
	Tools       []string   `json:"tools"`        // Declared tools this round
	AckUpdate   bool       `json:"ack_update"`   // AI acknowledges constraint update
	Parsed      bool       `json:"parsed"`       // Whether tag was found and parsed
}

// ── Trust State ───────────────────────────────────────────────

// TrustState tracks AI truthfulness.
type TrustState struct {
	Lies   int  `json:"lies"`   // Consecutive detected lies
	Locked bool `json:"locked"` // Lock mode: ignore AI self-report, use risk-profile midpoints
}

// ── Topology State (for API) ──────────────────────────────────

// SessionState is the full topology state exposed to the frontend.
type SessionState struct {
	SessionID    string      `json:"session_id"`
	CurrentCoord Coordinate  `json:"current_coord"`
	StartCoord   Coordinate  `json:"start_coord"`
	Constraint   Constraint  `json:"constraint"`
	Trajectory   []Node      `json:"trajectory"`
	Trust        TrustState  `json:"trust"`
	RejectCount  int         `json:"reject_count"`
	ClosedLoop   bool        `json:"closed_loop"`
	ClosedDist   float64     `json:"closed_distance,omitempty"`
	Warning      string      `json:"warning,omitempty"`
	Active       bool        `json:"active"`
}

// ── Risk Profile ──────────────────────────────────────────────

// RiskProfile maps a tool pattern to expected coordinate changes.
type RiskProfile struct {
	Pattern  string  `json:"pattern"` // Tool name pattern (supports * wildcard)
	DeltaYMin float64 `json:"delta_y_min"`
	DeltaYMax float64 `json:"delta_y_max"`
	DeltaZMin float64 `json:"delta_z_min"`
	DeltaZMax float64 `json:"delta_z_max"`
}

// ── Validation Result ─────────────────────────────────────────

// ValidationResult holds the outcome of topology validation.
type ValidationResult struct {
	Passed   bool   `json:"passed"`
	Reason   string `json:"reason,omitempty"`
	Rejected bool   `json:"rejected"`
	Warning  bool   `json:"warning"`
}

// ── Edit Result ───────────────────────────────────────────────

// EditResult holds the outcome of a message edit/delete operation.
type EditResult struct {
	DeletedNodes int            `json:"deleted_nodes"`
	NewMessages  []map[string]any `json:"new_messages"`
	Message      string         `json:"message"`
}

// ── Oscillation Detection ─────────────────────────────────────

// OscillationState tracks pattern oscillation in the trajectory.
type OscillationState struct {
	Pattern   string `json:"pattern"`   // Detected oscillation pattern description
	Detected  bool   `json:"detected"`  // Whether oscillation was detected
	Round     int    `json:"round"`     // Round where detected
}

// ── Constants ─────────────────────────────────────────────────

const (
	// Tolerance for truthfulness checks
	TruthfulnessTolerance = 0.2

	// Trust thresholds
	MaxLiesBeforeLock = 3

	// Rejection thresholds
	MaxConsecutiveRejects = 5

	// Silent round thresholds
	MaxSilentRounds = 2

	// Closed loop epsilon
	ClosedLoopEpsilon = 0.5

	// Checkpoint settings
	CheckpointNodeCount = 10
	CheckpointInterval  = 5 * time.Second

	// Recovery settings
	RecoveryLoadNodes = 50

	// Progress threshold for ForceTools
	ForceToolsProgressThreshold = 5.0

	// Recent history window for ForceTools check
	ForceToolsHistoryWindow = 10

	// Change throttling thresholds (v3.1 cache optimization)
	// Only update message[1] when coordinate deltas exceed these thresholds.
	CoordChangeThresholdX = 0.1
	CoordChangeThresholdY = 0.05
	CoordChangeThresholdZ = 0.05
)

// ShouldUpdateCoord returns true if the coordinate change is significant enough
// to warrant updating the topology state message (message[1]).
// Prevents unnecessary KV cache invalidations from micro-adjustments.
func ShouldUpdateCoord(prev, current Coordinate) bool {
	return abs(current.X-prev.X) >= CoordChangeThresholdX ||
		abs(current.Y-prev.Y) >= CoordChangeThresholdY ||
		abs(current.Z-prev.Z) >= CoordChangeThresholdZ
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
