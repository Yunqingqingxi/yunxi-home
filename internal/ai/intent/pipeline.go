package intent

import (
	"context"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// RouteResult is the outcome of the two-stage intent pipeline.
type RouteResult struct {
	Tool     string  // matched tool name
	Stage    string  // "rule", "llm", or "none"
	Strength float64 // confidence 0.0-1.0
	Pattern  string  // matching rule pattern (rule stage only)
}

// Pipeline is a two-stage intent router.
// Stage 1: exact rule triggers (zero latency, ~70 rules).
// Stage 2: LLM classification (300-500ms, only when rules miss).
type Pipeline struct {
	rules      []IntentRule
	classifier *Classifier
}

// NewPipeline creates a new intent pipeline.
// Pass classifier=nil to disable Stage 2 (rules-only mode).
func NewPipeline(classifier *Classifier) *Pipeline {
	return &Pipeline{
		rules:      defaultRules(),
		classifier: classifier,
	}
}

// Route runs the two-stage pipeline against a user message.
func (p *Pipeline) Route(ctx context.Context, userMsg string) RouteResult {
	// Stage 1: Exact rule triggers (zero latency)
	if result := matchRules(userMsg, p.rules); result != nil {
		log.Debug("意图路由(规则)", "工具", result.Tool,
			"置信度", result.Strength, "模式", result.Pattern)
		return RouteResult{
			Tool:     result.Tool,
			Stage:    "rule",
			Strength: result.Strength,
			Pattern:  result.Pattern,
		}
	}

	// Stage 2: LLM classification (300-500ms, only when rules miss)
	if p.classifier != nil {
		tool := p.classifier.Classify(ctx, userMsg)
		if tool != "" && tool != "NONE" {
			log.Debug("意图路由(LLM)", "工具", tool)
			return RouteResult{
				Tool:     tool,
				Stage:    "llm",
				Strength: 0.75,
			}
		}
	}

	return RouteResult{Stage: "none"}
}

// ToolNamesForClassifier builds the tool name list needed for the classifier prompt.
// Extracts Name + Description from registered tool definitions.
func ToolNamesForClassifier(defs []base.ToolDef) []base.ToolDef {
	return defs
}
