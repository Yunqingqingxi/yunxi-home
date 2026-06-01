package config

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/yxd/yunxi-home/internal/database"
)

// Load loads bootstrap config from YAML file + env vars.
func Load(configPath string) (*Config, error) {
	return loadYAML(configPath)
}

// LoadFromDB loads full application config, starting from bootstrap and overlaying DB-stored values.
func LoadFromDB(ctx context.Context, repo database.ConfigRepository, bootstrap *Config) (*Config, error) {
	cfg := bootstrap
	if cfg == nil {
		cfg = DefaultConfig()
	}

	sections, err := repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取数据库配置失败: %w", err)
	}

	if len(sections) == 0 {
		defaults := buildSectionDefaults(cfg)
		if err := repo.InitDefaults(ctx, defaults); err != nil {
			return nil, fmt.Errorf("初始化默认配置失败: %w", err)
		}
		sections, err = repo.GetAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("重新读取配置失败: %w", err)
		}
	}

	overlaySection(sections, "server", &cfg.Server)
	overlaySection(sections, "database", &cfg.Database)
	overlaySection(sections, "alidns", &cfg.AliDNS)
	overlaySection(sections, "detect", &cfg.Detect)
	overlaySection(sections, "notify", &cfg.Notify)
	overlaySection(sections, "auth", &cfg.Auth)
	overlaySection(sections, "ai", &cfg.AI)
	overlaySection(sections, "dns", &cfg.DNS)
	overlaySection(sections, "log", &cfg.Log)
	overlaySection(sections, "qqbot", &cfg.QQBot)
	overlaySection(sections, "nas", &cfg.NAS)
	overlaySection(sections, "terminal", &cfg.Terminal)
	overlaySection(sections, "sysctl", &cfg.Sysctl)
	overlaySection(sections, "dynamic_records", &cfg.DynamicRecords)

	if err := ensureDBPath(cfg); err != nil {
		return nil, err
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func overlaySection(sections map[string]string, key string, target any) {
	if data, ok := sections[key]; ok && data != "" {
		json.Unmarshal([]byte(data), target)
	}
}

func buildSectionDefaults(cfg *Config) map[string]string {
	m := make(map[string]string)
	m["server"] = mustJSON(cfg.Server)
	m["database"] = mustJSON(cfg.Database)
	m["alidns"] = mustJSON(cfg.AliDNS)
	m["detect"] = mustJSON(cfg.Detect)
	m["notify"] = mustJSON(cfg.Notify)
	m["auth"] = mustJSON(cfg.Auth)
	m["ai"] = mustJSON(cfg.AI)
	m["dns"] = mustJSON(cfg.DNS)
	m["log"] = mustJSON(cfg.Log)
	m["qqbot"] = mustJSON(cfg.QQBot)
	m["nas"] = mustJSON(cfg.NAS)
	m["terminal"] = mustJSON(cfg.Terminal)
	m["sysctl"] = mustJSON(cfg.Sysctl)
	m["dynamic_records"] = mustJSON(cfg.DynamicRecords)
	return m
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func SaveSection(ctx context.Context, repo database.ConfigRepository, section string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return repo.SetSection(ctx, section, string(b))
}

func ResolveEncryptionKey() ([]byte, error) {
	// Use a key file to persist the encryption key across restarts.
	// Default path is ./data/.encryption_key relative to working directory.
	keyFile := filepath.Join("data", ".encryption_key")
	if data, err := os.ReadFile(keyFile); err == nil {
		key, decodeErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if decodeErr == nil && len(key) == 32 {
			return key, nil
		}
	}
	// Generate a new key and persist it.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	encoded := base64.RawStdEncoding.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(keyFile), 0755); err != nil {
		return nil, fmt.Errorf("创建加密密钥目录失败: %w", err)
	}
	if err := os.WriteFile(keyFile, []byte(encoded+"\n"), 0600); err != nil {
		return nil, fmt.Errorf("持久化加密密钥失败: %w", err)
	}
	return raw, nil
}

func ensureDBPath(cfg *Config) error {
	absPath, err := filepath.Abs(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}
	cfg.Database.Path = absPath

	dirs := []string{
		cfg.Log.Dir,
		filepath.Dir(absPath),
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

func loadYAML(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	if err := ensureDBPath(cfg); err != nil {
		return nil, err
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

