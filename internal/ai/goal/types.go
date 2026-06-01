// Package goal 提供持久化多步目标追踪系统。
// 支持跨会话恢复、进度推送、自动摘要。
package goal

import (
	"sync"
	"time"
)

// Status 目标整体状态
type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// StepStatus 步骤状态
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepInProgress StepStatus = "in_progress"
	StepDone       StepStatus = "done"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
)

// Goal 多步目标
type Goal struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Steps       []Step     `json:"steps"`
	Status      Status     `json:"status"`
	Progress    int        `json:"progress"` // 0-100
	CurrentStep int        `json:"current_step"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SessionID   string     `json:"session_id"`
	Result      string     `json:"result,omitempty"`
}

// Step 单个步骤
type Step struct {
	ID       int            `json:"id"`
	Title    string         `json:"title"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
	Status   StepStatus     `json:"status"`
	Result   string         `json:"result,omitempty"`
	Error    string         `json:"error,omitempty"`
	Depends  []int          `json:"depends,omitempty"`
	Duration int64          `json:"duration_ms,omitempty"`
}

// ProgressEvent 进度事件（通过 SSE 推送）
type ProgressEvent struct {
	GoalID     string     `json:"goal_id"`
	StepID     int        `json:"step_id"`
	StepTitle  string     `json:"step_title"`
	Status     StepStatus `json:"status"`
	Progress   int        `json:"progress"` // 整体进度 0-100
	Message    string     `json:"message"`
}

// Manager 目标管理器
type Manager struct {
	goals   map[string]*Goal // goalID → Goal
	mu      sync.RWMutex
}

// NewManager 创建目标管理器
func NewManager() *Manager {
	return &Manager{goals: make(map[string]*Goal)}
}

// Create 创建一个新目标
func (m *Manager) Create(id, title, description, sessionID string, steps []Step) *Goal {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := &Goal{
		ID:          id,
		Title:       title,
		Description: description,
		Steps:       steps,
		Status:      StatusActive,
		Progress:    0,
		CurrentStep: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		SessionID:   sessionID,
	}
	if g.Steps == nil {
		g.Steps = []Step{}
	}
	m.goals[id] = g
	return g
}

// Get 获取目标
func (m *Manager) Get(id string) *Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.goals[id]
}

// List 列出所有目标
func (m *Manager) List() []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Goal, 0, len(m.goals))
	for _, g := range m.goals {
		result = append(result, g)
	}
	return result
}

// ListBySession 按会话列出目标
func (m *Manager) ListBySession(sessionID string) []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Goal
	for _, g := range m.goals {
		if g.SessionID == sessionID {
			result = append(result, g)
		}
	}
	return result
}

// ListActive 列出活跃的目标（含跨会话）
func (m *Manager) ListActive() []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Goal
	for _, g := range m.goals {
		if g.Status == StatusActive || g.Status == StatusPending {
			result = append(result, g)
		}
	}
	return result
}

// AdvanceStep 推进当前步骤
func (m *Manager) AdvanceStep(goalID string, result string, err error) *ProgressEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.goals[goalID]
	if !ok {
		return nil
	}

	step := &g.Steps[g.CurrentStep]
	if err != nil {
		step.Status = StepFailed
		step.Error = err.Error()
	} else {
		step.Status = StepDone
		step.Result = result
	}

	g.CurrentStep++
	g.UpdatedAt = time.Now()

	// 计算进度
	done := 0
	for _, s := range g.Steps {
		if s.Status == StepDone || s.Status == StepSkipped {
			done++
		}
	}
	if len(g.Steps) > 0 {
		g.Progress = done * 100 / len(g.Steps)
	} else {
		g.Progress = 100
	}

	if done >= len(g.Steps) {
		if g.CurrentStep > 0 && g.Steps[g.CurrentStep-1].Status == StepFailed {
			g.Status = StatusFailed
		} else {
			g.Status = StatusCompleted
		}
	}

	return &ProgressEvent{
		GoalID:    goalID,
		StepID:    step.ID,
		StepTitle: step.Title,
		Status:    step.Status,
		Progress:  g.Progress,
		Message:   step.Result,
	}
}

// MarkStep 手动标记步骤状态
func (m *Manager) MarkStep(goalID string, stepID int, status StepStatus, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.goals[goalID]
	if !ok {
		return
	}
	for i := range g.Steps {
		if g.Steps[i].ID == stepID {
			g.Steps[i].Status = status
			g.Steps[i].Result = result
			g.UpdatedAt = time.Now()
			break
		}
	}

	done := 0
	for _, s := range g.Steps {
		if s.Status == StepDone || s.Status == StepSkipped {
			done++
		}
	}
	if len(g.Steps) > 0 {
		g.Progress = done * 100 / len(g.Steps)
	}
}

// Abandon 放弃目标
func (m *Manager) Abandon(goalID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.goals[goalID]; ok {
		g.Status = StatusFailed
		g.UpdatedAt = time.Now()
	}
}

// Delete 删除目标
func (m *Manager) Delete(goalID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.goals, goalID)
}

// ResumePrompt 为目标生成恢复提示词
func (m *Manager) ResumePrompt(goalID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	g, ok := m.goals[goalID]
	if !ok || g.Status == StatusCompleted || g.Status == StatusFailed {
		return ""
	}

	p := "你有一个未完成的目标需要继续执行:\n"
	p += "目标: " + g.Title + "\n"
	p += "描述: " + g.Description + "\n"
	p += "进度: " + itoa(g.Progress) + "%\n\n"
	p += "已完成的步骤:\n"
	for _, s := range g.Steps {
		switch s.Status {
		case StepDone:
			p += "  ✅ 步骤" + itoa(s.ID) + ": " + s.Title + " — " + s.Result + "\n"
		case StepFailed:
			p += "  ❌ 步骤" + itoa(s.ID) + ": " + s.Title + " — " + s.Error + "\n"
		case StepInProgress:
			p += "  🔄 步骤" + itoa(s.ID) + ": " + s.Title + " (进行中)\n"
		}
	}
	p += "\n待执行的步骤:\n"
	for _, s := range g.Steps {
		if s.Status == StepPending {
			p += "  ⏳ 步骤" + itoa(s.ID) + ": " + s.Title + "\n"
		}
	}
	p += "\n请从第一个待执行步骤继续。"
	return p
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
