// Package coordinator 提供跨会话资源协调机制。
// 防止多个 AI 会话同时操作同一资源时产生冲突。
package coordinator

import (
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

// LockMode 锁模式
type LockMode string

const (
	LockRead  LockMode = "read"
	LockWrite LockMode = "write"
)

// ResourceLock 资源锁
type ResourceLock struct {
	Path      string    `json:"path"`
	SessionID string    `json:"session_id"`
	Mode      LockMode  `json:"mode"`
	LockedAt  time.Time `json:"locked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartedAt time.Time `json:"started_at"`
	LastSeen  time.Time `json:"last_seen"`
	GoalID    string    `json:"goal_id,omitempty"`
}

// ResourceEvent 资源变更事件
type ResourceEvent struct {
	Type      string    `json:"type"` // "locked" | "unlocked" | "modified" | "conflict"
	Path      string    `json:"path"`
	SessionID string    `json:"session_id"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// Subscriber 资源事件订阅者
type Subscriber struct {
	SessionID string
	Ch        chan ResourceEvent
}

// Coordinator 跨会话资源协调器
type Coordinator struct {
	locks       map[string]*ResourceLock // path → lock
	sessions    map[string]*SessionInfo  // sessionID → info
	subscribers map[string][]*Subscriber // sessionID → subscribers
	mu          sync.RWMutex
	lockTimeout time.Duration
}

// New 创建协调器
func New(lockTimeout time.Duration) *Coordinator {
	if lockTimeout <= 0 {
		lockTimeout = 30 * time.Second
	}
	c := &Coordinator{
		locks:       make(map[string]*ResourceLock),
		sessions:    make(map[string]*SessionInfo),
		subscribers: make(map[string][]*Subscriber),
		lockTimeout: lockTimeout,
	}
	go c.cleanupLoop()
	return c
}

// ── 会话管理 ──────────────────────────────────────────

// RegisterSession 注册一个会话
func (c *Coordinator) RegisterSession(id, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if existing, ok := c.sessions[id]; ok {
		existing.LastSeen = now
		existing.Title = title
		return
	}
	c.sessions[id] = &SessionInfo{ID: id, Title: title, StartedAt: now, LastSeen: now}
	log.Info("session registered", "id", id, "title", title)
}

// UnregisterSession 注销会话（释放其所有锁）
func (c *Coordinator) UnregisterSession(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 释放该会话持有的所有锁
	for path, lock := range c.locks {
		if lock.SessionID == id {
			delete(c.locks, path)
			c.broadcast(ResourceEvent{
				Type:      "unlocked",
				Path:      path,
				SessionID: id,
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("会话 %s 已断开，资源 %s 锁已释放", id, path),
			})
		}
	}
	delete(c.sessions, id)
	delete(c.subscribers, id)
	log.Info("session unregistered", "id", id)
}

// Heartbeat 心跳（更新 lastSeen + 续约锁）
func (c *Coordinator) Heartbeat(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.sessions[sessionID]; ok {
		s.LastSeen = time.Now()
	}
	// 续约该会话持有的所有锁
	for _, lock := range c.locks {
		if lock.SessionID == sessionID {
			lock.ExpiresAt = time.Now().Add(c.lockTimeout)
		}
	}
}

// ListSessions 列出所有活跃会话
func (c *Coordinator) ListSessions() []SessionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]SessionInfo, 0, len(c.sessions))
	for _, s := range c.sessions {
		if time.Since(s.LastSeen) < 5*time.Minute {
			result = append(result, *s)
		}
	}
	return result
}

// ── 资源锁 ────────────────────────────────────────────

// Acquire 获取资源锁。返回错误表示冲突。
func (c *Coordinator) Acquire(sessionID, path string, mode LockMode) (*ResourceLock, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// 检查是否已有锁
	if existing, ok := c.locks[path]; ok {
		// 同一会话重入：允许升级锁（read→write）
		if existing.SessionID == sessionID {
			if mode == LockWrite {
				existing.Mode = LockWrite
				existing.ExpiresAt = now.Add(c.lockTimeout)
			}
			return existing, nil
		}

		// 不同会话冲突
		conflictMsg := fmt.Sprintf("资源 %s 正被会话 %s 以 %s 模式锁定", path, existing.SessionID, existing.Mode)
		if existing.Mode == LockWrite || mode == LockWrite {
			// 写锁或请求写锁 → 完全互斥
			c.broadcast(ResourceEvent{
				Type: "conflict", Path: path, SessionID: sessionID,
				Timestamp: now, Message: conflictMsg,
			})
			return nil, fmt.Errorf("%s", conflictMsg)
		}
		// 读锁 + 读锁 → 共享
	}

	lock := &ResourceLock{
		Path:      path,
		SessionID: sessionID,
		Mode:      mode,
		LockedAt:  now,
		ExpiresAt: now.Add(c.lockTimeout),
	}
	c.locks[path] = lock

	c.broadcast(ResourceEvent{
		Type: "locked", Path: path, SessionID: sessionID,
		Timestamp: now,
		Message:   fmt.Sprintf("会话 %s 获取了 %s 的 %s 锁", sessionID, path, mode),
	})

	log.Debug("lock acquired", "path", path, "session", sessionID, "mode", mode)
	return lock, nil
}

