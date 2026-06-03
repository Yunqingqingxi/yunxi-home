package adapt

import (
	"context"
	"fmt"
	"math"
	"sort"
)

type Learner struct {
	repo       Repository
	profileMgr *ProfileManager
}

func NewLearner(repo Repository, profileMgr *ProfileManager) *Learner {
	return &Learner{repo: repo, profileMgr: profileMgr}
}

func (l *Learner) ToolSuccessRate(ctx context.Context, userID, toolName string) float64 {
	outcomes, err := l.repo.GetRecentToolOutcomes(ctx, userID, 50)
	if err != nil || len(outcomes) == 0 {
		return 0.5
	}
	var total, success int
	for _, o := range outcomes {
		if o.ToolName == toolName {
			total++
			if o.Success { success++ }
		}
	}
	if total == 0 { return 0.5 }
	return float64(success) / float64(total)
}

func (l *Learner) ToolFalsePositiveRate(ctx context.Context, userID, toolName string) float64 {
	outcomes, err := l.repo.GetRecentToolOutcomes(ctx, userID, 100)
	if err != nil || len(outcomes) == 0 { return 0 }
	var topoRejected, actuallySucceeded int
	for _, o := range outcomes {
		if o.ToolName == toolName && o.TopoRejected {
			topoRejected++
			if o.Success { actuallySucceeded++ }
		}
	}
	if topoRejected == 0 { return 0 }
	return float64(actuallySucceeded) / float64(topoRejected)
}

func (l *Learner) RecommendedTolerance(ctx context.Context, userID, toolName string) float64 {
	fpr := l.ToolFalsePositiveRate(ctx, userID, toolName)
	if fpr > 0.5 { return math.Min(2.0, 1.0+fpr) }
	if fpr > 0.3 { return 1.0 + fpr*0.5 }
	return 1.0
}

type ToolPair struct {
	Prev        string  `json:"prev"`
	Next        string  `json:"next"`
	Count       int     `json:"count"`
	SuccessRate float64 `json:"success_rate"`
}

func (l *Learner) CommonToolSequence(ctx context.Context, userID, taskCategory string) []ToolPair {
	outcomes, _ := l.repo.GetRecentToolOutcomes(ctx, userID, 200)
	if len(outcomes) < 3 { return nil }
	type pair struct{ prev, next string }
	counts := make(map[pair]int)
	successes := make(map[pair]int)
	for i := 1; i < len(outcomes); i++ {
		if outcomes[i-1].SessionID == outcomes[i].SessionID {
			p := pair{outcomes[i-1].ToolName, outcomes[i].ToolName}
			counts[p]++
			if outcomes[i].Success { successes[p]++ }
		}
	}
	var result []ToolPair
	for p, count := range counts {
		if count >= 2 {
			rate := float64(successes[p]) / float64(count)
			if rate >= 0.5 {
				result = append(result, ToolPair{Prev: p.prev, Next: p.next, Count: count, SuccessRate: rate})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	return result
}

func (l *Learner) TrustCalibration(ctx context.Context, userID string) (int, bool) {
	summaries, err := l.repo.GetSessionSummaries(ctx, userID, 20)
	if err != nil || len(summaries) < 3 { return 3, false }
	lockedSessions := 0
	for _, s := range summaries {
		if s.TrustLocked { lockedSessions++ }
	}
	lockRate := float64(lockedSessions) / float64(len(summaries))
	if lockRate > 0.4 { return 5, true }
	if lockRate > 0.2 { return 4, true }
	return 3, false
}

type AnomalyReport struct {
	HasAnomaly   bool     `json:"has_anomaly"`
	RoundSpike   bool     `json:"round_spike"`
	RejectSpike  bool     `json:"reject_spike"`
	FailureSpike bool     `json:"failure_spike"`
	Details      []string `json:"details"`
}

func (l *Learner) AnomalyCheck(ctx context.Context, userID string, latest *SessionSummary) *AnomalyReport {
	summaries, err := l.repo.GetSessionSummaries(ctx, userID, 20)
	if err != nil || len(summaries) < 5 { return nil }
	var avgRounds, avgFailures, avgRejects float64
	for _, s := range summaries {
		avgRounds += float64(s.Rounds)
		avgFailures += float64(s.ToolFailures)
		avgRejects += float64(s.TopoRejects)
	}
	n := float64(len(summaries))
	avgRounds /= n; avgFailures /= n; avgRejects /= n

	report := &AnomalyReport{}
	if float64(latest.Rounds) > avgRounds*2.5 && avgRounds > 2 {
		report.RoundSpike = true
		report.Details = append(report.Details, "Session rounds unusually high")
	}
	if float64(latest.TopoRejects) > avgRejects*3 && avgRejects > 0 {
		report.RejectSpike = true
		report.Details = append(report.Details, "Topology rejection spike")
	}
	if float64(latest.ToolFailures) > avgFailures*3 && avgFailures > 1 {
		report.FailureSpike = true
		report.Details = append(report.Details, "Tool failure spike")
	}
	if len(report.Details) > 0 { report.HasAnomaly = true; return report }
	return nil
}

func (l *Learner) WeeklySummary(ctx context.Context, userID string) string {
	summaries, err := l.repo.GetSessionSummaries(ctx, userID, 50)
	if err != nil || len(summaries) < 3 { return "" }
	recent := summaries
	if len(recent) > 10 { recent = recent[:10] }
	older := summaries
	if len(older) > 10 { older = older[len(older)-10:] }
	recentSuccess := avgSuccess(recent)
	olderSuccess := avgSuccess(older)

	var sb string
	if recentSuccess > olderSuccess+0.1 {
		sb += fmt.Sprintf("- Success rate: %.0f%% -> %.0f%%\n", olderSuccess*100, recentSuccess*100)
	}
	recentRejects := avgRejects(recent)
	olderRejects := avgRejects(older)
	if recentRejects < olderRejects && olderRejects > 2 {
		sb += "- Topology rejections decreasing\n"
	}
	p, _ := l.profileMgr.GetOrCreate(ctx, userID)
	if p != nil && p.SessionCount > 5 && p.CorrectionCount > 0 {
		sb += fmt.Sprintf("- %d corrections received\n", p.CorrectionCount)
	}
	return sb
}

func avgSuccess(ss []SessionSummary) float64 {
	if len(ss) == 0 { return 0 }
	var c int
	for _, s := range ss { if s.Completed { c++ } }
	return float64(c) / float64(len(ss))
}

func avgRejects(ss []SessionSummary) float64 {
	if len(ss) == 0 { return 0 }
	var total int
	for _, s := range ss { total += s.TopoRejects }
	return float64(total) / float64(len(ss))
}
