package notifier

import (
	"testing"
	"time"
)

func TestThrottlerAllowAndMark(t *testing.T) {
	th := NewThrottler()

	// 第一次应该允许
	if !th.AllowAndMark("example.com", 1) {
		t.Error("first request should be allowed")
	}

	// 短时间内第二次应被节流
	if th.AllowAndMark("example.com", 1) {
		t.Error("second request should be throttled")
	}

	// 不同域名应该允许
	if !th.AllowAndMark("other.com", 1) {
		t.Error("different domain should be allowed")
	}
}

func TestThrottlerAllow(t *testing.T) {
	th := NewThrottler()

	if !th.Allow("test.com", 60) {
		t.Error("first Allow should return true")
	}

	th.Mark("test.com")

	if th.Allow("test.com", 60) {
		t.Error("Allow after Mark within window should return false")
	}
}

func TestThrottlerReset(t *testing.T) {
	th := NewThrottler()

	th.Mark("test.com")
	if th.Allow("test.com", 60) {
		t.Error("should be throttled before reset")
	}

	th.Reset("test.com")
	if !th.Allow("test.com", 60) {
		t.Error("should be allowed after reset")
	}
}

func TestThrottlerSinceLast(t *testing.T) {
	th := NewThrottler()
	if s := th.SinceLast("nonexistent"); s != "从未发送" {
		t.Errorf("expected '从未发送', got %s", s)
	}
	th.Mark("test.com")
	time.Sleep(10 * time.Millisecond)
	if s := th.SinceLast("test.com"); s == "从未发送" {
		t.Error("should show time since last send")
	}
}

func TestManagerRegister(t *testing.T) {
	th := NewThrottler()
	mgr := NewManager(th)

	if mgr.Count() != 0 {
		t.Error("new manager should have 0 notifiers")
	}

	n := NewWebhookNotifier(WebhookConfig{Enabled: true, URL: "https://example.com/webhook"})
	mgr.Register(n)

	if mgr.Count() != 1 {
		t.Errorf("expected 1 notifier, got %d", mgr.Count())
	}

	// 禁用的通知器不应注册
	n2 := NewWebhookNotifier(WebhookConfig{Enabled: false})
	mgr.Register(n2)
	if mgr.Count() != 1 {
		t.Errorf("disabled notifier should not be registered, got %d", mgr.Count())
	}
}
