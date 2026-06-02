package test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/resilience"
)

// ── Circuit Breaker Tests ──

func TestCircuitBreakerClosedToOpen(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:      "test-cb",
		Threshold: 3,
		Cooldown:  100 * time.Millisecond,
	})

	failErr := errors.New("always fail")
	for i := 0; i < 3; i++ {
		err := cb.Call(func() error { return failErr })
		if !errors.Is(err, failErr) {
			t.Fatalf("attempt %d: expected failErr", i+1)
		}
	}

	if cb.State() != "open" {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// 后续调用应返回 ErrCircuitOpen
	err := cb.Call(func() error { return nil })
	if !errors.Is(err, resilience.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerRecoverAfterCooldown(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:        "test-cb",
		Threshold:   2,
		Cooldown:    50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	failErr := errors.New("fail")
	cb.Call(func() error { return failErr })
	cb.Call(func() error { return failErr })

	if cb.State() != "open" {
		t.Fatal("expected open")
	}

	time.Sleep(100 * time.Millisecond)

	err := cb.Call(func() error { return nil })
	if err != nil {
		t.Fatalf("expected success in half-open: %v", err)
	}
	if cb.State() != "closed" {
		t.Fatalf("expected closed after recovery, got %s", cb.State())
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:        "test-cb",
		Threshold:   2,
		Cooldown:    50 * time.Millisecond,
		HalfOpenMax: 1,
	})

	failErr := errors.New("fail")
	cb.Call(func() error { return failErr })
	cb.Call(func() error { return failErr })

	time.Sleep(100 * time.Millisecond)

	// 试探失败
	err := cb.Call(func() error { return failErr })
	if !errors.Is(err, failErr) {
		t.Fatalf("expected fail")
	}
	if cb.State() != "open" {
		t.Fatalf("expected re-open, got %s", cb.State())
	}
}

func TestCircuitBreakerSuccessResets(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:      "test-cb",
		Threshold: 5,
		Cooldown:  1 * time.Second,
	})

	failErr := errors.New("fail")
	for i := 0; i < 4; i++ {
		cb.Call(func() error { return failErr })
	}
	// 一次成功重置计数
	cb.Call(func() error { return nil })
	if cb.State() != "closed" {
		t.Errorf("expected closed, got %s", cb.State())
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:      "test-cb",
		Threshold: 2,
		Cooldown:  1 * time.Second,
	})

	failErr := errors.New("fail")
	cb.Call(func() error { return failErr })
	cb.Call(func() error { return failErr })
	if cb.State() != "open" {
		t.Fatal("expected open")
	}

	cb.Reset()
	if cb.State() != "closed" {
		t.Fatal("expected closed after reset")
	}

	stats := cb.Stats()
	if stats.State != "closed" || stats.Failures != 0 {
		t.Error("stats should be reset")
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:      "stats-cb",
		Threshold: 10,
	})

	cb.Call(func() error { return errors.New("fail") })
	stats := cb.Stats()
	if stats.Name != "stats-cb" {
		t.Errorf("wrong name: %s", stats.Name)
	}
	if stats.Failures != 1 {
		t.Errorf("expected 1 failure, got %d", stats.Failures)
	}
}

// ── Retry Tests ──

func TestIsRetryable(t *testing.T) {
	if !resilience.IsRetryable(resilience.ErrTimeout) {
		t.Error("timeout should be retryable")
	}
	if !resilience.IsRetryable(resilience.ErrNetwork) {
		t.Error("network should be retryable")
	}
	if !resilience.IsRetryable(resilience.ErrRateLimit) {
		t.Error("rate limit should be retryable")
	}
	if resilience.IsRetryable(resilience.ErrInvalidArgs) {
		t.Error("invalid args should NOT be retryable")
	}
	if resilience.IsRetryable(resilience.ErrPermission) {
		t.Error("permission should NOT be retryable")
	}
	if resilience.IsRetryable(nil) {
		t.Error("nil should NOT be retryable")
	}
}

