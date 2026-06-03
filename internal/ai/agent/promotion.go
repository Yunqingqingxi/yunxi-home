package agent

import (
	"fmt"

	"time"
)

// PromotionEngine 晋升决策引擎
type PromotionEngine struct {
	registry *RoleRegistry
	policy   PromotionPolicy
	selfID   string
	meta     *Metacognition
}

// NewPromotionEngine 创建晋升引擎
func NewPromotionEngine(selfID string, meta *Metacognition, registry *RoleRegistry) *PromotionEngine {
	return &PromotionEngine{
		registry: registry,
		policy:   DefaultPromotionPolicy(),
		selfID:   selfID,
		meta:     meta,
	}
}

// SetPolicy 更新晋升策略
func (pe *PromotionEngine) SetPolicy(p PromotionPolicy) {
	pe.policy = p
}

// EvaluatePromotion 评估是否应该晋升
// 返回：(是否请求晋升, 目标角色, 置信度, 原因)
func (pe *PromotionEngine) EvaluatePromotion() (bool, AgentRole, float64, string) {
	report := pe.meta.Evaluate()

	// 未完成任务数不足，不评估
	if report.TaskCompleted == 0 {
		return false, RoleExecutor, 0, "尚无完成的任务"
	}

	currentRole := pe.registry.Get(pe.selfID).Role
	targetRole := pe.determineTargetRole(currentRole, report)

	if targetRole == currentRole {
		return false, currentRole, 0, "已处于目标角色"
	}

	// 检查晋升授权
	approved, reason := AuthorizePromotion(report, targetRole, pe.registry, pe.policy)
	if !approved {
		return false, currentRole, 0, reason
	}

	// 计算置信度
	confidence := pe.calculateConfidence(report, targetRole)

	log.Info("promotion evaluated",
		"agent", pe.selfID,
		"from", string(currentRole),
		"to", string(targetRole),
		"confidence", confidence,
		"reason", reason,
	)

	return true, targetRole, confidence, reason
}

// EvaluateDemotion 评估是否应主动降级
// 返回：(是否应降级, 原因)
func (pe *PromotionEngine) EvaluateDemotion() (bool, string) {
	report := pe.meta.Evaluate()
	currentRole := pe.registry.Get(pe.selfID).Role

	if currentRole == RoleExecutor {
		return false, "已是最低角色"
	}

	// 管理角色：子任务失败率 > 30% 应主动降级
	if currentRole == RoleManager || currentRole == RoleSupervisor {
		recentRate := pe.meta.RecentSuccessRate()
		if recentRate < 0.7 {
			return true, fmt.Sprintf("近期成功率过低 (%.0f%%)", recentRate*100)
		}
		if report.ConflictCount > report.TaskCompleted {
			return true, "冲突次数超过完成任务数"
		}
	}

	// 连续失败过多
	if report.TaskFailed > report.TaskCompleted*2 {
		return true, "失败任务数远超成功数"
	}

	return false, ""
}

// RequestPromotion 执行晋升流程
func (pe *PromotionEngine) RequestPromotion(sm *StateMachine) bool {
	shouldPromote, targetRole, confidence, reason := pe.EvaluatePromotion()
	if !shouldPromote {
		return false
	}

	// 执行晋升
	pe.registry.Register(pe.selfID, targetRole, "auto", reason)

	// 更新状态机角色
	if sm != nil {
		sm.SetRole(targetRole)
	}

	// 设置临时晋升的 TTL（非 EXECUTOR 角色默认为 10 分钟）
	if targetRole != RoleExecutor {
		pe.registry.SetRoleTTL(pe.selfID, 10*time.Minute)
	}

	log.Info("agent promoted",
		"agent", pe.selfID,
		"role", string(targetRole),
		"confidence", fmt.Sprintf("%.2f", confidence),
	)

	return true
}

// RequestDemotion 主动降级
func (pe *PromotionEngine) RequestDemotion(sm *StateMachine) bool {
	shouldDemote, reason := pe.EvaluateDemotion()
	if !shouldDemote {
		return false
	}

	currentRole := pe.registry.Get(pe.selfID).Role
	pe.registry.Revoke(pe.selfID, reason)

	if sm != nil {
		sm.SetRole(RoleExecutor)
	}

	log.Info("agent self-demoted",
		"agent", pe.selfID,
		"from", string(currentRole),
		"reason", reason,
	)

	return true
}

// ── 内部方法 ──

func (pe *PromotionEngine) determineTargetRole(currentRole AgentRole, report *MetaReport) AgentRole {
	switch currentRole {
	case RoleExecutor:
		if report.SuccessRate >= pe.policy.MinSuccessRate &&
			report.TaskCompleted >= pe.policy.MinTasksCompleted &&
			!pe.registry.HasActiveSupervisor() {
			return RoleSupervisor
		}
	case RoleSupervisor:
		if report.SuccessRate >= 0.95 && report.TaskCompleted >= 10 {
			return RoleManager
		}
	}
	return currentRole // 保持不变
}

func (pe *PromotionEngine) calculateConfidence(report *MetaReport, targetRole AgentRole) float64 {
	confidence := report.SuccessRate

	// 任务经验加成
	if report.TaskCompleted > 20 {
		confidence += 0.05
	}

	// 冲突惩罚
	if report.ConflictCount > 0 {
		penalty := float64(report.ConflictCount) * 0.02
		confidence -= penalty
	}

	// 负载惩罚
	if report.CurrentLoad > 0.8 {
		confidence -= 0.05
	}

	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}
