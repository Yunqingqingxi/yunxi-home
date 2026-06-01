package database

import (
	"context"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// MySQLHistoryRepo implements HistoryRepository for MySQL.
type MySQLHistoryRepo struct{ db Executor }

func NewMySQLHistoryRepo(db Executor) *MySQLHistoryRepo { return &MySQLHistoryRepo{db: db} }

var _ HistoryRepository = (*MySQLHistoryRepo)(nil)

func (r *MySQLHistoryRepo) Create(ctx context.Context, rec *models.HistoryRecord) (int64, error) {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO histories (domain, rr, old_ip, new_ip, type, status, error_msg, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Domain, rec.RR, rec.OldIP, rec.NewIP, rec.Type, rec.Status, rec.ErrorMsg, now)
	if err != nil {
		return 0, fmt.Errorf("create history: %w", err)
	}
	return result.LastInsertId()
}

func (r *MySQLHistoryRepo) GetByID(ctx context.Context, id int64) (*models.HistoryRecord, error) {
	q := `SELECT id, domain, rr, old_ip, new_ip, type, status, error_msg, created_at FROM histories WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	var rec models.HistoryRecord
	err := row.Scan(&rec.ID, &rec.Domain, &rec.RR, &rec.OldIP, &rec.NewIP, &rec.Type, &rec.Status, &rec.ErrorMsg, &rec.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *MySQLHistoryRepo) List(ctx context.Context, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Size < 1 || params.Size > 100 {
		params.Size = 20
	}

	// Count
	var total int64
	countQ := "SELECT COUNT(*) FROM histories"
	args := []interface{}{}
	if params.Domain != "" {
		countQ += " WHERE domain = ?"
		args = append(args, params.Domain)
	}
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count histories: %w", err)
	}

	// Data
	offset := (params.Page - 1) * params.Size
	dataQ := `SELECT id, domain, rr, old_ip, new_ip, type, status, error_msg, created_at FROM histories`
	dataArgs := []interface{}{}
	if params.Domain != "" {
		dataQ += " WHERE domain = ?"
		dataArgs = append(dataArgs, params.Domain)
	}
	dataQ += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, params.Size, offset)

	rows, err := r.db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("list histories: %w", err)
	}
	defer rows.Close()

	var records []models.HistoryRecord
	for rows.Next() {
		var rec models.HistoryRecord
		if err := rows.Scan(&rec.ID, &rec.Domain, &rec.RR, &rec.OldIP, &rec.NewIP, &rec.Type, &rec.Status, &rec.ErrorMsg, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		records = append(records, rec)
	}
	return &ListResult{Records: records, Total: total, Page: params.Page, Size: params.Size}, rows.Err()
}

func (r *MySQLHistoryRepo) GetStats(ctx context.Context, days int) ([]HistoryStats, error) {
	q := `SELECT DATE(created_at) as date, COUNT(*) as total,
		SUM(CASE WHEN status='success' THEN 1 ELSE 0 END) as success,
		SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) as failed
		FROM histories WHERE created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
		GROUP BY DATE(created_at) ORDER BY date ASC`

	rows, err := r.db.QueryContext(ctx, q, days)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()
	var stats []HistoryStats
	for rows.Next() {
		var s HistoryStats
		if err := rows.Scan(&s.Date, &s.Total, &s.Success, &s.Failed); err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}
		stats = append(stats, s)
	}
	if stats == nil {
		stats = []HistoryStats{}
	}
	return stats, rows.Err()
}

func (r *MySQLHistoryRepo) CleanOld(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result, err := r.db.ExecContext(ctx, "DELETE FROM histories WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("clean histories: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
