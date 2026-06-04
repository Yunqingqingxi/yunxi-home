package memory

import (
	"context"
	"time"
)

// MemoryType classifies a memory entry.
type MemoryType string

const (
	TypeUser      MemoryType = "user"
	TypeProject   MemoryType = "project"
	TypeReference MemoryType = "reference"
	TypeFeedback  MemoryType = "feedback"
)

// Memory is a single persistent memory entry.
type Memory struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Type         MemoryType `json:"type"`
	Content      string     `json:"content"`
	Source       string     `json:"source"`    // "file" or "agent"
	ContextTags  []string   `json:"context_tags,omitempty"` // 关联的专用上下文ID，空=通用记忆
	Importance   float64    `json:"importance"` // 0.0-1.0, decays over time
	AccessCount  int        `json:"access_count"`
	LastAccessed time.Time  `json:"last_accessed_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ComputeImportance calculates the current importance score considering decay.
// Importance decays by 10% per week since last access.
func (m *Memory) ComputeImportance() float64 {
	if m.Importance == 0 {
		return 0.5 // default
	}
	weeksSinceAccess := time.Since(m.LastAccessed).Hours() / (24 * 7)
	decay := 1.0 / (1.0 + weeksSinceAccess*0.1)
	score := m.Importance * decay
	// Boost for frequently accessed memories
	if m.AccessCount > 5 {
		score *= 1.1
	}
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.1 {
		score = 0.1
	}
	return score
}

// Repository persists memories.
type Repository interface {
	EnsureSchema(ctx context.Context) error
	GetAll(ctx context.Context) ([]*Memory, error)
	GetByName(ctx context.Context, name string) (*Memory, error)
	Save(ctx context.Context, m *Memory) error
	Delete(ctx context.Context, name string) error
}
