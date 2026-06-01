package middleware

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yxd/yunxi-home/internal/ai/base"
)

// ── DenyEngine: 硬边界规则引擎 ──────────────────────────────────
//
// 在工具执行前进行强制安全检查。任何匹配 Deny 规则的操作被直接拒绝，
// 不依赖模型遵循提示词——这是不可绕过的代码层拦截。
//
// 优先级：Deny > Allow > Ask（Ask 由 chat 层的 confirm_required 处理）

// DenyRule 单条拒绝规则
type DenyRule struct {
	Name        string            // 规则名称（用于日志）
	ToolPattern string            // glob 匹配工具名（支持 * 通配符）
	ArgMatch    map[string]string // 参数名 → 正则表达式（全部匹配才命中）
	Reason      string            // 拒绝原因（返回给 AI）
}

// DenyEngine 拒绝规则引擎
type DenyEngine struct {
	rules []DenyRule
}

// NewDenyEngine 创建引擎并注册内置安全规则
func NewDenyEngine() *DenyEngine {
	e := &DenyEngine{}
	e.registerBuiltinRules()
	return e
}

// Check 检查工具调用是否被拒绝。返回 nil 表示允许，返回 ToolResult 表示拒绝。
func (e *DenyEngine) Check(toolName string, args map[string]any) *base.ToolResult {
	for i := range e.rules {
		r := &e.rules[i]
		if !matchToolPattern(r.ToolPattern, toolName) {
			continue
		}
		if !matchArgs(r.ArgMatch, args) {
			continue
		}
		return &base.ToolResult{
			Status: base.StatusError,
			Error: &base.ToolError{
				Code:      base.ErrCodePermissionDenied,
				Message:   fmt.Sprintf("[安全拦截] %s", r.Reason),
				Retryable: false,
			},
			Summary: fmt.Sprintf("操作被拒绝: %s (规则: %s)", r.Reason, r.Name),
		}
	}
	return nil
}

// AddRule 添加自定义拒绝规则
func (e *DenyEngine) AddRule(r DenyRule) {
	e.rules = append(e.rules, r)
}

// Rules 返回所有已注册规则（只读）
func (e *DenyEngine) Rules() []DenyRule { return e.rules }

// ── 内置安全规则 ──────────────────────────────────────────────

func (e *DenyEngine) registerBuiltinRules() {
	builtin := []DenyRule{
		{
			Name:        "destroy-root",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)rm\s+-rf\s+/\s*`},
			Reason:      "禁止执行 rm -rf / 及变体",
		},
		{
			Name:        "destroy-home",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)rm\s+-rf\s+~(\s|/|$)`},
			Reason:      "禁止执行 rm -rf ~ 及变体",
		},
		{
			Name:        "destroy-etc",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)rm\s+-rf\s+/etc`},
			Reason:      "禁止破坏 /etc 系统配置目录",
		},
		{
			Name:        "format-disk",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)(mkfs\.|dd\s+if=.*of=/dev/)`},
			Reason:      "禁止格式化或覆写磁盘设备",
		},
		{
			Name:        "fork-bomb",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `:\(\)\s*\{`},
			Reason:      "禁止 fork bomb 模式",
		},
		{
			Name:        "chmod-system",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)chmod\s+-R\s+777\s+/`},
			Reason:      "禁止递归修改系统根目录权限",
		},
		{
			Name:        "delete-system-file",
			ToolPattern: "file_delete",
			ArgMatch:    map[string]string{"path": `^/(etc|boot|sys|proc|dev)/`},
			Reason:      "禁止删除系统目录下的文件",
		},
		{
			Name:        "delete-sandbox-root",
			ToolPattern: "file_delete",
			ArgMatch:    map[string]string{"path": `^/$`},
			Reason:      "禁止删除沙箱根目录",
		},
		{
			Name:        "curl-pipe-bash",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)curl.*\|.*(ba)?sh`},
			Reason:      "禁止 curl/wget 管道到 shell 执行",
		},
		{
			Name:        "wget-pipe-bash",
			ToolPattern: "run_command",
			ArgMatch:    map[string]string{"command": `(?i)wget.*\|.*(ba)?sh`},
			Reason:      "禁止 wget 管道到 shell 执行",
		},
	}
	e.rules = append(e.rules, builtin...)
}

// ── 匹配逻辑 ──────────────────────────────────────────────────

// matchToolPattern 检查工具名是否匹配 glob 模式
func matchToolPattern(pattern, toolName string) bool {
	if pattern == "*" {
		return true
	}
	// Simple glob: * matches any sequence
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(toolName, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(toolName, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(toolName, pattern[:len(pattern)-1])
	}
	return pattern == toolName
}

// matchArgs 检查所有参数是否匹配对应的正则
func matchArgs(matchers map[string]string, args map[string]any) bool {
	if len(matchers) == 0 {
		return true
	}
	for key, pattern := range matchers {
		val, ok := args[key]
		if !ok {
			return false
		}
		strVal := fmt.Sprintf("%v", val)
		matched, err := regexp.MatchString(pattern, strVal)
		if err != nil || !matched {
			return false
		}
	}
	return true
}
