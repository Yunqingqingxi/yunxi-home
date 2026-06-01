package toolreg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/util/fileio"
)

// ── MCP 市场：搜索 + 安装 + mcp.json 管理 ─────────────────────

// MCPSearchResult npm 搜索结果
type MCPSearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	NpmURL      string `json:"npm_url"`
	Score       int    `json:"score"` // 模糊匹配评分
}

// SearchMCPMarket 搜索 MCP 服务器：优先精确匹配已知库，再搜 npm
func SearchMCPMarket(query string) ([]MCPSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("请提供搜索关键词")
	}
	lowerQ := strings.ToLower(query)

	// 1. 先在已知库中精确/模糊匹配
	var knownResults []MCPSearchResult
	for pkg, params := range knownMCPServers {
		shortName := pkg
		if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
			shortName = pkg[idx+1:]
		}
		shortName = strings.TrimPrefix(shortName, "server-")
		shortName = strings.TrimPrefix(shortName, "mcp-")

		score := matchScore(pkg+" "+shortName, "", lowerQ)
		if strings.EqualFold(pkg, query) || strings.EqualFold(shortName, lowerQ) {
			score = 100 // 精确匹配
		}
		if score >= 30 {
			knownResults = append(knownResults, MCPSearchResult{
				Name:        pkg,
				Version:     "latest",
				Description: fmt.Sprintf("MCP 服务器 (%d 个参数)", len(params)),
				NpmURL:      "https://npmjs.com/package/" + pkg,
				Score:       score,
			})
		}
	}
	// 也检查热门推荐
	for _, s := range GetPopularMCPServers() {
		shortName := s.Name
		if strings.EqualFold(s.Package, query) || strings.EqualFold(shortName, lowerQ) {
			// 检查是否已在结果中
			found := false
			for _, r := range knownResults {
				if r.Name == s.Package { found = true; break }
			}
			if !found {
				knownResults = append(knownResults, MCPSearchResult{
					Name:        s.Package,
					Version:     "latest",
					Description: s.Description,
					NpmURL:      s.NpmURL,
					Score:       100,
				})
			}
		}
	}

	sort.Slice(knownResults, func(i, j int) bool { return knownResults[i].Score > knownResults[j].Score })

	// 如果有精确匹配（score=100）来自已知库，直接返回
	if len(knownResults) > 0 && knownResults[0].Score >= 100 {
		if len(knownResults) > 10 { knownResults = knownResults[:10] }
		return knownResults, nil
	}

	// 2. 已知库结果 + npm 搜索结果合并
	searchTerms := []string{
		"@modelcontextprotocol/server-" + lowerQ,
		"mcp-server-" + lowerQ,
		"mcp-" + lowerQ + "-server",
	}
	seen := make(map[string]bool)
	for _, r := range knownResults {
		seen[r.Name] = true
	}
	allResults := knownResults

	for _, term := range searchTerms {
		results, err := searchNPM(term)
		if err != nil { continue }
		for _, r := range results {
			if !seen[r.Name] {
				seen[r.Name] = true
				r.Score = matchScore(r.Name, r.Description, lowerQ)
				allResults = append(allResults, r)
			}
		}
	}

	sort.Slice(allResults, func(i, j int) bool { return allResults[i].Score > allResults[j].Score })

	if len(allResults) == 0 {
		return nil, fmt.Errorf("未找到匹配。\n\n可用: filesystem github postgres puppeteer brave-search slack sentry memory sequential-thinking sqlite google-maps everything\n\n示例: /get-mcp filesystem")
	}
	if len(allResults) > 10 { allResults = allResults[:10] }
	return allResults, nil
}

