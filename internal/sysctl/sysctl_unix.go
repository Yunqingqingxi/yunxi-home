//go:build !windows

package sysctl

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/yxd/yunxi-home/internal/sysctl/base"
)

func listProcesses(limit int) ([]ProcessInfo, error) {
	cmd := exec.Command("ps", "aux", "--sort=-%mem")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var procs []ProcessInfo
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		pid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)
		procs = append(procs, ProcessInfo{
			PID:     pid,
			User:    fields[0],
			CPU:     cpu,
			MemPct:  mem,
			Name:    fields[10],
			Command: strings.Join(fields[10:], " "),
		})
		if len(procs) >= limit {
			break
		}
	}
	return procs, nil
}

// ListServices 列出 systemd 服务状态
func (c *systemController) ListServices() ([]base.ServiceInfo, error) {
	if !c.serviceControl {
		return nil, nil
	}
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-legend", "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(string(out), "\n")
	var services []ServiceInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "\u25cf") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		if !strings.HasSuffix(name, ".service") {
			continue
		}
		name = strings.TrimSuffix(name, ".service")
		services = append(services, ServiceInfo{
			Name:   name,
			Status: fields[2],
		})
	}
	return services, nil
}

// ControlService 控制 systemd 服务 (start/stop/restart)
func (c *systemController) ControlService(name, action string) error {
	if !c.serviceControl {
		return fmt.Errorf("服务管理未启用")
	}
	allowed := map[string]bool{"start": true, "stop": true, "restart": true, "enable": true, "disable": true}
	if !allowed[action] {
		return fmt.Errorf("不支持的操作: %s", action)
	}
	cmd := exec.Command("systemctl", action, name+".service")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
