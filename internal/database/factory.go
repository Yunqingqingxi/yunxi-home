package database

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"

	"golang.org/x/crypto/bcrypt"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

var log = logger.ForComponent("database")

// Backend bundles all repository instances created for a storage driver.
type Backend struct {
	DomainRepo   DomainRepository
	HistoryRepo  HistoryRepository
	UserRepo     UserRepository
	ChatRepo     ChatSessionRepository
	ConfigRepo   ConfigRepository
	FilePermRepo FilePermissionRepository
	ShareRepo    ShareRepository
	GoalRepo     GoalRepository
	TodoRepo     TodoRepository
	PromptRepo   PromptRepository
	SQLiteDB     *DB // non-nil only for sqlite driver, for config storage
	Close        func() error
	Driver       string
}

// BackendConfig is the configuration for creating a Backend, without importing config package.
type BackendConfig struct {
	Driver   string // "sqlite" | "mysql" | "file"
	Path     string // sqlite: db file path, file: data directory
	MySQLCfg *MySQLConfig
}

// NewBackend creates a Backend based on cfg.Driver.
func NewBackend(cfg BackendConfig) (*Backend, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite"
	}

	switch driver {
	case "sqlite":
		return newSQLiteBackend(cfg)
	case "mysql":
		return newMySQLBackend(cfg)
	case "file":
		return newFileBackend(cfg)
	default:
		return nil, fmt.Errorf("不支持的存储驱动: %s (支持: sqlite, mysql, file)", driver)
	}
}

// NewSQLiteBackendWithDB creates a sqlite backend reusing an existing *DB connection.
func NewSQLiteBackendWithDB(db *DB) *Backend {
	return &Backend{
		DomainRepo:   NewDomainRepo(db),
		HistoryRepo:  NewHistoryRepo(db),
		UserRepo:     NewUserRepo(db),
		ChatRepo:     NewChatSessionRepo(db),
		ConfigRepo:   NewConfigRepo(db),
		FilePermRepo: NewFilePermRepo(db),
		ShareRepo:    NewShareRepo(db.DB),
		GoalRepo:     &sqliteGoalRepo{db: db},
		TodoRepo:     &sqliteTodoRepo{db: db},
		PromptRepo:   NewPromptRepo(db),
		SQLiteDB:     db,
		Close:        func() error { return nil },
		Driver:       "sqlite",
	}
}

// ── SQLite backend ─────────────────────────────────────────────────

func newSQLiteBackend(cfg BackendConfig) (*Backend, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("sqlite 模式下必须配置 path")
	}

	db, err := New(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}

	return &Backend{
		DomainRepo:   NewDomainRepo(db),
		HistoryRepo:  NewHistoryRepo(db),
		UserRepo:     NewUserRepo(db),
		ChatRepo:     NewChatSessionRepo(db),
		ConfigRepo:   NewConfigRepo(db),
		FilePermRepo: NewFilePermRepo(db),
		ShareRepo:    NewShareRepo(db.DB),
		GoalRepo:     &sqliteGoalRepo{db: db},
		TodoRepo:     &sqliteTodoRepo{db: db},
		PromptRepo:   NewPromptRepo(db),
		SQLiteDB:     db,
		Close:        db.Close,
		Driver:       "sqlite",
	}, nil
}

// ── MySQL backend ──────────────────────────────────────────────────

func newMySQLBackend(cfg BackendConfig) (*Backend, error) {
	if cfg.MySQLCfg == nil || cfg.MySQLCfg.Host == "" {
		return nil, fmt.Errorf("mysql 模式下必须配置 mysql.host")
	}

	db, err := NewMySQL(*cfg.MySQLCfg)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	return &Backend{
		DomainRepo:   NewMySQLDomainRepo(db),
		HistoryRepo:  NewMySQLHistoryRepo(db),
		UserRepo:     NewMySQLFullUserRepo(db),
		ChatRepo:     NewMySQLChatSessionRepo(db),
		ConfigRepo:   NewMySQLConfigRepo(db),
		FilePermRepo: NewMySQLFilePermRepo(db),
		ShareRepo:    NewMySQLShareRepo(db),
		GoalRepo:     &mysqlGoalRepo{db: db},
		TodoRepo:     &mysqlTodoRepo{db: db},
		PromptRepo:   NewMySQLPromptRepo(db),
		Close:        db.Close,
		Driver:       "mysql",
	}, nil
}

// ── File backend ───────────────────────────────────────────────────

func newFileBackend(cfg BackendConfig) (*Backend, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("file 模式下必须配置 path")
	}

	store := NewFileStore(cfg.Path)

	domainRepo := NewFileDomainRepo(store)
	historyRepo := NewFileHistoryRepo(store)
	userRepo := NewFileUserRepo(store)

	fileInitDefaultAdmin(userRepo)

	return &Backend{
		DomainRepo:   domainRepo,
		HistoryRepo:  historyRepo,
		UserRepo:     userRepo,
		ChatRepo:     NewFileChatRepo(store),
		ConfigRepo:   NewFileConfigRepo(store),
		FilePermRepo: NewFileFilePermRepo(store),
		ShareRepo:    NewFileShareRepo(store),
		GoalRepo:     &fileGoalRepo{store: store},
		TodoRepo:     &fileTodoRepo{store: store},
		PromptRepo:   nil, // file backend doesn't support prompt management
		Close:        func() error { return nil },
		Driver:       "file",
	}, nil
}

func fileInitDefaultAdmin(repo UserRepository) {
	ctx := context.Background()
	users, err := repo.List(ctx)
	if err != nil || len(users) > 0 {
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	repo.Create(ctx, &models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         models.RoleAdmin,
	})
	log.Info("file 后端已创建默认管理员用户")
}
