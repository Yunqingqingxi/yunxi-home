package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/mcp"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/observability"
	"github.com/Yunqingqingxi/yunxi-home/internal/dns"
	"github.com/Yunqingqingxi/yunxi-home/internal/config"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/docker"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
	"github.com/Yunqingqingxi/yunxi-home/internal/notifier"
	"github.com/Yunqingqingxi/yunxi-home/internal/scheduler"
	"github.com/Yunqingqingxi/yunxi-home/internal/sysctl"
	"github.com/Yunqingqingxi/yunxi-home/internal/terminal"
	"github.com/Yunqingqingxi/yunxi-home/internal/web/handlers"
	echomw "github.com/Yunqingqingxi/yunxi-home/internal/web/middleware"
	filemw "github.com/Yunqingqingxi/yunxi-home/internal/middleware"
)

var log = logger.ForComponent("web")

//go:embed all:static
var staticFiles embed.FS

type Server struct {
	echo        *echo.Echo
	addr        string
	api         *echo.Group
	jwtSecret   string
	configH     *handlers.ConfigHandler
	chatH       *handlers.ChatHandler
	statusH     *handlers.StatusHandler
	onShutdown  []func() // hooks called during Shutdown
}

// AddShutdownHook registers a function to be called during server shutdown.
func (s *Server) AddShutdownHook(fn func()) { s.onShutdown = append(s.onShutdown, fn) }

// SetBotInfoProvider sets a callback to provide runtime bot info in config responses.
func (s *Server) SetBotInfoProvider(fn func() []map[string]any) {
	if s.configH != nil { s.configH.SetBotInfoProvider(fn) }
}

// SetCollector sets the system metrics collector on the status handler.
func (s *Server) SetCollector(c *sysctl.SystemCollector) {
	if s.statusH != nil { s.statusH.SetCollector(c) }
}

// SetOnQQBotChanged 设置 qqbot 配置变更后重启 Bot 的回调
func (s *Server) SetOnQQBotChanged(fn func()) {
	if s.configH != nil { s.configH.SetOnQQBotChanged(fn) }
}

// SetAIEnabledCheck 设置 AI 启用状态的运行时检查函数
func (s *Server) SetAIEnabledCheck(fn func() bool) {
	if s.chatH != nil { s.chatH.SetAIEnabledCheck(fn) }
}

// SetOnAIChanged sets the callback invoked after AI provider config is saved.
func (s *Server) SetOnAIChanged(fn func()) {
	if s.configH != nil { s.configH.SetOnAIChanged(fn) }
}

// SetNotifyConfigProvider sets a function that returns the current notify config summary.
func (s *Server) SetNotifyConfigProvider(fn func() map[string]any) {
	if s.statusH != nil { s.statusH.SetNotifyConfigProvider(fn) }
}

// SetProviderModelsProvider sets a function that returns the current AI model list.
func (s *Server) SetProviderModelsProvider(fn func() []string) {
	if s.statusH != nil { s.statusH.SetProviderModelsProvider(fn) }
}

