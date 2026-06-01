package toolreg

import (
	"context"
	"fmt"
	"runtime"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
	"github.com/Yunqingqingxi/yunxi-home/internal/config"
	"github.com/Yunqingqingxi/yunxi-home/internal/docker"
)

// RegisterExtended 注册扩展模块的 AI 工具 (NAS, Docker, Sysctl)
func RegisterExtended(r *register.Registry, dockerMgr *docker.Manager, cfg *config.Config) {
	if dockerMgr == nil || !dockerMgr.IsAvailable() {
		return
	}

	// ── Docker 容器管理 ──────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "docker_list_containers",
		Description: "列出所有 Docker 容器的运行状态（名称/镜像/状态/端口/运行时间）。先调用此工具了解容器概况，再用 docker_control_container 操作具体容器。",
		Category:    "docker",
		RiskLevel:   "readonly",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"all": {Type: "boolean", Description: "是否包含已停止的容器，默认 false"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			showAll := false
			if v, ok := args["all"].(bool); ok {
				showAll = v
			}
			containers, err := dockerMgr.ListContainers(ctx, showAll)
			if err != nil {
				return "", err
			}
			return ToJSON(containers)
		},
	})

	r.Register(&base.ToolDef{
		Name:        "docker_control_container",
		Description: "控制 Docker 容器的启停（start/stop/restart/pause/unpause）。⚠️ 停止运行中的服务会中断用户访问。操作前先用 docker_list_containers 确认容器名和当前状态。",
		Category:    "docker",
		RiskLevel:   "dangerous",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"container": {Type: "string", Description: "容器名称，例如 nextcloud, jellyfin"},
				"action":    {Type: "string", Description: "操作", Enum: []string{"start", "stop", "restart", "pause", "unpause"}},
			},
			Required: []string{"container", "action"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["container"].(string)
			action, _ := args["action"].(string)
			out, err := dockerMgr.ContainerAction(ctx, name, action)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("容器 %s %s 成功\n%s", name, action, out), nil
		},
	})

	r.Register(&base.ToolDef{
		Name:        "docker_get_logs",
		Description: "获取 Docker 容器的最近日志。当用户问'查看nextcloud的日志'时调用。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"container": {Type: "string", Description: "容器名称"},
				"tail":      {Type: "integer", Description: "最近多少行，默认 50"},
			},
			Required: []string{"container"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["container"].(string)
			tail := GetInt(args, "tail", 50)
			logs, err := dockerMgr.GetLogs(ctx, name, tail)
			if err != nil {
				return "", err
			}
			return logs, nil
		},
	})

	r.Register(&base.ToolDef{
		Name:        "docker_compose",
		Description: "管理 Docker Compose 项目栈。当用户说'重启所有服务'或'更新docker镜像'时调用。支持 up/down/restart/ps/pull。",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"action":      {Type: "string", Description: "操作", Enum: []string{"up", "down", "restart", "ps", "pull"}},
				"project_dir": {Type: "string", Description: "项目目录路径，默认 /app/deploy"},
			},
			Required: []string{"action"},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			action, _ := args["action"].(string)
			dir, _ := args["project_dir"].(string)
			if dir == "" {
				dir = "/app/deploy"
			}
			out, err := dockerMgr.ComposeAction(ctx, dir, action)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("docker compose %s 成功\n%s", action, out), nil
		},
	})

	// ── 系统资源监控 ──────────────────────────────────

	r.Register(&base.ToolDef{
		Name:        "get_server_resources",
		Description: "获取服务器资源使用详情: CPU 核心数、内存总量/使用量、协程数。用于了解服务器负载情况。",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		Handler: func(ctx context.Context, args map[string]any) (string, error) {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return ToJSON(map[string]any{
				"cpu_cores":    runtime.NumCPU(),
				"goroutines":   runtime.NumGoroutine(),
				"go_version":   runtime.Version(),
				"heap_alloc_mb": float64(m.HeapAlloc) / 1024 / 1024,
				"heap_sys_mb":  float64(m.HeapSys) / 1024 / 1024,
				"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
				"num_gc":       m.NumGC,
			})
		},
	})
}
