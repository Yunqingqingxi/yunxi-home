package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var log = logger.ForComponent("persistence")

// AgentSnapshot Agent 完整上下文快照
type AgentSnapshot struct {
	Version      int               `json:"version"`
	AgentID      string            `json:"agent_id"`
	SessionID    string            `json:"session_id"`
	State        string            `json:"state"`
	Role         string            `json:"role"`
	Goal         string            `json:"goal,omitempty"`
	Round        int               `json:"round"`
	Progress     float64           `json:"progress"`
	MessagesJSON string            `json:"messages_json"`   // 序列化的对话历史
	ToolCallStack []string         `json:"tool_call_stack"` // 工具调用栈
	Metadata     map[string]string `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at"`
	ParentID     string            `json:"parent_id,omitempty"`
}

// SnapshotManager 快照管理器
type SnapshotManager struct {
	mu         sync.RWMutex
	baseDir    string
	snapshots  map[string][]*AgentSnapshot // agentID → versions (newest last)
	maxPerAgent int
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(baseDir string) (*SnapshotManager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("snapshot dir: %w", err)
	}
	return &SnapshotManager{
		baseDir:      baseDir,
		snapshots:    make(map[string][]*AgentSnapshot),
		maxPerAgent:  5, // 每个 Agent 最多保留 5 个快照
	}, nil
}

// Save 创建快照并持久化到磁盘
func (sm *SnapshotManager) Save(snap *AgentSnapshot) error {
	if snap.Version == 0 {
		snap.Version = 1
	}
	snap.CreatedAt = time.Now()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 自动递增版本号
	existing := sm.snapshots[snap.AgentID]
	if len(existing) > 0 {
		snap.Version = existing[len(existing)-1].Version + 1
	}
	existing = append(existing, snap)

	// 限制快照数量
	if len(existing) > sm.maxPerAgent {
		// 删除最旧的磁盘文件
		oldest := existing[0]
		sm.removeFile(oldest)
		existing = existing[1:]
	}
	sm.snapshots[snap.AgentID] = existing

	// 持久化到磁盘
	filePath := sm.filePath(snap.AgentID, snap.Version)
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot marshal: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("snapshot write: %w", err)
	}

	log.Info("snapshot saved",
		"agent", snap.AgentID,
		"version", snap.Version,
		"state", snap.State,
		"path", filePath,
	)
	return nil
}

// Load 加载最新快照
func (sm *SnapshotManager) Load(agentID string) (*AgentSnapshot, error) {
	sm.mu.RLock()
	versions, ok := sm.snapshots[agentID]
	sm.mu.RUnlock()

	if ok && len(versions) > 0 {
		return versions[len(versions)-1], nil
	}

	// 从磁盘加载
	return sm.loadFromDisk(agentID, 0) // 0 = latest
}

// LoadVersion 加载指定版本快照
func (sm *SnapshotManager) LoadVersion(agentID string, version int) (*AgentSnapshot, error) {
	return sm.loadFromDisk(agentID, version)
}

// ListVersions 列出指定 Agent 的所有快照版本
func (sm *SnapshotManager) ListVersions(agentID string) []int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	versions, ok := sm.snapshots[agentID]
	if !ok {
		return nil
	}
	result := make([]int, len(versions))
	for i, s := range versions {
		result[i] = s.Version
	}
	return result
}

// Delete 删除指定 Agent 的所有快照
func (sm *SnapshotManager) Delete(agentID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	versions := sm.snapshots[agentID]
	for _, s := range versions {
		sm.removeFile(s)
	}
	delete(sm.snapshots, agentID)
}

// Rollback 回滚到指定版本（删除更新的快照）
func (sm *SnapshotManager) Rollback(agentID string, targetVersion int) (*AgentSnapshot, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	versions := sm.snapshots[agentID]
	var target *AgentSnapshot
	var keep []*AgentSnapshot

	for _, s := range versions {
		if s.Version <= targetVersion {
			keep = append(keep, s)
			if s.Version == targetVersion {
				target = s
			}
		} else {
			sm.removeFile(s) // 删除更新的快照文件
		}
	}

	if target == nil {
		return nil, fmt.Errorf("version %d not found for agent %s", targetVersion, agentID)
	}

	sm.snapshots[agentID] = keep
	log.Info("snapshot rolled back", "agent", agentID, "version", targetVersion)
	return target, nil
}

// Cleanup 清理旧快照（保留最近 N 个）
func (sm *SnapshotManager) Cleanup(agentID string, keep int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	versions := sm.snapshots[agentID]
	if len(versions) <= keep {
		return
	}

	for _, s := range versions[:len(versions)-keep] {
		sm.removeFile(s)
	}
	sm.snapshots[agentID] = versions[len(versions)-keep:]
}

// ── 内部方法 ──

func (sm *SnapshotManager) filePath(agentID string, version int) string {
	filename := fmt.Sprintf("%s_v%d.json", agentID, version)
	return filepath.Join(sm.baseDir, filename)
}

func (sm *SnapshotManager) loadFromDisk(agentID string, version int) (*AgentSnapshot, error) {
	var filePath string
	if version == 0 {
		// 查找最新版本
		pattern := filepath.Join(sm.baseDir, fmt.Sprintf("%s_v*.json", agentID))
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			return nil, fmt.Errorf("no snapshots found for agent %s", agentID)
		}
		filePath = matches[len(matches)-1] // 最新
	} else {
		filePath = sm.filePath(agentID, version)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("snapshot read %s: %w", filePath, err)
	}

	var snap AgentSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("snapshot unmarshal: %w", err)
	}

	return &snap, nil
}

func (sm *SnapshotManager) removeFile(snap *AgentSnapshot) {
	path := sm.filePath(snap.AgentID, snap.Version)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Warn("snapshot remove failed", "path", path, "error", err)
	}
}

// ── 便捷构造方法 ──

// NewSnapshot 创建新的 Agent 快照
func NewSnapshot(agentID, sessionID, state, role string) *AgentSnapshot {
	return &AgentSnapshot{
		AgentID:   agentID,
		SessionID: sessionID,
		State:     state,
		Role:      role,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
	}
}
