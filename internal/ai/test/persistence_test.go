package test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/observability"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/persistence"
)

// ── WAL Tests ──

func TestWALWriteAndReplay(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := persistence.NewWAL(walPath)
	if err != nil {
		t.Fatalf("NewWAL failed: %v", err)
	}
	defer w.Close()

	w.Write("sess-1", "state_change", "reasoning", `{"from":"start","to":"reasoning"}`)
	w.Write("sess-1", "state_change", "executing", `{"from":"reasoning","to":"executing"}`)
	w.Write("sess-1", "tool_call", "executing", `{"tool":"file_read"}`)
	w.Write("sess-1", "state_change", "done", `{"from":"executing","to":"done"}`)
	w.Sync()

	entries, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].SessionID != "sess-1" {
		t.Errorf("wrong session: %s", entries[0].SessionID)
	}
	if entries[3].State != "done" {
		t.Errorf("wrong final state: %s", entries[3].State)
	}
}

func TestWALWriteStateChange(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "statechange.wal")

	w, _ := persistence.NewWAL(walPath)
	defer w.Close()

	w.WriteStateChange("sess-1", "start", "reasoning", "task assigned")
	w.WriteToolCall("sess-1", "file_read", `{"path":"/test"}`)

	entries, _ := w.Replay()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Type != "state_change" {
		t.Errorf("wrong type: %s", entries[0].Type)
	}
	if entries[1].Type != "tool_call" {
		t.Errorf("wrong type: %s", entries[1].Type)
	}
}

func TestWALLastLSN(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "lsn.wal")

	w, _ := persistence.NewWAL(walPath)
	defer w.Close()

	for i := 0; i < 5; i++ {
		w.Write("sess-1", "state_change", "test", "")
	}
	if w.LastLSN() != 5 {
		t.Errorf("expected LSN 5, got %d", w.LastLSN())
	}
}

func TestWALReplayEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "empty.wal")

	w, _ := persistence.NewWAL(walPath)
	w.Close()

	entries, err := w.Replay()
	if err != nil {
		t.Fatalf("Replay empty failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestWALReplayCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "corrupt.wal")

	w, _ := persistence.NewWAL(walPath)
	w.Write("sess-1", "good", "ok", "{}")
	w.Sync()
	w.Close() // Close before manual write

	// Manually append a bad line
	f, _ := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("this is not valid json\n")
	f.Close()

	// Re-open WAL for replay
	w2, _ := persistence.NewWAL(walPath)
	defer w2.Close()
	entries, err := w2.Replay()
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
}

// ── Snapshot Tests ──

func TestSnapshotSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	sm, err := persistence.NewSnapshotManager(tmpDir)
	if err != nil {
		t.Fatalf("NewSnapshotManager: %v", err)
	}

	snap := persistence.NewSnapshot("agent-1", "sess-1", "suspended", "supervisor")
	snap.Goal = "Test task"
	snap.Round = 5
	snap.Progress = 0.5
	snap.MessagesJSON = `[{"role":"user","content":"hello"}]`
	snap.Metadata["source"] = "unit-test"

	err = sm.Save(snap)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := sm.Load("agent-1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.State != "suspended" {
		t.Errorf("wrong state: %s", loaded.State)
	}
	if loaded.Goal != "Test task" {
		t.Errorf("wrong goal: %s", loaded.Goal)
	}
	if loaded.Round != 5 {
		t.Errorf("wrong round: %d", loaded.Round)
	}
	if loaded.Version != 1 {
		t.Errorf("wrong version: %d", loaded.Version)
	}
}

func TestSnapshotVersionIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	sm, _ := persistence.NewSnapshotManager(tmpDir)

	for i := 0; i < 3; i++ {
		snap := persistence.NewSnapshot("agent-1", "sess-1", "test", "executor")
		sm.Save(snap)
	}

	loaded, _ := sm.Load("agent-1")
	if loaded.Version != 3 {
		t.Errorf("expected version 3, got %d", loaded.Version)
	}
}

