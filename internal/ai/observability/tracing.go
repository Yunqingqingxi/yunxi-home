package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

func init() {
	// 将 trace/span ID 提取器注册到 logger 包，使 ctxLogHandler
	// 能自动从 context 注入 trace_id / span_id 到每条日志。
	logger.TraceIDExtractor = TraceIDFromCtx
	logger.SpanIDExtractor = SpanIDFromCtx
}

// ── Trace ID 管理 ──

type traceIDKey struct{}
type spanIDKey struct{}

// GenerateTraceID 生成新的 trace ID（16 字节 hex）
func GenerateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSpanID 生成新的 span ID（8 字节 hex）
func GenerateSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// WithTraceID 将 trace ID 注入 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceIDFromCtx 从 context 提取 trace ID
func TraceIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

// WithSpanID 将 span ID 注入 context
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey{}, spanID)
}

// SpanIDFromCtx 从 context 提取 span ID
func SpanIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(spanIDKey{}).(string); ok {
		return v
	}
	return ""
}

// ── Span（追踪跨度）──

// Span 追踪跨度
type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]string
	Events     []SpanEvent
}

// SpanEvent 跨度事件
type SpanEvent struct {
	Name      string
	Timestamp time.Time
	Attributes map[string]string
}

// NewSpan 创建新的 span
func NewSpan(ctx context.Context, name string) (context.Context, *Span) {
	traceID := TraceIDFromCtx(ctx)
	if traceID == "" {
		traceID = GenerateTraceID()
		ctx = WithTraceID(ctx, traceID)
	}

	spanID := GenerateSpanID()
	parentID := SpanIDFromCtx(ctx)
	ctx = WithSpanID(ctx, spanID)

	span := &Span{
		TraceID:    traceID,
		SpanID:     spanID,
		ParentID:   parentID,
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]string),
		Events:     make([]SpanEvent, 0),
	}

	slog.LogAttrs(ctx, slog.LevelDebug, "span 开始",
		slog.String(logger.KeyTraceID, traceID),
		slog.String(logger.KeySpanID, spanID),
		slog.String("span_name", name),
		slog.String("parent_span", parentID),
	)

	return ctx, span
}

// Finish 完成 span
func (s *Span) Finish() {
	s.EndTime = time.Now()

	slog.LogAttrs(context.Background(), slog.LevelDebug, "span 结束",
		slog.String(logger.KeyTraceID, s.TraceID),
		slog.String(logger.KeySpanID, s.SpanID),
		slog.String("span_name", s.Name),
		slog.Float64("span_dur_ms", float64(s.Duration().Microseconds())/1000),
	)
}

// SetAttr 设置属性
func (s *Span) SetAttr(key, value string) {
	s.Attributes[key] = value
}

// AddEvent 添加事件
func (s *Span) AddEvent(name string, attrs map[string]string) {
	s.Events = append(s.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

// Duration 跨度耗时
func (s *Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// String 打印 span 树格式（用于调试）
func (s *Span) String() string {
	return fmt.Sprintf("[%s] %s (%v) trace=%s span=%s",
		s.Name, s.Duration(), s.EndTime.Sub(s.StartTime), s.TraceID, s.SpanID)
}

// ── 全局追踪器 ──

// Tracer 追踪器（用于创建和记录 span）
type Tracer struct {
	serviceName string
	spans       []*Span // 内存中的 span 历史
	maxSpans    int
}

// NewTracer 创建追踪器
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		spans:       make([]*Span, 0, 256),
		maxSpans:    1000,
	}
}

// StartSpan 创建新的根 span
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	ctx, span := NewSpan(ctx, name)
	span.SetAttr("service.name", t.serviceName)

	t.spans = append(t.spans, span)
	if len(t.spans) > t.maxSpans {
		t.spans = t.spans[len(t.spans)-t.maxSpans:]
	}

	return ctx, span
}

// RecentSpans 返回最近的 span
func (t *Tracer) RecentSpans(n int) []*Span {
	if n <= 0 || n > len(t.spans) {
		n = len(t.spans)
	}
	start := len(t.spans) - n
	result := make([]*Span, n)
	copy(result, t.spans[start:])
	return result
}
