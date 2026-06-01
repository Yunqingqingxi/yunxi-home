package todo

import (
	"sync"
	"time"
)

// Manager 管理所有会话的 Todo 列表。
// 线程安全，可被 SSE goroutine 和工具 handler 并发访问。
type Manager struct {
	mu    sync.RWMutex
	lists map[string]*List // sessionID → List
}

// NewManager 创建 TodoManager
func NewManager() *Manager {
	return &Manager{lists: make(map[string]*List)}
}

// Update 全量替换指定会话的 Todo 列表。
// 返回更新后的列表，便于 SSE 推送。
func (m *Manager) Update(sessionID string, items []Item) *List {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 分配 ID（如果 item.ID == 0）
	for i := range items {
		if items[i].ID == 0 {
			items[i].ID = i + 1
		}
	}

	lst := &List{
		SessionID: sessionID,
		Items:     items,
		UpdatedAt: time.Now(),
	}
	m.lists[sessionID] = lst
	return lst
}

// Get 获取指定会话的 Todo 列表
func (m *Manager) Get(sessionID string) *List {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lists[sessionID]
}

// Delete 删除指定会话的 Todo 列表（会话结束时调用）
func (m *Manager) Delete(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.lists, sessionID)
}
