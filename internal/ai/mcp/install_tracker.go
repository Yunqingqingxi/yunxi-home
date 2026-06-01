package mcp

import (
	"sync"
	"time"
)

// ── InstallTracker ──────────────────────────────────────────────────

// InstallStep records one step of an installation process.
type InstallStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // "pending" | "running" | "done" | "error"
	Message string `json:"message"`
}

// InstallTask tracks the progress of a single MCP installation.
type InstallTask struct {
	ID        string        `json:"id"`
	Package   string        `json:"package"`
	Status    string        `json:"status"` // "running" | "done" | "error"
	Progress  int           `json:"progress"`
	Steps     []InstallStep `json:"steps"`
	Error     string        `json:"error,omitempty"`
	CreatedAt time.Time     `json:"created_at"`

	mu   sync.Mutex
	done chan struct{}
}

func (t *InstallTask) addStep(step, status, msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Steps = append(t.Steps, InstallStep{Step: step, Status: status, Message: msg})
}

func (t *InstallTask) updateProgress(pct int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Progress = pct
}

func (t *InstallTask) markDone() {
	t.mu.Lock()
	t.Status = "done"
	t.mu.Unlock()
	select {
	case t.done <- struct{}{}:
	default:
	}
}

// InstallTracker manages installation task state (not a global singleton).
type InstallTracker struct {
	mu    sync.Mutex
	tasks map[string]*InstallTask
}

func newInstallTracker() *InstallTracker {
	return &InstallTracker{tasks: make(map[string]*InstallTask)}
}

func (t *InstallTracker) createTask(id, pkg string) *InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	task := &InstallTask{
		ID:        id,
		Package:   pkg,
		Status:    "running",
		Steps:     make([]InstallStep, 0),
		CreatedAt: time.Now(),
		done:      make(chan struct{}, 1),
	}
	t.tasks[id] = task
	return task
}

// Get returns a task by ID.
func (t *InstallTracker) Get(id string) *InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tasks[id]
}

// List returns all tasks.
func (t *InstallTracker) List() []*InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]*InstallTask, 0, len(t.tasks))
	for _, tk := range t.tasks {
		result = append(result, tk)
	}
	return result
}

// CleanOld removes tasks older than 1 hour.
func (t *InstallTracker) CleanOld() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, tk := range t.tasks {
		if tk.Status == "done" && time.Since(tk.CreatedAt) > 1*time.Hour {
			delete(t.tasks, id)
		}
	}
}
