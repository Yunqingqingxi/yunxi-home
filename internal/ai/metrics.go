package ai

import (
	"context"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ── MetricEvent ──────────────────────────────────────────

// MetricEvent 单个监控事件，来自 LLM 请求、工具调用、对话轮次等。
type MetricEvent struct {
	Timestamp time.Time         `json:"ts"`
	Type      string            `json:"type"`   // "llm_request", "tool_call", "chat_round", "loop_detected"
	Labels    map[string]string `json:"labels"` // tool_name, model, session_id, status, error_type
	Value     float64           `json:"value"`  // 延迟秒数 / token 数 / 结果字节数 / 1（计数器模式）
	Extra     map[string]any    `json:"extra"`  // args_summary, result_preview, error_detail（可选）
}

// ── CounterSnapshot ──────────────────────────────────────

// CounterSnapshot is a read-only snapshot of the atomic usage counters
// that should be persisted across restarts.
type CounterSnapshot struct {
	InputTokens   int64            `json:"input_tokens"`
	OutputTokens  int64            `json:"output_tokens"`
	CostMicros    int64            `json:"cost_micros"`
	Requests      int64            `json:"requests"`
	Errors        int64            `json:"errors"`
	ToolCalls     int64            `json:"tool_calls"`
	ToolErrors    int64            `json:"tool_errors"`
	TopTools      map[string]int64 `json:"top_tools"`
	// Agent v2.0 counters (persisted across restarts)
	SubAgentSpawned int64 `json:"sub_agent_spawned"`
	SubAgentSuccess int64 `json:"sub_agent_success"`
	SubAgentFailed  int64 `json:"sub_agent_failed"`
	LockConflicts   int64 `json:"lock_conflicts"`
	RolePromotions  int64 `json:"role_promotions"`
	RoleDemotions   int64 `json:"role_demotions"`
}

// ── ToolsSnapshot ───────────────────────────────────────

// ToolsSnapshot 返回当前内存中聚合的工具指标快照，供分析引擎或 API 查询。
type ToolsSnapshot struct {
	Tools map[string]ToolMetrics `json:"tools"`
	TotalLLMRequests int64 `json:"total_llm_requests"`
	TotalLLMErrors   int64 `json:"total_llm_errors"`
	TotalToolCalls   int64 `json:"total_tool_calls"`
	TotalToolErrors  int64 `json:"total_tool_errors"`
	LoopDetected     int64 `json:"loop_detected"`
	Since            time.Time `json:"since"`
	// Agent v2.0 counters
	SubAgentSpawned int64 `json:"sub_agent_spawned"`
	SubAgentSuccess int64 `json:"sub_agent_success"`
	SubAgentFailed  int64 `json:"sub_agent_failed"`
	LockConflicts   int64 `json:"lock_conflicts"`
	RolePromotions  int64 `json:"role_promotions"`
	RoleDemotions   int64 `json:"role_demotions"`
}

// ToolMetrics 单个工具的聚合指标
type ToolMetrics struct {
	Calls       int64           `json:"calls"`
	Errors      int64           `json:"errors"`
	TotalLatency float64        `json:"total_latency"` // 秒，用于计算均值
	P50         float64         `json:"p50"`
	P95         float64         `json:"p95"`
	P99         float64         `json:"p99"`
	ResultBytes int64           `json:"result_bytes"`
	Truncations int64           `json:"truncations"`
	RecentLats  []float64       `json:"-"` // 最近 N 次延迟样本，用于百分位计算
}

// ── MetricsCollector ────────────────────────────────────

// MetricsCollector 非阻塞、环形缓冲的指标采集器。
// 热路径上的 Record() 仅做原子操作 + 环形写，不阻塞业务逻辑。
type MetricsCollector struct {
	buffer    []MetricEvent
	cursor    atomic.Int64    // 环形缓冲区写入位置
	size      int64
	startTime time.Time

	// 原子计数器（锁无关读取）
	TotalLLMRequests  atomic.Int64
	TotalLLMErrors    atomic.Int64
	TotalToolCalls    atomic.Int64
	TotalToolErrors   atomic.Int64
	LoopDetected      atomic.Int64
	TotalInputTokens  atomic.Int64
	TotalOutputTokens atomic.Int64
	TotalCostMicros   atomic.Int64 // cost in micro-dollars (USD * 1e6)
	// Agent v2.0
	SubAgentSpawned atomic.Int64
	SubAgentSuccess atomic.Int64
	SubAgentFailed  atomic.Int64
	LockConflicts   atomic.Int64
	RolePromotions  atomic.Int64
	RoleDemotions   atomic.Int64

	// 工具延迟汇总（EWMA + 最近样本）
	toolMu    sync.RWMutex
	toolStats map[string]*toolStatsInternal

	// 订阅者
	subMu     sync.RWMutex
	subscribers []chan<- MetricEvent

	// 批量落库回调
	flushFn     func(ctx context.Context, batch []MetricEvent) error
	flushBatch  []MetricEvent
	flushMu     sync.Mutex
	flushSize   int
	flushTick   time.Duration

	// 计数器持久化回调（每 30s 调用一次）
	saveFn   func(CounterSnapshot)
}

type toolStatsInternal struct {
	calls       int64
	errors      int64
	totalLat    float64
	resultBytes int64
	truncations int64
	recentLats  []float64 // 环形，最多存 200 个样本
	latCursor   int
}

const (
	defaultRingSize   = 16384
	defaultFlushSize  = 100
	defaultFlushTick  = 30 * time.Second
	maxLatSamples     = 200
)

// SetFlushFn sets the batch persistence callback (e.g., to write to ai_event_log).
func (mc *MetricsCollector) SetFlushFn(fn func(ctx context.Context, batch []MetricEvent) error) {
	mc.flushFn = fn
	if fn != nil { go mc.flushLoop() }
}

// NewMetricsCollector creates a new metrics collector.
// saveFn 可选：设置后每 30 秒将计数器快照传入回调用于持久化。
func NewMetricsCollector(flushFn func(ctx context.Context, batch []MetricEvent) error, saveFn func(CounterSnapshot)) *MetricsCollector {
	mc := &MetricsCollector{
		buffer:     make([]MetricEvent, defaultRingSize),
		size:       defaultRingSize,
		startTime:  time.Now(),
		toolStats:  make(map[string]*toolStatsInternal),
		flushFn:    flushFn,
		flushBatch: make([]MetricEvent, 0, defaultFlushSize),
		flushSize:  defaultFlushSize,
		flushTick:  defaultFlushTick,
		saveFn:     saveFn,
	}
	if flushFn != nil {
		go mc.flushLoop()
	}
	if saveFn != nil {
		go mc.saveLoop()
	}
	return mc
}

// ── Record ──────────────────────────────────────────────

// Record 记录单个监控事件。非阻塞，永不 panic。
func (mc *MetricsCollector) Record(ev MetricEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	// 1. 更新原子计数器
	mc.updateCounters(&ev)

	// 2. 写入环形缓冲区
	pos := mc.cursor.Add(1) - 1
	idx := int(pos % mc.size)
	mc.buffer[idx] = ev

	// 3. 广播给订阅者（非阻塞发送）
	mc.subMu.RLock()
	for _, ch := range mc.subscribers {
		select {
		case ch <- ev:
		default:
			// 订阅者消费太慢，丢弃本次事件
		}
	}
	mc.subMu.RUnlock()

	// 4. 异步批量落库
	if mc.flushFn != nil {
		mc.flushMu.Lock()
		mc.flushBatch = append(mc.flushBatch, ev)
		shouldFlush := len(mc.flushBatch) >= mc.flushSize
		mc.flushMu.Unlock()
		if shouldFlush {
			go mc.doFlush()
		}
	}
}

// RecordTokens records token usage and cost for a completed LLM request.
func (mc *MetricsCollector) RecordTokens(input, output int64, cost float64) {
	mc.TotalInputTokens.Add(input)
	mc.TotalOutputTokens.Add(output)
	if cost > 0 {
		mc.TotalCostMicros.Add(int64(cost * 1e6))
	}
}

func (mc *MetricsCollector) updateCounters(ev *MetricEvent) {
	switch ev.Type {
	case "llm_request":
		mc.TotalLLMRequests.Add(1)
		if ev.Labels["status"] == "error" {
			mc.TotalLLMErrors.Add(1)
		}
	case "tool_call":
		mc.TotalToolCalls.Add(1)
		mc.recordToolLatency(ev)
		if ev.Labels["status"] == "error" {
			mc.TotalToolErrors.Add(1)
		}
	case "loop_detected":
		mc.LoopDetected.Add(1)
	}
}

func (mc *MetricsCollector) recordToolLatency(ev *MetricEvent) {
	toolName := ev.Labels["tool"]
	if toolName == "" {
		return
	}

	mc.toolMu.Lock()
	ts, ok := mc.toolStats[toolName]
	if !ok {
		ts = &toolStatsInternal{
			recentLats: make([]float64, maxLatSamples),
		}
		mc.toolStats[toolName] = ts
	}
	ts.calls++
	ts.totalLat += ev.Value
	if ev.Labels["status"] == "error" {
		ts.errors++
	}
	if resultBytes, ok := ev.Extra["result_len"].(float64); ok {
		ts.resultBytes += int64(resultBytes)
	}
	if truncated, ok := ev.Extra["truncated"].(bool); ok && truncated {
		ts.truncations++
	}
	// 环形写入延迟样本
	ts.recentLats[ts.latCursor%maxLatSamples] = ev.Value
	ts.latCursor++
	mc.toolMu.Unlock()
}

// ── Subscribe ───────────────────────────────────────────

// Subscribe 返回一个接收实时事件的 channel。消费者必须快速消费，
// 否则事件会被丢弃（非阻塞发送）。
func (mc *MetricsCollector) Subscribe(bufSize int) <-chan MetricEvent {
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan MetricEvent, bufSize)
	mc.subMu.Lock()
	mc.subscribers = append(mc.subscribers, ch)
	mc.subMu.Unlock()
	return ch
}

