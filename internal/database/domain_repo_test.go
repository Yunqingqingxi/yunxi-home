package database

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()

	path := "/tmp/test_dns_updater_" + t.Name() + ".db"
	os.Remove(path) // 清理旧文件

	db, err := New(path)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.Remove(path)
	})

	return db
}

func TestDomainRepoCreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDomainRepo(db)
	ctx := context.Background()

	rec := &models.DomainRecord{
		Domain:  "example.com",
		RR:      "@",
		Type:    "AAAA",
		TTL:     600,
		Enabled: true,
	}

	id, err := repo.Create(ctx, rec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("invalid id: %d", id)
	}

	// 获取
	got, err := repo.GetByDomain(ctx, "example.com", "@", "AAAA")
	if err != nil {
		t.Fatalf("GetByDomain failed: %v", err)
	}
	if got.Domain != "example.com" || got.RR != "@" {
		t.Errorf("unexpected record: %+v", got)
	}
}

func TestDomainRepoList(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDomainRepo(db)
	ctx := context.Background()

	repo.Create(ctx, &models.DomainRecord{Domain: "a.com", RR: "@", Type: "AAAA", Enabled: true})
	repo.Create(ctx, &models.DomainRecord{Domain: "b.com", RR: "www", Type: "A", Enabled: false})

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 records, got %d", len(all))
	}

	enabled, err := repo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled failed: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled record, got %d", len(enabled))
	}
}

func TestDomainRepoUpdateValue(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDomainRepo(db)
	ctx := context.Background()

	repo.Create(ctx, &models.DomainRecord{
		Domain: "example.com", RR: "@", Type: "AAAA", Value: "2001::1", Enabled: true,
	})

	rec, _ := repo.GetByDomain(ctx, "example.com", "@", "AAAA")
	if err := repo.UpdateValue(ctx, rec.ID, "rid123", "2001::2"); err != nil {
		t.Fatalf("UpdateValue failed: %v", err)
	}

	updated, _ := repo.GetByDomain(ctx, "example.com", "@", "AAAA")
	if updated.Value != "2001::2" {
		t.Errorf("expected Value=2001::2, got %s", updated.Value)
	}
	if updated.RecordID != "rid123" {
		t.Errorf("expected RecordID=rid123, got %s", updated.RecordID)
	}
}

func TestDomainRepoDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDomainRepo(db)
	ctx := context.Background()

	repo.Create(ctx, &models.DomainRecord{Domain: "example.com", RR: "@", Type: "AAAA"})
	rec, _ := repo.GetByDomain(ctx, "example.com", "@", "AAAA")

	if err := repo.Delete(ctx, rec.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.GetByDomain(ctx, "example.com", "@", "AAAA")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows after delete, got %v", err)
	}
}
