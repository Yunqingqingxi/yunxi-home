package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/config"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"github.com/Yunqingqingxi/yunxi-home/internal/notifier"
)

type ConfigHandler struct {
	cfg            *config.Config
	repo           database.ConfigRepository
	nm             *notifier.Manager
	botInfoFunc    func() []map[string]any
	onQQBotChanged func() // qqbot 配置变更时回调
	onAIChanged    func() // AI provider 配置变更时回调
}

func NewConfigHandler(cfg *config.Config, repo database.ConfigRepository, nm *notifier.Manager) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, repo: repo, nm: nm}
}

func (h *ConfigHandler) SetOnQQBotChanged(fn func()) { h.onQQBotChanged = fn }
func (h *ConfigHandler) SetOnAIChanged(fn func())     { h.onAIChanged = fn }

func (h *ConfigHandler) SetBotInfoProvider(fn func() []map[string]any) { h.botInfoFunc = fn }

func (h *ConfigHandler) GetConfig(c echo.Context) error {
	result := buildConfigView(h.cfg)
	// Merge runtime bot info
	if h.botInfoFunc != nil {
		infos := h.botInfoFunc()
		if len(infos) > 0 {
			if botData, ok := result["qqbot"].(map[string]any); ok {
				info := infos[0]
				if u, ok := info["username"].(string); ok && u != "" {
					botData["username"] = u
				}
				if a, ok := info["avatar"].(string); ok && a != "" {
					botData["avatar"] = a
				}
				if on, ok := info["online"]; ok {
					botData["online"] = on
				}
			}
		}
	}
	return c.JSON(http.StatusOK, successResp(result))
}

var validSections = map[string]bool{
	"server": true, "database": true, "detect": true,
	"notify": true, "auth": true, "ai": true, "qqbot": true, "log": true, "dns": true,
	"nas": true, "terminal": true, "sysctl": true, "dynamic_records": true,
}

func (h *ConfigHandler) GetSection(c echo.Context) error {
	section := c.Param("section")
	if !validSections[section] {
		return c.JSON(http.StatusBadRequest, errorResp("无效的配置分类: "+section))
	}
	data, _ := json.Marshal(h.cfg)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if sec, ok := raw[section]; ok {
		if secMap, ok := sec.(map[string]any); ok {
			maskSectionSecrets(section, secMap)
			return c.JSON(http.StatusOK, successResp(secMap))
		}
		return c.JSON(http.StatusOK, successResp(sec))
	}
	return c.JSON(http.StatusNotFound, errorResp("未找到配置: "+section))
}

// TestAISection POST /api/config/ai/test — test AI provider keys without saving.
func (h *ConfigHandler) TestAISection(c echo.Context) error {
	var body map[string]any
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	h.preserveMaskedSecrets("ai", body)
	// Apply to in-memory config temporarily
	prevProviders := make(map[string]config.AIProviderConfig)
	for k, v := range h.cfg.AI.Providers {
		prevProviders[k] = v
	}
	reloadAISection(h.cfg, body)
	testResults := h.testAIProviders(c.Request().Context(), false)
	// Restore original config
	h.cfg.AI.Providers = prevProviders
	return c.JSON(http.StatusOK, successResp(map[string]any{
		"tests": testResults,
	}))
}

