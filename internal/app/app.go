// Package app provides the application container that wires all subsystems together
// and manages their lifecycle (startup, runtime, graceful shutdown).
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/adapt"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/deepseek"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/intent"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/memory"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/provider"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/qwen"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/query"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/topology"
	"github.com/Yunqingqingxi/yunxi-home/internal/config"
	"github.com/Yunqingqingxi/yunxi-home/internal/crypto"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/dns"
	"github.com/Yunqingqingxi/yunxi-home/internal/docker"
	"github.com/Yunqingqingxi/yunxi-home/internal/ipdetect"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
	"github.com/Yunqingqingxi/yunxi-home/internal/notifier"
	"github.com/Yunqingqingxi/yunxi-home/internal/qqbot"
	"github.com/Yunqingqingxi/yunxi-home/internal/scheduler"
	"github.com/Yunqingqingxi/yunxi-home/internal/sysctl"
	"github.com/Yunqingqingxi/yunxi-home/internal/toolreg"
	"github.com/Yunqingqingxi/yunxi-home/internal/web"
)

// App is the application container that owns all subsystems and manages their lifecycle.
type App struct {
	Config        *config.Config
	EncryptedRepo database.ConfigRepository

	// Database
	SQLiteDB *database.DB
	Backend  *database.Backend

	// Core services
	Detector  ipdetect.Detector
	DNSClient dns.Provider
	Scheduler *scheduler.Scheduler
	Notifier  *notifier.Manager
	Throttler *notifier.Throttler

	// Optional services
	Bot       *qqbot.Bot
	AIService *ai.Service
	SandboxFS nas.FileService
	DockerMgr *docker.Manager

	// Infrastructure
	Collector *sysctl.SystemCollector
	Server    *web.Server
	ProvReg   *provider.Registry

	// Repositories
	DomainRepo  database.DomainRepository
	HistoryRepo database.HistoryRepository
	UserRepo    database.UserRepository
	ShareRepo   database.ShareRepository
	PermRepo    database.FilePermissionRepository

	onShutdown []func()
}

// New creates and initializes all subsystems. Returns a fully wired App ready to Start.
func New(configPath string) (*App, error) {
	app := &App{}

	// 1. Load bootstrap config
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}
	bootstrapCfg := cfg

	// 2. Initialize logging
	if _, err := logger.Init(cfg.Log.Level, cfg.Log.Dir, cfg.Log.Format); err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	slog.Info("Yunxi Home v3.0.0 启动中...")
	slog.Info("配置加载完成",
		"host", cfg.Server.Host, "port", cfg.Server.Port,
		"records", len(cfg.DynamicRecords),
		"nas", cfg.NAS.Enabled, "terminal", cfg.Terminal.Enabled,
		"sysctl", cfg.Sysctl.Enabled,
	)

	if err := logger.CleanOldLogs(cfg.Log.Dir, cfg.Log.MaxDays); err != nil {
		slog.Warn("清理旧日志失败", "error", err)
	}

	// 3. Initialize SQLite (primary DB)
	sqliteDB, err := database.New(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("初始化 SQLite 失败: %w", err)
	}
	app.SQLiteDB = sqliteDB
	slog.Info("SQLite 已连接", "path", cfg.Database.Path)

	// 3a. Config repository with encryption
	configRepo := database.NewConfigRepo(sqliteDB)
	encKey, err := config.ResolveEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("解析加密密钥失败: %w", err)
	}
	encryptedRepo := crypto.NewEncryptedConfigRepo(configRepo, encKey)
	app.EncryptedRepo = encryptedRepo

	cfg, err = config.LoadFromDB(context.Background(), encryptedRepo, bootstrapCfg)
	if err != nil {
		return nil, fmt.Errorf("从数据库加载配置失败: %w", err)
	}
	slog.Info("数据库配置加载完成")
	app.Config = cfg

	// 3b. Data storage backend
	backend, err := app.initBackend(sqliteDB)
	if err != nil {
		return nil, fmt.Errorf("初始化存储后端失败: %w", err)
	}
	app.Backend = backend
	slog.Info("存储后端已启用", "driver", backend.Driver)

	app.DomainRepo = backend.DomainRepo
	app.HistoryRepo = backend.HistoryRepo
	app.UserRepo = backend.UserRepo

	// 4. Initialize core services
	if err := app.initCoreServices(sqliteDB); err != nil {
		return nil, err
	}

	// 5. Initialize optional services
	app.initOptionalServices(sqliteDB)

	// 6. Wire HTTP server
	if err := app.initHTTPServer(sqliteDB); err != nil {
		return nil, err
	}

	return app, nil
}

