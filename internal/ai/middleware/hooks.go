// Package middleware provides hook-based event interception for tool execution.
// Inspired by Claude Code's hook system (hookify plugin).
//
// Hooks intercept tool execution at key lifecycle points:
//   - PreToolUse: before a tool executes — can warn or block
//   - PostToolUse: after a tool executes — can warn or modify behavior
//
// Rules are defined as YAML files with frontmatter, supporting:
//   - Regex pattern matching on tool args, results, file paths
//   - Multiple conditions (AND logic)
//   - Actions: warn (allow) or block (prevent)
//   - Hot-reload: rules take effect immediately without restart
package middleware

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"gopkg.in/yaml.v3"
)

var hookLog = logger.ForComponent("hooks")

// HookEvent identifies the tool lifecycle point being intercepted.
type HookEvent string

const (
	HookPreTool  HookEvent = "pre_tool"  // Before tool execution
	HookPostTool HookEvent = "post_tool" // After tool execution
	HookPrompt   HookEvent = "prompt"    // On user prompt (pre-processing)
	HookStop     HookEvent = "stop"      // Before session stop
)

// HookAction determines what happens when a rule matches.
type HookAction string

const (
	HookWarn  HookAction = "warn"  // Show warning, allow operation
	HookBlock HookAction = "block" // Prevent operation
)

// HookCondition defines a matching condition for a hook rule.
type HookCondition struct {
	Field    string `yaml:"field" json:"field"`       // tool_name, args, file_path, command, new_text
	Operator string `yaml:"operator" json:"operator"` // regex_match, contains, equals, not_contains
	Pattern  string `yaml:"pattern" json:"pattern"`
}

// HookRule defines a single hook rule, loaded from .hook/*.yaml files.
type HookRule struct {
	Name        string          `yaml:"name" json:"name"`
	Enabled     bool            `yaml:"enabled" json:"enabled"`
	Event       HookEvent       `yaml:"event" json:"event"`
	Action      HookAction      `yaml:"action" json:"action"`
	Pattern     string          `yaml:"pattern" json:"pattern"`         // simple single-pattern mode
	Conditions  []HookCondition `yaml:"conditions" json:"conditions"`   // advanced multi-condition mode
	Message     string          `yaml:"message" json:"message"`         // body content (warning text)
	compiledRe  *regexp.Regexp  `yaml:"-" json:"-"`
	compiledConds []compiledCondition `yaml:"-" json:"-"`
}

type compiledCondition struct {
	field    string
	op       string
	re       *regexp.Regexp
}

// HookRegistry manages hook rules with hot-reload support.
type HookRegistry struct {
	mu    sync.RWMutex
	rules map[string]*HookRule // name → rule
	dirs  []string             // directories to scan for .hook/*.yaml
}

// NewHookRegistry creates a hook registry scanning the given directories.
func NewHookRegistry(dirs ...string) *HookRegistry {
	hr := &HookRegistry{
		rules: make(map[string]*HookRule),
		dirs:  dirs,
	}
	hr.Reload()
	return hr
}

// AddDir adds a scan directory and reloads.
func (hr *HookRegistry) AddDir(dir string) {
	hr.mu.Lock()
	hr.dirs = append(hr.dirs, dir)
	hr.mu.Unlock()
	hr.Reload()
}

// Reload rescans all directories and reloads rules.
func (hr *HookRegistry) Reload() {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.rules = make(map[string]*HookRule)
	for _, dir := range hr.dirs {
		hookDir := filepath.Join(dir, ".hooks")
		entries, err := os.ReadDir(hookDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(hookDir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			// YAML frontmatter format: the entire file is YAML
			var rule HookRule
			// Try parsing as YAML
			if err := yaml.Unmarshal(data, &rule); err != nil {
				// Try parsing body as message + YAML frontmatter
				parts := strings.SplitN(string(data), "---", 3)
				if len(parts) >= 3 {
					yaml.Unmarshal([]byte(parts[1]), &rule)
					rule.Message = strings.TrimSpace(parts[2])
				}
			}
			if rule.Name == "" {
				continue
			}
			if !rule.Enabled {
				continue
			}
			// Compile regex patterns
			if rule.Pattern != "" {
				re, err := regexp.Compile(rule.Pattern)
				if err == nil {
					rule.compiledRe = re
				}
			}
			for _, cond := range rule.Conditions {
				re, err := regexp.Compile(cond.Pattern)
				if err == nil {
					rule.compiledConds = append(rule.compiledConds, compiledCondition{
						field: cond.Field,
						op:    cond.Operator,
						re:    re,
					})
				}
			}
			hr.rules[rule.Name] = &rule
			hookLog.Info("hook rule loaded", "name", rule.Name, "event", rule.Event, "action", rule.Action)
		}
	}
}

// Check evaluates all matching rules for the given event.
// Returns (blocked bool, warnings []string).
func (hr *HookRegistry) Check(event HookEvent, context map[string]string) (bool, []string) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	var warnings []string
	for _, rule := range hr.rules {
		if rule.Event != event && rule.Event != "all" {
			continue
		}
		if !hr.matches(rule, context) {
			continue
		}
		msg := hr.formatMessage(rule, context)
		if rule.Action == HookBlock {
			hookLog.Warn("hook blocked", "name", rule.Name, "event", event)
			return true, []string{msg}
		}
		warnings = append(warnings, msg)
	}
	return false, warnings
}

func (hr *HookRegistry) matches(rule *HookRule, ctx map[string]string) bool {
	// Multi-condition mode (AND)
	if len(rule.compiledConds) > 0 {
		for _, cond := range rule.compiledConds {
			val := ctx[cond.field]
			switch cond.op {
			case "regex_match":
				if !cond.re.MatchString(val) { return false }
			case "contains":
				if !strings.Contains(val, cond.re.String()) { return false }
			case "equals":
				if val != cond.re.String() { return false }
			case "not_contains":
				if strings.Contains(val, cond.re.String()) { return false }
			case "starts_with":
				if !strings.HasPrefix(val, cond.re.String()) { return false }
			case "ends_with":
				if !strings.HasSuffix(val, cond.re.String()) { return false }
			}
		}
		return true
	}
	// Simple single-pattern mode
	if rule.compiledRe != nil {
		// Check against all context values
		for _, v := range ctx {
			if rule.compiledRe.MatchString(v) {
				return true
			}
		}
		return false
	}
	return true // No conditions = match all
}

// CheckToolPreUse is a convenience wrapper for PreToolUse hooks.
func (hr *HookRegistry) CheckToolPreUse(toolName string, args map[string]any) (bool, []string) {
	argsJSON, _ := json.Marshal(args)
	return hr.Check(HookPreTool, map[string]string{
		"tool_name": toolName,
		"args":      string(argsJSON),
	})
}

// CheckToolPostUse is a convenience wrapper for PostToolUse hooks.
func (hr *HookRegistry) CheckToolPostUse(toolName string, args map[string]any, result string) (bool, []string) {
	argsJSON, _ := json.Marshal(args)
	return hr.Check(HookPostTool, map[string]string{
		"tool_name": toolName,
		"args":      string(argsJSON),
		"result":    result,
	})
}

func (hr *HookRegistry) formatMessage(rule *HookRule, ctx map[string]string) string {
	if rule.Message != "" {
		msg := rule.Message
		for k, v := range ctx {
			msg = strings.ReplaceAll(msg, "{{"+k+"}}", v)
		}
		return msg
	}
	return fmt.Sprintf("Hook [%s] matched: %s", rule.Name, rule.Event)
}
