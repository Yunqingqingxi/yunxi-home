// Package persistence 提供写前日志（WAL）和快照管理。
package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"os"
	"sync"
	"time"
)

// WALEntry 单条 WAL 记录
type WALEntry struct {
	LSN       int64     `json:"lsn"`       // 日志序列号
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`      // state_change / tool_call / checkpoint / snapshot
	State     string    `json:"state"`     // 状态值
	Payload   string    `json:"payload"`   // JSON 编码的额外数据
	Timestamp time.Time `json:"timestamp"`
}

// WAL 写前日志（Write-Ahead Log）
type WAL struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
	nextLSN  int64
	maxSize  int64 // 最大文件大小（字节），超出后轮转
}

// NewWAL 创建 WAL
func NewWAL(filePath string) (*WAL, error) {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to stat WAL file: %w", err)
	}

	w := &WAL{
		file:     f,
		filePath: filePath,
		nextLSN:  info.Size()/256 + 1, // Approximate: avg entry ~256 bytes
		maxSize:  50 * 1024 * 1024,     // 50MB default
	}

	log.Info("WAL opened", "path", filePath, "size", info.Size())
	return w, nil
}

// Write 写入一条 WAL 记录（在 DB 写之前调用）
func (w *WAL) Write(sessionID, entryType, state, payload string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := WALEntry{
		LSN:       w.nextLSN,
		SessionID: sessionID,
		Type:      entryType,
		State:     state,
		Payload:   payload,
		Timestamp: time.Now(),
	}
	w.nextLSN++

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("WAL marshal: %w", err)
	}
	data = append(data, '\n')

	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("WAL write: %w", err)
	}

	// 检查是否需要轮转
	if info, err := w.file.Stat(); err == nil && info.Size() > w.maxSize {
		go w.rotate()
	}

	return nil
}

// WriteStateChange 便捷方法：记录状态变更
func (w *WAL) WriteStateChange(sessionID, fromState, toState, reason string) error {
	payload, _ := json.Marshal(map[string]string{
		"from":   fromState,
		"to":     toState,
		"reason": reason,
	})
	return w.Write(sessionID, "state_change", toState, string(payload))
}

// WriteToolCall 便捷方法：记录工具调用
func (w *WAL) WriteToolCall(sessionID, toolName, args string) error {
	payload, _ := json.Marshal(map[string]string{
		"tool": toolName,
		"args": args,
	})
	return w.Write(sessionID, "tool_call", "executing", string(payload))
}

// Sync 强制刷盘
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// Close 关闭 WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.file.Sync(); err != nil {
		log.Warn("WAL sync on close failed", "error", err)
	}
	return w.file.Close()
}

// rotate 轮转 WAL 文件
func (w *WAL) rotate() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 重命名当前文件
	backupPath := w.filePath + "." + time.Now().Format("20060102_150405")
	w.file.Close()
	if err := os.Rename(w.filePath, backupPath); err != nil {
		log.Error("WAL rotate: rename failed", "error", err)
		return
	}

	// 创建新文件
	f, err := os.OpenFile(w.filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Error("WAL rotate: create new file failed", "error", err)
		return
	}
	w.file = f
	log.Info("WAL rotated", "backup", backupPath)
}

// Replay 重放 WAL 记录（用于崩溃恢复）
func (w *WAL) Replay() ([]WALEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 需要先同步当前文件
	w.file.Sync()

	// 重新打开读取
	data, err := os.ReadFile(w.filePath)
	if err != nil {
		return nil, fmt.Errorf("WAL replay: read failed: %w", err)
	}

	var entries []WALEntry
	lines := splitLines(string(data))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry WALEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Warn("WAL replay: skipped corrupted entry", "line", line[:min(50, len(line))])
			continue
		}
		entries = append(entries, entry)
	}

	log.Info("WAL replay complete", "entries", len(entries))
	return entries, nil
}

// LastLSN 返回最后一条记录的 LSN
func (w *WAL) LastLSN() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nextLSN - 1
}

// ── Helpers ──

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
