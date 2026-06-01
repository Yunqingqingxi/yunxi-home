package skill

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// ToolDef 返回 run_skill 工具定义（支持 YAML Manifest + 编程式 Skill）
func ToolDef(loader *Loader, runner *Runner) *base.ToolDef {
	return ToolDefWithExecutor(loader, runner, nil)
}

// ToolDefWithExecutor 返回 run_skill 工具定义，同时支持 YAML 和编程式技能。
func ToolDefWithExecutor(loader *Loader, runner *Runner, executor *Executor) *base.ToolDef {
	return &base.ToolDef{
		Name:        "run_skill",
		Description: "运行一个预定义的工作流 Skill。当用户需要执行复杂的多步操作时，优先查找并使用 Skill。可用的 Skill 有: " + skillListSummaryV2(loader, executor),
		Category:    "agent",
		RiskLevel:   "mutation",
		Parameters: base.ToolParams{
			Type: "object",
			Properties: map[string]base.ParamProp{
				"skill_name": {Type: "string", Description: "要运行的 Skill 名称"},
				"params":     {Type: "object", Description: "可选，传递给 Skill 的额外参数"},
			},
			Required: []string{"skill_name"},
		},
		Examples: []base.ToolExample{
			{Description: "运行健康检查", Args: map[string]any{"skill_name": "healthcheck"}},
			{Description: "清理 Docker", Args: map[string]any{"skill_name": "docker-cleanup"}},
			{Description: "回显消息", Args: map[string]any{"skill_name": "echo", "params": map[string]any{"message": "hello"}}},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleSkillRunV2(loader, runner, executor, ctx, args)
		},
	}
}

// ListToolDef 返回 list_skills 工具定义（YAML only）
func ListToolDef(loader *Loader) *base.ToolDef {
	return ListToolDefWithExecutor(loader, nil)
}

// ListToolDefWithExecutor 返回 list_skills 工具定义，包含编程式和 YAML 技能。
func ListToolDefWithExecutor(loader *Loader, executor *Executor) *base.ToolDef {
	return &base.ToolDef{
		Name:        "list_skills",
		Description: "列出所有可用的预定义 Skill 工作流及其描述。",
		Category:    "agent",
		RiskLevel:   "readonly",
		IsConcurrencySafe: true,
		Parameters: base.ToolParams{
			Type:       "object",
			Properties: map[string]base.ParamProp{},
		},
		HandlerV2: func(ctx context.Context, args map[string]any) *base.ToolResult {
			return handleListSkillsV2(loader, executor)
		},
	}
}

// ── V2 处理函数 (支持 Executor) ────────────────────────────────────

func handleSkillRunV2(loader *Loader, runner *Runner, executor *Executor, ctx context.Context, args map[string]any) *base.ToolResult {
	name, _ := args["skill_name"].(string)
	if name == "" {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: base.ErrCodeInvalidArgs, Message: "请指定 skill_name"},
			Summary: "缺少 skill_name 参数",
		}
	}

	params, _ := args["params"].(map[string]any)
	if params == nil {
		params = make(map[string]any)
	}

	// 1. 尝试 Executor（编程式技能优先）
	if executor != nil {
		if executor.Registry().Has(name) {
			result, err := executor.Run(ctx, name, params)
			if err != nil {
				return &base.ToolResult{
					Status:  base.StatusError,
					Error:   &base.ToolError{Code: base.ErrCodeExecFailed, Message: err.Error()},
					Summary: fmt.Sprintf("Skill '%s' 执行失败: %v", name, err),
				}
			}
			return &base.ToolResult{
				Status:  base.StatusSuccess,
				Summary: fmt.Sprintf("[%s] %v", name, result),
			}
		}
	}

	// 2. 回退到 YAML Loader
	if loader == nil || runner == nil {
		available := ""
		if executor != nil {
			available = executor.Registry().Summary()
		}
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: "SKILL_NOT_FOUND", Message: "Skill 不存在: " + name},
			Summary: fmt.Sprintf("Skill '%s' 不存在。可用:\n%s", name, available),
		}
	}

	skill := loader.Get(name)
	if skill == nil {
		return &base.ToolResult{
			Status:  base.StatusError,
			Error:   &base.ToolError{Code: "SKILL_NOT_FOUND", Message: "Skill 不存在: " + name},
			Summary: fmt.Sprintf("Skill '%s' 不存在。可用: %s", name, strings.Join(loader.All(), ", ")),
		}
	}

	exec := runner.Execute(ctx, skill)

	if exec.Status == StepFailed {
		var failures []string
		for _, sr := range exec.Steps {
			if sr.Status == StepFailed {
				failures = append(failures, fmt.Sprintf("步骤 %d (%s): %s", sr.ID, sr.Tool, sr.Error))
			}
		}
		return &base.ToolResult{
			Status:  base.StatusPartial,
			Data:    exec,
			Summary: fmt.Sprintf("Skill '%s' 部分失败 (%d/%d 成功)\n%s", name, countDone(exec.Steps), exec.TotalSteps, strings.Join(failures, "\n")),
		}
	}

	var results []string
	for _, sr := range exec.Steps {
		results = append(results, fmt.Sprintf("✅ 步骤 %d (%s): %s", sr.ID, sr.Purpose, sr.Result))
	}
	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Data:    exec,
		Summary: fmt.Sprintf("Skill '%s' 执行成功 (%d/%d)\n%s", name, countDone(exec.Steps), exec.TotalSteps, strings.Join(results, "\n")),
	}
}

func handleListSkillsV2(loader *Loader, executor *Executor) *base.ToolResult {
	var all map[string]string
	if executor != nil {
		all = executor.Registry().ListAll()
	} else if loader != nil {
		all = make(map[string]string)
		for _, n := range loader.All() {
			if s := loader.Get(n); s != nil {
				all[n] = s.Description
			}
		}
	}
	if len(all) == 0 {
		return &base.ToolResult{Status: base.StatusSuccess, Summary: "当前没有可用的 Skill"}
	}

	names := make([]string, 0, len(all))
	for n := range all { names = append(names, n) }
	sort.Strings(names)
	var lines []string
	for _, n := range names {
		lines = append(lines, fmt.Sprintf("- %s: %s", n, all[n]))
	}
	return &base.ToolResult{
		Status:  base.StatusSuccess,
		Summary: "可用 Skill:\n" + strings.Join(lines, "\n"),
	}
}

func skillListSummaryV2(loader *Loader, executor *Executor) string {
	var all map[string]string
	if executor != nil {
		all = executor.Registry().ListAll()
	}
	if loader != nil {
		for _, n := range loader.All() {
			if s := loader.Get(n); s != nil {
				if all == nil { all = make(map[string]string) }
				all[n] = s.Description
			}
		}
	}
	if len(all) == 0 { return "暂无预定义 Skill" }
	names := make([]string, 0, len(all))
	for n := range all { names = append(names, n) }
	sort.Strings(names)
	var parts []string
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s(%s)", n, all[n]))
	}
	return strings.Join(parts, ", ")
}

func countDone(steps []StepResult) int {
	n := 0
	for _, s := range steps {
		if s.Status == StepDone { n++ }
	}
	return n
}
