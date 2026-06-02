package database

import (
	"context"
	"fmt"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
)

// Migrator copies data between two backends.
type Migrator struct {
	src *Backend
	dst *Backend
}

// NewMigrator creates a data migrator.
func NewMigrator(src, dst *Backend) *Migrator {
	return &Migrator{src: src, dst: dst}
}

// ProgressEvent is emitted during migration for each entity type.
type ProgressEvent struct {
	Entity string `json:"entity"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Error  string `json:"error,omitempty"`
}

// Migrate copies all data from src to dst, calling onProgress after each entity batch.
func (m *Migrator) Migrate(ctx context.Context, onProgress func(ProgressEvent)) error {
	entities := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"users", m.migrateUsers},
		{"config", m.migrateConfig},
		{"domains", m.migrateDomains},
		{"histories", m.migrateHistories},
		{"chat_sessions", m.migrateChatSessions},
		{"shares", m.migrateShares},
		{"file_permissions", m.migrateFilePerms},
	}

	for _, e := range entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		log.Info("开始迁移", "entity", e.name)
		if err := e.fn(ctx); err != nil {
			if onProgress != nil {
				onProgress(ProgressEvent{Entity: e.name, Error: err.Error()})
			}
			log.Error("迁移失败", "entity", e.name, "error", err)
			return fmt.Errorf("migrate %s: %w", e.name, err)
		}
		if onProgress != nil {
			onProgress(ProgressEvent{Entity: e.name, Done: 1, Total: 1})
		}
		log.Info("迁移完成", "entity", e.name)
	}

	return nil
}

func (m *Migrator) migrateUsers(ctx context.Context) error {
	users, err := m.src.UserRepo.List(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if _, err := m.dst.UserRepo.Create(ctx, &models.User{
			Username:     u.Username,
			PasswordHash: u.PasswordHash,
			Role:         u.Role,
			StorageQuota: u.StorageQuota,
			StorageUsed:  u.StorageUsed,
		}); err != nil {
			log.Warn("迁移用户失败", "username", u.Username, "error", err)
		}
	}
	return nil
}

func (m *Migrator) migrateConfig(ctx context.Context) error {
	all, err := m.src.ConfigRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	for section, data := range all {
		if err := m.dst.ConfigRepo.SetSection(ctx, section, data); err != nil {
			log.Warn("迁移配置失败", "section", section, "error", err)
		}
	}
	return nil
}

func (m *Migrator) migrateDomains(ctx context.Context) error {
	recs, err := m.src.DomainRepo.List(ctx)
	if err != nil {
		return err
	}
	for _, r := range recs {
		rec := r
		if _, err := m.dst.DomainRepo.Create(ctx, &rec); err != nil {
			log.Warn("迁移域名失败", "domain", r.Domain, "error", err)
		}
	}
	return nil
}

func (m *Migrator) migrateHistories(ctx context.Context) error {
	// Migrate in batches to avoid memory issues
	page := 1
	batchSize := 500
	for {
		result, err := m.src.HistoryRepo.List(ctx, ListParams{Page: page, Size: batchSize})
		if err != nil {
			return err
		}
		if len(result.Records) == 0 {
			break
		}
		for _, r := range result.Records {
			rec := r
			if _, err := m.dst.HistoryRepo.Create(ctx, &rec); err != nil {
				log.Warn("迁移历史失败", "id", r.ID, "error", err)
			}
		}
		if int64(page*batchSize) >= result.Total {
			break
		}
		page++
	}
	return nil
}

func (m *Migrator) migrateChatSessions(ctx context.Context) error {
	sessions, err := m.src.ChatRepo.List(ctx)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		sess := s
		if err := m.dst.ChatRepo.Upsert(ctx, &sess); err != nil {
			log.Warn("迁移会话失败", "id", s.ID, "error", err)
		}
	}
	return nil
}

func (m *Migrator) migrateShares(ctx context.Context) error {
	shares, total, err := m.src.ShareRepo.List(ctx, 10000, 0)
	if err != nil {
		return err
	}
	_ = total
	for _, s := range shares {
		share := nas.Share{
			Token:     s.Token,
			FilePath:  s.FilePath,
			Password:  s.Password,
			ExpiresAt: s.ExpiresAt,
		}
		if _, err := m.dst.ShareRepo.Create(ctx, &share); err != nil {
			log.Warn("迁移分享失败", "token", s.Token, "error", err)
		}
	}
	return nil
}

func (m *Migrator) migrateFilePerms(ctx context.Context) error {
	perms, err := m.src.FilePermRepo.ListAll(ctx)
	if err != nil {
		return err
	}
	for _, p := range perms {
		perm := p
		if err := m.dst.FilePermRepo.Upsert(ctx, &perm); err != nil {
			log.Warn("迁移文件权限失败", "path", p.Path, "error", err)
		}
	}
	return nil
}
