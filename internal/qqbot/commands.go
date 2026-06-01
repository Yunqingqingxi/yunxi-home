package qqbot

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/scheduler"
)

func (b *Bot) RegisterCommands(
	domainRepo database.DomainRepository,
	historyRepo database.HistoryRepository,
	sched *scheduler.Scheduler,
) {
	// /help
	b.RegisterCommand("/help", func(ctx context.Context, args []string) string {
		return "[Yunxi Home 指令]\n" +
			"/status - 系统运行状态\n" +
			"/trigger - 触发 DNS 更新\n" +
			"/domains - 域名记录列表\n" +
			"/get-mcp <关键词> - 搜索安装 MCP 工具\n" +
			"/add <域名> <RR> <A|AAAA> - 添加记录\n" +
			"/delete <ID> - 删除记录\n" +
			"/enable <ID> - 启用记录\n" +
			"/disable <ID> - 停用记录\n" +
			"/history [条数] - 更新历史\n" +
			"/clear - 清除对话记录\n" +
			"/compact - 压缩对话上下文\n" +
			"/gc - 清理系统内存\n" +
			"/list-skills - 列出可用技能\n" +
			"/skills-create <描述> - 用 AI 创建新技能\n" +
			"/reload-skills - 热重载技能\n" +
			"/reload-mcp - 热重载 MCP 工具\n" +
			"/help - 显示此帮助\n" +
			"\n非指令消息将由 AI 助手处理"
	})

	// /clear - 清除对话记录
	b.RegisterCommand("/clear", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		userID, _ := ctx.Value("user_id").(string)
		if userID == "" {
			return "无法识别用户会话"
		}
		sessionID := "qqbot_" + userID
		b.aiService.ClearSession(sessionID)
		return "对话记录已清除"
	})

	// /compact - 压缩对话上下文
	b.RegisterCommand("/compact", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		userID, _ := ctx.Value("user_id").(string)
		if userID == "" {
			return "无法识别用户会话"
		}
		sessionID := "qqbot_" + userID
		return b.aiService.CompactSession(sessionID)
	})

	// /status
	b.RegisterCommand("/status", func(ctx context.Context, args []string) string {
		status, err := sched.GetStatus(ctx)
		if err != nil {
			return fmt.Sprintf("获取状态失败: %v", err)
		}
		running := "已停止"
		if v, ok := status["running"].(bool); ok && v {
			running = "运行中"
		}
		return fmt.Sprintf(
			"[系统状态]\n调度器: %s | Goroutines: %d\n域名记录: %v | 定时任务: %v\n通知渠道: %v | Go版本: %s",
			running,
			runtime.NumGoroutine(),
			status["total"],
			status["cron_entries"],
			status["notifiers"],
			runtime.Version(),
		)
	})

	// /trigger
	b.RegisterCommand("/trigger", func(ctx context.Context, args []string) string {
		if err := sched.TriggerUpdate(ctx); err != nil {
			return "触发失败: " + err.Error()
		}
		return "已触发 DNS 更新检测"
	})

	// /domains
	b.RegisterCommand("/domains", func(ctx context.Context, args []string) string {
		records, err := domainRepo.List(ctx)
		if err != nil {
			return "获取列表失败: " + err.Error()
		}
		if len(records) == 0 {
			return "暂无域名记录，使用 /add 添加"
		}
		var sb strings.Builder
		sb.WriteString("[域名记录]\n")
		for _, r := range records {
			status := "停用"
			if r.Enabled {
				status = "启用"
			}
			domain := r.Domain
			if r.RR != "@" {
				domain = r.RR + "." + r.Domain
			}
			val := r.Value
			if val == "" {
				val = "-"
			}
			fmt.Fprintf(&sb, "[%d] %s | %s | %s | %s\n", r.ID, domain, r.Type, status, val)
		}
		return sb.String()
	})

	// /add <domain> <rr> <type>
	b.RegisterCommand("/add", func(ctx context.Context, args []string) string {
		if len(args) < 3 {
			return "用法: /add <域名> <RR> <A|AAAA>\n例如: /add example.com @ AAAA"
		}
		domain := args[0]
		rr := args[1]
		recType := strings.ToUpper(args[2])
		if recType != "A" && recType != "AAAA" {
			return "类型必须是 A 或 AAAA"
		}
		rec := &models.DomainRecord{
			Domain:   domain,
			RR:       rr,
			Type:     recType,
			TTL:      600,
			CronExpr: "0 */5 * * * *",
			Enabled:  true,
		}
		id, err := domainRepo.Create(ctx, rec)
		if err != nil {
			return "添加失败: " + err.Error()
		}
		rec.ID = id
		if sched != nil {
			sched.RegisterRecord(*rec)
		}
		return fmt.Sprintf("已添加: %s (%s) ID=%d", domain, recType, id)
	})

	// /delete <id>
	b.RegisterCommand("/delete", func(ctx context.Context, args []string) string {
		if len(args) < 1 {
			return "用法: /delete <记录ID>"
		}
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return "无效的 ID"
		}
		if err := domainRepo.Delete(ctx, id); err != nil {
			return "删除失败: " + err.Error()
		}
		if sched != nil {
			sched.UnregisterRecord(id)
		}
		return fmt.Sprintf("已删除记录 ID=%d", id)
	})

	// /enable <id>
	b.RegisterCommand("/enable", func(ctx context.Context, args []string) string {
		if len(args) < 1 {
			return "用法: /enable <记录ID>"
		}
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return "无效的 ID"
		}
		rec, err := domainRepo.GetByID(ctx, id)
		if err != nil {
			return "记录不存在: " + err.Error()
		}
		rec.Enabled = true
		if err := domainRepo.Update(ctx, rec); err != nil {
			return "启用失败: " + err.Error()
		}
		if sched != nil {
			sched.RegisterRecord(*rec)
		}
		return fmt.Sprintf("已启用: %s", rec.Domain)
	})

	// /disable <id>
	b.RegisterCommand("/disable", func(ctx context.Context, args []string) string {
		if len(args) < 1 {
			return "用法: /disable <记录ID>"
		}
		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return "无效的 ID"
		}
		rec, err := domainRepo.GetByID(ctx, id)
		if err != nil {
			return "记录不存在: " + err.Error()
		}
		rec.Enabled = false
		if err := domainRepo.Update(ctx, rec); err != nil {
			return "停用失败: " + err.Error()
		}
		if sched != nil {
			sched.UnregisterRecord(id)
		}
		return fmt.Sprintf("已停用: %s", rec.Domain)
	})

	// /history [n]
	b.RegisterCommand("/history", func(ctx context.Context, args []string) string {
		limit := 5
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &limit)
		}
		if limit > 20 {
			limit = 20
		}
		if limit < 1 {
			limit = 5
		}
		result, err := historyRepo.List(ctx, database.ListParams{Page: 1, Size: limit})
		if err != nil {
			return "获取历史失败: " + err.Error()
		}
		if len(result.Records) == 0 {
			return "暂无更新历史"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[最近 %d 条更新]\n", len(result.Records)))
		for _, r := range result.Records {
			icon := "OK"
			if r.Status == "failed" {
				icon = "FAIL"
			}
			fmt.Fprintf(&sb, "[%s] %s\n  %s → %s\n", icon, r.Domain, r.OldIP, r.NewIP)
		}
		return sb.String()
	})

	// /gc - 触发内存清理
	b.RegisterCommand("/gc", func(ctx context.Context, args []string) string {
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		runtime.GC()
		runtime.ReadMemStats(&m2)
		freed := int64(m1.HeapInuse) - int64(m2.HeapInuse)
		if freed < 0 {
			freed = 0
		}
		return fmt.Sprintf("GC 完成，释放 %.1f MB", float64(freed)/1024/1024)
	})

	// /skills-create <描述> - 用 AI 创建新技能
	b.RegisterCommand("/skills-create", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		desc := strings.Join(args, " ")
		if desc == "" {
			return "用法: /skills-create <技能描述>\n例如: /skills-create 一个检查磁盘空间的技能"
		}
		result, err := b.aiService.CreateSkill(ctx, desc)
		if err != nil {
			return "创建技能失败: " + err.Error()
		}
		return result
	})

	// /list-skills - 列出可用技能
	b.RegisterCommand("/list-skills", func(ctx context.Context, args []string) string {
		if b.skillRunner == nil {
			return "技能系统未启用"
		}
		skills := b.skillRunner.ListSkills()
		if len(skills) == 0 {
			return "没有可用技能"
		}
		var sb strings.Builder
		sb.WriteString("[可用技能]\n")
		for name, desc := range skills {
			fmt.Fprintf(&sb, "/%s - %s\n", name, desc)
		}
		sb.WriteString("\n发送 /技能名 来执行")
		return sb.String()
	})

	// /reload-skills - 热重载技能
	b.RegisterCommand("/reload-skills", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		if err := b.aiService.ReloadSkills(); err != nil {
			return "技能重载失败: " + err.Error()
		}
		// 重新注册技能指令
		b.SetSkillRunner(b.skillRunner)
		return "技能已热重载"
	})

	// /reload-mcp - 热重载 MCP 工具
	b.RegisterCommand("/reload-mcp", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		if err := b.aiService.ReloadMCP(); err != nil {
			return "MCP 重载失败: " + err.Error()
		}
		return "MCP 工具已热重载"
	})

	// /get-mcp <keyword> - 联网搜索并安装 MCP 服务器
	b.RegisterCommand("/get-mcp", func(ctx context.Context, args []string) string {
		if b.aiService == nil {
			return "AI 服务未启用"
		}
		query := strings.Join(args, " ")
		return b.aiService.GetMCPServer(ctx, query)
	})
}
