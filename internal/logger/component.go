package logger

import "log/slog"

// ForComponent 返回一个预挂 component 属性的 *slog.Logger。
// 每个包应在包级别声明：
//
//	var log = logger.ForComponent("dns")
//
// 之后所有 log.Info(...) 调用自动带上 component="dns"。
func ForComponent(name string) *slog.Logger {
	return slog.Default().With(slog.String(KeyComponent, name))
}