func TestSnapshotRollback(t *testing.T) {
	tmpDir := t.TempDir()
	sm, _ := persistence.NewSnapshotManager(tmpDir)

	for i := 0; i < 5; i++ {
		snap := persistence.NewSnapshot("agent-1", "sess-1", "test", "executor")
		sm.Save(snap)
	}

	rolled, err := sm.Rollback("agent-1", 3)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.Version != 3 {
		t.Errorf("expected version 3, got %d", rolled.Version)
	}

	versions := sm.ListVersions("agent-1")
	if len(versions) != 3 {
		t.Errorf("expected 3 after rollback, got %d", len(versions))
	}
}

func TestSnapshotDelete(t *testing.T) {
	tmpDir := t.TempDir()
	sm, _ := persistence.NewSnapshotManager(tmpDir)

	snap := persistence.NewSnapshot("agent-1", "sess-1", "done", "executor")
	sm.Save(snap)
	sm.Delete("agent-1")

	_, err := sm.Load("agent-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestSnapshotCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	sm, _ := persistence.NewSnapshotManager(tmpDir)

	for i := 0; i < 8; i++ {
		snap := persistence.NewSnapshot("agent-1", "sess-1", "test", "executor")
		sm.Save(snap)
	}

	sm.Cleanup("agent-1", 3)
	versions := sm.ListVersions("agent-1")
	if len(versions) != 3 {
		t.Errorf("expected 3 after cleanup, got %d", len(versions))
	}
}

func TestSnapshotFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	sm, _ := persistence.NewSnapshotManager(tmpDir)

	snap := persistence.NewSnapshot("agent-1", "sess-1", "done", "executor")
	sm.Save(snap)

	path := filepath.Join(tmpDir, "agent-1_v1.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("snapshot file not found: %s", path)
	}
}

// ── Observability Tests ──

func TestAgentMetricsBasic(t *testing.T) {
	m := observability.NewAgentMetrics()

	m.RecordRequest()
	m.RecordTokens(100, 50)
	m.RecordToolCall("file_read", 15)
	m.RecordToolCall("file_read", 25)
	m.RecordToolCall("run_command", 200)
	m.RecordRequestEnd(false)

	snap := m.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", snap.TotalRequests)
	}
	if snap.TotalTokensIn != 100 {
		t.Errorf("expected 100 tokens in, got %d", snap.TotalTokensIn)
	}
	if snap.TotalTokensOut != 50 {
		t.Errorf("expected 50 tokens out, got %d", snap.TotalTokensOut)
	}
}

func TestAgentMetricsSubAgent(t *testing.T) {
	m := observability.NewAgentMetrics()

	m.RecordSubAgentSpawn()
	m.RecordSubAgentSpawn()
	m.RecordSubAgentSpawn()

	m.RecordSubAgentResult(true)
	m.RecordSubAgentResult(true)
	m.RecordSubAgentResult(false)

	rate := m.SubAgentSuccessRate()
	if rate != 2.0/3.0 {
		t.Errorf("expected %.2f success rate, got %.2f", 2.0/3.0, rate)
	}
}

func TestAgentMetricsTopology(t *testing.T) {
	m := observability.NewAgentMetrics()

	m.RecordTopologyReject()
	m.RecordTopologyReject()
	m.RecordTopologyLie()
	m.RecordTrustLock()

	snap := m.Snapshot()
	if snap.TopologyRejects != 2 {
		t.Errorf("expected 2 rejects, got %d", snap.TopologyRejects)
	}
	if snap.TopologyLies != 1 {
		t.Errorf("expected 1 lie, got %d", snap.TopologyLies)
	}
	if snap.TrustLockCount != 1 {
		t.Errorf("expected 1 trust lock, got %d", snap.TrustLockCount)
	}
}

