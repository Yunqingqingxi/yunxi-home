package ai

import (
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/agent"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// ── Interrupt Types ──────────────────────────────────────────────

// CancelMode controls how a session is cancelled.
type CancelMode string

const (
	// CancelSoft: injectCh sends Priority=interrupt, current round completes, then saves snapshot.
	CancelSoft CancelMode = "soft"
	// CancelHard: immediately cancel LLM stream + tool context, then save snapshot.
	CancelHard CancelMode = "hard"
	// CancelModeSnapshot: only flush topology + session to DB without interrupting the stream.
	CancelModeSnapshot CancelMode = "snapshot"
)

// InjectCh priority levels. Lower number = higher priority.
const (
	PriorityInterrupt = 0
	PriorityUser      = 1
	PrioritySystem    = 2
	PriorityInfo      = 3
)

// InterruptSnapshot is returned to the frontend after a session is interrupted.
type InterruptSnapshot struct {
	Round    int    `json:"round"`
	Progress int    `json:"progress"`
	LastTool string `json:"last_tool"`
	LastTask string `json:"last_task"`
	Mode     string `json:"mode"`
	Status   string `json:"status"`
}

// ── CancelSession ────────────────────────────────────────────────

// CancelSession cancels an active session stream.
//   - soft: signals the agent loop to stop after the current round
//   - hard: immediately cancels the LLM stream context
//   - snapshot: flushes state to DB without interrupting
func (s *Service) CancelSession(sessionID string, mode string) (any, error) {
	log.Info("CancelSession 收到请求", "session", sessionID, "mode", mode)

	s.activeStreamsMu.Lock()
	streamCancel, hasStream := s.activeStreams[sessionID]
	streamCh := s.activeStreamChs[sessionID]
	s.activeStreamsMu.Unlock()

	if !hasStream {
		// Check if session exists at all
		if sess := s.sessions.Get(sessionID); sess == nil {
			log.Warn("CancelSession: 会话不存在", "session", sessionID)
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		// Session exists but no active stream — mark as interrupted anyway
		s.sessions.SetState(sessionID, models.SessionStateInterrupted, "", "")
		log.Info("CancelSession: 无活跃流，标记为已中断", "session", sessionID)
		return &InterruptSnapshot{
			Status: "idle",
			Mode:   mode,
		}, nil
	}

	// Build snapshot from current session state
	snapshot := s.buildSnapshot(sessionID)
	cancelMode := CancelMode(mode)

	log.Info("CancelSession: 开始执行中断",
		"session", sessionID,
		"cancel_mode", mode,
		"progress", snapshot.Progress,
		"last_tool", snapshot.LastTool,
		"round", snapshot.Round,
	)

	switch cancelMode {
	case CancelHard:
		// Immediately cancel the stream context → LLM stream + tool heartbeat both stop
		if streamCancel != nil {
			streamCancel()
			log.Info("CancelSession: hard cancel 已触发 streamCancel", "session", sessionID)
		}
		snapshot.Mode = "hard"
		snapshot.Status = "interrupted"

	case CancelModeSnapshot:
		// Just flush — don't interrupt
		s.saveSnapshot(sessionID)
		snapshot.Mode = "snapshot"
		snapshot.Status = "saved"
		log.Info("CancelSession: snapshot 已保存", "session", sessionID)

	default: // CancelSoft
		// Send interrupt signal through injectCh — the loop picks it up at next iteration
		msg := fmt.Sprintf("[中断] 用户请求停止，进度 %d%%，最后执行：%s",
			snapshot.Progress, snapshot.LastTask)
		s.InjectWithPriority(sessionID, msg, "interrupt", "cancel_session")
		log.Info("CancelSession: soft interrupt 已注入 injectCh", "session", sessionID, "msg", msg)
		snapshot.Mode = "soft"
		snapshot.Status = "interrupting"
	}

	// Emit interrupted event to SSE channel
	if streamCh != nil {
		ev := base.ChatStreamEvent{
			Type:    "interrupted",
			Content: fmt.Sprintf("进度 %d%%，最后执行：%s", snapshot.Progress, snapshot.LastTask),
		}
		select {
		case streamCh <- ev:
			log.Debug("CancelSession: interrupted SSE 已发送到主通道", "session", sessionID)
		default:
			log.Warn("CancelSession: 主 SSE 通道已满，仅发送到 eventBus", "session", sessionID)
		}
		// Also publish to event bus for reconnecting clients
		eb := s.eventBus.getOrCreate(sessionID)
		eb.publish(ev)
	}

	log.Info("CancelSession: 完成",
		"session", sessionID,
		"mode", mode,
		"snapshot_status", snapshot.Status,
		"progress", snapshot.Progress,
		"last_tool", snapshot.LastTool,
	)

	return snapshot, nil
}

// ── Snapshot ────────────────────────────────────────────────────

// buildSnapshot creates a progress snapshot from the current session state.
func (s *Service) buildSnapshot(sessionID string) *InterruptSnapshot {
	snap := &InterruptSnapshot{
		Status: "running",
	}

	// Get topology state for progress estimation
	if s.tracker != nil {
		st := s.tracker.GetState(sessionID)
		if st != nil {
			// Progress based on coordinate X (0..10 scale)
			snap.Progress = int(st.CurrentCoord.X * 10)
			if snap.Progress > 100 {
				snap.Progress = 100
			}
			snap.Round = len(st.Trajectory)
			if len(st.Trajectory) > 0 {
				last := st.Trajectory[len(st.Trajectory)-1]
				snap.LastTool = last.ToolCall
			}
		}
	}

	// Fallback: get round from session turn stats
	if snap.Round == 0 {
		stats := s.sessions.GetTurnStats(sessionID)
		snap.Round = stats.TurnCount
		if snap.Progress == 0 && stats.TurnCount > 0 {
			snap.Progress = stats.TurnCount * 10
			if snap.Progress > 100 {
				snap.Progress = 95
			}
		}
	}

	if snap.LastTask == "" && snap.LastTool != "" {
		snap.LastTask = snap.LastTool
	}

	return snap
}

// saveSnapshot persists the session state to DB.
func (s *Service) saveSnapshot(sessionID string) {
	// Persist session state
	s.sessions.SetState(sessionID, models.SessionStateInterrupted, "", "")

	log.Info("snapshot 已持久化",
		"session", sessionID,
		"state", models.SessionStateInterrupted,
	)
}

// ── InjectCh Priority Draining ──────────────────────────────────

// drainInjectChNonBlocking reads all available messages from injectCh,
// sorts by priority (lower = higher), and returns them. If an interrupt
// message is found, returns interrupt=true and the pre-interrupt messages.
func (s *Service) drainInjectChNonBlocking(sessionID string) (msgs []InjectedMessage, hasInterrupt bool) {
	s.injectMu.RLock()
	ch, ok := s.injections[sessionID]
	s.injectMu.RUnlock()
	if !ok {
		return nil, false
	}

	// Drain all available messages
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
			if msg.Priority == "interrupt" && msg.Source == "cancel_session" {
				hasInterrupt = true
			}
		default:
			// Sort by priority: interrupt < user < system < info
			sortInjectMessages(msgs)
			if len(msgs) > 0 {
				log.Debug("injectCh 批量消费",
					"session", sessionID,
					"count", len(msgs),
					"has_interrupt", hasInterrupt,
				)
			}
			return msgs, hasInterrupt
		}
	}
}

