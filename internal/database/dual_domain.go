package database

import (
	"context"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// DualDomainRepo 双数据库域名仓库
type DualDomainRepo struct {
	Sqlite *DomainRepo
	Mysql  *DomainRepo
	syncCh chan<- SyncJob
}

// NewDualDomainRepo 创建双数据库域名仓库
func NewDualDomainRepo(sqliteDB, mysqlDB Executor, syncCh chan<- SyncJob) *DualDomainRepo {
	return &DualDomainRepo{
		Sqlite: NewDomainRepo(sqliteDB),
		Mysql:  NewDomainRepo(mysqlDB),
		syncCh: syncCh,
	}
}

// GetMySQL 返回 MySQL executor（供 Syncer 使用）
func (r *DualDomainRepo) GetMySQL() Executor {
	return r.Mysql.db
}

func (r *DualDomainRepo) signal(table string, id int64) {
	select {
	case r.syncCh <- SyncJob{Table: table, ID: id}:
	default:
	}
}

// Create 写入 SQLite，发同步信号
func (r *DualDomainRepo) Create(ctx context.Context, rec *models.DomainRecord) (int64, error) {
	id, err := r.Sqlite.Create(ctx, rec)
	if err == nil {
		r.signal("domains", id)
	}
	return id, err
}

// GetByID 双库查询，取 updated_at 较新者
func (r *DualDomainRepo) GetByID(ctx context.Context, id int64) (*models.DomainRecord, error) {
	sqliteRec, sqliteErr := r.Sqlite.GetByID(ctx, id)
	mysqlRec, _ := r.Mysql.GetByID(ctx, id)
	return pickDomain(sqliteRec, sqliteErr, mysqlRec), nil
}

// GetByDomain 双库查询
func (r *DualDomainRepo) GetByDomain(ctx context.Context, domain, rr, recType string) (*models.DomainRecord, error) {
	sqliteRec, sqliteErr := r.Sqlite.GetByDomain(ctx, domain, rr, recType)
	mysqlRec, _ := r.Mysql.GetByDomain(ctx, domain, rr, recType)
	return pickDomain(sqliteRec, sqliteErr, mysqlRec), nil
}

// List 合并双库，按 updated_at 去重
func (r *DualDomainRepo) List(ctx context.Context) ([]models.DomainRecord, error) {
	sqliteRecs, err := r.Sqlite.List(ctx)
	if err != nil {
		return nil, err
	}
	mysqlRecs, _ := r.Mysql.List(ctx)
	return mergeDomains(sqliteRecs, mysqlRecs), nil
}

// ListEnabled 合并双库已启用记录
func (r *DualDomainRepo) ListEnabled(ctx context.Context) ([]models.DomainRecord, error) {
	sqliteRecs, err := r.Sqlite.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	mysqlRecs, _ := r.Mysql.ListEnabled(ctx)
	return mergeDomains(sqliteRecs, mysqlRecs), nil
}

// Update 写入 SQLite，发同步信号
func (r *DualDomainRepo) Update(ctx context.Context, rec *models.DomainRecord) error {
	if err := r.Sqlite.Update(ctx, rec); err != nil {
		return err
	}
	r.signal("domains", rec.ID)
	return nil
}

// UpdateValue 高频 IP 更新，发同步信号
func (r *DualDomainRepo) UpdateValue(ctx context.Context, id int64, recordID, value string) error {
	if err := r.Sqlite.UpdateValue(ctx, id, recordID, value); err != nil {
		return err
	}
	r.signal("domains", id)
	return nil
}

// Delete 删除 SQLite 记录，发同步信号
func (r *DualDomainRepo) Delete(ctx context.Context, id int64) error {
	if err := r.Sqlite.Delete(ctx, id); err != nil {
		return err
	}
	r.signal("domains", id)
	return nil
}

// Upsert 写入 SQLite
func (r *DualDomainRepo) Upsert(ctx context.Context, rec *models.DomainRecord) error {
	if err := r.Sqlite.Upsert(ctx, rec); err != nil {
		return err
	}
	r.signal("domains", rec.ID)
	return nil
}

func pickDomain(sqliteRec *models.DomainRecord, sqliteErr error, mysqlRec *models.DomainRecord) *models.DomainRecord {
	if sqliteErr != nil && mysqlRec != nil {
		return mysqlRec
	}
	if sqliteErr != nil {
		return nil
	}
	if mysqlRec != nil && mysqlRec.UpdatedAt.After(sqliteRec.UpdatedAt) {
		return mysqlRec
	}
	return sqliteRec
}

func mergeDomains(sqliteRecs, mysqlRecs []models.DomainRecord) []models.DomainRecord {
	index := make(map[string]models.DomainRecord)
	for _, r := range sqliteRecs {
		index[r.Domain+"/"+r.RR+"/"+r.Type] = r
	}
	for _, r := range mysqlRecs {
		key := r.Domain + "/" + r.RR + "/" + r.Type
		existing, ok := index[key]
		if !ok || r.UpdatedAt.After(existing.UpdatedAt) {
			index[key] = r
		}
	}
	result := make([]models.DomainRecord, 0, len(index))
	for _, r := range index {
		result = append(result, r)
	}
	return result
}
