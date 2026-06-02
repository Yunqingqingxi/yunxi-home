package toolreg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

)

// ── 技能市场：在线搜索 + 下载安装 ──────────────────────────

// SkillMarketItem 技能市场条目
type SkillMarketItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Source      string `json:"source"`      // 来源 URL
	DownloadURL string `json:"download_url"` // 原始 YAML 下载地址
	Stars       int    `json:"stars"`
	Author      string `json:"author"`
}

// SearchSkillsOnline 从 GitHub 搜索技能 YAML 文件
func SearchSkillsOnline(query string) ([]SkillMarketItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		query = "mcp+skill"
	}

	// 搜索 GitHub: topic:mcp-skill + 关键词，或搜索 skill YAML 文件
	searchURL := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s+skill+yaml+in:name,description&sort=stars&order=desc&per_page=15",
		url.QueryEscape(query),
	)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	// 如有 GITHUB_TOKEN 环境变量则使用（提高限流）
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 403 {
		// 限流 → 返回内置推荐列表
		return getBuiltinSkills(query), nil
	}

	var data struct {
		Items []struct {
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			HTMLURL     string `json:"html_url"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			DefaultBranch string `json:"default_branch"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &data); err != nil || len(data.Items) == 0 {
		return getBuiltinSkills(query), nil
	}

	var results []SkillMarketItem
	for _, item := range data.Items {
		// 生成原始 YAML 下载地址
		downloadURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/skill.yaml", item.FullName, item.DefaultBranch)
		if item.DefaultBranch == "" {
			downloadURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/main/skill.yaml", item.FullName)
		}

		results = append(results, SkillMarketItem{
			Name:        item.Name,
			Description: item.Description,
			Category:    "general",
			Source:      item.HTMLURL,
			DownloadURL: downloadURL,
			Stars:       item.Stars,
			Author:      item.Owner.Login,
		})
	}

	// 按星数排序
	sort.Slice(results, func(i, j int) bool { return results[i].Stars > results[j].Stars })
	return results, nil
}

// getBuiltinSkills 返回内置推荐技能列表（GitHub API 不可用时）
// 所有 URL 均指向 anthropics/skills 仓库中真实存在的技能
func getBuiltinSkills(query string) []SkillMarketItem {
	base := "https://raw.githubusercontent.com/anthropics/skills/main/skills"
	all := []SkillMarketItem{
		{
			Name: "xlsx", Description: "创建/读取/编辑 Excel 电子表格（.xlsx/.csv/.tsv），支持公式、图表和数据清洗",
			Category: "document", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/xlsx/SKILL.md",
		},
		{
			Name: "pdf", Description: "PDF 文件处理：读取/提取/合并/拆分/旋转/水印/OCR 识别",
			Category: "document", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/pdf/SKILL.md",
		},
		{
			Name: "docx", Description: "Word 文档创建与编辑：格式化、目录、页眉页脚、批注、模板",
			Category: "document", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/docx/SKILL.md",
		},
		{
			Name: "pptx", Description: "PowerPoint 演示文稿：创建幻灯片/演讲者备注/模板/图表",
			Category: "document", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/pptx/SKILL.md",
		},
		{
			Name: "frontend-design", Description: "创建高质量前端界面：React/Tailwind/shadcn/ui，支持 Dashboard/Landing Page",
			Category: "dev", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/frontend-design/SKILL.md",
		},
		{
			Name: "webapp-testing", Description: "使用 Playwright 测试 Web 应用：截图/交互/调试/日志",
			Category: "dev", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/webapp-testing/SKILL.md",
		},
		{
			Name: "mcp-builder", Description: "构建 MCP 服务器：Python (FastMCP) 或 Node/TypeScript (MCP SDK)",
			Category: "dev", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/mcp-builder/SKILL.md",
		},
		{
			Name: "skill-creator", Description: "创建和管理技能：编写/修改/评估/优化 Skills",
			Category: "dev", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/skill-creator/SKILL.md",
		},
		{
			Name: "web-artifacts-builder", Description: "构建复杂 claude.ai HTML Artifacts：React/Tailwind/shadcn 多组件应用",
			Category: "dev", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/web-artifacts-builder/SKILL.md",
		},
		{
			Name: "canvas-design", Description: "创建视觉设计作品：海报/PNG/PDF，使用设计哲学驱动",
			Category: "design", Source: "https://github.com/anthropics/skills",
			DownloadURL: base + "/canvas-design/SKILL.md",
		},
	}
	query = strings.ToLower(query)
	if query == "" || query == "mcp+skill" {
		return all
	}
	var filtered []SkillMarketItem
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.Description), query) ||
			strings.Contains(strings.ToLower(s.Category), query) {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		return all
	}
	return filtered
}