func (h *ConfigHandler) UpdateSection(c echo.Context) error {
	section := c.Param("section")
	if !validSections[section] {
		return c.JSON(http.StatusBadRequest, errorResp("无效的配置分类: "+section))
	}
	var body map[string]any
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}

	// ai 变更：先测试连接，通过后才保存
	if section == "ai" {
		h.preserveMaskedSecrets(section, body)
		reloadAISection(h.cfg, body)
		testResults := h.testAIProviders(c.Request().Context(), false)
		allPassed := true
		for _, r := range testResults {
			if !r["enabled"].(bool) {
				allPassed = false
			}
		}
		if !allPassed {
			return c.JSON(http.StatusOK, successResp(map[string]any{
				"message": "连接测试失败，配置未保存",
				"tests":   testResults,
			}))
		}
		// 测试通过，持久化完整配置（防止丢失其他 Provider）
		data, _ := json.Marshal(h.cfg.AI)
		h.repo.SetSection(c.Request().Context(), section, string(data))
		h.reloadNotifiers()
		if h.onAIChanged != nil {
			h.onAIChanged()
		}
		return c.JSON(http.StatusOK, successResp(map[string]any{
			"message": "配置已更新",
			"tests":   testResults,
		}))
	}

	// 掩码值保护：空值或占位符不覆盖真实密钥
	h.preserveMaskedSecrets(section, body)
	data, err := json.Marshal(body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("序列化配置失败"))
	}
	if err := h.repo.SetSection(c.Request().Context(), section, string(data)); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("保存配置失败"))
	}
	reloadSection(h.cfg, section, body)
	// 日志级别即时生效（无需重启）
	if section == "log" {
		if lvl, ok := body["level"].(string); ok {
			logger.SetLevel(lvl)
		}
	}
	h.reloadNotifiers()
	// qqbot 变更后回调（重启 Bot + 刷新信息）
	if section == "qqbot" && h.onQQBotChanged != nil {
		h.onQQBotChanged()
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "配置已更新"}))
}

func (h *ConfigHandler) UpdateConfig(c echo.Context) error {
	var body map[string]json.RawMessage
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请求参数无效"))
	}
	ctx := c.Request().Context()
	for section, raw := range body {
		if !validSections[section] {
			continue
		}
		var data map[string]any
		json.Unmarshal(raw, &data)
		// 掩码值保护：空值或占位符不覆盖真实密钥
		h.preserveMaskedSecrets(section, data)
		// 重新序列化以保留保护后的值
		protected, _ := json.Marshal(data)
		if err := h.repo.SetSection(ctx, section, string(protected)); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("保存 "+section+" 失败"))
		}
		reloadSection(h.cfg, section, data)
		// qqbot 变更后回调（重启 Bot + 刷新信息）
		if section == "qqbot" && h.onQQBotChanged != nil {
			h.onQQBotChanged()
		}
	}
	h.reloadNotifiers()
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "配置已更新"}))
}

func reloadSection(cfg *config.Config, section string, data map[string]any) {
	b, _ := json.Marshal(data)
	switch section {
	case "server":
		json.Unmarshal(b, &cfg.Server)
	case "database":
		json.Unmarshal(b, &cfg.Database)
	case "detect":
		json.Unmarshal(b, &cfg.Detect)
	case "notify":
		json.Unmarshal(b, &cfg.Notify)
	case "auth":
		json.Unmarshal(b, &cfg.Auth)
	case "ai":
		reloadAISection(cfg, data)
	case "qqbot":
		json.Unmarshal(b, &cfg.QQBot)
	case "dns":
		json.Unmarshal(b, &cfg.DNS)
			// 双向同步: dns.aliyun -> alidns
			if cfg.DNS.Aliyun.AccessKeyID != "" { cfg.AliDNS.AccessKeyID = cfg.DNS.Aliyun.AccessKeyID }
			if cfg.DNS.Aliyun.AccessKeySecret != "" { cfg.AliDNS.AccessKeySecret = cfg.DNS.Aliyun.AccessKeySecret }
			if cfg.DNS.Aliyun.Endpoint != "" { cfg.AliDNS.Endpoint = cfg.DNS.Aliyun.Endpoint }
	case "log":
		json.Unmarshal(b, &cfg.Log)
	case "nas":
		json.Unmarshal(b, &cfg.NAS)
	case "terminal":
		json.Unmarshal(b, &cfg.Terminal)
	case "sysctl":
		json.Unmarshal(b, &cfg.Sysctl)
	case "dynamic_records":
		json.Unmarshal(b, &cfg.DynamicRecords)
	}
}

