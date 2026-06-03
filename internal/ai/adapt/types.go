// Package adapt provides a cross-subsystem User Adaptation Layer that learns
// from every interaction and feeds improvements back into memory, prompt, agent,
// topology, and observability subsystems.
package adapt

import "time"

// VerbosityLevel describes how detailed the AI'"'"'s responses should be.
type VerbosityLevel string

const (
	VerbosityDefault   VerbosityLevel = ""
	VerbosityConcise   VerbosityLevel = "concise"
	VerbosityDetailed  VerbosityLevel = "detailed"
	VerbosityTechnical VerbosityLevel = "technical"
)

// UserProfile accumulates per-user preferences and behavior patterns over time.
type UserProfile struct {
	UserID          string             `json:"user_id"`
	Verbosity       VerbosityLevel     `json:"verbosity"`
	TopDomains      []string           `json:"top_domains"`
	TopTools        []string           `json:"top_tools"`
	ToolSuccess     map[string]float64 `json:"tool_success"`
	TaskPatterns    []TaskPattern      `json:"task_patterns"`
	CorrectionCount int                `json:"correction_count"`
	CancelCount     int                `json:"cancel_count"`
	SessionCount    int                `json:"session_count"`
	LastActiveAt    time.Time          `json:"last_active_at"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// TaskPattern describes a recurring task the user asks for.
type TaskPattern struct {
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	Tools       []string `json:"tools"`
	SuccessRate float64  `json:"success_rate"`
	Count       int      `json:"count"`
}

// FeedbackType classifies implicit user feedback.
type FeedbackType string

const (
	FeedbackCorrection FeedbackType = "correction"
	FeedbackCancelled  FeedbackType = "cancelled"
	FeedbackEdited     FeedbackType = "edited"
	FeedbackReask      FeedbackType = "reask"
	FeedbackSuccess    FeedbackType = "success"
	FeedbackFailed     FeedbackType = "failed"
)

// FeedbackEvent records a single implicit feedback signal.
type FeedbackEvent struct {
	ID           string       `json:"id"`
	UserID       string       `json:"user_id"`
	SessionID    string       `json:"session_id"`
	Type         FeedbackType `json:"type"`
	ToolName     string       `json:"tool_name,omitempty"`
	TaskCategory string       `json:"task_category,omitempty"`
	OriginalMsg  string       `json:"original_msg,omitempty"`
	EditedMsg    string       `json:"edited_msg,omitempty"`
	Detail       string       `json:"detail,omitempty"`
	Timestamp    time.Time    `json:"timestamp"`
}

// ToolOutcome records a single tool execution result for learning.
type ToolOutcome struct {
	SessionID    string    `json:"session_id"`
	Round        int       `json:"round"`
	ToolName     string    `json:"tool_name"`
	Success      bool      `json:"success"`
	DurationMs   int64     `json:"duration_ms"`
	TopoPassed   bool      `json:"topo_passed"`
	TopoRejected bool      `json:"topo_rejected"`
	TrustLocked  bool      `json:"trust_locked"`
	Timestamp    time.Time `json:"timestamp"`
}

// SessionSummary captures key metrics from a completed session.
type SessionSummary struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	TaskCategory  string    `json:"task_category"`
	Rounds        int       `json:"rounds"`
	ToolCalls     int       `json:"tool_calls"`
	ToolSuccesses int       `json:"tool_successes"`
	ToolFailures  int       `json:"tool_failures"`
	TopoRejects   int       `json:"topo_rejects"`
	TokensIn      int64     `json:"tokens_in"`
	TokensOut     int64     `json:"tokens_out"`
	TrustLocked   bool      `json:"trust_locked"`
	Completed     bool      `json:"completed"`
	DurationSec   float64   `json:"duration_sec"`
	Timestamp     time.Time `json:"timestamp"`
}

// PromptEffectiveness tracks how well a prompt variant performs.
type PromptEffectiveness struct {
	PromptID     string  `json:"prompt_id"`
	Variant      string  `json:"variant"`
	UseCount     int     `json:"use_count"`
	SuccessCount int     `json:"success_count"`
	EditCount    int     `json:"edit_count"`
	CancelCount  int     `json:"cancel_count"`
	AvgRounds    float64 `json:"avg_rounds"`
}
