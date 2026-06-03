package adapt

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var layerLog = logger.ForComponent("adapt")

// Layer is the top-level orchestrator for the User Adaptation Layer.
// It ties together profile management, feedback detection, prompt selection,
// and cross-session learning.
type Layer struct {
	repo       Repository
	ProfileMgr *ProfileManager
	Feedback   *FeedbackDetector
	Selector   *PromptSelector
	Learner    *Learner
}

// NewLayer creates a new adaptation layer backed by the given DB.
func NewLayer(db *sql.DB) *Layer {
	repo := NewSQLiteRepo(db)
	profileMgr := NewProfileManager(repo)
	return &Layer{
		repo:       repo,
		ProfileMgr: profileMgr,
		Feedback:   NewFeedbackDetector(repo, profileMgr),
		Selector:   NewPromptSelector(repo, profileMgr),
		Learner:    NewLearner(repo, profileMgr),
	}
}

// EnsureSchema creates the adaptation database tables if they don't exist.
func (l *Layer) EnsureSchema(ctx context.Context) error {
	return l.repo.EnsureSchema(ctx)
}

// ── Session Lifecycle Hooks ────────────────────────────────────────────────────

// OnSessionStart is called at the beginning of a chat session.
// It returns a profile summary string to inject into the system prompt,
// and may auto-create memories from previous session feedback.
func (l *Layer) OnSessionStart(ctx context.Context, userID, userMessage string) (profileSummary string) {
	if userID == "" {
		return ""
	}

	// Ensure profile exists
	p, err := l.ProfileMgr.GetOrCreate(ctx, userID)
	if err != nil {
		layerLog.Warn("failed to load profile", "user_id", userID, "error", err)
		return ""
	}

	// Select the best prompt variant for this user
	variant := l.Selector.SelectVariant(ctx, userID, "general")
	if variant != "" && variant != "default" {
		layerLog.Debug("prompt variant selected", "user_id", userID, "variant", variant)
	}

	_ = p // used below through Summarize

	// Return the profile summary for system prompt injection
	return l.ProfileMgr.Summarize(userID)
}

// OnSessionEnd is called when a chat session completes (or is interrupted).
// It updates the profile, saves a session summary, records prompt outcomes,
// and auto-creates memories from implicit feedback.
func (l *Layer) OnSessionEnd(ctx context.Context, summary *SessionSummary) {
	if summary == nil || summary.UserID == "" {
		return
	}

	// Persist session summary
	if err := l.repo.SaveSessionSummary(ctx, summary); err != nil {
		layerLog.Warn("failed to save session summary", "session", summary.SessionID, "error", err)
	}

	// Update profile with session outcomes
	if err := l.ProfileMgr.RecordSessionEnd(ctx, summary); err != nil {
		layerLog.Warn("failed to update profile", "user_id", summary.UserID, "error", err)
	}

	// Record prompt outcome
	success := summary.Completed
	edited := false // determined by caller if necessary
	cancelled := summary.TrustLocked
	l.Selector.RecordPromptOutcome(ctx, "general", "default", success, edited, cancelled, summary.Rounds)

	// Check for anomalies
	report := l.Learner.AnomalyCheck(ctx, summary.UserID, summary)
	if report != nil && report.HasAnomaly {
		layerLog.Warn("anomaly detected", "user_id", summary.UserID,
			"details", fmt.Sprintf("%v", report.Details))
	}

	layerLog.Info("session recorded for adaptation",
		"user_id", summary.UserID,
		"session", summary.SessionID,
		"category", summary.TaskCategory,
		"rounds", summary.Rounds,
		"tools", summary.ToolCalls,
		"successes", summary.ToolSuccesses,
		"failures", summary.ToolFailures,
	)
}

// OnToolResult records a tool execution outcome for learning.
func (l *Layer) OnToolResult(ctx context.Context, sessionID, userID, toolName string, success bool, durationMs int64, round int) {
	if userID == "" {
		return
	}
	outcome := &ToolOutcome{
		SessionID:  sessionID,
		Round:      round,
		ToolName:   toolName,
		Success:    success,
		DurationMs: durationMs,
		Timestamp:  time.Now().UTC(),
	}
	if err := l.repo.RecordToolOutcome(ctx, outcome); err != nil {
		layerLog.Debug("failed to record tool outcome", "error", err)
	}
}

