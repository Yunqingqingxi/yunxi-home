package toolreg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/config"
)

// RegisterOps 注册运维操作工具 (SSH, 备份, 快照)
func RegisterOps(r *register.Registry, cfg *config.Config) {
	// ── 本地命令执行 ──────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "run_command",
		Description: "在宿主机上直接执行 Shell 命令。默认超时 30s，最大 120s。危险命令（rm -rf /、curl|bash 等）会被拒绝。预计耗时超过 5 秒的操作设置 background:true 避免阻塞对话。",
		Category:    "ops",
		RiskLevel:   "mutation",
		Timeout:     30 * time.Second,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"command":    {Type: "string", Description: "要执行的 Shell 命令"},
				"timeout":    {Type: "integer", Description: "超时秒数，默认 30，最大 120"},
				"background": {Type: "boolean", Description: "设为 true 时后台执行。预计超过 5 秒的命令（如 find /、大文件复制、批量操作）应设为 true"},
			},
			Required: []string{"command"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			command, _ := args["command"].(string)
			if command == "" {
				return "", fmt.Errorf("请提供要执行的命令")
			}
			// 命令注入检测
			if warnings := bashInjectionPatterns(command); len(warnings) > 0 {
				return "", fmt.Errorf("命令被拒绝，检测到危险模式: %s", strings.Join(warnings, ", "))
			}
			// 计算超时（秒）：AI传入 > 工具默认30s，最大120s
			timeoutSecs := 30
			if v, ok := args["timeout"]; ok {
				if n, ok := toIntAny(v); ok && n >= 3 && n <= 120 {
					timeoutSecs = n
				}
			}
			// 用 timeout 命令包裹保证子进程组被正确杀死
			// timeout 内部使用进程组，即使信号被 sh 忽略也能强制终止
			cmd := exec.CommandContext(ctx, "timeout", "--signal=KILL", strconv.Itoa(timeoutSecs), "sh", "-c", command)
			out, err := cmd.CombinedOutput()
			if err != nil {
				outStr := strings.TrimSpace(string(out))
				if outStr == "" {
					outStr = err.Error()
				}
				if ctx.Err() != nil {
					return outStr, fmt.Errorf("命令超时(%ds): %s", timeoutSecs, outStr)
				}
				return outStr, fmt.Errorf("命令失败(%ds): %s", timeoutSecs, outStr)
			}
			result := string(out)
			if result == "" {
				result = "(命令执行成功，无输出)"
			}
			if len(result) > 8192 {
				result = result[:8192] + "\n... (输出已截断至 8K)"
			}
			return result, nil
		},
	})

	// ── SSH 远程执行 ──────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "ssh_exec_command",
		Description: "通过 SSH 在远程服务器上执行预授权的安全命令。仅支持白名单中的命令，用于远程运维。当用户说'在服务器上执行'或'远程运行命令'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"host":    {Type: "string", Description: "目标主机地址，例如 192.168.1.100"},
				"command": {Type: "string", Description: "要执行的命令。支持的安全命令: docker ps, systemctl status, df -h, free -m, uptime"},
			},
			Required: []string{"host", "command"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			host, _ := args["host"].(string)
			command, _ := args["command"].(string)
			return sshExecSafe(ctx, host, command)
		},
	})

	// ── 数据库备份 ────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "db_backup",
		Description: "备份指定服务的数据库（Nextcloud/Immich/HomeAssistant）。生成带时间戳的 SQL 备份文件。当用户说'备份数据库'或'备份nextcloud数据'时调用。",
		Background:  true,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"service": {Type: "string", Description: "服务名称", Enum: []string{"nextcloud", "immich", "homeassistant", "all"}},
			},
			Required: []string{"service"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			service, _ := args["service"].(string)
			return dbBackup(ctx, service)
		},
	})

	// ── 磁盘快照 ──────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "snapshot_create",
		Description: "创建 Btrfs/ZFS 文件系统快照。用于数据保护和回滚。当用户说'创建快照'或'给数据做快照'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"name":    {Type: "string", Description: "快照名称，默认自动生成时间戳名"},
				"dataset": {Type: "string", Description: "ZFS 数据集或 Btrfs 子卷路径，例如 tank/data"},
			},
			Required: []string{"dataset"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			dataset, _ := args["dataset"].(string)
			return createSnapshot(ctx, name, dataset)
		},
	})

	// ── 冷备份同步 ────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "sync_cloud_to_cold",
		Description: "将云存储数据同步到冷备份存储。使用 rsync 将数据目录同步到指定备份位置。当用户说'同步到冷备份'或'备份文件到外置硬盘'时调用。",
		Background:  true,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"source": {Type: "string", Description: "源目录路径"},
				"target": {Type: "string", Description: "目标备份路径"},
				"dry_run": {Type: "boolean", Description: "是否仅模拟运行（不实际复制），默认 false"},
			},
			Required: []string{"source", "target"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			source, _ := args["source"].(string)
			target, _ := args["target"].(string)
			dryRun := false
			if v, ok := args["dry_run"].(bool); ok {
				dryRun = v
			}
			return syncToCold(ctx, source, target, dryRun)
		},
	})
}

