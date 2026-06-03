package agent

import (

	"sync"
	"time"
)

// RoleRecord 角色记录
type RoleRecord struct {
	AgentID    string    `json:"agent_id"`
	Role       AgentRole `json:"role"`
	PromotedAt time.Time `json:"promoted_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"` // 零值 = 永不过期
	PromotedBy string    `json:"promoted_by"`
	Reason     string    `json:"reason"`
	History    []RoleChangeEntry `json:"history,omitempty"`
}

// RoleChangeEntry 角色变更条目
type RoleChangeEntry struct {
	FromRole AgentRole `json:"from_role"`
	ToRole   AgentRole `json:"to_role"`
	At       time.Time `json:"at"`
	Reason   string    `json:"reason"`
}

// RoleRegistry 角色注册表（in-memory + 可选 DB 持久化）
type RoleRegistry struct {
	mu      sync.RWMutex
	records map[string]*RoleRecord // agentID → record
	maxHistory int                  // 每个角色最大历史条目数
}

// NewRoleRegistry 创建角色注册表
func NewRoleRegistry() *RoleRegistry {
	return &RoleRegistry{
		records:    make(map[string]*RoleRecord),
		maxHistory: 20,
	}
}

// Register 注册 Agent 角色
func (r *RoleRegistry) Register(agentID string, role AgentRole, promotedBy string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.records[agentID]
	if !ok {
		r.records[agentID] = &RoleRecord{
			AgentID:    agentID,
			Role:       role,
			PromotedAt: time.Now(),
			PromotedBy: promotedBy,
			Reason:     reason,
			History:    make([]RoleChangeEntry, 0, r.maxHistory),
		}
		return
	}

	// 记录历史变更
	entry := RoleChangeEntry{
		FromRole: existing.Role,
		ToRole:   role,
		At:       time.Now(),
		Reason:   reason,
	}
	existing.History = append(existing.History, entry)
	if len(existing.History) > r.maxHistory {
		existing.History = existing.History[len(existing.History)-r.maxHistory:]
	}

	existing.Role = role
	existing.PromotedAt = time.Now()
	existing.PromotedBy = promotedBy
	existing.Reason = reason

	log.Info("role registered",
		"agent", agentID,
		"role", string(role),
		"promoted_by", promotedBy,
		"reason", reason,
	)
}

// SetRoleTTL 设置角色有效期（用于临时晋升）
func (r *RoleRegistry) SetRoleTTL(agentID string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.records[agentID]; ok && ttl > 0 {
		rec.ExpiresAt = time.Now().Add(ttl)
		log.Info("role TTL set", "agent", agentID, "ttl", ttl)
	}
}

// Get 获取角色记录
func (r *RoleRegistry) Get(agentID string) *RoleRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.records[agentID]
	if !ok {
		return &RoleRecord{AgentID: agentID, Role: RoleExecutor}
	}
	// 检查 TTL 过期
	if !rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt) {
		// 过期降级（需要写锁，这里返回降级后的信息）
		// 真正的降级在 cleanupLoop 中处理
		return &RoleRecord{
			AgentID:    agentID,
			Role:       RoleExecutor,
			PromotedBy: "ttl_expired",
			Reason:     "角色 TTL 已过期",
		}
	}
	return rec
}

// Revoke 撤销角色（降级回 EXECUTOR）
func (r *RoleRegistry) Revoke(agentID string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.records[agentID]; ok {
		rec.History = append(rec.History, RoleChangeEntry{
			FromRole: rec.Role,
			ToRole:   RoleExecutor,
			At:       time.Now(),
			Reason:   reason,
		})
		rec.Role = RoleExecutor
		rec.Reason = reason
		rec.ExpiresAt = time.Time{}
		log.Info("role revoked", "agent", agentID, "reason", reason)
	}
}

// ListByRole 列出指定角色的所有 Agent
func (r *RoleRegistry) ListByRole(role AgentRole) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var ids []string
	for id, rec := range r.records {
		if rec.Role == role {
			ids = append(ids, id)
		}
	}
	return ids
}

// HasActiveSupervisor 是否有活跃的监督者
func (r *RoleRegistry) HasActiveSupervisor() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rec := range r.records {
		if rec.Role == RoleSupervisor || rec.Role == RoleManager {
			if rec.ExpiresAt.IsZero() || time.Now().Before(rec.ExpiresAt) {
				return true
			}
		}
	}
	return false
}

// CleanupExpired 清理过期角色（由后台循环调用）
func (r *RoleRegistry) CleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for id, rec := range r.records {
		if !rec.ExpiresAt.IsZero() && now.After(rec.ExpiresAt) {
			rec.History = append(rec.History, RoleChangeEntry{
				FromRole: rec.Role,
				ToRole:   RoleExecutor,
				At:       now,
				Reason:   "ttl_expired_cleanup",
			})
			rec.Role = RoleExecutor
			rec.ExpiresAt = time.Time{}
			rec.Reason = "TTL 过期自动降级"
			log.Info("role expired, auto-demoted", "agent", id)
		}
	}
}

// StartCleanupLoop 启动后台清理循环
func (r *RoleRegistry) StartCleanupLoop(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			r.CleanupExpired()
		}
	}()
}

// All 返回所有角色记录（用于调试/监控）
func (r *RoleRegistry) All() []*RoleRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*RoleRecord, 0, len(r.records))
	for _, rec := range r.records {
		result = append(result, rec)
	}
	return result
}

// ── 晋升授权策略 ──

// PromotionPolicy 晋升策略
type PromotionPolicy struct {
	MinSuccessRate    float64 // 最低成功率（默认 0.9）
	MinTasksCompleted int     // 最少完成任务数（默认 5）
	AllowAutoApprove  bool    // 是否允许自动批准
	RequireNoExistingManager bool // 是否需要无现有管理者
}

// DefaultPromotionPolicy 默认晋升策略
func DefaultPromotionPolicy() PromotionPolicy {
	return PromotionPolicy{
		MinSuccessRate:          0.9,
		MinTasksCompleted:       5,
		AllowAutoApprove:        true,
		RequireNoExistingManager: true,
	}
}

// AuthorizePromotion 授权晋升请求
func AuthorizePromotion(meta *MetaReport, targetRole AgentRole, registry *RoleRegistry, policy PromotionPolicy) (approved bool, reason string) {
	// 基本信息检查
	if meta.SuccessRate < policy.MinSuccessRate {
		return false, "成功率不达标"
	}
	if meta.TaskCompleted < policy.MinTasksCompleted {
		return false, "完成任务数不足"
	}

	// 冲突过多不适合晋升
	if meta.ConflictCount > meta.TaskCompleted/2 {
		return false, "冲突次数过多"
	}

	// 目标角色检查
	switch targetRole {
	case RoleSupervisor:
		if policy.RequireNoExistingManager && registry.HasActiveSupervisor() {
			return false, "已有活跃管理者"
		}
	case RoleManager:
		// 管理角色需要更高的门槛
		if meta.SuccessRate < 0.95 {
			return false, "管理角色需要 95% 以上成功率"
		}
		if meta.TaskCompleted < 10 {
			return false, "管理角色需要至少完成 10 个任务"
		}
	}

	if policy.AllowAutoApprove {
		return true, "自动批准"
	}
	return false, "需要人工审批"
}
