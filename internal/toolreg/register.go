// Package toolreg 将后端模块包装为 AI 工具，注册到 Registry。
// 通过此包将现有模块方法包装为 AI 可调用的 Tool，不修改原有代码。
package toolreg

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/dns"
	"github.com/Yunqingqingxi/yunxi-home/internal/config"
	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
	"github.com/Yunqingqingxi/yunxi-home/internal/scheduler"
)

// RegisterAll 注册所有后端功能为 AI 工具
func RegisterAll(r *register.Registry, dnsProvider dns.Provider, domainRepo database.DomainRepository,
	historyRepo database.HistoryRepository, sched *scheduler.Scheduler, cfg *config.Config) {

	// ── DNS 域名管理 ────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "list_cloud_domains",
		Description: "查询阿里云 DNS 账号下的所有域名列表。返回 {total, domains[{DomainId, DomainName, RecordCount}]}。当用户问'有哪些域名'或'查看云端域名'时调用。获取域名后可用 list_cloud_records 查看具体解析。",
		IsConcurrencySafe: true,
		Category:    "dns",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"keyword": {Type: "string", Description: "域名关键字搜索，可模糊匹配"},
			},
		},
		Examples: []base.ToolExample{
			{Description: "列出所有域名", Args: map[string]any{}},
			{Description: "搜索包含 example 的域名", Args: map[string]any{"keyword": "example"}},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			keyword, _ := args["keyword"].(string)
		result, err := dnsProvider.ListDomains(ctx, keyword, 1, 50)
			if err != nil {
				return "", fmt.Errorf("获取云端域名列表失败: %w", err)
			}
			return ToJSON(map[string]any{
				"total":   result.TotalCount,
				"domains": result.Domains,
			})
		},
	})

	r.Register(&base.ToolDef{
		Name:        "query_dns_records",
		Description: "查询指定域名的 DNS 解析记录，返回 {RecordId, RR, Type, Value, TTL, Status}。支持按主机记录和类型过滤。当用户问'blog.example.com 解析到哪'时调用。修改记录前应先用此工具查询当前值。",
		IsConcurrencySafe: true,
		Category:    "dns",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"domain_name": {Type: "string", Description: "域名，例如 example.com"},
				"rr":          {Type: "string", Description: "主机记录，例如 www 或 @ 表示主域名"},
				"record_type": {Type: "string", Description: "记录类型", Enum: []string{"A", "AAAA", "CNAME", "MX", "TXT"}},
			},
			Required: []string{"domain_name"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			domainName, _ := args["domain_name"].(string)
			rr, _ := args["rr"].(string)
			recordType, _ := args["record_type"].(string)

record, err := dnsProvider.FindRecord(ctx, domainName, rr, recordType)
			if err != nil {
				return "", fmt.Errorf("查询 DNS 记录失败: %w", err)
			}
			if record == nil {
				return "未找到匹配的 DNS 记录", nil
			}
			return ToJSON(record)
		},
	})

	// ── 本地域名记录管理 ──────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "list_domain_records",
		Description: "列出系统中已配置的所有动态 DNS 域名记录（本地数据库）。当用户问'配置了哪些记录'或'查看监控的域名'时调用。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			records, err := domainRepo.List(ctx)
			if err != nil {
				return "", fmt.Errorf("获取域名记录列表失败: %w", err)
			}
			if records == nil {
				records = []models.DomainRecord{}
			}
			return ToJSON(records)
		},
	})

	r.Register(&base.ToolDef{
		Name:        "add_domain_record",
		Description: "添加一条动态 DNS 监控记录。当用户说'添加一个域名'或'新增监控记录'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"domain":    {Type: "string", Description: "域名，例如 example.com"},
				"rr":        {Type: "string", Description: "主机记录，例如 www 或 @（主域名）"},
				"type":      {Type: "string", Description: "记录类型", Enum: []string{"A", "AAAA"}},
				"ttl":       {Type: "integer", Description: "TTL 秒数，默认 600"},
				"cron_expr": {Type: "string", Description: "Cron 表达式，默认 '0 */5 * * * *'（每5分钟）"},
				"enabled":   {Type: "boolean", Description: "是否启用，默认 true"},
			},
			Required: []string{"domain", "rr", "type"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			domain, _ := args["domain"].(string)
			rr, _ := args["rr"].(string)
			recType, _ := args["type"].(string)
			ttl := GetInt(args, "ttl", 600)
			cronExpr, _ := args["cron_expr"].(string)
			enabled := GetBool(args, "enabled", true)

			if recType != "A" && recType != "AAAA" {
				return "", fmt.Errorf("记录类型必须是 A 或 AAAA")
			}
			if cronExpr == "" {
				cronExpr = "0 */5 * * * *"
			}

			rec := &models.DomainRecord{
				Domain:   domain,
				RR:       rr,
				Type:     recType,
				TTL:      ttl,
				CronExpr: cronExpr,
				Enabled:  enabled,
			}

			id, err := domainRepo.Create(ctx, rec)
			if err != nil {
				return "", fmt.Errorf("创建记录失败: %w", err)
			}
			rec.ID = id

			if sched != nil {
				sched.RegisterRecord(*rec)
			}

			return ToJSON(map[string]any{
				"message": "记录已创建",
				"id":      id,
			})
		},
	})

	r.Register(&base.ToolDef{
		Name:        "delete_domain_record",
		Description: "删除一条动态 DNS 监控记录。当用户说'删除记录'或'取消监控某个域名'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"id": {Type: "integer", Description: "记录的 ID"},
			},
			Required: []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			id := int64(GetInt(args, "id", 0))
			if id <= 0 {
				return "", fmt.Errorf("请提供有效的记录 ID")
			}

			if err := domainRepo.Delete(ctx, id); err != nil {
				return "", fmt.Errorf("删除记录失败: %w", err)
			}
			if sched != nil {
				sched.UnregisterRecord(id)
			}
			return ToJSON(map[string]any{"message": "记录已删除", "id": id})
		},
	})

	r.Register(&base.ToolDef{
		Name:        "update_domain_record",
		Description: "更新一条动态 DNS 监控记录的配置。当用户说'修改记录'或'更新配置'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"id":        {Type: "integer", Description: "记录的 ID"},
				"domain":    {Type: "string", Description: "新的域名"},
				"rr":        {Type: "string", Description: "新的主机记录"},
				"type":      {Type: "string", Description: "新的记录类型", Enum: []string{"A", "AAAA"}},
				"ttl":       {Type: "integer", Description: "新的 TTL 值"},
				"cron_expr": {Type: "string", Description: "新的 Cron 表达式"},
				"enabled":   {Type: "boolean", Description: "是否启用"},
			},
			Required: []string{"id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			id := int64(GetInt(args, "id", 0))
			rec, err := domainRepo.GetByID(ctx, id)
			if err != nil {
				return "", fmt.Errorf("获取记录失败: %w", err)
			}

			if v, ok := args["domain"].(string); ok && v != "" {
				rec.Domain = v
			}
			if v, ok := args["rr"].(string); ok && v != "" {
				rec.RR = v
			}
			if v, ok := args["type"].(string); ok && v != "" {
				rec.Type = v
			}
			if v, ok := args["ttl"].(float64); ok && v > 0 {
				rec.TTL = int(v)
			}
			if v, ok := args["cron_expr"].(string); ok && v != "" {
				rec.CronExpr = v
			}
			if v, ok := args["enabled"].(bool); ok {
				rec.Enabled = v
			}

			if err := domainRepo.Update(ctx, rec); err != nil {
				return "", fmt.Errorf("更新记录失败: %w", err)
			}
			if sched != nil {
				sched.RegisterRecord(*rec)
			}
			return ToJSON(map[string]any{"message": "记录已更新"})
		},
	})

	// ── 调度器控制 ──────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "trigger_dns_update",
		Description: "手动触发一次 DNS 更新检测。当用户说'立即更新'或'检查IP变化'时调用。",
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			if sched == nil {
				return "调度器未初始化", nil
			}
			if err := sched.TriggerUpdate(ctx); err != nil {
				return "", fmt.Errorf("触发更新失败: %w", err)
			}
			return "DNS 更新任务已触发", nil
		},
	})

	r.Register(&base.ToolDef{
		Name:        "get_system_status",
		Description: "获取系统运行状态和资源使用情况。当用户问'系统状态'、'运行情况'、'CPU/内存使用'时调用。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			if sched == nil {
				return "调度器未初始化", nil
			}
			status, err := sched.GetStatus(ctx)
			if err != nil {
				return "", fmt.Errorf("获取状态失败: %w", err)
			}
			return ToJSON(map[string]any{
				"scheduler":   status,
				"goroutines":  runtime.NumGoroutine(),
				"go_version":  runtime.Version(),
				"cpu_cores":   runtime.NumCPU(),
			})
		},
	})

	r.Register(&base.ToolDef{
		Name:        "clear_system_memory",
		Description: "清理系统内存（运行 GC 并释放内核缓存）。用户说'清理内存'或'释放内存'时调用。",
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for i := 0; i < 3; i++ {
				runtime.GC()
				debug.FreeOSMemory()
				runtime.ReadMemStats(&after)
				if after.HeapInuse >= before.HeapInuse && i > 0 {
					break
				}
			}
			freedMB := float64(before.HeapInuse-after.HeapInuse) / 1024 / 1024
			return fmt.Sprintf("内存已清理，释放了 %.1f MB 堆内存", freedMB), nil
		},
	})

	// ── 更新历史 ────────────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "get_update_history",
		Description: "查询 DNS 更新的历史记录。当用户问'更新历史'或'最近的变更'时调用。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"domain": {Type: "string", Description: "按域名过滤，可选"},
				"page":   {Type: "integer", Description: "页码，从 1 开始，默认 1"},
				"size":   {Type: "integer", Description: "每页数量，默认 10"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			domain, _ := args["domain"].(string)
			page := GetInt(args, "page", 1)
			size := GetInt(args, "size", 10)

			result, err := historyRepo.List(ctx, database.ListParams{
				Domain: domain,
				Page:   page,
				Size:   size,
			})
			if err != nil {
				return "", fmt.Errorf("查询历史失败: %w", err)
			}
			return ToJSON(result)
		},
	})

	r.Register(&base.ToolDef{
		Name:        "clean_history",
		Description: "清理旧的更新历史记录。用户说'清理历史'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"days": {Type: "integer", Description: "保留最近多少天的记录，默认 30"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			days := GetInt(args, "days", 30)
			n, err := historyRepo.CleanOld(ctx, days)
			if err != nil {
				return "", fmt.Errorf("清理历史失败: %w", err)
			}
			return fmt.Sprintf("已清理 %d 条超过 %d 天的历史记录", n, days), nil
		},
	})

	// ── 云 DNS 记录操作 ──────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "update_dns_record_value",
		Description: "直接修改阿里云 DNS 上指定记录的解析值。当用户说'修改DNS记录值'或'更新IP'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"record_id":   {Type: "string", Description: "DNS 记录 ID（通过 query_dns_records 获取）"},
				"rr":          {Type: "string", Description: "主机记录"},
				"record_type": {Type: "string", Description: "记录类型", Enum: []string{"A", "AAAA", "CNAME", "MX", "TXT"}},
				"value":       {Type: "string", Description: "新的记录值"},
				"ttl":         {Type: "integer", Description: "TTL 秒数，默认 600"},
			},
			Required: []string{"record_id", "rr", "record_type", "value"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			recordID, _ := args["record_id"].(string)
			rr, _ := args["rr"].(string)
			recType, _ := args["record_type"].(string)
			value, _ := args["value"].(string)
			ttl := GetInt(args, "ttl", 600)

	if err := dnsProvider.UpdateRecord(ctx, recordID, rr, recType, value, ttl); err != nil {
				return "", fmt.Errorf("更新 DNS 记录失败: %w", err)
			}
			return fmt.Sprintf("DNS 记录 %s (%s) 已更新为 %s", rr, recType, value), nil
		},
	})

	// ── 新增: 云 DNS 记录增删查改 ────────────────────────

	r.Register(&base.ToolDef{
		Name:        "add_cloud_dns_record",
		Description: "在阿里云 DNS 上新增一条解析记录。当用户说'添加一条解析'或'新增DNS记录'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"domain_name": {Type: "string", Description: "域名，例如 example.com"},
				"rr":          {Type: "string", Description: "主机记录，例如 www 或 @（主域名）"},
				"record_type": {Type: "string", Description: "记录类型", Enum: []string{"A", "AAAA", "CNAME", "MX", "TXT"}},
				"value":       {Type: "string", Description: "记录值，例如 IP 地址或域名"},
				"ttl":         {Type: "integer", Description: "TTL 秒数，默认 600"},
			},
			Required: []string{"domain_name", "rr", "record_type", "value"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			domainName, _ := args["domain_name"].(string)
			rr, _ := args["rr"].(string)
			recType, _ := args["record_type"].(string)
			value, _ := args["value"].(string)
			ttl := GetInt(args, "ttl", 600)

recordID, err := dnsProvider.AddRecord(ctx, domainName, rr, recType, value, ttl)
			if err != nil {
				return "", fmt.Errorf("添加云解析记录失败: %w", err)
			}
			return fmt.Sprintf("云解析记录已添加: %s.%s (%s) -> %s, RecordID: %s", rr, domainName, recType, value, recordID), nil
		},
	})

	r.Register(&base.ToolDef{
		Name:        "delete_cloud_dns_record",
		Description: "删除阿里云 DNS 上的一条解析记录。当用户说'删除解析记录'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"record_id": {Type: "string", Description: "云解析记录 ID"},
			},
			Required: []string{"record_id"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			recordID, _ := args["record_id"].(string)
			if recordID == "" {
				return "", fmt.Errorf("请输入云解析记录 ID")
			}
	if err := dnsProvider.DeleteRecord(ctx, recordID); err != nil {
				return "", fmt.Errorf("删除云解析记录失败: %w", err)
			}
			return fmt.Sprintf("云解析记录 %s 已删除", recordID), nil
		},
	})

	r.Register(&base.ToolDef{
		Name:        "list_cloud_records",
		Description: "列出阿里云 DNS 上指定域名的全部解析记录。当用户问'某个域名有哪些解析记录'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"domain_name": {Type: "string", Description: "域名，例如 example.com"},
				"page":        {Type: "integer", Description: "页码，从 1 开始，默认 1"},
				"size":        {Type: "integer", Description: "每页数量，默认 50，最大 500"},
			},
			Required: []string{"domain_name"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			domainName, _ := args["domain_name"].(string)
			page := GetInt(args, "page", 1)
			size := GetInt(args, "size", 50)

records, total, err := dnsProvider.ListAllRecords(ctx, domainName, page, size)
			if err != nil {
				return "", fmt.Errorf("列出云解析记录失败: %w", err)
			}
			return ToJSON(map[string]any{
				"total":   total,
				"page":    page,
				"records": records,
			})
		},
	})

	r.Register(&base.ToolDef{
		Name:        "set_cloud_record_status",
		Description: "启用或停用阿里云 DNS 上的一条解析记录。当用户说'启用解析'或'暂停解析'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"record_id": {Type: "string", Description: "云解析记录 ID"},
				"status":    {Type: "string", Description: "目标状态", Enum: []string{"Enable", "Disable"}},
			},
			Required: []string{"record_id", "status"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			recordID, _ := args["record_id"].(string)
			status, _ := args["status"].(string)
			if status != "Enable" && status != "Disable" {
				return "", fmt.Errorf("状态必须是 Enable 或 Disable")
			}
			if err := dnsProvider.SetRecordStatus(ctx, recordID, status); err != nil {
				return "", fmt.Errorf("设置云解析状态失败: %w", err)
			}
			label := "已启用"
			if status == "Disable" {
				label = "已停用"
			}
			return fmt.Sprintf("云解析记录 %s %s", recordID, label), nil
		},
	})

	// ── 新增: 系统配置 ──────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "get_system_config",
		Description: "查看当前系统配置，包括服务器、数据库、通知、认证等设置。敏感信息（密码、密钥）已脱敏。当用户问'系统配置'或'当前设置'时调用。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			if cfg == nil {
				return "配置未加载", nil
			}
			view := map[string]any{
				"server": map[string]any{
					"host":             cfg.Server.Host,
					"port":             cfg.Server.Port,
					"shutdown_timeout": cfg.Server.ShutdownTimeout,
				},
				"database": map[string]any{
					"path":      cfg.Database.Path,
					"has_mysql": cfg.Database.MySQL != nil && cfg.Database.MySQL.Host != "",
				},
				"alidns": map[string]any{
					"endpoint":   cfg.AliDNS.Endpoint,
					"region_id":  cfg.AliDNS.RegionID,
					"has_key":    cfg.AliDNS.AccessKeyID != "",
					"has_secret": cfg.AliDNS.AccessKeySecret != "",
				},
				"detect": map[string]any{
					"interval":     cfg.Detect.Interval,
					"ipv6_enabled": cfg.Detect.IPv6Enabled,
					"ipv4_enabled": cfg.Detect.IPv4Enabled,
					"dns_servers":  cfg.Detect.DNSServers,
				},
				"notify": map[string]any{
					"email": map[string]any{
						"enabled":  cfg.Notify.Email.Enabled,
						"host":     cfg.Notify.Email.Host,
						"port":     cfg.Notify.Email.Port,
						"user":     MaskIfSet(cfg.Notify.Email.User),
						"password": MaskIfSet(cfg.Notify.Email.Password),
						"to":       cfg.Notify.Email.To,
					},
					"webhook": map[string]any{
						"enabled": cfg.Notify.Webhook.Enabled,
						"url":     cfg.Notify.Webhook.URL,
					},
					"dingtalk": map[string]any{
						"enabled":     cfg.Notify.DingTalk.Enabled,
						"webhook_url": cfg.Notify.DingTalk.WebhookURL,
					},
				},
				"auth": map[string]any{
					"username":     cfg.Auth.Username,
					"has_password": cfg.Auth.Password != "",
					"has_jwt":      cfg.Auth.JWTSecret != "",
				},
				"ai": func() map[string]any {
					m := make(map[string]any, len(cfg.AI.Providers))
					for key, pcfg := range cfg.AI.Providers {
						m[key] = map[string]any{
							"enabled":  pcfg.Enabled,
							"has_key":  pcfg.APIKey != "",
						}
					}
					return m
				}(),
				"qqbot": map[string]any{
					"enabled":    cfg.QQBot.Enabled,
					"app_id":     cfg.QQBot.AppID,
					"group_id":   cfg.QQBot.GroupID,
					"has_secret": cfg.QQBot.AppSecret != "",
				},
				"log": map[string]any{
					"level":    cfg.Log.Level,
					"dir":      cfg.Log.Dir,
					"max_days": cfg.Log.MaxDays,
				},
			}
			return ToJSON(view)
		},
	})

	// ── 新增: 网络接口信息 ──────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "get_network_info",
		Description: "查看服务器网络接口信息，包括网卡名、MAC 地址、IPv4/IPv6 地址。当用户问'网络信息'或'IP地址'时调用。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			interfaces, err := net.Interfaces()
			if err != nil {
				return "", fmt.Errorf("获取网络接口失败: %w", err)
			}

			type ifaceInfo struct {
				Name  string   `json:"name"`
				MAC   string   `json:"mac"`
				Addrs []string `json:"addrs"`
			}

			var result []ifaceInfo
			for _, iface := range interfaces {
				if iface.Flags&net.FlagUp == 0 {
					continue
				}
				info := ifaceInfo{
					Name: iface.Name,
					MAC:  iface.HardwareAddr.String(),
				}
				addrs, err := iface.Addrs()
				if err != nil {
					continue
				}
				for _, addr := range addrs {
					if ipnet, ok := addr.(*net.IPNet); ok {
						info.Addrs = append(info.Addrs, ipnet.IP.String())
					}
				}
				if len(info.Addrs) > 0 {
					result = append(result, info)
				}
			}
			return ToJSON(result)
			},
})

