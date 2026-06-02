package deepseek

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// ── API Types ──────────────────────────────────────────────

type dsMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent *string    `json:"reasoning_content,omitempty"`
	ToolCalls        []dsTC     `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type dsTC struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Function dsFunction `json:"function"`
	Index    int        `json:"index"`
}

type dsFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type dsToolDef struct {
	Type     string       `json:"type"`
	Function dsToolFunc   `json:"function"`
}

type dsToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type dsRequest struct {
	Model     string       `json:"model"`
	Messages  []dsMessage  `json:"messages"`
	Stream    bool         `json:"stream"`
	Tools     []dsToolDef  `json:"tools,omitempty"`
	Thinking  *dsThinking  `json:"thinking,omitempty"`
}

type dsThinking struct {
	Type string `json:"type"`
}

type dsChunk struct {
	Choices []dsChoice `json:"choices"`
	Usage   *dsUsage   `json:"usage,omitempty"`
}

type dsChoice struct {
	Delta        dsDelta `json:"delta"`
	FinishReason string  `json:"finish_reason"`
}

type dsDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
	ToolCalls        []dsTC `json:"tool_calls"`
}

type dsUsage struct {
	PromptTokens          int `json:"prompt_tokens"`
	CompletionTokens      int `json:"completion_tokens"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
}

type toolCallAccum struct {
	id   string
	name string
	args strings.Builder
}

// ── Provider ───────────────────────────────────────────────

type Provider struct {
	apiKey      string
	baseURL     string
	model       string
	client      *http.Client
	inputPrice  float64
	outputPrice float64
	cachePrice  float64
	cacheMetrics struct {
		HitTokens     int
		MissTokens    int
		TotalRequests int
		TotalCost     float64
	}
}

type Config struct {
	APIKey string
}

func init() {
	base.RegisterProvider("deepseek", func(cfg base.ProviderConfig) (base.AIProvider, error) {
		return New(Config{APIKey: cfg.APIKey}), nil
	})
}

func New(cfg Config) *Provider {
	// v3.1: IPv4-only transport — DeepSeek has no AAAA record, Go's DualStack causes
	// intermittent "unexpected EOF" / "TLS handshake timeout" when IPv6 is attempted.
	// DialContext forces tcp4; TLS is handled by the HTTP client on top.
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Provider{
		apiKey:     cfg.APIKey,
		baseURL:    "https://api.deepseek.com",
		model:      "deepseek-v4-flash",
		client:     &http.Client{Transport: transport, Timeout: 10 * time.Minute},
		inputPrice: 0.28,   // 元/百万 tokens
		outputPrice: 1.10,  // 元/百万 tokens
		cachePrice: 0.07,   // 元/百万 tokens (cache hit)
	}
}

func (p *Provider) Models() []string {
	return []string{"deepseek-v4-flash", "deepseek-v4-pro"}
}

func (p *Provider) DefaultReasoning() string { return "high" }

// ── ChatStream ─────────────────────────────────────────────

