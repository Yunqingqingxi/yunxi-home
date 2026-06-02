package agent

import (
	"sync"
	"testing"
	"time"
)

// ── State Transition Tests (from agent-lifecycle.md §7.1) ──

func TestStateMachine_InitToReasoning(t *testing.T) {
	sm := NewSubAgentStateMachine()
	if sm.CurrentState() != StateStart {
		t.Fatalf("expected start, got %s", sm.CurrentState())
	}
	if err := sm.Transition(EvTaskAssigned, "test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateReasoning {
		t.Fatalf("expected reasoning, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ReasoningToExecuting(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	if err := sm.Transition(EvPlanReady, "plan done"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateExecuting {
		t.Fatalf("expected executing, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ReasoningToWaitingHuman(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	if err := sm.Transition(EvNeedInput, "need confirmation"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateWaitingHuman {
		t.Fatalf("expected waiting_human, got %s", sm.CurrentState())
	}
}

func TestStateMachine_WaitingHumanToReasoning(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvNeedInput, "need input")
	if err := sm.Transition(EvInputReceived, "user responded"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateReasoning {
		t.Fatalf("expected reasoning after input, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ExecutingToWaitingLock(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan done")
	if err := sm.Transition(EvResourceConflict, "file lock conflict"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateWaitingLock {
		t.Fatalf("expected waiting_lock, got %s", sm.CurrentState())
	}
}

func TestStateMachine_WaitingLockToExecuting(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan done")
	sm.Transition(EvResourceConflict, "conflict")
	if err := sm.Transition(EvLockAcquired, "lock granted"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateExecuting {
		t.Fatalf("expected executing after lock, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ExecutingToDelegate(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan done")
	if err := sm.Transition(EvDelegate, "spawned sub-agent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateDelegate {
		t.Fatalf("expected delegate, got %s", sm.CurrentState())
	}
}

func TestStateMachine_DelegateToExecuting(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan done")
	sm.Transition(EvDelegate, "spawned")
	if err := sm.Transition(EvSubagentDone, "all sub-agents complete"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateExecuting {
		t.Fatalf("expected executing after delegate, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ExecutingToSuspended(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan done")
	if err := sm.Transition(EvInterrupt, "user paused"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateSuspended {
		t.Fatalf("expected suspended, got %s", sm.CurrentState())
	}
}

func TestStateMachine_SuspendedToReasoning(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvInterrupt, "user paused")
	if err := sm.Transition(EvResume, "user resumed"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateReasoning {
		t.Fatalf("expected reasoning after resume, got %s", sm.CurrentState())
	}
}

func TestStateMachine_ToDone(t *testing.T) {
	tests := []struct {
		name  string
		setup func(sm *StateMachine)
	}{
		{"from reasoning", func(sm *StateMachine) { sm.Transition(EvTaskAssigned, "start") }},
		{"from executing", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvPlanReady, "plan")
		}},
		{"from delegate", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvPlanReady, "plan")
			sm.Transition(EvDelegate, "spawned")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSubAgentStateMachine()
			tt.setup(sm)
			if err := sm.Transition(EvTaskComplete, "done"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sm.CurrentState() != StateDone {
				t.Fatalf("expected done, got %s", sm.CurrentState())
			}
		})
	}
}

func TestStateMachine_ToCancel(t *testing.T) {
	// Cancel should work from any non-terminal state
	states := []struct {
		name  string
		setup func(sm *StateMachine)
	}{
		{"from start", func(sm *StateMachine) {}},
		{"from reasoning", func(sm *StateMachine) { sm.Transition(EvTaskAssigned, "start") }},
		{"from executing", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvPlanReady, "plan")
		}},
		{"from waiting_lock", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvPlanReady, "plan")
			sm.Transition(EvResourceConflict, "conflict")
		}},
		{"from waiting_human", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvNeedInput, "need input")
		}},
		{"from suspended", func(sm *StateMachine) {
			sm.Transition(EvTaskAssigned, "start")
			sm.Transition(EvInterrupt, "paused")
		}},
	}

	for _, tt := range states {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewSubAgentStateMachine()
			tt.setup(sm)
			if err := sm.Transition(EvCancel, "user cancelled"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sm.CurrentState() != StateCancel {
				t.Fatalf("expected cancel, got %s", sm.CurrentState())
			}
		})
	}
}

