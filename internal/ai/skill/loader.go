package skill

import (
	"fmt"

	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Loader loads Skill definitions from a directory tree.
// Supports two formats:
//   - Subdirectory format: skills/<name>/SKILL.md with YAML frontmatter
//   - Flat file format: skills/<name>.yaml or skills/<name>.md
type Loader struct {
	mu      sync.RWMutex
	skills  map[string]*Manifest // name → Manifest
	dirPath string
}

// NewLoader creates a Loader and loads all skills from dirPath.
func NewLoader(dirPath string) (*Loader, error) {
	l := &Loader{
		skills:  make(map[string]*Manifest),
		dirPath: dirPath,
	}
	if err := l.reload(); err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	return l, nil
}

// Get returns a single Skill by name.
func (l *Loader) Get(name string) *Manifest {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.skills[name]
}

// All returns all loaded skill names.
func (l *Loader) All() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	names := make([]string, 0, len(l.skills))
	for name := range l.skills {
		names = append(names, name)
	}
	return names
}

// Reload re-scans the directory and replaces all loaded skills.
func (l *Loader) Reload() error {
	return l.reload()
}

func (l *Loader) reload() error {
	entries, err := os.ReadDir(l.dirPath)
	if err != nil {
		return fmt.Errorf("read skill dir (%s): %w", l.dirPath, err)
	}

	newSkills := make(map[string]*Manifest)

	for _, entry := range entries {
		if entry.Name() == ".system" {
			continue // skip system internals
		}

		if entry.IsDir() {
			// subdirectory format: <name>/SKILL.md
			l.loadSkillDir(entry.Name(), newSkills)
		} else {
			// Flat file format: <name>.yaml or <name>.md
			l.loadFlatFile(entry.Name(), newSkills)
		}
	}

	l.mu.Lock()
	l.skills = newSkills
	l.mu.Unlock()

	log.Info("skills loaded", "count", len(newSkills), "dir", l.dirPath)
	return nil
}

// loadSkillDir loads a skill from a subdirectory containing SKILL.md.
func (l *Loader) loadSkillDir(dirName string, skills map[string]*Manifest) {
	dirPath := filepath.Join(l.dirPath, dirName)

	// Look for SKILL.md or skill.yaml
	candidates := []string{"SKILL.md", "skill.yaml", "skill.md", "SKILL.yaml"}
	var skillFile string
	for _, name := range candidates {
		p := filepath.Join(dirPath, name)
		if _, err := os.Stat(p); err == nil {
			skillFile = p
			break
		}
	}
	if skillFile == "" {
		log.Debug("no SKILL.md found in skill dir", "dir", dirPath)
		return
	}

	data, err := os.ReadFile(skillFile)
	if err != nil {
		log.Warn("failed to read skill file", "path", skillFile, "error", err)
		return
	}

	content := string(data)
	m := parseSkill(content, dirName)

	if m.Name == "" {
		log.Warn("skill missing name", "dir", dirPath)
		return
	}

	skills[m.Name] = &m
}

// loadFlatFile loads a skill from a flat .yaml or .md file.
func (l *Loader) loadFlatFile(filename string, skills map[string]*Manifest) {
	ext := filepath.Ext(filename)
	if ext != ".yaml" && ext != ".md" {
		return
	}

	fullPath := filepath.Join(l.dirPath, filename)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		log.Warn("failed to read skill file", "path", fullPath, "error", err)
		return
	}

	m := parseSkill(string(data), strings.TrimSuffix(filename, ext))

	if m.Name == "" {
		log.Warn("skill missing name", "path", fullPath)
		return
	}

	skills[m.Name] = &m
}

// parseSkill parses skill content, detecting format (frontmatter+MD, pure MD, YAML).
func parseSkill(content, fallbackName string) Manifest {
	// Try frontmatter + Markdown first (subdirectory format: ---\nkey: val\n---\n# Body)
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		if m, ok := parseFrontmatter(content, fallbackName); ok {
			return m
		}
	}

	// Try YAML
	var m Manifest
	if err := yaml.Unmarshal([]byte(content), &m); err == nil && m.Name != "" && len(m.Steps) > 0 {
		return m
	}

	// Fallback: plain Markdown
	return parseMarkdownSkill(content, fallbackName)
}

// parseFrontmatter extracts YAML frontmatter from a Markdown file.
func parseFrontmatter(content, fallbackName string) (Manifest, bool) {
	// Find frontmatter delimiters
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return Manifest{}, false
	}
	rest := strings.TrimPrefix(strings.TrimSpace(content), "---")
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return Manifest{}, false
	}
	fmRaw := rest[:idx]
	bodyRaw := strings.TrimSpace(rest[idx+3:])

	// Parse frontmatter YAML
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return Manifest{}, false
	}

	name := fm.Name
	if name == "" {
		name = fallbackName
	}
	description := fm.Description
	if description == "" {
		description = extractFirstLine(bodyRaw)
	}

	return Manifest{
		Name:        name,
		Description: truncate(description, 200),
		Category:    "general",
		RiskLevel:   "readonly",
		Steps: []StepDef{{
			ID:      1,
			Tool:    "system",
			Args:    map[string]any{"skill_body": bodyRaw},
			Purpose: description,
		}},
	}, true
}

// parseMarkdownSkill converts a plain Markdown skill file into a Manifest.
func parseMarkdownSkill(content, fallbackName string) Manifest {
	lines := strings.Split(content, "\n")
	name := fallbackName
	description := ""
	var bodyLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// First # heading as name
		if strings.HasPrefix(trimmed, "# ") && name == fallbackName {
			name = strings.TrimPrefix(trimmed, "# ")
			continue
		}
		// First non-empty non-heading line as description
		if !strings.HasPrefix(trimmed, "#") && trimmed != "" && description == "" {
			description = truncate(trimmed, 200)
		}
		if !strings.HasPrefix(trimmed, "#") && trimmed != "" {
			bodyLines = append(bodyLines, trimmed)
		}
	}

	if description == "" {
		description = name
	}

	return Manifest{
		Name:        name,
		Description: description,
		Category:    "general",
		RiskLevel:   "readonly",
		Steps: []StepDef{{
			ID:      1,
			Tool:    "system",
			Args:    map[string]any{"skill_body": strings.Join(bodyLines, "\n")},
			Purpose: description,
		}},
	}
}

func extractFirstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return truncate(trimmed, 200)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
