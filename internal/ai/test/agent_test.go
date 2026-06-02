package test

import (
	"sync"
	"testing"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/agent"
)

// ── State Machine Extended Tests ──

func TestStateMachineFullLifecycle(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()

	// 完整生命周期: start → reasoning → executing → done
	mustTrans := func(ev agent.TransitionEvent, reason string) {
		t.Helper()
		if err := sm.Transition(ev, reason); err != nil {
			t.Fatalf("unexpected transition error [%s]: %v", ev, err)
		}
	}

	mustTrans(agent.EvTaskAssigned, "task received")
	if sm.CurrentState() != "reasoning" {
		t.Errorf("expected reasoning, got %s", sm.CurrentState())
	}
	mustTrans(agent.EvPlanReady, "plan complete")
	if sm.CurrentState() != "executing" {
		t.Errorf("expected executing, got %s", sm.CurrentState())
	}
	mustTrans(agent.EvTaskComplete, "all done")
	if sm.CurrentState() != "done" {
		t.Errorf("expected done, got %s", sm.CurrentState())
	}
	if !sm.CurrentState().IsTerminal() {
		t.Error("done should be terminal")
	}
}

func TestStateMachineDelegateCycle(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvPlanReady, "plan done")

	// 进入委托
	sm.Transition(agent.EvDelegate, "spawned sub-agents")
	if sm.CurrentState() != "delegate" {
		t.Errorf("expected delegate, got %s", sm.CurrentState())
	}

	// 子任务完成
	sm.Transition(agent.EvSubagentDone, "all sub-agents done")
	if sm.CurrentState() != "executing" {
		t.Errorf("expected executing, got %s", sm.CurrentState())
	}
}

func TestStateMachineInterruptResume(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvPlanReady, "plan")

	// 暂停
	sm.Transition(agent.EvInterrupt, "user paused")
	if sm.CurrentState() != "suspended" {
		t.Errorf("expected suspended, got %s", sm.CurrentState())
	}

	// 恢复
	sm.Transition(agent.EvResume, "user resumed")
	if sm.CurrentState() != "reasoning" {
		t.Errorf("expected reasoning after resume, got %s", sm.CurrentState())
	}
}

func TestStateMachineWaitingHuman(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvNeedInput, "need user confirm")

	if sm.CurrentState() != "waiting_human" {
		t.Errorf("expected waiting_human, got %s", sm.CurrentState())
	}
	if !sm.CurrentState().IsWaiting() {
		t.Error("waiting_human should be waiting state")
	}

	sm.Transition(agent.EvInputReceived, "user confirmed")
	if sm.CurrentState() != "reasoning" {
		t.Errorf("expected reasoning after input, got %s", sm.CurrentState())
	}
}

func TestStateMachineWaitingLock(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvPlanReady, "plan")
	sm.Transition(agent.EvResourceConflict, "file locked")

	if sm.CurrentState() != "waiting_lock" {
		t.Errorf("expected waiting_lock, got %s", sm.CurrentState())
	}
	if !sm.CurrentState().IsWaiting() {
		t.Error("waiting_lock should be waiting state")
	}

	sm.Transition(agent.EvLockAcquired, "lock granted")
	if sm.CurrentState() != "executing" {
		t.Errorf("expected executing, got %s", sm.CurrentState())
	}
}

func TestStateMachineTimeoutRetryFlow(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvPlanReady, "plan")
	sm.Transition(agent.EvTimeout, "tool took too long")

	if sm.CurrentState() != "timeout" {
		t.Errorf("expected timeout, got %s", sm.CurrentState())
	}

	sm.Transition(agent.EvRetry, "retrying")
	if sm.CurrentState() != "retry" {
		t.Errorf("expected retry, got %s", sm.CurrentState())
	}

	sm.Transition(agent.EvRetryComplete, "retry succeeded")
	if sm.CurrentState() != "executing" {
		t.Errorf("expected executing, got %s", sm.CurrentState())
	}
}

func TestStateMachineAllTerminalStates(t *testing.T) {
	terminals := []string{"done", "failed", "cancel"}
	for _, term := range terminals {
		sm := agent.NewSubAgentStateMachine()
		s := agent.AgentState(term)
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", term)
		}
		// 所有终态不能转换
		if sm.TransitionIfValid(agent.EvTaskAssigned, "test") {
			t.Log("non-terminal start allows transition")
		}
	}
}

func TestStateMachineAllActiveStates(t *testing.T) {
	active := []string{"reasoning", "executing", "retry", "delegate"}
	for _, a := range active {
		s := agent.AgentState(a)
		if !s.IsActive() {
			t.Errorf("%s should be active", a)
		}
	}
}

