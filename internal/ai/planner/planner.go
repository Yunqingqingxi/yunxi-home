// Package planner 提供多步任务计划生成和执行引擎。
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/yxd/yunxi-home/internal/ai/base"
	"github.com/yxd/yunxi-home/internal/ai/register"
)

// Engine 计划执行引擎
type Engine struct {
	registry *register.Registry
}

// New 创建计划引擎
func New(reg *register.Registry) *Engine {
	return &Engine{registry: reg}
}

// PlanPrompt 注入到 system prompt 中，教 LLM 生成计划 JSON
const PlanPrompt = `
## 多步任务计划模式

当用户的任务需要 2 个或以上的工具调用，或涉及"批量/所有/一键/全部"等关键词时，你必须先输出一个 JSON 执行计划。

### 计划格式
` + "```json" + `
{
  "plan": {
    "steps": [
      {"id": 1, "tool": "tool_name", "args": {...}, "depends": [], "purpose": "为什么执行这步"},
      {"id": 2, "tool": "tool_name", "args": {...}, "depends": [1], "purpose": "依赖步骤1的结果"}
    ],
    "rollback_on_failure": false,
    "max_concurrency": 3
  }
}
` + "```" + `

### 规则
- id 从 1 开始递增
- depends 为空数组表示可与其他无依赖步骤并发
- 相同 id 的不同工具自动并发执行
- 一个步骤依赖另一个步骤时，必须等前者完成后才执行
- 独立任务（如"备份 A、B、C"）放在同一个 id，并发执行
- 如果只是简单问答或单工具调用，不要生成计划，直接正常回复`

// ShouldPlan 判断用户消息是否需要计划模式
func ShouldPlan(userMessage string) bool {
	triggers := []string{
		"备份所有", "全部备份", "所有容器", "批量",
		"一键", "所有数据库", "全部更新", "全部检查",
		"先", "然后", "接着", "之后", "最后",
		"同时", "并行",
	}
	lower := strings.ToLower(userMessage)
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// ParsePlan 从 LLM 输出中提取 JSON Plan
func ParsePlan(text string) (*base.Plan, error) {
	// 预处理：提取 JSON 块（去掉 markdown 代码块、前后空白）
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// 查找 plan 或 steps
	start := strings.Index(text, `"plan":`)
	if start < 0 {
		start = strings.Index(text, `"steps":`)
		if start < 0 {
			return nil, fmt.Errorf("未找到 plan JSON")
		}
		text = `{"plan":{` + text[start:] + `}}`
	} else {
		// 提取从 "plan": 开始的完整 JSON，处理已有外层 {}
		inner := text[start:]
		if strings.HasPrefix(text, "{") {
			// 已有一层 {}，去掉外层后加 plan 包装
			inner = strings.TrimPrefix(text, "{")
			inner = strings.TrimSuffix(inner, "}")
			inner = strings.TrimSpace(inner)
			if !strings.HasPrefix(inner, `"plan":`) {
				inner = `"plan":` + inner
			}
		}
		text = `{` + inner + `}`
	}
	// Find the closing brace
	depth := 0
	end := -1
	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("无法解析 plan JSON")
	}
	text = text[:end]

	var wrapper struct {
		Plan base.Plan `json:"plan"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return nil, fmt.Errorf("plan JSON 解析失败: %w", err)
	}
	if len(wrapper.Plan.Steps) == 0 {
		return nil, fmt.Errorf("plan 中无步骤")
	}
	return &wrapper.Plan, nil
}

// Execute 执行计划中的所有步骤
func (e *Engine) Execute(ctx context.Context, plan *base.Plan) *base.PlanResult {
	start := time.Now()
	result := &base.PlanResult{
		Steps:      make([]base.StepResult, len(plan.Steps)),
		TotalSteps: len(plan.Steps),
	}

	// 按依赖关系分组执行
	executed := make(map[int]bool)
	stepResults := make(map[int]*base.StepResult)

	for len(executed) < len(plan.Steps) {
		// 找出所有依赖已满足的步骤
		var ready []int
		for i := range plan.Steps {
			step := &plan.Steps[i]
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
			slog.Warn("plan deadlock detected", "executed", len(executed), "remaining", len(plan.Steps)-len(executed))
			break
		}

		// 并发执行就绪的步骤
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, idx := range ready {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				step := &plan.Steps[i]
				sr := e.executeStep(ctx, step)
				mu.Lock()
				stepResults[step.ID] = sr
				executed[step.ID] = true
				result.Steps[i] = *sr
				mu.Unlock()
			}(idx)
		}
		wg.Wait()

		// 检查是否有步骤失败且需要回滚
		if plan.RollbackOnFailure {
			for _, idx := range ready {
				if result.Steps[idx].Status == base.StatusError {
					slog.Warn("plan step failed, rollback requested", "step", result.Steps[idx].ID)
					result.DurationMs = time.Since(start).Milliseconds()
					return result
				}
			}
		}
	}

	// 统计
	for _, sr := range result.Steps {
		switch sr.Status {
		case base.StatusSuccess:
			result.Successes++
		case base.StatusError:
			result.Failures++
		}
	}
	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func (e *Engine) executeStep(ctx context.Context, step *base.PlanStep) *base.StepResult {
	slog.Info("executing plan step", "id", step.ID, "tool", step.Tool, "purpose", step.Purpose)

	tool, ok := e.registry.Get(step.Tool)
	if !ok {
		return &base.StepResult{
			ID:     step.ID,
			Tool:   step.Tool,
			Status: base.StatusError,
			Result: &base.ToolResult{
				Status: base.StatusError,
				Error:  &base.ToolError{Code: base.ErrCodeUnknown, Message: "未知工具: " + step.Tool},
				Summary: "工具未注册",
			},
		}
	}

	var result *base.ToolResult
	if tool.HandlerV2 != nil {
		result = tool.HandlerV2(ctx, step.Args)
	} else {
		data, err := tool.Handler(ctx, step.Args)
		if err != nil {
			result = &base.ToolResult{
				Status:  base.StatusError,
				Error:   &base.ToolError{Code: base.ErrCodeExecFailed, Message: err.Error()},
				Summary: "执行失败: " + err.Error(),
			}
		} else {
			result = &base.ToolResult{Status: base.StatusSuccess, Data: data, Summary: truncate(data, 200)}
		}
	}

	return &base.StepResult{
		ID:     step.ID,
		Tool:   step.Tool,
		Status: result.Status,
		Result: result,
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
