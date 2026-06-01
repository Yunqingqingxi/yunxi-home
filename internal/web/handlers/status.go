package handlers

import (
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yxd/yunxi-home/internal/ai"
	"github.com/yxd/yunxi-home/internal/ai/mcp"
	"github.com/yxd/yunxi-home/internal/database"
	"github.com/yxd/yunxi-home/internal/scheduler"
	"github.com/yxd/yunxi-home/internal/sysctl"
)

// StatusHandler 状态和控制 Handler
type StatusHandler struct {
	scheduler    *scheduler.Scheduler
	userRepo     database.UserRepository
	collector    *sysctl.SystemCollector
	mcpSvc       mcp.MCPService
	metrics      *ai.MetricsCollector
	notifyCfg    func() map[string]any
	provModels   func() []string
	startTime    time.Time
}

// NewStatusHandler 创建状态 Handler
func NewStatusHandler(s *scheduler.Scheduler, userRepo database.UserRepository) *StatusHandler {
	return &StatusHandler{
		scheduler: s,
		userRepo:  userRepo,
		startTime: time.Now(),
	}
}

// SetCollector sets the system metrics collector.
func (h *StatusHandler) SetCollector(c *sysctl.SystemCollector) { h.collector = c }
// SetMCPService sets the MCP subsystem for status overview.
func (h *StatusHandler) SetMCPService(svc mcp.MCPService)        { h.mcpSvc = svc }
// SetMetricsCollector sets the AI metrics collector.
func (h *StatusHandler) SetMetricsCollector(mc *ai.MetricsCollector) { h.metrics = mc }
// SetNotifyConfigProvider sets a function that returns current notify config summary.
func (h *StatusHandler) SetNotifyConfigProvider(fn func() map[string]any) { h.notifyCfg = fn }
func (h *StatusHandler) SetProviderModelsProvider(fn func() []string)    { h.provModels = fn }

// StatusResponse 状态响应
type StatusResponse struct {
	Version    string                 `json:"version"`
	Uptime     string                 `json:"uptime"`
	GoVersion  string                 `json:"go_version"`
	Goroutines int                    `json:"goroutines"`
	Scheduler  map[string]interface{} `json:"scheduler"`
	System     *SystemInfo            `json:"system,omitempty"`
	AI         map[string]any         `json:"ai,omitempty"`
	MCP        map[string]any         `json:"mcp,omitempty"`
	GoRuntime  map[string]any         `json:"go_runtime,omitempty"`
	Process    map[string]any         `json:"process,omitempty"`
	Notify     map[string]any         `json:"notify,omitempty"`
}

