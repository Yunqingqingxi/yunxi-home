package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var log = logger.ForComponent("ai.memory")

// Manager manages persistent memories.
type Manager struct {
	repo     Repository
	memories map[string]*Memory
	dir      string // memory/ directory path for file write-back
	mu       sync.RWMutex
}

// NewManager creates a new Manager.
func NewManager(repo Repository) *Manager {
	return &Manager{
		repo:     repo,
		memories: make(map[string]*Memory),
	}
}

// EnsureSchema ensures the memories table exists.
func (m *Manager) EnsureSchema(ctx context.Context) error {
	return m.repo.EnsureSchema(ctx)
}

// InitFromDir loads .md files from a directory into DB (skips existing names).
func (m *Manager) InitFromDir(dir string) error {
	m.dir = dir // remember for file write-back
	log.Info("开始从目录导入记忆种子文件", "dir", dir)

	fileMems, err := LoadFromDir(dir)
	if err != nil {
		log.Error("扫描记忆目录失败", "dir", dir, "error", err)
		return fmt.Errorf("扫描记忆目录失败: %w", err)
	}
	if len(fileMems) == 0 {
		log.Info("记忆目录为空，跳过导入", "dir", dir)
		return nil
	}

	ctx := context.Background()
	existing, err := m.repo.GetAll(ctx)
	if err != nil {
		log.Error("读取已有记忆失败", "error", err)
		return fmt.Errorf("读取已有记忆失败: %w", err)
	}
	existingNames := make(map[string]bool, len(existing))
	for _, em := range existing {
		existingNames[em.Name] = true
	}

	imported := 0
	skipped := 0
	for _, fm := range fileMems {
		if existingNames[fm.Name] {
			log.Info("记忆已存在于 DB，跳过导入", "name", fm.Name)
			skipped++
			continue
		}
		fm.Source = "file"
		if err := m.Save(ctx, fm); err != nil {
			log.Warn("导入记忆失败", "name", fm.Name, "error", err)
			continue
		}
		existingNames[fm.Name] = true
		imported++
	}
	log.Info("记忆文件导入完成", "imported", imported, "skipped_existing", skipped, "total_files", len(fileMems))
	return nil
}

// LoadFromDB loads all memories from DB into the in-memory index.
func (m *Manager) LoadFromDB(ctx context.Context) error {
	log.Info("从 DB 加载记忆...")

	memories, err := m.repo.GetAll(ctx)
	if err != nil {
		log.Error("从 DB 加载记忆失败", "error", err)
		return fmt.Errorf("从 DB 加载记忆失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories = make(map[string]*Memory, len(memories))
	for _, mem := range memories {
		m.memories[mem.Name] = mem
	}
	log.Info("记忆加载完成", "count", len(m.memories))
	return nil
}

// Summary returns a compact memory list for the system prompt.
// Only includes GENERAL memories (no context_tags). Context-specific memories are loaded
// via SummaryByContexts when the corresponding specialized context is activated.
func (m *Manager) Summary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.memories) == 0 {
		return ""
	}

	// Filter for general memories (no context_tags)
	var generalMemories []*Memory
	for _, mem := range m.memories {
		if len(mem.ContextTags) == 0 {
			generalMemories = append(generalMemories, mem)
		}
	}

	if len(generalMemories) == 0 {
		return ""
	}

	// Sort by name
	sort.Slice(generalMemories, func(i, j int) bool {
		return generalMemories[i].Name < generalMemories[j].Name
	})

	var sb strings.Builder
	sb.WriteString("- 以下是关于用户和本项目的长期记忆（始终有效），跨会话保持。\n")
	sb.WriteString("- 需要详细信息时使用 recall 工具检索。\n")
	sb.WriteString("- 发现需要记住的新信息时使用 remember 工具保存。\n\n")
	for _, mem := range generalMemories {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Name, mem.Description))
	}
	return sb.String()
}

