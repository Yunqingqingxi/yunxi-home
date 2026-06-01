// Package dns 统一 DNS 管理模块入口。
//
// 消费者只需导入此包即可使用所有 DNS 功能，无需关心底层实现。
//
//	import dns "github.com/yxd/yunxi-home/internal/dns"
//	client := dns.NewAliDNS(ak, sk, endpoint, servers)
//	records, total, _ := client.ListAllRecords(ctx, "example.com", 1, 50)
package dns

import (
	"github.com/yxd/yunxi-home/internal/dns/alidns"
	"github.com/yxd/yunxi-home/internal/dns/base"
)

// ── 类型别名（消费者直接使用 dns.Provider, dns.Record 等）─────────

// Provider DNS 提供商接口
type Provider = base.Provider

// Record DNS 解析记录
type Record = base.Record

// Domain 域名信息
type Domain = base.Domain

// ListResult 域名列表查询结果
type ListResult = base.ListResult

// Error 通用 DNS 错误
type Error = base.Error

// ── 工厂函数 ─────────────────────────────────────────────

// NewAliDNS 创建阿里云 DNS 客户端，返回 Provider 接口。
func NewAliDNS(accessKeyID, accessKeySecret, endpoint string, dnsServers []string) Provider {
	return alidns.NewClient(accessKeyID, accessKeySecret, endpoint, dnsServers)
}
