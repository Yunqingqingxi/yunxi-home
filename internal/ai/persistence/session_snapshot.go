// Package persistence provides session state persistence for agent recovery across restarts.
// Inspired by Claude Code's ~/.claude/ session persistence model.
package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var log = logger.ForComponent("persistence")

// SessionSnapshot captures the full state of an active session for recovery.
type SessionSnapshot struct {
	SessionID   string    `json:"session_id"`
	UpdatedAt   time.Time `json:"updated_at"`
	HasStream   bool      `json:"has_stream"`
	HasAgents   bool      `json:"has_agents"`
	AgentCount  int       `json:"agent_count"`
	AgentIDs    []string  `json:"agent_ids"`
	MessageCount int      `json:"message_count"`
	LastUserMsg string    `json:"last_user_msg"`
	TopologyActive bool   `json:"topology_active"`
}

// SessionStore persists session snapshots to disk for recovery across restarts.
// Uses atomic writes (write tmp → rename) for crash safety.
type SessionStore struct {
	mu   sync.RWMutex
	dir  string
}

// NewSessionStore creates a session store rooted at the given directory.
func NewSessionStore(dir string) *SessionStore {
	os.MkdirAll(dir, 0755)
	ss := &SessionStore{dir: dir}
	// Clean up old snapshots on startup
	go ss.cleanupLoop()
	return ss
}

// Save persists a session snapshot atomically.
func (ss *SessionStore) Save(snap SessionSnapshot) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	snap.UpdatedAt = time.Now()
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	path := ss.path(snap.SessionID)
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Load reads a session snapshot from disk.
func (ss *SessionStore) Load(sessionID string) (*SessionSnapshot, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	data, err := os.ReadFile(ss.path(sessionID))
	if err != nil {
		return nil, err
	}
	var snap SessionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

// Delete removes a session snapshot (called when session is cleaned up).
func (ss *SessionStore) Delete(sessionID string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return os.Remove(ss.path(sessionID))
}

// ListActive returns all session snapshots currently on disk.
func (ss *SessionStore) ListActive() ([]SessionSnapshot, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	entries, err := os.ReadDir(ss.dir)
	if err != nil {
		return nil, err
	}
	var snaps []SessionSnapshot
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ss.dir, e.Name()))
		if err != nil {
			continue
		}
		var snap SessionSnapshot
		if json.Unmarshal(data, &snap) == nil {
			snaps = append(snaps, snap)
		}
	}
	return snaps, nil
}

func (ss *SessionStore) path(sessionID string) string {
	return filepath.Join(ss.dir, sessionID+".json")
}

func (ss *SessionStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ss.mu.Lock()
		cutoff := time.Now().Add(-24 * time.Hour)
		entries, _ := os.ReadDir(ss.dir)
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(ss.dir, e.Name()))
			}
		}
		ss.mu.Unlock()
	}
}
