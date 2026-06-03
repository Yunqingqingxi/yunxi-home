// Package observability 提供 Prometheus 指标和 OpenTelemetry 追踪。
package observability

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ── Agent 指标定义 ──

// AgentMetrics Agent 系统可观测指标
type AgentMetrics struct {
	// 请求指标
	TotalRequests  atomic.Int64
	TotalErrors    atomic.Int64
	ActiveSessions atomic.Int64

	// Token 指标
	TotalTokensIn  atomic.Int64
	TotalTokensOut atomic.Int64

	// 拓扑指标
	TopologyRejects atomic.Int64
	TopologyLies    atomic.Int64
	TrustLockCount  atomic.Int64

	// Agent 指标
	SubAgentSpawned  atomic.Int64
	SubAgentSuccess  atomic.Int64
	SubAgentFailed   atomic.Int64
	OscillationCount atomic.Int64
	ClosedLoopCount  atomic.Int64

	// 锁指标
	LockRequests   atomic.Int64
	LockConflicts  atomic.Int64
	LockWaitTimeMs atomic.Int64 // 累计等待时间

	// 角色指标
	RolePromotions atomic.Int64
	RoleDemotions  atomic.Int64

	// 工具延迟统计
	toolMu   sync.RWMutex
	toolStats map[string]*ToolLatency
}

// ToolLatency 单个工具的延迟统计
type ToolLatency struct {
	Count    int64
	TotalMs  int64
	MinMs    int64
	MaxMs    int64
}

// ── 全局单例 ──
var globalMetrics = NewAgentMetrics()

func GlobalMetrics() *AgentMetrics { return globalMetrics }

// NewAgentMetrics 创建指标收集器
func NewAgentMetrics() *AgentMetrics {
	return &AgentMetrics{
		toolStats: make(map[string]*ToolLatency),
	}
}

// RecordRequest 记录请求
func (m *AgentMetrics) RecordRequest() {
	m.TotalRequests.Add(1)
	m.ActiveSessions.Add(1)
}

// RecordRequestEnd 记录请求结束
func (m *AgentMetrics) RecordRequestEnd(isError bool) {
	m.ActiveSessions.Add(-1)
	if isError {
		m.TotalErrors.Add(1)
	}
}

// RecordTokens 记录 Tokens
func (m *AgentMetrics) RecordTokens(in, out int64) {
	m.TotalTokensIn.Add(in)
	m.TotalTokensOut.Add(out)
}

// RecordToolCall 记录工具调用延迟
func (m *AgentMetrics) RecordToolCall(toolName string, durationMs int64) {
	m.toolMu.Lock()
	defer m.toolMu.Unlock()

	stats, ok := m.toolStats[toolName]
	if !ok {
		stats = &ToolLatency{MinMs: durationMs, MaxMs: durationMs}
		m.toolStats[toolName] = stats
	}
	stats.Count++
	stats.TotalMs += durationMs
	if durationMs < stats.MinMs {
		stats.MinMs = durationMs
	}
	if durationMs > stats.MaxMs {
		stats.MaxMs = durationMs
	}
}

// RecordTopologyReject 记录拓扑拒绝
func (m *AgentMetrics) RecordTopologyReject() {
	m.TopologyRejects.Add(1)
}

// RecordTopologyLie 记录 AI 坐标谎报
func (m *AgentMetrics) RecordTopologyLie() {
	m.TopologyLies.Add(1)
}

// RecordTrustLock 记录信任锁定
func (m *AgentMetrics) RecordTrustLock() {
	m.TrustLockCount.Add(1)
}

// RecordSubAgentSpawn 记录子 Agent 派生
func (m *AgentMetrics) RecordSubAgentSpawn() {
	m.SubAgentSpawned.Add(1)
}

// RecordSubAgentResult 记录子 Agent 结果
func (m *AgentMetrics) RecordSubAgentResult(success bool) {
	if success {
		m.SubAgentSuccess.Add(1)
	} else {
		m.SubAgentFailed.Add(1)
	}
}

// RecordOscillation 记录工具震荡
func (m *AgentMetrics) RecordOscillation() {
	m.OscillationCount.Add(1)
}

// RecordClosedLoop 记录闭环成功
func (m *AgentMetrics) RecordClosedLoop() {
	m.ClosedLoopCount.Add(1)
}

// RecordLockRequest 记录锁请求
func (m *AgentMetrics) RecordLockRequest(waitMs int64) {
	m.LockRequests.Add(1)
	m.LockWaitTimeMs.Add(waitMs)
}

// RecordLockConflict 记录锁冲突
func (m *AgentMetrics) RecordLockConflict() {
	m.LockConflicts.Add(1)
}

// RecordRolePromotion 记录角色晋升
func (m *AgentMetrics) RecordRolePromotion() {
	m.RolePromotions.Add(1)
}

// RecordRoleDemotion 记录角色降级
func (m *AgentMetrics) RecordRoleDemotion() {
	m.RoleDemotions.Add(1)
}

