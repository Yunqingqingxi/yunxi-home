// Package sysctl 统一系统控制模块入口。
//
// 消费者只需导入此包即可使用系统控制功能。
//
//	import "github.com/yxd/yunxi-home/internal/sysctl"
//	ctl := sysctl.New(true, true)
//	info, _ := ctl.GetSystemInfo()
package sysctl

import "github.com/yxd/yunxi-home/internal/sysctl/base"

// ── 类型别名 ─────────────────────────────────────────────────

// ProcessInfo 进程信息
type ProcessInfo = base.ProcessInfo

// ServiceInfo 服务信息
type ServiceInfo = base.ServiceInfo

// SysInfo 系统信息
type SysInfo = base.SysInfo

// Controller 系统控制接口
type Controller = base.Controller