// searchNPM 调用 npm registry API
func searchNPM(keyword string) ([]MCPSearchResult, error) {
	apiURL := fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=%s&size=15",
		url.QueryEscape(keyword))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("npm API 返回 %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
				Links       struct {
					Npm string `json:"npm"`
				} `json:"links"`
			} `json:"package"`
		} `json:"objects"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var results []MCPSearchResult
	for _, obj := range data.Objects {
		pkg := obj.Package
		npmURL := pkg.Links.Npm
		if npmURL == "" {
			npmURL = "https://www.npmjs.com/package/" + pkg.Name
		}
		results = append(results, MCPSearchResult{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Description: pkg.Description,
			NpmURL:      npmURL,
		})
	}
	return results, nil
}

// matchScore 计算模糊匹配分数
func matchScore(name, description, query string) int {
	score := 0
	lower := strings.ToLower(name + " " + description)
	queryParts := strings.Fields(query)

	// 名称精确包含查询
	if strings.Contains(strings.ToLower(name), query) {
		score += 50
	}

	// 每个查询词匹配
	for _, part := range queryParts {
		if strings.Contains(strings.ToLower(name), part) {
			score += 20
		}
		if strings.Contains(lower, part) {
			score += 5
		}
	}

	// server 关键字加分
	if strings.Contains(strings.ToLower(name), "server") {
		score += 5
	}

	// @modelcontextprotocol 官方包加分
	if strings.HasPrefix(name, "@modelcontextprotocol/") {
		score += 30
	}

	return score
}

// InstallMCPServer 安装 MCP 服务器并验证连接
func InstallMCPServer(packageName, mcpConfigPath string) (string, error) {
	// 1. 确保 npx 可用
	if _, err := exec.LookPath("npx"); err != nil {
		return "", fmt.Errorf("npx 未安装。请先安装 Node.js:\ncurl -fsSL https://deb.nodesource.com/setup_20.x | sudo bash -\nsudo apt install -y nodejs")
	}

	// 2. 安装包（npm install -g 确保持久化，失败则用 npx 自动下载）
	installCmd := exec.Command("npm", "install", "-g", packageName)
	installOutput, installErr := installCmd.CombinedOutput()
	if installErr != nil {
		// npm install -g 失败不阻塞，npx -y 会在运行时自动安装
	}

	// 3. 生成服务器名称（去掉 scope 前缀）
	serverName := packageName
	if idx := strings.LastIndex(packageName, "/"); idx >= 0 {
		serverName = packageName[idx+1:]
	}
	serverName = strings.TrimPrefix(serverName, "server-")
	serverName = strings.TrimPrefix(serverName, "mcp-")

	// 4. 读取/创建 mcp.json
	config := readMCPConfig(mcpConfigPath)

	// 5. 检查是否已存在
	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
		config["mcpServers"] = servers
	}

	if existing, exists := servers[serverName]; exists {
		// 已存在 → 覆盖更新（支持修复失败安装）
		if existingMap, ok := existing.(map[string]any); ok {
			if cmd, _ := existingMap["command"].(string); cmd == "npx" {
				servers[serverName] = map[string]any{
					"command": "npx",
					"args":    []string{"-y", packageName},
				}
				if err := writeMCPConfig(mcpConfigPath, config); err != nil {
					return "", fmt.Errorf("更新 mcp.json 失败: %w", err)
				}
				return fmt.Sprintf("🔄 已更新 MCP 服务器 '%s' 配置 (%s)，正在验证连接...", serverName, packageName), nil
			}
		}
		return fmt.Sprintf("MCP 服务器 '%s' 已存在配置中 (%s)。\n如需重新配置，请先手动删除 mcp.json 中的对应条目。", serverName, mcpConfigPath), nil
	}

	// 6. 添加到配置
	servers[serverName] = map[string]any{
		"command": "npx",
		"args":    []string{"-y", packageName},
	}

	// 7. 写回 mcp.json
	if err := writeMCPConfig(mcpConfigPath, config); err != nil {
		return "", fmt.Errorf("写入 mcp.json 失败: %w", err)
	}

	_ = installOutput // consumed above

	// 8. 验证连接 — 尝试启动服务器并获取工具列表
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ MCP 服务器 '%s' (%s) 已安装并配置。\n", serverName, packageName))
	sb.WriteString(fmt.Sprintf("📄 配置已写入: %s\n", mcpConfigPath))

	connResult := verifyMCPServer(serverName, packageName)
	sb.WriteString(connResult)

	return sb.String(), nil
}

// verifyMCPServer 启动 MCP 服务器验证可用性（initialize + tools/list）
func verifyMCPServer(serverName, packageName string) string {
	cmd := exec.Command("npx", "-y", packageName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Sprintf("⚠️ 无法创建 stdin: %v", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Sprintf("⚠️ 无法创建 stdout: %v", err)
	}
	// 单独捕获 stderr，不和 stdout 混淆
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("❌ 启动失败: %v\n请检查 npm 是否正常: npm --version", err)
	}

	done := make(chan string, 1)
	go func() {
		defer cmd.Process.Kill()

		// 发送 initialize
		stdin.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"yunxi-home","version":"1.0.0"}}}` + "\n"))

		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 65536), 65536)

		// 先检查 stderr（如果进程快速失败）
		time.Sleep(500 * time.Millisecond)
		if stderrBuf.Len() > 0 {
			errText := stderrBuf.String()
			if strings.Contains(errText, "权限不够") || strings.Contains(errText, "Permission denied") {
				done <- fmt.Sprintf("❌ 权限不足：%s\n请检查 node/npx 是否对服务用户可访问。", strings.TrimSpace(errText))
				return
			}
			if strings.Contains(errText, "Cannot find module") || strings.Contains(errText, "MODULE_NOT_FOUND") {
				done <- fmt.Sprintf("❌ npm 模块缺失：%s\n请运行: npm install -g %s", strings.TrimSpace(errText), packageName)
				return
			}
			if strings.Contains(errText, "Error") || strings.Contains(errText, "error") {
				done <- fmt.Sprintf("❌ 启动错误：%s", truncateStr(strings.TrimSpace(errText), 300))
				return
			}
		}

		if !scanner.Scan() {
			stderrText := strings.TrimSpace(stderrBuf.String())
			if stderrText != "" {
				done <- fmt.Sprintf("❌ 服务器启动失败: %s", truncateStr(stderrText, 300))
			} else {
				done <- fmt.Sprintf("❌ 服务器无响应。\n手动测试: npx -y %s", packageName)
			}
			return
		}

		var initResp map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &initResp); err != nil {
			raw := string(scanner.Bytes())
			// 检查是否是 stderr 混入（非 JSON 文本）
			if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
				done <- fmt.Sprintf("❌ 非 JSON 响应（进程可能崩溃）:\n%s\nStderr: %s",
					truncateStr(raw, 200), truncateStr(strings.TrimSpace(stderrBuf.String()), 200))
				return
			}
			done <- fmt.Sprintf("❌ JSON 解析失败: %v\n数据: %s", err, truncateStr(raw, 200))
			return
		}
		_ = initResp

		// 发送 initialized 通知
		stdin.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"))

		// 请求 tools/list
		stdin.Write([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"))

		var toolsCount int
		if scanner.Scan() {
			var listResp map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &listResp); err == nil {
				if result, ok := listResp["result"].(map[string]any); ok {
					if tools, ok := result["tools"].([]any); ok {
						toolsCount = len(tools)
					}
				}
			}
		}

		if toolsCount > 0 {
			done <- fmt.Sprintf("✅ 连接验证成功！发现 %d 个工具，已自动重载。", toolsCount)
		} else {
			done <- fmt.Sprintf("⚠️ 服务器已启动但未返回工具。\n手动验证: npx -y %s", packageName)
		}
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		return fmt.Sprintf("⚠️ 连接超时（15s）。\n手动测试: npx -y %s", packageName)
	}
}

