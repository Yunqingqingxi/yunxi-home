package base

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"

	dbase "github.com/Yunqingqingxi/yunxi-home/internal/database/base"
)

var log = logger.ForComponent("ai.base")

// ── PromptStore v4.0 — DB-first 提示词存储 ──────────────────────────
//
// 架构：
//   1. 所有提示词存储在 DB prompts 表中（category: general | specialized）
//   2. 极少数特殊方法提示词保留为 Go 常量（CorePrompt, QQBotSuffix, TopologyPrompt）
//   3. 通用提示词（general）：每轮对话自动注入 system message
//   4. 专用提示词（specialized）：AI tool-call 激活优先，关键词匹配降级
//   5. 内存缓存 + 热重载支持

// PromptStore manages system prompts with DB-backed hot-reload support.
type PromptStore struct {
	repo              dbase.PromptRepository
	general           []dbase.PromptRecord // 通用提示词缓存
	specialized       []dbase.PromptRecord // 专用提示词缓存
	activatedContexts map[string]map[string]bool // sessionID -> contextID -> true
	generalPrompt     string               // 预渲染的通用提示词（拼接所有 general）
	mu                sync.RWMutex
}

// NewPromptStore creates a PromptStore backed by a PromptRepository.
func NewPromptStore(repo dbase.PromptRepository) *PromptStore {
	return &PromptStore{
		repo:              repo,
		activatedContexts: make(map[string]map[string]bool),
	}
}

// SeedDefaults writes the built-in minimal prompts to DB on first run.
func (ps *PromptStore) SeedDefaults(ctx context.Context) error {
	if ps.repo == nil {
		return nil
	}
	seeds := SeedPrompts()
	return ps.repo.InitDefaults(ctx, seeds)
}

// LoadAll loads all prompts from DB into memory cache.
func (ps *PromptStore) LoadAll(ctx context.Context) error {
	if ps.repo == nil {
		return nil
	}
	all, err := ps.repo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("load prompts: %w", err)
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.general = nil
	ps.specialized = nil

	for _, p := range all {
		if !p.Enabled {
			continue
		}
		switch p.Category {
		case "general":
			ps.general = append(ps.general, p)
		case "specialized":
			ps.specialized = append(ps.specialized, p)
		}
	}

	// Sort by priority (descending)
	sort.Slice(ps.general, func(i, j int) bool { return ps.general[i].Priority > ps.general[j].Priority })
	sort.Slice(ps.specialized, func(i, j int) bool { return ps.specialized[i].Priority > ps.specialized[j].Priority })

	// Pre-render general prompt
	ps.generalPrompt = ps.buildGeneralPromptLocked()

	log.Info("PromptStore loaded from DB", "general", len(ps.general), "specialized", len(ps.specialized))
	return nil
}

// Reload re-reads all prompts from DB and clears activated contexts.
func (ps *PromptStore) Reload(ctx context.Context) error {
	ps.mu.Lock()
	ps.general = nil
	ps.specialized = nil
	ps.generalPrompt = ""
	ps.activatedContexts = make(map[string]map[string]bool)
	ps.mu.Unlock()
	return ps.LoadAll(ctx)
}

