package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// RegisterTools 将 MCP 管理器中所有服务器的工具注册到 AI 工具注册表。
// 工具名加 `mcp_` 前缀以避免与内置工具冲突。
func RegisterTools(mgr *Manager, reg ToolRegistry) {
	for _, entry := range mgr.AllTools() {
		// 为每个工具创建闭包捕获的变量
		svrName := entry.Server
		toolName := entry.Tool.Name
		desc := entry.Tool.Description
		schema := entry.Tool.InputSchema
		regName := "mcp_" + svrName + "_" + sanitizeName(toolName)

		td := &base.ToolDef{
			Name:        regName,
			Description: "[MCP:" + svrName + "] " + desc,
			Category:    "mcp",
			RiskLevel:   "mutation",
			Parameters:   convertSchema(schema),
			HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
				return callMCPTool(ctx, mgr, svrName, toolName, args)
			},
		}
		reg.Register(regName, td)
	}
}

// ToolRegistry 工具注册的最小接口（避免循环导入）
type ToolRegistry interface {
	Register(name string, td *base.ToolDef)
}

// convertSchema 将 JSON Schema 转换为 base.ToolParams
func convertSchema(schema map[string]any) base.ToolParams {
	params := base.ToolParams{
		Type:       "object",
		Properties: make(map[string]base.ParamProp),
	}

	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return params
	}

	requiredMap := make(map[string]bool)
	if reqArr, ok := schema["required"].([]any); ok {
		for _, r := range reqArr {
			if s, ok := r.(string); ok {
				requiredMap[s] = true
				params.Required = append(params.Required, s)
			}
		}
	}

	for name, raw := range props {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pp := base.ParamProp{}
		if t, ok := prop["type"].(string); ok {
			pp.Type = t
		} else {
			pp.Type = "string"
		}
		if d, ok := prop["description"].(string); ok {
			pp.Description = d
		}
		if e, ok := prop["enum"].([]any); ok {
			for _, v := range e {
				pp.Enum = append(pp.Enum, fmt.Sprintf("%v", v))
			}
		}
		// 嵌套属性
		if items, ok := prop["items"].(map[string]any); ok {
			itemProp := base.ParamProp{}
			if t, ok := items["type"].(string); ok {
				itemProp.Type = t
			}
			pp.Items = &itemProp
		}
		params.Properties[name] = pp
	}

	return params
}

func callMCPTool(ctx context.Context, mgr *Manager, serverName, toolName string, args map[string]any) *base.ToolResult {
	client := mgr.GetClient(serverName)
	if client == nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Summary: fmt.Sprintf("MCP 服务器 %s 未连接", serverName),
			Error:   &base.ToolError{Code: "MCP_NOT_FOUND", Message: fmt.Sprintf("server %s not connected", serverName)},
		}
	}

	output, err := client.CallTool(ctx, toolName, args)
	if err != nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Summary: fmt.Sprintf("MCP 工具调用失败: %s", err.Error()),
			Error:   &base.ToolError{Code: "MCP_CALL_FAILED", Message: err.Error()},
		}
	}

	// 截断过长输出
	summary := output
	if len(summary) > 4000 {
		summary = summary[:4000] + "\n...(已截断)"
	}

	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: summary,
		Data:    output,
	}
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}
