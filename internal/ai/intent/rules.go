// Package intent provides a two-stage intent routing pipeline for tool selection.
// Stage 1: exact rule triggers (zero latency, ~70 rules)
// Stage 2: LLM-based classification (300-500ms, fallback when rules miss)
package intent

import "strings"

// IntentRule defines a single trigger rule mapping user phrases to a tool.
type IntentRule struct {
	Tool     string
	Patterns []string // case-insensitive substring matches
	Strength float64  // 0.0-1.0, must be >= 0.70 to be considered a hit
}

// defaultRules returns the initial ~70 trigger rules covering all 20+ tools.
// Each tool gets 3-5 common phrasings.
func defaultRules() []IntentRule {
	return []IntentRule{
		// ── Docker ──────────────────────────────────────────
		{Tool: "docker_list_containers", Patterns: []string{
			"docker ps", "docker containers", "查看容器", "列出容器",
			"运行中的容器", "容器列表", "docker 状态", "docker status",
		}, Strength: 0.95},
		{Tool: "docker_control_container", Patterns: []string{
			"启动容器", "停止容器", "重启容器", "docker start", "docker stop",
			"docker restart", "暂停容器", "恢复容器",
		}, Strength: 0.92},
		{Tool: "docker_get_logs", Patterns: []string{
			"docker logs", "容器日志", "查看日志",
			"docker log", "容器运行日志",
		}, Strength: 0.90},
		{Tool: "docker_compose", Patterns: []string{
			"docker-compose", "compose up", "compose down",
			"compose 部署", "编排", "docker compose",
		}, Strength: 0.90},

		// ── DNS ──────────────────────────────────────────
		{Tool: "query_dns_records", Patterns: []string{
			"DNS记录", "解析记录", "dns record", "域名解析",
			"查询解析", "dns 查询", "A记录", "CNAME记录",
			"AAAA记录", "MX记录", "NS记录", "TXT记录",
		}, Strength: 0.85},
		{Tool: "list_cloud_domains", Patterns: []string{
			"域名列表", "我的域名", "已添加的域名",
			"cloud domains", "所有域名",
		}, Strength: 0.85},
		{Tool: "list_cloud_records", Patterns: []string{
			"列出记录", "cloud records", "dns 解析列表",
			"解析记录列表",
		}, Strength: 0.83},
		{Tool: "list_domain_records", Patterns: []string{
			"domain records", "域名的记录", "域名记录",
		}, Strength: 0.83},
		{Tool: "add_cloud_dns_record", Patterns: []string{
			"添加DNS", "新增解析", "添加记录", "add record",
			"增加解析", "创建DNS", "新建解析", "添加一条",
		}, Strength: 0.88},
		{Tool: "delete_cloud_dns_record", Patterns: []string{
			"删除DNS", "移除解析", "删除记录", "delete record",
			"去掉解析", "删掉记录",
		}, Strength: 0.88},
		{Tool: "add_domain_record", Patterns: []string{
			"添加域名记录", "新增域名解析",
		}, Strength: 0.85},
		{Tool: "delete_domain_record", Patterns: []string{
			"删除域名记录", "移除域名解析",
		}, Strength: 0.85},
		{Tool: "update_domain_record", Patterns: []string{
			"更新记录", "修改解析", "修改记录", "update record",
			"编辑记录",
		}, Strength: 0.85},
		{Tool: "update_dns_record_value", Patterns: []string{
			"修改DNS值", "更新解析值", "改IP",
		}, Strength: 0.83},
		{Tool: "trigger_dns_update", Patterns: []string{
			"触发更新", "强制更新", "立即更新", "手动更新",
			"trigger update", "手动同步",
		}, Strength: 0.88},
		{Tool: "set_cloud_record_status", Patterns: []string{
			"启用记录", "禁用记录", "暂停解析", "恢复解析",
			"enable record", "disable record",
		}, Strength: 0.85},

		// ── 系统 ──────────────────────────────────────────
		{Tool: "get_server_resources", Patterns: []string{
			"服务器资源", "CPU使用", "内存使用", "系统资源",
			"磁盘使用", "资源监控", "server resources",
			"cpu usage", "memory usage", "查看资源",
			"负载", "系统负载", "系统监控",
		}, Strength: 0.88},
		{Tool: "get_system_status", Patterns: []string{
			"系统状态", "运行状态", "服务状态", "server status",
			"状态检查", "健康检查", "系统信息", "运行情况",
		}, Strength: 0.88},
		{Tool: "clear_system_memory", Patterns: []string{
			"清理内存", "释放内存", "free memory", "clear memory",
			"内存回收", "GC", "垃圾回收",
		}, Strength: 0.88},
		{Tool: "get_system_config", Patterns: []string{
			"系统配置", "当前配置", "查看配置", "server config",
			"配置信息",
		}, Strength: 0.83},
		{Tool: "get_network_info", Patterns: []string{
			"网络信息", "IP地址", "网络状态", "网卡信息",
			"network info", "公网IP", "内网IP",
		}, Strength: 0.85},
		{Tool: "read_app_log", Patterns: []string{
			"应用日志", "app log", "运行时日志", "服务器日志",
			"查看log", "系统日志",
		}, Strength: 0.83},
		{Tool: "ping_host", Patterns: []string{
			"ping", "连通性", "能不能通", "网络检测",
			"延迟测试", "测速",
		}, Strength: 0.83},
		{Tool: "list_services", Patterns: []string{
			"服务列表", "所有服务", "已安装服务", "service list",
			"systemctl list",
		}, Strength: 0.83},

		// ── 文件 ──────────────────────────────────────────
		{Tool: "file_list", Patterns: []string{
			"列出文件", "文件列表", "ls", "有什么文件",
			"查看目录", "目录下", "dir", "list files",
			"文件夹里有什么",
		}, Strength: 0.85},
		{Tool: "file_read", Patterns: []string{
			"读取文件", "查看文件", "cat", "打开文件",
			"读文件", "文件内容", "read file",
		}, Strength: 0.85},
		{Tool: "file_write", Patterns: []string{
			"写入文件", "创建文件", "写文件", "新建文件",
			"write file", "保存文件", "编辑文件", "修改文件",
		}, Strength: 0.83},
		{Tool: "file_delete", Patterns: []string{
			"删除文件", "rm", "移除文件", "delete file",
			"删掉文件", "清理文件",
		}, Strength: 0.83},
		{Tool: "file_search", Patterns: []string{
			"搜索文件", "查找文件", "find", "grep",
			"找文件", "search file", "检索文件",
		}, Strength: 0.85},
		{Tool: "file_disk_info", Patterns: []string{
			"磁盘信息", "磁盘空间", "df -h", "硬盘空间",
			"存储空间", "容量", "还有多少空间",
		}, Strength: 0.88},
		{Tool: "file_mkdir", Patterns: []string{
			"创建目录", "mkdir", "新建文件夹", "创建文件夹",
		}, Strength: 0.85},
		{Tool: "file_copy", Patterns: []string{
			"复制文件", "cp", "拷贝", "copy file",
			"备份文件",
		}, Strength: 0.83},
		{Tool: "file_move", Patterns: []string{
			"移动文件", "mv", "剪切", "move file",
			"搬移",
		}, Strength: 0.83},
		{Tool: "file_rename", Patterns: []string{
			"重命名", "改名", "rename", "改文件名",
		}, Strength: 0.83},
		{Tool: "file_download", Patterns: []string{
			"下载文件", "download", "下载",
		}, Strength: 0.80},

		// ── 运维 ──────────────────────────────────────────
		{Tool: "run_command", Patterns: []string{
			"执行命令", "运行命令", "shell", "命令行",
			"bash", "cmd", "exec",
		}, Strength: 0.78},
		{Tool: "ssh_exec_command", Patterns: []string{
			"SSH执行", "远程执行", "ssh", "远程命令",
			"远程服务器",
		}, Strength: 0.83},
		{Tool: "db_backup", Patterns: []string{
			"数据库备份", "备份数据库", "db backup",
			"备份DB",
		}, Strength: 0.88},
		{Tool: "snapshot_create", Patterns: []string{
			"创建快照", "snapshot", "系统快照", "打快照",
		}, Strength: 0.85},
		{Tool: "sync_cloud_to_cold", Patterns: []string{
			"同步到冷存储", "云端同步", "冷备", "sync cloud",
			"数据归档",
		}, Strength: 0.83},

		// ── 历史 ──────────────────────────────────────────
		{Tool: "get_update_history", Patterns: []string{
			"更新历史", "历史记录", "操作记录", "history",
			"变更记录",
		}, Strength: 0.83},
		{Tool: "clean_history", Patterns: []string{
			"清理历史", "清除记录", "clean history",
		}, Strength: 0.83},
	}
}

// MatchResult holds the outcome of a rule match.
type MatchResult struct {
	Tool     string
	Strength float64
	Pattern  string // the pattern that triggered the match
}

// matchRules runs all rules against the user message (case-insensitive).
// Returns the highest-strength match, or nil if no rule scores >= 0.70.
func matchRules(userMsg string, rules []IntentRule) *MatchResult {
	lower := strings.ToLower(userMsg)
	var best *MatchResult
	for _, r := range rules {
		for _, pat := range r.Patterns {
			if strings.Contains(lower, strings.ToLower(pat)) {
				if best == nil || r.Strength > best.Strength {
					best = &MatchResult{Tool: r.Tool, Strength: r.Strength, Pattern: pat}
				}
				break // inner loop — found match for this rule, skip remaining patterns
			}
		}
	}
	if best != nil && best.Strength < 0.70 {
		return nil
	}
	return best
}
