package collab

import (
	"fmt"

	"sort"
	"sync"
	"time"
)

// LockManager 细粒度资源锁管理器（in-process 实现）
type LockManager struct {
	mu           sync.Mutex
	leases       map[string]*LockLease      // resourceID → 当前租约
	waitQueues   map[string][]*LockRequest  // resourceID → 等待队列
	leaseCounter int64                      // 租约 ID 计数器
	defaultTTL   time.Duration              // 默认锁超时
	heartbeatTTL time.Duration              // 心跳间隔
}

// NewLockManager 创建锁管理器
func NewLockManager() *LockManager {
	lm := &LockManager{
		leases:       make(map[string]*LockLease),
		waitQueues:   make(map[string][]*LockRequest),
		defaultTTL:   30 * time.Second,
		heartbeatTTL: 10 * time.Second,
	}
	// 启动后台清理过期租约
	go lm.cleanupLoop()
	return lm
}

// SetDefaultTTL 设置默认锁超时
func (lm *LockManager) SetDefaultTTL(ttl time.Duration) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.defaultTTL = ttl
	lm.heartbeatTTL = ttl / 3
}

// TryLock 非阻塞尝试获取锁
func (lm *LockManager) TryLock(req *LockRequest) *LockResponse {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	return lm.tryLockLocked(req)
}

// Lock 阻塞获取锁（排队等待）
func (lm *LockManager) Lock(req *LockRequest) *LockResponse {
	// 先尝试非阻塞
	resp := lm.TryLock(req)
	if resp != nil && resp.Granted {
		return resp
	}

	// 加入等待队列
	lm.mu.Lock()
	lm.enqueueWait(req)
	lm.mu.Unlock()

	// 轮询等待
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(req.TTL)
	if req.TTL <= 0 {
		deadline = time.After(lm.defaultTTL * 3) // 3x TTL 作为等待上限
	}

	for {
		select {
		case <-ticker.C:
			resp := lm.TryLock(req)
			if resp != nil && resp.Granted {
				return resp
			}
		case <-deadline:
			return &LockResponse{
				Granted: false,
				Message: "等待锁超时",
			}
		}
	}
}

// Release 释放锁
func (lm *LockManager) Release(leaseID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for resourceID, lease := range lm.leases {
		if lease.LeaseID == leaseID {
			delete(lm.leases, resourceID)
			log.Debug("lock released", "lease", leaseID, "resource", resourceID)
			// 从等待队列中唤醒下一个
			lm.wakeNext(resourceID)
			return nil
		}
	}
	return fmt.Errorf("lease not found: %s", leaseID)
}

// Renew 续期租约
func (lm *LockManager) Renew(leaseID string, extend time.Duration) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for _, lease := range lm.leases {
		if lease.LeaseID == leaseID {
			if lease.IsExpired() {
				return fmt.Errorf("lease expired: %s", leaseID)
			}
			lease.ExpiresAt = time.Now().Add(extend)
			lease.RenewedAt = time.Now()
			log.Debug("lock renewed", "lease", leaseID, "expires", lease.ExpiresAt)
			return nil
		}
	}
	return fmt.Errorf("lease not found: %s", leaseID)
}

// GetLease 查询锁状态
func (lm *LockManager) GetLease(resourceID string) *LockLease {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lease, ok := lm.leases[resourceID]
	if !ok {
		return nil
	}
	if lease.IsExpired() {
		delete(lm.leases, resourceID)
		lm.wakeNext(resourceID)
		return nil
	}
	return lease
}

// ListLeases 列出所有活跃租约
func (lm *LockManager) ListLeases() []*LockLease {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	result := make([]*LockLease, 0, len(lm.leases))
	for _, lease := range lm.leases {
		if !lease.IsExpired() {
			result = append(result, lease)
		}
	}
	return result
}

// ── 内部方法 ──

func (lm *LockManager) tryLockLocked(req *LockRequest) *LockResponse {
	// 清理过期租约
	if existing, ok := lm.leases[req.ResourceID]; ok {
		if existing.IsExpired() {
			delete(lm.leases, req.ResourceID)
		} else if req.LockType == LockWrite || existing.LockType == LockWrite {
			// 写锁独占 / 已有写锁存在
			return &LockResponse{
				Granted:   false,
				OwnerID:   existing.AgentID,
				Message:   fmt.Sprintf("资源 %s 已被 %s 持有", req.ResourceID, existing.AgentID),
				RetryAfter: time.Until(existing.ExpiresAt),
			}
		}
		// 读锁共享：多个读锁可以共存
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = lm.defaultTTL
	}

	lease := &LockLease{
		LeaseID:    lm.genLeaseID(),
		AgentID:    req.AgentID,
		ResourceID: req.ResourceID,
		LockType:   req.LockType,
		Priority:   req.Priority,
		GrantedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(ttl),
		RenewedAt:  time.Now(),
	}
	lm.leases[req.ResourceID] = lease

	log.Debug("lock granted",
		"resource", req.ResourceID,
		"agent", req.AgentID,
		"type", req.LockType,
		"lease", lease.LeaseID,
	)

	return &LockResponse{
		Granted: true,
		LeaseID: lease.LeaseID,
	}
}

func (lm *LockManager) enqueueWait(req *LockRequest) {
	lm.waitQueues[req.ResourceID] = append(lm.waitQueues[req.ResourceID], req)

	// 优先级排序（高优先级在前）+ FIFO 作为次要排序
	queue := lm.waitQueues[req.ResourceID]
	sort.SliceStable(queue, func(i, j int) bool {
		return queue[i].Priority > queue[j].Priority
	})
	lm.waitQueues[req.ResourceID] = queue
}

func (lm *LockManager) wakeNext(resourceID string) {
	queue, ok := lm.waitQueues[resourceID]
	if !ok || len(queue) == 0 {
		return
	}
	// 下一个等待者在 Lock() 方法轮询中会自动获取
	// 这里只做清理：如果等待队列不为空，不删除队列
}

func (lm *LockManager) cleanupLoop() {
	ticker := time.NewTicker(lm.heartbeatTTL)
	defer ticker.Stop()
	for range ticker.C {
		lm.cleanup()
	}
}

func (lm *LockManager) cleanup() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	for resourceID, lease := range lm.leases {
		if lease.IsExpired() {
			delete(lm.leases, resourceID)
			log.Debug("lock expired", "resource", resourceID, "lease", lease.LeaseID)
			lm.wakeNext(resourceID)
		}
	}
}

func (lm *LockManager) genLeaseID() string {
	lm.leaseCounter++
	return fmt.Sprintf("lease_%d", lm.leaseCounter)
}