// ── 对外交互工具 ──────────────────────────────────

r.Register(&base.ToolDef{
	Name:        "ping_host",
	Description: "Ping 一个主机名或 IP 地址检测网络连通性。当用户问'能不能访问xxx'或'检查网络'时调用。",
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"host": {Type: "string", Description: "主机名或 IP 地址"},
			"count": {Type: "integer", Description: "Ping 次数，默认 3"},
		},
		Required: []string{"host"},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		host, _ := args["host"].(string)
		if host == "" {
			return "", fmt.Errorf("请指定主机名")
		}
		count := GetInt(args, "count", 3)
		if count < 1 {
			count = 1
		}
		if count > 10 {
			count = 10
		}
		// 使用 go-fastping 或系统 ping
		cmd := exec.Command("ping", "-c", strconv.Itoa(count), "-W", "3", host)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ToJSON(map[string]any{
				"host":    host,
				"reachable": false,
				"output":  string(output),
			})
		}
		return ToJSON(map[string]any{
			"host":     host,
			"reachable": true,
			"output":   string(output),
		})
	},
})

r.Register(&base.ToolDef{
	Name:        "read_app_log",
	Description: "读取云兮应用自身的运行日志。当用户问'查看日志'、'有什么错误'时调用。默认返回最近 50 行。",
		IsConcurrencySafe: true,
	Parameters: base.ToolParams{
		Type: "object",
		Properties: map[string]base.ParamProp{
			"lines": {Type: "integer", Description: "读取的行数，默认 50，最大 200"},
			"level": {Type: "string", Description: "日志级别过滤: ERROR/WARN/INFO，默认不过滤"},
		},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		lines := GetInt(args, "lines", 50)
		if lines < 1 {
			lines = 50
		}
		if lines > 200 {
			lines = 200
		}
		level, _ := args["level"].(string)
		// 查找最近日志文件
		logDir := cfg.Log.Dir
		if logDir == "" {
			logDir = "./log"
		}
		// 查找最新的日志文件
		cmd := exec.Command("sh", "-c",
			fmt.Sprintf("find %s -name '*.log' -type f | sort -r | head -1 | xargs tail -n %d", logDir, lines))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("读取日志失败: %w", err)
		}
		result := string(output)
		// 按级别过滤
		if level != "" {
			filtered := ""
			for _, line := range strings.Split(result, "\n") {
				if strings.Contains(line, "level="+strings.ToUpper(level)) {
					filtered += line + "\n"
				}
			}
			result = filtered
		}
		return ToJSON(map[string]any{
			"lines": len(strings.Split(result, "\n")),
			"level": level,
			"log":   result,
		})
	},
})

