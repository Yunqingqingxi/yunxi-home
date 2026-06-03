package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// RememberTool returns a tool for the AI agent to save or update memories.
func (m *Manager) RememberTool() *base.ToolDef {
	return &base.ToolDef{
		Name:        "remember",
		Description: "保存或更新一条持久记忆。同名记忆会被覆盖更新。更新已有记忆时，先用 recall 读取原内容，合并新信息后再写入。",
		Category:    "ops",
		RiskLevel:   "mutation",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name": {
					Type:        "string",
					Description: "记忆名称（kebab-case）。同名=更新，新名=创建",
				},
				"content": {
					Type:        "string",
					Description: "记忆内容（Markdown 格式）。更新时请保留旧内容中有用的部分，追加新信息",
				},
				"type": {
					Type:        "string",
					Description: "类型：user / project / reference / feedback",
					Enum:        []string{"user", "project", "reference", "feedback"},
				},
				"description": {
					Type:        "string",
					Description: "一句话描述，方便以后搜索匹配。更新时如无变化可保持原样",
				},
			},
			Required: []string{"name", "content"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			return m.handleRemember(ctx, args)
		},
	}
}

// RecallTool returns a tool for the AI agent to search memories.
func (m *Manager) RecallTool() *base.ToolDef {
	return &base.ToolDef{
		Name:        "recall",
		Description: "检索持久记忆中保存的信息。可以按名称精确查找或按关键词搜索。",
		Category:    "ops",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name": {
					Type:        "string",
					Description: "记忆名称（可选，精确查找时使用）",
				},
				"query": {
					Type:        "string",
					Description: "搜索关键词（可选，模糊搜索时使用）",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			return m.handleRecall(ctx, args)
		},
	}
}

func (m *Manager) handleRemember(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)
	memType, _ := args["type"].(string)
	description, _ := args["description"].(string)

	if name == "" || content == "" {
		return "", fmt.Errorf("name 和 content 不能为空")
	}

	if memType == "" {
		memType = "reference"
	}

	// 检查是新建还是更新
	_, err := m.Get(name)
	action := "已更新"
	if err != nil {
		action = "已创建"
	}

	mem := &Memory{
		Name:        name,
		Description: description,
		Type:        MemoryType(memType),
		Content:     content,
		Source:      "agent",
	}

	if err := m.Save(ctx, mem); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s记忆: [%s] %s", action, name, description), nil
}

func (m *Manager) handleRecall(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	query, _ := args["query"].(string)

	if name != "" {
		mem, err := m.Get(name)
		if err != nil {
			return fmt.Sprintf("未找到名为 '%s' 的记忆", name), nil
		}
		return formatMemory(mem), nil
	}

	if query != "" {
		matched := m.Match(query)
		if len(matched) == 0 {
			return fmt.Sprintf("未找到与 '%s' 相关的记忆", query), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("找到 %d 条相关记忆:\n\n", len(matched)))
		for _, mem := range matched {
			sb.WriteString(formatMemory(mem))
			sb.WriteString("\n---\n")
		}
		return sb.String(), nil
	}

	summary := m.Summary()
	if summary == "" {
		return "暂无任何记忆。", nil
	}
	return summary, nil
}

func formatMemory(mem *Memory) string {
	return fmt.Sprintf("### [%s] %s\n类型: %s\n\n%s", mem.Name, mem.Description, mem.Type, mem.Content)
}