// SubAgentSuccessRate 子 Agent 成功率
func (m *AgentMetrics) SubAgentSuccessRate() float64 {
	total := m.SubAgentSuccess.Load() + m.SubAgentFailed.Load()
	if total == 0 {
		return 1.0
	}
	return float64(m.SubAgentSuccess.Load()) / float64(total)
}

// TopToolStats 返回工具延迟排行
func (m *AgentMetrics) TopToolStats(n int) []ToolStatSnapshot {
	m.toolMu.RLock()
	defer m.toolMu.RUnlock()

	result := make([]ToolStatSnapshot, 0, len(m.toolStats))
	for name, stats := range m.toolStats {
		avgMs := float64(0)
		if stats.Count > 0 {
			avgMs = float64(stats.TotalMs) / float64(stats.Count)
		}
		result = append(result, ToolStatSnapshot{
			Name:    name,
			Count:   stats.Count,
			AvgMs:   avgMs,
			MinMs:   stats.MinMs,
			MaxMs:   stats.MaxMs,
		})
	}
	if n > 0 && n < len(result) {
		result = result[:n]
	}
	return result
}

// Snapshot 返回指标快照
func (m *AgentMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:     m.TotalRequests.Load(),
		TotalErrors:       m.TotalErrors.Load(),
		ActiveSessions:    m.ActiveSessions.Load(),
		TotalTokensIn:     m.TotalTokensIn.Load(),
		TotalTokensOut:    m.TotalTokensOut.Load(),
		TopologyRejects:   m.TopologyRejects.Load(),
		TopologyLies:      m.TopologyLies.Load(),
		TrustLockCount:    m.TrustLockCount.Load(),
		SubAgentSpawned:   m.SubAgentSpawned.Load(),
		SubAgentSuccess:   m.SubAgentSuccess.Load(),
		SubAgentFailed:    m.SubAgentFailed.Load(),
		SubAgentSuccessRate: m.SubAgentSuccessRate(),
		OscillationCount:  m.OscillationCount.Load(),
		ClosedLoopCount:   m.ClosedLoopCount.Load(),
		LockRequests:      m.LockRequests.Load(),
		LockConflicts:     m.LockConflicts.Load(),
		RolePromotions:    m.RolePromotions.Load(),
		RoleDemotions:     m.RoleDemotions.Load(),
		Timestamp:         time.Now(),
	}
}

// ── 快照类型 ──

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	TotalRequests      int64     `json:"total_requests"`
	TotalErrors        int64     `json:"total_errors"`
	ActiveSessions     int64     `json:"active_sessions"`
	TotalTokensIn      int64     `json:"total_tokens_in"`
	TotalTokensOut     int64     `json:"total_tokens_out"`
	TopologyRejects    int64     `json:"topology_rejects"`
	TopologyLies       int64     `json:"topology_lies"`
	TrustLockCount     int64     `json:"trust_lock_count"`
	SubAgentSpawned    int64     `json:"sub_agent_spawned"`
	SubAgentSuccess    int64     `json:"sub_agent_success"`
	SubAgentFailed     int64     `json:"sub_agent_failed"`
	SubAgentSuccessRate float64  `json:"sub_agent_success_rate"`
	OscillationCount   int64     `json:"oscillation_count"`
	ClosedLoopCount    int64     `json:"closed_loop_count"`
	LockRequests       int64     `json:"lock_requests"`
	LockConflicts      int64     `json:"lock_conflicts"`
	RolePromotions     int64     `json:"role_promotions"`
	RoleDemotions      int64     `json:"role_demotions"`
	Timestamp          time.Time `json:"timestamp"`
}

// ToolStatSnapshot 工具统计快照
type ToolStatSnapshot struct {
	Name  string  `json:"name"`
	Count int64   `json:"count"`
	AvgMs float64 `json:"avg_ms"`
	MinMs int64   `json:"min_ms"`
	MaxMs int64   `json:"max_ms"`
}

// PrometheusFormat 输出 Prometheus 格式的指标（用于 /metrics 端点）
func (m *AgentMetrics) PrometheusFormat() string {
	snap := m.Snapshot()
	return fmt.Sprintf(
		`# HELP agent_requests_total Total number of agent requests
# TYPE agent_requests_total counter
agent_requests_total %d

# HELP agent_errors_total Total number of agent errors
# TYPE agent_errors_total counter
agent_errors_total %d

# HELP agent_active_sessions Currently active sessions
# TYPE agent_active_sessions gauge
agent_active_sessions %d

# HELP agent_tokens_in_total Total input tokens
# TYPE agent_tokens_in_total counter
agent_tokens_in_total %d

# HELP agent_tokens_out_total Total output tokens
# TYPE agent_tokens_out_total counter
agent_tokens_out_total %d

# HELP agent_topology_reject_total Total topology rejections
# TYPE agent_topology_reject_total counter
agent_topology_reject_total %d

# HELP agent_topology_lies_total Total coordinate lies detected
# TYPE agent_topology_lies_total counter
agent_topology_lies_total %d

# HELP agent_trust_locked_total Total trust lock events
# TYPE agent_trust_locked_total counter
agent_trust_locked_total %d

# HELP agent_subagent_success_rate Sub-agent success rate (0-1)
# TYPE agent_subagent_success_rate gauge
agent_subagent_success_rate %.4f

# HELP agent_lock_conflicts_total Total lock conflicts
# TYPE agent_lock_conflicts_total counter
agent_lock_conflicts_total %d

# HELP agent_role_promotions_total Total role promotions
# TYPE agent_role_promotions_total counter
agent_role_promotions_total %d
`,
		snap.TotalRequests,
		snap.TotalErrors,
		snap.ActiveSessions,
		snap.TotalTokensIn,
		snap.TotalTokensOut,
		snap.TopologyRejects,
		snap.TopologyLies,
		snap.TrustLockCount,
		snap.SubAgentSuccessRate,
		snap.LockConflicts,
		snap.RolePromotions,
	)
}