// DownloadAndInstallSkill 下载技能 YAML 并安装到 skills 目录
func DownloadAndInstallSkill(downloadURL, skillsDir string) (string, error) {
	// 1. 下载 YAML
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	yamlData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	content := string(yamlData)

	// 2. 检测格式并提取技能名称
	var name string
	var ext string

	if strings.HasPrefix(strings.TrimSpace(content), "# ") || strings.Contains(content, "\n## ") {
		// SKILL.md 格式（Markdown）
		name = extractMDTitle(content)
		ext = ".md"
	} else if strings.Contains(content, "name:") && strings.Contains(content, "steps:") {
		// skill.yaml 格式（YAML）
		name = extractYAMLField(content, "name")
		ext = ".yaml"
	} else {
		return "", fmt.Errorf("文件格式不正确：不支持的文件类型（需为 SKILL.md 或 skill.yaml）")
	}

	if name == "" {
		return "", fmt.Errorf("无法从文件中提取技能名称")
	}

	// 3. 确保目录存在
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return "", fmt.Errorf("创建 skills 目录失败: %w", err)
	}

	// 4. 写入文件（保留原始扩展名）
	filePath := filepath.Join(skillsDir, name+ext)
	if err := os.WriteFile(filePath, yamlData, 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	return fmt.Sprintf("✅ 技能 '%s' 已安装\n📄 %s", name, filePath), nil
}

// extractYAMLField 从 YAML 文本提取字段值
func extractYAMLField(yaml, key string) string {
	for _, line := range strings.Split(yaml, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

// extractMDTitle 从 Markdown 文本提取标题（第一个 # 开头的内容）
func extractMDTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// ── MCP 热门推荐 ──

// PopularMCPServer 热门 MCP 服务器
type PopularMCPServer struct {
	Name        string `json:"name"`
	Package     string `json:"package"`
	Description string `json:"description"`
	Category    string `json:"category"`
	NpmURL      string `json:"npm_url"`
}

// GetPopularMCPServers 返回热门 MCP 服务器推荐列表
func GetPopularMCPServers() []PopularMCPServer {
	return []PopularMCPServer{
		{Package: "@modelcontextprotocol/server-filesystem", Name: "filesystem", Category: "文件", Description: "文件系统访问：读/写/搜索/列表，支持安全边界限制", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-filesystem"},
		{Package: "@modelcontextprotocol/server-github", Name: "github", Category: "开发", Description: "GitHub API 集成：仓库管理/PR/Issue/搜索", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-github"},
		{Package: "@modelcontextprotocol/server-postgres", Name: "postgres", Category: "数据库", Description: "PostgreSQL 数据库：执行查询/查看表结构/数据操作", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-postgres"},
		{Package: "@modelcontextprotocol/server-puppeteer", Name: "puppeteer", Category: "浏览器", Description: "浏览器自动化：网页截图/爬取/表单填写/测试", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-puppeteer"},
		{Package: "@modelcontextprotocol/server-brave-search", Name: "brave-search", Category: "搜索", Description: "Brave 搜索引擎集成：网页搜索/新闻/图片", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-brave-search"},
		{Package: "@modelcontextprotocol/server-slack", Name: "slack", Category: "通讯", Description: "Slack 集成：发送消息/频道管理/文件上传", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-slack"},
		{Package: "@modelcontextprotocol/server-sentry", Name: "sentry", Category: "监控", Description: "Sentry 错误追踪：查看 Issues/Events/项目统计", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-sentry"},
		{Package: "@modelcontextprotocol/server-memory", Name: "memory", Category: "AI", Description: "持久化记忆存储：为 AI 提供跨会话记忆能力", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-memory"},
		{Package: "@modelcontextprotocol/server-sequential-thinking", Name: "sequential-thinking", Category: "AI", Description: "增强推理：逐步思考链，支持分支和修正", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-sequential-thinking"},
		{Package: "@modelcontextprotocol/server-everything", Name: "everything", Category: "测试", Description: "全功能参考服务器，包含所有 MCP 特性示例", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-everything"},
		{Package: "@modelcontextprotocol/server-sqlite", Name: "sqlite", Category: "数据库", Description: "SQLite 数据库操作：查询/建表/CRUD", NpmURL: "https://npmjs.com/package/@modelcontextprotocol/server-sqlite"},
		{Package: "@anthropic/mcp-server-google-maps", Name: "google-maps", Category: "地图", Description: "Google Maps API：地理编码/路线规划/地点搜索", NpmURL: "https://npmjs.com/package/@anthropic/mcp-server-google-maps"},
	}
}
