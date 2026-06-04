package notifier

import (
	"fmt"
	"sync"
	"time"
)

const (
	// throttleCleanupInterval 后台清理过期条目间隔
	throttleCleanupInterval = 10 * time.Minute
	// throttleMaxAge 条目最大保留时间（超过此时间的条目会被清理）
	throttleMaxAge = 24 * time.Hour
)

// Throttler 通知节流器，防止短时间内重复发送通知
type Throttler struct {
	mu       sync.RWMutex
	lastSent map[string]time.Time
	done     chan struct{}
	stopOnce sync.Once
}

// NewThrottler 创建节流器
func NewThrottler() *Throttler {
	t := &Throttler{
		lastSent: make(map[string]time.Time),
		done:     make(chan struct{}),
	}
	go t.cleanupLoop()
	return t
}

// Stop 停止后台清理 goroutine。
func (t *Throttler) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
	})
}

// cleanupLoop 定期清理超过 maxAge 的条目，防止内存泄漏
func (t *Throttler) cleanupLoop() {
	ticker := time.NewTicker(throttleCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			t.cleanStale()
		}
	}
}

// cleanStale 删除超过 maxAge 的条目
func (t *Throttler) cleanStale() {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-throttleMaxAge)
	for domain, last := range t.lastSent {
		if last.Before(cutoff) {
			delete(t.lastSent, domain)
		}
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
