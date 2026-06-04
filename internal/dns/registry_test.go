package dns

import (
	"context"
	"testing"

	"github.com/Yunqingqingxi/yunxi-home/internal/dns/base"
)

// mockProvider implements base.Provider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) ListDomains(ctx context.Context, keyword string, page, size int) (*base.ListResult, error) {
	return &base.ListResult{}, nil
}
func (m *mockProvider) ListAllRecords(ctx context.Context, domainName string, page, size int) ([]base.Record, int, error) {
	return nil, 0, nil
}
func (m *mockProvider) FindRecord(ctx context.Context, domainName, rr, recordType string) (*base.Record, error) {
	return nil, nil
}
func (m *mockProvider) UpdateRecord(ctx context.Context, recordID, rr, recordType, value string, ttl int) error {
	return nil
}
func (m *mockProvider) AddRecord(ctx context.Context, domainName, rr, recordType, value string, ttl int) (string, error) {
	return "mock-id", nil
}
func (m *mockProvider) SetRecordStatus(ctx context.Context, recordID, status string) error { return nil }
func (m *mockProvider) DeleteRecord(ctx context.Context, recordID string) error            { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register("aliyun", &mockProvider{name: "aliyun"})
	r.Register("cloudflare", &mockProvider{name: "cloudflare"})

	p, err := r.Get("aliyun")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}

	p, err = r.Get("")
	if err != nil {
		t.Fatalf("default provider error: %v", err)
	}
	if p == nil {
		t.Fatal("expected default provider")
	}

	_, err = r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

func TestRegistry_Default(t *testing.T) {
	r := NewRegistry()
	r.Register("aliyun", &mockProvider{name: "aliyun"})

	if !r.IsConfigured() {
		t.Error("expected configured after registration")
	}

	names := r.List()
	if len(names) != 1 || names[0] != "aliyun" {
		t.Errorf("unexpected provider list: %v", names)
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	r.Register("aliyun", &mockProvider{name: "aliyun"})
	r.Register("cloudflare", &mockProvider{name: "cloudflare"})
	r.SetDefault("cloudflare")

	r.Remove("cloudflare")
	if r.IsConfigured() {
		// Should still have aliyun
		names := r.List()
		if len(names) != 1 || names[0] != "aliyun" {
			t.Errorf("expected [aliyun], got %v", names)
		}
	} else {
		t.Error("expected still configured after removing one provider")
	}
}

func TestRegistry_ReplaceAll(t *testing.T) {
	r := NewRegistry()
	r.Register("aliyun", &mockProvider{name: "aliyun"})

	err := r.ReplaceAll(map[string]base.Provider{
		"cloudflare": &mockProvider{name: "cloudflare"},
	}, "cloudflare")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.Get("aliyun")
	if err == nil {
		t.Error("aliyun should have been removed by ReplaceAll")
	}
	_, err = r.Get("cloudflare")
	if err != nil {
		t.Errorf("cloudflare should exist: %v", err)
	}
}
