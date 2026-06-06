package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// MySQLFullUserRepo implements full UserRepository for MySQL (primary mode).
type MySQLFullUserRepo struct{ db Executor }

func NewMySQLFullUserRepo(db Executor) *MySQLFullUserRepo { return &MySQLFullUserRepo{db: db} }

var _ UserRepository = (*MySQLFullUserRepo)(nil)

func (r *MySQLFullUserRepo) Create(ctx context.Context, user *models.User) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, storage_quota, storage_used, created_at) VALUES (?, ?, ?, ?, ?, NOW())`,
		user.Username, user.PasswordHash, string(user.Role), user.StorageQuota, user.StorageUsed)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return result.LastInsertId()
}

func (r *MySQLFullUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	q := `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users WHERE username = ?`
	row := r.db.QueryRowContext(ctx, q, username)
	var u models.User
	var role string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.StorageQuota, &u.StorageUsed, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	u.Role = models.UserRole(role)
	return &u, nil
}

func (r *MySQLFullUserRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	q := `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)
	var u models.User
	var role string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.StorageQuota, &u.StorageUsed, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	u.Role = models.UserRole(role)
	return &u, nil
}

func (r *MySQLFullUserRepo) List(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		var role string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.StorageQuota, &u.StorageUsed, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.Role = models.UserRole(role)
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *MySQLFullUserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	result, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash=? WHERE id=?", passwordHash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *MySQLFullUserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id=?", id)
	return err
}

func (r *MySQLFullUserRepo) UpdateRole(ctx context.Context, id int64, role string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET role=? WHERE id=?", role, id)
	return err
}

func (r *MySQLFullUserRepo) UpdateQuota(ctx context.Context, id int64, quota int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET storage_quota=? WHERE id=?", quota, id)
	return err
}

func (r *MySQLFullUserRepo) AddStorageUsed(ctx context.Context, id int64, delta int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET storage_used=storage_used+? WHERE id=?", delta, id)
	return err
}

func (r *MySQLFullUserRepo) InitDefaultAdmin(ctx context.Context, username, password string) error {
	users, err := r.List(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("generate hash: %w", err)
	}
	_, err = r.Create(ctx, &models.User{Username: username, PasswordHash: string(hash), Role: models.RoleAdmin})
	if err != nil {
		return fmt.Errorf("create default admin: %w", err)
	}
	return nil
}