func (a *App) initBackend(sqliteDB *database.DB) (*database.Backend, error) {
	cfg := a.Config
	backendCfg := database.BackendConfig{
		Driver: cfg.Database.Driver,
		Path:   cfg.Database.Path,
	}
	if cfg.Database.MySQL != nil {
		backendCfg.MySQLCfg = &database.MySQLConfig{
			Host:     cfg.Database.MySQL.Host,
			Port:     cfg.Database.MySQL.Port,
			User:     cfg.Database.MySQL.User,
			Password: cfg.Database.MySQL.Password,
			DBName:   cfg.Database.MySQL.DBName,
		}
	}

	if backendCfg.Driver == "" || backendCfg.Driver == "sqlite" {
		return database.NewSQLiteBackendWithDB(sqliteDB), nil
	}

	bk, err := database.NewBackend(backendCfg)
	if err != nil {
		slog.Warn("存储后端初始化失败，回退到 SQLite", "driver", backendCfg.Driver, "error", err)
		return database.NewSQLiteBackendWithDB(sqliteDB), nil
	}
	return bk, nil
}

func (a *App) initCoreServices(sqliteDB *database.DB) error {
	cfg := a.Config

	// IP Detector
	a.Detector = ipdetect.NewDetector(&ipdetect.DetectorConfig{DNSServers: cfg.Detect.DNSServers})

	// DNS Client
	accessKeyID := cfg.DNS.Aliyun.AccessKeyID
	accessKeySecret := cfg.DNS.Aliyun.AccessKeySecret
	endpoint := cfg.DNS.Aliyun.Endpoint
	if accessKeyID == "" {
		accessKeyID = cfg.AliDNS.AccessKeyID
		accessKeySecret = cfg.AliDNS.AccessKeySecret
	}
	if endpoint == "" {
		endpoint = cfg.AliDNS.Endpoint
	}
	if endpoint == "" {
		endpoint = "alidns.aliyuncs.com"
	}
	a.DNSClient = dns.NewAliDNS(accessKeyID, accessKeySecret, endpoint, cfg.Detect.DNSServers)

	// Notifier
	a.Throttler = notifier.NewThrottler()
	nm := notifier.NewManager(a.Throttler)

	nm.Register(notifier.NewEmailNotifier(notifier.EmailConfig{
		Enabled:  cfg.Notify.Email.Enabled,
		Host:     cfg.Notify.Email.Host,
		Port:     cfg.Notify.Email.Port,
		User:     cfg.Notify.Email.User,
		Password: cfg.Notify.Email.Password,
		To:       cfg.Notify.Email.To,
	}))
	nm.Register(notifier.NewWebhookNotifier(notifier.WebhookConfig{
		Enabled: cfg.Notify.Webhook.Enabled,
		URL:     cfg.Notify.Webhook.URL,
		Secret:  cfg.Notify.Webhook.Secret,
		Method:  "POST",
	}))
	a.Notifier = nm

	// Scheduler
	a.Scheduler = scheduler.New(a.Detector, a.DNSClient, a.DomainRepo, a.HistoryRepo, nm, cfg.Detect.Interval)
	if err := a.Scheduler.Start(); err != nil {
		return fmt.Errorf("启动调度器失败: %w", err)
	}

	// Docker manager
	a.DockerMgr = docker.New(true)

	// Sandbox file service
	if cfg.NAS.SandboxRoot != "" {
		a.SandboxFS = nas.NewSandbox(cfg.NAS.SandboxRoot)
		a.SandboxFS.StartChunkGC(30 * time.Minute)
		slog.Info("AI 文件沙箱已启用", "root", cfg.NAS.SandboxRoot)
	} else {
		slog.Info("AI 文件沙箱未启用 (sandbox_root 为空)")
	}

	// Repositories
	a.ShareRepo = database.NewShareRepo(sqliteDB.DB)
	a.PermRepo = database.NewFilePermRepo(sqliteDB.DB)

	return nil
}

