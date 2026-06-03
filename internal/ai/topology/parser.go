package topology

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ── Topology Tag Parser ───────────────────────────────────────
//
// Parses <topology x="3.5" y="0.6" z="1.2" tools="file_write,run_command" ack="constraint_updated" />
// from the end of AI output.

var (
	topologyTagRe = regexp.MustCompile(`<topology\s+([^>]*?)\s*/>`)
	attrRe        = regexp.MustCompile(`(\w+)="([^"]*)"`)
)

// StripTopologyTag removes all <topology ... /> tags from text.
// Used to prevent the raw tag from leaking into frontend display.
func StripTopologyTag(text string) string {
	return topologyTagRe.ReplaceAllString(text, "")
}

// ParseTopology extracts topology coordinates and metadata from AI output text.
// Returns a ParseResult with Parsed=false if no tag is found.
func ParseTopology(text string) ParseResult {
	result := ParseResult{}

	// Find the last topology tag (there should only be one)
	matches := topologyTagRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return result
	}

	// Use the last match
	attrs := matches[len(matches)-1][1]

	// Parse attributes
	attrMap := parseAttributes(attrs)

	// Parse coordinates
	if xStr, ok := attrMap["x"]; ok {
		if x, err := strconv.ParseFloat(xStr, 64); err == nil {
			result.Coord.X = clampFloat(x, 0, 10000) // 累积制，AI自评分
		}
	}
	if yStr, ok := attrMap["y"]; ok {
		if y, err := strconv.ParseFloat(yStr, 64); err == nil {
			result.Coord.Y = clampFloat(y, -1.0, 1.0)
		}
	}
	if zStr, ok := attrMap["z"]; ok {
		if z, err := strconv.ParseFloat(zStr, 64); err == nil {
			result.Coord.Z = clampFloat(z, 0, 100) // Z cap is generous, validator will enforce R
		}
	}

	// Parse tools
	if toolsStr, ok := attrMap["tools"]; ok && toolsStr != "" {
		tools := strings.Split(toolsStr, ",")
		for _, t := range tools {
			t = strings.TrimSpace(t)
			if t != "" {
				result.Tools = append(result.Tools, t)
			}
		}
	}

	// Parse ack
	if ack, ok := attrMap["ack"]; ok && ack == "constraint_updated" {
		result.AckUpdate = true
	}

	result.Parsed = true
	return result
}

// parseAttributes extracts key="value" pairs from the attributes string.
func parseAttributes(attrs string) map[string]string {
	result := make(map[string]string)
	matches := attrRe.FindAllStringSubmatch(attrs, -1)
	for _, m := range matches {
		if len(m) == 3 {
			result[m[1]] = m[2]
		}
	}
	return result
}

// clampFloat clamps a float64 value to [min, max].
func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ── Coordinate Math ───────────────────────────────────────────

// Distance computes Euclidean distance between two coordinates.
func Distance(a, b Coordinate) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// DeltaY computes the Y-axis change between two coordinates.
func DeltaY(prev, curr Coordinate) float64 {
	return math.Abs(curr.Y - prev.Y)
}

// IsAtOrigin checks if a coordinate is at or near the origin (0,0,0).
func IsAtOrigin(c Coordinate, epsilon float64) bool {
	return Distance(c, Coordinate{}) <= epsilon
}

// ClampCoordinate ensures all coordinate values are within valid ranges.
func ClampCoordinate(c Coordinate) Coordinate {
	return Coordinate{
		X: clampFloat(c.X, 0, 10),
		Y: clampFloat(c.Y, -1.0, 1.0),
		Z: clampFloat(c.Z, 0, 100),
	}
}
