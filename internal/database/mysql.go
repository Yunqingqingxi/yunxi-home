package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfig MySQL 连接配置
type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

// NewMySQL 创建 MySQL 连接并执行迁移
func NewMySQL(cfg MySQLConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&timeout=5s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("MySQL 连接失败: %w", err)
	}

	if err := migrateMySQL(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("MySQL 迁移失败: %w", err)
	}

	return db, nil
}

func migrateMySQL(db *sql.DB) error {
tables := []string{
`CREATE TABLE IF NOT EXISTS domains (
id         BIGINT AUTO_INCREMENT PRIMARY KEY,
domain     VARCHAR(255) NOT NULL,
record_id  VARCHAR(64) NOT NULL DEFAULT '',
rr         VARCHAR(64) NOT NULL,
type       VARCHAR(10) NOT NULL,
value      VARCHAR(512) NOT NULL DEFAULT '',
ttl        INT NOT NULL DEFAULT 600,
enabled    TINYINT(1) NOT NULL DEFAULT 1,
cron_expr  VARCHAR(64) NOT NULL DEFAULT '*/5 * * * *',
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
UNIQUE KEY uk_domain_rr_type (domain, rr, type),
INDEX idx_domains_domain (domain)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE TABLE IF NOT EXISTS histories (
id         BIGINT AUTO_INCREMENT PRIMARY KEY,
domain     VARCHAR(255) NOT NULL,
rr         VARCHAR(64) NOT NULL DEFAULT '',
old_ip     VARCHAR(128) NOT NULL DEFAULT '',
new_ip     VARCHAR(128) NOT NULL DEFAULT '',
type       VARCHAR(10) NOT NULL DEFAULT 'AAAA',
status     VARCHAR(10) NOT NULL DEFAULT 'success',
error_msg  TEXT NOT NULL,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
INDEX idx_histories_domain (domain),
INDEX idx_histories_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE TABLE IF NOT EXISTS config (
section    VARCHAR(64) PRIMARY KEY,
data       TEXT NOT NULL,
updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE TABLE IF NOT EXISTS chat_sessions (
id         VARCHAR(128) PRIMARY KEY,
type       VARCHAR(16) NOT NULL DEFAULT 'chat',
title      VARCHAR(255) NOT NULL DEFAULT '',
messages   MEDIUMTEXT NOT NULL,
pinned     TINYINT(1) NOT NULL DEFAULT 0,
created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE INDEX IF NOT EXISTS idx_chat_sessions_type ON chat_sessions(type)`,

			`CREATE TABLE IF NOT EXISTS users (
 id            BIGINT AUTO_INCREMENT PRIMARY KEY,
username      VARCHAR(64) NOT NULL UNIQUE,
password_hash VARCHAR(255) NOT NULL,
role          VARCHAR(16) NOT NULL DEFAULT 'admin',
storage_quota BIGINT NOT NULL DEFAULT 0,
storage_used  BIGINT NOT NULL DEFAULT 0,
created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE INDEX idx_users_username (username)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE TABLE IF NOT EXISTS file_permissions (
id          BIGINT AUTO_INCREMENT PRIMARY KEY,
user_id     BIGINT NOT NULL,
path        VARCHAR(512) NOT NULL,
can_read    TINYINT(1) NOT NULL DEFAULT 1,
can_write   TINYINT(1) NOT NULL DEFAULT 0,
can_delete  TINYINT(1) NOT NULL DEFAULT 0,
can_share   TINYINT(1) NOT NULL DEFAULT 0,
created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
UNIQUE INDEX idx_fp_user_path (user_id, path),
INDEX idx_fp_user_id (user_id),
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

`CREATE TABLE IF NOT EXISTS shares (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
				token      VARCHAR(64) NOT NULL UNIQUE,
				file_path  VARCHAR(1024) NOT NULL,
				password   VARCHAR(255) NOT NULL DEFAULT '',
				expires_at DATETIME,
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				downloads  BIGINT NOT NULL DEFAULT 0,
				UNIQUE INDEX idx_shares_token (token)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

			`CREATE TABLE IF NOT EXISTS session_goals (
		id          VARCHAR(128) PRIMARY KEY,
		session_id  VARCHAR(128) NOT NULL,
		title       VARCHAR(255) NOT NULL DEFAULT '',
		steps_json  TEXT NOT NULL,
		status      VARCHAR(16) NOT NULL DEFAULT 'active',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_session_goals_session (session_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

	`CREATE TABLE IF NOT EXISTS session_todos (
		session_id  VARCHAR(128) PRIMARY KEY,
		items_json  TEXT NOT NULL,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

			// ── AI 监控表 ────────────────────────────────

		`CREATE TABLE IF NOT EXISTS ai_event_log (
			id          BIGINT AUTO_INCREMENT PRIMARY KEY,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			session_id  VARCHAR(128) NOT NULL DEFAULT '',
			event_type  VARCHAR(32) NOT NULL,
			tool_name   VARCHAR(64) NOT NULL DEFAULT '',
			model       VARCHAR(64) NOT NULL DEFAULT '',
			status      VARCHAR(16) NOT NULL DEFAULT 'success',
			value       DOUBLE NOT NULL DEFAULT 0,
			labels_json TEXT NOT NULL,
			extra_json  TEXT NOT NULL,
			INDEX idx_ai_event_type (event_type),
			INDEX idx_ai_event_tool (tool_name),
			INDEX idx_ai_event_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS ai_metrics_hourly (
			hour       VARCHAR(16) PRIMARY KEY,
			data       MEDIUMTEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// ALTER: 补充旧表缺失列 (兼容不同版本数据库)
			`ALTER TABLE users ADD COLUMN storage_quota BIGINT NOT NULL DEFAULT 0`,
			`ALTER TABLE users ADD COLUMN storage_used BIGINT NOT NULL DEFAULT 0`,
			`ALTER TABLE histories ADD COLUMN rr VARCHAR(64) NOT NULL DEFAULT ''`,
			// 确保 chat_sessions.messages 列存在且为 MEDIUMTEXT
			`ALTER TABLE chat_sessions ADD COLUMN messages MEDIUMTEXT NOT NULL`,
			`ALTER TABLE chat_sessions MODIFY COLUMN messages MEDIUMTEXT NOT NULL`,
			// 添加 type 列 + 数据迁移
			`ALTER TABLE chat_sessions ADD COLUMN type VARCHAR(16) NOT NULL DEFAULT 'chat'`,
		// v4 — sidebar pin support
		`ALTER TABLE chat_sessions ADD COLUMN pinned TINYINT(1) NOT NULL DEFAULT 0`,
			`UPDATE chat_sessions SET type='qqbot' WHERE id LIKE 'qqbot_%'`,
			`UPDATE chat_sessions SET id=CONCAT('chat_', UNIX_TIMESTAMP(created_at)*1000), type='chat' WHERE id='default'`,
		}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			// 忽略 "Duplicate key name" 错误 (errno 1061)
			if strings.Contains(err.Error(), "Duplicate key name") || strings.Contains(err.Error(), "Duplicate column") {
				continue
			}
			return fmt.Errorf("执行 DDL 失败: %w\nSQL: %s", err, ddl)
		}
	}

	return nil
}

