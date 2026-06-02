package notifier

import (
	"context"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"time"
)

var log = logger.ForComponent("notifier")

// Manager 通知管理器，管理多个通知渠道
type Manager struct {
	notifiers []Notifier
	throttler *Throttler
}

// NewManager 创建通知管理器
func NewManager(throttle *Throttler) *Manager {
	return &Manager{
		throttler: throttle,
	}
}

// Register 注册通知器
func (m *Manager) Register(n Notifier) {
	if n.IsEnabled() {
		m.notifiers = append(m.notifiers, n)
		log.Info("通知渠道已注册", "name", n.Name())
	}
}

// SendNotification 发送 DNS 变更通知（通过所有已注册的渠道）
func (m *Manager) SendNotification(ctx context.Context, domain, rr, recordType, oldIP, newIP string) {
	log.Info("发送DNS变更通知", "域名", domain, "记录", rr, "类型", recordType, "旧IP", oldIP, "新IP", newIP)
	if len(m.notifiers) == 0 {
		return
	}

	key := fmt.Sprintf("%s/%s/%s", domain, rr, recordType)

	// 节流检查
	if !m.throttler.AllowAndMark(key, 10) {
		log.Debug("通知已节流，跳过发送", "key", key)
		return
	}

	fullDomain := rr
	if rr == "@" {
		fullDomain = domain
	} else {
		fullDomain = rr + "." + domain
	}

	event := ChangeEvent{
		Domain:     domain,
		FullDomain: fullDomain,
		Type:       recordType,
		OldIP:      oldIP,
		NewIP:      newIP,
		Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
	}

	m.sendToAll(ctx, event)
}

// sendToAll 并发发送到所有渠道 (独立 context，不受调用方超时影响)
func (m *Manager) sendToAll(_ context.Context, event ChangeEvent) {
	for _, n := range m.notifiers {
		go func(notifier Notifier) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := notifier.Send(ctx, event); err != nil {
				log.Error("通知发送失败",
					"channel", notifier.Name(),
					"domain", event.FullDomain,
					"error", err,
				)
			} else {
				log.Debug("通知发送成功",
					"channel", notifier.Name(),
					"domain", event.FullDomain,
				)
			}
		}(n)
	}
}

// Count 返回已注册的通知器数量
func (m *Manager) Count() int {
	return len(m.notifiers)
}

// Reload 根据当前配置重新加载通知渠道（用于模块化配置变更后同步）
func (m *Manager) Reload(emailCfg EmailConfig, webhookCfg WebhookConfig, extraNotifiers ...Notifier) {
	m.notifiers = nil
	if emailCfg.Enabled {
		m.notifiers = append(m.notifiers, NewEmailNotifier(emailCfg))
		log.Info("通知渠道已注册(热加载)", "name", "email")
	}
	if webhookCfg.Enabled {
		m.notifiers = append(m.notifiers, NewWebhookNotifier(webhookCfg))
		log.Info("通知渠道已注册(热加载)", "name", "webhook")
	}
	for _, n := range extraNotifiers {
		m.notifiers = append(m.notifiers, n)
		name := fmt.Sprintf("%T", n)
		log.Info("通知渠道已注册(热加载)", "name", name)
	}
}
