// Package query provides a lightweight, option-driven AI client for single-turn,
// non-conversation queries. It wraps base.AIProvider to handle timeout, error,
// streaming collection, and token tracking in a consistent way.
//
// Use this for one-shot tasks like title generation, summarization, intent
// classification, and hint generation. Do NOT use for multi-turn chat (that
// remains the main Service.StreamChat loop) or sub-agent loops.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// DefaultTimeout is used when no explicit timeout is configured.
const DefaultTimeout = 30 * time.Second

// ── Options ────────────────────────────────────────────────────────────────────

type options struct {
	timeout  time.Duration
	model    string
	jsonMode bool
	tools    []base.ToolDef
}

// Option configures a single-turn query.
type Option func(*options)

// WithTimeout sets a per-query timeout. Default is 30s.
func WithTimeout(d time.Duration) Option { return func(o *options) { o.timeout = d } }

// WithModel overrides the default model for this query.
func WithModel(name string) Option { return func(o *options) { o.model = name } }

// WithJSONOutput requests the provider to emit structured JSON (where supported).
func WithJSONOutput(enabled bool) Option { return func(o *options) { o.jsonMode = enabled } }

// WithTools passes tool definitions to the provider for this query.
func WithTools(tools []base.ToolDef) Option { return func(o *options) { o.tools = tools } }

// ── Result ─────────────────────────────────────────────────────────────────────

// Result holds the outcome of a single-turn AI query.
type Result struct {
	Content   string            `json:"content"`   // The accumulated text response
	Reasoning string            `json:"reasoning"` // Thinking/reasoning content (if any)
	Usage     *base.StreamUsage `json:"usage"`     // Token usage (may be nil)
	Err       error             `json:"-"`         // Error (timeout, provider error, etc.)
}

// OK returns true if the query succeeded without error.
func (r *Result) OK() bool { return r.Err == nil && r.Content != "" }

// ── Client ─────────────────────────────────────────────────────────────────────

// Client wraps a base.AIProvider for consistent single-turn queries.
type Client struct {
	provider base.AIProvider
}

// New creates a new Client backed by the given provider.
func New(provider base.AIProvider) *Client {
	return &Client{provider: provider}
}

// Chat sends a single-turn query and returns the accumulated text response.
// It handles timeout creation, stream iteration, error collection, and usage capture.
func (c *Client) Chat(ctx context.Context, messages []base.Message, opts ...Option) Result {
	cfg := c.resolveOpts(opts)

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	// Apply model override via context
	if cfg.model != "" {
		ctx = context.WithValue(ctx, base.ModelOverrideKey{}, cfg.model)
	}

	// Apply JSON output mode
	if cfg.jsonMode {
		ctx = context.WithValue(ctx, base.JSONOutputKey{}, true)
	}

	stream, err := c.provider.ChatStream(ctx, messages, cfg.tools)
	if err != nil {
		return Result{Err: fmt.Errorf("provider call failed: %w", err)}
	}

	var contentBuf, reasoningBuf strings.Builder
	var usage *base.StreamUsage
	var streamErr error

	for ev := range stream {
		switch ev.Type {
		case "content":
			contentBuf.WriteString(ev.Content)
		case "thinking":
			reasoningBuf.WriteString(ev.Content)
		case "error":
			if streamErr == nil {
				streamErr = fmt.Errorf("%s", ev.Content)
			}
		case "done":
			if ev.Usage != nil {
				usage = ev.Usage
			}
		}
	}

	// Check context deadline AFTER stream ends (provider may have timed out)
	if ctx.Err() != nil && streamErr == nil {
		streamErr = ctx.Err()
	}

	return Result{
		Content:   strings.TrimSpace(contentBuf.String()),
		Reasoning: strings.TrimSpace(reasoningBuf.String()),
		Usage:     usage,
		Err:       streamErr,
	}
}

// ChatJSON is like Chat but unmarshals the response content into target.
// It automatically appends WithJSONOutput(true) to the options.
func (c *Client) ChatJSON(ctx context.Context, messages []base.Message, target any, opts ...Option) error {
	opts = append(opts, WithJSONOutput(true))
	result := c.Chat(ctx, messages, opts...)
	if result.Err != nil {
		return result.Err
	}
	if result.Content == "" {
		return fmt.Errorf("empty JSON response")
	}
	if err := json.Unmarshal([]byte(result.Content), target); err != nil {
		return fmt.Errorf("json parse: %w (raw: %.200s)", err, result.Content)
	}
	return nil
}

// Provider returns the underlying provider (for compatibility with existing code).
func (c *Client) Provider() base.AIProvider { return c.provider }

// ── Helpers ────────────────────────────────────────────────────────────────────

func (c *Client) resolveOpts(opts []Option) options {
	cfg := options{timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
