package qqbot

import (
	"context"
	"fmt"

	"github.com/Yunqingqingxi/yunxi-home/internal/notifier"
)

// NewNotifier 创建 QQ Bot 通知器（实现 notifier.Notifier 接口）
func (b *Bot) NewNotifier() notifier.Notifier {
	return &botNotifier{bot: b}
}

type botNotifier struct {
	bot *Bot
}

func (n *botNotifier) Name() string       { return "qqbot" }
func (n *botNotifier) IsEnabled() bool    { return n.bot != nil }

func (n *botNotifier) Send(ctx context.Context, event notifier.ChangeEvent) error {
	msg := fmt.Sprintf(
		"## DNS 变更通知\n\n- **域名**: %s\n- **类型**: %s\n- **旧 IP**: `%s`\n- **新 IP**: `%s`\n- **时间**: %s",
		event.Domain, event.Type, event.OldIP, event.NewIP, event.Timestamp,
	)
	return n.bot.SendGroupMarkdown(ctx, msg)
}
