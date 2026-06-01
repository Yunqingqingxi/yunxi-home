package database

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/yxd/yunxi-home/internal/models"
)

// SyncJob 同步任务
type SyncJob struct {
	Table string
	ID    int64
}

// Syncer SQLite -> MySQL 后台同步器
type Syncer struct {
	syncCh     <-chan SyncJob
	sqliteDom  *DomainRepo
	mysqlDom   *DomainRepo
	sqliteHist *HistoryRepo
	mysqlHist  *HistoryRepo
	sqliteUser *UserRepo
	mysqlUser  *MySQLUserRepo
	sqliteCfg  ConfigRepository
	mysqlCfg   ConfigRepository
	stopCh     chan struct{}
}

// batchSize 历史记录分页同步的每页大小
const batchSize = 500

// NewSyncer 创建同步器
func NewSyncer(sqliteExec, mysqlExec Executor, syncCh <-chan SyncJob, sqliteCfg, mysqlCfg ConfigRepository) *Syncer {
	return &Syncer{
		syncCh:     syncCh,
		sqliteDom:  NewDomainRepo(sqliteExec),
		mysqlDom:   NewDomainRepo(mysqlExec),
		sqliteHist: NewHistoryRepo(sqliteExec),
		mysqlHist:  NewHistoryRepo(mysqlExec),
		sqliteUser: NewUserRepo(sqliteExec),
		mysqlUser:  NewMySQLUserRepo(mysqlExec),
		sqliteCfg:  sqliteCfg,
		mysqlCfg:   mysqlCfg,
		stopCh:     make(chan struct{}),
	}
}

// Start 启动后台同步
func (s *Syncer) Start(ctx context.Context) {
	slog.Info("MySQL 同步器已启动")

	// 1. 启动时全量同步
	s.fullSyncDomains(ctx)
	s.syncAllHistory(ctx)
	s.syncAllConfig(ctx)
	s.fullSyncUsers(ctx)
	slog.Info("MySQL 初始全量同步完成")

	// 2. 定期全量校准（每 10 分钟）
	reconcileTicker := time.NewTicker(10 * time.Minute)
	defer reconcileTicker.Stop()

	for {
		select {
		case job := <-s.syncCh:
			s.handleSync(ctx, job)
		case <-reconcileTicker.C:
			s.fullSyncDomains(ctx)
			s.syncAllHistory(ctx)
			s.syncAllConfig(ctx)
			s.fullSyncUsers(ctx)
				slog.Debug("MySQL 定期校准完成")
		case <-s.stopCh:
			slog.Info("MySQL 同步器已停止")
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Syncer) handleSync(ctx context.Context, job SyncJob) {
	switch job.Table {
	case "domains":
		s.syncOneDomain(ctx, job.ID)
	case "histories":
		s.syncHistoryIncremental(ctx)
	case "users":
		s.fullSyncUsers(ctx)
	}
}

func (s *Syncer) syncOneDomain(ctx context.Context, id int64) {
	rec, err := s.sqliteDom.GetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			_ = s.mysqlDom.Delete(ctx, id)
		}
		return
	}
	_ = s.mysqlDom.Upsert(ctx, rec)
}

func (s *Syncer) fullSyncDomains(ctx context.Context) {
	sqliteRecs, err := s.sqliteDom.List(ctx)
	if err != nil {
		slog.Warn("全量同步: 读取 SQLite domains 失败", "error", err)
		return
	}
	mysqlRecs, _ := s.mysqlDom.List(ctx)

	// 构建 MySQL 记录索引
	mysqlMap := make(map[string]*models.DomainRecord)
	for i := range mysqlRecs {
		key := mysqlRecs[i].Domain + "/" + mysqlRecs[i].RR + "/" + mysqlRecs[i].Type
		mysqlMap[key] = &mysqlRecs[i]
	}

	// 同步 SQLite -> MySQL
	for _, rec := range sqliteRecs {
		key := rec.Domain + "/" + rec.RR + "/" + rec.Type
		if existing, ok := mysqlMap[key]; ok {
			if rec.UpdatedAt.After(existing.UpdatedAt) {
				_ = s.mysqlDom.Update(ctx, &rec)
			}
		} else {
			_ = s.mysqlDom.Upsert(ctx, &rec)
		}
		delete(mysqlMap, key)
	}

	// 删除 MySQL 中多余记录
	for _, rec := range mysqlMap {
		_ = s.mysqlDom.Delete(ctx, rec.ID)
	}
}

