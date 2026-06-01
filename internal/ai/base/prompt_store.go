package base

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// ── PromptStore — 提示词外置存储 ─────────────────────────────
//
// 三层架构：
//   1. Go 常量 (编译时 fallback，永不删除)
//   2. DB 记录 (运行时加载，可热重载)
//   3. 内存缓存 (合并策略：DB 有值用 DB，否则用 Go 常量)
//
// 复用现有 config 表（section + data 两列），section 命名空间 prompt_*。
//
// v3.1 缓存优化：
//   - Intent hash cache: 跨会话共享，相同意图组合复用渲染结果。
//   - BuildSystemPrompt 严格幂等，不含 time.Now()/random/UUID。
//   - Topology prompt 不在此处——由 ensureTopologyState() 注入 message[1]
//     (分层注入), 使 message[0] 保持静态，KV 缓存友好。

// PromptStore manages system prompt sections with DB-backed hot-reload support.
type PromptStore struct {
	configRepo  ConfigGetter
	defaults    map[string]string // Go 常量 fallback
	cache       map[string]string // DB 记录缓存
	promptCache sync.Map          // map[intentHash]cachedPrompt — cross-session cache
	mu          sync.RWMutex
}

type cachedPrompt struct {
	prompt string
}

// ConfigGetter is the minimal interface PromptStore needs from a config repository.
type ConfigGetter interface {
	GetSection(ctx context.Context, section string) (string, error)
	GetAll(ctx context.Context) (map[string]string, error)
	SetSection(ctx context.Context, section, data string) error
}

// PromptSection maps section names to their Go fallback constants.
type PromptSection struct {
	Name    string // e.g. "prompt_identity"
	Default string // Go constant value
}

// NewPromptStore creates a PromptStore with the given Go defaults and config repo.
func NewPromptStore(repo ConfigGetter) *PromptStore {
	ps := &PromptStore{
		configRepo: repo,
		defaults:   make(map[string]string),
		cache:      make(map[string]string),
	}
	ps.registerDefaults()
	return ps
}

// registerDefaults populates the Go fallback map from existing constants.
func (ps *PromptStore) registerDefaults() {
	ps.defaults = map[string]string{
		"prompt_identity":      IdentityRules,
		"prompt_environment":   EnvironmentRules,
		"prompt_core":          CoreRules,
		"prompt_communication": CommunicationRules,
		"prompt_filesystem":    FilesystemRules,
		"prompt_command_exec":  CommandExecutionRules,
		"prompt_task_boundary": TaskBoundaryRules,
		"prompt_mcp_status":    MCPStatusRules,
		"prompt_slash_command": SlashCommandRules,
		"prompt_file_sending":  FileSendingRules,
		"prompt_tool_strategy": ToolStrategy,
		"prompt_timeout_guide": TimeoutGuide,
		"prompt_core_compact":  CorePrompt,
		"prompt_topology":      TopologyPrompt,
		"prompt_code_review":   CodeReviewPrompt,
	}
}

// LoadFromDB loads all prompt_* sections from the database into the cache.
func (ps *PromptStore) LoadFromDB(ctx context.Context) error {
	if ps.configRepo == nil {
		return nil
	}
	all, err := ps.configRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for section, data := range all {
		if strings.HasPrefix(section, "prompt_") && data != "" {
			ps.cache[section] = data
		}
	}
	slog.Info("PromptStore loaded from DB", "sections", len(ps.cache))
	return nil
}

// Reload re-reads all prompt sections from DB and clears the intent cache.
func (ps *PromptStore) Reload(ctx context.Context) error {
	ps.mu.Lock()
	ps.cache = make(map[string]string)
	ps.mu.Unlock()
	ps.promptCache = sync.Map{} // Clear intent cache (prompts may have changed)
	return ps.LoadFromDB(ctx)
}

// Get returns the effective prompt text for a section.
func (ps *PromptStore) Get(section string) string {
	ps.mu.RLock()
	if dbVal, ok := ps.cache[section]; ok && dbVal != "" {
		ps.mu.RUnlock()
		return dbVal
	}
	ps.mu.RUnlock()
	if def, ok := ps.defaults[section]; ok {
		return def
	}
	return ""
}

