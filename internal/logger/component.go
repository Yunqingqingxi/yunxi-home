package logger

import "log/slog"

// Logger 是 slog.Logger 的类型别名，允许外部包通过 logger.Logger
// 引用 *slog.Logger 而无需导入 log/slog。
type Logger = slog.Logger

// Default 返回 slog.Default() 的别名，避免外部直接引用 slog。
func Default() *Logger {
	return slog.Default()
}

// ForComponent 返回一个预挂 component 属性的 *Logger。
// 每个包应在包级别声明：
//
//	var log = logger.ForComponent("dns")
//
// 之后所有 log.Info(...) 调用自动带上 component="dns"。
func ForComponent(name string) *Logger {
	return slog.Default().With(slog.String(KeyComponent, name))
}