func TestStateMachine_ToFailed(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan")
	if err := sm.Transition(EvError, "unrecoverable error"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateFailed {
		t.Fatalf("expected failed, got %s", sm.CurrentState())
	}
}

func TestStateMachine_TimeoutRetry(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan")
	sm.Transition(EvTimeout, "execution took too long")
	if sm.CurrentState() != StateTimeout {
		t.Fatalf("expected timeout, got %s", sm.CurrentState())
	}
	if err := sm.Transition(EvRetry, "retrying"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateRetry {
		t.Fatalf("expected retry, got %s", sm.CurrentState())
	}
	if err := sm.Transition(EvRetryComplete, "retry done"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateExecuting {
		t.Fatalf("expected executing after retry, got %s", sm.CurrentState())
	}
}

// ── Invalid Transition Tests ──

func TestStateMachine_InvalidTransition(t *testing.T) {
	sm := NewSubAgentStateMachine()
	// Cannot go from start directly to executing
	err := sm.Transition(EvPlanReady, "should fail")
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
	if _, ok := err.(*InvalidTransitionError); !ok {
		t.Fatalf("expected InvalidTransitionError, got %T", err)
	}
}

func TestStateMachine_TerminalNoTransition(t *testing.T) {
	terminals := []AgentState{StateDone, StateFailed, StateCancel}
	events := []TransitionEvent{EvTaskAssigned, EvPlanReady, EvError, EvCancel, EvResume}

	for _, term := range terminals {
		for _, ev := range events {
			sm := &StateMachine{current: term, history: make([]TransitionRecord, 0)}
			err := sm.Transition(ev, "test")
			if err == nil {
				t.Errorf("terminal state %s should reject event %s", term, ev)
			}
		}
	}
}

func TestStateMachine_CanTransition(t *testing.T) {
	sm := NewSubAgentStateMachine()
	if !sm.CanTransition(EvTaskAssigned) {
		t.Error("should allow task_assigned from start")
	}
	if sm.CanTransition(EvPlanReady) {
		t.Error("should NOT allow plan_ready from start")
	}

	sm.Transition(EvTaskAssigned, "start")
	if !sm.CanTransition(EvPlanReady) {
		t.Error("should allow plan_ready from reasoning")
	}
	if !sm.CanTransition(EvCancel) {
		t.Error("should allow cancel from reasoning")
	}
}

func TestStateMachine_TransitionIfValid(t *testing.T) {
	sm := NewSubAgentStateMachine()
	// Valid
	if !sm.TransitionIfValid(EvTaskAssigned, "test") {
		t.Error("expected true for valid transition")
	}
	// Invalid (already in reasoning, can't task_assign again)
	if sm.TransitionIfValid(EvTaskAssigned, "double assign") {
		t.Error("expected false for invalid transition")
	}
	// State should still be reasoning
	if sm.CurrentState() != StateReasoning {
		t.Errorf("expected reasoning, got %s", sm.CurrentState())
	}
}

// ── Callback Tests ──

func TestStateMachine_OnTransition(t *testing.T) {
	sm := NewSubAgentStateMachine()
	var called bool
	var from, to AgentState
	var ev TransitionEvent

	sm.OnTransition(func(f, t2 AgentState, e TransitionEvent, reason string) {
		called = true
		from = f
		to = t2
		ev = e
	})

	sm.Transition(EvTaskAssigned, "test")
	if !called {
		t.Fatal("callback was not called")
	}
	if from != StateStart || to != StateReasoning || ev != EvTaskAssigned {
		t.Errorf("callback got wrong args: %s->%s via %s", from, to, ev)
	}
}

func TestStateMachine_MultipleCallbacks(t *testing.T) {
	sm := NewSubAgentStateMachine()
	count := 0
	sm.OnTransition(func(_, _ AgentState, _ TransitionEvent, _ string) { count++ })
	sm.OnTransition(func(_, _ AgentState, _ TransitionEvent, _ string) { count++ })
	sm.OnTransition(func(_, _ AgentState, _ TransitionEvent, _ string) { count++ })

	sm.Transition(EvTaskAssigned, "test")
	if count != 3 {
		t.Errorf("expected 3 callbacks, got %d", count)
	}
}

// ── History Tests ──

func TestStateMachine_History(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "step 1")
	sm.Transition(EvPlanReady, "step 2")
	sm.Transition(EvTaskComplete, "step 3")

	history := sm.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[0].From != StateStart || history[0].To != StateReasoning {
		t.Errorf("wrong first entry: %s->%s", history[0].From, history[0].To)
	}
	if history[2].To != StateDone {
		t.Errorf("expected last to be done, got %s", history[2].To)
	}
}

func TestStateMachine_HistoryTruncation(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.maxHistory = 10 // Small for testing
	// Fill history with transitions
	for i := 0; i < 12; i++ {
		sm.mu.Lock()
		sm.current = StateStart // Reset to allow more transitions
		sm.mu.Unlock()
		sm.Transition(EvTaskAssigned, "step")
		sm.Transition(EvPlanReady, "step")
		sm.Transition(EvTaskComplete, "step")
	}
	history := sm.History()
	if len(history) > sm.maxHistory {
		t.Errorf("history should be truncated, got %d entries (max %d)", len(history), sm.maxHistory)
	}
}

// ── Snapshot Tests ──

func TestStateMachine_Snapshot(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")

	snap := sm.Snapshot()
	if snap.Current != StateReasoning {
		t.Errorf("snapshot state wrong: %s", snap.Current)
	}
	if snap.Role != RoleExecutor {
		t.Errorf("snapshot role wrong: %s", snap.Role)
	}
	if snap.HistoryLen != 1 {
		t.Errorf("snapshot history len wrong: %d", snap.HistoryLen)
	}
}

// ── Reset Tests ──

func TestStateMachine_Reset(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	sm.Transition(EvPlanReady, "plan")
	sm.Transition(EvTaskComplete, "done")

	sm.Reset()
	if sm.CurrentState() != StateStart {
		t.Errorf("expected start after reset, got %s", sm.CurrentState())
	}
	// History should be preserved
	if len(sm.History()) != 3 {
		t.Errorf("history should be preserved, got %d entries", len(sm.History()))
	}
}

// ── Role Tests ──

func TestStateMachine_RoleManagement(t *testing.T) {
	sm := NewSubAgentStateMachine()
	if sm.Role() != RoleExecutor {
		t.Errorf("sub-agent should start as executor, got %s", sm.Role())
	}

	smMain := NewMainAgentStateMachine()
	if smMain.Role() != RoleManager {
		t.Errorf("main agent should start as manager, got %s", smMain.Role())
	}

	smMain.SetRole(RoleSupervisor)
	if smMain.Role() != RoleSupervisor {
		t.Errorf("role not updated, got %s", smMain.Role())
	}
}

// ── Status Mapping Tests ──

func TestStatusFromAgentState(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected Status
	}{
		{StateStart, StatusRunning},
		{StateReasoning, StatusRunning},
		{StateExecuting, StatusRunning},
		{StateRetry, StatusRunning},
		{StateWaitingLock, StatusRunning},
		{StateWaitingHuman, StatusRunning},
		{StateDelegate, StatusRunning},
		{StateSuspended, StatusPending},
		{StateTimeout, StatusPending},
		{StateDone, StatusDone},
		{StateFailed, StatusError},
		{StateCancel, StatusError},
	}

	for _, tt := range tests {
		result := StatusFromAgentState(tt.state)
		if result != tt.expected {
			t.Errorf("StatusFromAgentState(%s) = %s, want %s", tt.state, result, tt.expected)
		}
	}
}

// ── Parse Functions ──

func TestParseRole(t *testing.T) {
	tests := []struct {
		input    string
		expected AgentRole
	}{
		{"executor", RoleExecutor},
		{"EXECUTOR", RoleExecutor},
		{"supervisor", RoleSupervisor},
		{"SUPERVISOR", RoleSupervisor},
		{"manager", RoleManager},
		{"MANAGER", RoleManager},
		{"unknown", RoleExecutor},
		{"", RoleExecutor},
	}

	for _, tt := range tests {
		result := ParseRole(tt.input)
		if result != tt.expected {
			t.Errorf("ParseRole(%q) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		input    string
		expected AgentState
	}{
		{"start", StateStart}, {"START", StateStart},
		{"reasoning", StateReasoning}, {"REASONING", StateReasoning},
		{"executing", StateExecuting}, {"EXECUTING", StateExecuting},
		{"waiting_lock", StateWaitingLock}, {"WAITING_LOCK", StateWaitingLock},
		{"waiting_human", StateWaitingHuman}, {"WAITING_HUMAN", StateWaitingHuman},
		{"delegate", StateDelegate}, {"DELEGATE", StateDelegate},
		{"suspended", StateSuspended}, {"SUSPENDED", StateSuspended},
		{"timeout", StateTimeout}, {"TIMEOUT", StateTimeout},
		{"retry", StateRetry}, {"RETRY", StateRetry},
		{"done", StateDone}, {"DONE", StateDone},
		{"failed", StateFailed}, {"FAILED", StateFailed},
		{"cancel", StateCancel}, {"CANCEL", StateCancel},
		{"unknown", StateStart}, {"", StateStart},
	}

	for _, tt := range tests {
		result := ParseState(tt.input)
		if result != tt.expected {
			t.Errorf("ParseState(%q) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

// ── IsTerminal / IsActive / IsWaiting Tests ──

func TestAgentState_IsTerminal(t *testing.T) {
	if StateDone.IsTerminal() != true {
		t.Error("done should be terminal")
	}
	if StateFailed.IsTerminal() != true {
		t.Error("failed should be terminal")
	}
	if StateCancel.IsTerminal() != true {
		t.Error("cancel should be terminal")
	}
	if StateExecuting.IsTerminal() != false {
		t.Error("executing should not be terminal")
	}
	if StateStart.IsTerminal() != false {
		t.Error("start should not be terminal")
	}
}

func TestAgentState_IsActive(t *testing.T) {
	for _, s := range []AgentState{StateReasoning, StateExecuting, StateRetry, StateDelegate} {
		if !s.IsActive() {
			t.Errorf("%s should be active", s)
		}
	}
	for _, s := range []AgentState{StateStart, StateDone, StateFailed, StateCancel} {
		if s.IsActive() {
			t.Errorf("%s should not be active", s)
		}
	}
}

func TestAgentState_IsWaiting(t *testing.T) {
	if !StateWaitingLock.IsWaiting() {
		t.Error("waiting_lock should be waiting")
	}
	if !StateWaitingHuman.IsWaiting() {
		t.Error("waiting_human should be waiting")
	}
	if StateExecuting.IsWaiting() {
		t.Error("executing should not be waiting")
	}
}

// ── Concurrency Tests ──

func TestStateMachine_ConcurrentAccess(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")

	var wg sync.WaitGroup
	concurrency := 100

	// Concurrent reads
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.CurrentState()
			_ = sm.CanTransition(EvPlanReady)
			_ = sm.Snapshot()
			_ = sm.History()
		}()
	}

	// Concurrent writes (different goroutine)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			sm.TransitionIfValid(EvPlanReady, "concurrent")
			sm.mu.Lock()
			sm.current = StateStart // Reset for next iteration
			sm.mu.Unlock()
		}
	}()

	wg.Wait()
	// No race detector errors = pass
}

// ── Edge Cases ──

func TestStateMachine_EmptyCallback(t *testing.T) {
	sm := NewSubAgentStateMachine()
	// Should not panic with nil-like callbacks
	sm.Transition(EvTaskAssigned, "test")
}

func TestStateMachine_Timestamp(t *testing.T) {
	sm := NewSubAgentStateMachine()
	started := sm.StartedAt()
	if started.After(time.Now()) {
		t.Error("started at should be in the past")
	}
	if sm.LastTransition().After(time.Now()) {
		t.Error("last transition should be in the past")
	}

	time.Sleep(10 * time.Millisecond)
	sm.Transition(EvTaskAssigned, "start")
	if !sm.LastTransition().After(started) {
		t.Error("last transition should be after started")
	}
}

func TestStateMachine_ReasoningDirectToDone(t *testing.T) {
	sm := NewSubAgentStateMachine()
	sm.Transition(EvTaskAssigned, "start")
	// Reasoning → Done (agent decides no action needed)
	if err := sm.Transition(EvTaskComplete, "nothing to do"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sm.CurrentState() != StateDone {
		t.Fatalf("expected done, got %s", sm.CurrentState())
	}
}

func TestInvalidTransitionError(t *testing.T) {
	err := &InvalidTransitionError{
		From:  StateStart,
		Event: EvPlanReady,
		Cause: "cannot execute from start",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}