// truncateStr 截断字符串
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// readMCPConfig 读取 mcp.json（带日志）
func readMCPConfig(path string) map[string]any {
	var config map[string]any
	if err := fileio.ReadJSON(path, &config); err != nil {
		slog.Warn("读取 mcp.json 失败，返回空配置", "path", path, "error", err)
		return map[string]any{}
	}
	return config
}

// writeMCPConfig 写入 mcp.json（原子写入）
func writeMCPConfig(path string, config map[string]any) error {
	return fileio.WriteJSON(path, config)
}

// FormatMCPSearchResults 格式化搜索结果为可读文本
func FormatMCPSearchResults(results []MCPSearchResult, query string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 搜索 '%s' — 找到 %d 个 MCP 服务器:\n\n", query, len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("  %d. **%s** `v%s` (匹配度: %d%%)\n     %s\n     %s\n\n",
			i+1, r.Name, r.Version, r.Score, r.Description, r.NpmURL))
	}
	sb.WriteString(fmt.Sprintf("──\n安装: /get-mcp <完整包名>\n例如: /get-mcp %s", results[0].Name))
	return sb.String()
}

// ── MCP 参数检测 ──────────────────────────────────────────

// RequiredParam MCP 服务器需要的配置参数
type RequiredParam struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
}

// MCPInstallResult 安装结果（含参数需求）
type MCPInstallResult struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	ServerName string          `json:"server_name"`
	Package    string          `json:"package"`
	ToolCount  int             `json:"tool_count"`
	NeedParams []RequiredParam `json:"need_params,omitempty"`
}

