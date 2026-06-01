package topology

import (
	"fmt"
	"math"
	"strings"
)

// ── Validator ─────────────────────────────────────────────────
//
// Three independent validators run in order:
//   1. CheckGeometry   — |Δy| ≤ A and z ≤ R
//   2. CheckTruthfulness — tool risk profile Δy/Δz expected range ± tolerance
//   3. CheckToolDeclared — claimed tools vs actual tools

// ── Geometry Validator ────────────────────────────────────────

// CheckGeometry validates that the proposed coordinate stays within geometric constraints.
func CheckGeometry(prev, proposed Coordinate, constraint Constraint) ValidationResult {
	// Check amplitude: |Δy| ≤ A
	dy := math.Abs(proposed.Y - prev.Y)
	if dy > constraint.A {
		return ValidationResult{
			Passed:   false,
			Rejected: true,
			Reason:   fmt.Sprintf("振幅超限: |Δy|=%.2f > A=%.2f", dy, constraint.A),
		}
	}

	// Check radius: z ≤ R
	if proposed.Z > constraint.R {
		return ValidationResult{
			Passed:   false,
			Rejected: true,
			Reason:   fmt.Sprintf("偏离超限: z=%.2f > R=%.2f", proposed.Z, constraint.R),
		}
	}

	return ValidationResult{Passed: true}
}

// ── Truthfulness Validator ────────────────────────────────────

// CheckTruthfulness validates that the coordinate delta matches the expected
// risk profile for the tool being called.
func CheckTruthfulness(toolName string, prev, proposed Coordinate) ValidationResult {
	profile := MatchRiskProfile(toolName)

	actualDY := proposed.Y - prev.Y
	actualDZ := proposed.Z - prev.Z

	// Check if actual Δy is within expected range (±tolerance)
	yOk := actualDY >= profile.DeltaYMin-TruthfulnessTolerance &&
		actualDY <= profile.DeltaYMax+TruthfulnessTolerance

	// Check if actual Δz is within expected range (±tolerance)
	zOk := actualDZ >= profile.DeltaZMin-TruthfulnessTolerance &&
		actualDZ <= profile.DeltaZMax+TruthfulnessTolerance

	if !yOk || !zOk {
		var reasons []string
		if !yOk {
			reasons = append(reasons, fmt.Sprintf(
				"Δy=%.2f 超出预期 [%.2f, %.2f] (±%.2f)",
				actualDY, profile.DeltaYMin, profile.DeltaYMax, TruthfulnessTolerance,
			))
		}
		if !zOk {
			reasons = append(reasons, fmt.Sprintf(
				"Δz=%.2f 超出预期 [%.2f, %.2f] (±%.2f)",
				actualDZ, profile.DeltaZMin, profile.DeltaZMax, TruthfulnessTolerance,
			))
		}
		return ValidationResult{
			Passed:   false,
			Rejected: true,
			Reason:   fmt.Sprintf("坐标伪造: %s (工具=%s, 预期Δy=[%.2f,%.2f] Δz=[%.2f,%.2f])",
				strings.Join(reasons, "; "), toolName,
				profile.DeltaYMin, profile.DeltaYMax,
				profile.DeltaZMin, profile.DeltaZMax),
		}
	}

	return ValidationResult{Passed: true}
}

// ── Tool Declaration Validator ────────────────────────────────

// ToolDeclResult holds the outcome of tool declaration validation.
type ToolDeclResult struct {
	Passed          bool
	DeclaredNotUsed []string // Tools AI declared but didn't call
	UsedNotDeclared []string // Tools AI called but didn't declare
	Message         string
}

// CheckToolDeclared compares claimed tool names against actual tool calls.
func CheckToolDeclared(claimedTools, actualTools []string) ToolDeclResult {
	claimed := make(map[string]bool, len(claimedTools))
	for _, t := range claimedTools {
		claimed[strings.TrimSpace(t)] = true
	}

	actual := make(map[string]bool, len(actualTools))
	for _, t := range actualTools {
		actual[strings.TrimSpace(t)] = true
	}

	var declaredNotUsed, usedNotDeclared []string

	for _, t := range claimedTools {
		t = strings.TrimSpace(t)
		if !actual[t] {
			declaredNotUsed = append(declaredNotUsed, t)
		}
	}

	for _, t := range actualTools {
		t = strings.TrimSpace(t)
		if !claimed[t] {
			usedNotDeclared = append(usedNotDeclared, t)
		}
	}

	result := ToolDeclResult{Passed: true}

	if len(declaredNotUsed) > 0 && len(actualTools) == 0 {
		// AI declared tools but called none — inject reminder
		result.Passed = false
		result.DeclaredNotUsed = declaredNotUsed
		result.Message = fmt.Sprintf("请执行声明的工具: %s", strings.Join(declaredNotUsed, ", "))
	}

	if len(usedNotDeclared) > 0 {
		// AI called tools without declaring — warning
		result.Passed = false
		result.UsedNotDeclared = usedNotDeclared
		result.Message = fmt.Sprintf("警告: 调用了未声明的工具: %s (将计入谎报计数)",
			strings.Join(usedNotDeclared, ", "))
	}

	// Empty both: check if X didn't grow (handled by caller with oscillation detection)
	if len(claimedTools) == 0 && len(actualTools) == 0 {
		result.Passed = true // Silent round, handled upstream
	}

	return result
}

// ── Closed Loop Validator ─────────────────────────────────────

// CheckClosedLoop validates that the trajectory returns to near-origin when T=true.
func CheckClosedLoop(start, current Coordinate, constraint Constraint) (bool, float64) {
	if !constraint.T {
		return true, 0 // Closed loop not required
	}

	dist := Distance(start, current)
	if dist <= ClosedLoopEpsilon {
		return true, dist
	}
	return false, dist
}