// New creates the web server. sandboxFS is an optional sandboxed file service for AI file tools.
func New(cfg *config.Config, configRepo database.ConfigRepository, domainRepo database.DomainRepository, historyRepo database.HistoryRepository, userRepo database.UserRepository, s *scheduler.Scheduler, dnsClient dns.Provider, aiSvc *ai.Service, sandboxFS nas.FileService, permRepo database.FilePermissionRepository, nm *notifier.Manager) *Server {
	e := echo.New()
	e.HideBanner = true

	// 自定义请求日志：跳过静态资源和健康检查，减少噪声
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			path := c.Request().URL.Path
			// 跳过静态资源、健康检查和轮询端点
			if strings.HasPrefix(path, "/assets/") ||
				strings.HasPrefix(path, "/favicon") ||
				path == "/health" ||
				path == "/ready" ||
				path == "/api/status" {
				return err
			}
			latency := time.Since(start)
			log.Info("request",
				"method", c.Request().Method,
				"uri", c.Request().RequestURI,
				"status", c.Response().Status,
				"latency", latency.String(),
			)
			return err
		}
	})
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) { return true, nil },
		AllowMethods:    []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:          86400,
	}))
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(cfg.Server.RateLimit))))

	jwtCfg := echomw.JWTConfig{
		Secret:     cfg.Auth.JWTSecret,
		Expiration: 24 * time.Hour,
	}

	if err := userRepo.InitDefaultAdmin(context.Background(), cfg.Auth.Username, cfg.Auth.Password); err != nil {
		e.Logger.Warnf("init admin failed: %v", err)
	}

	authH := handlers.NewAuthHandler(userRepo, jwtCfg)
	domainH := handlers.NewDomainHandler(domainRepo, s, dnsClient)
	historyH := handlers.NewHistoryHandler(historyRepo)
	statusH := handlers.NewStatusHandler(s, userRepo)
	configH := handlers.NewConfigHandler(cfg, configRepo, nm)

	// ── 初始化新模块 ──────────────────────────────────

	var filesH *handlers.FilesHandler
	if cfg.NAS.Enabled {
		// use sandboxFS when available, fall back to configured root dir
		var fileFS nas.FileService
		if sandboxFS != nil {
			fileFS = sandboxFS
		} else {
			fileFS = nas.New(cfg.NAS.RootDir, cfg.NAS.AllowedDirs)
		}
		filesH = handlers.NewFilesHandler(fileFS).WithUserRepo(userRepo)
	}

	var sysctlH *handlers.SysctlHandler
	if cfg.Sysctl.Enabled {
		ctrl := sysctl.New(cfg.Sysctl.ServiceControl, cfg.Sysctl.ProcessControl)
		sysctlH = handlers.NewSysctlHandler(ctrl)
	}

	var termH *terminal.TerminalHandler
	if cfg.Terminal.Enabled {
		termH = terminal.NewHandler(true, cfg.Terminal.AdminOnly)
	}

	// Public routes
	e.GET("/health", statusH.Health)
	e.GET("/ready", statusH.Ready)
	// Public file download (HMAC-signed) for QQ Bot file sending
	e.GET("/dl", func(c echo.Context) error {
		p := c.QueryParam("p")
		t := c.QueryParam("t")
		e := c.QueryParam("e")
		if p == "" || t == "" || e == "" {
			return c.String(http.StatusBadRequest, "missing params")
		}
		mac := hmac.New(sha256.New, []byte(cfg.Auth.JWTSecret))
		mac.Write([]byte(p + "|" + e))
		expected := hex.EncodeToString(mac.Sum(nil))[:32]
		if t != expected {
			return c.String(http.StatusForbidden, "invalid token")
		}
		fullPath := filepath.Join(cfg.NAS.SandboxRoot, p)
		if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(cfg.NAS.SandboxRoot)) {
			return c.String(http.StatusForbidden, "path traversal")
		}
		return c.File(fullPath)
	})

	// Auth routes
	e.POST("/api/auth/login", authH.Login)
	e.GET("/api/auth/status", authH.Status)
	e.POST("/api/auth/setup", authH.Setup)

	// Protected routes
	api := e.Group("/api")
	api.Use(echomw.JWTAuth(cfg.Auth.JWTSecret))

	api.POST("/auth/refresh", authH.Refresh)
	api.POST("/auth/change-password", authH.ChangePassword)

	api.GET("/domains/cloud", domainH.ListCloudDomains)
	api.GET("/domains/cloud/records", domainH.ListCloudRecords)
	api.POST("/domains/cloud/records", domainH.CreateCloudRecord)
	api.PUT("/domains/cloud/records/:recordId", domainH.UpdateCloudRecord)
	api.DELETE("/domains/cloud/records/:recordId", domainH.DeleteCloudRecord)
	api.GET("/domains", domainH.List)
	api.GET("/domains/:id", domainH.Get)
	api.POST("/domains", domainH.Create)
	api.PUT("/domains/:id", domainH.Update)
	api.DELETE("/domains/:id", domainH.Delete)

	api.GET("/history", historyH.List)
	api.GET("/history/stats", historyH.GetStats)
	api.DELETE("/history/clean", historyH.CleanOld)

	api.GET("/config", configH.GetConfig)
	api.GET("/config/:section", configH.GetSection)
	api.PUT("/config/:section", configH.UpdateSection)
	api.POST("/config/ai/test", configH.TestAISection)
		api.PUT("/config", configH.UpdateConfig)

	api.GET("/status", statusH.Status)
	api.POST("/trigger", statusH.Trigger)
	api.POST("/system/gc", statusH.ClearMemory)
	api.GET("/system/setup-status", statusH.SetupStatus)
	api.POST("/system/run-setup", statusH.RunSetup)

	// Chat routes — always available, static response when AI not configured
	chatH := handlers.NewChatHandler(aiSvc)
	chatH.SetMCPConfigPath(cfg.Paths.MCPConfig)
	chatH.SetSkillsDir(cfg.Paths.Skills)
	if aiSvc != nil {
		mcpSub := mcp.NewSubsystem(cfg.Paths.MCPConfig, ai.MCPRegistryAdapter{Reg: aiSvc.GetRegistry()})
		chatH.SetMCPService(mcpSub)
		aiSvc.SetMCPSubsystem(mcpSub)
		statusH.SetMCPService(mcpSub)
		statusH.SetMetricsCollector(aiSvc.GetMetrics())
		statusH.SetAgentMetrics(observability.GlobalMetrics())
		// 异步加载 MCP 配置，避免阻塞 server 启动
		go func() {
			if err := mcpSub.Load(); err != nil {
				log.Warn("MCP subsystem load failed", "error", err)
			}
		}()
	}
	api.POST("/chat", chatH.Chat)
	api.POST("/chat/title", chatH.GenerateTitle)
	api.POST("/chat/confirm", chatH.ConfirmAction)
	api.POST("/chat/respond", chatH.RespondInteractive)
	api.POST("/chat/inject", chatH.InjectMessage)
	api.POST("/chat/command", chatH.RunCommand)
	api.GET("/chat/commands", chatH.GetCommands)
	api.POST("/chat/clear", chatH.ClearSession)
	api.POST("/chat/clear-all", chatH.ClearAllSessions)
	api.GET("/chat/sessions", chatH.ListSessions)
	api.GET("/chat/stream/:id", chatH.StreamSession)
	api.GET("/chat/sessions/:id", chatH.GetSessionDetail)
	api.PATCH("/chat/sessions/:id", chatH.UpdateSessionMeta)
	api.DELETE("/chat/sessions/:id", chatH.DeleteSession)
	api.GET("/chat/tools", chatH.Tools)
	api.GET("/chat/hints", chatH.Hints)
	// v3.1 拓扑几何约束 + 提示词管理 API
	api.GET("/config/prompts", chatH.GetPrompts)
	api.PUT("/config/prompts/:section", chatH.UpdatePrompt)
	api.POST("/config/prompts/:section/reset", chatH.ResetPrompt)
	api.GET("/chat/sessions/:id/topology", chatH.GetTopology)
	api.PUT("/chat/sessions/:id/topology/constraint", chatH.UpdateConstraint)
	api.POST("/chat/sessions/:id/topology/override", chatH.OverrideNode)
	api.POST("/chat/sessions/:id/topology/trust-reset", chatH.ResetTrust)
	api.PUT("/chat/sessions/:id/messages/:messageIndex", chatH.EditMessage)
	api.DELETE("/chat/sessions/:id/messages/:messageIndex", chatH.DeleteMessage)
	api.POST("/chat/sessions/:id/interrupt", chatH.InterruptSession)
	// 技能与 MCP 市场
	api.POST("/market/search-skills", chatH.SearchSkills)
	api.POST("/market/install-skill", chatH.InstallSkill)
	api.POST("/market/install-mcp", chatH.InstallMCP)
	api.POST("/market/install-mcp-stream", chatH.InstallMCPStream)
	api.GET("/market/install-tasks", chatH.ListInstallTasks)
	api.GET("/market/popular-mcp", chatH.PopularMCP)
	api.GET("/market/installed", chatH.GetInstalled)

	// Log routes
	if aiSvc != nil && aiSvc.ChatLogger() != nil {
		logH := handlers.NewLogHandler(aiSvc.ChatLogger(), cfg.Log.Dir)
		api.GET("/logs/chat/sessions", logH.ListChatSessions)
		api.GET("/logs/chat/:id", logH.GetChatLog)
		api.GET("/logs/chat/:id/errors", logH.GetChatErrors)
		api.GET("/logs/chat/:id/text", logH.GetChatLogText)
		api.GET("/logs/chat/:id/tail", logH.TailChatLog)
		api.GET("/logs/chat/:id/download", logH.DownloadChatLog)
		api.GET("/logs/system", logH.ListSystemLogs)
		api.GET("/logs/system/:date", logH.GetSystemLog)
		api.GET("/logs/system/:date/download", logH.DownloadSystemLog)
		api.DELETE("/logs/chat/:id", logH.DeleteChatLog)
		api.DELETE("/logs/system/:date", logH.DeleteSystemLog)
	}

	if aiSvc != nil {
			// Cron 定时任务 API
			cronH := handlers.NewCronHandler(aiSvc)
			api.GET("/cron/tasks", cronH.ListTasks)
			api.DELETE("/cron/tasks/:id", cronH.DeleteTask)
		}

	// ── NAS 文件管理路由 ──────────────────────────────
	if filesH != nil {
		nasAPI := api.Group("/nas", filemw.FileAccess(permRepo, userRepo, cfg.Auth.JWTSecret))
		nasAPI.GET("/files", filesH.ListFiles)
		nasAPI.GET("/files/download", filesH.DownloadFile)
		nasAPI.POST("/files/upload", filesH.UploadFile)
		nasAPI.POST("/files/mkdir", filesH.CreateDir)
		nasAPI.DELETE("/files", filesH.DeleteItem)
		nasAPI.PUT("/files/rename", filesH.RenameItem)
		nasAPI.GET("/diskinfo", filesH.GetDiskInfo)
		nasAPI.GET("/search", filesH.SearchFiles)
		nasAPI.GET("/files/stream", filesH.StreamFile)
		nasAPI.GET("/files/preview", filesH.PreviewFile)
		nasAPI.POST("/files/copy", filesH.CopyItem)
		nasAPI.GET("/files/stat", filesH.StatFile)
		nasAPI.POST("/files/batch-delete", filesH.BatchDeleteItems)
		nasAPI.PUT("/files/save", filesH.SaveFileContent)
		nasAPI.GET("/files/tree", filesH.GetFileTree)
		nasAPI.POST("/files/move", filesH.MoveFile)
		nasAPI.POST("/files/batch-download", filesH.BatchDownload)
		// 分片上传路由 (不使用 FileAccess 中间件，分片本身没有文件路径)
		api.POST("/nas/files/upload/init", filesH.InitChunkUpload)
		api.POST("/nas/files/upload/chunk", filesH.SaveChunk)
		api.POST("/nas/files/upload/complete", filesH.CompleteChunkUpload)
		api.GET("/nas/files/upload/status", filesH.GetChunkStatus)
		api.POST("/nas/files/upload/abort", filesH.AbortChunkUpload)
		}
	// ── 沙箱状态 ────────────────────────────────────
	api.GET("/sandbox/status", func(c echo.Context) error {
		if sandboxFS == nil {
			return c.JSON(http.StatusOK, map[string]any{"sandbox": false, "message": "Sandbox not configured"})
		}
		info := sandboxFS.SandboxInfo()
		return c.JSON(http.StatusOK, info)
	})


	// ── 系统控制路由 ──────────────────────────────────
	if sysctlH != nil {
		sysctlAPI := api.Group("/sysctl")
		sysctlAPI.GET("/info", sysctlH.GetSystemInfo)
		sysctlAPI.GET("/processes", sysctlH.ListProcesses)
		sysctlAPI.POST("/processes/:pid/kill", sysctlH.KillProcess)
		sysctlAPI.GET("/services", sysctlH.ListServices)
		sysctlAPI.POST("/services/:name/:action", sysctlH.ControlService)
	}

	// ── 终端 WebSocket ────────────────────────────────
	if termH != nil {
		api.GET("/terminal", termH.Handle)
	}

		// ── 管理员路由 (用户/权限管理) ────────────────
	adminAPI := api.Group("/admin", filemw.RequireAdmin(cfg.Auth.JWTSecret))
	adminH := handlers.NewAdminHandler(userRepo, permRepo)
	adminAPI.GET("/users", adminH.ListUsers)
	adminAPI.POST("/users", adminH.CreateUser)
	adminAPI.PUT("/users/:id", adminH.UpdateUser)
	adminAPI.DELETE("/users/:id", adminH.DeleteUser)
	adminAPI.GET("/file-permissions", adminH.ListFilePerms)
	adminAPI.POST("/file-permissions", adminH.UpsertFilePerm)
	adminAPI.DELETE("/file-permissions/:id", adminH.DeleteFilePerm)

	// SPA fallback
	staticFS, err := fs.Sub(staticFiles, "static")
	if err == nil {
		fileServer := http.FileServer(http.FS(staticFS))
		e.GET("/*", echo.WrapHandler(http.StripPrefix("/", cacheControl(fileServer))))
	}

	return &Server{
		echo:      e,
		addr:      fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		api:       api,
		jwtSecret: cfg.Auth.JWTSecret,
		configH:   configH,
		chatH:     chatH,
		statusH:   statusH,
	}
}

