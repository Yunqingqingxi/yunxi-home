package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/yxd/yunxi-home/internal/util/fileio"
)

// ── MCPService / Subsystem ──────────────────────────────────────────

// MCPService defines the public API for the MCP subsystem.
type MCPService interface {
	// Install installs an MCP server package and returns progress via the channel.
	Install(ctx context.Context, pkg string, env map[string]string) (<-chan InstallEvent, error)
	// Uninstall removes a previously installed MCP server by name.
	Uninstall(name string) error

	// List returns status for all installed MCP servers.
	List() []ServerStatus
	// Status returns the current state of a single MCP server.
	Status(name string) (*ServerStatus, error)

	// Reload performs a full reload from the config file.
	Reload() error
	// HealthCheck pings all connected servers and updates status.
	HealthCheck()
}

// ServerStatus represents the current state of an installed MCP server.
type ServerStatus struct {
	Name      string `json:"name"`
	Package   string `json:"package"`
	Connected bool   `json:"connected"`
	Tools     int    `json:"tools"`
	Error     string `json:"error,omitempty"`
}

// InstallEvent represents a progress event during MCP installation.
type InstallEvent struct {
	TaskID   string `json:"task_id"`
	Step     string `json:"step"`     // start|download|config|connect|reload|done
	Status   string `json:"status"`   // running|done|error|success|warning|need_input
	Message  string `json:"message"`
	Progress int    `json:"progress"`
}

// Subsystem manages the MCP server lifecycle.
type Subsystem struct {
	manager  *Manager
	tracker  *InstallTracker
	registry ToolRegistry
	cfgPath  string
	mu       sync.RWMutex
	status   map[string]*ServerStatus // current state of each server
}

// NewSubsystem creates a new MCP Subsystem.
func NewSubsystem(cfgPath string, registry ToolRegistry) *Subsystem {
	return &Subsystem{
		manager:  NewManager(),
		tracker:  newInstallTracker(),
		registry: registry,
		cfgPath:  cfgPath,
		status:   make(map[string]*ServerStatus),
	}
}

// ── Lifecycle ──────────────────────────────────────────────────────

// Load reads mcp.json and connects to all configured MCP servers.
func (s *Subsystem) Load() error {
	data, err := os.ReadFile(s.cfgPath)
	if err != nil {
		slog.Info("MCP config not found, skipping", "path", s.cfgPath)
		return nil
	}
	return s.loadConfig(data)
}

// Reload re-reads the config file and reconnects all servers.
func (s *Subsystem) Reload() error {
	data, err := os.ReadFile(s.cfgPath)
	if err != nil {
		return fmt.Errorf("read mcp.json: %w", err)
	}

	s.mu.Lock()
	s.manager.CloseAll()
	s.manager = NewManager()
	s.status = make(map[string]*ServerStatus)
	s.mu.Unlock()

	return s.loadConfig(data)
}

func (s *Subsystem) loadConfig(data []byte) error {
	var cfg struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse mcp.json: %w", err)
	}

	for name, svr := range cfg.MCPServers {
		s.manager.AddServer(ServerConfig{
			Name: name, Command: svr.Command, Args: svr.Args, Env: svr.Env,
		})
		s.status[name] = &ServerStatus{Name: name, Package: svr.Command, Connected: false}
	}

	if err := s.manager.ConnectAll(); err != nil {
		return err
	}
	// 连接后同步实际状态
	for name := range s.status {
		if client := s.manager.GetClient(name); client != nil {
			s.status[name].Connected = client.IsConnected()
			s.status[name].Tools = len(client.Tools())
			if !client.IsConnected() {
				s.status[name].Error = "连接失败"
			}
		}
	}
	RegisterTools(s.manager, s.registry)
	slog.Info("MCP tools loaded", "servers", len(cfg.MCPServers))
	return nil
}

// ── Install / Uninstall ────────────────────────────────────────────