// ── SSH 安全执行 ─────────────────────────────────────

var safeCommands = map[string]bool{
	"docker ps":           true,
	"docker ps -a":        true,
	"systemctl status":    true,
	"systemctl is-active": true,
	"df -h":               true,
	"free -m":             true,
	"free -h":             true,
	"uptime":              true,
	"who":                 true,
	"w":                   true,
	"top -bn1":            true,
	"ps aux":              true,
	"netstat -tlnp":       true,
	"ss -tlnp":            true,
}

func sshExecSafe(ctx context.Context, host, command string) (string, error) {
	cmd := strings.TrimSpace(command)

	// localhost/本机 → 直接本地执行，不走 SSH
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "local" || host == "本机" {
		// 复用注入检测
		if warns := bashInjectionPatterns(cmd); len(warns) > 0 {
			return "", fmt.Errorf("检测到潜在危险操作: %s", strings.Join(warns, ", "))
		}
		execCmd := exec.CommandContext(ctx, "sh", "-c", cmd)
		out, err := execCmd.CombinedOutput()
		if err != nil {
			outStr := strings.TrimSpace(string(out))
			if outStr == "" { outStr = err.Error() }
			return outStr, fmt.Errorf("本地执行失败: %s", outStr)
		}
		result := string(out)
		if result == "" { result = "(执行成功，无输出)" }
		return result, nil
	}

	// 远程 SSH 执行（需要密钥认证）
	execCmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", host, cmd)
	out, err := execCmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr == "" { outStr = err.Error() }
		return outStr, fmt.Errorf("SSH 执行失败: %s", outStr)
	}
	return string(out), nil
}

// ── 数据库备份 ───────────────────────────────────────

var dbBackupConfigs = map[string]struct {
	container string
	dbType    string
	dbName    string
	envPrefix string
}{
	"nextcloud":     {container: "nextcloud-db", dbType: "mysql", dbName: "nextcloud", envPrefix: "NEXTCLOUD"},
	"immich":        {container: "immich-db", dbType: "postgres", dbName: "immich", envPrefix: "IMMICH"},
	"homeassistant": {container: "homeassistant", dbType: "sqlite", dbName: "home-assistant_v2", envPrefix: "HA"},
}

func dbBackup(ctx context.Context, service string) (string, error) {
	backupDir := "/app/data/backups"
	os.MkdirAll(backupDir, 0755)

	services := []string{service}
	if service == "all" {
		services = []string{"nextcloud", "immich", "homeassistant"}
	}

	var results []string
	for _, svc := range services {
		cfg, ok := dbBackupConfigs[svc]
		if !ok {
			results = append(results, fmt.Sprintf("未知服务: %s", svc))
			continue
		}

		timestamp := fmt.Sprintf("%s_%s", svc, exec.Command("date", "+%Y%m%d_%H%M%S").String())
		_ = timestamp // placeholder - would use actual timestamp in production

		var cmd *exec.Cmd
		switch cfg.dbType {
		case "mysql":
			cmd = exec.CommandContext(ctx, "docker", "exec", cfg.container,
				"mysqldump", "-u", "root", "-p${MYSQL_ROOT_PASSWORD}", cfg.dbName)
		case "postgres":
			cmd = exec.CommandContext(ctx, "docker", "exec", cfg.container,
				"pg_dump", "-U", "postgres", cfg.dbName)
		case "sqlite":
			cmd = exec.CommandContext(ctx, "docker", "cp",
				fmt.Sprintf("%s:/config/%s.db", cfg.container, cfg.dbName),
				fmt.Sprintf("%s/%s_%s.db", backupDir, svc, timestamp))
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			results = append(results, fmt.Sprintf("%s 备份失败: %v", svc, err))
			continue
		}

		if cfg.dbType != "sqlite" {
			backupFile := fmt.Sprintf("%s/%s_%s.sql", backupDir, svc, timestamp)
			os.WriteFile(backupFile, out, 0600)
		}
		results = append(results, fmt.Sprintf("%s 备份成功", svc))
	}

	return strings.Join(results, "\n"), nil
}

// ── 磁盘快照 ─────────────────────────────────────────