// OnTopologyValidation records a topology validation result for a tool.
func (l *Layer) OnTopologyValidation(ctx context.Context, sessionID, userID, toolName string, passed, trustLocked bool, round int) {
	if userID == "" {
		return
	}
	outcome := &ToolOutcome{
		SessionID:    sessionID,
		Round:        round,
		ToolName:     toolName,
		Success:      passed, // topology passed = success from topology perspective
		DurationMs:   0,
		TopoPassed:   passed,
		TopoRejected: !passed,
		TrustLocked:  trustLocked,
		Timestamp:    time.Now().UTC(),
	}
	if err := l.repo.RecordToolOutcome(ctx, outcome); err != nil {
		layerLog.Debug("failed to record topology outcome", "error", err)
	}
}

// OnUserMessage processes a user message to detect re-asks and corrections.
// Returns any feedback event detected, which callers can use to adjust behavior.
func (l *Layer) OnUserMessage(sessionID, userID, message string) *FeedbackEvent {
	if userID == "" {
		return nil
	}

	// Store for re-ask detection
	l.Feedback.RecordUserMessage(sessionID, userID, message)

	// Check for re-ask
	if ev := l.Feedback.DetectReask(sessionID, userID, message); ev != nil {
		_ = l.repo.RecordFeedback(context.Background(), ev)
		layerLog.Debug("re-ask detected", "session", sessionID, "user", userID)
		return ev
	}

	// Check for correction
	if ev := l.Feedback.DetectCorrection(message); ev != nil {
		ev.UserID = userID
		ev.SessionID = sessionID
		_ = l.repo.RecordFeedback(context.Background(), ev)
		_ = l.ProfileMgr.RecordFeedback(context.Background(), ev)
		layerLog.Debug("correction detected", "session", sessionID, "user", userID)
		return ev
	}

	return nil
}

// OnCancellation records a user cancellation as implicit feedback.
func (l *Layer) OnCancellation(sessionID, userID, lastTool, taskCategory string, round int) {
	if userID == "" {
		return
	}
	if ev := l.Feedback.RecordCancellation(sessionID, userID, lastTool, taskCategory, round); ev != nil {
		_ = l.repo.RecordFeedback(context.Background(), ev)
		_ = l.ProfileMgr.RecordFeedback(context.Background(), ev)
	}
}

// GetAdaptiveTolerance returns the recommended topology tolerance multiplier
// for a tool based on this user's historical false-positive rate.
func (l *Layer) GetAdaptiveTolerance(ctx context.Context, userID, toolName string) float64 {
	if userID == "" {
		return 1.0
	}
	return l.Learner.RecommendedTolerance(ctx, userID, toolName)
}

// GetTrustCalibration returns the recommended trust threshold (maxLiesBeforeLock)
// and whether it should be raised for this user.
func (l *Layer) GetTrustCalibration(ctx context.Context, userID string) (maxLies int, raiseThreshold bool) {
	return l.Learner.TrustCalibration(ctx, userID)
}

// GetCommonToolSequence returns frequently-successful tool sequences for a task category.
func (l *Layer) GetCommonToolSequence(ctx context.Context, userID, taskCategory string) []ToolPair {
	return l.Learner.CommonToolSequence(ctx, userID, taskCategory)
}

// GetWeeklySummary returns a human-readable improvement summary string.
func (l *Layer) GetWeeklySummary(ctx context.Context, userID string) string {
	return l.Learner.WeeklySummary(ctx, userID)
}

// InferTaskCategory guesses the task category from a user message.
func (l *Layer) InferTaskCategory(userID, message string) string {
	return l.ProfileMgr.InferTaskCategory(userID, message)
}

// RecordFeedback persists a user feedback event and updates the profile.
func (l *Layer) RecordFeedback(ctx context.Context, ev *FeedbackEvent) {
	_ = l.repo.RecordFeedback(ctx, ev)
	_ = l.ProfileMgr.RecordFeedback(ctx, ev)
}
