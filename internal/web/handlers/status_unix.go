//go:build !windows

package handlers

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"

	"log/slog"
)

// ── /proc readers (Linux) ──────────────────────────────────────

func readProcNetDev() map[string]devBytesPair {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		slog.Warn("无法读取/proc/net/dev", "error", err)
		return nil
	}
	defer f.Close()
	result := make(map[string]devBytesPair)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)
		result[name] = devBytesPair{rx, tx}
	}
	return result
}

func readCPUTick() (total int64, idle int64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0
	}

	for i := 1; i < len(fields); i++ {
		val, _ := strconv.ParseInt(fields[i], 10, 64)
		total += val
		if i == 4 {
			idle = val
		}
	}
	return total, idle
}

func readMemAvailable() (avail int64, total int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemAvailable:":
			avail = val
		case "MemTotal:":
			total = val
		}
	}
	return avail, total
}

func readMemInfo() (memTotal, memAvailable int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			memTotal = val
		case "MemAvailable:":
			memAvailable = val
		}
	}
	return
}

func readLoadAvg() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func syncFilesystems() {
	syscall.Sync()
}

func dropCaches() bool {
	err1 := os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0)
	_ = os.WriteFile("/proc/sys/vm/compact_memory", []byte("1\n"), 0)
	return err1 == nil
}

func fillMemInfoUnix(info *SystemInfo) {
	memTotal, memAvailable := readMemInfo()
	if memTotal > 0 {
		info.MemTotal = formatBytes(memTotal * 1024)
		info.MemUsed = formatBytes((memTotal - memAvailable) * 1024)
		info.MemUsage = float64(memTotal-memAvailable) / float64(memTotal) * 100
	}
	info.LoadAvg = readLoadAvg()
}

func checkUnixUsers() (yunxiUser, yunxiGroup bool) {
	if _, err := os.Stat("/etc/passwd"); err == nil {
		data, _ := os.ReadFile("/etc/passwd")
		yunxiUser = strings.Contains(string(data), "yunxi:")
	}
	if _, err := os.Stat("/etc/group"); err == nil {
		data, _ := os.ReadFile("/etc/group")
		yunxiGroup = strings.Contains(string(data), "yunxi:")
	}
	return
}

func writeSudoers() error {
	return os.WriteFile("/etc/sudoers.d/yunxi", []byte("yunxi ALL=(ALL) NOPASSWD: ALL\n"), 0440)
}

func readUptimeSeconds() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) > 0 {
		v, _ := strconv.ParseFloat(parts[0], 64)
		return v
	}
	return 0
}

