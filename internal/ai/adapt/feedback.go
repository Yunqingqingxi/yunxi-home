package adapt

import (
	"fmt"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// FeedbackDetector analyzes conversation patterns to detect implicit feedback.
type FeedbackDetector struct {
	repo            Repository
	profileMgr      *ProfileManager
	recentMessages  map[string][]userMessageRecord // sessionID -> recent messages
	cancelledStates map[string]*cancelState         // sessionID -> cancelled round state
}

type userMessageRecord struct {
	content   string
	timestamp time.Time
}

type cancelState struct {
	lastTool     string
	taskCategory string
	round        int
}

func NewFeedbackDetector(repo Repository, profileMgr *ProfileManager) *FeedbackDetector {
	return &FeedbackDetector{
		repo:            repo,
		profileMgr:      profileMgr,
		recentMessages:  make(map[string][]userMessageRecord),
		cancelledStates: make(map[string]*cancelState),
	}
}

// RecordUserMessage stores a user message for re-ask detection.
func (fd *FeedbackDetector) RecordUserMessage(sessionID, userID, message string) {
	fd.recentMessages[sessionID] = append(fd.recentMessages[sessionID], userMessageRecord{
		content:   message,
		timestamp: time.Now(),
	})
	// Keep only last 5 messages per session
	if len(fd.recentMessages[sessionID]) > 5 {
		fd.recentMessages[sessionID] = fd.recentMessages[sessionID][1:]
	}
}

// DetectReask checks if the latest user message is a re-ask of a previous one.
func (fd *FeedbackDetector) DetectReask(sessionID, userID, message string) *FeedbackEvent {
	msgs := fd.recentMessages[sessionID]
	if len(msgs) < 2 {
		return nil
	}
	prev := msgs[len(msgs)-2]
	if time.Since(prev.timestamp) > 5*time.Minute {
		return nil
	}
	similarity := jaccardSimilarity(prev.content, message)
	if similarity < 0.4 {
		return nil
	}
	return &FeedbackEvent{
		ID:        fmt.Sprintf("reask_%s_%d", sessionID, time.Now().UnixNano()),
		UserID:    userID,
		SessionID: sessionID,
		Type:      FeedbackReask,
		Detail:    fmt.Sprintf("similarity=%.2f", similarity),
		Timestamp: time.Now().UTC(),
	}
}

// DetectCancellation records the state when a user cancels.
func (fd *FeedbackDetector) RecordCancellation(sessionID, userID, lastTool, taskCategory string, round int) *FeedbackEvent {
	fd.cancelledStates[sessionID] = &cancelState{
		lastTool:     lastTool,
		taskCategory: taskCategory,
		round:        round,
	}
	return &FeedbackEvent{
		ID:           fmt.Sprintf("cancel_%s_%d", sessionID, time.Now().UnixNano()),
		UserID:       userID,
		SessionID:    sessionID,
		Type:         FeedbackCancelled,
		ToolName:     lastTool,
		TaskCategory: taskCategory,
		Detail:       fmt.Sprintf("round=%d", round),
		Timestamp:    time.Now().UTC(),
	}
}

// DetectCorrection checks if a user message is a correction of AI output.
func (fd *FeedbackDetector) DetectCorrection(userMessage string) *FeedbackEvent {
	correctionPhrases := []string{
		"no, ", "wrong, ", "incorrect", "that's wrong", "not what i asked",
		"不对", "错误", "不是", "你错了", "重新", "redo", "retry",
		"actually, ", "i meant", "应该是",
	}
	lower := strings.ToLower(userMessage)
	for _, phrase := range correctionPhrases {
		if strings.HasPrefix(lower, phrase) || strings.Contains(lower, phrase) {
			return &FeedbackEvent{
				ID:        fmt.Sprintf("corr_%d", time.Now().UnixNano()),
				Type:      FeedbackCorrection,
				Detail:    fmt.Sprintf("matched phrase: %q", phrase),
				Timestamp: time.Now().UTC(),
			}
		}
	}
	return nil
}

// ── SSE Event Detection ───────────────────────────────────────────────────────

// OnSSEEvent processes SSE events in real-time to detect feedback signals.
// Returns a FeedbackEvent if a signal was detected, nil otherwise.
func (fd *FeedbackDetector) OnSSEEvent(ev base.ChatStreamEvent, sessionID, userID string) *FeedbackEvent {
	switch ev.Type {
	case "tool_call", "tool_start":
		if ev.Tool != "" {
			fd.cancelledStates[sessionID] = &cancelState{
				lastTool: ev.Tool,
				round:    0, // round tracking done at call site
			}
		}

	case "tool_result":
		// Check if the tool result indicates success (no error event followed)
		// We clear the cancelled state when we see a tool_result
		if fd.cancelledStates[sessionID] != nil {
			if fd.cancelledStates[sessionID].lastTool == ev.Tool {
				delete(fd.cancelledStates, sessionID)
			}
		}

	case "interrupted":
		cs := fd.cancelledStates[sessionID]
		if cs != nil {
			taskCat := fd.profileMgr.InferTaskCategory(userID, "")
			return &FeedbackEvent{
				ID:           fmt.Sprintf("cancel_%s_%d", sessionID, time.Now().UnixNano()),
				UserID:       userID,
				SessionID:    sessionID,
				Type:         FeedbackCancelled,
				ToolName:     cs.lastTool,
				TaskCategory: taskCat,
				Detail:       fmt.Sprintf("round=%d reason=%s", cs.round, ev.Content),
				Timestamp:    time.Now().UTC(),
			}
		}

	case "done":
		return &FeedbackEvent{
			ID:        fmt.Sprintf("done_%s_%d", sessionID, time.Now().UnixNano()),
			UserID:    userID,
			SessionID: sessionID,
			Type:      FeedbackSuccess,
			Timestamp: time.Now().UTC(),
		}
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jaccardSimilarity(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 1.0
	}
	setA := make(map[string]bool)
	for _, w := range wordsA {
		setA[w] = true
	}
	intersection := 0
	union := make(map[string]bool)
	for _, w := range wordsB {
		union[w] = true
		if setA[w] {
			intersection++
		}
	}
	for w := range setA {
		union[w] = true
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func tokenize(s string) []string {
	lower := strings.ToLower(s)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '!' || r == '?' || r == ';' || r == ':'
	})
	var result []string
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
		"were": true, "to": true, "of": true, "in": true, "for": true, "on": true,
		"and": true, "or": true, "it": true, "i": true, "you": true, "he": true,
		"she": true, "we": true, "they": true, "this": true, "that": true,
		"的": true, "了": true, "是": true, "我": true, "你": true, "在": true,
	}
	for _, w := range words {
		if len(w) > 1 && !stopWords[w] {
			result = append(result, w)
		}
	}
	return result
}
