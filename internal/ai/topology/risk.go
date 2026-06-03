package topology

import (
	"strings"
	"sync"
)

// ── Risk Profile Table ────────────────────────────────────────
//
// Maps tool name patterns to expected coordinate deltas.
// Profiles are dynamically extensible — new tools register automatically.
// Unmatched patterns fall back to the "*" wildcard entry.

var (
	riskProfiles   []RiskProfile
	riskProfilesMu sync.RWMutex
)

func init() {
	// Built-in risk profiles
	RegisterRiskProfiles([]RiskProfile{
		// Read-only / query tools — low complexity, low deviation
		{Pattern: "file_read", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "file_list", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "file_info", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "file_search", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "get_system_*", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "get_*", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},
		{Pattern: "list_*", DeltaYMin: -0.3, DeltaYMax: 0.1, DeltaZMin: 0, DeltaZMax: 0.2},

		// Write tools — moderate complexity
		{Pattern: "file_write", DeltaYMin: 0.2, DeltaYMax: 0.4, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "file_mkdir", DeltaYMin: 0.2, DeltaYMax: 0.4, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "file_edit", DeltaYMin: 0.2, DeltaYMax: 0.4, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "file_copy", DeltaYMin: 0.2, DeltaYMax: 0.4, DeltaZMin: 0, DeltaZMax: 0.3},

		// Destructive tools — high complexity
		{Pattern: "file_delete", DeltaYMin: 0.4, DeltaYMax: 0.8, DeltaZMin: 0, DeltaZMax: 0.4},
		{Pattern: "file_rename", DeltaYMin: 0.4, DeltaYMax: 0.8, DeltaZMin: 0, DeltaZMax: 0.4},

		// Command execution — high complexity, high deviation
		{Pattern: "run_command", DeltaYMin: 0.4, DeltaYMax: 1.0, DeltaZMin: 0, DeltaZMax: 1.0},

		// run_command_* sub-command overrides — narrower profiles for known subcommands
		// Longer patterns match first, so specific subcommands get tighter constraints.
		// Build/compile commands — high progress, moderate complexity
		{Pattern: "run_command_go_build", DeltaYMin: 0.2, DeltaYMax: 0.6, DeltaZMin: 0, DeltaZMax: 0.5},
		{Pattern: "run_command_go_test", DeltaYMin: 0.2, DeltaYMax: 0.5, DeltaZMin: 0, DeltaZMax: 0.4},
		{Pattern: "run_command_npm_run", DeltaYMin: 0.2, DeltaYMax: 0.6, DeltaZMin: 0, DeltaZMax: 0.5},
		// Query/info commands — low complexity, low deviation
		{Pattern: "run_command_*_query", DeltaYMin: -0.2, DeltaYMax: 0.3, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "run_command_*_list", DeltaYMin: -0.2, DeltaYMax: 0.3, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "run_command_*_info", DeltaYMin: -0.2, DeltaYMax: 0.3, DeltaZMin: 0, DeltaZMax: 0.3},
		{Pattern: "run_command_*_status", DeltaYMin: -0.2, DeltaYMax: 0.3, DeltaZMin: 0, DeltaZMax: 0.3},
		// General wildcard for run_command_* — slightly narrower than plain run_command
		{Pattern: "run_command_*", DeltaYMin: 0.3, DeltaYMax: 0.8, DeltaZMin: 0, DeltaZMax: 0.7},

		// Docker / system control — high complexity, high deviation
		{Pattern: "docker_*", DeltaYMin: 0.5, DeltaYMax: 1.0, DeltaZMin: 0, DeltaZMax: 0.8},
		{Pattern: "systemctl_*", DeltaYMin: 0.5, DeltaYMax: 1.0, DeltaZMin: 0, DeltaZMax: 0.8},

		// Sub-agent spawn — moderate-high complexity
		{Pattern: "spawn_agent", DeltaYMin: 0.3, DeltaYMax: 0.8, DeltaZMin: 0, DeltaZMax: 0.5},

		// Skill execution — moderate complexity
		{Pattern: "run_skill", DeltaYMin: 0.2, DeltaYMax: 0.6, DeltaZMin: 0, DeltaZMax: 0.5},

		// Cron / scheduling — moderate complexity
		{Pattern: "cron_*", DeltaYMin: 0.2, DeltaYMax: 0.5, DeltaZMin: 0, DeltaZMax: 0.4},

		// Confirmation / interactive — low complexity
		{Pattern: "request_confirmation", DeltaYMin: -0.1, DeltaYMax: 0.2, DeltaZMin: 0, DeltaZMax: 0.1},

		// MCP tools — moderate complexity
		{Pattern: "mcp__*", DeltaYMin: 0.2, DeltaYMax: 0.6, DeltaZMin: 0, DeltaZMax: 0.5},

		// Wildcard fallback — generous tolerance for unknown tools
		{Pattern: "*", DeltaYMin: -0.5, DeltaYMax: 0.5, DeltaZMin: 0, DeltaZMax: 0.5},
	})
}

