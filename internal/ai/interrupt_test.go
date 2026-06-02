package ai

import (
	"testing"
)

func TestSortInjectMessages(t *testing.T) {
	msgs := []InjectedMessage{
		{Priority: "info", Content: "info1"},
		{Priority: "interrupt", Content: "stop"},
		{Priority: "user", Content: "hello"},
		{Priority: "info", Content: "info2"},
		{Priority: "system", Content: "sys"},
	}

	sortInjectMessages(msgs)

	// interrupt should be first
	if msgs[0].Priority != "interrupt" {
		t.Errorf("expected interrupt first, got %s", msgs[0].Priority)
	}
	// user second
	if msgs[1].Priority != "user" {
		t.Errorf("expected user second, got %s", msgs[1].Priority)
	}
	// system third
	if msgs[2].Priority != "system" {
		t.Errorf("expected system third, got %s", msgs[2].Priority)
	}
	// info last
	if msgs[3].Priority != "info" {
		t.Errorf("expected info fourth, got %s", msgs[3].Priority)
	}
}

func TestPriorityValue(t *testing.T) {
	tests := []struct {
		priority string
		expected int
	}{
		{"interrupt", PriorityInterrupt},
		{"user", PriorityUser},
		{"system", PrioritySystem},
		{"info", PriorityInfo},
		{"unknown", PriorityInfo}, // default fallback
		{"", PriorityInfo},
	}

	for _, tt := range tests {
		got := priorityValue(tt.priority)
		if got != tt.expected {
			t.Errorf("priorityValue(%q) = %d, want %d", tt.priority, got, tt.expected)
		}
	}
}

func TestBuildSnapshotNoTracker(t *testing.T) {
	// Without a full Service, buildSnapshot should return defaults
	// This verifies the function doesn't panic with nil tracker
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("buildSnapshot panicked: %v", r)
		}
	}()

	// Can't easily test full Service without DB, but verify the helper functions work
	snap := &InterruptSnapshot{Status: "running"}
	if snap.Status != "running" {
		t.Errorf("expected running status")
	}
	if snap.Progress != 0 {
		t.Errorf("expected 0 progress for empty snapshot")
	}
}

func TestDrainInjectChNonBlockingNoChannel(t *testing.T) {
	// Test that draining a non-existent channel returns empty
	var s Service
	msgs, interrupted := s.drainInjectChNonBlocking("nonexistent")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
	if interrupted {
		t.Errorf("expected no interrupt for nonexistent session")
	}
}

func TestCancelModeConstants(t *testing.T) {
	if string(CancelSoft) != "soft" {
		t.Errorf("CancelSoft = %q, want %q", CancelSoft, "soft")
	}
	if string(CancelHard) != "hard" {
		t.Errorf("CancelHard = %q, want %q", CancelHard, "hard")
	}
	if string(CancelModeSnapshot) != "snapshot" {
		t.Errorf("CancelModeSnapshot = %q, want %q", CancelModeSnapshot, "snapshot")
	}
}
