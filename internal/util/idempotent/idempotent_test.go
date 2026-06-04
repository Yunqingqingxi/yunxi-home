package idempotent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRegistry_DeduplicatesConcurrent(t *testing.T) {
	r := NewRegistry()
	key := Key("test-op", map[string]string{"domain": "example.com"})

	var callCount int
	var mu sync.Mutex

	fn := func() (interface{}, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return "ok", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := r.Do(key, 5*time.Second, fn)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != "ok" {
				t.Errorf("unexpected result: %v", result)
			}
		}()
	}
	wg.Wait()

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRegistry_ErrorPropagation(t *testing.T) {
	r := NewRegistry()
	key := Key("error-op", nil)

	expectedErr := errors.New("operation failed")
	result, err := r.Do(key, 5*time.Second, func() (interface{}, error) {
		return nil, expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result on error")
	}
}

func TestKey_Deterministic(t *testing.T) {
	k1 := Key("op", map[string]string{"a": "1"})
	k2 := Key("op", map[string]string{"a": "1"})
	k3 := Key("op", map[string]string{"a": "2"})

	if k1 != k2 {
		t.Errorf("same inputs should produce same key: %q vs %q", k1, k2)
	}
	if k1 == k3 {
		t.Errorf("different inputs should produce different keys")
	}
}

func TestUndoStack_ReverseAll(t *testing.T) {
	us := NewUndoStack()
	var order []string

	us.Push("step1", func() error { order = append(order, "undo1"); return nil })
	us.Push("step2", func() error { order = append(order, "undo2"); return nil })
	us.Push("step3", func() error { order = append(order, "undo3"); return nil })

	err := us.UndoAll(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(order) != 3 || order[0] != "undo3" || order[1] != "undo2" || order[2] != "undo1" {
		t.Errorf("wrong undo order: %v (expected [undo3 undo2 undo1])", order)
	}

	if us.Len() != 0 {
		t.Errorf("stack should be empty after UndoAll, got %d", us.Len())
	}
}

func TestUndoStack_ContinuesOnError(t *testing.T) {
	us := NewUndoStack()
	var count int

	us.Push("fail", func() error { return errors.New("fail") })
	us.Push("ok", func() error { count++; return nil })

	err := us.UndoAll(context.Background())
	if err == nil {
		t.Error("expected error from failed undo")
	}
	if count != 1 {
		t.Errorf("second undo should still run, got count=%d", count)
	}
}

func TestCache_SetGet(t *testing.T) {
	c := NewCache()
	c.Set("key1", "value1", nil, 5*time.Second)

	result, err, ok := c.Get("key1")
	if !ok {
		t.Error("expected cache hit")
	}
	if result != "value1" {
		t.Errorf("expected 'value1', got %v", result)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	_, _, ok = c.Get("missing")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := NewCache()
	c.Set("key1", "value1", nil, 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	_, _, ok := c.Get("key1")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}