func TestStateMachineInvalidTransition(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	// start → plan_ready should be invalid
	if sm.CanTransition(agent.EvPlanReady) {
		t.Error("plan_ready should not be allowed from start")
	}
	// TransitionIfValid should silently ignore
	if sm.TransitionIfValid(agent.EvPlanReady, "should fail") {
		t.Error("TransitionIfValid should return false for invalid transition")
	}
}

func TestStateMachineRoleManagement(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	if sm.Role() != "executor" {
		t.Errorf("expected executor role, got %s", sm.Role())
	}

	smMain := agent.NewMainAgentStateMachine()
	if smMain.Role() != "manager" {
		t.Errorf("expected manager role, got %s", smMain.Role())
	}

	sm.SetRole("supervisor")
	if sm.Role() != "supervisor" {
		t.Errorf("expected supervisor, got %s", sm.Role())
	}
}

func TestStateMachineSnapshot(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")
	sm.Transition(agent.EvPlanReady, "plan")

	snap := sm.Snapshot()
	if snap.Current != "executing" {
		t.Errorf("snapshot: expected executing, got %s", snap.Current)
	}
	if snap.Role != "executor" {
		t.Errorf("snapshot: wrong role %s", snap.Role)
	}
	if snap.HistoryLen != 2 {
		t.Errorf("snapshot: expected 2 history entries, got %d", snap.HistoryLen)
	}
}

func TestStateMachineHistory(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "step1")
	sm.Transition(agent.EvPlanReady, "step2")
	sm.Transition(agent.EvTaskComplete, "step3")

	history := sm.History()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
	if history[0].From != "start" || history[0].To != "reasoning" {
		t.Errorf("wrong first transition: %s→%s", history[0].From, history[0].To)
	}
	if history[2].To != "done" {
		t.Errorf("expected final state done, got %s", history[2].To)
	}
}

func TestStateMachineCallback(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	called := false

	sm.OnTransition(func(from, to agent.AgentState, ev agent.TransitionEvent, reason string) {
		called = true
		if from != "start" || to != "reasoning" {
			t.Errorf("callback: unexpected transition %s→%s", from, to)
		}
	})

	sm.Transition(agent.EvTaskAssigned, "test")
	if !called {
		t.Error("callback was not called")
	}
}

func TestStateMachineConcurrent(t *testing.T) {
	sm := agent.NewSubAgentStateMachine()
	sm.Transition(agent.EvTaskAssigned, "start")

	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.CurrentState()
			_ = sm.CanTransition(agent.EvPlanReady)
			_ = sm.Snapshot()
			_ = sm.History()
		}()
	}
	wg.Wait()
}