func (a *App) initOptionalServices(sqliteDB *database.DB) {
	// QQ Bot
	a.initQQBot()

	// AI Service
	a.initAIService(sqliteDB)

	// Wire QQ Bot with AI
	if a.Bot != nil && a.AIService != nil {
		a.Bot.SetAIService(newQQBotAIAdapter(a.AIService))
		a.Bot.SetSkillRunner(&qqBotSkillAdapter{svc: a.AIService})
		slog.Info("QQ Bot AI 对话已启用")
	}

	// System collector
	a.Collector = sysctl.NewCollector()
	a.Collector.Start(1 * time.Second)
}

func (a *App) initQQBot() {
	cfg := a.Config
	if cfg.QQBot.AppID == "" || cfg.QQBot.AppSecret == "" {
		return
	}

	bot, err := qqbot.New(qqbot.Config{
		AppID:       cfg.QQBot.AppID,
		AppSecret:   cfg.QQBot.AppSecret,
		GroupID:     cfg.QQBot.GroupID,
		SandboxRoot: cfg.NAS.SandboxRoot,
		SignSecret:  cfg.Auth.JWTSecret,
	})
	if err != nil {
		slog.Warn("QQ Bot 初始化失败", "app_id", cfg.QQBot.AppID, "error", err)
		return
	}

	bot.FetchBotInfo(context.Background())
	if info := bot.GetBotInfo(); info.Username != "" {
		cfg.QQBot.Username = info.Username
		cfg.QQBot.Avatar = info.Avatar
		if err := config.SaveSection(context.Background(), a.EncryptedRepo, "qqbot", cfg.QQBot); err != nil {
			slog.Warn("持久化 QQ Bot 信息失败", "error", err)
		}
	}

	if cfg.QQBot.Enabled {
		a.Notifier.Register(bot.NewNotifier())
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("QQ Bot WebSocket panic", "app_id", cfg.QQBot.AppID, "panic", r)
				}
			}()
			if err := bot.Start(context.Background()); err != nil {
				slog.Error("QQ Bot WebSocket 断开", "app_id", cfg.QQBot.AppID, "error", err)
			}
		}()
		slog.Info("QQ Bot 已启用", "app_id", cfg.QQBot.AppID)
	} else {
		slog.Info("QQ Bot 已配置但未启用", "app_id", cfg.QQBot.AppID, "username", bot.GetBotInfo().Username)
	}

	a.Bot = bot
	if bot != nil {
		bot.RegisterCommands(a.DomainRepo, a.HistoryRepo, a.Scheduler)
	}
}

