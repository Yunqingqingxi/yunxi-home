package sysctl

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"github.com/Yunqingqingxi/yunxi-home/internal/sysctl/base"
)

var log = logger.ForComponent("sysctl")

// systemController 系统控制器实现
type systemController struct {
	serviceControl bool
	processControl bool
}

// 编译期检查接口实现
var _ base.Controller = (*systemController)(nil)

// New 创建系统控制器
func New(serviceControl, processControl bool) Controller {
	return &systemController{
		serviceControl: serviceControl,
		processControl: processControl,
	}
}

// GetSystemInfo 获取系统基本信息
func (c *systemController) GetSystemInfo() (*base.SysInfo, error) {
	log.Info("获取系统信息")
	hostname, _ := os.Hostname()
	return &base.SysInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
	}, nil
}

// ListProcesses 列出进程 (跨平台: 使用 ps 或 tasklist)
func (c *systemController) ListProcesses(limit int) ([]base.ProcessInfo, error) {
	if !c.processControl {
		return nil, fmt.Errorf("进程管理未启用")
	}
	if limit <= 0 {
		limit = 50
	}
	return listProcesses(limit)
}

// KillProcess 终止进程
func (c *systemController) KillProcess(pid int, force bool) error {
	if !c.processControl {
		return fmt.Errorf("进程管理未启用")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("进程不存在: %w", err)
	}
	if force {
		return proc.Kill()
	}
	return proc.Signal(os.Interrupt)
}

// RunCommand 执行系统命令并返回输出
func (c *systemController) RunCommand(name string, args ...string) (string, error) {
	log.Info("执行命令", "命令", name)
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("命令执行失败: %w", err)
	}
	return string(output), nil
}

// parseUptime 解析系统运行时间 (Linux: /proc/uptime, Windows: stub)
func parseUptime() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return ""
	}
	parts := strings.Fields(string(data))
	if len(parts) == 0 {
		return ""
	}
	secs, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return ""
	}
	days := int(secs) / 86400
	hours := (int(secs) % 86400) / 3600
	mins := (int(secs) % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// parseLoadAvg 解析系统负载 (Linux: /proc/loadavg, Windows: stub)
func parseLoadAvg() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
