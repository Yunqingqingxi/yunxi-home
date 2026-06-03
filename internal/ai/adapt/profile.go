package adapt

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ProfileManager manages user profiles with in-memory caching.
type ProfileManager struct {
	repo  Repository
	cache map[string]*UserProfile
	mu    sync.RWMutex
}

func NewProfileManager(repo Repository) *ProfileManager {
	return &ProfileManager{repo: repo, cache: make(map[string]*UserProfile)}
}

// GetOrCreate returns the user profile, creating a default one if absent.
func (pm *ProfileManager) GetOrCreate(ctx context.Context, userID string) (*UserProfile, error) {
	pm.mu.RLock()
	if p, ok := pm.cache[userID]; ok {
		pm.mu.RUnlock()
		return p, nil
	}
	pm.mu.RUnlock()

	p, err := pm.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		now := time.Now().UTC()
		p = &UserProfile{
			UserID:       userID,
			ToolSuccess:  make(map[string]float64),
			CreatedAt:    now,
			UpdatedAt:    now,
			LastActiveAt: now,
		}
		if err := pm.repo.UpsertProfile(ctx, p); err != nil {
			return nil, err
		}
	}

	pm.mu.Lock()
	pm.cache[userID] = p
	pm.mu.Unlock()

	return p, nil
}

// RecordSessionEnd updates the profile after a session completes.
func (pm *ProfileManager) RecordSessionEnd(ctx context.Context, summary *SessionSummary) error {
	p, err := pm.GetOrCreate(ctx, summary.UserID)
	if err != nil {
		return err
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	p.SessionCount++
	p.LastActiveAt = time.Now().UTC()

	// Update tool success rates with exponential moving average
	alpha := 0.3 // weight for new data
	for _, toolName := range summary.uniqueTools() {
		oldRate := p.ToolSuccess[toolName]
		newRate := alpha*summary.toolSuccessRate(toolName) + (1-alpha)*oldRate
		p.ToolSuccess[toolName] = clampRate(newRate)
	}

	// Update task patterns
	if summary.TaskCategory != "" {
		pm.updateTaskPatterns(p, summary)
	}

	p.UpdatedAt = time.Now().UTC()
	return pm.repo.UpsertProfile(ctx, p)
}

// RecordFeedback processes a feedback event and updates the profile.
func (pm *ProfileManager) RecordFeedback(ctx context.Context, ev *FeedbackEvent) error {
	p, err := pm.GetOrCreate(ctx, ev.UserID)
	if err != nil {
		return err
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	switch ev.Type {
	case FeedbackCorrection, FeedbackEdited:
		p.CorrectionCount++
		// Shift verbosity based on corrections
		if p.CorrectionCount >= 3 && p.Verbosity == VerbosityDefault {
			// User frequently corrects - they likely want more detailed output
			p.Verbosity = VerbosityDetailed
		}
	case FeedbackCancelled:
		p.CancelCount++
	}

	p.UpdatedAt = time.Now().UTC()
	return pm.repo.UpsertProfile(ctx, p)
}

// Summarize returns a compact summary of the profile for injecting into system prompt.
func (pm *ProfileManager) Summarize(userID string) string {
	pm.mu.RLock()
	p := pm.cache[userID]
	pm.mu.RUnlock()
	if p == nil || p.SessionCount == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## User Profile (learned over time)\n")
	sb.WriteString(fmt.Sprintf("- Sessions completed: %d\n", p.SessionCount))

	if p.Verbosity != VerbosityDefault {
		sb.WriteString(fmt.Sprintf("- Preferred style: %s\n", p.Verbosity))
	}

	if len(p.TopTools) > 0 {
		top := p.TopTools
		if len(top) > 5 {
			top = top[:5]
		}
		sb.WriteString(fmt.Sprintf("- Most used tools: %s\n", strings.Join(top, ", ")))
	}

	if len(p.TaskPatterns) > 0 {
		sb.WriteString("- Common tasks:\n")
		for _, tp := range p.TaskPatterns {
			if tp.Count >= 2 {
				sb.WriteString(fmt.Sprintf("  - %s (x%d, %.0f%% success)\n",
					tp.Category, tp.Count, tp.SuccessRate*100))
			}
		}
	}

	if p.CorrectionCount > 0 {
		sb.WriteString(fmt.Sprintf("- User has corrected AI %d times - be more careful\n", p.CorrectionCount))
	}

	return sb.String()
}

// InferTaskCategory guesses the task category from the user message.
func (pm *ProfileManager) InferTaskCategory(userID, message string) string {
	lower := strings.ToLower(message)

	categories := map[string][]string{
		"code_fix":    {"fix", "bug", "error", "broken", "doesn't work", "不工作", "出错", "修复", "修"},
		"code_review": {"review", "检查", "审查", "代码审查"},
		"deploy":      {"deploy", "部署", "release", "发布"},
		"file_manage": {"file", "文件", "目录", "directory", "list", "show"},
		"config":      {"config", "配置", "setting", "设置"},
		"analysis":    {"analyze", "分析", "check", "look at", "看看"},
		"build":       {"build", "compile", "编译", "构建"},
		"query":       {"what", "how", "why", "when", "什么", "怎么", "为什么"},
	}

	best := ""
	bestScore := 0
	for cat, keywords := range categories {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = cat
		}
	}
	if bestScore >= 2 {
		return best
	}
	return ""
}

func (pm *ProfileManager) updateTaskPatterns(p *UserProfile, summary *SessionSummary) {
	for i, tp := range p.TaskPatterns {
		if tp.Category == summary.TaskCategory {
			n := float64(tp.Count + 1)
			p.TaskPatterns[i].SuccessRate = (tp.SuccessRate*float64(tp.Count) + boolToFloat(summary.Completed)) / n
			p.TaskPatterns[i].Count++
			p.TaskPatterns[i].Tools = mergeTools(tp.Tools, summary.uniqueTools())
			return
		}
	}
	// New pattern
	p.TaskPatterns = append(p.TaskPatterns, TaskPattern{
		Category:    summary.TaskCategory,
		Tools:       summary.uniqueTools(),
		SuccessRate: boolToFloat(summary.Completed),
		Count:       1,
	})
	if len(p.TaskPatterns) > 20 {
		p.TaskPatterns = p.TaskPatterns[len(p.TaskPatterns)-20:]
	}
}

func (pm *ProfileManager) UpdateTopTools(p *UserProfile, toolCounts map[string]int) {
	type tc struct {
		name  string
		count int
	}
	var sorted []tc
	for name, count := range toolCounts {
		sorted = append(sorted, tc{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	p.TopTools = make([]string, 0, len(sorted))
	for _, s := range sorted {
		p.TopTools = append(p.TopTools, s.name)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *SessionSummary) uniqueTools() []string {
	// Stub - actual tool list not stored in summary, filled by caller
	return nil
}

func (s *SessionSummary) toolSuccessRate(toolName string) float64 {
	total := s.ToolSuccesses + s.ToolFailures
	if total == 0 {
		return 0.5
	}
	return float64(s.ToolSuccesses) / float64(total)
}

func clampRate(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func mergeTools(existing, new []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range existing {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	for _, t := range new {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// Ensure json import is used
var _ = json.Marshal