func (a *App) initAIService(sqliteDB *database.DB) {
	cfg := a.Config

	a.ProvReg = provider.New()
	for key, pcfg := range cfg.AI.Providers {
		if pcfg.APIKey == "" {
			continue
		}
		p, err := base.CreateProvider(key, base.ProviderConfig{APIKey: pcfg.APIKey})
		if err != nil {
			slog.Warn("未知 AI 提供商", "name", key, "error", err)
			continue
		}
		a.ProvReg.RegisterProvider(p)
	}
	a.ProvReg.SetDefaultModel(cfg.AI.DefaultModel)

	if !a.ProvReg.IsConfigured() {
		slog.Info("AI 助手未启用")
		return
	}

	registry := register.New()
	toolreg.RegisterAll(registry, a.DNSClient, a.DomainRepo, a.HistoryRepo, a.Scheduler, cfg)
	toolreg.RegisterExtended(registry, a.DockerMgr, cfg)
	toolreg.RegisterOps(registry, cfg)
	toolreg.RegisterFileTools(registry, a.SandboxFS)

	cronRepo := database.NewCronTaskRepo(sqliteDB.DB)
	if err := cronRepo.EnsureSchema(context.Background()); err != nil {
		slog.Warn("创建 cron_tasks 表失败", "error", err)
	}

	svcCfg := ai.DefaultServiceConfig()
	svcCfg.CronRepo = cronRepo
	svcCfg.SkillsDir = cfg.Paths.Skills
	svcCfg.MCPConfigPath = cfg.Paths.MCPConfig

	// Topology tracker
	topoRepo := topology.NewSQLiteRepo(sqliteDB.DB)
	if err := topoRepo.EnsureSchema(context.Background()); err != nil {
		slog.Warn("创建拓扑表失败", "error", err)
	}
	svcCfg.Tracker = topology.NewTracker(topoRepo)

	// Prompt store
	promptRepo := database.NewPromptRepo(sqliteDB)
	svcCfg.PromptStore = base.NewPromptStore(promptRepo)
	if err := svcCfg.PromptStore.SeedDefaults(context.Background()); err != nil {
		slog.Warn("初始化提示词种子数据失败", "error", err)
	}
	if err := svcCfg.PromptStore.LoadAll(context.Background()); err != nil {
		slog.Warn("加载提示词失败", "error", err)
	}

	// Memory system
	memRepo := memory.NewDBRepo(sqliteDB.DB)
	if err := memRepo.EnsureSchema(context.Background()); err != nil {
		slog.Warn("创建 memories 表失败", "error", err)
	}
	memMgr := memory.NewManager(memRepo)
	if err := memMgr.LoadFromDB(context.Background()); err != nil {
		slog.Warn("加载记忆失败", "error", err)
	}
	if cfg.Paths.Memory != "" {
		if err := memMgr.InitFromDir(cfg.Paths.Memory); err != nil {
			slog.Warn("导入记忆文件失败", "error", err)
		}
	}
	slog.Info("记忆系统已启用", "记忆数", memMgr.Count())
	svcCfg.MemoryManager = memMgr

	// Intent pipeline
	intentClassifier := intent.NewClassifier(query.New(a.ProvReg), registry.All())
	svcCfg.IntentPipeline = intent.NewPipeline(intentClassifier)
	slog.Info("意图路由已启用")

	// Adapt layer
	svcCfg.AdaptLayer = adapt.NewLayer(sqliteDB.DB)
	if err := svcCfg.AdaptLayer.EnsureSchema(context.Background()); err != nil {
		slog.Warn("初始化用户适应层失败", "error", err)
	} else {
		slog.Info("用户适应层已启用")
	}

	// Recover interrupted topology sessions
	if err := svcCfg.Tracker.RecoverActiveSessions(context.Background()); err != nil {
		slog.Warn("恢复拓扑会话失败", "error", err)
	}

	// Metrics persistence
	svcCfg.MetricsSaveFn = func(snap ai.CounterSnapshot) {
		data, _ := json.Marshal(snap)
		configRepo := database.NewConfigRepo(sqliteDB)
		if err := configRepo.SetSection(context.Background(), "usage_metrics", string(data)); err != nil {
			slog.Warn("持久化 AI 用量指标失败", "error", err)
		}
	}

	// Create AI service
	chatSessionRepo := database.NewChatSessionRepo(sqliteDB)
	a.AIService = ai.NewService(a.ProvReg, registry, chatSessionRepo, svcCfg)

	// Restore counters
	configRepo := database.NewConfigRepo(sqliteDB)
	if raw, err := configRepo.GetSection(context.Background(), "usage_metrics"); err == nil && raw != "" {
		var snap ai.CounterSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err == nil {
			a.AIService.GetMetrics().LoadFromSnapshot(snap)
			slog.Info("已恢复 AI 用量指标",
				"input_tokens", snap.InputTokens,
				"output_tokens", snap.OutputTokens,
				"requests", snap.Requests,
			)
		}
	}

	// Register agent tools
	a.AIService.RegisterAgentTools()

	// Analytics
	if analyticsRepo := database.NewAnalyticsRepository(sqliteDB); analyticsRepo != nil {
		a.AIService.GetMetrics().SetFlushFn(func(ctx context.Context, batch []ai.MetricEvent) error {
			rows := make([]database.AIMetricRow, len(batch))
			for i, ev := range batch {
				labelsJSON, _ := json.Marshal(ev.Labels)
				extraJSON, _ := json.Marshal(ev.Extra)
				rows[i] = database.AIMetricRow{
					CreatedAt:  ev.Timestamp.Format("2006-01-02 15:04:05"),
					SessionID:  ev.Labels["session"],
					EventType:  ev.Type,
					ToolName:   ev.Labels["tool"],
					Model:      ev.Labels["model"],
					Status:     ev.Labels["status"],
					Value:      ev.Value,
					LabelsJSON: string(labelsJSON),
					ExtraJSON:  string(extraJSON),
				}
			}
			return analyticsRepo.InsertEvents(ctx, rows)
		})
	}

	slog.Info("AI 助手已启用", "providers", a.ProvReg.AllModels(), "tools", len(registry.All()))
}

