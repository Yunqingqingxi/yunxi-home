package models

import "time"

// HistoryRecord DNS 更新历史记录
type HistoryRecord struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`     // 域名
	RR        string    `json:"rr"`         // 主机记录，如 @、www
	OldIP     string    `json:"old_ip"`     // 旧 IP
	NewIP     string    `json:"new_ip"`     // 新 IP
	Type      string    `json:"type"`       // A 或 AAAA
	Status    string    `json:"status"`     // success / failed
	ErrorMsg  string    `json:"error_msg"`  // 失败时的错误信息
	CreatedAt time.Time `json:"created_at"` // 记录时间
}
