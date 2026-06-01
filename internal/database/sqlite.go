package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB 数据库包装
type DB struct {
	*sql.DB
}

// New 打开 SQLite 数据库并执行迁移
func New(dbPath string) (*DB, error) {
	// 启用 WAL 模式、外键约束
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_sync=NORMAL&_foreign_keys=on&_busy_timeout=5000", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 连接池配置
	db.SetMaxOpenConns(1)    // SQLite 单写者，设为 1
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// 验证连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	wrapper := &DB{db}

	// 执行迁移
	if err := wrapper.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return wrapper, nil
}

// migrate 创建/更新表结构
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS domains (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			domain      TEXT NOT NULL,
			record_id   TEXT NOT NULL DEFAULT '',
			rr          TEXT NOT NULL,
			type        TEXT NOT NULL CHECK(type IN ('A', 'AAAA')),
			value       TEXT NOT NULL DEFAULT '',
			ttl         INTEGER NOT NULL DEFAULT 600,
			enabled     INTEGER NOT NULL DEFAULT 1,
			cron_expr   TEXT NOT NULL DEFAULT '*/5 * * * *',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(domain, rr, type)
		)`,

		`CREATE TABLE IF NOT EXISTS histories (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			domain      TEXT NOT NULL,
			rr          TEXT NOT NULL DEFAULT '',
			old_ip      TEXT NOT NULL DEFAULT '',
			new_ip      TEXT NOT NULL DEFAULT '',
			type        TEXT NOT NULL DEFAULT 'AAAA',
			status      TEXT NOT NULL DEFAULT 'success' CHECK(status IN ('success', 'failed')),
			error_msg   TEXT NOT NULL DEFAULT '',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role          TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('admin', 'user')),
			created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS config (
			section    TEXT PRIMARY KEY,
			data       TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id         TEXT PRIMARY KEY,
			type       TEXT NOT NULL DEFAULT 'chat',
			title      TEXT NOT NULL DEFAULT '',
			messages   TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS session_goals (
			id          TEXT PRIMARY KEY,
			session_id  TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
			title       TEXT NOT NULL,
			steps_json  TEXT NOT NULL DEFAULT '[]',
			status      TEXT NOT NULL DEFAULT 'active',
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS session_todos (
			session_id TEXT PRIMARY KEY REFERENCES chat_sessions(id) ON DELETE CASCADE,
			items_json TEXT NOT NULL DEFAULT '[]',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS shares (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			token      TEXT NOT NULL UNIQUE COLLATE NOCASE,
			file_path  TEXT NOT NULL,
			password   TEXT NOT NULL DEFAULT '',
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			downloads  INTEGER NOT NULL DEFAULT 0
		)`,

		`CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_histories_domain ON histories(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_histories_created ON histories(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_token ON shares(token)`,

		`CREATE TABLE IF NOT EXISTS file_permissions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER NOT NULL,
			path        TEXT NOT NULL,
			can_read    INTEGER NOT NULL DEFAULT 1,
			can_write   INTEGER NOT NULL DEFAULT 0,
			can_delete  INTEGER NOT NULL DEFAULT 0,
			can_share   INTEGER NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, path),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_fp_user_id ON file_permissions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_fp_path ON file_permissions(path)`,

		// ── AI 监控表 ────────────────────────────────

		`CREATE TABLE IF NOT EXISTS ai_event_log (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
			session_id  TEXT NOT NULL DEFAULT '',
			event_type  TEXT NOT NULL,
			tool_name   TEXT NOT NULL DEFAULT '',
			model       TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT 'success',
			value       REAL NOT NULL DEFAULT 0,
			labels_json TEXT NOT NULL DEFAULT '{}',
			extra_json  TEXT NOT NULL DEFAULT '{}'
		)`,

		`CREATE INDEX IF NOT EXISTS idx_ai_event_type ON ai_event_log(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_event_tool ON ai_event_log(tool_name)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_event_created ON ai_event_log(created_at)`,

		`CREATE TABLE IF NOT EXISTS ai_metrics_hourly (
			hour       TEXT PRIMARY KEY,
			data       TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		// Idempotent ALTER TABLE migrations for existing databases
		`ALTER TABLE chat_sessions ADD COLUMN messages TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE chat_sessions ADD COLUMN type TEXT NOT NULL DEFAULT 'chat'`,
		// Migrate existing session types
		`UPDATE chat_sessions SET type='qqbot' WHERE id LIKE 'qqbot_%'`,
		`UPDATE chat_sessions SET id='chat_' || CAST((julianday(created_at)-2440587.5)*86400000 AS INTEGER), type='chat' WHERE id='default'`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_type ON chat_sessions(type)`,
		`ALTER TABLE users ADD COLUMN storage_quota INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN storage_used INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE histories ADD COLUMN rr TEXT NOT NULL DEFAULT ''`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			// SQLite: ignore "duplicate column name" errors for idempotent ALTER TABLE
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return fmt.Errorf("执行迁移失败: %w\nSQL: %s", err, m)
		}
	}

	return nil
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	return db.DB.Close()
}

// BeginTx 开启事务
func (db *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return db.DB.BeginTx(ctx, nil)
}
