package qwen

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

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
		client:      &http.Client{Timeout: 10 * time.Minute},
		inputPrice:  0.0008 / 1000, // ¥0.0008/1K tokens (qwen-plus)
		outputPrice: 0.002 / 1000,  // ¥0.002/1K tokens (qwen-plus)
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
		contentBuf                  strings.Builder
		toolCallMap                 = make(map[int]*toolCallAccum)
		finished                    bool
		firstToken                  time.Time
		lastChunk                   *qwenChunk
		lastEvent                   = time.Now()
		promptTokens, compTokens    int
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
		var chunk qwenChunk
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			continue
		}
		lastChunk, lastEvent = &chunk, time.Now()
		for _, c := range chunk.Choices {
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
		// Accumulate usage from final chunk
		if chunk.Usage != nil {
			promptTokens = chunk.Usage.PromptTokens
			compTokens = chunk.Usage.CompletionTokens
		}
	}

	// ── End of stream ──
	if !finished && contentBuf.Len() == 0 && len(toolCallMap) == 0 {
		emit(base.ChatStreamEvent{Type: "error", Content: "Qwen 模型未返回有效响应"})
		return
	}
	slog.Info("Qwen 流式响应完成", "模型", p.model, "内容长度", contentBuf.Len(), "工具调用数", len(toolCallMap))

	for _, tc := range toolCallMap {
		args := tc.args.String()
		if args == "" { args = "{}" }
		emit(base.ChatStreamEvent{Type: "tool_call", Tool: tc.name, Args: args})
	}

	if lastChunk != nil {
		cost := float64(promptTokens)*p.inputPrice + float64(compTokens)*p.outputPrice
		emit(base.ChatStreamEvent{Type: "done", Usage: &base.StreamUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: compTokens,
			TotalTokens:      promptTokens + compTokens,
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
