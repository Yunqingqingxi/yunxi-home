package toolreg

import (
	"sync"
	"time"
)

// ── 安装任务追踪（前后端共享状态，刷新不丢失）──

// InstallStep 安装步骤
type InstallStep struct {
	Step    string `json:"step"`
	Status  string `json:"status"` // "pending" | "running" | "done" | "error"
	Message string `json:"message"`
}

// InstallTask 安装任务
type InstallTask struct {
	ID        string         `json:"id"`
	Package   string         `json:"package"`
	Status    string         `json:"status"` // "running" | "done" | "error"
	Progress  int            `json:"progress"`
	Steps     []InstallStep  `json:"steps"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	Done      chan struct{}  `json:"-"` // 内部信号
}

// InstallTracker 全局安装任务追踪器
type InstallTracker struct {
	mu    sync.Mutex
	tasks map[string]*InstallTask
}

var globalTracker = &InstallTracker{tasks: make(map[string]*InstallTask)}

// GetInstallTracker 获取全局追踪器
func GetInstallTracker() *InstallTracker { return globalTracker }

// CreateTask 创建新任务
func (t *InstallTracker) CreateTask(id, pkg string) *InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	task := &InstallTask{
		ID:        id,
		Package:   pkg,
		Status:    "running",
		Progress:  0,
		Steps:     make([]InstallStep, 0),
		CreatedAt: time.Now(),
		Done:      make(chan struct{}, 1),
	}
	t.tasks[id] = task
	return task
}

// GetTask 获取任务
func (t *InstallTracker) GetTask(id string) *InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.tasks[id]
}

// ListTasks 列出所有活跃任务
func (t *InstallTracker) ListTasks() []*InstallTask {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]*InstallTask, 0, len(t.tasks))
	for _, tk := range t.tasks {
		result = append(result, tk)
	}
	return result
}

// AddStep 添加步骤
func (t *InstallTracker) AddStep(id string, step InstallStep) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tk, ok := t.tasks[id]; ok {
		tk.Steps = append(tk.Steps, step)
	}
}

// UpdateProgress 更新进度和状态
func (t *InstallTracker) UpdateProgress(id string, progress int, status string, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if tk, ok := t.tasks[id]; ok {
		tk.Progress = progress
		if status != "" { tk.Status = status }
		if errMsg != "" { tk.Error = errMsg }
		if status == "done" || status == "error" {
			select {
			case tk.Done <- struct{}{}:
			default:
			}
		}
	}
}

// CleanOld 清理超过 1 小时的已完成任务
func (t *InstallTracker) CleanOld() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, tk := range t.tasks {
		if tk.Status != "running" && time.Since(tk.CreatedAt) > 1*time.Hour {
			delete(t.tasks, id)
		}
	}
}
