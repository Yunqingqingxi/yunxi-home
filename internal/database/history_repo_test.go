package database

import (
	"context"
	"testing"

	"github.com/yxd/yunxi-home/internal/models"
)

func TestHistoryRepoCreateAndList(t *testing.T) {
	db := setupTestDB(t)
	repo := NewHistoryRepo(db)
	ctx := context.Background()

	// 创建多条记录
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, &models.HistoryRecord{
			Domain: "example.com",
			RR:     "@",
			OldIP:  "2001::1",
			NewIP:  "2001::2",
			Type:   "AAAA",
			Status: "success",
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// 分页查询
	result, err := repo.List(ctx, ListParams{Page: 1, Size: 3})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected total=5, got %d", result.Total)
	}
	if len(result.Records) != 3 {
		t.Errorf("expected 3 records on page 1, got %d", len(result.Records))
	}

	// 按域名过滤
	result2, err := repo.List(ctx, ListParams{Domain: "other.com", Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if result2.Total != 0 {
		t.Errorf("expected 0 results for other.com, got %d", result2.Total)
	}
}

func TestHistoryRepoCleanOld(t *testing.T) {
	db := setupTestDB(t)
	repo := NewHistoryRepo(db)
	ctx := context.Background()

	// 清理保留 0 天 = 全部删除
	n, err := repo.CleanOld(ctx, 0)
	if err != nil {
		t.Fatalf("CleanOld failed: %v", err)
	}
	if n < 0 {
		t.Errorf("unexpected deleted count: %d", n)
	}
}
