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
