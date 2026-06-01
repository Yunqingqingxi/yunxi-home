package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxLogFileSize  = 50 * 1024 * 1024 // 50MB — 超过则轮转
	maxRotationKeep = 5                // 最多保留 5 个轮转文件
)

// currentLevel stores the runtime-adjustable log level (atomic for safe concurrent access).
var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(slog.LevelInfo))
}

// SetLevel switches the global log level at runtime.
// Accepted values: "debug", "info", "warn", "error".
func SetLevel(level string) {
	switch level {
	case "debug":
		currentLevel.Store(int32(slog.LevelDebug))
	case "info":
		currentLevel.Store(int32(slog.LevelInfo))
	case "warn":
		currentLevel.Store(int32(slog.LevelWarn))
	case "error":
		currentLevel.Store(int32(slog.LevelError))
	default:
		return
	}
	slog.Info("日志级别已切换", "level", level)
}

// GetLevel returns the current log level string.
func GetLevel() string {
	l := slog.Level(currentLevel.Load())
	switch l {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return "info"
	}
}

// levelGuardHandler wraps a slog.Handler and filters records based on the
// atomic currentLevel. This allows runtime level changes without recreating
// the handler.
type levelGuardHandler struct {
	inner slog.Handler
}

func (h *levelGuardHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.Level(currentLevel.Load())
}

func (h *levelGuardHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.inner.Handle(ctx, r)
}

func (h *levelGuardHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelGuardHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *levelGuardHandler) WithGroup(name string) slog.Handler {
	return &levelGuardHandler{inner: h.inner.WithGroup(name)}
}

// Init 初始化结构化日志
func Init(level, dir, format string) (*slog.Logger, error) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	currentLevel.Store(int32(logLevel))

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // allow everything through; levelGuard will filter
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				// 使用北京时间
				t := a.Value.Time().In(time.FixedZone("CST", 8*60*60))
				a.Value = slog.StringValue(t.Format("2006-01-02 15:04:05.000"))
			}
			return a
		},
	}

	// 同时输出到控制台和文件
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if dir != "" {
		logFile, err := openLogFile(dir)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %w", err)
		}
		// 用 DedupWriter 包裹文件输出，折叠连续重复行
		dedup := NewDedupWriter(logFile)
		writers = append(writers, dedup)
	}

	// 根据配置选择 handler 格式
	var baseHandler slog.Handler
	multiWriter := io.MultiWriter(writers...)
	if format == "json" {
		baseHandler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		baseHandler = slog.NewTextHandler(multiWriter, opts)
	}

	// 用 levelGuard 包裹，支持运行时切换级别
	handler := &levelGuardHandler{inner: baseHandler}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, nil
}

// openLogFile 创建按日期组织的日志文件，写入前检查大小并轮转
func openLogFile(dir string) (*os.File, error) {
	now := time.Now().In(time.FixedZone("CST", 8*60*60))
	logDir := filepath.Join(dir, fmt.Sprintf("%04d", now.Year()), fmt.Sprintf("%02d", now.Month()), fmt.Sprintf("%02d", now.Day()))

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "yunxi-home.log")

	// 检查现有文件大小，超过限制则轮转
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > maxLogFileSize {
		rotateSystemLogs(logDir, "yunxi-home.log")
	}

	return os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

// rotateSystemLogs 将当前日志文件轮转为带编号的备份文件
// yunxi-home.log → yunxi-home-001.log (数字越大越旧，最多保留 5 个)
func rotateSystemLogs(logDir, baseName string) {
	for i := maxRotationKeep; i >= 1; i-- {
		oldPath := filepath.Join(logDir, fmt.Sprintf("%s-%03d.log", strings.TrimSuffix(baseName, ".log"), i))
		newPath := filepath.Join(logDir, fmt.Sprintf("%s-%03d.log", strings.TrimSuffix(baseName, ".log"), i+1))
		if i >= maxRotationKeep {
			// 删除最旧的轮转文件
			os.Remove(oldPath)
		} else {
			os.Rename(oldPath, newPath)
		}
	}
	// 当前文件 → -001
	currentPath := filepath.Join(logDir, baseName)
	firstRotated := filepath.Join(logDir, fmt.Sprintf("%s-001.log", strings.TrimSuffix(baseName, ".log")))
	os.Rename(currentPath, firstRotated)
}

// CleanOldLogs 清理超过 maxDays 天的旧日志
func CleanOldLogs(dir string, maxDays int) error {
	if dir == "" {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -maxDays)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if !info.IsDir() {
			return nil
		}

		// 检查是否是日期目录 (YYYY/MM/DD)
		rel, _ := filepath.Rel(dir, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 {
			return nil
		}

		// 尝试解析路径中的日期
		cleanPath := filepath.ToSlash(rel)
		var year, month, day int
		n, _ := fmt.Sscanf(cleanPath, "%04d/%02d/%02d", &year, &month, &day)
		if n == 3 {
			dirDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
			if dirDate.Before(cutoff) {
				parent := filepath.Dir(path)
				os.RemoveAll(path)
				// 清理空的父目录
				removeEmptyParents(parent)
			}
		}
		return nil
	})
}

func removeEmptyParents(dir string) {
	for dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		parent := filepath.Dir(dir)
		os.Remove(dir)
		dir = parent
	}
}

// sortedLogFiles 返回日志目录下按名称排序的日志文件（用于 scanSystemLogs 获取所有相关文件）
func sortedLogFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}