// Release 释放资源锁
func (c *Coordinator) Release(sessionID, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if lock, ok := c.locks[path]; ok && lock.SessionID == sessionID {
		delete(c.locks, path)
		c.broadcast(ResourceEvent{
			Type: "unlocked", Path: path, SessionID: sessionID,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("会话 %s 释放了 %s 的锁", sessionID, path),
		})
	}
}

// NotifyModification 通知其他会话：某资源已被修改
func (c *Coordinator) NotifyModification(sessionID, path, summary string) {
	c.broadcast(ResourceEvent{
		Type: "modified", Path: path, SessionID: sessionID,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("会话 %s 修改了 %s: %s", sessionID, path, summary),
	})
}

// CheckConflict 检查操作是否会冲突（不获取锁）
func (c *Coordinator) CheckConflict(sessionID, path string, mode LockMode) *ResourceLock {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if existing, ok := c.locks[path]; ok {
		if existing.SessionID != sessionID {
			if existing.Mode == LockWrite || mode == LockWrite {
				return existing
			}
		}
	}
	return nil
}

// GetLock 查询某资源的锁状态
func (c *Coordinator) GetLock(path string) *ResourceLock {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.locks[path]
}

// ListLocks 列出所有活跃锁
func (c *Coordinator) ListLocks() []ResourceLock {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ResourceLock, 0, len(c.locks))
	for _, l := range c.locks {
		result = append(result, *l)
	}
	return result
}

// ── 事件订阅 ──────────────────────────────────────────

// Subscribe 订阅资源事件
func (c *Coordinator) Subscribe(sessionID string, bufSize int) <-chan ResourceEvent {
	if bufSize <= 0 {
		bufSize = 32
	}
	ch := make(chan ResourceEvent, bufSize)
	sub := &Subscriber{SessionID: sessionID, Ch: ch}

	c.mu.Lock()
	c.subscribers[sessionID] = append(c.subscribers[sessionID], sub)
	c.mu.Unlock()

	return ch
}

// Unsubscribe 取消订阅
func (c *Coordinator) Unsubscribe(sessionID string, ch <-chan ResourceEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	subs := c.subscribers[sessionID]
	filtered := make([]*Subscriber, 0, len(subs))
	for _, sub := range subs {
		if sub.Ch != ch {
			filtered = append(filtered, sub)
		}
	}
	if len(filtered) == 0 {
		delete(c.subscribers, sessionID)
	} else {
		c.subscribers[sessionID] = filtered
	}
}

// broadcast 向所有其他会话广播事件
func (c *Coordinator) broadcast(ev ResourceEvent) {
	for sid, subs := range c.subscribers {
		if sid == ev.SessionID {
			continue // 不发给事件源头
		}
		for _, sub := range subs {
			select {
			case sub.Ch <- ev:
			default:
				// 订阅者消费太慢，丢弃
			}
		}
	}
}

// ── 后台清理 ──────────────────────────────────────────

func (c *Coordinator) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for path, lock := range c.locks {
			if now.After(lock.ExpiresAt) {
				delete(c.locks, path)
				log.Info("lock expired", "path", path, "session", lock.SessionID)
				c.broadcast(ResourceEvent{
					Type: "unlocked", Path: path, SessionID: lock.SessionID,
					Timestamp: now, Message: fmt.Sprintf("%s 的锁已过期自动释放", path),
				})
			}
		}
		// 清理过期会话
		for id, s := range c.sessions {
			if now.Sub(s.LastSeen) > 10*time.Minute {
				c.unregisterSessionLocked(id)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Coordinator) unregisterSessionLocked(id string) {
	for path, lock := range c.locks {
		if lock.SessionID == id {
			delete(c.locks, path)
		}
	}
	delete(c.sessions, id)
	delete(c.subscribers, id)
}

// ── 会话间消息 ────────────────────────────────────────

// InterSessionMsg 跨会话消息
type InterSessionMsg struct {
	From      string    `json:"from"`
	To        string    `json:"to"` // 空=广播
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SendMessage 发送跨会话消息
func (c *Coordinator) SendMessage(from, to, content string) {
	// 将消息作为特殊资源事件广播
	c.broadcast(ResourceEvent{
		Type:      "message",
		Path:      "", // 非资源路径
		SessionID: from,
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("[%s→%s] %s", from, to, content),
	})
}

// AcquireWithRetry 带重试的获取锁（等待释放）
func (c *Coordinator) AcquireWithRetry(sessionID, path string, mode LockMode, timeout time.Duration) (*ResourceLock, error) {
	deadline := time.Now().Add(timeout)
	for {
		lock, err := c.Acquire(sessionID, path, mode)
		if err == nil {
			return lock, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("获取锁超时 (%v): %w", timeout, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
