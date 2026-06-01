package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// HistoryRepo 更新历史记录仓库
type HistoryRepo struct {
	db Executor
}

// Ensure HistoryRepo implements HistoryRepository
var _ HistoryRepository = (*HistoryRepo)(nil)

// NewHistoryRepo 创建历史记录仓库
func NewHistoryRepo(db Executor) *HistoryRepo {
	return &HistoryRepo{db: db}
}

// Create 创建历史记录
func (r *HistoryRepo) Create(ctx context.Context, rec *models.HistoryRecord) (int64, error) {
	query := `INSERT INTO histories (domain, rr, old_ip, new_ip, type, status, error_msg, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query,
		rec.Domain, rec.RR, rec.OldIP, rec.NewIP, rec.Type, rec.Status, rec.ErrorMsg, now,
	)
	if err != nil {
		return 0, fmt.Errorf("创建历史记录失败: %w", err)
	}
	return result.LastInsertId()
}

// GetByID 根据 ID 获取记录
func (r *HistoryRepo) GetByID(ctx context.Context, id int64) (*models.HistoryRecord, error) {
	query := `SELECT id, domain, rr, old_ip, new_ip, type, status, error_msg, created_at FROM histories WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)

	var rec models.HistoryRecord
	err := row.Scan(&rec.ID, &rec.Domain, &rec.RR, &rec.OldIP, &rec.NewIP, &rec.Type, &rec.Status, &rec.ErrorMsg, &rec.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("查询历史记录失败: %w", err)
	}
	return &rec, nil
}

// List 分页查询历史记录
func (r *HistoryRepo) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Size < 1 || params.Size > 100 {
		params.Size = 20
	}

	// 查询总数
	var total int64
	countQuery := "SELECT COUNT(*) FROM histories"
	args := []interface{}{}
	if params.Domain != "" {
		countQuery += " WHERE domain = ?"
		args = append(args, params.Domain)
	}
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("查询历史记录总数失败: %w", err)
	}

	// 查询数据
	offset := (params.Page - 1) * params.Size
	dataQuery := "SELECT id, domain, rr, old_ip, new_ip, type, status, error_msg, created_at FROM histories"
	dataArgs := []interface{}{}
	if params.Domain != "" {
		dataQuery += " WHERE domain = ?"
		dataArgs = append(dataArgs, params.Domain)
	}
	dataQuery += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, params.Size, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询历史记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []models.HistoryRecord
	for rows.Next() {
		var rec models.HistoryRecord
		if err := rows.Scan(&rec.ID, &rec.Domain, &rec.RR, &rec.OldIP, &rec.NewIP, &rec.Type, &rec.Status, &rec.ErrorMsg, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描历史记录失败: %w", err)
		}
		records = append(records, rec)
	}

	return &ListResult{
		Records: records,
		Total:   total,
		Page:    params.Page,
		Size:    params.Size,
	}, rows.Err()
}

// GetStats 获取最近 N 天的每日统计
func (r *HistoryRepo) GetStats(ctx context.Context, days int) ([]HistoryStats, error) {
	query := `SELECT DATE(created_at) as date, COUNT(*) as total,
		SUM(CASE WHEN status='success' THEN 1 ELSE 0 END) as success,
		SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) as failed
		FROM histories WHERE created_at >= datetime('now', '-' || ? || ' days')
		GROUP BY DATE(created_at) ORDER BY date ASC`

	rows, err := r.db.QueryContext(ctx, query, days)
	if err != nil { return nil, fmt.Errorf("query stats: %w", err) }
	defer rows.Close()

	var stats []HistoryStats
	for rows.Next() {
		var s HistoryStats
		if err := rows.Scan(&s.Date, &s.Total, &s.Success, &s.Failed); err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}
		stats = append(stats, s)
	}
	if stats == nil { stats = []HistoryStats{} }
	return stats, rows.Err()
}

// CleanOld 清理超过 days 天的旧记录
func (r *HistoryRepo) CleanOld(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result, err := r.db.ExecContext(ctx, "DELETE FROM histories WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("清理旧历史记录失败: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
