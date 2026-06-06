package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// UserRepo 用户仓库
type UserRepo struct {
	db Executor
}

// NewUserRepo 创建用户仓库
func NewUserRepo(db Executor) *UserRepo {
	return &UserRepo{db: db}
}

// Ensure UserRepo implements UserRepository
var _ UserRepository = (*UserRepo)(nil)

// Create 创建用户
func (r *UserRepo) Create(ctx context.Context, user *models.User) (int64, error) {
	query := `INSERT INTO users (username, password_hash, role, storage_quota, storage_used, created_at) VALUES (?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, user.Username, user.PasswordHash, string(user.Role), user.StorageQuota, user.StorageUsed, now)
	if err != nil {
		return 0, fmt.Errorf("创建用户失败: %w", err)
	}
	return result.LastInsertId()
}

// GetByUsername 根据用户名查找用户
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users WHERE username = ?`

	row := r.db.QueryRowContext(ctx, query, username)

	var user models.User
	var role string
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &role, &user.StorageQuota, &user.StorageUsed, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	user.Role = models.UserRole(role)
	return &user, nil
}

// GetByID 根据 ID 查找用户
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)

	var user models.User
	var role string
	err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &role, &user.StorageQuota, &user.StorageUsed, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	user.Role = models.UserRole(role)
	return &user, nil
}

// List 列出所有用户
func (r *UserRepo) List(ctx context.Context) ([]models.User, error) {
	query := `SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var role string
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &role, &user.StorageQuota, &user.StorageUsed, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描用户失败: %w", err)
		}
		user.Role = models.UserRole(role)
		users = append(users, user)
	}
	return users, rows.Err()
}

// UpdatePassword 更新用户密码
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	result, err := r.db.ExecContext(ctx, "UPDATE users SET password_hash=? WHERE id=?", passwordHash, id)
	if err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete 删除用户
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id=?", id)
	return err
}

// UpdateRole 更新用户角色
func (r *UserRepo) UpdateRole(ctx context.Context, id int64, role string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET role=? WHERE id=?", role, id)
	return err
}

// UpdateQuota 更新用户存储配额
func (r *UserRepo) UpdateQuota(ctx context.Context, id int64, quota int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET storage_quota=? WHERE id=?", quota, id)
	return err
}

// AddStorageUsed 增加用户已用存储 (delta 可为负数表示减少)
func (r *UserRepo) AddStorageUsed(ctx context.Context, id int64, delta int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET storage_used=storage_used+? WHERE id=?", delta, id)
	return err
}

// InitDefaultAdmin 初始化默认管理员账户（首次运行时）
func (r *UserRepo) InitDefaultAdmin(ctx context.Context, username, password string) error {
	// 检查是否已存在用户
	users, err := r.List(ctx)
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil // 已有用户，跳过初始化
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("生成密码哈希失败: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	}

	_, err = r.Create(ctx, user)
	if err != nil {
		return fmt.Errorf("创建默认管理员失败: %w", err)
	}

	return nil
}

// MySQLUserRepo implements UserRepository for MySQL (sync target only).
type MySQLUserRepo struct {
	db Executor
}

func NewMySQLUserRepo(db Executor) *MySQLUserRepo { return &MySQLUserRepo{db: db} }

func (r *MySQLUserRepo) List(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, username, password_hash, role, storage_quota, storage_used, created_at FROM users ORDER BY id")
	if err != nil { return nil, err }
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User; var role string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.StorageQuota, &u.StorageUsed, &u.CreatedAt); err != nil { return nil, err }
		u.Role = models.UserRole(role)
		users = append(users, u)
	}
	return users, nil
}

func (r *MySQLUserRepo) Upsert(ctx context.Context, u *models.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, role, storage_quota, storage_used, created_at)
		 VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE username=VALUES(username),password_hash=VALUES(password_hash),role=VALUES(role),storage_quota=VALUES(storage_quota),storage_used=VALUES(storage_used)`,
		u.ID, u.Username, u.PasswordHash, string(u.Role), u.StorageQuota, u.StorageUsed, u.CreatedAt)
	return err
}

func (r *MySQLUserRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id=?", id)
	return err
}