func (a *App) initHTTPServer(sqliteDB *database.DB) error {
	cfg := a.Config
	server := web.New(cfg, a.EncryptedRepo, a.DomainRepo, a.HistoryRepo, a.UserRepo,
		a.Scheduler, a.DNSClient, a.AIService, a.SandboxFS,
		a.PermRepo, a.Notifier)

	server.SetCollector(a.Collector)

	// QQ Bot runtime info provider
	server.SetBotInfoProvider(func() []map[string]any {
		if a.Bot == nil {
			return nil
		}
		info := a.Bot.GetBotInfo()
		return []map[string]any{{"app_id": info.AppID, "username": info.Username, "avatar": info.Avatar, "online": info.Online}}
	})

	// QQ Bot hot reload callback
	server.SetOnQQBotChanged(func() {
		a.reloadQQBot()
	})

	// AI enabled check
	server.SetAIEnabledCheck(func() bool {
		if a.ProvReg == nil || !a.ProvReg.IsConfigured() {
			return false
		}
		for _, pcfg := range cfg.AI.Providers {
			if pcfg.Enabled && pcfg.APIKey != "" {
				return true
			}
		}
		return false
	})

	// AI provider hot reload
	server.SetOnAIChanged(func() {
		a.reloadAIProviders()
	})

	// Notify config provider
	server.SetNotifyConfigProvider(func() map[string]any {
		return map[string]any{
			"email_enabled":   cfg.Notify.Email.Enabled,
			"webhook_enabled": cfg.Notify.Webhook.Enabled,
			"dingtalk_enabled": cfg.Notify.DingTalk.Enabled,
		}
	})

	// Provider models
	server.SetProviderModelsProvider(func() []string {
		if a.ProvReg == nil {
			return nil
		}
		return a.ProvReg.AllModels()
	})

	// Share and Docker routes
	server.WireSharesAndDocker(a.ShareRepo, cfg.NAS, a.DockerMgr, a.SandboxFS)

	// Shutdown hooks
	if a.AIService != nil {
		server.AddShutdownHook(func() { a.AIService.Shutdown() })
	}

	// Register the app's Shutdown as a server hook so cleanup runs in reverse order
	server.AddShutdownHook(func() {
		a.Throttler.Stop()
	})

	a.Server = server
	return nil
}

// reloadQQBot restarts the QQ Bot with updated configuration.
func (a *App) reloadQQBot() {
	cfg := a.Config
	if cfg.QQBot.AppID == "" || cfg.QQBot.AppSecret == "" {
		return
	}
	newBot, err := qqbot.New(qqbot.Config{
		AppID: cfg.QQBot.AppID, AppSecret: cfg.QQBot.AppSecret,
		GroupID: cfg.QQBot.GroupID, SandboxRoot: cfg.NAS.SandboxRoot,
		SignSecret: cfg.Auth.JWTSecret,
	})
	if err != nil {
		slog.Warn("QQ Bot 重启失败", "error", err)
		return
	}
	newBot.FetchBotInfo(context.Background())
	if info := newBot.GetBotInfo(); info.Username != "" {
		cfg.QQBot.Username = info.Username
		cfg.QQBot.Avatar = info.Avatar
		if err := config.SaveSection(context.Background(), a.EncryptedRepo, "qqbot", cfg.QQBot); err != nil {
			slog.Warn("持久化 QQ Bot 信息失败", "error", err)
		}
	}
	a.Bot = newBot
	a.Notifier.Register(newBot.NewNotifier())
	if a.AIService != nil {
		newBot.SetAIService(newQQBotAIAdapter(a.AIService))
		newBot.SetSkillRunner(&qqBotSkillAdapter{svc: a.AIService})
	}
	newBot.RegisterCommands(a.DomainRepo, a.HistoryRepo, a.Scheduler)
	if cfg.QQBot.Enabled {
		go func() {
			defer func() { if r := recover(); r != nil { slog.Error("QQ Bot panic", "error", r) } }()
			if err := newBot.Start(context.Background()); err != nil {
				slog.Error("QQ Bot WebSocket 断开", "error", err)
			}
		}()
		slog.Info("QQ Bot 已重启", "app_id", cfg.QQBot.AppID)
	}
}