// RegisterRiskProfiles appends new risk profiles to the table.
func RegisterRiskProfiles(profiles []RiskProfile) {
	riskProfilesMu.Lock()
	defer riskProfilesMu.Unlock()
	riskProfiles = append(riskProfiles, profiles...)
}

// RegisterRiskProfile adds a single risk profile, replacing an existing one with the same pattern.
func RegisterRiskProfile(profile RiskProfile) {
	riskProfilesMu.Lock()
	defer riskProfilesMu.Unlock()
	for i, p := range riskProfiles {
		if p.Pattern == profile.Pattern {
			riskProfiles[i] = profile
			return
		}
	}
	riskProfiles = append(riskProfiles, profile)
}

// MatchRiskProfile finds the best matching risk profile for a tool name.
// Returns the wildcard fallback if no specific match is found.
func MatchRiskProfile(toolName string) RiskProfile {
	riskProfilesMu.RLock()
	defer riskProfilesMu.RUnlock()

	var fallback RiskProfile
	var bestMatch RiskProfile
	bestLen := 0

	for _, p := range riskProfiles {
		if p.Pattern == "*" {
			fallback = p
			continue
		}
		if matchPattern(p.Pattern, toolName) {
			if len(p.Pattern) > bestLen {
				bestMatch = p
				bestLen = len(p.Pattern)
			}
		}
	}

	if bestLen > 0 {
		return bestMatch
	}
	return fallback
}

// matchPattern checks if a tool name matches a pattern.
// Supports * wildcard: "docker_*" matches "docker_start", "get_*" matches "get_system_status".
func matchPattern(pattern, name string) bool {
	if pattern == name {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}
	return false
}

// GetAllRiskProfiles returns a copy of all registered risk profiles.
func GetAllRiskProfiles() []RiskProfile {
	riskProfilesMu.RLock()
	defer riskProfilesMu.RUnlock()
	cp := make([]RiskProfile, len(riskProfiles))
	copy(cp, riskProfiles)
	return cp
}

// EstimateProgressDelta 估算工具调用对应的 X 轴（进度）增量。
// 用于 AI 未自报 <topology> 标签时的系统估算。
// EstimateProgressDelta 估算工具调用对应的 X 轴（累积进度）增量。
// 适用于累积任务制：X 只增不减，简单任务 +1~3，复杂任务 +5~10。
func EstimateProgressDelta(toolName string) float64 {
	switch {
	// 查询/读取类：小幅推进
	case matchPattern("file_read", toolName),
		matchPattern("file_list", toolName),
		matchPattern("file_info", toolName),
		matchPattern("file_search", toolName),
		matchPattern("get_*", toolName),
		matchPattern("list_*", toolName),
		matchPattern("query_*", toolName):
		return 2.0

	// 写入/修改类：中等推进
	case matchPattern("file_write", toolName),
		matchPattern("file_mkdir", toolName),
		matchPattern("file_edit", toolName),
		matchPattern("file_copy", toolName),
		matchPattern("file_move", toolName),
		matchPattern("add_*", toolName),
		matchPattern("create_*", toolName),
		matchPattern("update_*", toolName),
		matchPattern("set_*", toolName),
		matchPattern("file_delete", toolName),
		matchPattern("delete_*", toolName):
		return 3.0

	// 执行/运维类：显著推进
	case matchPattern("run_command", toolName),
		matchPattern("docker_*", toolName),
		matchPattern("ssh_*", toolName),
		matchPattern("systemctl_*", toolName):
		return 5.0

	// 子Agent/技能/MCP：大幅推进（代表拆分了子任务）
	case matchPattern("spawn_agent", toolName),
		matchPattern("run_skill", toolName):
		return 8.0

	case matchPattern("mcp__*", toolName):
		return 4.0

	default:
		return 2.0
	}
}
