package logger

import (
	"context"
	"log/slog"
)

// TraceIDExtractor 和 SpanIDExtractor 是可选的回调函数，由外部 tracing 包
// 注册，用于从 context 中提取 trace/span ID 并自动注入到每条日志记录中。
// 如果未注册（nil），则跳过注入，不影响正常日志。
var (
	TraceIDExtractor func(context.Context) string
	SpanIDExtractor  func(context.Context) string
)

// ctxLogHandler 从 context 中提取 trace/span ID 注入到 slog 记录。
// 它包裹在内层 handler 之外，对所有通过 *Context 方法（InfoContext 等）
// 传入的 context 生效。
type ctxLogHandler struct {
	inner slog.Handler
}

func (h *ctxLogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *ctxLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// 从 context 提取 trace context —— 仅在 extractor 已注册时生效
	if TraceIDExtractor != nil {
		if traceID := TraceIDExtractor(ctx); traceID != "" {
			r.AddAttrs(slog.String(KeyTraceID, traceID))
		}
	}
	if SpanIDExtractor != nil {
		if spanID := SpanIDExtractor(ctx); spanID != "" {
			r.AddAttrs(slog.String(KeySpanID, spanID))
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *ctxLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ctxLogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ctxLogHandler) WithGroup(name string) slog.Handler {
	return &ctxLogHandler{inner: h.inner.WithGroup(name)}
}
