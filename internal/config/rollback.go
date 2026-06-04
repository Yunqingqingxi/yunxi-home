package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RollbackStore saves recent config snapshots to enable rollback on failed changes.
type RollbackStore struct {
	mu        sync.RWMutex
	snapshots map[string][]rollbackEntry // section → recent snapshots
	maxPerKey int
	maxAge    time.Duration
}

type rollbackEntry struct {
	Data      string
	SavedAt   time.Time
	Version   int
}

const (
	defaultMaxSnapshots = 5              // keep last 5 versions per section
	defaultMaxAge        = 7 * 24 * time.Hour // 7 days
)

// NewRollbackStore creates a new rollback store.
func NewRollbackStore() *RollbackStore {
	return &RollbackStore{
		snapshots: make(map[string][]rollbackEntry),
		maxPerKey: defaultMaxSnapshots,
		maxAge:    defaultMaxAge,
	}
}

// Save saves a snapshot of a config section before modification.
// Returns a version number that can be used to Restore.
func (rs *RollbackStore) Save(section string, data interface{}) int {
	raw, err := json.Marshal(data)
	if err != nil {
		slog.Warn("rollback: failed to marshal section", "section", section, "error", err)
		return -1
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	entries := rs.snapshots[section]
	version := 0
	if len(entries) > 0 {
		version = entries[len(entries)-1].Version + 1
	}

	rs.snapshots[section] = append(entries, rollbackEntry{
		Data:    string(raw),
		SavedAt: time.Now(),
		Version: version,
	})

	// Keep only the last N entries
	if len(rs.snapshots[section]) > rs.maxPerKey {
		rs.snapshots[section] = rs.snapshots[section][len(rs.snapshots[section])-rs.maxPerKey:]
	}

	// Cleanup expired entries across all sections
	rs.cleanupLocked()

	return version
}

// Latest returns the most recent snapshot for a section.
func (rs *RollbackStore) Latest(section string) (string, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	entries := rs.snapshots[section]
	if len(entries) == 0 {
		return "", false
	}
	return entries[len(entries)-1].Data, true
}

// RestoreToVersion returns the snapshot at a specific version.
func (rs *RollbackStore) RestoreToVersion(section string, version int) (string, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	for _, entry := range rs.snapshots[section] {
		if entry.Version == version {
			return entry.Data, true
		}
	}
	return "", false
}

// List returns all saved versions for a section.
func (rs *RollbackStore) List(section string) []map[string]interface{} {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var result []map[string]interface{}
	for _, entry := range rs.snapshots[section] {
		result = append(result, map[string]interface{}{
			"version":  entry.Version,
			"saved_at": entry.SavedAt.Format(time.RFC3339),
		})
	}
	return result
}

// Clear removes all snapshots for a section.
func (rs *RollbackStore) Clear(section string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	delete(rs.snapshots, section)
}

// cleanupLocked removes expired entries. Must be called with mu held (Write lock).
func (rs *RollbackStore) cleanupLocked() {
	cutoff := time.Now().Add(-rs.maxAge)
	for section, entries := range rs.snapshots {
		valid := entries[:0]
		for _, e := range entries {
			if e.SavedAt.After(cutoff) {
				valid = append(valid, e)
			}
		}
		if len(valid) == 0 {
			delete(rs.snapshots, section)
		} else {
			rs.snapshots[section] = valid
		}
	}
}

// ── Safe config change helper ──────────────────────────────────────────

// SafeChange executes a config change with rollback on failure.
//   - saveSnapshot: called to persist the previous state as a rollback point
//   - applyChange:  called to apply the new configuration
//   - validateChange: called to verify the change succeeded; returns error if rollback needed
//   - rollback:     called to restore the previous state
func SafeChange(
	section string,
	store *RollbackStore,
	previousState interface{},
	saveSnapshot func() error,
	applyChange func() error,
	validateChange func(ctx context.Context) error,
	rollback func() error,
	ctx context.Context,
) error {
	// 1. Save rollback point
	store.Save(section, previousState)
	if err := saveSnapshot(); err != nil {
		slog.Warn("未能保存变更前快照", "section", section, "error", err)
		// Continue anyway; the in-memory snapshot is saved
	}

	// 2. Apply the change
	if err := applyChange(); err != nil {
		return fmt.Errorf("应用配置变更失败: %w", err)
	}

	// 3. Validate the change
	if validateChange != nil {
		if err := validateChange(ctx); err != nil {
			slog.Warn("配置变更验证失败，执行回滚", "section", section, "error", err)
			if rbErr := rollback(); rbErr != nil {
				slog.Error("配置回滚失败!", "section", section, "error", rbErr)
				return fmt.Errorf("变更验证失败 (%w)，且回滚也失败 (%w)", err, rbErr)
			}
			return fmt.Errorf("配置变更验证失败，已自动回滚: %w", err)
		}
	}

	slog.Info("配置变更已安全应用", "section", section)
	return nil
}
