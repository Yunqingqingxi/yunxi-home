// Package builtin 提供内置的编程式技能。
package builtin

import (
	"context"
	"fmt"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/skill"
)

// EchoSkill 回显输入的参数（示例技能）
type EchoSkill struct{}

func (s *EchoSkill) Name() string        { return "echo" }
func (s *EchoSkill) Description() string { return "回显输入的参数，用于测试技能系统" }
func (s *EchoSkill) Category() string    { return "general" }
func (s *EchoSkill) RiskLevel() string   { return "readonly" }

func (s *EchoSkill) Parameters() skill.ParamSchema {
	return skill.ParamSchema{
		Type: "object",
		Properties: map[string]skill.ParamProp{
			"message": {Type: "string", Description: "要回显的消息"},
			"repeat":  {Type: "integer", Description: "重复次数，默认 1", Default: 1},
		},
		Required: []string{"message"},
	}
}

func (s *EchoSkill) Run(ctx context.Context, params map[string]any, mcp skill.MCPContext) (any, error) {
	msg, _ := params["message"].(string)
	repeat := 1
	if r, ok := params["repeat"].(float64); ok && r > 0 {
		repeat = int(r)
	}

	if mcp != nil {
		mcp.Logger().Info("echo skill called", "message", msg, "repeat", repeat)
	}

	var result string
	for i := 0; i < repeat; i++ {
		result += msg + "\n"
	}
	return result, nil
}

// All 返回所有内置技能
func All() []skill.ProgrammaticSkill {
	return []skill.ProgrammaticSkill{
		&EchoSkill{},
	}
}

// RegisterAll 将所有内置技能注册到给定的注册中心
func RegisterAll(reg *skill.Registry) {
	for _, s := range All() {
		reg.Register(s)
	}
}

// RegisterAllTo is a convenience for RegisterAll
func RegisterAllTo(reg *skill.Registry) { RegisterAll(reg) }

// FormatResult 将技能执行结果格式化为用户可读字符串（供命令系统使用）
func FormatResult(name string, result any) string {
	switch v := result.(type) {
	case string:
		return v
	case []string:
		var s string
		for _, line := range v {
			s += line + "\n"
		}
		return s
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("[%s] 执行完成: %v", name, result)
	}
}
