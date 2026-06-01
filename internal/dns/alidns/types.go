package alidns

import "github.com/Yunqingqingxi/yunxi-home/internal/dns/base"

// ── 阿里云 API 请求/响应类型 ──────────────────────────────

// DomainRecord 阿里云域名记录（嵌入通用 Record）
type DomainRecord struct {
	base.Record
	Line   string `json:"Line"`
	Locked bool   `json:"Locked"`
	Weight int    `json:"Weight"`
}

// DomainRecords 阿里云记录列表
type DomainRecords struct {
	Record []DomainRecord `json:"Record"`
}

// DomainInfo 阿里云域名信息
type DomainInfo struct {
	DomainID    string `json:"DomainId"`
	DomainName  string `json:"DomainName"`
	RecordCount int    `json:"RecordCount"`
}

// CommonRequest 阿里云 API 通用请求参数
type CommonRequest struct {
	Format           string `url:"Format"`
	Version          string `url:"Version"`
	AccessKeyID      string `url:"AccessKeyId"`
	SignatureMethod  string `url:"SignatureMethod"`
	Timestamp        string `url:"Timestamp"`
	SignatureVersion string `url:"SignatureVersion"`
	SignatureNonce   string `url:"SignatureNonce"`
	Signature        string `url:"Signature,omitempty"`
	Action           string `url:"Action"`
}

// AddRequest 添加域名记录请求
type AddRequest struct {
	DomainName string `url:"DomainName"`
	RR         string `url:"RR"`
	Type       string `url:"Type"`
	Value      string `url:"Value"`
	TTL        int    `url:"TTL,omitempty"`
	Priority   int    `url:"Priority,omitempty"`
	Line       string `url:"Line,omitempty"`
}

// UpdateRequest 更新域名记录请求
type UpdateRequest struct {
	RecordID string `url:"RecordId"`
	RR       string `url:"RR"`
	Type     string `url:"Type"`
	Value    string `url:"Value"`
	TTL      int    `url:"TTL,omitempty"`
	Line     string `url:"Line,omitempty"`
}

// DescribeRequest 查询域名记录请求
type DescribeRequest struct {
	DomainName  string `url:"DomainName"`
	PageNumber  int    `url:"PageNumber,omitempty"`
	PageSize    int    `url:"PageSize,omitempty"`
	RRKeyWord   string `url:"RRKeyWord,omitempty"`
	TypeKeyWord string `url:"TypeKeyWord,omitempty"`
	SearchMode  string `url:"SearchMode,omitempty"`
}

// DescribeResponse 查询域名记录响应
type DescribeResponse struct {
	RequestID     string        `json:"RequestId"`
	TotalCount    int           `json:"TotalCount"`
	PageNumber    int           `json:"PageNumber"`
	PageSize      int           `json:"PageSize"`
	DomainRecords DomainRecords `json:"DomainRecords"`
}

// Response 阿里云通用响应
type Response struct {
	RequestID string `json:"RequestId"`
	RecordID  string `json:"RecordId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
}

// ── 类型转换 ──────────────────────────────────────────────

// toBaseRecord 将阿里云 DomainRecord 转为通用 Record
func toBaseRecord(r *DomainRecord) *base.Record {
	if r == nil {
		return nil
	}
	return &r.Record
}

// toBaseRecords 批量转换
func toBaseRecords(records []DomainRecord) []base.Record {
	result := make([]base.Record, len(records))
	for i, r := range records {
		result[i] = r.Record
	}
	return result
}

// toBaseDomains 将阿里云 DomainInfo 列表转为通用 Domain 列表
func toBaseDomains(domains []DomainInfo) []base.Domain {
	result := make([]base.Domain, len(domains))
	for i, d := range domains {
		result[i] = base.Domain{
			DomainID:    d.DomainID,
			DomainName:  d.DomainName,
			RecordCount: d.RecordCount,
		}
	}
	return result
}