func (p *Provider) ChatStream(ctx context.Context, messages []base.Message, tools []base.ToolDef) (<-chan base.ChatStreamEvent, error) {
	dsMsgs := make([]dsMessage, len(messages))
	for i, m := range messages {
		dm := dsMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.ReasoningContent != "" {
			rc := m.ReasoningContent
			dm.ReasoningContent = &rc
		}
		for _, tc := range m.ToolCalls {
			dm.ToolCalls = append(dm.ToolCalls, dsTC{
				ID:   tc.ID,
				Type: tc.Type,
				Function: dsFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		if m.ToolCallID != "" {
			dm.ToolCallID = m.ToolCallID
		}
		dsMsgs[i] = dm
	}

	dsTools := make([]dsToolDef, 0, len(tools))
	for _, t := range tools {
		dsTools = append(dsTools, dsToolDef{
			Type: "function",
			Function: dsToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	// Resolve model override from context
	model := p.model
	if override, ok := ctx.Value(base.ModelOverrideKey{}).(string); ok && override != "" {
		model = override
	}

	// Resolve thinking config from context (reasoning intensity)
	var thinking *dsThinking
	if intensity, ok := ctx.Value(base.ReasoningIntensityKey{}).(string); ok {
		switch intensity {
		case "low":
			// omit thinking field entirely for fast mode
		case "high":
			thinking = &dsThinking{Type: "enabled"}
		default:
			thinking = &dsThinking{Type: "enabled"}
		}
	}

	reqBody := dsRequest{
		Model:    model,
		Messages: dsMsgs,
		Stream:   true,
		Tools:    dsTools,
		Thinking: thinking,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	// v3.1: SHA256 of system prompt for prefix cache identification
	if len(messages) > 0 && messages[0].Role == "system" {
		req.Header.Set("X-Prompt-Hash", base.SystemPromptHash(messages[0].Content))
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 检测非 SSE 响应
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("非流式响应: %s", strings.TrimSpace(string(body)))
	}

	ch := make(chan base.ChatStreamEvent, 64)
	go p.readStream(ctx, resp, ch)
	return ch, nil
}

// ── readStream ─────────────────────────────────────────────

func (p *Provider) readStream(ctx context.Context, resp *http.Response, ch chan<- base.ChatStreamEvent) {
	defer resp.Body.Close()
	defer close(ch)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	var (
		contentBuf, reasoningBuf strings.Builder
		toolCallMap              = make(map[int]*toolCallAccum)
		finished                 bool
		firstToken               time.Time
		lastChunk                *dsChunk
		lastEvent                = time.Now()
	)
	done := make(chan struct{})
	defer close(done)
	go func() {
		tk := time.NewTicker(15 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				if time.Since(lastEvent) > 15*time.Second {
					select {
					case ch <- base.ChatStreamEvent{Type: "keepalive", Content: "等待响应..."}:
					default:
					}
				}
			case <-done:
				return
			}
		}
	}()
	emit := func(ev base.ChatStreamEvent) {
		select {
		case ch <- ev:
		default:
		}
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, ":") {
			lastEvent = time.Now()
			continue
		}
		if line == "" || line == "data: [DONE]" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var chunk dsChunk
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			continue
		}
		lastChunk, lastEvent = &chunk, time.Now()
		for _, c := range chunk.Choices {
			if c.Delta.ReasoningContent != "" {
				if firstToken.IsZero() {
					firstToken = time.Now()
				}
				reasoningBuf.WriteString(c.Delta.ReasoningContent)
				emit(base.ChatStreamEvent{Type: "thinking", Content: c.Delta.ReasoningContent})
			}
			if c.Delta.Content != "" {
				if firstToken.IsZero() {
					firstToken = time.Now()
				}
				contentBuf.WriteString(c.Delta.Content)
				emit(base.ChatStreamEvent{Type: "content", Content: c.Delta.Content})
			}
			for _, tc := range c.Delta.ToolCalls {
				idx := tc.Index
				if _, ok := toolCallMap[idx]; !ok {
					toolCallMap[idx] = &toolCallAccum{}
				}
				if tc.ID != "" {
					toolCallMap[idx].id = tc.ID
				}
				if tc.Function.Name != "" {
					toolCallMap[idx].name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCallMap[idx].args.WriteString(tc.Function.Arguments)
				}
			}
			if c.FinishReason != "" {
				finished = true
			}
		}
	}

	// ── End of stream ──
	if !finished && contentBuf.Len() == 0 && reasoningBuf.Len() == 0 && len(toolCallMap) == 0 {
		emit(base.ChatStreamEvent{Type: "error", Content: "模型未返回有效响应"})
		return
	}
	var cost float64
	var total int
		if lastChunk != nil && lastChunk.Usage != nil {
			p.cacheMetrics.HitTokens += lastChunk.Usage.PromptCacheHitTokens
		p.cacheMetrics.MissTokens += lastChunk.Usage.PromptCacheMissTokens
		p.cacheMetrics.TotalRequests++
		cost = float64(lastChunk.Usage.PromptCacheHitTokens)*p.cachePrice/1e6 +
			float64(lastChunk.Usage.PromptCacheMissTokens)*p.inputPrice/1e6 +
			float64(lastChunk.Usage.CompletionTokens)*p.outputPrice/1e6
		p.cacheMetrics.TotalCost += cost
		total = lastChunk.Usage.PromptCacheHitTokens + lastChunk.Usage.PromptCacheMissTokens
		rate := float64(0)
		if total > 0 {
			rate = float64(lastChunk.Usage.PromptCacheHitTokens) / float64(total) * 100
		}
		slog.Info("DeepSeek", "model", p.model, "hit", lastChunk.Usage.PromptCacheHitTokens,
			"miss", lastChunk.Usage.PromptCacheMissTokens, "out", lastChunk.Usage.CompletionTokens,
			"rate", fmt.Sprintf("%.1f%%", rate), "cost", fmt.Sprintf("%.6f元", cost))
	}
	slog.Info("流式响应完成", "模型", p.model, "内容长度", contentBuf.Len(), "思考长度", reasoningBuf.Len(), "工具调用数", len(toolCallMap))

	for _, tc := range toolCallMap {
		args := tc.args.String()
		if args == "" {
			args = "{}"
		}
		emit(base.ChatStreamEvent{Type: "tool_call", Tool: tc.name, Args: args})
	}
	if lastChunk != nil && lastChunk.Usage != nil {
			total := lastChunk.Usage.PromptCacheHitTokens + lastChunk.Usage.PromptCacheMissTokens
			emit(base.ChatStreamEvent{Type: "done", Usage: &base.StreamUsage{
				PromptTokens:     total,
				CompletionTokens: lastChunk.Usage.CompletionTokens,
				TotalTokens:      total + lastChunk.Usage.CompletionTokens,
				Cost:             cost,
			}})
		} else {
			emit(base.ChatStreamEvent{Type: "done"})
		}
}

// TestConnection validates the API key by listing models.
func (p *Provider) TestConnection(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", strings.TrimSuffix(p.baseURL, "/")+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API 返回 %d", resp.StatusCode)
	}
	return nil
}

