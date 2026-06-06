package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

func defaultSandboxRoot() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".yunxi", "data", "yunxiFiles")
	}
	return "/opt/yunxi-home/data/yunxiFiles"
}

type ServerConfig struct {
	Host            string   `mapstructure:"host" json:"host"`
	Port            int      `mapstructure:"port" json:"port"`
	ShutdownTimeout int      `mapstructure:"shutdown_timeout" json:"shutdown_timeout"`
	RateLimit       int      `mapstructure:"rate_limit" json:"rate_limit"`
	AllowedOrigins  []string `mapstructure:"allowed_origins" json:"allowed_origins"`
}

type DatabaseConfig struct {
	Driver string       `mapstructure:"driver" json:"driver"` // "sqlite" | "mysql" | "file"
	Path   string       `mapstructure:"path" json:"path"`     // sqlite: db文件路径, file: 数据目录
	MySQL  *MySQLConfig `mapstructure:"mysql,omitempty" json:"mysql,omitempty"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	User     string `mapstructure:"user" json:"user"`
	Password string `mapstructure:"password" json:"password"`
	DBName   string `mapstructure:"dbname" json:"dbname"`
}

type AliDNSConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id" json:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret" json:"access_key_secret"`
	Endpoint        string `mapstructure:"endpoint" json:"endpoint"`
	RegionID        string `mapstructure:"region_id" json:"region_id"`
}

type DNSConfig struct {
	Aliyun AliDNSConfig `mapstructure:"aliyun" json:"aliyun"`
}

type DetectConfig struct {
	Interval    string   `mapstructure:"interval" json:"interval"`
	IPv6Enabled bool     `mapstructure:"ipv6_enabled" json:"ipv6_enabled"`
	IPv4Enabled bool     `mapstructure:"ipv4_enabled" json:"ipv4_enabled"`
	DNSServers  []string `mapstructure:"dns_servers" json:"dns_servers,omitempty"`
	Providers   []string `mapstructure:"providers" json:"providers"`
}

type EmailConfig struct {
	Enabled  bool     `mapstructure:"enabled" json:"enabled"`
	Host     string   `mapstructure:"host" json:"host"`
	Port     int      `mapstructure:"port" json:"port"`
	User     string   `mapstructure:"user" json:"user"`
	Password string   `mapstructure:"password" json:"password"`
	To       []string `mapstructure:"to" json:"to"`
}

type WebhookConfig struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	URL     string `mapstructure:"url" json:"url"`
	Secret  string `mapstructure:"secret" json:"secret"`
}

type DingTalkConfig struct {
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
	WebhookURL string `mapstructure:"webhook_url" json:"webhook_url"`
	Secret     string `mapstructure:"secret" json:"secret"`
}

type NotifyConfig struct {
	Email    EmailConfig    `mapstructure:"email" json:"email"`
	Webhook  WebhookConfig  `mapstructure:"webhook" json:"webhook"`
	DingTalk DingTalkConfig `mapstructure:"dingtalk" json:"dingtalk"`
}

type AuthConfig struct {
	Username  string `mapstructure:"username" json:"username"`
	Password  string `mapstructure:"password" json:"password"`
	JWTSecret string `mapstructure:"jwt_secret" json:"jwt_secret"`
}

type AIProviderConfig struct {
	Enabled bool   `mapstructure:"enabled" json:"enabled"`
	APIKey  string `mapstructure:"api_key" json:"api_key"`
}

type AIConfig struct {
	Providers              map[string]AIProviderConfig `json:"-" mapstructure:"-"`
	DefaultModel           string                      `json:"default_model" mapstructure:"default_model"`
	DefaultReasoning       string                      `json:"default_reasoning" mapstructure:"default_reasoning"` // low|medium|high
	ExpandThinkingOnStream bool                        `json:"expand_thinking_on_stream" mapstructure:"expand_thinking_on_stream"`
	SkillsDir              string                      `json:"skills_dir,omitempty" mapstructure:"skills_dir"` // 技能 YAML 目录路径
}

// MarshalJSON produces a flat JSON object where provider keys (e.g. "deepseek", "qwen")
// are at the same level as scalar fields like "default_model".
func (a AIConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(a.Providers)+4)
	for k, v := range a.Providers {
		m[k] = v
	}
	m["default_model"] = a.DefaultModel
	m["default_reasoning"] = a.DefaultReasoning
	m["expand_thinking_on_stream"] = a.ExpandThinkingOnStream
	if a.SkillsDir != "" {
		m["skills_dir"] = a.SkillsDir
	}
	return json.Marshal(m)
}

// UnmarshalJSON parses a flat JSON object, placing object-typed entries into
// Providers and extracting known scalar fields.
func (a *AIConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	a.Providers = make(map[string]AIProviderConfig)
	for k, v := range raw {
		switch k {
		case "default_model":
			if err := json.Unmarshal(v, &a.DefaultModel); err != nil {
				return err
			}
		case "default_reasoning":
			if err := json.Unmarshal(v, &a.DefaultReasoning); err != nil {
				return err
			}
		case "expand_thinking_on_stream":
			if err := json.Unmarshal(v, &a.ExpandThinkingOnStream); err != nil {
				return err
			}
		case "skills_dir":
			if err := json.Unmarshal(v, &a.SkillsDir); err != nil {
				return err
			}
		default:
			trimmed := bytes.TrimSpace(v)
			if len(trimmed) > 0 && trimmed[0] == '{' {
				var p AIProviderConfig
				if err := json.Unmarshal(v, &p); err != nil {
					return err
				}
				a.Providers[k] = p
			}
		}
	}
	return nil
}

