//go:build windows

package sysctl

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/yxd/yunxi-home/internal/sysctl/base"
)

func listProcesses(limit int) ([]ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var procs []ProcessInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV format: "name.exe","pid","session","session#","mem"
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}
		name := strings.Trim(fields[0], "\"")
		pidStr := strings.Trim(fields[1], "\"")
		memStr := strings.Trim(fields[4], "\"")
		memStr = strings.TrimSuffix(memStr, " K")
		memStr = strings.ReplaceAll(memStr, ",", "")

		pid, _ := strconv.Atoi(pidStr)
		memKB, _ := strconv.ParseFloat(memStr, 64)
		procs = append(procs, ProcessInfo{
			PID:   pid,
			Name:  name,
			MemMB: memKB / 1024,
		})
		if len(procs) >= limit {
			break
		}
	}
	return procs, nil
}

// ListServices Windows 不支持 systemd
func (c *systemController) ListServices() ([]base.ServiceInfo, error) {
	return nil, nil
}

// ControlService Windows 不支持 systemd
func (c *systemController) ControlService(name, action string) error {
	return nil
}