// ── Snapshot ────────────────────────────────────────────

// Snapshot 返回当前内存中聚合指标的只读快照，开销很小。
func (mc *MetricsCollector) Snapshot() ToolsSnapshot {
	mc.toolMu.RLock()
	defer mc.toolMu.RUnlock()

	snap := ToolsSnapshot{
		Tools:            make(map[string]ToolMetrics, len(mc.toolStats)),
		TotalLLMRequests: mc.TotalLLMRequests.Load(),
		TotalLLMErrors:   mc.TotalLLMErrors.Load(),
		TotalToolCalls:   mc.TotalToolCalls.Load(),
		TotalToolErrors:  mc.TotalToolErrors.Load(),
		LoopDetected:     mc.LoopDetected.Load(),
		Since:            mc.startTime,
		SubAgentSpawned:  mc.SubAgentSpawned.Load(),
		SubAgentSuccess:  mc.SubAgentSuccess.Load(),
		SubAgentFailed:   mc.SubAgentFailed.Load(),
		LockConflicts:    mc.LockConflicts.Load(),
		RolePromotions:   mc.RolePromotions.Load(),
		RoleDemotions:    mc.RoleDemotions.Load(),
	}

	for name, ts := range mc.toolStats {
		tm := ToolMetrics{
			Calls:        ts.calls,
			Errors:       ts.errors,
			TotalLatency: ts.totalLat,
			ResultBytes:  ts.resultBytes,
			Truncations:  ts.truncations,
		}
		// 计算百分位
		samples := ts.recentLats
		count := min(ts.latCursor, maxLatSamples)
		if count > 0 {
			valid := make([]float64, count)
			copy(valid, samples[:count])
			tm.P50, tm.P95, tm.P99 = percentile(valid)
		}
		snap.Tools[name] = tm
	}

	return snap
}