type QQBotConfig struct {
	Enabled   bool   `mapstructure:"enabled" json:"enabled"`
	AppID     string `mapstructure:"app_id" json:"app_id"`
	AppSecret string `mapstructure:"app_secret" json:"app_secret,omitempty"`
	GroupID   string `mapstructure:"group_id" json:"group_id"`
	Username  string `mapstructure:"username" json:"username"`
	Avatar    string `mapstructure:"avatar" json:"avatar"`
}

type LogConfig struct {
	Level   string `mapstructure:"level" json:"level"`
	Dir     string `mapstructure:"dir" json:"dir"`
	MaxDays int    `mapstructure:"max_days" json:"max_days"`
	Format  string `mapstructure:"format" json:"format"` // "text" (default) or "json"
}

type DynamicRecordConfig struct {
	DomainName string `mapstructure:"domain_name" json:"domain_name"`
	RR         string `mapstructure:"rr" json:"rr"`
	Type       string `mapstructure:"type" json:"type"`
	TTL        int    `mapstructure:"ttl" json:"ttl"`
	Cron       string `mapstructure:"cron" json:"cron"`
	Enabled    bool   `mapstructure:"enabled" json:"enabled"`
}

type NASConfig struct {
	Enabled        bool     `mapstructure:"enabled" json:"enabled"`
	RootDir        string   `mapstructure:"root_dir" json:"root_dir"`
	AllowedDirs    []string `mapstructure:"allowed_dirs" json:"allowed_dirs,omitempty"`
	SandboxRoot    string   `mapstructure:"sandbox_root" json:"sandbox_root"`
	DownloadSecret string   `mapstructure:"download_secret" json:"download_secret,omitempty"`
}

type TerminalConfig struct {
	Enabled   bool `mapstructure:"enabled" json:"enabled"`
	AdminOnly bool `mapstructure:"admin_only" json:"admin_only"`
}

type SysctlConfig struct {
	Enabled        bool `mapstructure:"enabled" json:"enabled"`
	ServiceControl bool `mapstructure:"service_control" json:"service_control"`
	ProcessControl bool `mapstructure:"process_control" json:"process_control"`
}

// PathsConfig holds filesystem paths for subsystems that use file-driven config.
type PathsConfig struct {
	MCPConfig string `mapstructure:"mcp_config" json:"mcp_config"` // mcp.json 路径
	Skills    string `mapstructure:"skills" json:"skills"`          // 技能 YAML/MD 目录
	Memory    string `mapstructure:"memory" json:"memory"`          // 记忆 .md 文件目录
}

type Config struct {
	Server         ServerConfig          `mapstructure:"server" json:"server"`
	Database       DatabaseConfig        `mapstructure:"database" json:"database"`
	AliDNS         AliDNSConfig          `mapstructure:"alidns" json:"alidns"`
	DNS            DNSConfig             `mapstructure:"dns" json:"dns"`
	Detect         DetectConfig          `mapstructure:"detect" json:"detect"`
	Notify         NotifyConfig          `mapstructure:"notify" json:"notify"`
	Auth           AuthConfig            `mapstructure:"auth" json:"auth"`
	Log            LogConfig             `mapstructure:"log" json:"log"`
	AI             AIConfig              `mapstructure:"ai" json:"ai"`
	aiLegacy       AIConfig              `mapstructure:"ai_legacy"`
	QQBot          QQBotConfig           `mapstructure:"qqbot" json:"qqbot"`
	NAS            NASConfig             `mapstructure:"nas" json:"nas"`
	Terminal       TerminalConfig        `mapstructure:"terminal" json:"terminal"`
	Sysctl         SysctlConfig          `mapstructure:"sysctl" json:"sysctl"`
	Paths          PathsConfig           `mapstructure:"paths" json:"paths"`
	DynamicRecords []DynamicRecordConfig `mapstructure:"dynamic_records" json:"dynamic_records"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            9981,
			ShutdownTimeout: 30,
			RateLimit:       20,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "./data/yunxi-home.db",
		},
		AliDNS: AliDNSConfig{
			Endpoint: "alidns.aliyuncs.com",
			RegionID: "cn-hangzhou",
		},
		Detect: DetectConfig{
			Interval:    "*/5 * * * *",
			IPv6Enabled: true,
			IPv4Enabled: false,
			Providers:   []string{"api6", "ifconfig", "local"},
			DNSServers:  []string{"223.5.5.5:53", "223.6.6.6:53"},
		},
		Notify: NotifyConfig{
			Email: EmailConfig{Enabled: false, Host: "smtp.qq.com", Port: 587},
		},
		Auth: AuthConfig{Username: "admin"},
		AI: AIConfig{
			Providers: map[string]AIProviderConfig{
				"deepseek": {Enabled: false},
				"qwen":     {Enabled: false},
			},
			DefaultModel:     "deepseek-v4-flash",
			DefaultReasoning: "high",
		},
		Log:  LogConfig{Level: "info", Dir: "./log", MaxDays: 30, Format: "text"},
		NAS: NASConfig{
			Enabled:     true,
			RootDir:     "/",
			AllowedDirs: []string{"/"},
			SandboxRoot: defaultSandboxRoot(),
		},
		Terminal: TerminalConfig{Enabled: true, AdminOnly: true},
		Sysctl:   SysctlConfig{Enabled: true, ServiceControl: true, ProcessControl: true},
		Paths: PathsConfig{
			MCPConfig: "mcp.json",
			Skills:    "skills",
			Memory:    "memory",
		},
	}
}
