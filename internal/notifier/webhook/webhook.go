// Package webhook 提供 Webhook 通知能力，实现 base.Notifier 接口。
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"

	"github.com/Yunqingqingxi/yunxi-home/internal/notifier/base"
)

var log = logger.ForComponent("notifier")

// Config Webhook 配置
type Config struct {
	Enabled bool
	URL     string
	Secret  string // HMAC 签名密钥
	Method  string // GET / POST
	Headers map[string]string
}

// Notifier Webhook 通知器
type Notifier struct {
	config Config
	client *http.Client
}

// New 创建 Webhook 通知器
func New(cfg Config) *Notifier {
	return &Notifier{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// 编译期检查接口实现
var _ base.Notifier = (*Notifier)(nil)

// Name 通知器名称
func (n *Notifier) Name() string {
	return "webhook"
}

// IsEnabled 是否启用
func (n *Notifier) IsEnabled() bool {
	return n.config.Enabled
}

// Send 发送 Webhook 通知
func (n *Notifier) Send(ctx context.Context, event base.ChangeEvent) error {
	log.Info("Webhook发送中", "URL", n.config.URL, "域名", event.FullDomain)
	if !n.config.Enabled {
		return nil
	}

	payload, err := json.Marshal(map[string]interface{}{
		"event":     "dns_update",
		"domain":    event.FullDomain,
		"type":      event.Type,
		"old_ip":    event.OldIP,
		"new_ip":    event.NewIP,
		"timestamp": event.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("序列化 Webhook 请求体失败: %w", err)
	}

	method := n.config.Method
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, n.config.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建 Webhook 请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Yunxi-Home/3.0")

	// 自定义请求头
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}

	// HMAC 签名（如果配置了 Secret）
	if n.config.Secret != "" {
		sig := sign(n.config.Secret, payload)
		req.Header.Set("X-Signature", sig)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Webhook 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Webhook 返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// sign 生成 HMAC-SHA256 签名
func sign(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
