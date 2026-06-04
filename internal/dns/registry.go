package dns

import (
	"fmt"
	"sync"

	"github.com/Yunqingqingxi/yunxi-home/internal/dns/base"
)

// Registry manages multiple DNS providers and routes operations to the correct one.
// Different domain records can use different DNS providers (e.g., Aliyun for .cn, Cloudflare for .com).
type Registry struct {
	mu        sync.RWMutex
	providers map[string]base.Provider // provider name → provider instance
	default_  string                   // default provider name
}

// NewRegistry creates a DNS provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]base.Provider),
	}
}

// Register adds a DNS provider to the registry.
// The provider name (e.g., "aliyun", "cloudflare") is used for per-record routing.
func (r *Registry) Register(name string, p base.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
	if r.default_ == "" {
		r.default_ = name
	}
}

// SetDefault sets the default provider name used when a record doesn't specify one.
func (r *Registry) SetDefault(name string) error {
	r.mu.RLock()
	_, ok := r.providers[name]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown DNS provider: %s", name)
	}
	r.mu.Lock()
	r.default_ = name
	r.mu.Unlock()
	return nil
}

// Get returns a provider by name. Returns the default provider if name is empty.
func (r *Registry) Get(name string) (base.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		name = r.default_
	}
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("DNS provider not found: %s (available: %v)", name, r.listLocked())
	}
	return p, nil
}

// Default returns the default provider.
func (r *Registry) Default() (base.Provider, error) {
	return r.Get("")
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.listLocked()
}

// IsConfigured returns true if at least one provider is registered.
func (r *Registry) IsConfigured() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers) > 0
}

func (r *Registry) listLocked() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Remove removes a registered provider by name.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
	if r.default_ == name {
		r.default_ = ""
		// Pick a new default
		for n := range r.providers {
			r.default_ = n
			break
		}
	}
}

// ReplaceAll atomically replaces all registered providers.
func (r *Registry) ReplaceAll(providers map[string]base.Provider, defaultName string) error {
	if _, ok := providers[defaultName]; !ok && defaultName != "" {
		return fmt.Errorf("default provider %s not in provider map", defaultName)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = make(map[string]base.Provider, len(providers))
	for name, p := range providers {
		r.providers[name] = p
	}
	if defaultName != "" {
		r.default_ = defaultName
	} else if len(providers) > 0 {
		for name := range providers {
			r.default_ = name
			break
		}
	}
	return nil
}
