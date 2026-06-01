package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// FilePermRepo file permission repository
type FilePermRepo struct {
	db Executor
}

func NewFilePermRepo(db Executor) *FilePermRepo {
	return &FilePermRepo{db: db}
}

var _ FilePermissionRepository = (*FilePermRepo)(nil)

// GetByUserAndPath returns the best-matching permission for user+path (longest prefix match)
func (r *FilePermRepo) GetByUserAndPath(ctx context.Context, userID int64, filePath string) (*models.FilePermission, error) {
	query := `SELECT id, user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at
		FROM file_permissions WHERE user_id = ? AND ? LIKE (path || '%')
		ORDER BY LENGTH(path) DESC LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, userID, filePath)

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

// ListByUser lists all permissions for a user
func (r *FilePermRepo) ListByUser(ctx context.Context, userID int64) ([]models.FilePermission, error) {
	query := `SELECT id, user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at
		FROM file_permissions WHERE user_id = ? ORDER BY path`
	rows, err := r.db.QueryContext(ctx, query, userID)
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

// ListAll lists all permissions (admin)
func (r *FilePermRepo) ListAll(ctx context.Context) ([]models.FilePermission, error) {
	query := `SELECT fp.id, fp.user_id, fp.path, fp.can_read, fp.can_write, fp.can_delete, fp.can_share, fp.created_at, fp.updated_at, u.username
		FROM file_permissions fp LEFT JOIN users u ON u.id = fp.user_id ORDER BY u.username, fp.path`
	rows, err := r.db.QueryContext(ctx, query)
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

// Upsert creates or updates a file permission
func (r *FilePermRepo) Upsert(ctx context.Context, p *models.FilePermission) error {
	now := time.Now()
	query := `INSERT INTO file_permissions (user_id, path, can_read, can_write, can_delete, can_share, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, path) DO UPDATE SET
			can_read=excluded.can_read, can_write=excluded.can_write,
			can_delete=excluded.can_delete, can_share=excluded.can_share,
			updated_at=excluded.updated_at`
	_, err := r.db.ExecContext(ctx, query, p.UserID, p.Path, p.CanRead, p.CanWrite, p.CanDelete, p.CanShare, now, now)
	if err != nil {
		return fmt.Errorf("upsert file_permission: %w", err)
	}
	return nil
}

// Delete removes a file permission
func (r *FilePermRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM file_permissions WHERE id=?", id)
	return err
}

// GetUserHomePerm returns the implicit home-directory permission mask
// Users always have full access to /home/{username}/
func GetUserHomePerm(username string, filePath string) *models.FilePermMask {
	homePrefix := "/home/" + username + "/"
	if len(filePath) >= len(homePrefix) && filePath[:len(homePrefix)] == homePrefix {
		return &models.FilePermMask{Read: true, Write: true, Delete: true, Share: true}
	}
	return nil
}