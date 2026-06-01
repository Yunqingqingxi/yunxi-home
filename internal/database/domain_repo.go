package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yxd/yunxi-home/internal/models"
)

// DomainRepo 域名记录仓库
type DomainRepo struct {
	db Executor
}

// NewDomainRepo 创建域名记录仓库
func NewDomainRepo(db Executor) *DomainRepo {
	return &DomainRepo{db: db}
}

// Ensure DomainRepo implements DomainRepository
var _ DomainRepository = (*DomainRepo)(nil)

// Create 创建域名记录
func (r *DomainRepo) Create(ctx context.Context, rec *models.DomainRecord) (int64, error) {
	query := `INSERT INTO domains (domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query,
		rec.Domain, rec.RecordID, rec.RR, rec.Type, rec.Value,
		rec.TTL, boolToInt(rec.Enabled), rec.CronExpr, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("创建域名记录失败: %w", err)
	}
	return result.LastInsertId()
}

// GetByID 根据 ID 获取记录
func (r *DomainRepo) GetByID(ctx context.Context, id int64) (*models.DomainRecord, error) {
	query := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at FROM domains WHERE id = ?`
	return r.scanOne(ctx, query, id)
}

// GetByDomain 根据域名、RR、类型获取记录
func (r *DomainRepo) GetByDomain(ctx context.Context, domain, rr, recType string) (*models.DomainRecord, error) {
	query := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at
		FROM domains WHERE domain = ? AND rr = ? AND type = ?`
	return r.scanOne(ctx, query, domain, rr, recType)
}

// List 列出所有域名记录
func (r *DomainRepo) List(ctx context.Context) ([]models.DomainRecord, error) {
	query := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at
		FROM domains ORDER BY domain, rr`
	return r.scanMany(ctx, query)
}

// ListEnabled 列出所有已启用的记录
func (r *DomainRepo) ListEnabled(ctx context.Context) ([]models.DomainRecord, error) {
	query := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at
		FROM domains WHERE enabled = 1 ORDER BY domain, rr`
	return r.scanMany(ctx, query)
}

// Update 更新域名记录
func (r *DomainRepo) Update(ctx context.Context, rec *models.DomainRecord) error {
	query := `UPDATE domains SET record_id=?, value=?, ttl=?, enabled=?, cron_expr=?, updated_at=? WHERE id=?`

	result, err := r.db.ExecContext(ctx, query,
		rec.RecordID, rec.Value, rec.TTL, boolToInt(rec.Enabled), rec.CronExpr, time.Now(), rec.ID,
	)
	if err != nil {
		return fmt.Errorf("更新域名记录失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateValue 仅更新 IP 值（高频操作）
func (r *DomainRepo) UpdateValue(ctx context.Context, id int64, recordID, value string) error {
	query := `UPDATE domains SET record_id=?, value=?, updated_at=? WHERE id=?`

	_, err := r.db.ExecContext(ctx, query, recordID, value, time.Now(), id)
	if err != nil {
		return fmt.Errorf("更新 IP 值失败: %w", err)
	}
	return nil
}

// Delete 删除域名记录
func (r *DomainRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM domains WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("删除域名记录失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Upsert 插入或更新域名记录
func (r *DomainRepo) Upsert(ctx context.Context, rec *models.DomainRecord) error {
	existing, err := r.GetByDomain(ctx, rec.Domain, rec.RR, rec.Type)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existing != nil {
		rec.ID = existing.ID
		rec.CreatedAt = existing.CreatedAt
		return r.Update(ctx, rec)
	}
	_, err = r.Create(ctx, rec)
	return err
}

func (r *DomainRepo) scanOne(ctx context.Context, query string, args ...interface{}) (*models.DomainRecord, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	var rec models.DomainRecord
	var enabled int
	err := row.Scan(&rec.ID, &rec.Domain, &rec.RecordID, &rec.RR, &rec.Type,
		&rec.Value, &rec.TTL, &enabled, &rec.CronExpr, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("查询域名记录失败: %w", err)
	}
	rec.Enabled = enabled == 1
	return &rec, nil
}

func (r *DomainRepo) scanMany(ctx context.Context, query string, args ...interface{}) ([]models.DomainRecord, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询域名记录列表失败: %w", err)
	}
	defer rows.Close()

	var records []models.DomainRecord
	for rows.Next() {
		var rec models.DomainRecord
		var enabled int
		if err := rows.Scan(&rec.ID, &rec.Domain, &rec.RecordID, &rec.RR, &rec.Type,
			&rec.Value, &rec.TTL, &enabled, &rec.CronExpr, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("扫描域名记录失败: %w", err)
		}
		rec.Enabled = enabled == 1
		records = append(records, rec)
	}
	return records, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
