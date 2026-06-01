// Package base 定义 DNS 模块的通用类型和接口，零外部依赖。
// 所有 DNS 提供商实现均基于此包的类型。
package base

// Record DNS 解析记录（通用表示）
type Record struct {
	RecordID string `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Status   string `json:"Status"`
}

// Domain 域名信息
type Domain struct {
	DomainID    string `json:"DomainId"`
	DomainName  string `json:"DomainName"`
	RecordCount int    `json:"RecordCount"`
}

// ListResult 域名列表查询结果
type ListResult struct {
	TotalCount int      `json:"total"`
	Domains    []Domain `json:"domains"`
}