func TestAgentMetricsLockAndRole(t *testing.T) {
	m := observability.NewAgentMetrics()

	m.RecordLockRequest(35)
	m.RecordLockConflict()
	m.RecordRolePromotion()
	m.RecordRolePromotion()
	m.RecordRoleDemotion()

	snap := m.Snapshot()
	if snap.LockRequests != 1 {
		t.Errorf("expected 1 lock request, got %d", snap.LockRequests)
	}
	if snap.LockConflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", snap.LockConflicts)
	}
	if snap.RolePromotions != 2 {
		t.Errorf("expected 2 promotions, got %d", snap.RolePromotions)
	}
}

func TestAgentMetricsToolStats(t *testing.T) {
	m := observability.NewAgentMetrics()

	m.RecordToolCall("fast_tool", 5)
	m.RecordToolCall("fast_tool", 15)
	m.RecordToolCall("slow_tool", 500)
	m.RecordToolCall("slow_tool", 700)
	m.RecordToolCall("slow_tool", 300)

	stats := m.TopToolStats(2)
	if len(stats) > 2 {
		t.Errorf("expected max 2 stats, got %d", len(stats))
	}
}

func TestAgentMetricsConcurrent(t *testing.T) {
	m := observability.NewAgentMetrics()
	var wg sync.WaitGroup
	concurrency := 50

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RecordRequest()
			m.RecordTokens(10, 5)
			m.RecordToolCall("test", 10)
			m.RecordRequestEnd(false)
		}()
	}
	wg.Wait()

	snap := m.Snapshot()
	if snap.TotalRequests != int64(concurrency) {
		t.Errorf("expected %d requests, got %d", concurrency, snap.TotalRequests)
	}
}

func TestAgentMetricsPrometheusFormat(t *testing.T) {
	m := observability.NewAgentMetrics()
	m.RecordRequest()
	m.RecordTokens(100, 50)
	m.RecordRequestEnd(false)

	format := m.PrometheusFormat()
	if format == "" {
		t.Error("Prometheus format should not be empty")
	}
}

// ── Tracing Tests ──

func TestTraceIDGeneration(t *testing.T) {
	id1 := observability.GenerateTraceID()
	id2 := observability.GenerateTraceID()

	if id1 == id2 {
		t.Error("trace IDs should be unique")
	}
	if len(id1) != 32 {
		t.Errorf("expected 32-char hex, got %d", len(id1))
	}
}

func TestSpanIDGeneration(t *testing.T) {
	id1 := observability.GenerateSpanID()
	id2 := observability.GenerateSpanID()

	if id1 == id2 {
		t.Error("span IDs should be unique")
	}
	if len(id1) != 16 {
		t.Errorf("expected 16-char hex, got %d", len(id1))
	}
}

func TestTraceIDContextPropagation(t *testing.T) {
	ctx := t.Context()
	traceID := observability.GenerateTraceID()
	ctx = observability.WithTraceID(ctx, traceID)

	if observability.TraceIDFromCtx(ctx) != traceID {
		t.Error("trace ID not propagated through context")
	}
}

func TestSpanLifecycle(t *testing.T) {
	ctx := t.Context()
	traceID := "abc123def456"
	ctx = observability.WithTraceID(ctx, traceID)

	ctx, span := observability.NewSpan(ctx, "test-operation")
	span.SetAttr("key", "value")
	span.AddEvent("step-1", map[string]string{"detail": "started"})
	span.Finish()

	if span.TraceID != traceID {
		t.Errorf("wrong trace ID: %s", span.TraceID)
	}
	if span.Name != "test-operation" {
		t.Errorf("wrong name: %s", span.Name)
	}
	if span.Duration() < 0 {
		t.Error("duration should be non-negative")
	}
	if span.Attributes["key"] != "value" {
		t.Error("attribute not set")
	}
}

func TestObservabilityTracer(t *testing.T) {
	tracer := observability.NewTracer("test-service")

	ctx := t.Context()
	ctx, span1 := tracer.StartSpan(ctx, "operation-1")
	span1.Finish()

	ctx, span2 := tracer.StartSpan(ctx, "operation-2")
	span2.Finish()

	spans := tracer.RecentSpans(10)
	if len(spans) < 2 {
		t.Errorf("expected at least 2 spans, got %d", len(spans))
	}
}