// sortInjectMessages sorts messages by priority (lower = higher priority).
func sortInjectMessages(msgs []InjectedMessage) {
	if len(msgs) <= 1 {
		return
	}
	// Simple insertion sort — injectCh batch sizes are small (< 10)
	for i := 1; i < len(msgs); i++ {
		for j := i; j > 0 && priorityValue(msgs[j].Priority) < priorityValue(msgs[j-1].Priority); j-- {
			msgs[j], msgs[j-1] = msgs[j-1], msgs[j]
		}
	}
}

func priorityValue(p string) int {
	switch p {
	case "interrupt":
		return PriorityInterrupt
	case "user":
		return PriorityUser
	case "system":
		return PrioritySystem
	default:
		return PriorityInfo
	}
}

// ── Resume Detection ────────────────────────────────────────────

// IsSessionInterrupted checks if a session was interrupted.
func (s *Service) IsSessionInterrupted(sessionID string) bool {
	state, _, _ := s.sessions.GetState(sessionID)
	return state == models.SessionStateInterrupted
}

// HasActiveStream checks if there's an active LLM stream for the session.
func (s *Service) HasActiveStream(sessionID string) bool {
	s.activeStreamsMu.Lock()
	defer s.activeStreamsMu.Unlock()
	_, ok := s.activeStreams[sessionID]
	return ok
}

// HasRunningAgents checks if any sub-agents are currently running for the given session.
func (s *Service) HasRunningAgents(sessionID string) bool {
	for _, a := range s.agentMgr.ListAll() {
		if a.ParentID == sessionID && (a.Status == "running" || a.Status == "pending") {
			return true
		}
	}
	return false
}

// GetSessionAgents returns all sub-agents for the given session.
func (s *Service) GetSessionAgents(sessionID string) []*agent.SubAgent {
	var result []*agent.SubAgent
	for _, a := range s.agentMgr.ListAll() {
		if a.ParentID == sessionID {
			result = append(result, a)
		}
	}
	return result
}

// ── RebaseTopology ──────────────────────────────────────────────

// RebaseTopology rewinds the topology tracker to match a given message index.
// Returns the number of deleted topology nodes.
func (s *Service) RebaseTopology(sessionID string, messageIndex int) (int, error) {
	if s.tracker == nil {
		return 0, nil
	}
	topoRound := messageIndex / 2 // rough: each user+assistant pair ≈ 1 topology round
	return s.tracker.Rebase(sessionID, topoRound)
}