// Install installs an MCP server package and streams progress events.
// This is the single entry point for MCP installation.
func (s *Subsystem) Install(ctx context.Context, pkg string, env map[string]string) (<-chan InstallEvent, error) {
	taskID := "mcp_" + fmt.Sprintf("%d", time.Now().UnixNano())
	task := s.tracker.createTask(taskID, pkg)
	ch := make(chan InstallEvent, 32)

	go func() {
		defer close(ch)
		defer task.markDone()

		emit := func(step, status, msg string, pct int) {
			ev := InstallEvent{TaskID: taskID, Step: step, Status: status, Message: msg, Progress: pct}
			select {
			case ch <- ev:
			default:
			}
			task.addStep(step, status, msg)
			task.updateProgress(pct)
		}

		emit("start", "running", fmt.Sprintf("开始安装 %s", pkg), 5)

		// Step 1: npm install (best-effort)
		emit("download", "running", "正在下载包...", 10)
		if _, err := exec.LookPath("npm"); err == nil {
			cmd := exec.Command("npm", "install", "-g", pkg)
			out, installErr := cmd.CombinedOutput()
			if installErr != nil {
				emit("download", "running", fmt.Sprintf("npm install 警告: %s (使用 npx)", truncStr(string(out), 100)), 20)
			} else {
				emit("download", "done", "包下载完成", 25)
			}
		} else {
			emit("download", "done", "跳过 npm install (将使用 npx 自动下载)", 25)
		}

		// Step 2: Write mcp.json (incremental — append single server)
		emit("config", "running", "写入 mcp.json 配置...", 35)
		serverName := pkg
		if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
			serverName = pkg[idx+1:]
		}
		serverName = strings.TrimPrefix(serverName, "server-")
		serverName = strings.TrimPrefix(serverName, "mcp-")

		if err := s.addServerToConfig(pkg, serverName, env); err != nil {
			emit("config", "error", fmt.Sprintf("配置写入失败: %v", err), 35)
			return
		}
		emit("config", "done", "配置已写入 mcp.json", 50)

		// Step 3: Add to Manager and connect (incremental — only this one server)
		emit("connect", "running", "正在启动 MCP 服务器并验证连接...", 55)
		serverCfg := ServerConfig{
			Name: serverName, Command: "npx", Args: []string{"-y", pkg}, Env: env,
		}
		client := s.manager.AddServer(serverCfg)
		if err := client.Connect(); err != nil {
			emit("connect", "error", fmt.Sprintf("连接失败: %v", err), 85)
			s.manager.RemoveServer(serverName)
			return
		}
		toolCount := len(client.Tools())
		emit("connect", "done", fmt.Sprintf("✅ 连接成功！发现 %d 个工具", toolCount), 85)

		// Step 4: Register only this server's tools (incremental)
		emit("reload", "running", "注册 MCP 工具...", 90)
		RegisterTools(s.manager, s.registry)
		s.updateStatus(serverName, &ServerStatus{
			Name: serverName, Package: pkg, Connected: true, Tools: toolCount,
		})
		emit("reload", "done", "MCP 工具已注册", 95)

		emit("done", "success", fmt.Sprintf("✅ %s 安装完成！%d 个工具可用", serverName, toolCount), 100)
	}()

	return ch, nil
}

// Uninstall removes an MCP server by name.
func (s *Subsystem) Uninstall(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from config file
	config := s.readConfig()
	servers, _ := config["mcpServers"].(map[string]any)
	if servers != nil {
		delete(servers, name)
	}
	if err := s.writeConfig(config); err != nil {
		return fmt.Errorf("write mcp.json: %w", err)
	}

	// Remove from manager
	s.manager.RemoveServer(name)

	// Remove from status
	delete(s.status, name)

	slog.Info("MCP server uninstalled", "name", name)
	return nil
}

// ── Query ───────────────────────────────────────────────────────────

// List returns status for all known MCP servers.
func (s *Subsystem) List() []ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ServerStatus, 0, len(s.status))
	for _, st := range s.status {
		// Sync with actual client state
		client := s.manager.GetClient(st.Name)
		if client != nil {
			st.Connected = client.IsConnected()
			st.Tools = len(client.Tools())
		}
		result = append(result, *st)
	}
	return result
}

// Status returns the current state of a single MCP server.
func (s *Subsystem) Status(name string) (*ServerStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st, ok := s.status[name]
	if !ok {
		return nil, fmt.Errorf("MCP server %s not found", name)
	}
	client := s.manager.GetClient(name)
	if client != nil {
		st.Connected = client.IsConnected()
		st.Tools = len(client.Tools())
	}
	return st, nil
}

// Tracker returns the InstallTracker for async task polling.
func (s *Subsystem) Tracker() *InstallTracker { return s.tracker }

// ConfigPath returns the configured mcp.json path.
func (s *Subsystem) ConfigPath() string { return s.cfgPath }

// ── Health ──────────────────────────────────────────────────────────

// HealthCheck pings all connected servers and updates their status.
func (s *Subsystem) HealthCheck() {
	s.mu.RLock()
	names := make([]string, 0, len(s.status))
	for name := range s.status {
		names = append(names, name)
	}
	s.mu.RUnlock()

	for _, name := range names {
		client := s.manager.GetClient(name)
		if client == nil {
			continue
		}
		connected := client.IsConnected()
		s.mu.Lock()
		if st, ok := s.status[name]; ok {
			st.Connected = connected
			st.Tools = len(client.Tools())
			if !connected {
				st.Error = "disconnected"
			} else {
				st.Error = ""
			}
		}
		s.mu.Unlock()
	}
}

// StartHealthCheck runs periodic health checks.
func (s *Subsystem) StartHealthCheck(interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.HealthCheck()
			}
		}
	}()
	return cancel
}

// ── Internal helpers ────────────────────────────────────────────────

func (s *Subsystem) readConfig() map[string]any {
	var config map[string]any
	if err := fileio.ReadJSON(s.cfgPath, &config); err != nil {
		return map[string]any{}
	}
	return config
}

func (s *Subsystem) writeConfig(config map[string]any) error {
	return fileio.WriteJSON(s.cfgPath, config)
}

func (s *Subsystem) addServerToConfig(pkg, name string, env map[string]string) error {
	config := s.readConfig()
	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
		config["mcpServers"] = servers
	}

	serverCfg := map[string]any{
		"command": "npx",
		"args":    []string{"-y", pkg},
	}
	if len(env) > 0 {
		serverCfg["env"] = env
	}
	servers[name] = serverCfg

	return s.writeConfig(config)
}

func (s *Subsystem) updateStatus(name string, st *ServerStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[name] = st
}

// Client returns the underlying Manager for direct access (used by RegisterTools, GetMCPServer, etc.).
func (s *Subsystem) Manager() *Manager { return s.manager }

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
