// Package async 提供后台异步任务调度：长操作不阻塞对话，完成后回调注入。
package async

import (
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

var log = logger.ForComponent("async")

// Status 任务状态
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Task 后台任务
type Task struct {
	ID        string     `json:"id"`
	Tool      string     `json:"tool"`
	Args      map[string]any `json:"args"`
	SessionID string     `json:"session_id"`
	Status    Status     `json:"status"`
	Progress  int        `json:"progress"`  // 0-100
	Message   string     `json:"message"`   // 状态描述
	Result    string     `json:"result,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	DoneAt    time.Time  `json:"done_at,omitempty"`
	cancel    chan struct{}
}

// ProgressEvent 进度事件（通过回调推送）
type ProgressEvent struct {
	TaskID   string `json:"task_id"`
	Tool     string `json:"tool"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
	Status   Status `json:"status"`
}

// InjectFunc 消息注入回调（由 chat.go 提供，将结果注入会话对话流）
type InjectFunc func(sessionID, content string, priority string)

// ProgressFunc SSE 进度推送回调
type ProgressFunc func(sessionID string, ev ProgressEvent)

// Executor 后台异步任务执行器
type Executor struct {
	tasks        map[string]*Task
	mu           sync.RWMutex
	workerCount  int
	workerCh     chan *Task
	injectFn     InjectFunc
	progressFn   ProgressFunc
}

// New 创建后台任务执行器
func New(workerCount int, injectFn InjectFunc, progressFn ProgressFunc) *Executor {
	if workerCount <= 0 {
		workerCount = 3
	}
	e := &Executor{
		tasks:       make(map[string]*Task),
		workerCount: workerCount,
		workerCh:    make(chan *Task, 64),
		injectFn:    injectFn,
		progressFn:  progressFn,
	}
	for i := 0; i < workerCount; i++ {
		go e.worker(i)
	}
	go e.cleanupLoop()
	return e
}

// Submit 提交后台任务，返回 taskID
func (e *Executor) Submit(tool, sessionID string, args map[string]any, runner func(ctx *TaskContext) (string, error)) string {
	id := fmt.Sprintf("bg_%s_%d", sessionID[:min(8, len(sessionID))], time.Now().UnixNano())

	task := &Task{
		ID:        id,
		Tool:      tool,
		Args:      args,
		SessionID: sessionID,
		Status:    StatusPending,
		Progress:  0,
		Message:   "等待执行...",
		StartedAt: time.Now(),
		cancel:    make(chan struct{}, 1),
	}

	e.mu.Lock()
	e.tasks[id] = task
	e.mu.Unlock()

	// 提交到 worker 队列（非阻塞）
	select {
	case e.workerCh <- task:
		// 闭包捕获 runner
		go func() {
			task.Status = StatusRunning
			task.Message = "正在执行..."
			e.emitProgress(task)

			ctx := &TaskContext{
				TaskID:    id,
				SessionID: sessionID,
				Args:      args,
				Cancel:    task.cancel,
				SetProgress: func(p int, msg string) {
					e.mu.Lock()
					task.Progress = p
					task.Message = msg
					e.mu.Unlock()
					e.emitProgress(task)
				},
			}

			result, err := runner(ctx)

			e.mu.Lock()
			if err != nil {
				task.Status = StatusFailed
				task.Error = err.Error()
				task.Message = "执行失败"
			} else {
				task.Status = StatusCompleted
				task.Result = result
				task.Message = "执行完成"
				task.Progress = 100
			}
			task.DoneAt = time.Now()
			e.mu.Unlock()

			// 回调注入
			e.emitProgress(task)
			if e.injectFn != nil {
				priority := "info"
				content := ""
				if task.Status == StatusCompleted {
					priority = "info"
					content = fmt.Sprintf("[后台任务完成] %s 执行成功: %s", tool, result)
				} else {
					priority = "warning"
					content = fmt.Sprintf("[后台任务失败] %s: %s", tool, task.Error)
				}
				e.injectFn(sessionID, content, priority)
			}
		}()
	default:
		task.Status = StatusFailed
		task.Message = "任务队列已满"
	}

	return id
}

// worker 后台 worker
func (e *Executor) worker(id int) {
	log.Info("async worker started", "worker", id)
	// Workers just acknowledge that tasks are being processed
	// Actual work is done in goroutines launched by Submit
}

// Get 获取任务状态
func (e *Executor) Get(taskID string) *Task {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tasks[taskID]
}

// List 列出所有任务
func (e *Executor) List() []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Task, 0, len(e.tasks))
	for _, t := range e.tasks {
		result = append(result, t)
	}
	return result
}

// ListBySession 按会话列出任务
func (e *Executor) ListBySession(sessionID string) []*Task {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Task
	for _, t := range e.tasks {
		if t.SessionID == sessionID {
			result = append(result, t)
		}
	}
	return result
}

// Cancel 取消任务
func (e *Executor) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	task, ok := e.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status != StatusRunning && task.Status != StatusPending {
		return fmt.Errorf("task %s is %s, cannot cancel", taskID, task.Status)
	}
	task.Status = StatusCancelled
	task.Message = "已取消"
	task.DoneAt = time.Now()
	select {
	case task.cancel <- struct{}{}:
	default:
	}
	if e.injectFn != nil {
		e.injectFn(task.SessionID, fmt.Sprintf("[后台任务取消] %s 已取消", task.Tool), "info")
	}
	return nil
}

func (e *Executor) emitProgress(task *Task) {
	if e.progressFn != nil {
		e.progressFn(task.SessionID, ProgressEvent{
			TaskID:   task.ID,
			Tool:     task.Tool,
			Progress: task.Progress,
			Message:  task.Message,
			Status:   task.Status,
		})
	}
}

func (e *Executor) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		e.mu.Lock()
		cutoff := time.Now().Add(-1 * time.Hour)
		for id, t := range e.tasks {
			if t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled {
				if t.DoneAt.Before(cutoff) {
					delete(e.tasks, id)
				}
			}
		}
		e.mu.Unlock()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TaskContext 任务执行上下文
type TaskContext struct {
	TaskID      string
	SessionID   string
	Args        map[string]any
	Cancel      chan struct{}
	SetProgress func(percent int, message string)
}
