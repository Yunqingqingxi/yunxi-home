package qwen

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
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"github.com/Yunqingqingxi/yunxi-home/internal/util/safego"
)

var log = logger.ForComponent("qwen")

// ── API Types ──────────────────────────────────────────────

type qwenMessage struct {
	Role       string   `json:"role"`
	Content    string   `json:"content"`
	ToolCalls  []qwenTC `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
}

type qwenTC struct {
	ID       string      `json:"id"`
	Type     string      `json:"type"`
	Function qwenFunc    `json:"function"`
	Index    int         `json:"index"`
}

type qwenFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type qwenToolDef struct {
	Type     string       `json:"type"`
	Function qwenToolFunc `json:"function"`
}

type qwenToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type qwenRequest struct {
	Model    string        `json:"model"`
	Messages []qwenMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Tools    []qwenToolDef `json:"tools,omitempty"`
}

type qwenChunk struct {
	Choices []qwenChoice `json:"choices"`
	Usage   *qwenUsage   `json:"usage,omitempty"`
}

type qwenChoice struct {
	Delta        qwenDelta `json:"delta"`
	FinishReason string    `json:"finish_reason"`
}

type qwenDelta struct {
	Content   string   `json:"content"`
	ToolCalls []qwenTC `json:"tool_calls"`
}

type qwenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ── Provider ───────────────────────────────────────────────

type Provider struct {
	apiKey      string
	baseURL     string
	model       string
	client      *http.Client
	inputPrice  float64
	outputPrice float64
}

type Config struct {
	APIKey string
}

func init() {
	base.RegisterProvider("qwen", func(cfg base.ProviderConfig) (base.AIProvider, error) {
		return New(Config{APIKey: cfg.APIKey}), nil
	})
}

func New(cfg Config) *Provider {
	return &Provider{
		apiKey:      cfg.APIKey,
		baseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		model:       "qwen-plus",
		client:      provider.NewStandardHTTPClient(10 * time.Minute),
		inputPrice:  0.8,  // 元/百万 tokens (qwen-plus: ¥0.0008/1K)
		outputPrice: 2.0,  // 元/百万 tokens (qwen-plus: ¥0.002/1K)
	}
}

func (p *Provider) Models() []string {
	return []string{"qwen-plus", "qwen-max"}
}

func (p *Provider) DefaultReasoning() string { return "medium" }

// ── ChatStream ─────────────────────────────────────────────

func (p *Provider) ChatStream(ctx context.Context, messages []base.Message, tools []base.ToolDef) (<-chan base.ChatStreamEvent, error) {
	qMsgs := make([]qwenMessage, len(messages))
	for i, m := range messages {
		qm := qwenMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		for _, tc := range m.ToolCalls {
			qm.ToolCalls = append(qm.ToolCalls, qwenTC{
				ID:   tc.ID,
				Type: tc.Type,
				Function: qwenFunc{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		if m.ToolCallID != "" {
			qm.ToolCallID = m.ToolCallID
		}
		qMsgs[i] = qm
	}

	qTools := make([]qwenToolDef, 0, len(tools))
	for _, t := range tools {
		qTools = append(qTools, qwenToolDef{
			Type: "function",
			Function: qwenToolFunc{
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

	reqBody := qwenRequest{
		Model:    model,
		Messages: qMsgs,
		Stream:   true,
		Tools:    qTools,
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
		return nil, fmt.Errorf("Qwen API error: %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	// 检测非 SSE 响应（如 JSON 错误）
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("Qwen 非流式响应: %s", strings.TrimSpace(string(body)))
	}

	ch := make(chan base.ChatStreamEvent, 64)
	safego.Go("qwen-readstream", func() {
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

	var lastChunk *qwenChunk
	var promptTokens, compTokens int

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

		var chunk qwenChunk
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			continue
		}
		lastChunk = &chunk

		for _, c := range chunk.Choices {
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
				ContentDelta:      c.Delta.Content,
				FinishReason:      c.FinishReason,
				ToolCallFragments: fragments,
			}, emit)
		}
		if chunk.Usage != nil {
			promptTokens = chunk.Usage.PromptTokens
			compTokens = chunk.Usage.CompletionTokens
		}
	}

	// ── End of stream ──
	if !streamCtx.Finished && !streamCtx.HasContent() {
		emit(base.ChatStreamEvent{Type: "error", Content: "Qwen 模型未返回有效响应"})
		return
	}
	log.Info("Qwen 流式响应完成", "模型", p.model, "内容长度", streamCtx.ContentBuf.Len(),
		"工具调用数", streamCtx.ToolAccum.Len())

	// Emit tool calls and done
	streamCtx.EmitToolCalls(emit)
	if lastChunk != nil {
		cost := provider.CalculateCost(promptTokens, compTokens, 0, p.inputPrice, p.outputPrice, 0)
		emit(base.ChatStreamEvent{Type: "done", Usage: &base.StreamUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: compTokens,
			TotalTokens:      promptTokens + compTokens,
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