// 已知 MCP 服务器的参数声明
var knownMCPServers = map[string][]RequiredParam{
	"@modelcontextprotocol/server-github":        {{Name: "GITHUB_PERSONAL_ACCESS_TOKEN", Label: "GitHub Token", Description: "Settings → Developer settings → Personal access tokens", Required: true}},
	"@modelcontextprotocol/server-postgres":      {{Name: "DATABASE_URL", Label: "数据库连接", Description: "如 postgresql://user:pass@localhost:5432/db", Required: true}},
	"@modelcontextprotocol/server-brave-search":  {{Name: "BRAVE_API_KEY", Label: "Brave API Key", Description: "https://brave.com/search/api/", Required: true}},
	"@modelcontextprotocol/server-slack":         {{Name: "SLACK_BOT_TOKEN", Label: "Slack Bot Token", Description: "Bot User OAuth Token (xoxb-...)", Required: true}},
	"@modelcontextprotocol/server-sentry":        {{Name: "SENTRY_TOKEN", Label: "Sentry Auth Token", Description: "Settings → Account → API → Auth Tokens", Required: true}},
	"@anthropic/mcp-server-google-maps":          {{Name: "GOOGLE_MAPS_API_KEY", Label: "Google Maps API Key", Description: "Google Cloud Maps API 密钥", Required: true}},
	"@modelcontextprotocol/server-memory":        {{Name: "MEMORY_FILE_PATH", Label: "存储路径", Description: "记忆文件存储路径", Default: "/opt/yunxi-home/data/mcp-memory.json", Required: false}},
	"@modelcontextprotocol/server-filesystem":    {{Name: "ALLOWED_DIRECTORIES", Label: "允许的目录", Description: "允许访问的目录，多个用 : 分隔", Default: "/opt/yunxi-home/data/yunxiFiles", Required: true}},
	"@modelcontextprotocol/server-sqlite":        {{Name: "SQLITE_DB_PATH", Label: "数据库路径", Description: "SQLite 文件路径", Default: "/opt/yunxi-home/data/mcp-sqlite.db", Required: false}},
	"@modelcontextprotocol/server-puppeteer":     {{Name: "PUPPETEER_HEADLESS", Label: "无头模式", Description: "是否无头运行", Default: "true", Required: false}},
}

// DetectRequiredParams 检测已知 MCP 服务器需要的参数
func DetectRequiredParams(packageName string) ([]RequiredParam, bool) {
	if params, ok := knownMCPServers[packageName]; ok {
		return params, true
	}
	for key, params := range knownMCPServers {
		if strings.Contains(key, packageName) || strings.Contains(packageName, key) {
			return params, true
		}
	}
	return nil, false
}

// InstallMCPWithParams 安装 MCP 并写入环境变量到 mcp.json
func InstallMCPWithParams(packageName, mcpConfigPath string, envVars map[string]string) (*MCPInstallResult, error) {
	result := &MCPInstallResult{Package: packageName}

	serverName := packageName
	if idx := strings.LastIndex(packageName, "/"); idx >= 0 {
		serverName = packageName[idx+1:]
	}
	serverName = strings.TrimPrefix(serverName, "server-")
	serverName = strings.TrimPrefix(serverName, "mcp-")
	result.ServerName = serverName

	config := readMCPConfig(mcpConfigPath)
	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
		config["mcpServers"] = servers
	}

	serverCfg := map[string]any{
		"command": "npx",
		"args":    []string{"-y", packageName},
	}
	if len(envVars) > 0 {
		serverCfg["env"] = envVars
	}
	servers[serverName] = serverCfg

	if err := writeMCPConfig(mcpConfigPath, config); err != nil {
		return nil, fmt.Errorf("写入 mcp.json 失败: %w", err)
	}

	result.Message = fmt.Sprintf("✅ MCP 服务器 '%s' 已安装并配置", serverName)
	if len(envVars) > 0 {
		keys := make([]string, 0, len(envVars))
		for k := range envVars { keys = append(keys, k) }
		result.Message += fmt.Sprintf("\n🔑 环境变量: %s", strings.Join(keys, ", "))
	}
	result.Message += fmt.Sprintf("\n📄 配置已写入: %s\n请执行 /reload-mcp 重载工具。", mcpConfigPath)
	result.Success = true
	return result, nil
}
