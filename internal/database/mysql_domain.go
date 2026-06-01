package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yxd/yunxi-home/internal/models"
)

// MySQLDomainRepo implements DomainRepository for MySQL.
type MySQLDomainRepo struct{ db Executor }

func NewMySQLDomainRepo(db Executor) *MySQLDomainRepo { return &MySQLDomainRepo{db: db} }

var _ DomainRepository = (*MySQLDomainRepo)(nil)

func (r *MySQLDomainRepo) Create(ctx context.Context, rec *models.DomainRecord) (int64, error) {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO domains (domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.Domain, rec.RecordID, rec.RR, rec.Type, rec.Value, rec.TTL, boolToInt(rec.Enabled), rec.CronExpr, now, now)
	if err != nil {
		return 0, fmt.Errorf("create domain: %w", err)
	}
	return result.LastInsertId()
}

func (r *MySQLDomainRepo) GetByID(ctx context.Context, id int64) (*models.DomainRecord, error) {
	q := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at FROM domains WHERE id = ?`
	return r.scanOne(ctx, q, id)
}

func (r *MySQLDomainRepo) GetByDomain(ctx context.Context, domain, rr, recType string) (*models.DomainRecord, error) {
	q := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at
		FROM domains WHERE domain = ? AND rr = ? AND type = ?`
	return r.scanOne(ctx, q, domain, rr, recType)
}

func (r *MySQLDomainRepo) List(ctx context.Context) ([]models.DomainRecord, error) {
	q := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at FROM domains ORDER BY domain, rr`
	return r.scanMany(ctx, q)
}

func (r *MySQLDomainRepo) ListEnabled(ctx context.Context) ([]models.DomainRecord, error) {
	q := `SELECT id, domain, record_id, rr, type, value, ttl, enabled, cron_expr, created_at, updated_at FROM domains WHERE enabled = 1 ORDER BY domain, rr`
	return r.scanMany(ctx, q)
}

func (r *MySQLDomainRepo) Update(ctx context.Context, rec *models.DomainRecord) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE domains SET record_id=?, value=?, ttl=?, enabled=?, cron_expr=?, updated_at=? WHERE id=?`,
		rec.RecordID, rec.Value, rec.TTL, boolToInt(rec.Enabled), rec.CronExpr, time.Now(), rec.ID)
	if err != nil {
		return fmt.Errorf("update domain: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *MySQLDomainRepo) UpdateValue(ctx context.Context, id int64, recordID, value string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE domains SET record_id=?, value=?, updated_at=? WHERE id=?`, recordID, value, time.Now(), id)
	return err
}

func (r *MySQLDomainRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM domains WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *MySQLDomainRepo) Upsert(ctx context.Context, rec *models.DomainRecord) error {
	existing, err := r.GetByDomain(ctx, rec.Domain, rec.RR, rec.Type)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existing != nil {
		rec.ID = existing.ID
		return r.Update(ctx, rec)
	}
	_, err = r.Create(ctx, rec)
	return err
}

func (r *MySQLDomainRepo) scanOne(ctx context.Context, q string, args ...interface{}) (*models.DomainRecord, error) {
	row := r.db.QueryRowContext(ctx, q, args...)
	var rec models.DomainRecord
	var enabled int
	err := row.Scan(&rec.ID, &rec.Domain, &rec.RecordID, &rec.RR, &rec.Type, &rec.Value, &rec.TTL, &enabled, &rec.CronExpr, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan domain: %w", err)
	}
	rec.Enabled = enabled == 1
	return &rec, nil
}

func (r *MySQLDomainRepo) scanMany(ctx context.Context, q string, args ...interface{}) ([]models.DomainRecord, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query domains: %w", err)
	}
	defer rows.Close()
	var recs []models.DomainRecord
	for rows.Next() {
		var rec models.DomainRecord
		var enabled int
		if err := rows.Scan(&rec.ID, &rec.Domain, &rec.RecordID, &rec.RR, &rec.Type, &rec.Value, &rec.TTL, &enabled, &rec.CronExpr, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		rec.Enabled = enabled == 1
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}