// syncAllHistory 分页同步 SQLite 的历史记录到 MySQL，避免一次性加载大量数据
func (s *Syncer) syncAllHistory(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 先获取 MySQL 最大 ID，用于去重
	mysqlRecs, err := s.mysqlHist.List(ctx, ListParams{Page: 1, Size: 1})
	var maxMySQLID int64
	if err == nil && mysqlRecs != nil && len(mysqlRecs.Records) > 0 {
		maxMySQLID = mysqlRecs.Records[0].ID
	}

	page := 1
	totalSynced := 0
	for {
		select {
		case <-ctx.Done():
			slog.Warn("历史同步超时或取消", "synced", totalSynced)
			return
		default:
		}

		sqliteRecs, err := s.sqliteHist.List(ctx, ListParams{Page: page, Size: batchSize})
		if err != nil {
			if page == 1 {
				slog.Warn("全量同步: 读取 SQLite histories 失败", "error", err)
			}
			return
		}
		if len(sqliteRecs.Records) == 0 {
			break
		}

		// 如果该页所有记录 ID 都 <= maxMySQLID，说明后续也都是旧数据
		allOld := true
		for _, rec := range sqliteRecs.Records {
			if rec.ID > maxMySQLID {
				allOld = false
				if _, err := s.mysqlHist.Create(ctx, &rec); err == nil {
					totalSynced++
				}
			}
		}
		if allOld && page > 1 {
			break
		}
		page++
	}

	if totalSynced > 0 {
		slog.Info("历史全量同步完成", "synced", totalSynced)
	}
}

func (s *Syncer) syncHistoryIncremental(ctx context.Context) {
	mysqlRecs, _ := s.mysqlHist.List(ctx, ListParams{Page: 1, Size: 1})
	var maxMySQLID int64
	if mysqlRecs != nil && len(mysqlRecs.Records) > 0 {
		maxMySQLID = mysqlRecs.Records[0].ID
	}

	sqliteRecs, err := s.sqliteHist.List(ctx, ListParams{Page: 1, Size: batchSize})
	if err != nil {
		return
	}

	for _, rec := range sqliteRecs.Records {
		if rec.ID > maxMySQLID {
			_, _ = s.mysqlHist.Create(ctx, &rec)
		}
	}
}

// syncAllConfig 全量同步 config 表到 MySQL
func (s *Syncer) syncAllConfig(ctx context.Context) {
	sqliteSections, err := s.sqliteCfg.GetAll(ctx)
	if err != nil {
		slog.Warn("全量同步: 读取 SQLite config 失败", "error", err)
		return
	}
	mysqlSections, _ := s.mysqlCfg.GetAll(ctx)

	for section, data := range sqliteSections {
		if existing, ok := mysqlSections[section]; ok && existing == data {
			continue
		}
		if err := s.mysqlCfg.SetSection(ctx, section, data); err != nil {
			slog.Warn("同步 config 到 MySQL 失败", "section", section, "error", err)
		}
	}
}

func (s *Syncer) fullSyncUsers(ctx context.Context) {
	sqliteUsers, err := s.sqliteUser.List(ctx)
	if err != nil { slog.Warn("全量同步: 读取 SQLite users 失败", "error", err); return }
	for i := range sqliteUsers {
		if err := s.mysqlUser.Upsert(ctx, &sqliteUsers[i]); err != nil {
			slog.Warn("同步 user 到 MySQL 失败", "username", sqliteUsers[i].Username, "error", err)
		}
	}
	mysqlUsers, _ := s.mysqlUser.List(ctx)
	sqliteIDs := make(map[int64]bool)
	for _, u := range sqliteUsers { sqliteIDs[u.ID] = true }
	for _, u := range mysqlUsers {
		if !sqliteIDs[u.ID] { s.mysqlUser.Delete(ctx, u.ID) }
	}
}

// Stop 停止同步器
func (s *Syncer) Stop() {
	close(s.stopCh)
}
