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
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        MemoryType `json:"type"`
	Content     string     `json:"content"`
	Source      string     `json:"source"` // "file" or "agent"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Repository persists memories.
type Repository interface {
	EnsureSchema(ctx context.Context) error
	GetAll(ctx context.Context) ([]*Memory, error)
	GetByName(ctx context.Context, name string) (*Memory, error)
	Save(ctx context.Context, m *Memory) error
	Delete(ctx context.Context, name string) error
}