// SnapshotCounters returns a read-only snapshot of the persistent usage counters.
func (mc *MetricsCollector) SnapshotCounters() CounterSnapshot {
	mc.toolMu.RLock()
	topTools := make(map[string]int64, len(mc.toolStats))
	for name, ts := range mc.toolStats {
		if ts.calls > 0 {
			topTools[name] = ts.calls
		}
	}
	mc.toolMu.RUnlock()

	return CounterSnapshot{
		InputTokens:     mc.TotalInputTokens.Load(),
		OutputTokens:    mc.TotalOutputTokens.Load(),
		CostMicros:      mc.TotalCostMicros.Load(),
		Requests:        mc.TotalLLMRequests.Load(),
		Errors:          mc.TotalLLMErrors.Load(),
		ToolCalls:       mc.TotalToolCalls.Load(),
		ToolErrors:      mc.TotalToolErrors.Load(),
		TopTools:        topTools,
		SubAgentSpawned: mc.SubAgentSpawned.Load(),
		SubAgentSuccess: mc.SubAgentSuccess.Load(),
		SubAgentFailed:  mc.SubAgentFailed.Load(),
		LockConflicts:   mc.LockConflicts.Load(),
		RolePromotions:  mc.RolePromotions.Load(),
		RoleDemotions:   mc.RoleDemotions.Load(),
	}
}

