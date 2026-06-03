// Package base 定义数据访问层的通用接口和类型，零外部依赖（仅 models + stdlib）。
// 所有数据仓库实现均基于此包的接口。
package base

import (
	"context"
	"database/sql"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
)

// Executor 是 Repository 所需的最小数据库操作集合。
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// DomainRepository 域名记录仓库接口
type DomainRepository interface {
	Create(ctx context.Context, rec *models.DomainRecord) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.DomainRecord, error)
	GetByDomain(ctx context.Context, domain, rr, recType string) (*models.DomainRecord, error)
	List(ctx context.Context) ([]models.DomainRecord, error)
	ListEnabled(ctx context.Context) ([]models.DomainRecord, error)
	Update(ctx context.Context, rec *models.DomainRecord) error
	UpdateValue(ctx context.Context, id int64, recordID, value string) error
	Delete(ctx context.Context, id int64) error
	Upsert(ctx context.Context, rec *models.DomainRecord) error
}

// ListParams 分页查询参数
type ListParams struct {
	Domain string // 按域名过滤，空则不过滤
	Status string // 按状态过滤，空则不过滤
	Page   int    // 页码，从 1 开始
	Size   int    // 每页条数
}

// ListResult 分页查询结果
type ListResult struct {
	Records []models.HistoryRecord `json:"records"`
	Total   int64                  `json:"total"`
	Page    int                    `json:"page"`
	Size    int                    `json:"size"`
	Domains []string               `json:"domains,omitempty"`
}

// HistoryRepository 历史记录仓库接口
type HistoryRepository interface {
	Create(ctx context.Context, rec *models.HistoryRecord) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.HistoryRecord, error)
	List(ctx context.Context, params ListParams) (*ListResult, error)
	CleanOld(ctx context.Context, days int) (int64, error)
	GetStats(ctx context.Context, days int) ([]HistoryStats, error)
}

// HistoryStats 每日聚合统计
type HistoryStats struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

// UserRepository 用户仓库接口
type UserRepository interface {
	Create(ctx context.Context, user *models.User) (int64, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByID(ctx context.Context, id int64) (*models.User, error)
	List(ctx context.Context) ([]models.User, error)
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	Delete(ctx context.Context, id int64) error
	UpdateRole(ctx context.Context, id int64, role string) error
	UpdateQuota(ctx context.Context, id int64, quota int64) error
	AddStorageUsed(ctx context.Context, id int64, delta int64) error
	InitDefaultAdmin(ctx context.Context, username, password string) error
}

// ChatSessionRepository 持久化 AI 聊天会话。
type ChatSessionRepository interface {
	List(ctx context.Context) ([]models.ChatSession, error)
	ListByType(ctx context.Context, sessionType string) ([]models.ChatSession, error)
	Upsert(ctx context.Context, s *models.ChatSession) error
	// UpdateSessionMeta updates only metadata fields (title, pinned) without touching messages.
	UpdateSessionMeta(ctx context.Context, id string, title *string, pinned *bool) error
	Delete(ctx context.Context, id string) error
	DeleteByType(ctx context.Context, sessionType string) (int64, error)
	DeleteStale(ctx context.Context, sessionType string, olderThan time.Duration) (int64, error)
	DeleteAll(ctx context.Context) error
}

// GoalRepository persists AI agent goals per session.
type GoalRepository interface {
	Upsert(ctx context.Context, sessionID string, goalsJSON string) error
	Get(ctx context.Context, sessionID string) (string, error)
	Delete(ctx context.Context, sessionID string) error
}

// TodoRepository persists AI todo items per session.
type TodoRepository interface {
	Upsert(ctx context.Context, sessionID string, itemsJSON string) error
	Get(ctx context.Context, sessionID string) (string, error)
	Delete(ctx context.Context, sessionID string) error
}

// ConfigRepository 配置存储接口 (数据库后端)
type ConfigRepository interface {
	// GetSection 返回某个配置段的 JSON 数据，不存在则返回空字符串。
	GetSection(ctx context.Context, section string) (string, error)
	// GetAll 返回所有配置段，key → JSON 数据。
	GetAll(ctx context.Context) (map[string]string, error)
	// SetSection 写入或更新某个配置段的 JSON 数据。
	SetSection(ctx context.Context, section, data string) error
	// InitDefaults 在表为空时写入默认配置段。
	InitDefaults(ctx context.Context, defaults map[string]string) error
}

// FilePermissionRepository 文件权限存储
type FilePermissionRepository interface {
	GetByUserAndPath(ctx context.Context, userID int64, filePath string) (*models.FilePermission, error)
	ListByUser(ctx context.Context, userID int64) ([]models.FilePermission, error)
	ListAll(ctx context.Context) ([]models.FilePermission, error)
	Upsert(ctx context.Context, p *models.FilePermission) error
	Delete(ctx context.Context, id int64) error
}

// PromptRecord 提示词记录（跨后端通用类型）
type PromptRecord struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // "general" | "specialized"
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Keywords  string    `json:"keywords"` // JSON array string
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PromptRepository 提示词仓库接口
type PromptRepository interface {
	GetByCategory(ctx context.Context, category string) ([]PromptRecord, error)
	GetAll(ctx context.Context) ([]PromptRecord, error)
	GetByID(ctx context.Context, id string) (*PromptRecord, error)
	Upsert(ctx context.Context, p *PromptRecord) error
	Delete(ctx context.Context, id string) error
	InitDefaults(ctx context.Context, prompts []PromptRecord) error
}

// ShareRepository 分享数据仓库
type ShareRepository interface {
	Create(ctx context.Context, share *nas.Share) (int64, error)
	GetByToken(ctx context.Context, token string) (*nas.Share, error)
	List(ctx context.Context, limit, offset int) ([]nas.Share, int64, error)
	Delete(ctx context.Context, id int64) error
	IncrementDownloads(ctx context.Context, id int64) error
	CleanExpired(ctx context.Context) (int64, error)
}