// reloadAISection merges partial AI provider updates into the existing config.
// It only updates the providers present in data, preserving others.
func reloadAISection(cfg *config.Config, data map[string]any) {
	if cfg.AI.Providers == nil {
		cfg.AI.Providers = make(map[string]config.AIProviderConfig)
	}
	for key, val := range data {
		switch key {
		case "default_model":
			if s, ok := val.(string); ok {
				cfg.AI.DefaultModel = s
			}
		case "default_reasoning":
			if s, ok := val.(string); ok {
				cfg.AI.DefaultReasoning = s
			}
		case "skills_dir":
			if s, ok := val.(string); ok {
				cfg.AI.SkillsDir = s
			}
		default:
			// Provider config: merge into existing
			b, _ := json.Marshal(val)
			var pcfg config.AIProviderConfig
			json.Unmarshal(b, &pcfg)
			// Preserve existing API key if the incoming is masked
			if isMasked(pcfg.APIKey) {
				if existing, ok := cfg.AI.Providers[key]; ok {
					pcfg.APIKey = existing.APIKey
				}
			}
			cfg.AI.Providers[key] = pcfg
		}
	}
}

func isMasked(s string) bool {
	return s == "" || strings.HasPrefix(s, "••••") || s == "••••••••"
}

func (h *ConfigHandler) preserveMaskedSecrets(section string, body map[string]any) {
	switch section {
	case "qqbot":
		if clear, _ := body["_clear_secret"].(bool); clear {
			delete(body, "_clear_secret")
		} else if s, _ := body["app_secret"].(string); isMasked(s) && h.cfg.QQBot.AppSecret != "" {
			body["app_secret"] = h.cfg.QQBot.AppSecret
		}
	case "dns":
		if ali, ok := body["aliyun"].(map[string]any); ok {
			if clear, _ := ali["_clear_secret"].(bool); clear {
				delete(ali, "_clear_secret")
			} else if s, _ := ali["access_key_secret"].(string); isMasked(s) && h.cfg.DNS.Aliyun.AccessKeySecret != "" {
				ali["access_key_secret"] = h.cfg.DNS.Aliyun.AccessKeySecret
			}
		}
	case "notify":
		if email, ok := body["email"].(map[string]any); ok {
			if clear, _ := email["_clear_password"].(bool); clear {
				delete(email, "_clear_password")
			} else if s, _ := email["password"].(string); isMasked(s) && h.cfg.Notify.Email.Password != "" {
				email["password"] = h.cfg.Notify.Email.Password
			}
		}
		if webhook, ok := body["webhook"].(map[string]any); ok {
			if s, _ := webhook["secret"].(string); isMasked(s) && h.cfg.Notify.Webhook.Secret != "" {
				webhook["secret"] = h.cfg.Notify.Webhook.Secret
			}
		}
		if dingtalk, ok := body["dingtalk"].(map[string]any); ok {
			if s, _ := dingtalk["secret"].(string); isMasked(s) && h.cfg.Notify.DingTalk.Secret != "" {
				dingtalk["secret"] = h.cfg.Notify.DingTalk.Secret
			}
		}
	case "ai":
		for key, val := range body {
			if m, ok := val.(map[string]any); ok {
				if clear, _ := m["_clear_key"].(bool); clear {
					delete(m, "_clear_key")
				} else if s, _ := m["api_key"].(string); isMasked(s) {
					if providerCfg, exists := h.cfg.AI.Providers[key]; exists && providerCfg.APIKey != "" {
						m["api_key"] = providerCfg.APIKey
					}
				}
			}
		}
	case "database":
		if mysql, ok := body["mysql"].(map[string]any); ok {
			if s, _ := mysql["password"].(string); isMasked(s) && h.cfg.Database.MySQL != nil && h.cfg.Database.MySQL.Password != "" {
				mysql["password"] = h.cfg.Database.MySQL.Password
			}
		}
	case "auth":
		if s, _ := body["password"].(string); isMasked(s) && h.cfg.Auth.Password != "" {
			body["password"] = h.cfg.Auth.Password
		}
	}
}

