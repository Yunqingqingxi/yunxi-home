package skill

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/ai/register"
)

// Runner 执行 Skill 的步骤序列
type Runner struct {
	registry *register.Registry
	// ProgressFunc 在每步状态变化时回调（用于 SSE 推送）
	ProgressFunc func(exec *Execution)
}

// NewRunner 创建 SkillRunner
func NewRunner(reg *register.Registry, progressFn func(*Execution)) *Runner {
	return &Runner{registry: reg, ProgressFunc: progressFn}
}

// Execute 执行 Skill 的所有步骤，按依赖关系 DAG 调度
func (r *Runner) Execute(ctx context.Context, skill *Manifest) *Execution {
	exec := &Execution{
		SkillName:  skill.Name,
		TotalSteps: len(skill.Steps),
		Steps:      make([]StepResult, len(skill.Steps)),
		Status:     StepRunning,
	}

	start := time.Now()
	executed := make(map[int]bool)
	stepResults := make(map[int]string) // step ID → result text

	for len(executed) < len(skill.Steps) {
		// 找所有依赖已满足的步骤
		var ready []int
		for i := range skill.Steps {
			step := &skill.Steps[i]
			if executed[step.ID] {
				continue
			}
			depsMet := true
			for _, depID := range step.Depends {
				if !executed[depID] {
					depsMet = false
					break
				}
			}
			if depsMet {
				ready = append(ready, i)
			}
		}

		if len(ready) == 0 {
			slog.Warn("skill deadlock", "skill", skill.Name, "executed", len(executed), "remaining", len(skill.Steps)-len(executed))
			exec.Status = StepFailed
			break
		}

		// 并发执行就绪步骤
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, idx := range ready {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				step := &skill.Steps[i]

				// 模板变量替换
				args := resolveArgs(step.Args, stepResults)
				sr := r.executeStep(ctx, step, args)

				mu.Lock()
				stepResults[step.ID] = sr.Result
				executed[step.ID] = true
				exec.Steps[i] = *sr
				exec.CurrentStep = len(executed)
				mu.Unlock()

				// 推送进度
				if r.ProgressFunc != nil {
					r.ProgressFunc(exec)
				}
			}(idx)
		}
		wg.Wait()
	}

	// 检查最终状态
	exec.Status = StepDone
	allDone := true
	for _, sr := range exec.Steps {
		if sr.Status == StepFailed {
			exec.Status = StepFailed
			allDone = false
		} else if sr.Status != StepDone {
			allDone = false
		}
	}
	if allDone {
		exec.Status = StepDone
	}

	slog.Info("skill executed",
		"skill", skill.Name,
		"status", exec.Status,
		"steps", exec.TotalSteps,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return exec
}

func (r *Runner) executeStep(ctx context.Context, step *StepDef, args map[string]any) *StepResult {
	slog.Info("executing skill step", "id", step.ID, "tool", step.Tool, "purpose", step.Purpose)

	// 将执行上下文传递到后续 executor 的 context 中
	tool, ok := r.registry.Get(step.Tool)
	if !ok {
		return &StepResult{
			ID: step.ID, Tool: step.Tool, Purpose: step.Purpose,
			Status: StepFailed, Error: "未知工具: " + step.Tool,
		}
	}

	var result *base.ToolResult
	if tool.HandlerV2 != nil {
		result = tool.HandlerV2(ctx, args)
	} else {
		data, err := tool.Handler(ctx, args)
		if err != nil {
			result = &base.ToolResult{
				Status: base.StatusError,
				Error:  &base.ToolError{Code: base.ErrCodeExecFailed, Message: err.Error()},
				Summary: "失败: " + err.Error(),
			}
		} else {
			result = &base.ToolResult{Status: base.StatusSuccess, Data: data, Summary: data}
		}
	}

	sr := &StepResult{
		ID: step.ID, Tool: step.Tool, Purpose: step.Purpose,
	}
	if result.Status == base.StatusError {
		sr.Status = StepFailed
		if result.Error != nil {
			sr.Error = result.Error.Message
		}
	} else {
		sr.Status = StepDone
		sr.Result = result.Summary
	}
	return sr
}

// resolveArgs 将模板变量 {{.stepN.field}} 替换为实际值
func resolveArgs(args map[string]any, stepResults map[int]string) map[string]any {
	resolved := make(map[string]any, len(args))
	for k, v := range args {
		switch val := v.(type) {
		case string:
			resolved[k] = resolveString(val, stepResults)
		default:
			resolved[k] = v
		}
	}
	return resolved
}

func resolveString(s string, stepResults map[int]string) string {
	for id, result := range stepResults {
		placeholder := fmt.Sprintf("{{.step%d}}", id)
		s = strings.ReplaceAll(s, placeholder, result)
		placeholder = fmt.Sprintf("{{.step%d.result}}", id)
		s = strings.ReplaceAll(s, placeholder, result)
	}
	return s
}