type NetInterface struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	MAC     string `json:"mac"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}

type SystemInfo struct {
	Hostname    string         `json:"hostname"`
	Platform    string         `json:"platform"`
	Arch        string         `json:"arch"`
	CPUCores    int            `json:"cpu_cores"`
	CPUUsage    float64        `json:"cpu_usage"`
	MemTotal    string         `json:"mem_total"`
	MemUsed     string         `json:"mem_used"`
	MemUsage    float64        `json:"mem_usage"`
	LoadAvg     string         `json:"load_avg"`
	LocalIPv4    string         `json:"local_ipv4"`
	LocalIPv6    string         `json:"local_ipv6"`
	NetRxBytes   int64          `json:"net_rx_bytes"`
	NetTxBytes   int64          `json:"net_tx_bytes"`
	Interfaces   []NetInterface `json:"interfaces"`
}

// Status 获取当前状态
// GET /api/status
func (h *StatusHandler) Status(c echo.Context) error {
	schedStatus, err := h.scheduler.GetStatus(c.Request().Context())
	if err != nil {
		slog.Warn("获取调度器状态失败", "error", err)
		schedStatus = map[string]interface{}{"error": "获取状态失败"}
	}
	if raw, ok := schedStatus["interval"].(string); ok {
		schedStatus["interval"] = raw
		schedStatus["interval_human"] = formatCronHuman(raw)
	}

	uptime := time.Since(h.startTime).Truncate(time.Second).String()

	resp := StatusResponse{
		Version:    "4.0.0",
		Uptime:     uptime,
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		Scheduler:  schedStatus,
		System:     h.getSystemInfo(),
		AI:         h.buildAIOverview(),
		MCP:        h.buildMCPOverview(),
		GoRuntime:  h.buildGoRuntime(),
		Process:    h.buildProcessStats(),
		Notify:     h.buildNotifyOverview(),
	}

	// Add available models to AI overview
	if resp.AI != nil && h.provModels != nil {
		resp.AI["models"] = h.provModels()
	}

	return c.JSON(http.StatusOK, successResp(resp))
}

func (h *StatusHandler) getSystemInfo() *SystemInfo {
	hostname, _ := os.Hostname()
	info := &SystemInfo{
		Hostname: hostname,
		Platform: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Arch:     runtime.GOARCH,
	}

	// Use collector cache if available (lock-free)
	if h.collector != nil {
		s := h.collector.Get()
		info.CPUCores = s.CPUCount
		info.CPUUsage = math.Round(s.CPUUsage*10) / 10
		info.MemTotal = formatBytes(s.MemTotal)
		info.MemUsed = formatBytes(s.MemUsed)
		info.MemUsage = math.Round(s.MemUsage*10) / 10
		info.LoadAvg = s.LoadAvg
		info.NetRxBytes = s.NetRxBytes
		info.NetTxBytes = s.NetTxBytes
		for _, iface := range s.Interfaces {
			info.Interfaces = append(info.Interfaces, NetInterface{
				Name: iface.Name, RxBytes: iface.RxBytes, TxBytes: iface.TxBytes,
			})
		}
		// Fill addr/mac from net.Interfaces
		fillInterfaceAddrs(info.Interfaces)
		info.LocalIPv4, info.LocalIPv6 = localIPs()
		return info
	}

	// Fallback: read /proc directly
	info.CPUCores = runtime.NumCPU()
	devBytes := readProcNetDev()
	var totalRx, totalTx int64
	for _, v := range devBytes { totalRx += v.rx; totalTx += v.tx }
	info.NetRxBytes, info.NetTxBytes = totalRx, totalTx
	info.Interfaces, info.LocalIPv4, info.LocalIPv6 = getNetworkInfoWithDev(devBytes)
	fillMemInfoUnix(info)
	return info
}

func fillInterfaceAddrs(ifaces []NetInterface) {
	nifs, _ := net.Interfaces()
	for i := range ifaces {
		for _, nif := range nifs {
			if nif.Name == ifaces[i].Name {
				ifaces[i].MAC = nif.HardwareAddr.String()
				if addrs, err := nif.Addrs(); err == nil && len(addrs) > 0 {
					ifaces[i].Addr = strings.Split(addrs[0].String(), "/")[0]
				}
				if ifaces[i].MAC == "" { ifaces[i].MAC = "-" }
				break
			}
		}
	}
}

func localIPs() (v4, v6 string) {
	nifs, _ := net.Interfaces()
	for _, nif := range nifs {
		if nif.Flags&net.FlagUp == 0 { continue }
		addrs, _ := nif.Addrs()
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok { continue }
			if ip := ipnet.IP; ip.To4() != nil && !ip.IsLoopback() {
				if v4 == "" { v4 = ip.String() }
			} else if !ip.IsLoopback() && ip.IsGlobalUnicast() {
				if v6 == "" { v6 = ip.String() }
			}
		}
	}
	return
}

// ── AI Overview ─────────────────────────────────────────────────────

func (h *StatusHandler) buildAIOverview() map[string]any {
	overview := map[string]any{
		"requests": 0, "errors": 0, "tool_calls": 0, "tool_errors": 0,
		"input_tokens": 0, "output_tokens": 0, "cost_usd": 0.0,
	}
	if h.metrics == nil {
		return overview
	}
	snap := h.metrics.Snapshot()
	overview["requests"] = snap.TotalLLMRequests
	overview["errors"] = snap.TotalLLMErrors
	overview["tool_calls"] = snap.TotalToolCalls
	overview["tool_errors"] = snap.TotalToolErrors
	overview["loops_detected"] = snap.LoopDetected
	overview["input_tokens"] = h.metrics.TotalInputTokens.Load()
	overview["output_tokens"] = h.metrics.TotalOutputTokens.Load()
	overview["cost_usd"] = float64(h.metrics.TotalCostMicros.Load()) / 1e6
	overview["started_at"] = snap.Since.Format(time.RFC3339)

	// Top tools
	type toolEntry struct{ name string; count int64; avgMs float64 }
	var entries []toolEntry
	for name, s := range snap.Tools {
		avgMs := 0.0
		if s.Calls > 0 {
			avgMs = s.TotalLatency / float64(s.Calls) * 1000
		}
		entries = append(entries, toolEntry{name, s.Calls, avgMs})
	}
	// Sort by count desc (simple bubble for small N)
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].count > entries[i].count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	top5 := make([]map[string]any, 0)
	for i, e := range entries {
		if i >= 5 { break }
		top5 = append(top5, map[string]any{"name": e.name, "count": e.count, "avg_ms": math.Round(e.avgMs*10) / 10})
	}
	overview["top_tools"] = top5

	return overview
}

// ── MCP Overview ────────────────────────────────────────────────────

func (h *StatusHandler) buildMCPOverview() map[string]any {
	overview := map[string]any{"total": 0, "connected": 0, "tools": 0, "servers": []any{}}
	if h.mcpSvc == nil {
		return overview
	}
	servers := h.mcpSvc.List()
	connected, totalTools := 0, 0
	for _, s := range servers {
		if s.Connected { connected++; totalTools += s.Tools }
	}
	overview["total"] = len(servers)
	overview["connected"] = connected
	overview["tools"] = totalTools
	overview["servers"] = servers
	return overview
}

// ── Go Runtime ──────────────────────────────────────────────────────

func (h *StatusHandler) buildGoRuntime() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"goroutines":     runtime.NumGoroutine(),
		"heap_alloc_mb":  m.HeapAlloc / 1024 / 1024,
		"heap_sys_mb":    m.HeapSys / 1024 / 1024,
		"num_gc":         m.NumGC,
		"gc_pause_us":    m.PauseNs[(m.NumGC+255)%256] / 1000,
		"go_version":     runtime.Version(),
	}
}

func (h *StatusHandler) buildNotifyOverview() map[string]any {
	if h.notifyCfg == nil {
		return map[string]any{"email_enabled": false, "webhook_enabled": false, "dingtalk_enabled": false}
	}
	return h.notifyCfg()
}

func (h *StatusHandler) buildProcessStats() map[string]any {
	data, _ := os.ReadFile("/proc/self/status")
	rss, threads := int64(0), int64(0)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 { rss, _ = parseInt64(fields[1]) }
		}
		if strings.HasPrefix(line, "Threads:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 { threads, _ = parseInt64(fields[1]) }
		}
	}
	return map[string]any{"rss_kb": rss, "threads": threads}
}

func parseInt64(s string) (int64, error) {
	var v int64
	for _, c := range s {
		if c >= '0' && c <= '9' { v = v*10 + int64(c-'0') }
	}
	return v, nil
}
func formatCronHuman(cron string) string {
	if cron == "" {
		return "未配置"
	}
	if strings.HasPrefix(cron, "*/") || strings.HasPrefix(cron, "0 */") {
		parts := strings.Fields(cron)
		var minField string
		if len(parts) == 6 {
			minField = parts[1]
		} else {
			minField = parts[0]
		}
		if strings.HasPrefix(minField, "*/") {
			mins := strings.TrimPrefix(minField, "*/")
			return "每 " + mins + " 分钟"
		}
	}
	parts := strings.Fields(cron)
	if len(parts) >= 3 {
		hour := parts[2]
		min := parts[1]
		if hour != "*" && min != "*" {
			return "每天 " + hour + ":" + min
		}
	}
	return cron
}

func getNetworkInfoWithDev(devBytes map[string]devBytesPair) ([]NetInterface, string, string) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, "", ""
	}

	var result []NetInterface
	var localIPv4, localIPv6 string

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil || iface.Flags&net.FlagUp == 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		rx, tx := devBytes[iface.Name].rx, devBytes[iface.Name].tx
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			macStr := mac
			if mac == "" {
				macStr = "-"
			}
			if ip.To4() != nil {
				result = append(result, NetInterface{Name: iface.Name, Addr: ip.String(), MAC: macStr, RxBytes: rx, TxBytes: tx})
				if localIPv4 == "" && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
					localIPv4 = ip.String()
				}
			} else {
				result = append(result, NetInterface{Name: iface.Name, Addr: ip.String(), MAC: macStr, RxBytes: rx, TxBytes: tx})
				if localIPv6 == "" && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && ip.IsGlobalUnicast() {
					localIPv6 = ip.String()
				}
			}
		}
	}

	return result, localIPv4, localIPv6
}

type devBytesPair struct{ rx, tx int64 }




// Trigger 手动触发检测更新
func (h *StatusHandler) Trigger(c echo.Context) error {
	if err := h.scheduler.TriggerUpdate(c.Request().Context()); err != nil {
		slog.Warn("触发更新失败", "error", err)
		return c.JSON(http.StatusInternalServerError, errorResp("触发更新失败"))
	}

	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": "更新任务已触发",
	}))
}

// Health 存活探针
func (h *StatusHandler) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "alive"})
}

// ClearMemory 触发全局系统内存清理
// POST /api/system/gc
func (h *StatusHandler) ClearMemory(c echo.Context) error {
	beforeFree, beforeTotal := readMemAvailable()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 5; i++ {
		runtime.GC()
		debug.FreeOSMemory()
		runtime.ReadMemStats(&after)
		if after.HeapInuse >= before.HeapInuse && i > 1 {
			break
		}
		before.HeapInuse = after.HeapInuse
	}
	heapFreed := int64(before.HeapInuse) - int64(after.HeapInuse)
	if heapFreed < 0 {
		heapFreed = 0
	}

	syncFilesystems()
	dropOk := dropCaches()

	afterFree, _ := readMemAvailable()
	sysFreed := afterFree - beforeFree
	if sysFreed < 0 {
		sysFreed = 0
	}

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"message":         "全局内存已清理",
		"heap_freed_kb":   heapFreed / 1024,
		"system_freed_kb": sysFreed,
		"before_free_kb":  beforeFree,
		"after_free_kb":   afterFree,
		"total_mem_kb":    beforeTotal,
		"drop_caches":     dropOk,
	}))
}



// Ready 就绪探针
func (h *StatusHandler) Ready(c echo.Context) error {
	if !h.scheduler.IsRunning() {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}

// SetupStatus 返回系统初始化状态（引导程序用）
func (h *StatusHandler) SetupStatus(c echo.Context) error {
	type Status struct {
		AdminPasswordSet bool     `json:"admin_password_set"`
		YunxiUserExists  bool     `json:"yunxi_user_exists"`
		YunxiGroupExists bool     `json:"yunxi_group_exists"`
		SandboxOK        bool     `json:"sandbox_ok"`
		SandboxPath      string   `json:"sandbox_path"`
		Commands         []string `json:"commands"`
	}
	sandboxPath := defaultSandboxPath()
	st := Status{SandboxPath: sandboxPath}

	// 检查 admin 密码
	if h.userRepo != nil {
		admin, err := h.userRepo.GetByUsername(c.Request().Context(), "admin")
		if err == nil && admin.PasswordHash != "" && admin.PasswordHash != "$2a$10$default" {
			st.AdminPasswordSet = true
		}
	}

	// Platform-specific user/group checks
	st.YunxiUserExists, st.YunxiGroupExists = checkUnixUsers()

	// 检查沙箱权限
	if info, err := os.Stat(sandboxPath); err == nil && info.IsDir() {
		st.SandboxOK = true
	}

	// 生成修复命令
	if !st.YunxiGroupExists {
		st.Commands = append(st.Commands, "sudo groupadd yunxi")
	}
	if !st.YunxiUserExists {
		st.Commands = append(st.Commands, "sudo useradd -r -g yunxi -s /usr/sbin/nologin -d /opt/yunxi-home yunxi")
	}
	if !st.SandboxOK {
		st.Commands = append(st.Commands, "sudo mkdir -p "+sandboxPath)
	}
	st.Commands = append(st.Commands,
		"sudo chown -R yunxi:yunxi /opt/yunxi-home/data",
		"sudo chmod -R 770 /opt/yunxi-home/data",
	)
	return c.JSON(http.StatusOK, successResp(st))
}

// RunSetup 执行系统初始化命令（引导程序一键配置）
func (h *StatusHandler) RunSetup(c echo.Context) error {
	type Step struct {
		Name    string `json:"name"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	var steps []Step
	sandboxPath := defaultSandboxPath()

	// 1. 创建 yunxi 用户组
	addGroup := func() Step {
		_, yunxiGroup := checkUnixUsers()
		if yunxiGroup {
			return Step{Name: "yunxi 用户组", Success: true, Message: "已存在"}
		}
		return Step{Name: "yunxi 用户组", Success: false, Message: "权限不足，请手动执行: sudo groupadd yunxi"}
	}
	steps = append(steps, addGroup())

	// 2. 创建 yunxi 用户
	addUser := func() Step {
		yunxiUser, _ := checkUnixUsers()
		if yunxiUser {
			return Step{Name: "yunxi 用户", Success: true, Message: "已存在"}
		}
		return Step{Name: "yunxi 用户", Success: false, Message: "权限不足，请手动执行: sudo useradd -r -g yunxi -s /usr/sbin/nologin -d /opt/yunxi-home yunxi"}
	}
	steps = append(steps, addUser())

	// 3. 沙箱目录
	addSandbox := func() Step {
		os.MkdirAll(sandboxPath, 0770)
		if info, err := os.Stat(sandboxPath); err == nil && info.IsDir() {
			return Step{Name: "沙箱目录", Success: true, Message: sandboxPath}
		}
		return Step{Name: "沙箱目录", Success: false, Message: "创建失败: " + sandboxPath}
	}
	steps = append(steps, addSandbox())

	// 4. sudo 免密配置
	addSudo := func() Step {
		err := writeSudoers()
		if err != nil {
			return Step{Name: "sudo 权限", Success: false, Message: "写入失败，请手动执行: echo 'yunxi ALL=(ALL) NOPASSWD: ALL' | sudo tee /etc/sudoers.d/yunxi"}
		}
		return Step{Name: "sudo 免密权限", Success: true, Message: "yunxi 用户可免密执行 sudo"}
	}
	steps = append(steps, addSudo())

	// 5. 尝试 chown（可能无权限）
	if runtime.GOOS != "windows" { os.Chown(sandboxPath, 0, 0) } // best-effort
	os.Chmod(sandboxPath, 0770)

	// 统计
	allOk := true
	for _, s := range steps {
		if !s.Success {
			allOk = false
			break
		}
	}
	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"all_ok": allOk,
		"steps":  steps,
	}))
}

func defaultSandboxPath() string {
	if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".yunxi", "data", "yunxiFiles")
	}
	return "/opt/yunxi-home/data/yunxiFiles"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