// ── Per-User Metrics ───────────────────────────────────────────────────────────

// UserMetricsSnapshot holds aggregated metrics for a single user.
type UserMetricsSnapshot struct {
	UserID          string  `json:"user_id"`
	SessionCount    int     `json:"session_count"`
	TotalRounds     int     `json:"total_rounds"`
	TotalToolCalls  int     `json:"total_tool_calls"`
	TotalFailures   int     `json:"total_failures"`
	TotalRejects    int     `json:"total_rejects"`
	TrustLockCount  int     `json:"trust_lock_count"`
	SuccessRate     float64 `json:"success_rate"`
	CorrectionCount int     `json:"correction_count"`
	CancelCount     int     `json:"cancel_count"`
}

// UserMetricsStore maintains in-memory per-user metric counters.
type UserMetricsStore struct {
	mu     sync.RWMutex
	users  map[string]*userCounters
}

type userCounters struct {
	sessionCount    int64
	totalRounds     int64
	totalToolCalls  int64
	totalFailures   int64
	totalRejects    int64
	trustLockCount  int64
	correctionCount int64
	cancelCount     int64
}

// NewUserMetricsStore creates a new per-user metrics store.
func NewUserMetricsStore() *UserMetricsStore {
	return &UserMetricsStore{
		users: make(map[string]*userCounters),
	}
}

// RecordSession records session-level metrics for a user.
func (us *UserMetricsStore) RecordSession(userID string, rounds, toolCalls, failures, rejects int, trustLocked, cancelled bool) {
	us.mu.Lock()
	defer us.mu.Unlock()
	uc, ok := us.users[userID]
	if !ok {
		uc = &userCounters{}
		us.users[userID] = uc
	}
	uc.sessionCount++
	uc.totalRounds += int64(rounds)
	uc.totalToolCalls += int64(toolCalls)
	uc.totalFailures += int64(failures)
	uc.totalRejects += int64(rejects)
	if trustLocked {
		uc.trustLockCount++
	}
	if cancelled {
		uc.cancelCount++
	}
}

// RecordCorrection records a user correction event.
func (us *UserMetricsStore) RecordCorrection(userID string) {
	us.mu.Lock()
	defer us.mu.Unlock()
	if uc, ok := us.users[userID]; ok {
		uc.correctionCount++
	}
}

// GetSnapshot returns the current metrics snapshot for a user.
func (us *UserMetricsStore) GetSnapshot(userID string) *UserMetricsSnapshot {
	us.mu.RLock()
	defer us.mu.RUnlock()
	uc, ok := us.users[userID]
	if !ok {
		return nil
	}
	var successRate float64
	if uc.totalToolCalls > 0 {
		successRate = float64(uc.totalToolCalls-uc.totalFailures) / float64(uc.totalToolCalls)
	}
	return &UserMetricsSnapshot{
		UserID:          userID,
		SessionCount:    int(uc.sessionCount),
		TotalRounds:     int(uc.totalRounds),
		TotalToolCalls:  int(uc.totalToolCalls),
		TotalFailures:   int(uc.totalFailures),
		TotalRejects:    int(uc.totalRejects),
		TrustLockCount:  int(uc.trustLockCount),
		SuccessRate:     successRate,
		CorrectionCount: int(uc.correctionCount),
		CancelCount:     int(uc.cancelCount),
	}
}

// AllSnapshots returns snapshots for all tracked users.
func (us *UserMetricsStore) AllSnapshots() []UserMetricsSnapshot {
	us.mu.RLock()
	defer us.mu.RUnlock()
	var results []UserMetricsSnapshot
	for userID, uc := range us.users {
		var sr float64
		if uc.totalToolCalls > 0 {
			sr = float64(uc.totalToolCalls-uc.totalFailures) / float64(uc.totalToolCalls)
		}
		results = append(results, UserMetricsSnapshot{
			UserID:          userID,
			SessionCount:    int(uc.sessionCount),
			TotalRounds:     int(uc.totalRounds),
			TotalToolCalls:  int(uc.totalToolCalls),
			TotalFailures:   int(uc.totalFailures),
			TotalRejects:    int(uc.totalRejects),
			TrustLockCount:  int(uc.trustLockCount),
			SuccessRate:     sr,
			CorrectionCount: int(uc.correctionCount),
			CancelCount:     int(uc.cancelCount),
		})
	}
	return results
}