// buildGeneralPromptLocked concatenates all general prompts. Caller must hold ps.mu.
func (ps *PromptStore) buildGeneralPromptLocked() string {
	var sb strings.Builder
	for _, p := range ps.general {
		sb.WriteString(p.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// BuildGeneralPrompt returns the pre-rendered general prompt (all enabled general prompts).
func (ps *PromptStore) BuildGeneralPrompt() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.generalPrompt
}

// ActivateContext activates a specialized context for a session.
func (ps *PromptStore) ActivateContext(sessionID, contextID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.activatedContexts[sessionID] == nil {
		ps.activatedContexts[sessionID] = make(map[string]bool)
	}
	ps.activatedContexts[sessionID][contextID] = true
	log.Debug("context activated", "session", sessionID, "context", contextID)
}

// DeactivateContext deactivates a specialized context for a session.
func (ps *PromptStore) DeactivateContext(sessionID, contextID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if m := ps.activatedContexts[sessionID]; m != nil {
		delete(m, contextID)
	}
}

// GetActivatedContexts returns the list of activated context IDs for a session.
func (ps *PromptStore) GetActivatedContexts(sessionID string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	m := ps.activatedContexts[sessionID]
	if m == nil {
		return nil
	}
	var ids []string
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GetSpecializedPrompt returns the content for a specialized prompt by ID.
func (ps *PromptStore) GetSpecializedPrompt(id string) string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, p := range ps.specialized {
		if p.ID == id {
			return p.Content
		}
	}
	return ""
}

// GetAllSpecialized returns all specialized prompts (for tool enum generation).
func (ps *PromptStore) GetAllSpecialized() []dbase.PromptRecord {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	cp := make([]dbase.PromptRecord, len(ps.specialized))
	copy(cp, ps.specialized)
	return cp
}

// MatchContexts performs keyword matching to find relevant specialized contexts.
// This is the fallback when AI doesn't call activate_specialized_context.
func (ps *PromptStore) MatchContexts(userMessage string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	msg := strings.ToLower(userMessage)
	var matched []string

	for _, p := range ps.specialized {
		if !p.Enabled {
			continue
		}
		var keywords []string
		if err := json.Unmarshal([]byte(p.Keywords), &keywords); err != nil {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				matched = append(matched, p.ID)
				break
			}
		}
	}
	return matched
}

// BuildSystemPrompt assembles the complete system prompt:
// general prompts + activated specialized prompts.
func (ps *PromptStore) BuildSystemPrompt(sessionID, userMessage string, recentToolCalls []string) string {
	ps.mu.RLock()
	generalPrompt := ps.generalPrompt
	generalNames := make([]string, len(ps.general))
	for i, p := range ps.general {
		generalNames[i] = p.ID
	}
	activated := make(map[string]bool)
	if m := ps.activatedContexts[sessionID]; m != nil {
		for k, v := range m {
			activated[k] = v
		}
	}
	ps.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(generalPrompt)

	// Append activated specialized prompts
	var activeSpecialized []string
	for contextID := range activated {
		if content := ps.GetSpecializedPrompt(contextID); content != "" {
			sb.WriteString("\n\n")
			sb.WriteString(content)
			activeSpecialized = append(activeSpecialized, contextID)
		}
	}

	log.Info("System Prompt 已组装",
		"session", sessionID,
		"general_count", len(generalNames),
		"general_ids", strings.Join(generalNames, ","),
		"specialized_activated", len(activeSpecialized),
		"specialized_ids", strings.Join(activeSpecialized, ","),
	)

	return sb.String()
}

// TryAutoActivate runs keyword matching and activates matched contexts for a session.
// Returns the newly activated context IDs.
func (ps *PromptStore) TryAutoActivate(sessionID, userMessage string) []string {
	matched := ps.MatchContexts(userMessage)
	if len(matched) == 0 {
		return nil
	}

	ps.mu.Lock()
	if ps.activatedContexts[sessionID] == nil {
		ps.activatedContexts[sessionID] = make(map[string]bool)
	}
	var newActivated []string
	for _, id := range matched {
		if !ps.activatedContexts[sessionID][id] {
			ps.activatedContexts[sessionID][id] = true
			newActivated = append(newActivated, id)
		}
	}
	ps.mu.Unlock()

	if len(newActivated) > 0 {
		log.Info("自动激活专用提示词", "session", sessionID, "matched_keywords", len(matched), "new_activated", strings.Join(newActivated, ","))
	}
	return newActivated
}

// ── Prompt Management API ────────────────────────────────────────────

// PromptInfo is the public representation of a prompt for the management API.
type PromptInfo struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Keywords  string `json:"keywords"`
	Priority  int    `json:"priority"`
	Enabled   bool   `json:"enabled"`
	Source    string `json:"source"` // "db" | "builtin" | "custom"
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// GetAllPrompts returns all prompts as PromptInfo for the management API.
func (ps *PromptStore) GetAllPrompts() []PromptInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var result []PromptInfo
	for _, p := range ps.general {
		result = append(result, promptToInfo(p, "db"))
	}
	for _, p := range ps.specialized {
		result = append(result, promptToInfo(p, "db"))
	}
	return result
}

func promptToInfo(p dbase.PromptRecord, source string) PromptInfo {
	return PromptInfo{
		ID:        p.ID,
		Category:  p.Category,
		Name:      p.Name,
		Content:   p.Content,
		Keywords:  p.Keywords,
		Priority:  p.Priority,
		Enabled:   p.Enabled,
		Source:    source,
		CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// UpdatePrompt updates a prompt in DB and refreshes cache.
func (ps *PromptStore) UpdatePrompt(ctx context.Context, p dbase.PromptRecord) error {
	if ps.repo == nil {
		return nil
	}
	if err := ps.repo.Upsert(ctx, &p); err != nil {
		return err
	}
	return ps.LoadAll(ctx)
}

// DeletePrompt deletes a prompt from DB and refreshes cache.
func (ps *PromptStore) DeletePrompt(ctx context.Context, id string) error {
	if ps.repo == nil {
		return nil
	}
	if err := ps.repo.Delete(ctx, id); err != nil {
		return err
	}
	return ps.LoadAll(ctx)
}

// ── Utility ──────────────────────────────────────────────────────────

// hashStrings returns a stable hash of sorted strings.
func hashStrings(ss []string) string {
	sorted := make([]string, len(ss))
	copy(sorted, ss)
	sort.Strings(sorted)
	h := md5.New()
	for _, s := range sorted {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// SystemPromptHash returns the MD5 of a rendered system prompt for provider cache headers.
func SystemPromptHash(prompt string) string {
	h := md5.New()
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))
}

// ── Topology State Message (layered injection) ──────────────────────

// BuildTopologyMessage creates the compact topology state message for message[1].
// Format: <t:x,y,z|A:a,R:r,T:t>  (~15 tokens)
func BuildTopologyMessage(x, y, z, a, r float64, t bool, acked bool) string {
	tFlag := 0
	if t {
		tFlag = 1
	}
	ackPart := ""
	if acked {
		ackPart = "|ack:1"
	}
	return fmt.Sprintf("<t:%.1f,%.2f,%.2f|A:%.1f,R:%.1f,T:%d%s>",
		x, y, z, a, r, tFlag, ackPart)
}

// TopologyMsgPrefix is the prefix used to identify topology state messages in history.
const TopologyMsgPrefix = "<t:"