// GetAllSections returns all prompt sections with their effective values and source.
func (ps *PromptStore) GetAllSections() map[string]PromptSectionInfo {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make(map[string]PromptSectionInfo, len(ps.defaults))
	for name, def := range ps.defaults {
		if dbVal, ok := ps.cache[name]; ok && dbVal != "" {
			result[name] = PromptSectionInfo{Name: name, Value: dbVal, Source: "db", HasDefault: true}
		} else {
			result[name] = PromptSectionInfo{Name: name, Value: def, Source: "builtin", HasDefault: true}
		}
	}
	return result
}

// PromptSectionInfo holds the effective value and its source for a prompt section.
type PromptSectionInfo struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Source     string `json:"source"` // "db" | "builtin"
	HasDefault bool   `json:"has_default"`
}

// UpdateSection writes a new value for a prompt section to DB and refreshes cache.
func (ps *PromptStore) UpdateSection(ctx context.Context, section, data string) error {
	if ps.configRepo == nil {
		return nil
	}
	if !strings.HasPrefix(section, "prompt_") {
		section = "prompt_" + section
	}
	if err := ps.configRepo.SetSection(ctx, section, data); err != nil {
		return err
	}
	ps.mu.Lock()
	ps.cache[section] = data
	ps.mu.Unlock()
	ps.promptCache = sync.Map{} // Invalidate intent cache
	slog.Info("PromptStore section updated", "section", section, "len", len(data))
	return nil
}

// ResetSection removes the DB override for a section, falling back to Go default.
func (ps *PromptStore) ResetSection(ctx context.Context, section string) error {
	if ps.configRepo == nil {
		return nil
	}
	if !strings.HasPrefix(section, "prompt_") {
		section = "prompt_" + section
	}
	if err := ps.configRepo.SetSection(ctx, section, ""); err != nil {
		return err
	}
	ps.mu.Lock()
	delete(ps.cache, section)
	ps.mu.Unlock()
	ps.promptCache = sync.Map{}
	slog.Info("PromptStore section reset to default", "section", section)
	return nil
}

// InitDefaults writes all Go default values to DB so there is a complete baseline.
func (ps *PromptStore) InitDefaults(ctx context.Context) error {
	if ps.configRepo == nil {
		return nil
	}
	existing, err := ps.configRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	hasPrompts := false
	for section := range existing {
		if strings.HasPrefix(section, "prompt_") {
			hasPrompts = true
			break
		}
	}
	if hasPrompts {
		return nil
	}
	for section, data := range ps.defaults {
		if err := ps.configRepo.SetSection(ctx, section, data); err != nil {
			return err
		}
	}
	slog.Info("PromptStore defaults seeded to DB", "sections", len(ps.defaults))
	return nil
}

// ── System Prompt Builder ─────────────────────────────────────
//
// v3.1 cache optimization:
//   - Intent hash cache: cross-session shared, same intent combo → same prompt.
//   - Strictly idempotent: same inputs → identical output. NO time.Now(), random, UUID.
//   - Topology prompt NOT included here — lives in message[1] via ensureTopologyState()
//     for KV cache friendliness (layered injection).

// IntentDetector detects scenario intents from user message and recent tool calls.
type IntentDetector func(userMessage string, recentToolCalls []string) []string

// DefaultIntentDetector is the default intent detection function (from prompt.go).
var DefaultIntentDetector IntentDetector = DetectIntent

// BuildSystemPrompt assembles the system prompt from prompt sections based on
// detected intents. Uses cross-session intent cache. Returns (prompt, intentHash).
func (ps *PromptStore) BuildSystemPrompt(userMessage string, recentToolCalls []string) (string, string) {
	detector := DefaultIntentDetector
	if detector == nil {
		detector = func(_ string, _ []string) []string { return nil }
	}

	intents := detector(userMessage, recentToolCalls)
	intentHash := hashStrings(intents)

	// Cross-session intent cache
	if entry, ok := ps.promptCache.Load(intentHash); ok {
		return entry.(cachedPrompt).prompt, intentHash
	}

	// Cache miss — render
	var sb strings.Builder
	sb.WriteString(ps.Get("prompt_core_compact"))

	seen := make(map[string]bool)
	for _, intent := range intents {
		if seen[intent] {
			continue
		}
		seen[intent] = true
		if text := ps.Get("prompt_" + intent); text != "" {
			sb.WriteString(text)
		}
	}

	result := sb.String()
	ps.promptCache.Store(intentHash, cachedPrompt{prompt: result})
	return result, intentHash
}

// hashStrings returns a stable hash of sorted strings (for intent cache keying).
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

// ── Topology State Message (layered injection) ────────────────

