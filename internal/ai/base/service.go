package base

import (
	"context"

	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// ChatService defines the AI chat service interface.
type ChatService interface {
	StreamChat(ctx context.Context, sessionID, userMessage string, model ...string) <-chan ChatStreamEvent
	GetHints(ctx context.Context, sessionID string) []string
	ClearSession(sessionID string)
	CompactSession(sessionID string) string
	ClearAllSessions()
	ListSessions() []models.ChatSession
	GetSessionHistory(sessionID string) (*models.ChatSessionDetail, error)
	GetToolsJSON() []byte
	// PlanMode 是否启用计划模式
	PlanMode() bool
	// Goal management
	GetActiveGoal(sessionID string) *GoalInfo
	CreateGoal(sessionID, title, description string, steps []GoalStepInfo) string
	AbandonGoal(goalID string)
	// Cross-session
	RegisterSession(sessionID, title string)
	UnregisterSession(sessionID string)
	// Message editing (fork)
	EditMessage(sessionID string, messageIndex int, newContent string) ([]map[string]any, error)
	ForkAt(sessionID string, messageIndex int) ([]map[string]any, error)
	GetRawHistory(sessionID string) ([]map[string]any, error)
	// Injection
	InjectMessage(sessionID, content string)
	HasActiveSession(sessionID string) bool
	ListActiveSessions() []string
}

// GoalInfo 目标信息（避免 import cycle）
type GoalInfo struct {
	ID          string
	Title       string
	Description string
	Progress    int
	Status      string
	Steps       []GoalStepInfo
}

// GoalStepInfo 步骤信息
type GoalStepInfo struct {
	ID     int
	Title  string
	Tool   string
	Status string
	Result string
}
