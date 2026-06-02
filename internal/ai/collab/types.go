// Package collab 提供 Agent 间协作通信能力（消息总线、分布式锁、冲突仲裁）。
// 单机部署使用 in-process 实现，API 设计兼容分布式扩展。
package collab

import (
	"time"
)

// ── 消息主题 ──

const (
	TopicLockRequest    = "lock.request"
	TopicLockGranted    = "lock.granted"
	TopicLockDenied     = "lock.denied"
	TopicCollisionReport = "collision.report"
	TopicAgentNegotiate  = "agent.negotiate"
	TopicRolePromoted    = "role.promoted"
	TopicRoleRevoked     = "role.revoked"
	TopicAgentAnnounce   = "agent.announce"
)

// CollabMessage 协作消息
type CollabMessage struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	FromAgent string    `json:"from_agent"`
	ToAgent   string    `json:"to_agent,omitempty"` // 空 = 广播
	Payload   []byte    `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

// ── 锁相关类型 ──

// LockRequest 锁请求
type LockRequest struct {
	AgentID    string        `json:"agent_id"`
	ResourceID string        `json:"resource_id"`  // 资源标识（如文件路径、对象属性）
	LockType   LockType      `json:"lock_type"`    // 读锁/写锁
	Priority   int           `json:"priority"`     // 优先级（越高越优先）
	TTL        time.Duration `json:"ttl"`          // 锁超时（0=使用默认值）
	WaitQueue  bool          `json:"wait_queue"`   // 是否排队等待（true=Lock, false=TryLock）
}

// LockType 锁类型
type LockType string

const (
	LockRead  LockType = "read"  // 读锁（共享）
	LockWrite LockType = "write" // 写锁（独占）
)

// LockResponse 锁响应
type LockResponse struct {
	Granted    bool      `json:"granted"`
	LeaseID    string    `json:"lease_id,omitempty"`
	OwnerID    string    `json:"owner_id,omitempty"`    // 当被拒绝时：当前持有者
	RetryAfter time.Duration `json:"retry_after,omitempty"` // 当被拒绝时：建议重试时间
	Message    string    `json:"message,omitempty"`
}

// LockLease 锁租约
type LockLease struct {
	LeaseID    string    `json:"lease_id"`
	AgentID    string    `json:"agent_id"`
	ResourceID string    `json:"resource_id"`
	LockType   LockType  `json:"lock_type"`
	Priority   int       `json:"priority"`
	GrantedAt  time.Time `json:"granted_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	RenewedAt  time.Time `json:"renewed_at"`
}

// IsExpired 检查租约是否过期
func (l *LockLease) IsExpired() bool {
	return time.Now().After(l.ExpiresAt)
}

// TTL 返回剩余生存时间
func (l *LockLease) TTL() time.Duration {
	remaining := time.Until(l.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ── 仲裁相关类型 ──

// ArbitrationStrategy 仲裁策略
type ArbitrationStrategy string

const (
	ArbPriorityPreempt ArbitrationStrategy = "priority_preempt" // 优先级抢占
	ArbNegotiate       ArbitrationStrategy = "negotiate"        // 协商模式
	ArbHumanEscalate   ArbitrationStrategy = "human_escalate"   // 人工介入
	ArbForceSuspend    ArbitrationStrategy = "force_suspend"    // 强制暂停低优先级
)

// ArbitrationDecision 仲裁决定
type ArbitrationDecision struct {
	Strategy    ArbitrationStrategy `json:"strategy"`
	WinnerID    string              `json:"winner_id"`
	LoserID     string              `json:"loser_id"`
	Action      string              `json:"action"`  // yield / wait / suspend / escalate
	RetryAfter  time.Duration       `json:"retry_after,omitempty"`
	Reason      string              `json:"reason"`
	DecidedAt   time.Time           `json:"decided_at"`
}

// CollisionReport 冲突报告
type CollisionReport struct {
	ResourceID string   `json:"resource_id"`
	Agents     []string `json:"agents"` // 冲突的 Agent ID 列表
	Details    string   `json:"details"`
	ReportedAt time.Time `json:"reported_at"`
}

// ── 角色变更通知 ──

// RoleChangeNotice 角色变更通知
type RoleChangeNotice struct {
	AgentID     string `json:"agent_id"`
	OldRole     string `json:"old_role"`
	NewRole     string `json:"new_role"`
	PromotedBy  string `json:"promoted_by,omitempty"`
	Reason      string `json:"reason"`
	TTLSeconds  int    `json:"ttl_seconds"` // 0 = 永久
}

// ── Agent 能力广播 ──

// AgentAnnouncement Agent 上线/能力广播
type AgentAnnouncement struct {
	AgentID     string   `json:"agent_id"`
	Role        string   `json:"role"`
	Capabilities []string `json:"capabilities"` // 支持的能力标签
	ParentID    string   `json:"parent_id,omitempty"`
	OnlineAt    time.Time `json:"online_at"`
}