// BuildTopologyMessage creates the compact topology state message for message[1].
// Format: <t:x,y,z|A:a,R:r,T:t>  (~15 tokens vs ~40 for verbose Chinese text)
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

// ── Code Review Prompt ─────────────────────────────────────────

const CodeReviewPrompt = "\n\n## 代码分析与优化" +
	"\n\n### 聚焦原则" +
	"\n- 只看代码，只谈代码。**禁止**讨论\"作为 AI 我应该...\"\"根据规则我需要...\"\"让我想想...\"等元思考" +
	"\n- 思考内容仅限于：读哪个文件、发现了什么问题、怎么改。不写任务规划、不写步骤清单" +
	"\n- 用户问的是项目代码，不是你的能力范围——**禁止**回复\"我可以帮你分析\"\"我能做的是\"等自我介绍" +
	"\n\n### 探索策略" +
	"\n- 分析项目时，**最多读取 5 个核心文件**后就必须开始输出结论" +
	"\n- 核心文件指：入口文件(main.go/index.js)、依赖文件(go.mod/package.json)、README、主模块入口" +
	"\n- **禁止**遍历所有子目录——列出顶层目录结构后，根据文件名判断模块职责即可" +
	"\n- `file_list` 最多使用 3 次：根目录→internal/→最多再深入 1 层" +
	"\n\n### 中断恢复" +
	"\n- 用户说\"重新开始\"\"继续\"\"恢复\"\"接着做\"\"上次的\"时，**不是新任务**——是让你从上次中断处继续" +
	"\n- 恢复时先检查上下文：如果历史里有之前的修改记录，直接基于那些记录继续，不要重新 file_list 分析" +
	"\n- 如果无法确定从哪里恢复，用一句话确认即可——**禁止**写 300 字内心独白猜测含义" +
	"\n\n### 输出要求" +
	"\n- 每条优化建议必须包含：**问题描述** + **为什么是问题** + **具体改进方案**(含示例代码)" +
	"\n- 按优先级排序：🥇 高优先(影响稳定性/安全) > 🥈 中优先(影响可维护性) > 🥉 低优先(代码风格)" +
	"\n- 每条建议控制在 150 字以内，简洁有力" +
	"\n\n### 避免的行为" +
	"\n- **禁止**在 5 个核心文件之外继续 file_list 深层子目录" +
	"\n- **禁止**读取 node_modules、vendor、.git、dist、build 等非源码目录" +
	"\n- **禁止**对同一目录反复 file_list" +
	"\n- **禁止**给出模糊建议如\"考虑重构\"\"可以优化性能\"——必须有具体文件和行级定位" +
	"\n- **禁止**用户说\"重新开始\"后写 300 字内心独白——一句话确认，然后行动" +
	"\n- **禁止**思考块中出现\"用户想要...\"\"我应该...\"\"根据规则...\"等非代码内容"

// ── Topology Prompt (Go fallback, for prompt_topology section) ─

// TopologyPrompt is injected when the topology constraint system is active.
// Note: with layered injection, this is only used when topologyActive=true in
// the old inline mode. Normal operation uses BuildTopologyMessage() for message[1].
const TopologyPrompt = "\n\n## 拓扑几何约束" +
	"\n你当前处于拓扑约束模式。每轮回复末尾必须输出拓扑坐标标签：" +
	"\n<topology x=\"进度\" y=\"复杂度变化\" z=\"偏离度\" tools=\"工具1,工具2\" />" +
	"\n\n坐标说明：" +
	"\n- x: 任务进度 0-10（0=未开始, 10=完成）" +
	"\n- y: 本轮操作复杂度变化 -1.0 到 1.0（负=降低复杂度如读取, 正=增加复杂度如写入/删除/执行命令）" +
	"\n- z: 偏离起点的距离 0 到 R（由约束参数决定）" +
	"\n\n约束参数（由系统设定）：" +
	"\n- 振幅上限 A: |Δy| 每轮不超过此值" +
	"\n- 半径上限 R: z 坐标不超过此值" +
	"\n- 闭环要求 T: 任务完成时需回到原点附近" +
	"\n\ntools 字段**仅列出本轮**你实际调用的工具名（逗号分隔），**不要**包含前几轮的工具。" +
	"\n纯文本回复(无工具调用)时 tools=\"\"。X≥9.5 时 tools 声明会被忽略(任务已完成)。" +
	"\n如果系统更新了约束参数，用 ack=\"constraint_updated\" 确认。"
