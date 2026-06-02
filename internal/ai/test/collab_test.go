package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/collab"
)

// ── Message Bus Tests ──

func TestMessageBusSubscribePublish(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	sub := bus.Subscribe("agent-1", []string{"test.topic"}, 32)
	defer bus.Unsubscribe("agent-1")

	bus.Publish("test.topic", "sender", []byte(`{"msg":"hello"}`), "")

	select {
	case msg := <-sub.Ch:
		if msg.Topic != "test.topic" {
			t.Errorf("wrong topic: %s", msg.Topic)
		}
		if msg.FromAgent != "sender" {
			t.Errorf("wrong sender: %s", msg.FromAgent)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBusPointToPoint(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	subA := bus.Subscribe("agent-A", []string{collab.TopicLockRequest}, 32)
	subB := bus.Subscribe("agent-B", []string{collab.TopicLockRequest}, 32)
	defer bus.Unsubscribe("agent-A")
	defer bus.Unsubscribe("agent-B")

	bus.Publish(collab.TopicLockRequest, "agent-A", []byte("ping"), "agent-B")

	// agent-A should not receive
	select {
	case <-subA.Ch:
		t.Error("agent-A should not receive point-to-point message")
	default:
	}

	// agent-B should receive
	select {
	case msg := <-subB.Ch:
		if string(msg.Payload) != "ping" {
			t.Errorf("wrong payload: %s", string(msg.Payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-B did not receive message")
	}
}

func TestMessageBusSelfExclude(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	sub := bus.Subscribe("self", []string{collab.TopicAgentAnnounce}, 32)
	defer bus.Unsubscribe("self")

	bus.Publish(collab.TopicAgentAnnounce, "self", []byte("hello"), "")

	select {
	case <-sub.Ch:
		t.Error("should not receive own broadcast")
	default:
	}
}

func TestMessageBusMultipleTopics(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	sub := bus.Subscribe("multi", []string{"topic1", "topic2"}, 64)
	defer bus.Unsubscribe("multi")

	bus.Publish("topic1", "src", []byte("msg1"), "")
	bus.Publish("topic2", "src", []byte("msg2"), "")

	received := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case msg := <-sub.Ch:
			received[string(msg.Payload)] = true
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for messages")
		}
	}
	if !received["msg1"] || !received["msg2"] {
		t.Error("did not receive both messages")
	}
}

func TestMessageBusUnsubscribe(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	sub := bus.Subscribe("temp", []string{"test"}, 32)
	bus.Unsubscribe("temp")

	bus.Publish("test", "src", []byte("msg"), "")

	select {
	case _, ok := <-sub.Ch:
		if ok {
			t.Error("should not receive after unsubscribe")
		}
	default:
	}
}

func TestMessageBusSubscriberCount(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	if bus.SubscriberCount() != 0 {
		t.Error("expected 0 initially")
	}

	bus.Subscribe("a1", []string{"t1"}, 32)
	bus.Subscribe("a2", []string{"t1"}, 32)

	if bus.SubscriberCount() != 2 {
		t.Errorf("expected 2, got %d", bus.SubscriberCount())
	}
	if bus.TopicSubscriberCount("t1") != 2 {
		t.Errorf("expected 2 topic subscribers, got %d", bus.TopicSubscriberCount("t1"))
	}
}

func TestMessageBusHistoryReplay(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	bus.Publish("topic.x", "early", []byte("historical"), "")

	sub := bus.Subscribe("late", []string{"topic.x"}, 64)
	defer bus.Unsubscribe("late")

	select {
	case msg := <-sub.Ch:
		if string(msg.Payload) != "historical" {
			t.Errorf("wrong payload: %s", string(msg.Payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no history replay received")
	}
}

func TestMessageBusClose(t *testing.T) {
	bus := collab.NewMessageBus()
	bus.Subscribe("c1", []string{"t1"}, 32)
	bus.Subscribe("c2", []string{"t1"}, 32)
	bus.Close()

	if bus.SubscriberCount() != 0 {
		t.Error("should have 0 after close")
	}
}

func TestMessageBusConcurrent(t *testing.T) {
	bus := collab.NewMessageBus()
	defer bus.Close()

	var wg sync.WaitGroup
	concurrency := 20

	for i := 0; i < concurrency; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		sub := bus.Subscribe(agentID, []string{"concurrent"}, 128)
		wg.Add(1)
		go func(s *collab.Subscription) {
			defer wg.Done()
			defer bus.Unsubscribe(s.AgentID)
			<-s.Ch
		}(sub)
	}

	time.Sleep(50 * time.Millisecond)
	bus.Publish("concurrent", "main", []byte("go!"), "")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for concurrent subscribers")
	}
}

// ── Lock Manager Tests ──

func TestLockManagerTryLock(t *testing.T) {
	lm := collab.NewLockManager()

	resp := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-1",
		ResourceID: "file:/tmp/test.txt",
		LockType:   collab.LockWrite,
		Priority:   5,
	})
	if !resp.Granted {
		t.Fatalf("expected granted: %s", resp.Message)
	}
	if resp.LeaseID == "" {
		t.Error("expected lease ID")
	}

	// 第二个应失败
	resp2 := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-2",
		ResourceID: "file:/tmp/test.txt",
		LockType:   collab.LockWrite,
		Priority:   10,
	})
	if resp2.Granted {
		t.Fatal("second should be denied")
	}
	if resp2.OwnerID != "agent-1" {
		t.Errorf("expected owner agent-1, got %s", resp2.OwnerID)
	}
}

func TestLockManagerRelease(t *testing.T) {
	lm := collab.NewLockManager()

	resp := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-1",
		ResourceID: "file:/tmp/release.txt",
		LockType:   collab.LockWrite,
	})
	if !resp.Granted {
		t.Fatal("failed to get lock")
	}

	lm.Release(resp.LeaseID)

	resp2 := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-2",
		ResourceID: "file:/tmp/release.txt",
		LockType:   collab.LockWrite,
	})
	if !resp2.Granted {
		t.Fatal("should get lock after release")
	}
}

