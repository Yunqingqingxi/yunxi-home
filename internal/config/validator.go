package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

var (
	ErrMissingAccessKeyID     = errors.New("阿里云 AccessKey ID 未配置")
	ErrMissingAccessKeySecret = errors.New("阿里云 AccessKey Secret 未配置")
	ErrMissingAuthPassword    = errors.New("管理后台密码未配置")
	ErrNoDynamicRecords       = errors.New("至少需要配置一条动态 DNS 记录")
	ErrInvalidRecordType      = errors.New("记录类型无效，仅支持 A 和 AAAA")
	ErrInvalidCronExpr        = errors.New("cron 表达式无效")
	ErrInvalidPort            = errors.New("端口号必须在 1-65535 之间")
)

// Validate 校验配置完整性
func Validate(cfg *Config) error {
	var errs []string

	// 服务器配置
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs = append(errs, ErrInvalidPort.Error())
	}

	// JWT Secret: 为空时自动生成随机 32 字节密钥
	if cfg.Auth.JWTSecret == "" {
		secret := generateJWTSecret()
		cfg.Auth.JWTSecret = secret
		slog.Warn("JWT Secret 未配置，已自动生成随机密钥。建议通过环境变量 DNS_UPDATER_AUTH_JWT_SECRET 设置固定值以保证重启后 Token 仍有效。")
	}

	// 阿里云配置（仅在有启用动态记录时需要）
	hasEnabled := false
	for _, r := range cfg.DynamicRecords {
		if r.Enabled {
			hasEnabled = true
			break
		}
	}

	if hasEnabled {
		if cfg.AliDNS.AccessKeyID == "" {
			errs = append(errs, ErrMissingAccessKeyID.Error())
		}
		if cfg.AliDNS.AccessKeySecret == "" {
			errs = append(errs, ErrMissingAccessKeySecret.Error())
		}
	}

	// 认证配置：密码为空时自动生成初始密码
	if cfg.Auth.Password == "" {
		cfg.Auth.Password = "admin123"
		slog.Warn("管理后台密码未配置，已使用默认密码 admin123，请尽快修改！")
	}

	// 动态记录校验
	for i, r := range cfg.DynamicRecords {
		prefix := fmt.Sprintf("dynamic_records[%d]", i)

		if r.DomainName == "" {
			errs = append(errs, prefix+": 域名不能为空")
		}
		if r.RR == "" {
			errs = append(errs, prefix+": 主机记录(RR)不能为空")
		}
		if r.Type != "A" && r.Type != "AAAA" {
			errs = append(errs, prefix+": "+ErrInvalidRecordType.Error())
		}
		if r.Cron == "" {
			errs = append(errs, prefix+": cron 表达式不能为空")
		}
	}

	// 邮件通知校验 — 配置不完整时自动禁用，避免启动失败
	if cfg.Notify.Email.Enabled {
		if cfg.Notify.Email.Host == "" || cfg.Notify.Email.User == "" ||
			cfg.Notify.Email.Password == "" || len(cfg.Notify.Email.To) == 0 {
			slog.Warn("邮件通知配置不完整，已自动禁用，请完善配置后重新启用")
			cfg.Notify.Email.Enabled = false
		}
	}

	// Webhook 通知校验
	if cfg.Notify.Webhook.Enabled && cfg.Notify.Webhook.URL == "" {
		errs = append(errs, "Webhook 通知已启用但 URL 未配置")
	}

	// 钉钉通知校验
	if cfg.Notify.DingTalk.Enabled && cfg.Notify.DingTalk.WebhookURL == "" {
		errs = append(errs, "钉钉通知已启用但 WebhookURL 未配置")
	}

	// 日志校验
	if cfg.Log.MaxDays < 1 || cfg.Log.MaxDays > 365 {
		errs = append(errs, "日志保留天数应在 1-365 之间")
	}

	if len(errs) > 0 {
		return fmt.Errorf("配置校验失败:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// generateJWTSecret 生成 32 字节随机 hex 密钥
func generateJWTSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// 极端情况回退到固定值
		return "yunxi-home-auto-generated-secret-please-change"
	}
	return hex.EncodeToString(b)
}