func maskSectionSecrets(section string, data map[string]any) {
	switch section {
	case "notify":
		if email, ok := data["email"].(map[string]any); ok {
			if v, ok := email["password"]; ok {
				delete(email, "password")
				email["has_password"] = v != ""
			}
		}
		if webhook, ok := data["webhook"].(map[string]any); ok {
			if v, ok := webhook["secret"]; ok {
				delete(webhook, "secret")
				webhook["has_secret"] = v != ""
			}
		}
		if dingtalk, ok := data["dingtalk"].(map[string]any); ok {
			if v, ok := dingtalk["secret"]; ok {
				delete(dingtalk, "secret")
				dingtalk["has_secret"] = v != ""
			}
		}
	case "qqbot":
		if v, ok := data["app_secret"]; ok {
			delete(data, "app_secret")
			data["has_secret"] = v != ""
		}
	case "ai":
		for _, val := range data {
			if m, ok := val.(map[string]any); ok {
				if v, exists := m["api_key"]; exists {
					delete(m, "api_key")
					m["has_key"] = v != ""
				}
			}
		}
	case "dns":
		if aliyun, ok := data["aliyun"].(map[string]any); ok {
			if v, ok := aliyun["access_key_secret"]; ok {
				delete(aliyun, "access_key_secret")
				aliyun["has_secret"] = v != ""
			}
		}
	case "database":
		if mysql, ok := data["mysql"].(map[string]any); ok {
			if v, ok := mysql["password"]; ok {
				delete(mysql, "password")
				mysql["has_password"] = v != ""
			}
		}
	case "auth":
		if v, ok := data["password"]; ok {
			delete(data, "password")
			data["has_password"] = v != ""
		}
	}
}

func buildConfigView(cfg *config.Config) map[string]any {
	// 构建 dns 视图：聚合旧 alidns 数据到 dns.aliyun（不修改 cfg 指针）
	dnsView := buildDNSView(cfg)
	// 构建 database 视图：掩码 MySQL 密码
	databaseView := buildDatabaseView(cfg)
	// 构建 notify 视图：掩码各渠道密钥
	notifyView := buildNotifyView(cfg)

	return map[string]any{
		"server":   cfg.Server,
		"database": databaseView,
		"dns":      dnsView,
		"detect":   cfg.Detect,
		"notify":   notifyView,
		"auth": map[string]any{
			"username":     cfg.Auth.Username,
			"has_password": cfg.Auth.Password != "",
		},
		"ai": func() map[string]any {
			m := make(map[string]any, len(cfg.AI.Providers)+2)
			for pkey, pcfg := range cfg.AI.Providers {
				m[pkey] = map[string]any{
					"enabled":  pcfg.Enabled,
					"has_key":  pcfg.APIKey != "",
				}
			}
			m["default_model"] = cfg.AI.DefaultModel
			m["default_reasoning"] = cfg.AI.DefaultReasoning
			return m
		}(),
		"qqbot": map[string]any{
			"enabled":    cfg.QQBot.Enabled,
			"app_id":     cfg.QQBot.AppID,
			"group_id":   cfg.QQBot.GroupID,
			"username":   cfg.QQBot.Username,
			"avatar":     cfg.QQBot.Avatar,
			"has_secret": cfg.QQBot.AppSecret != "",
		},
		"nas": map[string]any{
			"enabled":      cfg.NAS.Enabled,
			"root_dir":     cfg.NAS.RootDir,
			"sandbox_root": cfg.NAS.SandboxRoot,
		},
		"log":            cfg.Log,
		"terminal":       cfg.Terminal,
		"sysctl":         cfg.Sysctl,
		"dynamic_records": cfg.DynamicRecords,
	}
}

// buildDNSView 构建 DNS 视图（掩码密钥，不修改 cfg 指针）
func buildDNSView(cfg *config.Config) map[string]any {
	ali := cfg.DNS.Aliyun
	// 旧 alidns 数据聚合到 dns.aliyun（仅在 dns.aliyun 为空时回退）
	if ali.AccessKeyID == "" && cfg.AliDNS.AccessKeyID != "" {
		ali = cfg.AliDNS
	}
	return map[string]any{
		"aliyun": map[string]any{
			"access_key_id": ali.AccessKeyID,
			"endpoint":      ali.Endpoint,
			"region_id":     ali.RegionID,
			"has_secret":    ali.AccessKeySecret != "",
		},
	}
}