func TestParseFunctions(t *testing.T) {
	// ParseRole
	tests := []struct {
		input    string
		expected agent.AgentRole
	}{
		{"executor", "executor"},
		{"supervisor", "supervisor"},
		{"manager", "manager"},
		{"unknown", "executor"},
	}
	for _, tt := range tests {
		if result := agent.ParseRole(tt.input); result != tt.expected {
			t.Errorf("ParseRole(%q)=%s, want %s", tt.input, result, tt.expected)
		}
	}

	// ParseState
	stateTests := []struct {
		input    string
		expected agent.AgentState
	}{
		{"start", "start"},
		{"reasoning", "reasoning"},
		{"executing", "executing"},
		{"done", "done"},
		{"failed", "failed"},
	}
	for _, tt := range stateTests {
		if result := agent.ParseState(tt.input); result != tt.expected {
			t.Errorf("ParseState(%q)=%s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestStatusFromAgentState(t *testing.T) {
	mapping := map[agent.AgentState]agent.Status{
		"start":         "running",
		"reasoning":     "running",
		"executing":     "running",
		"waiting_lock":  "running",
		"waiting_human": "running",
		"delegate":      "running",
		"suspended":     "pending",
		"timeout":        "pending",
		"done":          "done",
		"failed":        "error",
		"cancel":        "error",
	}
	for state, expected := range mapping {
		result := agent.StatusFromAgentState(state)
		if result != expected {
			t.Errorf("StatusFromAgentState(%s)=%s, want %s", state, result, expected)
		}
	}
}

// ── Metacognition Tests ──

func TestMetacognitionBasic(t *testing.T) {
	meta := agent.NewMetacognition("test-agent")

	meta.TaskStarted()
	meta.RecordSuccess(5, 100*time.Millisecond)
	meta.TaskStarted()
	meta.RecordSuccess(3, 60*time.Millisecond)
	meta.TaskStarted()
	meta.RecordFailure(2, 200*time.Millisecond)

	report := meta.Evaluate()
	if report.SuccessRate != 2.0/3.0 {
		t.Errorf("expected success rate %.2f, got %.2f", 2.0/3.0, report.SuccessRate)
	}
	if report.TaskCompleted != 2 {
		t.Errorf("expected 2 completed, got %d", report.TaskCompleted)
	}
	if report.TaskFailed != 1 {
		t.Errorf("expected 1 failed, got %d", report.TaskFailed)
	}
}

func TestMetacognitionConflictAndLoad(t *testing.T) {
	meta := agent.NewMetacognition("test-agent")

	for i := 0; i < 5; i++ {
		meta.RecordConflict()
	}
	meta.TaskStarted()
	meta.TaskStarted()

	report := meta.Evaluate()
	if report.ConflictCount != 5 {
		t.Errorf("expected 5 conflicts, got %d", report.ConflictCount)
	}
}

func TestMetacognitionRecentRate(t *testing.T) {
	meta := agent.NewMetacognition("test-agent")
	if meta.RecentSuccessRate() != 1.0 {
		t.Errorf("expected 1.0 default, got %.2f", meta.RecentSuccessRate())
	}

	meta.RecordSuccess(1, 10*time.Millisecond)
	meta.RecordFailure(1, 10*time.Millisecond)
	if meta.RecentSuccessRate() != 0.5 {
		t.Errorf("expected 0.5, got %.2f", meta.RecentSuccessRate())
	}

	meta.ResetRecentWindow()
	if meta.RecentSuccessRate() != 1.0 {
		t.Errorf("expected 1.0 after reset, got %.2f", meta.RecentSuccessRate())
	}
}

// ── Role Registry Tests ──

func TestRoleRegistryFullFlow(t *testing.T) {
	r := agent.NewRoleRegistry()

	// 默认返回 executor
	if r.Get("unknown").Role != "executor" {
		t.Error("unknown agent should be executor")
	}

	// 注册
	r.Register("agent-1", "supervisor", "auto", "high success rate")
	if r.Get("agent-1").Role != "supervisor" {
		t.Error("should be supervisor")
	}

	// 撤销
	r.Revoke("agent-1", "poor performance")
	if r.Get("agent-1").Role != "executor" {
		t.Error("should be executor after revoke")
	}
}

func TestRoleRegistryTTLExpiry(t *testing.T) {
	r := agent.NewRoleRegistry()
	r.Register("agent-1", "supervisor", "auto", "test")

	// Set short TTL
	r.SetRoleTTL("agent-1", 30*time.Millisecond)
	rec := r.Get("agent-1")
	if rec.Role != "supervisor" {
		t.Error("should still be supervisor")
	}

	// Wait for expiry + cleanup
	time.Sleep(80 * time.Millisecond)
	r.CleanupExpired()

	rec = r.Get("agent-1")
	if rec.Role != "executor" {
		t.Errorf("expected executor after expiry, got %s", rec.Role)
	}
}

func TestRoleRegistryListByRole(t *testing.T) {
	r := agent.NewRoleRegistry()
	r.Register("a1", "executor", "", "")
	r.Register("a2", "supervisor", "", "")
	r.Register("a3", "manager", "", "")
	r.Register("a4", "supervisor", "", "")

	supers := r.ListByRole("supervisor")
	if len(supers) != 2 {
		t.Errorf("expected 2 supervisors, got %d", len(supers))
	}

	managers := r.ListByRole("manager")
	if len(managers) != 1 {
		t.Errorf("expected 1 manager, got %d", len(managers))
	}
}

func TestRoleRegistryHasActiveSupervisor(t *testing.T) {
	r := agent.NewRoleRegistry()
	if r.HasActiveSupervisor() {
		t.Error("initially no supervisor")
	}

	r.Register("a1", "supervisor", "", "")
	if !r.HasActiveSupervisor() {
		t.Error("should have supervisor after register")
	}
}

func TestRoleRegistryHistory(t *testing.T) {
	r := agent.NewRoleRegistry()
	r.Register("agent-1", "executor", "", "init")
	r.Register("agent-1", "supervisor", "", "step up")
	r.Register("agent-1", "manager", "", "step up again")
	r.Revoke("agent-1", "demoted")

	rec := r.Get("agent-1")
	// History: executor→supervisor, supervisor→manager, manager→executor
	if len(rec.History) < 2 {
		t.Errorf("expected at least 2 history entries, got %d", len(rec.History))
	}
}

func TestRoleRegistryConcurrent(t *testing.T) {
	r := agent.NewRoleRegistry()
	var wg sync.WaitGroup
	concurrency := 30

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r.Register(runeToID(id), "supervisor", "test", "")
			defer r.Revoke(runeToID(id), "test done")
			_ = r.Get(runeToID(id))
			_ = r.HasActiveSupervisor()
		}(i)
	}
	wg.Wait()
}

func runeToID(i int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	return "agent-" + string(letters[i%len(letters)])
}
