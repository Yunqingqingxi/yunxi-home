package config

import (
	"context"
	"errors"
	"testing"
)

func TestRollbackStore_SaveAndLatest(t *testing.T) {
	rs := NewRollbackStore()

	rs.Save("ai", map[string]string{"model": "deepseek"})
	rs.Save("ai", map[string]string{"model": "qwen"})

	latest, ok := rs.Latest("ai")
	if !ok {
		t.Fatal("expected latest snapshot")
	}
	if latest != `{"model":"qwen"}` {
		t.Errorf("unexpected latest: %s", latest)
	}
}

func TestRollbackStore_MaxSnapshots(t *testing.T) {
	rs := NewRollbackStore()

	for i := 0; i < 10; i++ {
		rs.Save("test", map[string]int{"version": i})
	}

	entries := rs.List("test")
	if len(entries) > 5 {
		t.Errorf("expected max 5 entries, got %d", len(entries))
	}
}

func TestSafeChange_RollbackOnValidationFailure(t *testing.T) {
	rs := NewRollbackStore()
	state := map[string]string{"status": "before"}
	snapshots := map[string]string{}
	current := "before"

	snapshotFn := func() error {
		snapshots["saved"] = current
		return nil
	}
	applyFn := func() error {
		current = "after"
		return nil
	}
	validateFn := func(ctx context.Context) error {
		return errors.New("validation failed")
	}
	rollbackFn := func() error {
		current = "before"
		return nil
	}

	err := SafeChange("test", rs, state, snapshotFn, applyFn, validateFn, rollbackFn, context.Background())
	if err == nil {
		t.Error("expected validation error")
	}
	if current != "before" {
		t.Errorf("expected rollback to 'before', got '%s'", current)
	}
}

func TestSafeChange_Success(t *testing.T) {
	rs := NewRollbackStore()
	state := map[string]string{"status": "before"}
	current := "before"

	applyFn := func() error { current = "after"; return nil }
	validateFn := func(ctx context.Context) error { return nil }
	rollbackFn := func() error { return nil }
	snapshotFn := func() error { return nil }

	err := SafeChange("test", rs, state, snapshotFn, applyFn, validateFn, rollbackFn, context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if current != "after" {
		t.Errorf("expected 'after', got '%s'", current)
	}
}

func TestRollbackStore_Clear(t *testing.T) {
	rs := NewRollbackStore()
	rs.Save("section", "data")
	rs.Clear("section")

	_, ok := rs.Latest("section")
	if ok {
		t.Error("expected no snapshot after clear")
	}
}