func TestLockManagerReadLockSharing(t *testing.T) {
	lm := collab.NewLockManager()

	r1 := lm.TryLock(&collab.LockRequest{AgentID: "r1", ResourceID: "shared", LockType: collab.LockRead})
	if !r1.Granted {
		t.Fatal("r1 should get lock")
	}

	r2 := lm.TryLock(&collab.LockRequest{AgentID: "r2", ResourceID: "shared", LockType: collab.LockRead})
	if !r2.Granted {
		t.Fatal("r2 should share read lock")
	}

	// 写锁应被阻止
	r3 := lm.TryLock(&collab.LockRequest{AgentID: "w1", ResourceID: "shared", LockType: collab.LockWrite})
	if r3.Granted {
		t.Fatal("w1 should not get write lock while readers hold")
	}
}

func TestLockManagerRenew(t *testing.T) {
	lm := collab.NewLockManager()
	lm.SetDefaultTTL(100 * time.Millisecond)

	resp := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-1",
		ResourceID: "file:/tmp/renew.txt",
		LockType:   collab.LockWrite,
	})

	lm.Renew(resp.LeaseID, 1*time.Second)
	time.Sleep(150 * time.Millisecond)

	lease := lm.GetLease("file:/tmp/renew.txt")
	if lease == nil {
		t.Fatal("lease should still be active after renew")
	}
}

func TestLockManagerExpiry(t *testing.T) {
	lm := collab.NewLockManager()
	lm.SetDefaultTTL(50 * time.Millisecond)

	resp := lm.TryLock(&collab.LockRequest{
		AgentID:    "agent-1",
		ResourceID: "file:/tmp/expire.txt",
		LockType:   collab.LockWrite,
	})
	if !resp.Granted {
		t.Fatal("failed to get lock")
	}

	time.Sleep(150 * time.Millisecond)

	lease := lm.GetLease("file:/tmp/expire.txt")
	if lease != nil {
		t.Error("lease should have expired")
	}
}

func TestLockManagerListLeases(t *testing.T) {
	lm := collab.NewLockManager()
	lm.TryLock(&collab.LockRequest{AgentID: "a1", ResourceID: "r1", LockType: collab.LockWrite})
	lm.TryLock(&collab.LockRequest{AgentID: "a2", ResourceID: "r2", LockType: collab.LockWrite})

	leases := lm.ListLeases()
	if len(leases) != 2 {
		t.Errorf("expected 2 leases, got %d", len(leases))
	}
}

func TestLockManagerReleaseNotFound(t *testing.T) {
	lm := collab.NewLockManager()
	err := lm.Release("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent lease")
	}
}

func TestLockManagerConcurrent(t *testing.T) {
	lm := collab.NewLockManager()
	var wg sync.WaitGroup
	concurrency := 20

	granted := make([]bool, concurrency)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := lm.TryLock(&collab.LockRequest{
				AgentID:    fmt.Sprintf("agent-%d", id),
				ResourceID: "concurrent-resource",
				LockType:   collab.LockWrite,
				Priority:   id,
			})
			granted[id] = resp.Granted
		}(i)
	}
	wg.Wait()

	grantedCount := 0
	for _, g := range granted {
		if g {
			grantedCount++
		}
	}
	if grantedCount != 1 {
		t.Errorf("exactly 1 should get lock, got %d", grantedCount)
	}
}

