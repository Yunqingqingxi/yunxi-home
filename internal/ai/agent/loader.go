package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentDef represents a parsed agent definition from YAML frontmatter.
type AgentDef struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Role        string            `yaml:"role" json:"role"`
	Model       string            `yaml:"model" json:"model"`
	Color       string            `yaml:"color" json:"color"`
	Tools       []string          `yaml:"tools" json:"tools"`
	Categories  []string          `yaml:"categories" json:"categories"`
	Risk        string            `yaml:"risk" json:"risk"`
	MaxRounds   int               `yaml:"max_rounds" json:"max_rounds"`
	Timeout     string            `yaml:"timeout" json:"timeout"`
	Background  bool              `yaml:"background" json:"background"`
	Extra       map[string]string `yaml:"extra" json:"extra,omitempty"`

	// SystemPrompt is the markdown content after the YAML frontmatter.
	SystemPrompt string `yaml:"-" json:"system_prompt"`

	// Source is the file path this definition was loaded from.
	Source string `yaml:"-" json:"source"`
}

// ParseTimeout parses the timeout string like "10m", "5m", "30s" into a Duration.
func (d *AgentDef) ParseTimeout() time.Duration {
	if d.Timeout == "" {
		return 10 * time.Minute
	}
	t, err := time.ParseDuration(d.Timeout)
	if err != nil {
		return 10 * time.Minute
	}
	return t
}

// EffectiveMaxRounds returns MaxRounds or a default of 50.
func (d *AgentDef) EffectiveMaxRounds() int {
	if d.MaxRounds <= 0 {
		return 50
	}
	return d.MaxRounds
}

// AgentLoader loads agent definitions from .md files with YAML frontmatter.
type AgentLoader struct {
	mu    sync.RWMutex
	defs  map[string]*AgentDef // name -> definition
	paths []string             // search directories
}

// NewAgentLoader creates an AgentLoader that scans the given directories
// for .md files with YAML frontmatter. Directories that don't exist are
// silently skipped.
func NewAgentLoader(dirs ...string) *AgentLoader {
	l := &AgentLoader{
		defs:  make(map[string]*AgentDef),
		paths: dirs,
	}
	l.Reload()
	return l
}

// Reload rescans all configured directories and reloads agent definitions.
func (l *AgentLoader) Reload() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.defs = make(map[string]*AgentDef)
	for _, dir := range l.paths {
		l.scanDir(dir)
	}
}

// Get returns the agent definition for the given name, or nil.
func (l *AgentLoader) Get(name string) *AgentDef {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.defs[name]
}

// All returns all loaded agent definitions.
func (l *AgentLoader) All() []*AgentDef {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*AgentDef, 0, len(l.defs))
	for _, d := range l.defs {
		result = append(result, d)
	}
	return result
}

// Names returns all loaded agent names.
func (l *AgentLoader) Names() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	names := make([]string, 0, len(l.defs))
	for name := range l.defs {
		names = append(names, name)
	}
	return names
}

// Count returns the number of loaded agent definitions.
func (l *AgentLoader) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.defs)
}

// scanDir scans a directory for .md files and parses them.
func (l *AgentLoader) scanDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // silently skip missing directories
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		def, err := parseAgentFile(path)
		if err != nil {
			continue // skip malformed files
		}
		if def.Name == "" {
			continue
		}
		// Don't overwrite if already loaded from a higher-priority dir
		if _, exists := l.defs[def.Name]; !exists {
			l.defs[def.Name] = def
		}
	}
}

// parseAgentFile parses a single .md file with YAML frontmatter.
// Format:
//
//	---
//	name: agent-name
//	description: ...
//	tools: [...]
//	---
//	System prompt content here...
func parseAgentFile(path string) (*AgentDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Extract YAML frontmatter between --- delimiters
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r") {
		return nil, fmt.Errorf("no YAML frontmatter in %s", path)
	}

	// Find the closing ---
	rest := content[3:] // skip opening ---
	// Handle both \n and \r\n
	rest = strings.TrimPrefix(rest, "\n")
	rest = strings.TrimPrefix(rest, "\r\n")

	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return nil, fmt.Errorf("unclosed YAML frontmatter in %s", path)
	}

	yamlStr := rest[:endIdx]
	// System prompt is everything after the closing ---
	systemPrompt := ""
	afterFront := rest[endIdx+4:] // skip "\n---"
	afterFront = strings.TrimPrefix(afterFront, "\n")
	afterFront = strings.TrimPrefix(afterFront, "\r\n")
	systemPrompt = strings.TrimSpace(afterFront)

	var def AgentDef
	if err := yaml.Unmarshal([]byte(yamlStr), &def); err != nil {
		return nil, fmt.Errorf("bad YAML in %s: %w", path, err)
	}

	def.SystemPrompt = systemPrompt
	def.Source = path
	return &def, nil
}

// SpawnTask converts an AgentDef into a SpawnTask suitable for Manager.Spawn.
func (d *AgentDef) ToSpawnTask(goalOverride string) SpawnTask {
	goal := d.SystemPrompt
	if goalOverride != "" {
		goal = goalOverride + "\n\n" + goal
	}
	if goal == "" {
		goal = d.Description
	}
	return SpawnTask{
		Goal:       goal,
		ToolFilter: d.Tools,
	}
}
