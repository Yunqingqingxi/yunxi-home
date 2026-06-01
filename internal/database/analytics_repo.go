package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AnalyticsRepository 将 AI 监控事件持久化到数据库，并提供聚合查询。
type AnalyticsRepository struct {
	exec Executor
}

// NewAnalyticsRepository 创建分析仓库。
func NewAnalyticsRepository(exec Executor) *AnalyticsRepository {
	return &AnalyticsRepository{exec: exec}
}

// ── 事件日志 ────────────────────────────────────────────

// InsertEvents 批量插入事件日志。单条 INSERT 也可以用，但批量效率更高。
func (r *AnalyticsRepository) InsertEvents(ctx context.Context, events []AIMetricRow) error {
	if len(events) == 0 {
		return nil
	}

	const query = `INSERT INTO ai_event_log
		(created_at, session_id, event_type, tool_name, model, status, value, labels_json, extra_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, ev := range events {
		labelsJSON := "{}"
		if ev.LabelsJSON != "" {
			labelsJSON = ev.LabelsJSON
		}
		extraJSON := "{}"
		if ev.ExtraJSON != "" {
			extraJSON = ev.ExtraJSON
		}
		if _, err := r.exec.ExecContext(ctx, query,
			ev.CreatedAt, ev.SessionID, ev.EventType, ev.ToolName,
			ev.Model, ev.Status, ev.Value, labelsJSON, extraJSON,
		); err != nil {
			return fmt.Errorf("insert ai_event_log: %w", err)
		}
	}
	return nil
}

// AIMetricRow 事件日志的一行。
type AIMetricRow struct {
	CreatedAt  string  `json:"created_at"`
	SessionID  string  `json:"session_id"`
	EventType  string  `json:"event_type"`
	ToolName   string  `json:"tool_name"`
	Model      string  `json:"model"`
	Status     string  `json:"status"`
	Value      float64 `json:"value"`
	LabelsJSON string  `json:"labels_json"`
	ExtraJSON  string  `json:"extra_json"`
}

// ── 事件查询 ────────────────────────────────────────────

// QueryEventsParams 事件查询参数。
type QueryEventsParams struct {
	EventType string    // 可选过滤
	ToolName  string    // 可选过滤
	Since     time.Time // 起始时间
	Limit     int       // 默认 100
	Offset    int       // 分页偏移
}

// QueryEventsResult 事件查询结果。
type QueryEventsResult struct {
	Events []AIMetricRow `json:"events"`
	Total  int64         `json:"total"`
}

// QueryEvents 按条件查询事件日志。
func (r *AnalyticsRepository) QueryEvents(ctx context.Context, params QueryEventsParams) (*QueryEventsResult, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}

	where := "WHERE created_at >= ?"
	args := []any{params.Since.Format("2006-01-02 15:04:05")}

	if params.EventType != "" {
		where += " AND event_type = ?"
		args = append(args, params.EventType)
	}
	if params.ToolName != "" {
		where += " AND tool_name = ?"
		args = append(args, params.ToolName)
	}

	// Count
	var total int64
	countSQL := "SELECT COUNT(*) FROM ai_event_log " + where
	if err := r.exec.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count ai_event_log: %w", err)
	}

	// Select
	selectSQL := fmt.Sprintf(
		"SELECT created_at, session_id, event_type, tool_name, model, status, value, labels_json, extra_json FROM ai_event_log %s ORDER BY created_at DESC LIMIT ? OFFSET ?",
		where,
	)
	selectArgs := append(args, params.Limit, params.Offset)

	rows, err := r.exec.QueryContext(ctx, selectSQL, selectArgs...)
	if err != nil {
		return nil, fmt.Errorf("query ai_event_log: %w", err)
	}
	defer rows.Close()

	var events []AIMetricRow
	for rows.Next() {
		var ev AIMetricRow
		if err := rows.Scan(&ev.CreatedAt, &ev.SessionID, &ev.EventType,
			&ev.ToolName, &ev.Model, &ev.Status, &ev.Value,
			&ev.LabelsJSON, &ev.ExtraJSON,
		); err != nil {
			return nil, fmt.Errorf("scan ai_event_log: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows ai_event_log: %w", err)
	}
	if events == nil {
		events = []AIMetricRow{}
	}

	return &QueryEventsResult{Events: events, Total: total}, nil
}

// ── 工具调用统计 ────────────────────────────────────────

// ToolStats 按工具的聚合统计。
type ToolStats struct {
	ToolName  string  `json:"tool_name"`
	Calls     int64   `json:"calls"`
	Errors    int64   `json:"errors"`
	AvgLatMs  float64 `json:"avg_lat_ms"`
	MaxLatMs  float64 `json:"max_lat_ms"`
	ResultKB  float64 `json:"result_kb"`
}

// GetToolStats 查询指定时间范围内按工具分组的统计。
func (r *AnalyticsRepository) GetToolStats(ctx context.Context, since time.Duration) ([]ToolStats, error) {
	sinceTime := time.Now().Add(-since).Format("2006-01-02 15:04:05")

	query := `SELECT
		tool_name,
		COUNT(*) as calls,
		SUM(CASE WHEN status='error' THEN 1 ELSE 0 END) as errors,
		AVG(CASE WHEN status='success' THEN value ELSE NULL END) as avg_lat,
		MAX(value) as max_lat
	FROM ai_event_log
	WHERE event_type = 'tool_call' AND created_at >= ?
	GROUP BY tool_name
	ORDER BY calls DESC`

	rows, err := r.exec.QueryContext(ctx, query, sinceTime)
	if err != nil {
		return nil, fmt.Errorf("query tool stats: %w", err)
	}
	defer rows.Close()

	var stats []ToolStats
	for rows.Next() {
		var s ToolStats
		var avgLat, maxLat sql.NullFloat64
		if err := rows.Scan(&s.ToolName, &s.Calls, &s.Errors, &avgLat, &maxLat); err != nil {
			return nil, fmt.Errorf("scan tool stats: %w", err)
		}
		if avgLat.Valid {
			s.AvgLatMs = avgLat.Float64 * 1000
		}
		if maxLat.Valid {
			s.MaxLatMs = maxLat.Float64 * 1000
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if stats == nil {
		stats = []ToolStats{}
	}
	return stats, nil
}

// ── 每日概览 ────────────────────────────────────────────

// DailySummary 单日 AI 使用概览。
type DailySummary struct {
	Date            string `json:"date"`
	LLMRequests     int64  `json:"llm_requests"`
	LLMErrors       int64  `json:"llm_errors"`
	ToolCalls       int64  `json:"tool_calls"`
	ToolErrors      int64  `json:"tool_errors"`
	LoopDetected    int64  `json:"loop_detected"`
	AvgRounds       float64 `json:"avg_rounds"`
}

// GetDailySummary 获取指定天数的每日概览。
func (r *AnalyticsRepository) GetDailySummary(ctx context.Context, days int) ([]DailySummary, error) {
	if days <= 0 {
		days = 7
	}
	sinceTime := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query := `SELECT
		DATE(created_at) as day,
		SUM(CASE WHEN event_type='llm_request' THEN 1 ELSE 0 END) as llm_requests,
		SUM(CASE WHEN event_type='llm_request' AND status='error' THEN 1 ELSE 0 END) as llm_errors,
		SUM(CASE WHEN event_type='tool_call' THEN 1 ELSE 0 END) as tool_calls,
		SUM(CASE WHEN event_type='tool_call' AND status='error' THEN 1 ELSE 0 END) as tool_errors,
		SUM(CASE WHEN event_type='loop_detected' THEN 1 ELSE 0 END) as loops
	FROM ai_event_log
	WHERE created_at >= ?
	GROUP BY DATE(created_at)
	ORDER BY day DESC`

	rows, err := r.exec.QueryContext(ctx, query, sinceTime)
	if err != nil {
		return nil, fmt.Errorf("query daily summary: %w", err)
	}
	defer rows.Close()

	var summaries []DailySummary
	for rows.Next() {
		var s DailySummary
		if err := rows.Scan(&s.Date, &s.LLMRequests, &s.LLMErrors,
			&s.ToolCalls, &s.ToolErrors, &s.LoopDetected); err != nil {
			return nil, fmt.Errorf("scan daily summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if summaries == nil {
		summaries = []DailySummary{}
	}
	return summaries, nil
}

// ── 清理 ────────────────────────────────────────────────

// CleanOldEvents 删除指定天数之前的事件日志。
func (r *AnalyticsRepository) CleanOldEvents(ctx context.Context, days int) (int64, error) {
	if days <= 0 {
		days = 7
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	result, err := r.exec.ExecContext(ctx, "DELETE FROM ai_event_log WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("clean ai_event_log: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// ── 指标快照持久化 ──────────────────────────────────────

// SaveHourlySnapshot 保存一个小时的聚合快照（幂等：存在则更新）。
func (r *AnalyticsRepository) SaveHourlySnapshot(ctx context.Context, hour string, snapJSON []byte) error {
	query := `INSERT INTO ai_metrics_hourly (hour, data) VALUES (?, ?)
		ON CONFLICT(hour) DO UPDATE SET data = excluded.data, updated_at = datetime('now')`
	_, err := r.exec.ExecContext(ctx, query, hour, string(snapJSON))
	if err != nil {
		// SQLite 的 UPSERT 语法不同，尝试 fallback
		return r.saveSnapshotFallback(ctx, hour, string(snapJSON))
	}
	return nil
}

func (r *AnalyticsRepository) saveSnapshotFallback(ctx context.Context, hour, data string) error {
	var exists int
	_ = r.exec.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_metrics_hourly WHERE hour = ?", hour).Scan(&exists)
	if exists > 0 {
		_, err := r.exec.ExecContext(ctx, "UPDATE ai_metrics_hourly SET data = ?, updated_at = datetime('now') WHERE hour = ?", data, hour)
		return err
	}
	_, err := r.exec.ExecContext(ctx, "INSERT INTO ai_metrics_hourly (hour, data) VALUES (?, ?)", hour, data)
	return err
}

// GetHourlySnapshots 获取指定时间范围的每小时快照。
func (r *AnalyticsRepository) GetHourlySnapshots(ctx context.Context, since time.Duration) ([]HourlySnapshot, error) {
	sinceTime := time.Now().Add(-since).Format("2006-01-02 15:04:05")

	rows, err := r.exec.QueryContext(ctx,
		"SELECT hour, data FROM ai_metrics_hourly WHERE hour >= ? ORDER BY hour ASC",
		sinceTime[:13], // 取到小时级别 "2026-05-30T14"
	)
	if err != nil {
		return nil, fmt.Errorf("query hourly snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []HourlySnapshot
	for rows.Next() {
		var s HourlySnapshot
		var raw string
		if err := rows.Scan(&s.Hour, &raw); err != nil {
			return nil, fmt.Errorf("scan hourly snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(raw), &s.Data); err != nil {
			s.Data = map[string]any{"raw": raw}
		}
		snaps = append(snaps, s)
	}
	if snaps == nil {
		snaps = []HourlySnapshot{}
	}
	return snaps, nil
}

// HourlySnapshot 每小时快照。
type HourlySnapshot struct {
	Hour string         `json:"hour"`
	Data map[string]any `json:"data"`
}