// SummaryByContexts returns compact descriptions of memories tagged with the given context IDs.
func (m *Manager) SummaryByContexts(contextIDs []string) string {
	if len(contextIDs) == 0 {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ctxSet := make(map[string]bool, len(contextIDs))
	for _, id := range contextIDs {
		ctxSet[id] = true
	}

	var matched []*Memory
	for _, mem := range m.memories {
		for _, tag := range mem.ContextTags {
			if ctxSet[tag] {
				matched = append(matched, mem)
				break
			}
		}
	}

	if len(matched) == 0 {
		return ""
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })

	var sb strings.Builder
	sb.WriteString("- 以下记忆与当前任务领域相关：\n")
	for _, mem := range matched {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Name, mem.Description))
	}
	return sb.String()
}

// Match finds memories relevant to the given query.
func (m *Manager) Match(query string) []*Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.memories) == 0 {
		return nil
	}

	lower := strings.ToLower(query)

	type scored struct {
		mem   *Memory
		score int
	}
	var candidates []scored

	for _, mem := range m.memories {
		s := matchScore(lower, mem)
		if s > 0 {
			candidates = append(candidates, scored{mem: mem, score: s})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	result := make([]*Memory, 0, 3)
	for _, c := range candidates {
		if len(result) >= 3 {
			break
		}
		if c.score >= 1 {
			result = append(result, c.mem)
		}
	}

	// 记录匹配结果
	if len(result) > 0 {
		names := make([]string, len(result))
		scores := make([]int, len(result))
		for i, r := range result {
			names[i] = r.Name
			scores[i] = 0
			// 找回对应分数
			for _, c := range candidates {
				if c.mem.Name == r.Name {
					scores[i] = c.score
					break
				}
			}
		}
		log.Info("记忆匹配完成", "query_preview", truncate(query, 60),
			"matched", len(result), "names", strings.Join(names, ","))
	} else if len(candidates) > 0 {
		log.Info("记忆匹配无达标结果", "query_preview", truncate(query, 60),
			"candidates", len(candidates), "min_score", 1)
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func matchScore(query string, mem *Memory) int {
	score := 0

	nameLower := strings.ToLower(mem.Name)
	if strings.Contains(query, nameLower) || strings.Contains(nameLower, query) {
		score += 3
	}

	descLower := strings.ToLower(mem.Description)
	for _, w := range strings.Fields(descLower) {
		if len(w) >= 2 && strings.Contains(query, w) {
			score++
		}
	}

	contentLower := strings.ToLower(mem.Content)
	if len(contentLower) > 500 {
		contentLower = contentLower[:500]
	}
	for _, w := range strings.Fields(contentLower) {
		if len(w) >= 3 && strings.Contains(query, w) {
			score++
		}
	}

	return score
}

// Get returns a memory by name.
func (m *Manager) Get(name string) (*Memory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mem, ok := m.memories[name]
	if !ok {
		log.Warn("记忆不存在", "name", name)
		return nil, fmt.Errorf("记忆不存在: %s", name)
	}
	return mem, nil
}

// Save persists a memory to DB and updates the in-memory index.
func (m *Manager) Save(ctx context.Context, mem *Memory) error {
	if err := m.repo.Save(ctx, mem); err != nil {
		log.Error("保存记忆失败", "name", mem.Name, "error", err)
		return fmt.Errorf("保存记忆失败: %w", err)
	}

	m.mu.Lock()
	_, existing := m.memories[mem.Name]
	m.memories[mem.Name] = mem
	m.mu.Unlock()

	// 同步回写 .md 文件（如果配置了目录）
	if m.dir != "" {
		if err := WriteToFile(m.dir, mem); err != nil {
			log.Warn("回写记忆文件失败", "name", mem.Name, "error", err)
		}
	}

	if existing {
		log.Info("记忆已更新", "name", mem.Name, "type", string(mem.Type), "content_len", len(mem.Content))
	} else {
		log.Info("记忆已创建", "name", mem.Name, "type", string(mem.Type), "content_len", len(mem.Content))
	}
	return nil
}

// Delete removes a memory from DB and the in-memory index.
func (m *Manager) Delete(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.repo.Delete(ctx, name); err != nil {
		log.Error("删除记忆失败", "name", name, "error", err)
		return fmt.Errorf("删除记忆失败: %w", err)
	}
	delete(m.memories, name)
	log.Info("记忆已删除", "name", name)
	return nil
}

// Count returns the number of loaded memories.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.memories)
}

// RecordAccess updates access metadata for a memory (called when matched or retrieved).
func (m *Manager) RecordAccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mem, ok := m.memories[name]; ok {
		mem.AccessCount++
		mem.LastAccessed = time.Now()
	}
}

// TopByImportance returns up to N memories sorted by computed importance (highest first).
func (m *Manager) TopByImportance(n int) []*Memory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type scored struct {
		mem   *Memory
		score float64
	}
	var items []scored
	for _, mem := range m.memories {
		items = append(items, scored{mem: mem, score: mem.ComputeImportance()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })

	result := make([]*Memory, 0, n)
	for i, item := range items {
		if i >= n {
			break
		}
		result = append(result, item.mem)
	}
	return result
}

// SummarizeCompact returns a concise memory summary for limited-context injection.
// Only includes top-5 most important memories with one-line descriptions.
func (m *Manager) SummarizeCompact() string {
	top := m.TopByImportance(5)
	if len(top) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 关键记忆\n")
	for _, mem := range top {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", mem.Name, mem.Description))
	}
	return sb.String()
}

// AutoCreateFromFeedback creates a feedback-type memory when correction patterns repeat.
// Called by the adapt layer when the same correction type is detected 3+ times.
func (m *Manager) AutoCreateFromFeedback(ctx context.Context, feedbackType, detail string) {
	if feedbackType == "" {
		return
	}
	name := fmt.Sprintf("feedback-%s-%d", feedbackType, time.Now().Unix())
	mem := &Memory{
		Name:        name,
		Description: fmt.Sprintf("Auto-created from repeated %s feedback", feedbackType),
		Type:        TypeFeedback,
		Content:     detail,
		Source:      "auto",
		Importance:  0.6,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := m.Save(ctx, mem); err != nil {
		log.Warn("auto-create memory failed", "name", name, "error", err)
	} else {
		log.Info("auto-created feedback memory", "name", name, "type", feedbackType)
	}
}

// CleanupStale removes memories whose importance has decayed below a threshold.
// Returns the number of deleted memories.
func (m *Manager) CleanupStale(ctx context.Context, threshold float64) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toDelete []string
	for name, mem := range m.memories {
		if mem.ComputeImportance() < threshold && mem.Source != "file" {
			toDelete = append(toDelete, name)
		}
	}

	for _, name := range toDelete {
		_ = m.repo.Delete(ctx, name)
		delete(m.memories, name)
	}

	if len(toDelete) > 0 {
		log.Info("cleaned stale memories", "count", len(toDelete), "threshold", threshold)
	}
	return len(toDelete)
}

// UpdateImportance adjusts a memory's importance score.
func (m *Manager) UpdateImportance(name string, delta float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mem, ok := m.memories[name]; ok {
		mem.Importance += delta
		if mem.Importance > 1.0 {
			mem.Importance = 1.0
		}
		if mem.Importance < 0.0 {
			mem.Importance = 0.0
		}
		mem.UpdatedAt = time.Now().UTC()
	}
}

// RecordMatchAndAccess updates access metadata for matched memories.
func (m *Manager) RecordMatchAndAccess(matched []*Memory) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mem := range matched {
		if existing, ok := m.memories[mem.Name]; ok {
			existing.AccessCount++
			existing.LastAccessed = time.Now().UTC()
			// Slightly boost importance on each access
			existing.Importance = existing.ComputeImportance() + 0.02
			if existing.Importance > 1.0 {
				existing.Importance = 1.0
			}
		}
	}
}
