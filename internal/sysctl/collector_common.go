package sysctl

import "time"

// SystemSample holds a point-in-time snapshot of system metrics.
type SystemSample struct {
	Timestamp  time.Time      `json:"timestamp"`
	CPUUsage   float64        `json:"cpu_usage"`
	CPUCount   int            `json:"cpu_count"`
	MemTotal   int64          `json:"mem_total"`
	MemUsed    int64          `json:"mem_used"`
	MemUsage   float64        `json:"mem_usage"`
	LoadAvg    string         `json:"load_avg"`
	NetRxBytes int64          `json:"net_rx_bytes"`
	NetTxBytes int64          `json:"net_tx_bytes"`
	NetRxRate  int64          `json:"net_rx_rate"`
	NetTxRate  int64          `json:"net_tx_rate"`
	Interfaces []NetInterface `json:"interfaces"`
}

// NetInterface holds per-interface network stats.
type NetInterface struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	MAC     string `json:"mac"`
	RxBytes int64  `json:"rx_bytes"`
	TxBytes int64  `json:"tx_bytes"`
}
