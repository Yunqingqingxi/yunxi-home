//go:build windows

package handlers

// ── /proc stubs (Windows) ──────────────────────────────────────

func readProcNetDev() map[string]devBytesPair { return nil }

func readCPUTick() (total int64, idle int64) { return 0, 0 }

func readMemAvailable() (avail int64, total int64) { return 0, 0 }

func readLoadAvg() string { return "" }

func syncFilesystems() {}

func dropCaches() bool { return false }

func fillMemInfoUnix(_ *SystemInfo) {}

func checkUnixUsers() (yunxiUser, yunxiGroup bool) { return false, false }

func writeSudoers() error { return nil }

func readUptimeSeconds() float64 { return 0 }
