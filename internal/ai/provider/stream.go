// Package provider contains shared streaming infrastructure for AI providers.
package provider

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/util/safego"
)

// ── Tool call accumulator ───────────────────────────────────────────────

// ToolAccum collects streaming tool call fragments and assembles complete calls.
type ToolAccum struct {
	mu   sync.Mutex
	accs map[int]*toolAccumEntry
}

type toolAccumEntry struct {
	ID   string
	Name string
	Args strings.Builder
}

// NewToolAccum creates a tool call accumulator.
func NewToolAccum() *ToolAccum {
	return &ToolAccum{accs: make(map[int]*toolAccumEntry)}
}

// Feed processes a single tool call fragment from a streaming chunk.
// idx: tool call index; id: tool call ID (only on first fragment); name: function name; args: partial JSON args
func (ta *ToolAccum) Feed(idx int, id, name, args string) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	e, ok := ta.accs[idx]
	if !ok {
		e = &toolAccumEntry{}
		ta.accs[idx] = e
	}
	if id != "" {
		e.ID = id
	}
	if name != "" {
		e.Name = name
	}
	if args != "" {
		e.Args.WriteString(args)
	}
}

// Flush returns all accumulated tool calls as base.ToolCall values.
func (ta *ToolAccum) Flush() []base.ToolCall {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	calls := make([]base.ToolCall, 0, len(ta.accs))
	for _, e := range ta.accs {
		args := e.Args.String()
		if args == "" {
			args = "{}"
		}
		calls = append(calls, base.ToolCall{
			ID:   e.ID,
			Type: "function",
			Function: base.FunctionCall{
				Name:      e.Name,
				Arguments: args,
			},
		})
	}
	return calls
}

// Len returns the number of accumulated tool calls.
func (ta *ToolAccum) Len() int {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	return len(ta.accs)
}

// ── SSE Scanner ─────────────────────────────────────────────────────────

// SSEEvent represents a parsed SSE data line.
type SSEEvent struct {
	Data string // raw JSON data after "data: " prefix
	Raw  string // original line
}

// SSEScanner wraps bufio.Scanner for SSE (Server-Sent Events) streams.
type SSEScanner struct {
	scanner *bufio.Scanner
}

// NewSSEScanner creates an SSE scanner over a reader with 256KB buffer.
func NewSSEScanner(r io.Reader) *SSEScanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 256*1024)
	return &SSEScanner{scanner: s}
}

// Next returns the next SSE data event, or nil if stream ended.
func (s *SSEScanner) Next() *SSEEvent {
	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		// Skip SSE comments (keepalive)
		if strings.HasPrefix(line, ":") {
			continue
		}
		// Skip empty lines and [DONE] marker
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			return &SSEEvent{
				Data: strings.TrimPrefix(line, "data: "),
				Raw:  line,
			}
		}
	}
	return nil
}

// Err returns any scanner error.
func (s *SSEScanner) Err() error {
	return s.scanner.Err()
}

// ── Keepalive helper ────────────────────────────────────────────────────

// StartKeepalive starts a keepalive goroutine that emits heartbeat events
// when no data has been received for the given interval. Returns a done channel.
// The caller must close(done) to stop the goroutine.
func StartKeepalive(ch chan<- base.ChatStreamEvent, interval time.Duration, lastEvent *time.Time) chan struct{} {
	done := make(chan struct{})
	safego.Go("provider-keepalive", func() {
		tk := time.NewTicker(interval)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				if time.Since(*lastEvent) > interval {
					select {
					case ch <- base.ChatStreamEvent{Type: "keepalive", Content: "等待响应..."}:
					default:
					}
				}
			case <-done:
				return
			}
		}
	})
	return done
}

// ── Non-blocking emitter ────────────────────────────────────────────────

// NewEmitter creates a non-blocking event emitter.
func NewEmitter(ch chan<- base.ChatStreamEvent) func(base.ChatStreamEvent) {
	return func(ev base.ChatStreamEvent) {
		select {
		case ch <- ev:
		default:
		}
	}
}

// ── Common usage calculator ─────────────────────────────────────────────

// CalculateCost computes the cost of a streaming response in yuan.
func CalculateCost(inputTokens, outputTokens, cacheHitTokens int, inputPrice, outputPrice, cachePrice float64) float64 {
	return float64(cacheHitTokens)*cachePrice/1e6 +
		float64(inputTokens-cacheHitTokens)*inputPrice/1e6 +
		float64(outputTokens)*outputPrice/1e6
}

// ── Common content extraction interface ─────────────────────────────────

// ChunkContent represents the content extracted from a single streaming chunk.
type ChunkContent struct {
	ContentDelta   string // new text content
	ThinkingDelta  string // new thinking/reasoning content
	FinishReason   string // non-empty when stream is finished
	ToolCallFragments []ToolCallFragment
}

// ToolCallFragment is a partial tool call from a streaming chunk.
type ToolCallFragment struct {
	Index int
	ID    string
	Name  string
	Args  string
}

// ── Streaming context ───────────────────────────────────────────────────

// StreamCtx holds the mutable state during a streaming session.
type StreamCtx struct {
	ContentBuf   strings.Builder
	ThinkingBuf  strings.Builder
	ToolAccum    *ToolAccum
	FirstTokenAt time.Time
	LastEventAt  time.Time
	Finished     bool
	LastUsage    *base.StreamUsage
}

// NewStreamCtx creates a new streaming context.
func NewStreamCtx() *StreamCtx {
	return &StreamCtx{
		ToolAccum:   NewToolAccum(),
		LastEventAt: time.Now(),
	}
}

// Feed processes a ChunkContent and emits corresponding SSE events.
func (sc *StreamCtx) Feed(chunk ChunkContent, emit func(base.ChatStreamEvent)) {
	if chunk.ThinkingDelta != "" {
		if sc.FirstTokenAt.IsZero() {
			sc.FirstTokenAt = time.Now()
		}
		sc.ThinkingBuf.WriteString(chunk.ThinkingDelta)
		emit(base.ChatStreamEvent{Type: "thinking", Content: chunk.ThinkingDelta})
	}
	if chunk.ContentDelta != "" {
		if sc.FirstTokenAt.IsZero() {
			sc.FirstTokenAt = time.Now()
		}
		sc.ContentBuf.WriteString(chunk.ContentDelta)
		emit(base.ChatStreamEvent{Type: "content", Content: chunk.ContentDelta})
	}
	for _, frag := range chunk.ToolCallFragments {
		sc.ToolAccum.Feed(frag.Index, frag.ID, frag.Name, frag.Args)
	}
	if chunk.FinishReason != "" {
		sc.Finished = true
	}
	sc.LastEventAt = time.Now()
}

// EmitToolCalls sends accumulated tool call events.
func (sc *StreamCtx) EmitToolCalls(emit func(base.ChatStreamEvent)) {
	for _, tc := range sc.ToolAccum.Flush() {
		emit(base.ChatStreamEvent{
			Type: "tool_call",
			Tool: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}
}

// EmitDone sends a done event with usage info.
func (sc *StreamCtx) EmitDone(emit func(base.ChatStreamEvent)) {
	if sc.LastUsage != nil {
		emit(base.ChatStreamEvent{Type: "done", Usage: sc.LastUsage})
	} else {
		emit(base.ChatStreamEvent{Type: "done"})
	}
}

// HasContent returns true if any content (text or tool calls) was received.
func (sc *StreamCtx) HasContent() bool {
	return sc.ContentBuf.Len() > 0 || sc.ThinkingBuf.Len() > 0 || sc.ToolAccum.Len() > 0
}