// buildDatabaseView 构建数据库视图（掩码 MySQL 密码）
func buildDatabaseView(cfg *config.Config) map[string]any {
	result := map[string]any{
		"path": cfg.Database.Path,
	}
	if cfg.Database.MySQL != nil {
		result["mysql"] = map[string]any{
			"host":         cfg.Database.MySQL.Host,
			"port":         cfg.Database.MySQL.Port,
			"user":         cfg.Database.MySQL.User,
			"dbname":       cfg.Database.MySQL.DBName,
			"has_password": cfg.Database.MySQL.Password != "",
		}
	}
	return result
}

// buildNotifyView 构建通知视图（掩码各渠道密钥）
func buildNotifyView(cfg *config.Config) map[string]any {
	return map[string]any{
		"email": map[string]any{
			"enabled":      cfg.Notify.Email.Enabled,
			"host":         cfg.Notify.Email.Host,
			"port":         cfg.Notify.Email.Port,
			"user":         cfg.Notify.Email.User,
			"to":           cfg.Notify.Email.To,
			"has_password": cfg.Notify.Email.Password != "",
		},
		"webhook": map[string]any{
			"enabled":    cfg.Notify.Webhook.Enabled,
			"url":        cfg.Notify.Webhook.URL,
			"has_secret": cfg.Notify.Webhook.Secret != "",
		},
		"dingtalk": map[string]any{
			"enabled":     cfg.Notify.DingTalk.Enabled,
			"webhook_url": cfg.Notify.DingTalk.WebhookURL,
			"has_secret":  cfg.Notify.DingTalk.Secret != "",
		},
	}
}

func (h *ConfigHandler) reloadNotifiers() {
	if h.nm != nil {
		emailCfg := notifier.EmailConfig{
			Enabled:  h.cfg.Notify.Email.Enabled,
			Host:     h.cfg.Notify.Email.Host,
			Port:     h.cfg.Notify.Email.Port,
			User:     h.cfg.Notify.Email.User,
			Password: h.cfg.Notify.Email.Password,
			To:       h.cfg.Notify.Email.To,
		}
		webhookCfg := notifier.WebhookConfig{
			Enabled: h.cfg.Notify.Webhook.Enabled,
			URL:     h.cfg.Notify.Webhook.URL,
			Secret:  h.cfg.Notify.Webhook.Secret,
		}
		h.nm.Reload(emailCfg, webhookCfg)
	}
}

var aiProviderBaseURLs = map[string]string{
	"deepseek": "https://api.deepseek.com",
	"qwen":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
}

// testAIProviders tests each AI provider with a non-empty API key and sets its Enabled.
func (h *ConfigHandler) testAIProviders(ctx context.Context, persist bool) map[string]map[string]any {
	results := make(map[string]map[string]any)
	for key, pcfg := range h.cfg.AI.Providers {
		if pcfg.APIKey == "" {
			if persist {
				h.cfg.AI.Providers[key] = config.AIProviderConfig{APIKey: ""}
			}
			continue
		}
		baseURL, ok := aiProviderBaseURLs[key]
		if !ok {
			continue
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
		req.Header.Set("Authorization", "Bearer "+pcfg.APIKey)
		resp, err := http.DefaultClient.Do(req)
		ok2 := err == nil && resp != nil && resp.StatusCode < 400
		if resp != nil {
			resp.Body.Close()
		}
		if persist {
			h.cfg.AI.Providers[key] = config.AIProviderConfig{
				Enabled: ok2,
				APIKey:  pcfg.APIKey,
			}
		}
		results[key] = map[string]any{"enabled": ok2}
		if !ok2 {
			msg := "连接失败"
			if err != nil {
				msg = err.Error()
			} else if resp != nil {
				msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			results[key]["error"] = msg
		}
	}
	if persist && len(results) > 0 {
		config.SaveSection(ctx, h.repo, "ai", h.cfg.AI)
	}
	return results
}
