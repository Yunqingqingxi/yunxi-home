package memory

import (
	"context"
	"fmt"
	"sort"
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
				"context_tags": {
					Type:        "string",
					Description: "关联的专用上下文ID，逗号分隔。如 'spec_code_review, spec_documentation'。留空=通用记忆",
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
	contextTagsStr, _ := args["context_tags"].(string)

	if name == "" || content == "" {
		return "", fmt.Errorf("name 和 content 不能为空")
	}

	if memType == "" {
		memType = "reference"
	}

	// Parse context_tags
	var contextTags []string
	if contextTagsStr != "" {
		for _, t := range strings.Split(contextTagsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				contextTags = append(contextTags, t)
			}
		}
	}

	// 检查是新建还是更新 — 保留已有 context_tags（除非明确指定）
	existing, err := m.Get(name)
	action := "已更新"
	if err != nil {
		action = "已创建"
	} else if contextTagsStr == "" && len(existing.ContextTags) > 0 {
		contextTags = existing.ContextTags
	}

	mem := &Memory{
		Name:        name,
		Description: description,
		Type:        MemoryType(memType),
		ContextTags: contextTags,
		Content:     content,
		Source:      "agent",
	}

	if err := m.Save(ctx, mem); err != nil {
		return "", err
	}

	tagInfo := ""
	if len(contextTags) > 0 {
		tagInfo = fmt.Sprintf(" (上下文: %s)", strings.Join(contextTags, ", "))
	}
	return fmt.Sprintf("%s记忆: [%s] %s%s", action, name, description, tagInfo), nil
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

// MemoryTagTool lets the AI tag/untag memories with context IDs.
func (m *Manager) MemoryTagTool() *base.ToolDef {
	return &base.ToolDef{
		Name:        "memory_tag",
		Description: "给记忆打上或移除专用上下文标签。打上标签后该记忆只在对应上下文激活时才注入提示词。移除所有标签则变为通用记忆。",
		Category:    "ops",
		RiskLevel:   "mutation",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name": {
					Type:        "string",
					Description: "记忆名称",
				},
				"context_tags": {
					Type:        "string",
					Description: "要设置的上下文标签，逗号分隔。如 'spec_code_review, spec_documentation'。留空=''则清除所有标签变为通用记忆",
				},
			},
			Required: []string{"name"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			tagsStr, _ := args["context_tags"].(string)

			if name == "" {
				return "请指定记忆名称", nil
			}

			mem, err := m.Get(name)
			if err != nil {
				return fmt.Sprintf("未找到记忆: %s", name), nil
			}

			var tags []string
			if tagsStr != "" {
				for _, t := range strings.Split(tagsStr, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
			mem.ContextTags = tags
			if err := m.Save(ctx, mem); err != nil {
				return "", err
			}

			if len(tags) == 0 {
				return fmt.Sprintf("已清除 [%s] 的上下文标签，现在是通用记忆", name), nil
			}
			return fmt.Sprintf("已设置 [%s] 的上下文标签: %s", name, strings.Join(tags, ", ")), nil
		},
	}
}

// ListMemoryTagsTool lets the AI see all memories and their context tags.
func (m *Manager) ListMemoryTagsTool() *base.ToolDef {
	return &base.ToolDef{
		Name:        "list_memory_tags",
		Description: "列出所有记忆及其上下文标签。在管理记忆分类或切换任务领域时使用。",
		Category:    "ops",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			m.mu.RLock()
			defer m.mu.RUnlock()

			if len(m.memories) == 0 {
				return "暂无任何记忆。", nil
			}

			names := make([]string, 0, len(m.memories))
			for name := range m.memories {
				names = append(names, name)
			}
			sort.Strings(names)

			var sb strings.Builder
			sb.WriteString("## 通用记忆（始终激活）\n")
			generalCount := 0
			for _, name := range names {
				mem := m.memories[name]
				if len(mem.ContextTags) == 0 {
					sb.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Name, mem.Description))
					generalCount++
				}
			}
			if generalCount == 0 {
				sb.WriteString("(无)\n")
			}

			sb.WriteString("\n## 上下文记忆（按需激活）\n")
			ctxMap := make(map[string][]string)
			for _, name := range names {
				mem := m.memories[name]
				for _, tag := range mem.ContextTags {
					ctxMap[tag] = append(ctxMap[tag], fmt.Sprintf("[%s] %s", mem.Name, mem.Description))
				}
			}
			if len(ctxMap) == 0 {
				sb.WriteString("(无)\n")
			} else {
				ctxNames := make([]string, 0, len(ctxMap))
				for tag := range ctxMap {
					ctxNames = append(ctxNames, tag)
				}
				sort.Strings(ctxNames)
				for _, tag := range ctxNames {
					sb.WriteString(fmt.Sprintf("\n### %s\n", tag))
					for _, entry := range ctxMap[tag] {
						sb.WriteString(fmt.Sprintf("- %s\n", entry))
					}
				}
			}
			return sb.String(), nil
		},
	}
}

func formatMemory(mem *Memory) string {
	return fmt.Sprintf("### [%s] %s\n类型: %s\n\n%s", mem.Name, mem.Description, mem.Type, mem.Content)
}
