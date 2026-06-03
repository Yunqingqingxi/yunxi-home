package intent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/query"
)

// Classifier uses the LLM to classify user intent into a tool name.
type Classifier struct {
	client      *query.Client
	toolCatalog string // pre-built tool name + description catalog
}

// NewClassifier creates a Stage 2 LLM classifier.
// toolDefs is the list of all registered tools from the registry.
func NewClassifier(client *query.Client, toolDefs []base.ToolDef) *Classifier {
	// Build a compact tool catalog string
	var sb strings.Builder
	for i, t := range toolDefs {
		desc := t.Description
		if desc == "" {
			desc = t.Name
		}
		// Truncate long descriptions
		if len(desc) > 80 {
			desc = desc[:80]
		}
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("%s: %s", t.Name, desc))
	}
	return &Classifier{
		client:      client,
		toolCatalog: sb.String(),
	}
}

// Classify runs the LLM with a small classification prompt and returns a tool name.
// Returns "" for chit-chat or on error/timeout.
func (c *Classifier) Classify(ctx context.Context, userMsg string) string {
	if c.client == nil {
		return ""
	}

	prompt := fmt.Sprintf(
		`你是一个意图路由器。从以下工具中选择最匹配的一个。

工具：
%s

规则：
- 仅输出工具名，不要解释、不要标点
- 闲聊消息（你好、谢谢、今天天气）输出 NONE
- 多意图时选最主要的
- 如果没有匹配的工具，输出 NONE

用户消息：%s`, c.toolCatalog, userMsg)

	messages := []base.Message{{Role: "user", Content: prompt}}

	result := c.client.Chat(ctx, messages, query.WithTimeout(5*time.Second))
	if result.Err != nil || result.Content == "" {
		return ""
	}

	tool := strings.TrimSpace(result.Content)
	// Sanitize: remove quotes, punctuation, newlines
	tool = strings.Trim(tool, `"'`+"`~\n\r\t ")
	tool = strings.ToLower(tool)

	if tool == "" || tool == "none" {
		return ""
	}
	return tool
}
