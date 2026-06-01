package base

import "context"

// Provider DNS 提供商接口。
// 所有 DNS 服务商（阿里云、CloudFlare、DNSPod 等）需实现此接口。
type Provider interface {
	// 域名管理
	ListDomains(ctx context.Context, keyword string, page, size int) (*ListResult, error)
	ListAllRecords(ctx context.Context, domainName string, page, size int) ([]Record, int, error)

	// 记录查询
	FindRecord(ctx context.Context, domainName, rr, recordType string) (*Record, error)

	// 记录变更
	UpdateRecord(ctx context.Context, recordID, rr, recordType, value string, ttl int) error
	AddRecord(ctx context.Context, domainName, rr, recordType, value string, ttl int) (string, error)
	SetRecordStatus(ctx context.Context, recordID, status string) error
	DeleteRecord(ctx context.Context, recordID string) error
}