func TestLockManagerLockLeaseTTL(t *testing.T) {
	lease := &collab.LockLease{
		ExpiresAt: time.Now().Add(50 * time.Millisecond),
	}
	if lease.IsExpired() {
		t.Error("should not be expired yet")
	}
	if lease.TTL() <= 0 {
		t.Error("should have positive TTL")
	}

	time.Sleep(100 * time.Millisecond)
	if !lease.IsExpired() {
		t.Error("should be expired")
	}
	if lease.TTL() != 0 {
		t.Error("expired lease should have 0 TTL")
	}
}

// ── Arbitrator Tests ──

func TestArbitratorPriorityPreempt(t *testing.T) {
	arb := collab.NewConflictArbitrator(collab.ArbPriorityPreempt)

	decision := arb.Arbitrate([]collab.AgentPriority{
		{AgentID: "low", Priority: 1, Role: "executor"},
		{AgentID: "high", Priority: 10, Role: "executor"},
	})

	if decision == nil {
		t.Fatal("expected decision")
	}
	if decision.WinnerID != "high" {
		t.Errorf("expected high to win, got %s", decision.WinnerID)
	}
	if decision.LoserID != "low" {
		t.Errorf("expected low to lose, got %s", decision.LoserID)
	}
	if decision.Action != "yield" {
		t.Errorf("expected yield, got %s", decision.Action)
	}
}

func TestArbitratorAllStrategies(t *testing.T) {
	tests := []struct {
		strategy       collab.ArbitrationStrategy
		expectedAction string
	}{
		{collab.ArbNegotiate, "negotiate"},
		{collab.ArbHumanEscalate, "escalate"},
		{collab.ArbForceSuspend, "suspend"},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			arb := collab.NewConflictArbitrator(tt.strategy)
			decision := arb.Arbitrate([]collab.AgentPriority{
				{AgentID: "a1", Priority: 5},
				{AgentID: "a2", Priority: 3},
			})
			if decision == nil {
				t.Fatal("expected decision")
			}
			if decision.Action != tt.expectedAction {
				t.Errorf("expected action %s, got %s", tt.expectedAction, decision.Action)
			}
		})
	}
}

func TestArbitratorResolveWith(t *testing.T) {
	arb := collab.NewConflictArbitrator(collab.ArbPriorityPreempt)

	decision := arb.ResolveWith(collab.ArbHumanEscalate, []collab.AgentPriority{
		{AgentID: "a1", Priority: 5},
		{AgentID: "a2", Priority: 3},
	})
	if decision.Action != "escalate" {
		t.Errorf("expected escalate, got %s", decision.Action)
	}

	// 原始策略应恢复
	if arb.Strategy() != collab.ArbPriorityPreempt {
		t.Error("original strategy should be restored")
	}
}

func TestArbitratorCollisionTracking(t *testing.T) {
	arb := collab.NewConflictArbitrator(collab.ArbPriorityPreempt)

	arb.ReportCollision(collab.CollisionReport{
		ResourceID: "file:/tmp/x.txt",
		Agents:     []string{"a1", "a2"},
	})
	arb.ReportCollision(collab.CollisionReport{
		ResourceID: "file:/tmp/y.txt",
		Agents:     []string{"a2", "a3"},
	})

	if arb.CollisionCount() != 2 {
		t.Errorf("expected 2, got %d", arb.CollisionCount())
	}

	recent := arb.RecentCollisions(1)
	if len(recent) != 1 {
		t.Errorf("expected 1 recent, got %d", len(recent))
	}
}

func TestArbitratorSingleAgent(t *testing.T) {
	arb := collab.NewConflictArbitrator(collab.ArbPriorityPreempt)
	decision := arb.Arbitrate([]collab.AgentPriority{
		{AgentID: "alone", Priority: 1},
	})
	if decision != nil {
		t.Error("should return nil for single agent")
	}
}

func TestLockResponseTypes(t *testing.T) {
	// Verify utility constants
	if collab.LockRead != "read" {
		t.Error("LockRead mismatch")
	}
	if collab.LockWrite != "write" {
		t.Error("LockWrite mismatch")
	}
	if collab.TopicLockRequest != "lock.request" {
		t.Error("TopicLockRequest mismatch")
	}
	if collab.TopicRolePromoted != "role.promoted" {
		t.Error("TopicRolePromoted mismatch")
	}
}

func TestArbitratorSetStrategy(t *testing.T) {
	arb := collab.NewConflictArbitrator(collab.ArbPriorityPreempt)
	arb.SetStrategy(collab.ArbForceSuspend)
	if arb.Strategy() != collab.ArbForceSuspend {
		t.Errorf("strategy not updated, got %s", arb.Strategy())
	}
}
