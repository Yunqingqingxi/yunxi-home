package notifier

import (
	"fmt"
	"sync"
	"time"
)

// Throttler 通知节流器，防止短时间内重复发送通知
type Throttler struct {
	mu       sync.RWMutex
	lastSent map[string]time.Time
}

// NewThrottler 创建节流器
func NewThrottler() *Throttler {
	return &Throttler{
		lastSent: make(map[string]time.Time),
	}

}

// Allow 检查是否允许发送通知
// domain: 域名标识
// throttleMinutes: 节流时长（分钟）
func (t *Throttler) Allow(domain string, throttleMinutes int) bool {
	t.mu.RLock()
	last, ok := t.lastSent[domain]
	t.mu.RUnlock()

	if !ok {
		return true
	}

	return time.Since(last) >= time.Duration(throttleMinutes)*time.Minute
}

// Mark 标记已发送通知
func (t *Throttler) Mark(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastSent[domain] = time.Now()
}

// AllowAndMark 检查并标记（原子操作）
func (t *Throttler) AllowAndMark(domain string, throttleMinutes int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	last, ok := t.lastSent[domain]
	if ok && time.Since(last) < time.Duration(throttleMinutes)*time.Minute {
		return false
	}

	t.lastSent[domain] = time.Now()
	return true
}

// Reset 重置指定域名的节流状态
func (t *Throttler) Reset(domain string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.lastSent, domain)
}

// SinceLast 获取距离上次通知的时间
func (t *Throttler) SinceLast(domain string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	last, ok := t.lastSent[domain]
	if !ok {
		return "从未发送"
	}
	return fmt.Sprintf("%.1f 分钟前", time.Since(last).Minutes())
}