r.Register(&base.ToolDef{
	Name:        "list_services",
	Description: "列出云兮家庭服务器中配置的所有服务及其访问地址。当用户问'有哪些服务'或'怎么访问nextcloud'时调用。",
		IsConcurrencySafe: true,
	Parameters: base.ToolParams{
		Type:       "object",
		Properties: map[string]base.ParamProp{},
	},
	Handler: func(ctx context.Context, args map[string]any) (string, error) {
		domain := ""
		if len(cfg.DynamicRecords) > 0 {
			domain = cfg.DynamicRecords[0].DomainName
		}
		services := []map[string]any{
			{"name": "文件管理", "url": "https://" + domain + "/files", "desc": "NAS 文件浏览和管理"},
			{"name": "DNS 管理", "url": "https://" + domain + "/domains", "desc": "DDNS 域名和 DNS 记录管理"},
			{"name": "AI 助手", "url": "https://" + domain + "/chat", "desc": "云兮智能助手"},
			{"name": "系统监控", "url": "https://" + domain + "/system", "desc": "系统资源和进程监控"},
			{"name": "终端", "url": "https://" + domain + "/terminal", "desc": "Web SSH 终端"},
		}
		return ToJSON(map[string]any{
		"domain":   domain,
		"services": services,
		})
	},
})
}

func init() {} // placeholder