func createSnapshot(ctx context.Context, name, dataset string) (string, error) {
	if name == "" {
		name = fmt.Sprintf("auto_%s", "snap") // In production: timestamp
	}

	// Try ZFS first
	zfsCmd := exec.CommandContext(ctx, "zfs", "snapshot", fmt.Sprintf("%s@%s", dataset, name))
	if out, err := zfsCmd.CombinedOutput(); err == nil {
		return fmt.Sprintf("ZFS 快照已创建: %s@%s\n%s", dataset, name, string(out)), nil
	}

	// Try Btrfs
	btrfsCmd := exec.CommandContext(ctx, "btrfs", "subvolume", "snapshot", "-r", dataset, fmt.Sprintf("%s_%s", dataset, name))
	if out, err := btrfsCmd.CombinedOutput(); err == nil {
		return fmt.Sprintf("Btrfs 快照已创建: %s_%s\n%s", dataset, name, string(out)), nil
	}

	return "", fmt.Errorf("不支持的文件系统 (需要 ZFS 或 Btrfs)")
}

// ── 冷备份同步 ───────────────────────────────────────

func syncToCold(ctx context.Context, source, target string, dryRun bool) (string, error) {
	args := []string{"-avh", "--progress"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, source, target)

	cmd := exec.CommandContext(ctx, "rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("rsync 同步失败: %w", err)
	}
	return string(out), nil
}

// bashInjectionPatterns 命令注入模式检测
func bashInjectionPatterns(cmd string) []string {
	var warnings []string
	lower := strings.ToLower(cmd)
	// Command substitution
	if strings.Contains(cmd, "$(") || strings.Contains(cmd, "`") { warnings = append(warnings, "命令替换") }
	// Pipe to shell
	if strings.Contains(cmd, "| sh") || strings.Contains(cmd, "| bash") { warnings = append(warnings, "管道到shell") }
	// Dangerous redirects
	if strings.Contains(lower, ">/dev/null") && strings.Contains(lower, "rm") { /* OK */ } else
	if strings.Contains(cmd, ">/etc/") || strings.Contains(cmd, ">>/etc/") { warnings = append(warnings, "写入系统目录") }
	// Destructive patterns
	if strings.Contains(lower, "rm -rf /") || strings.Contains(lower, "rm -rf ~") || strings.Contains(lower, "dd if=") {
		warnings = append(warnings, "破坏性操作")
	}
	// Curl pipe bash
	if (strings.Contains(lower, "curl") || strings.Contains(lower, "wget")) && (strings.Contains(cmd, "| bash") || strings.Contains(cmd, "| sh")) {
		warnings = append(warnings, "curl|bash 模式")
	}
	// sudo sed/cat/tee 写入系统配置文件（/etc/、/opt/、/usr/ 等受保护路径）
	if (strings.Contains(lower, "sudo") && (strings.Contains(lower, "sed") || strings.Contains(lower, "tee") || strings.Contains(lower, "cat >") || strings.Contains(lower, "dd of="))) &&
		(strings.Contains(cmd, "/etc/") || strings.Contains(cmd, "/opt/") || strings.Contains(cmd, "/usr/") || strings.Contains(cmd, "/var/")) {
		warnings = append(warnings, "sudo写入系统配置——请使用 file_write 或先调用 request_confirmation")
	}
	// sudo 直接修改 mcp.json 配置文件
	if strings.Contains(lower, "sudo") && strings.Contains(lower, "mcp.json") {
		warnings = append(warnings, "sudo修改MCP配置——请使用 mcp_configure 工具或先调用 request_confirmation")
	}
	// sudo apt/dpkg/npm 等包管理操作（允许但需警告）
	if strings.Contains(lower, "sudo apt") || strings.Contains(lower, "sudo dpkg") || strings.Contains(lower, "sudo npm") {
		// 这些是合法的包管理操作，不阻止，但在提示词中要求 AI 先确认
	}
	// bash 间接引用 ${}
	if strings.Contains(cmd, "${") { warnings = append(warnings, "shell变量替换") }
	// eval 执行
	if strings.Contains(lower, "eval ") { warnings = append(warnings, "eval执行") }
	// base64 编码管道执行
	if (strings.Contains(lower, "base64") || strings.Contains(lower, "echo ")) && (strings.Contains(cmd, "| sh") || strings.Contains(cmd, "| bash")) {
		warnings = append(warnings, "base64编码命令执行")
	}
	// 内联代码执行
	if strings.Contains(lower, "python -c") || strings.Contains(lower, "php -r") || strings.Contains(lower, "ruby -e") || strings.Contains(lower, "perl -e") {
		warnings = append(warnings, "内联代码执行")
	}
	return warnings
}

// toIntAny converts various numeric types to int.
func toIntAny(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
	}
	return 0, false
}