// LoadFromSnapshot restores counter values from a previously persisted snapshot.
func (mc *MetricsCollector) LoadFromSnapshot(s CounterSnapshot) {
	mc.TotalInputTokens.Store(s.InputTokens)
	mc.TotalOutputTokens.Store(s.OutputTokens)
	mc.TotalCostMicros.Store(s.CostMicros)
	mc.TotalLLMRequests.Store(s.Requests)
	mc.TotalLLMErrors.Store(s.Errors)
	mc.TotalToolCalls.Store(s.ToolCalls)
	mc.TotalToolErrors.Store(s.ToolErrors)
	mc.SubAgentSpawned.Store(s.SubAgentSpawned)
	mc.SubAgentSuccess.Store(s.SubAgentSuccess)
	mc.SubAgentFailed.Store(s.SubAgentFailed)
	mc.LockConflicts.Store(s.LockConflicts)
	mc.RolePromotions.Store(s.RolePromotions)
	mc.RoleDemotions.Store(s.RoleDemotions)
	// Restore per-tool call counts
	mc.toolMu.Lock()
	for name, count := range s.TopTools {
		if ts, ok := mc.toolStats[name]; ok {
			ts.calls += count // add to existing (in case some calls happened before restore)
		} else {
			mc.toolStats[name] = &toolStatsInternal{
				calls:      count,
				recentLats: make([]float64, maxLatSamples),
			}
		}
	}
	mc.toolMu.Unlock()
}

// ── RecentEvents ────────────────────────────────────────

// RecentEvents 返回环形缓冲区中的最近 N 条事件。
func (mc *MetricsCollector) RecentEvents(n int) []MetricEvent {
	if n <= 0 {
		n = 100
	}
	cur := mc.cursor.Load()
	total := int(min(int64(n), mc.size))
	events := make([]MetricEvent, 0, total)

	for i := total - 1; i >= 0; i-- {
		pos := (cur - 1 - int64(i)) % mc.size
		if pos < 0 {
			pos += mc.size
		}
		ev := mc.buffer[pos]
		if ev.Type != "" {
			events = append(events, ev)
		}
	}
	return events
}

// ── 批量落库 ────────────────────────────────────────────

func (mc *MetricsCollector) flushLoop() {
	ticker := time.NewTicker(mc.flushTick)
	defer ticker.Stop()
	for range ticker.C {
		mc.doFlush()
	}
}

func (mc *MetricsCollector) doFlush() {
	if mc.flushFn == nil {
		return
	}
	mc.flushMu.Lock()
	if len(mc.flushBatch) == 0 {
		mc.flushMu.Unlock()
		return
	}
	batch := make([]MetricEvent, len(mc.flushBatch))
	copy(batch, mc.flushBatch)
	mc.flushBatch = mc.flushBatch[:0]
	mc.flushMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mc.flushFn(ctx, batch); err != nil {
		log.Warn("metrics flush failed", "count", len(batch), "error", err)
	} else {
		log.Debug("metrics flushed", "count", len(batch))
	}
}

// Flush 主动触发一次落库。
func (mc *MetricsCollector) Flush() {
	mc.doFlush()
}

// saveLoop periodically persists the counter snapshot via saveFn.
func (mc *MetricsCollector) saveLoop() {
	ticker := time.NewTicker(mc.flushTick)
	defer ticker.Stop()
	for range ticker.C {
		if mc.saveFn != nil {
			mc.saveFn(mc.SnapshotCounters())
		}
	}
}

// ── 辅助 ────────────────────────────────────────────────

func percentile(sorted []float64) (p50, p95, p99 float64) {
	n := len(sorted)
	if n == 0 {
		return 0, 0, 0
	}
	// 排序
	for i := 1; i < n; i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	p50 = sorted[n*50/100]
	p95 = sorted[n*95/100]
	p99 = sorted[n*99/100]
	// 避免极端值过于夸张
	p99 = math.Min(p99, p95*5)
	return
}
