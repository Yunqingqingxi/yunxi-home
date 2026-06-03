package adapt

import (
	"context"
	"math"
	"sort"
)

// PromptSelector selects the best prompt variant for a given user and situation.
type PromptSelector struct {
	repo       Repository
	profileMgr *ProfileManager
}

func NewPromptSelector(repo Repository, profileMgr *ProfileManager) *PromptSelector {
	return &PromptSelector{repo: repo, profileMgr: profileMgr}
}

// SelectVariant picks the most effective prompt variant for this user.
// Returns the variant name ("concise", "detailed", "default") and a score.
func (ps *PromptSelector) SelectVariant(ctx context.Context, userID, promptID string) string {
	p, _ := ps.profileMgr.GetOrCreate(ctx, userID)

	// If user has an explicit verbosity preference, use it
	if p != nil && p.Verbosity != VerbosityDefault {
		return string(p.Verbosity)
	}

	// Otherwise, look at historical prompt effectiveness for default variant
	pe, err := ps.repo.GetPromptEffectiveness(ctx, promptID, "default")
	if err != nil || pe == nil {
		return "default"
	}

	// Use variant with best success-to-edit ratio
	if pe.UseCount < 3 {
		return "default" // not enough data
	}

	// Compute score: success rate weighted by use count
	score := float64(pe.SuccessCount) / float64(pe.UseCount)
	if score < 0.3 && pe.CancelCount > pe.SuccessCount {
		return "detailed" // fall back to more detail when concise fails
	}

	return pe.Variant
}

// RecordPromptUse records that a prompt was used in a session.
func (ps *PromptSelector) RecordPromptUse(ctx context.Context, promptID, variant string) {
	if variant == "" {
		variant = "default"
	}
	pe, _ := ps.repo.GetPromptEffectiveness(ctx, promptID, variant)
	if pe == nil {
		pe = &PromptEffectiveness{
			PromptID: promptID,
			Variant:  variant,
		}
	}
	pe.UseCount++
	_ = ps.repo.UpsertPromptEffectiveness(ctx, pe)
}

// RecordPromptOutcome updates effectiveness after session completes.
func (ps *PromptSelector) RecordPromptOutcome(ctx context.Context, promptID, variant string, success, edited, cancelled bool, rounds int) {
	if variant == "" {
		variant = "default"
	}
	pe, _ := ps.repo.GetPromptEffectiveness(ctx, promptID, variant)
	if pe == nil {
		pe = &PromptEffectiveness{
			PromptID: promptID,
			Variant:  variant,
		}
	}

	pe.UseCount++
	if success {
		pe.SuccessCount++
	}
	if edited {
		pe.EditCount++
	}
	if cancelled {
		pe.CancelCount++
	}

	n := float64(pe.UseCount)
	pe.AvgRounds = (pe.AvgRounds*(n-1) + float64(rounds)) / n

	_ = ps.repo.UpsertPromptEffectiveness(ctx, pe)
}

// RankSpecialized sorts specialized prompts by effectiveness for this user,
// putting the most successful ones first.
func (ps *PromptSelector) RankSpecialized(ctx context.Context) []PromptEffectiveness {
	all, err := ps.repo.ListPromptEffectiveness(ctx)
	if err != nil {
		return nil
	}
	sort.Slice(all, func(i, j int) bool {
		si := scorePrompt(&all[i])
		sj := scorePrompt(&all[j])
		return si > sj
	})
	return all
}

func scorePrompt(pe *PromptEffectiveness) float64 {
	if pe.UseCount == 0 {
		return 0
	}
	successRate := float64(pe.SuccessCount) / float64(pe.UseCount)
	penalty := float64(pe.CancelCount+pe.EditCount) / float64(pe.UseCount)
	// Bayesian prior: start with 0.5 for low-count prompts
	confidence := 1.0 - math.Exp(-float64(pe.UseCount)/5.0)
	return successRate*(1-penalty)*confidence + 0.5*(1-confidence)
}
