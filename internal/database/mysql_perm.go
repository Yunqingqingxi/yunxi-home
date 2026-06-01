package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// MySQLFilePermRepo implements FilePermissionRepository for MySQL.
type MySQLFilePermRepo struct{ db Executor }

func NewMySQLFilePermRepo(db Executor) *MySQLFilePermRepo { return &MySQLFilePermRepo{db: db} }

var _ FilePermissionRepository = (*MySQLFilePermRepo)(nil)

func (r *MySQLFilePermRepo) GetByUserAndPath(ctx context.Context, userID int64, filePath string) (*models.FilePermission, error) {
	q := `SELECT id, user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at
		FROM file_permissions WHERE user_id = ? AND ? LIKE CONCAT(path, '%')
		ORDER BY LENGTH(path) DESC LIMIT 1`
	row := r.db.QueryRowContext(ctx, q, userID, filePath)
	var p models.FilePermission
	err := row.Scan(&p.ID, &p.UserID, &p.Path, &p.CanRead, &p.CanWrite, &p.CanDelete, &p.CanShare, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query file_permissions: %w", err)
	}
	return &p, nil
}

func (r *MySQLFilePermRepo) ListByUser(ctx context.Context, userID int64) ([]models.FilePermission, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at FROM file_permissions WHERE user_id = ? ORDER BY path`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []models.FilePermission
	for rows.Next() {
		var p models.FilePermission
		if err := rows.Scan(&p.ID, &p.UserID, &p.Path, &p.CanRead, &p.CanWrite, &p.CanDelete, &p.CanShare, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *MySQLFilePermRepo) ListAll(ctx context.Context) ([]models.FilePermission, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT fp.id, fp.user_id, fp.path, fp.can_read, fp.can_write, fp.can_delete, fp.can_share, fp.created_at, fp.updated_at, u.username
		 FROM file_permissions fp LEFT JOIN users u ON u.id = fp.user_id ORDER BY u.username, fp.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []models.FilePermission
	for rows.Next() {
		var p models.FilePermission
		var username sql.NullString
		if err := rows.Scan(&p.ID, &p.UserID, &p.Path, &p.CanRead, &p.CanWrite, &p.CanDelete, &p.CanShare, &p.CreatedAt, &p.UpdatedAt, &username); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *MySQLFilePermRepo) Upsert(ctx context.Context, p *models.FilePermission) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO file_permissions (user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE can_read=VALUES(can_read), can_write=VALUES(can_write),
		 can_delete=VALUES(can_delete), can_share=VALUES(can_share), updated_at=VALUES(updated_at)`,
		p.UserID, p.Path, p.CanRead, p.CanWrite, p.CanDelete, p.CanShare, now, now)
	if err != nil {
		return fmt.Errorf("upsert file_permission: %w", err)
	}
	return nil
}

func (r *MySQLFilePermRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM file_permissions WHERE id=?", id)
	return err
}