// reloadAIProviders hot-reloads AI providers from config.
func (a *App) reloadAIProviders() {
	cfg := a.Config
	var providers []base.AIProvider
	for key, pcfg := range cfg.AI.Providers {
		if !pcfg.Enabled || pcfg.APIKey == "" {
			continue
		}
		var p base.AIProvider
		switch key {
		case "deepseek":
			p = deepseek.New(deepseek.Config{APIKey: pcfg.APIKey})
		case "qwen":
			p = qwen.New(qwen.Config{APIKey: pcfg.APIKey})
		}
		if p != nil {
			providers = append(providers, p)
		}
	}
	a.ProvReg.ReplaceAll(providers)
	a.ProvReg.SetDefaultModel(cfg.AI.DefaultModel)
	slog.Info("AI Provider 已热重载", "providers", a.ProvReg.AllModels())
}

// AddShutdownHook registers a function to be called during Shutdown.
func (a *App) AddShutdownHook(fn func()) {
	a.onShutdown = append(a.onShutdown, fn)
}

// Start begins serving HTTP requests. Blocks until the server stops.
func (a *App) Start() error {
	cfg := a.Config
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info("HTTP 服务器已启动", "addr", addr)
	if err := a.Server.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器异常退出: %w", err)
	}
	return nil
}

// Shutdown gracefully stops all subsystems in reverse dependency order.
func (a *App) Shutdown(ctx context.Context) error {
	// Stop scheduler first (no new updates)
	if a.Scheduler != nil {
		a.Scheduler.Stop()
	}

	// Stop server
	if a.Server != nil {
		if err := a.Server.Shutdown(ctx); err != nil {
			slog.Error("服务器关闭失败", "error", err)
		}
	}

	// Run custom shutdown hooks (reverse order)
	for i := len(a.onShutdown) - 1; i >= 0; i-- {
		a.onShutdown[i]()
	}

	// Close database
	if a.Backend != nil {
		a.Backend.Close()
	}
	if a.SQLiteDB != nil {
		a.SQLiteDB.Close()
	}

	slog.Info("Yunxi Home 已停止")
	return nil
}

// ── Adapters ────────────────────────────────────────────────────────────────────

// qqBotAIAdapter adapts ai.Service to the qqbot.AIService interface.
type qqBotAIAdapter struct{ svc *ai.Service }

func newQQBotAIAdapter(svc *ai.Service) *qqBotAIAdapter { return &qqBotAIAdapter{svc: svc} }

func (a *qqBotAIAdapter) StreamChat(ctx context.Context, sessionID, userID, userMessage string) <-chan qqbot.AIEvent {
	ch := make(chan qqbot.AIEvent, 64)
	go func() {
		defer close(ch)
		stream := a.svc.StreamChat(ctx, sessionID, userID, userMessage)
		for ev := range stream {
			ch <- qqbot.AIEvent{Type: ev.Type, Content: ev.Content, Tool: ev.Tool}
		}
	}()
	return ch
}
func (a *qqBotAIAdapter) InjectSystemMessage(sessionID, content string) {
	a.svc.InjectMessage(sessionID, content)
}
func (a *qqBotAIAdapter) ClearSession(sessionID string)           { a.svc.ClearSession(sessionID) }
func (a *qqBotAIAdapter) CompactSession(sessionID string) string  { return "上下文压缩已由 v3.1 拓扑约束系统自动管理" }
func (a *qqBotAIAdapter) ReloadSkills() error                      { return a.svc.ReloadSkills() }
func (a *qqBotAIAdapter) ReloadMCP() error                         { return a.svc.ReloadMCPTools("") }
func (a *qqBotAIAdapter) GetMCPServer(ctx context.Context, query string) string {
	return a.svc.GetMCPServer(ctx, query)
}
func (a *qqBotAIAdapter) CreateSkill(ctx context.Context, desc string) (string, error) {
	return a.svc.CreateSkill(ctx, desc)
}

// qqBotSkillAdapter adapts ai.Service to the qqbot.SkillRunner interface.
type qqBotSkillAdapter struct{ svc *ai.Service }

func (a *qqBotSkillAdapter) ListSkills() map[string]string { return a.svc.ListSkills() }
func (a *qqBotSkillAdapter) RunSkill(ctx context.Context, name string) string {
	return a.svc.RunSkill(ctx, name)
}

// Ensure interfaces
var _ qqbot.AIService = (*qqBotAIAdapter)(nil)
var _ qqbot.SkillRunner = (*qqBotSkillAdapter)(nil)
