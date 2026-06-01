package models

import "time"

// DomainRecord 域名解析记录（本地数据库存储）
type DomainRecord struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`     // 完整域名，如 example.com
	RecordID  string    `json:"record_id"`  // 阿里云返回的 RecordId
	RR        string    `json:"rr"`         // 主机记录，如 @、www
	Type      string    `json:"type"`       // A 或 AAAA
	Value     string    `json:"value"`      // 当前 IP 值
	TTL       int       `json:"ttl"`        // TTL 秒数
	Enabled   bool      `json:"enabled"`    // 是否启用
	CronExpr  string    `json:"cron_expr"`  // cron 表达式
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
