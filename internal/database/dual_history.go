package database

import (
	"context"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// DualHistoryRepo 双数据库历史仓库
type DualHistoryRepo struct {
	sqlite *HistoryRepo
	mysql  *HistoryRepo
	syncCh chan<- SyncJob
}

// NewDualHistoryRepo 创建双数据库历史仓库
func NewDualHistoryRepo(sqliteDB, mysqlDB Executor, syncCh chan<- SyncJob) *DualHistoryRepo {
	return &DualHistoryRepo{
		sqlite: NewHistoryRepo(sqliteDB),
		mysql:  NewHistoryRepo(mysqlDB),
		syncCh: syncCh,
	}
}

func (r *DualHistoryRepo) signal(table string, id int64) {
	select {
	case r.syncCh <- SyncJob{Table: table, ID: id}:
	default:
	}
}

// Create 写入 SQLite（历史记录 SQLite 为权威源）
func (r *DualHistoryRepo) Create(ctx context.Context, rec *models.HistoryRecord) (int64, error) {
	id, err := r.sqlite.Create(ctx, rec)
	if err == nil {
		r.signal("histories", id)
	}
	return id, err
}

// GetByID 从 SQLite 读取（权威源）
func (r *DualHistoryRepo) GetByID(ctx context.Context, id int64) (*models.HistoryRecord, error) {
	return r.sqlite.GetByID(ctx, id)
}

// List 从 SQLite 读取（权威源）
func (r *DualHistoryRepo) List(ctx context.Context, params ListParams) (*ListResult, error) {
	return r.sqlite.List(ctx, params)
}

// GetStats 从 SQLite 读取统计
func (r *DualHistoryRepo) GetStats(ctx context.Context, days int) ([]HistoryStats, error) {
	return r.sqlite.GetStats(ctx, days)
}

// CleanOld 清理 SQLite，同步到 MySQL
func (r *DualHistoryRepo) CleanOld(ctx context.Context, days int) (int64, error) {
	n, err := r.sqlite.CleanOld(ctx, days)
	if err == nil && n > 0 {
		r.signal("histories", 0)
	}
	return n, err
}
