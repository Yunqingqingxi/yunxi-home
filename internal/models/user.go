package models

import "time"

// UserRole 用户角色
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// User 系统用户
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // bcrypt hash
	Role         UserRole  `json:"role"`
	StorageQuota int64     `json:"storage_quota"` // 存储配额(字节), 0=无限
	StorageUsed  int64     `json:"storage_used"`  // 已用存储(字节)
	CreatedAt    time.Time `json:"created_at"`
}

// FilePermission 文件路径权限
type FilePermission struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Path      string    `json:"path"`      // 路径前缀 (如 /data/videos)
	CanRead   bool      `json:"can_read"`
	CanWrite  bool      `json:"can_write"`
	CanDelete bool      `json:"can_delete"`
	CanShare  bool      `json:"can_share"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FilePermMask compact permission bitmask for middleware
type FilePermMask struct {
	Read   bool
	Write  bool
	Delete bool
	Share  bool
}

// HasAccess returns true if any permission flag is set
func (m FilePermMask) HasAccess() bool {
	return m.Read || m.Write || m.Delete || m.Share
}
