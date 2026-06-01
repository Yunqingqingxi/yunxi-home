package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// Registry manages multiple AI providers and dispatches requests by model name.
type Registry struct {
	providers    []entry
	defaultModel string
}

type entry struct {
	name     string
	models   []string
	provider base.AIProvider
}

func New() *Registry {
	return &Registry{}
}

// Register adds a provider. It uses the provider's own Models() list.
func (r *Registry) RegisterProvider(p base.AIProvider) {
	models := p.Models()
	name := models[0]
	if idx := strings.Index(name, "-"); idx > 0 {
		name = name[:idx]
	}
	r.providers = append(r.providers, entry{name: name, models: models, provider: p})
}

// ReasoningFor returns the default reasoning depth for a given model.
func (r *Registry) ReasoningFor(model string) string {
	p := r.resolve(model)
	if p != nil {
		return p.DefaultReasoning()
	}
	return "medium"
}

// SetDefaultModel sets the default model name.
func (r *Registry) SetDefaultModel(model string) {
	r.defaultModel = model
}

// DefaultModel returns the configured default model.
func (r *Registry) DefaultModel() string {
	return r.defaultModel
}

// AllModels returns all registered model names across all providers.
func (r *Registry) AllModels() []string {
	var models []string
	for _, e := range r.providers {
		models = append(models, e.models...)
	}
	return models
}

// ReplaceAll atomically replaces all provider entries.
func (r *Registry) ReplaceAll(providers []base.AIProvider) {
	r.providers = nil
	for _, p := range providers {
		models := p.Models()
		name := models[0]
		if idx := strings.Index(name, "-"); idx > 0 {
			name = name[:idx]
		}
		r.providers = append(r.providers, entry{name: name, models: models, provider: p})
	}
}

// IsConfigured returns true if at least one provider is registered.
func (r *Registry) IsConfigured() bool {
	return len(r.providers) > 0
}

// resolve returns the provider for a given model name by prefix matching.
func (r *Registry) resolve(model string) base.AIProvider {
	for _, e := range r.providers {
		for _, m := range e.models {
			if m == model {
				return e.provider
			}
		}
	}
	// Fallback: prefix match (e.g. "deepseek-" → deepseek, "qwen-" → qwen)
	for _, e := range r.providers {
		for _, m := range e.models {
			if idx := strings.Index(m, "-"); idx > 0 {
				prefix := m[:idx]
				if strings.HasPrefix(model, prefix) {
					return e.provider
				}
			}
		}
	}
	return nil
}

// ChatStream implements base.AIProvider by dispatching to the correct provider.
func (r *Registry) ChatStream(ctx context.Context, messages []base.Message, tools []base.ToolDef) (<-chan base.ChatStreamEvent, error) {
	model := r.defaultModel
	if override, ok := ctx.Value(base.ModelOverrideKey{}).(string); ok && override != "" {
		model = override
	}

	p := r.resolve(model)
	if p == nil {
		ch := make(chan base.ChatStreamEvent, 1)
		ch <- base.ChatStreamEvent{Type: "error", Content: fmt.Sprintf("模型 %s 没有对应的 AI 提供者", model)}
		close(ch)
		return ch, nil
	}

	return p.ChatStream(ctx, messages, tools)
}

// TestConnection tests all registered providers by calling each one's TestConnection.
func (r *Registry) TestConnection(ctx context.Context) error {
	for _, e := range r.providers {
		if err := e.provider.TestConnection(ctx); err != nil {
			return fmt.Errorf("%s: %w", e.name, err)
		}
	}
	return nil
}

// Models returns all registered model names (implements AIProvider).
func (r *Registry) Models() []string { return r.AllModels() }

// DefaultReasoning returns a global default reasoning depth (implements AIProvider).
func (r *Registry) DefaultReasoning() string { return "medium" }
