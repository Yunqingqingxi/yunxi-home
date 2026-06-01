// Package base 定义通知模块的通用类型和接口，零外部依赖。
// 所有通知渠道实现均基于此包的类型。
package base

import "context"

// ChangeEvent DNS 变更事件
type ChangeEvent struct {
	Domain     string `json:"domain"`
	FullDomain string `json:"full_domain"`
	Type       string `json:"type"`
	OldIP      string `json:"old_ip"`
	NewIP      string `json:"new_ip"`
	Timestamp  string `json:"timestamp"`
}

// Notifier 通知接口
type Notifier interface {
	// Name 返回通知器名称
	Name() string
	// Send 发送通知
	Send(ctx context.Context, event ChangeEvent) error
	// IsEnabled 是否启用
	IsEnabled() bool
}
