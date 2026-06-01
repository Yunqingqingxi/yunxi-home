// Package base 定义系统控制模块的通用类型和接口，零外部依赖。
package base

import "context"

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	CPU     float64 `json:"cpu"`
	MemMB   float64 `json:"mem_mb"`
	MemPct  float64 `json:"mem_pct"`
	User    string  `json:"user,omitempty"`
	Status  string  `json:"status,omitempty"`
	Command string  `json:"command,omitempty"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // active, inactive, failed
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// SysInfo 系统信息
type SysInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUs     int    `json:"cpus"`
	Uptime   string `json:"uptime,omitempty"`
	LoadAvg  string `json:"load_avg,omitempty"`
}

// Controller 系统控制接口
type Controller interface {
	GetSystemInfo() (*SysInfo, error)
	ListProcesses(limit int) ([]ProcessInfo, error)
	KillProcess(pid int, force bool) error
	RunCommand(name string, args ...string) (string, error)
	ListServices() ([]ServiceInfo, error)
	ControlService(name, action string) error
}

// Ensure context import is used by implementations
var _ = context.Background
