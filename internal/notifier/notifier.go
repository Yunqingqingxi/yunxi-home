// Package notifier 统一通知管理模块入口。
//
// 消费者只需导入此包即可使用所有通知功能，无需关心底层实现。
//
//	import "github.com/yxd/yunxi-home/internal/notifier"
//	nm := notifier.NewManager(notifier.NewThrottler())
//	nm.Register(notifier.NewEmailNotifier(notifier.EmailConfig{...}))
package notifier

import (
	"github.com/yxd/yunxi-home/internal/notifier/base"
	"github.com/yxd/yunxi-home/internal/notifier/email"
	"github.com/yxd/yunxi-home/internal/notifier/webhook"
)

// ── 类型别名（消费者直接使用 notifier.Notifier, notifier.ChangeEvent 等）─────────

// Notifier 通知接口
type Notifier = base.Notifier

// ChangeEvent DNS 变更事件
type ChangeEvent = base.ChangeEvent

// Error 通用通知错误
type Error = base.Error

// ── 实现类型别名 ─────────────────────────────────────────────

// EmailNotifier 邮件通知器
type EmailNotifier = email.Notifier

// EmailConfig 邮件配置
type EmailConfig = email.Config

// WebhookNotifier Webhook 通知器
type WebhookNotifier = webhook.Notifier

// WebhookConfig Webhook 配置
type WebhookConfig = webhook.Config

// ── 工厂函数 ─────────────────────────────────────────────────

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(cfg EmailConfig) *EmailNotifier {
	return email.New(cfg)
}

// NewWebhookNotifier 创建 Webhook 通知器
func NewWebhookNotifier(cfg WebhookConfig) *WebhookNotifier {
	return webhook.New(cfg)
}
