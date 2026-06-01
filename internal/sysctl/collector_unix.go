//go:build !windows

package sysctl

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// SystemCollector samples /proc every interval and stores the latest in an atomic.Value.
type SystemCollector struct {
	current atomic.Value // *SystemSample
	prev    prevSample
	stopCh  chan struct{}
}

type prevSample struct {
	cpuTotal, cpuIdle int64
	netRx, netTx      int64
	netTs             time.Time
}

// NewCollector creates a SystemCollector. Call Start() to begin sampling.
func NewCollector() *SystemCollector {
	c := &SystemCollector{stopCh: make(chan struct{})}
	c.current.Store(&SystemSample{})
	return c
}

// Start begins background sampling at the given interval.
func (c *SystemCollector) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// First sample immediately
		c.current.Store(c.sample())
		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.current.Store(c.sample())
			}
		}
	}()
}

// Stop halts background sampling.
func (c *SystemCollector) Stop() { close(c.stopCh) }

// Get returns the latest sample (lock-free).
func (c *SystemCollector) Get() *SystemSample {
	s := c.current.Load()
	if s == nil {
		return &SystemSample{}
	}
	return s.(*SystemSample)
}

func (c *SystemCollector) sample() *SystemSample {
	s := &SystemSample{Timestamp: time.Now()}

	// CPU
	cpuTotal, cpuIdle := readCPUTick()
	s.CPUUsage = calcCPUUsage(c.prev.cpuTotal, c.prev.cpuIdle, cpuTotal, cpuIdle)
	s.CPUCount = countCPUOnline()
	c.prev.cpuTotal, c.prev.cpuIdle = cpuTotal, cpuIdle

	// Memory
	s.MemTotal, s.MemUsed = readMemInfo()
	if s.MemTotal > 0 {
		s.MemUsage = float64(s.MemUsed) / float64(s.MemTotal) * 100
	}

	// Load
	s.LoadAvg = readLoadAvg()

	// Network
	s.Interfaces = readNetDev()
	var totalRx, totalTx int64
	for _, iface := range s.Interfaces {
		totalRx += iface.RxBytes
		totalTx += iface.TxBytes
	}
	s.NetRxBytes = totalRx
	s.NetTxBytes = totalTx

	now := time.Now()
	s.NetRxRate = calcRate(c.prev.netRx, totalRx, c.prev.netTs, now)
	s.NetTxRate = calcRate(c.prev.netTx, totalTx, c.prev.netTs, now)
	c.prev.netRx, c.prev.netTx = totalRx, totalTx
	c.prev.netTs = now

	return s
}

// ── /proc readers ──────────────────────────────────────────────────

func readCPUTick() (total, idle int64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 1, 1
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 1, 1
		}
		for _, f := range fields[1:] {
			v, _ := strconv.ParseInt(f, 10, 64)
			total += v
		}
		idle, _ = strconv.ParseInt(fields[4], 10, 64)
		return total, idle
	}
	return 1, 1
}

func countCPUOnline() int {
	data, _ := os.ReadFile("/sys/devices/system/cpu/online")
	// Format: "0-3" or "0"
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 1
	}
	parts := strings.Split(s, "-")
	if len(parts) == 2 {
		hi, _ := strconv.Atoi(parts[1])
		return hi + 1
	}
	return 1
}

func readMemInfo() (total, used int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 1, 1
	}
	defer f.Close()

	var memTotal, memAvail int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseKB(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") || strings.HasPrefix(line, "MemFree:") {
			memAvail = parseKB(line)
		}
	}
	if memAvail == 0 {
		memAvail = memTotal / 4 // fallback estimate
	}
	return memTotal, memTotal - memAvail
}

func parseKB(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		v, _ := strconv.ParseInt(fields[1], 10, 64)
		return v * 1024 // KB → bytes
	}
	return 0
}

func readLoadAvg() string {
	data, _ := os.ReadFile("/proc/loadavg")
	return strings.TrimSpace(string(data))
}

func readNetDev() []NetInterface {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil
	}
	defer f.Close()

	var result []NetInterface
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "Inter-|") || strings.Contains(line, "face |") || line == "" {
			continue
		}
		// Format: "eth0: rxbytes ... txbytes ..."
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		if name == "lo" {
			continue // skip loopback from totals
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 10 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)

		result = append(result, NetInterface{
			Name:    name,
			RxBytes: rx,
			TxBytes: tx,
		})
	}
	return result
}

// ── helpers ────────────────────────────────────────────────────────

func calcCPUUsage(prevTotal, prevIdle, total, idle int64) float64 {
	dTotal := total - prevTotal
	dIdle := idle - prevIdle
	if dTotal <= 0 {
		return 0
	}
	return float64(dTotal-dIdle) / float64(dTotal) * 100
}

func calcRate(prevBytes, currBytes int64, prevTs, currTs time.Time) int64 {
	delta := currTs.Sub(prevTs).Seconds()
	if delta <= 0 {
		return 0
	}
	return int64(float64(currBytes-prevBytes) / delta)
}
