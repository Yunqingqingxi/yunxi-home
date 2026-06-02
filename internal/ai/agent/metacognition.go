package agent

import (
	"sync"
	"time"
)

// MetaReport 元认知评估报告
type MetaReport struct {
	AgentID       string    `json:"agent_id"`
	SuccessRate   float64   `json:"success_rate"`   // 成功率 0.0-1.0
	AvgLatencyMs  float64   `json:"avg_latency_ms"`  // 平均延迟
	ConflictCount int       `json:"conflict_count"` // 冲突次数
	TaskCompleted int       `json:"task_completed"` // 已完成任务数
	TaskFailed    int       `json:"task_failed"`    // 失败任务数
	TotalRounds   int       `json:"total_rounds"`   // 总轮次
	AvgRounds     float64   `json:"avg_rounds"`     // 平均每任务轮次
	CurrentLoad   float64   `json:"current_load"`   // 当前负载 0.0-1.0
	LastEvaluated time.Time `json:"last_evaluated"`
}

// Metacognition 元认知模块：Agent 自我评估
type Metacognition struct {
	mu           sync.RWMutex
	agentID      string

	// 累计指标
	totalTasks    int
	successTasks  int
	failedTasks   int
	totalRounds   int
	totalLatency  time.Duration // 累计延迟
	conflictCount int

	// 时间窗口指标（最近 1 小时）
	recentSuccess  int
	recentFailures int
	recentWindow   time.Time

	// 负载跟踪
	activeTasks  int
	maxTasks     int

	lastEval time.Time
}

// NewMetacognition 创建元认知模块
func NewMetacognition(agentID string) *Metacognition {
	return &Metacognition{
		agentID:      agentID,
		recentWindow: time.Now(),
		lastEval:     time.Now(),
	}
}

// RecordSuccess 记录一次成功
func (m *Metacognition) RecordSuccess(rounds int, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalTasks++
	m.successTasks++
	m.totalRounds += rounds
	m.totalLatency += latency
	m.recentSuccess++
	m.activeTasks--
	if m.activeTasks < 0 {
		m.activeTasks = 0
	}
	m.lastEval = time.Now()
}

// RecordFailure 记录一次失败
func (m *Metacognition) RecordFailure(rounds int, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalTasks++
	m.failedTasks++
	m.totalRounds += rounds
	m.totalLatency += latency
	m.recentFailures++
	m.activeTasks--
	if m.activeTasks < 0 {
		m.activeTasks = 0
	}
	m.lastEval = time.Now()
}

// RecordConflict 记录一次资源冲突
func (m *Metacognition) RecordConflict() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conflictCount++
	m.lastEval = time.Now()
}

// TaskStarted 任务开始（跟踪负载）
func (m *Metacognition) TaskStarted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTasks++
	m.lastEval = time.Now()
}

// TaskEnded 任务结束
func (m *Metacognition) TaskEnded() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTasks--
	if m.activeTasks < 0 {
		m.activeTasks = 0
	}
}

// Evaluate 生成当前元认知评估报告
func (m *Metacognition) Evaluate() *MetaReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 刷新时间窗口
	if time.Since(m.recentWindow) > 1*time.Hour {
		// 注意：这里不能写（只持有读锁），用新鲜值近似
	}

	var successRate float64
	if m.totalTasks > 0 {
		successRate = float64(m.successTasks) / float64(m.totalTasks)
	}

	var avgLatency float64
	if m.totalTasks > 0 {
		avgLatency = float64(m.totalLatency.Milliseconds()) / float64(m.totalTasks)
	}

	var avgRounds float64
	if m.totalTasks > 0 {
		avgRounds = float64(m.totalRounds) / float64(m.totalTasks)
	}

	var currentLoad float64
	if m.maxTasks > 0 {
		currentLoad = float64(m.activeTasks) / float64(m.maxTasks)
	}

	return &MetaReport{
		AgentID:       m.agentID,
		SuccessRate:   successRate,
		AvgLatencyMs:  avgLatency,
		ConflictCount: m.conflictCount,
		TaskCompleted: m.successTasks,
		TaskFailed:    m.failedTasks,
		TotalRounds:   m.totalRounds,
		AvgRounds:     avgRounds,
		CurrentLoad:   currentLoad,
		LastEvaluated: m.lastEval,
	}
}

// RecentSuccessRate 最近 1 小时成功率
func (m *Metacognition) RecentSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果窗口过期，重置
	total := m.recentSuccess + m.recentFailures
	if total == 0 {
		return 1.0 // 无数据，假设优秀
	}
	return float64(m.recentSuccess) / float64(total)
}

// ResetRecentWindow 重置时间窗口
func (m *Metacognition) ResetRecentWindow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recentSuccess = 0
	m.recentFailures = 0
	m.recentWindow = time.Now()
}

// TotalTasks 总任务数
func (m *Metacognition) TotalTasks() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalTasks
}
