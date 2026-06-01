// Package email 提供邮件通知能力，实现 base.Notifier 接口。
package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"log/slog"

	"github.com/yxd/yunxi-home/internal/notifier/base"
)

// Config 邮件配置
type Config struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Password string
	To       []string
}

// Notifier 邮件通知器
type Notifier struct {
	config Config
}

// New 创建邮件通知器
func New(cfg Config) *Notifier {
	return &Notifier{config: cfg}
}

// 编译期检查接口实现
var _ base.Notifier = (*Notifier)(nil)

// Name 通知器名称
func (n *Notifier) Name() string {
	return "email"
}

// IsEnabled 是否启用
func (n *Notifier) IsEnabled() bool {
	return n.config.Enabled
}

// Send 发送邮件通知
func (n *Notifier) Send(ctx context.Context, event base.ChangeEvent) error {
	slog.Info("邮件发送中", "收件人", n.config.To, "域名", event.FullDomain)
	if !n.config.Enabled {
		return nil
	}

	subject := fmt.Sprintf("[Yunxi Home] %s IP 变更通知", event.FullDomain)
	body := buildHTMLBody(event)

	msg := buildMessage(n.config.User, n.config.To, subject, body)

	addr := fmt.Sprintf("%s:%d", n.config.Host, n.config.Port)
	auth := smtp.PlainAuth("", n.config.User, n.config.Password, n.config.Host)

	// 使用带超时的连接
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, n.config.User, n.config.To, []byte(msg))
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("发送邮件失败: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// buildMessage 构造邮件消息
func buildMessage(from string, to []string, subject, body string) string {
	headers := map[string]string{
		"From":         from,
		"To":           strings.Join(to, ", "),
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}

// buildHTMLBody 构造 HTML 邮件正文
func buildHTMLBody(event base.ChangeEvent) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
	<div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 20px; border-radius: 8px 8px 0 0;">
		<h2 style="margin: 0;">🔔 DNS 记录变更通知</h2>
	</div>
	<div style="border: 1px solid #e0e0e0; border-top: none; padding: 20px; border-radius: 0 0 8px 8px;">
		<table style="width: 100%%; border-collapse: collapse;">
			<tr><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #666;">域名</td><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; font-weight: bold;">%s</td></tr>
			<tr><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #666;">记录类型</td><td style="padding: 10px; border-bottom: 1px solid #f0f0f0;">%s</td></tr>
			<tr><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #666;">旧 IP</td><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #e74c3c;">%s</td></tr>
			<tr><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #666;">新 IP</td><td style="padding: 10px; border-bottom: 1px solid #f0f0f0; color: #27ae60; font-weight: bold;">%s</td></tr>
			<tr><td style="padding: 10px; color: #666;">变更时间</td><td style="padding: 10px;">%s</td></tr>
		</table>
		<p style="color: #999; font-size: 12px; margin-top: 20px;">此邮件由 Yunxi Home 自动发送，请勿回复。</p>
	</div>
</body>
</html>`, event.FullDomain, event.Type, event.OldIP, event.NewIP, event.Timestamp)
}