func TestIsFatal(t *testing.T) {
	if !resilience.IsFatal(resilience.ErrInvalidArgs) {
		t.Error("invalid args should be fatal")
	}
	if !resilience.IsFatal(resilience.ErrPermission) {
		t.Error("permission should be fatal")
	}
	if !resilience.IsFatal(resilience.ErrQuotaExceed) {
		t.Error("quota exceeded should be fatal")
	}
	if resilience.IsFatal(resilience.ErrTimeout) {
		t.Error("timeout should NOT be fatal")
	}
}

func TestRetryDoSuccess(t *testing.T) {
	policy := resilience.RetryPolicy{
		MaxRetries: 2,
		Backoff:    10 * time.Millisecond,
	}
	count := 0
	err := resilience.Do(t.Context(), policy, func() error {
		count++
		if count < 2 {
			return resilience.ErrNetwork
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 attempts, got %d", count)
	}
}

func TestRetryDoExhausted(t *testing.T) {
	policy := resilience.RetryPolicy{
		MaxRetries: 2,
		Backoff:    10 * time.Millisecond,
	}
	count := 0
	err := resilience.Do(t.Context(), policy, func() error {
		count++
		return resilience.ErrNetwork
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if count != 3 { // 1 initial + 2 retries
		t.Errorf("expected 3 attempts, got %d", count)
	}
}

func TestRetryDoFatalNoRetry(t *testing.T) {
	policy := resilience.RetryPolicy{
		MaxRetries: 5,
		Backoff:    10 * time.Millisecond,
	}
	count := 0
	err := resilience.Do(t.Context(), policy, func() error {
		count++
		return resilience.ErrInvalidArgs
	})
	if !errors.Is(err, resilience.ErrInvalidArgs) {
		t.Fatalf("expected ErrInvalidArgs, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}
}

func TestRetryDoWithResult(t *testing.T) {
	policy := resilience.RetryPolicy{
		MaxRetries: 2,
		Backoff:    10 * time.Millisecond,
	}
	result, err := resilience.DoWithResult(t.Context(), policy, func() (string, error) {
		return "success", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got '%s'", result)
	}
}

func TestRetryNoRetry(t *testing.T) {
	policy := resilience.NoRetry()
	if policy.MaxRetries != 0 {
		t.Errorf("NoRetry should have 0 retries")
	}
	count := 0
	err := resilience.Do(t.Context(), policy, func() error {
		count++
		return resilience.ErrNetwork
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if count != 1 {
		t.Errorf("expected 1 attempt, got %d", count)
	}
}

func TestRetryConfigForTool(t *testing.T) {
	tests := []struct {
		risk     string
		maxRetry int
	}{
		{"readonly", 2},
		{"mutation", 1},
		{"dangerous", 0},
	}
	for _, tt := range tests {
		policy := resilience.RetryConfigForTool(tt.risk, "")
		if policy.MaxRetries != tt.maxRetry {
			t.Errorf("risk=%s: expected %d retries, got %d", tt.risk, tt.maxRetry, policy.MaxRetries)
		}
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	p := resilience.DefaultRetryPolicy()
	if p.MaxRetries != 2 {
		t.Errorf("expected MaxRetries=2, got %d", p.MaxRetries)
	}
	if p.Backoff != 1*time.Second {
		t.Errorf("expected Backoff=1s, got %v", p.Backoff)
	}
}

func TestCircuitBreakerConcurrent(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:      "concurrent-cb",
		Threshold: 20,
		Cooldown:  50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	concurrency := 30
	successCount := 0

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := cb.Call(func() error {
				time.Sleep(1 * time.Millisecond)
				return nil
			})
			if err == nil {
				successCount++
			}
		}()
	}
	wg.Wait()

	if successCount != concurrency {
		t.Errorf("expected %d successes, got %d", concurrency, successCount)
	}
}
