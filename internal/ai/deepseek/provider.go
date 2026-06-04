package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/provider"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/resilience"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"github.com/Yunqingqingxi/yunxi-home/internal/util/safego"
)

var log = logger.ForComponent("deepseek")

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
	// v3.2: Use shared HTTP client with IPv4-only transport.
	// DeepSeek has no AAAA record; Go's DualStack causes intermittent
	// "unexpected EOF" / "TLS handshake timeout" when IPv6 is attempted.
	httpCfg := provider.DefaultHTTPClientConfig()
	httpCfg.ForceIPv4 = true
	httpCfg.RequestTimeout = 10 * time.Minute

	return &Provider{
		apiKey:     cfg.APIKey,
		baseURL:    "https://api.deepseek.com",
		model:      "deepseek-v4-flash",
		client:     provider.NewHTTPClient(httpCfg),
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
		// 所有 assistant 消息都带 reasoning_content 字段（thinking 模式要求）
		rc := m.ReasoningContent
		dm.ReasoningContent = &rc
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

	bodyBytes, _ := json.Marshal(reqBody)

	// 重试循环：处理瞬时网络错误（unexpected EOF, connection reset 等）
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Debug("DeepSeek retry", "attempt", attempt+1, "backoff", backoff, "error", lastErr)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Accept", "text/event-stream")
		if len(messages) > 0 && messages[0].Role == "system" {
			req.Header.Set("X-Prompt-Hash", base.SystemPromptHash(messages[0].Content))
		}

		resp, err = p.client.Do(req)
		if err != nil {
			lastErr = err
			if !resilience.IsRetryable(err) {
				return nil, err
			}
			continue
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/event-stream") {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, fmt.Errorf("非流式响应: %s", strings.TrimSpace(string(body)))
		}
		break // success
	}
	if resp == nil {
		return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
	}

	ch := make(chan base.ChatStreamEvent, 64)
	safego.Go("deepseek-readstream", func() {
		p.readStream(ctx, resp, ch)
	})
	return ch, nil
}

// ── readStream ─────────────────────────────────────────────

func (p *Provider) readStream(ctx context.Context, resp *http.Response, ch chan<- base.ChatStreamEvent) {
	defer resp.Body.Close()
	defer close(ch)

	// Shared infrastructure
	streamCtx := provider.NewStreamCtx()
	scanner := provider.NewSSEScanner(resp.Body)
	emit := provider.NewEmitter(ch)
	done := provider.StartKeepalive(ch, 15*time.Second, &streamCtx.LastEventAt)
	defer close(done)

	var lastChunk *dsChunk

	// Main SSE loop
	for {
		if ctx.Err() != nil {
			return
		}
		ev := scanner.Next()
		if ev == nil {
			break
		}
		streamCtx.LastEventAt = time.Now()

		var chunk dsChunk
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			continue
		}
		lastChunk = &chunk

		for _, c := range chunk.Choices {
			// Convert to shared chunk format
			var fragments []provider.ToolCallFragment
			for _, tc := range c.Delta.ToolCalls {
				fragments = append(fragments, provider.ToolCallFragment{
					Index: tc.Index,
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Args:  tc.Function.Arguments,
				})
			}
			streamCtx.Feed(provider.ChunkContent{
				ContentDelta:       c.Delta.Content,
				ThinkingDelta:      c.Delta.ReasoningContent,
				FinishReason:       c.FinishReason,
				ToolCallFragments:  fragments,
			}, emit)
		}
	}

	// ── End of stream ──
	if err := scanner.Err(); err != nil {
		if resilience.IsRetryable(err) {
			emit(base.ChatStreamEvent{Type: "error", Content: fmt.Sprintf("流中断: %v (可重试)", err)})
		} else {
			emit(base.ChatStreamEvent{Type: "error", Content: fmt.Sprintf("流读取错误: %v", err)})
		}
		return
	}
	if !streamCtx.Finished && !streamCtx.HasContent() {
		emit(base.ChatStreamEvent{Type: "error", Content: "模型未返回有效响应"})
		return
	}

	// Cache metrics (DeepSeek-specific)
	var cost float64
	if lastChunk != nil && lastChunk.Usage != nil {
		p.cacheMetrics.HitTokens += lastChunk.Usage.PromptCacheHitTokens
		p.cacheMetrics.MissTokens += lastChunk.Usage.PromptCacheMissTokens
		p.cacheMetrics.TotalRequests++
		cost = provider.CalculateCost(
			lastChunk.Usage.PromptCacheHitTokens+lastChunk.Usage.PromptCacheMissTokens,
			lastChunk.Usage.CompletionTokens,
			lastChunk.Usage.PromptCacheHitTokens,
			p.inputPrice, p.outputPrice, p.cachePrice,
		)
		p.cacheMetrics.TotalCost += cost
		total := lastChunk.Usage.PromptCacheHitTokens + lastChunk.Usage.PromptCacheMissTokens
		rate := float64(0)
		if total > 0 {
			rate = float64(lastChunk.Usage.PromptCacheHitTokens) / float64(total) * 100
		}
		log.Info("DeepSeek", "model", p.model, "hit", lastChunk.Usage.PromptCacheHitTokens,
			"miss", lastChunk.Usage.PromptCacheMissTokens, "out", lastChunk.Usage.CompletionTokens,
			"rate", fmt.Sprintf("%.1f%%", rate), "cost", fmt.Sprintf("%.6f元", cost))
	}
	log.Info("流式响应完成", "模型", p.model, "内容长度", streamCtx.ContentBuf.Len(),
		"思考长度", streamCtx.ThinkingBuf.Len(), "工具调用数", streamCtx.ToolAccum.Len())

	// Emit tool calls and done
	streamCtx.EmitToolCalls(emit)
	if lastChunk != nil && lastChunk.Usage != nil {
		total := lastChunk.Usage.PromptCacheHitTokens + lastChunk.Usage.PromptCacheMissTokens
		emit(base.ChatStreamEvent{Type: "done", Usage: &base.StreamUsage{
			PromptTokens:     total,
			CompletionTokens: lastChunk.Usage.CompletionTokens,
			TotalTokens:      total + lastChunk.Usage.CompletionTokens,
			Cost:             cost,
		}})
	} else {
		streamCtx.EmitDone(emit)
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

