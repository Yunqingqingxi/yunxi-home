package skill

import (
	"fmt"
	"sort"
	"sync"
)

// ── SkillRegistry ───────────────────────────────────────────────────────
// 并发安全的技能注册中心。同时管理 YAML Manifest 和 Go ProgrammaticSkill。

// Registry 技能注册中心
type Registry struct {
	mu              sync.RWMutex
	skills          map[string]ProgrammaticSkill // name → skill 实例
	yamlSkills      map[string]*Manifest         // YAML 声明式技能（兼容旧系统）
}

// NewRegistry 创建技能注册中心
func NewRegistry() *Registry {
	return &Registry{
		skills:     make(map[string]ProgrammaticSkill),
		yamlSkills: make(map[string]*Manifest),
	}
}

// Register 注册一个编程式技能
func (r *Registry) Register(s ProgrammaticSkill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name()] = s
}

// RegisterYAML 注册一个 YAML 声明式技能（兼容旧 Loader）
func (r *Registry) RegisterYAML(m *Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.yamlSkills[m.Name] = m
}

// Get 获取编程式技能
func (r *Registry) Get(name string) (ProgrammaticSkill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// GetYAML 获取 YAML 技能
func (r *Registry) GetYAML(name string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.yamlSkills[name]
	return m, ok
}

// Has 检查技能是否存在（编程式或 YAML）
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.skills[name]
	if ok {
		return true
	}
	_, ok = r.yamlSkills[name]
	return ok
}

// List 返回所有技能名称（编程式优先，再 YAML）
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills)+len(r.yamlSkills))
	for n := range r.skills {
		names = append(names, n)
	}
	for n := range r.yamlSkills {
		if _, ok := r.skills[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// ListProgrammatic 返回所有编程式技能名称
func (r *Registry) ListProgrammatic() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for n := range r.skills {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ListAll 返回所有技能的名称和描述（用于 AI 工具描述）
func (r *Registry) ListAll() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]string, len(r.skills)+len(r.yamlSkills))
	for n, s := range r.skills {
		result[n] = s.Description()
	}
	for n, m := range r.yamlSkills {
		if _, ok := r.skills[n]; !ok {
			result[n] = m.Description
		}
	}
	return result
}

// Summary 返回所有技能的格式化摘要
func (r *Registry) Summary() string {
	all := r.ListAll()
	if len(all) == 0 {
		return "没有可用技能"
	}
	var sb string
	for name, desc := range all {
		sb += fmt.Sprintf("- %s: %s\n", name, desc)
	}
	return sb
}
