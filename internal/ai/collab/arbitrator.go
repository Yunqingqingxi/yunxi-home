package collab

import (
	"fmt"

	"sort"
	"sync"
	"time"
)

// ConflictArbitrator 冲突仲裁器
type ConflictArbitrator struct {
	mu             sync.Mutex
	strategy       ArbitrationStrategy
	negotiateTimeout time.Duration // 协商超时
	collisions     []CollisionReport
	maxCollisions  int
}

// NewConflictArbitrator 创建仲裁器
func NewConflictArbitrator(strategy ArbitrationStrategy) *ConflictArbitrator {
	return &ConflictArbitrator{
		strategy:         strategy,
		negotiateTimeout: 10 * time.Second,
		collisions:       make([]CollisionReport, 0, 64),
		maxCollisions:    200,
	}
}

// SetStrategy 设置仲裁策略
func (a *ConflictArbitrator) SetStrategy(s ArbitrationStrategy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.strategy = s
	log.Info("arbitration strategy changed", "strategy", s)
}

// Strategy 当前策略
func (a *ConflictArbitrator) Strategy() ArbitrationStrategy {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.strategy
}

// ReportCollision 报告资源冲突
func (a *ConflictArbitrator) ReportCollision(report CollisionReport) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.collisions = append(a.collisions, report)
	if len(a.collisions) > a.maxCollisions {
		a.collisions = a.collisions[len(a.collisions)-a.maxCollisions:]
	}
	log.Warn("collision reported",
		"resource", report.ResourceID,
		"agents", report.Agents,
		"details", report.Details,
	)
}

// Arbitrate 仲裁冲突：给定冲突的 Agent 及其优先级，返回仲裁决定
func (a *ConflictArbitrator) Arbitrate(agents []AgentPriority) *ArbitrationDecision {
	if len(agents) < 2 {
		return nil
	}

	// 按优先级降序排列
	sorted := make([]AgentPriority, len(agents))
	copy(sorted, agents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	winner := sorted[0]
	loser := sorted[len(sorted)-1]

	a.mu.Lock()
	strategy := a.strategy
	a.mu.Unlock()

	decision := &ArbitrationDecision{
		WinnerID:  winner.AgentID,
		LoserID:   loser.AgentID,
		DecidedAt: time.Now(),
	}

	switch strategy {
	case ArbPriorityPreempt:
		decision.Strategy = ArbPriorityPreempt
		decision.Action = "yield"
		decision.Reason = fmt.Sprintf(
			"高优先级 Agent %s (优先级 %d) 抢占资源，低优先级 %s (优先级 %d) 让出",
			winner.AgentID, winner.Priority, loser.AgentID, loser.Priority,
		)
		log.Info("arbitration: priority preempt", "winner", winner.AgentID, "loser", loser.AgentID)

	case ArbNegotiate:
		decision.Strategy = ArbNegotiate
		decision.Action = "negotiate"
		decision.RetryAfter = a.negotiateTimeout
		decision.Reason = fmt.Sprintf(
			"请 Agent %s 和 %s 在 %v 内自行协商解决冲突",
			winner.AgentID, loser.AgentID, a.negotiateTimeout,
		)
		log.Info("arbitration: negotiate", "agents", []string{winner.AgentID, loser.AgentID})

	case ArbHumanEscalate:
		decision.Strategy = ArbHumanEscalate
		decision.Action = "escalate"
		decision.Reason = "冲突已上报用户，等待人工决策"
		log.Warn("arbitration: escalated to human", "winner", winner.AgentID, "loser", loser.AgentID)

	case ArbForceSuspend:
		decision.Strategy = ArbForceSuspend
		decision.Action = "suspend"
		decision.Reason = fmt.Sprintf("低优先级 Agent %s 已被强制暂停", loser.AgentID)
		log.Warn("arbitration: forced suspension", "loser", loser.AgentID)

	default:
		// 默认：优先级抢占
		decision.Strategy = ArbPriorityPreempt
		decision.Action = "yield"
		decision.Reason = fmt.Sprintf("默认仲裁：高优先级 Agent %s 获胜", winner.AgentID)
	}

	return decision
}

// ResolveWith 使用指定策略解决冲突（覆盖全局策略）
func (a *ConflictArbitrator) ResolveWith(strategy ArbitrationStrategy, agents []AgentPriority) *ArbitrationDecision {
	a.mu.Lock()
	prev := a.strategy
	a.strategy = strategy
	a.mu.Unlock()

	decision := a.Arbitrate(agents)

	a.mu.Lock()
	a.strategy = prev
	a.mu.Unlock()

	return decision
}

// RecentCollisions 返回最近的冲突报告
func (a *ConflictArbitrator) RecentCollisions(limit int) []CollisionReport {
	a.mu.Lock()
	defer a.mu.Unlock()
	if limit <= 0 || limit > len(a.collisions) {
		limit = len(a.collisions)
	}
	start := len(a.collisions) - limit
	result := make([]CollisionReport, limit)
	copy(result, a.collisions[start:])
	return result
}

// CollisionCount 返回冲突总数
func (a *ConflictArbitrator) CollisionCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.collisions)
}

// AgentPriority Agent 优先级信息
type AgentPriority struct {
	AgentID     string `json:"agent_id"`
	Priority    int    `json:"priority"`
	Role        string `json:"role"`        // executor / supervisor / manager
	SuccessRate float64 `json:"success_rate"` // 历史成功率 (0.0-1.0)
	LockHeld    bool   `json:"lock_held"`   // 是否已持有锁
}