// WireSharesAndDocker wires share and docker handlers that depend on DB resources
func (s *Server) WireSharesAndDocker(shareRepo database.ShareRepository, nasCfg config.NASConfig, dockerMgr *docker.Manager, sandboxFS nas.FileService) {
	var shareFS nas.FileService
	if sandboxFS != nil {
		shareFS = sandboxFS
	} else {
		shareFS = nas.New(nasCfg.RootDir, nasCfg.AllowedDirs)
	}
	shareSvc := nas.NewShareService(shareFS)
	sharesH := handlers.NewSharesHandler(shareRepo, shareSvc)

	// Protected shares API
	s.api.POST("/nas/shares", sharesH.CreateShare)
	s.api.GET("/nas/shares", sharesH.ListShares)
	s.api.DELETE("/nas/shares/:id", sharesH.DeleteShare)

	// Public share access
	s.echo.GET("/s/:token", sharesH.AccessShare)

	// Docker API
	if dockerMgr != nil {
		dockerH := handlers.NewDockerHandler(dockerMgr)
		s.api.GET("/docker/containers", dockerH.ListContainers)
		s.api.POST("/docker/containers/:name/:action", dockerH.ContainerAction)
		s.api.GET("/docker/containers/:name/logs", dockerH.GetLogs)
		s.api.GET("/docker/containers/:name/stats", dockerH.Stats)
		s.api.POST("/docker/compose/:action", dockerH.ComposeAction)
	}
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ".") || strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error { return s.echo.Start(s.addr) }

func (s *Server) Shutdown(ctx context.Context) error {
	for _, fn := range s.onShutdown {
		fn()
	}
	return s.echo.Shutdown(ctx)
}
