package base

import (
	"encoding/json"

	dbase "github.com/Yunqingqingxi/yunxi-home/internal/database/base"
)

// SeedPrompts returns all prompts to seed into the DB on first run.
// Go constants from prompt.go serve as the seed data source; after seeding,
// all prompts are managed via DB.
func SeedPrompts() []dbase.PromptRecord {
	return []dbase.PromptRecord{
		// ── General Prompts (每轮必带) ──────────────────────────────
		{
			ID: "gen_identity", Category: "general", Name: "身份铁律",
			Content: IdentityRules, Keywords: "[]", Priority: 10, Enabled: true,
		},
		{
			ID: "gen_environment", Category: "general", Name: "运行环境",
			Content: EnvironmentRules, Keywords: "[]", Priority: 9, Enabled: true,
		},
		{
			ID: "gen_core_rules", Category: "general", Name: "核心行为规则",
			Content: CoreRules, Keywords: "[]", Priority: 8, Enabled: true,
		},
		{
			ID: "gen_communication", Category: "general", Name: "沟通风格",
			Content: CommunicationRules, Keywords: "[]", Priority: 7, Enabled: true,
		},
		{
			ID: "gen_tool_strategy", Category: "general", Name: "工具选择策略",
			Content: ToolStrategy, Keywords: "[]", Priority: 5, Enabled: true,
		},
		{
			ID: "gen_task_boundary", Category: "general", Name: "任务边界约束",
			Content: TaskBoundaryRules, Keywords: "[]", Priority: 4, Enabled: true,
		},
		{
			ID: "gen_mcp_status", Category: "general", Name: "MCP状态判断规范",
			Content: MCPStatusRules, Keywords: "[]", Priority: 3, Enabled: true,
		},
		{
			ID: "gen_slash_command", Category: "general", Name: "斜杠命令处理",
			Content: SlashCommandRules, Keywords: "[]", Priority: 2, Enabled: true,
		},

		// ── Specialized Prompts (按需激活) ──────────────────────────
		{
			ID: "spec_filesystem", Category: "specialized", Name: "文件系统规则",
			Content:  FilesystemRules,
			Keywords: mustMarshal([]string{"文件", "目录", "读取", "写入", "删除", "创建", "复制", "移动", "重命名", "搜索", "下载", "上传", "readme", "列表", "查看", "浏览", "查找", "编辑", "file", "read", "dir", "list", "ls", "cat", "mkdir", "rm", "cp", "mv"}),
			Priority: 10, Enabled: true,
		},
		{
			ID: "spec_project_runner", Category: "specialized", Name: "项目运行/编译规则",
			Content:  CommandExecutionRules + "\n" + TimeoutGuide,
			Keywords: mustMarshal([]string{"运行", "启动", "编译", "构建", "部署", "安装", "执行", "docker", "go build", "npm", "make", "build", "run", "start", "deploy", "install", "compile", "restart", "stop", "测试", "test", "打包", "package"}),
			Priority: 10, Enabled: true,
		},
		{
			ID: "spec_code_review", Category: "specialized", Name: "代码分析/审查规则",
			Content:  CodeReviewPrompt,
			Keywords: mustMarshal([]string{"分析", "优化", "代码", "重构", "项目结构", "建议", "review", "code", "架构", "拆分", "解耦", "依赖", "main.go", "入口", "编译", "报错", "错误", "修复", "bug", "error", "undefined", "重新开始", "继续", "恢复", "上次", "回滚", "重置", "接着", "模块划分", "结构分析"}),
			Priority: 10, Enabled: true,
		},
		{
			ID: "spec_mcp_dev", Category: "specialized", Name: "MCP服务器开发规则",
			Content:  MCPServerDevRules,
			Keywords: mustMarshal([]string{"mcp", "mcpserver", "mcpservers", "mcp服务器", "mcp工具", "mcp-server", "mcp.json", "reload_mcp"}),
			Priority: 8, Enabled: true,
		},
		{
			ID: "spec_file_sending", Category: "specialized", Name: "文件发送规则",
			Content:  FileSendingRules,
			Keywords: mustMarshal([]string{"发送", "分享", "send", "share"}),
			Priority: 5, Enabled: true,
		},
	}
}

func mustMarshal(v []string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
