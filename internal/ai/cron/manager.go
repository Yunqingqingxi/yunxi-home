package cron

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// InjectFunc 是注入消息到会话的回调函数类型
type InjectFunc func(sessionID, prompt string)

// Manager 管理所有 AI 创建的定时任务
type Manager struct {
	mu       sync.RWMutex
	tasks    map[string]*ScheduledTask
	injectFn InjectFunc
	nextID   int64
	stopCh   chan struct{}
	// 持久化（可选）
	repo TaskRepository
}

// TaskRepository 持久化接口
type TaskRepository interface {
	Save(task *ScheduledTask) error
	Delete(id string) error
	ListBySession(sessionID string) ([]*ScheduledTask, error)
	ListAll() ([]*ScheduledTask, error)
}

// NewManager 创建 CronManager，injectFn 用于向会话注入消息。
// repo 传 nil 表示仅内存模式。
func NewManager(injectFn InjectFunc, repo TaskRepository) *Manager {
	m := &Manager{
		tasks:    make(map[string]*ScheduledTask),
		injectFn: injectFn,
		repo:     repo,
		stopCh:   make(chan struct{}),
	}
	// 从数据库恢复
	if repo != nil {
		all, err := repo.ListAll()
		if err == nil {
			for _, t := range all {
				m.tasks[t.ID] = t
			}
			slog.Info("cron tasks restored from DB", "count", len(all))
		}
	}
	go m.runLoop()
	return m
}

// Stop 停止后台检查 loop
func (m *Manager) Stop() {
	close(m.stopCh)
}

// Create 创建定时任务
func (m *Manager) Create(sessionID, cronExpr, prompt string, recurring bool) (*ScheduledTask, error) {
	nextRun, err := NextRunTime(cronExpr, time.Now())
	if err != nil {
		return nil, fmt.Errorf("无效的 cron 表达式: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	task := &ScheduledTask{
		ID:        fmt.Sprintf("cron_%d", m.nextID),
		SessionID: sessionID,
		CronExpr:  cronExpr,
		Prompt:    prompt,
		Recurring: recurring,
		CreatedAt: time.Now(),
		NextRunAt: nextRun,
	}
	m.tasks[task.ID] = task

	if m.repo != nil {
		_ = m.repo.Save(task)
	}

	slog.Info("cron task created",
		"id", task.ID,
		"session", sessionID,
		"cron", cronExpr,
		"next", nextRun.Format(time.RFC3339),
	)
	return task, nil
}

// Delete 删除定时任务
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return false
	}
	delete(m.tasks, id)
	if m.repo != nil {
		_ = m.repo.Delete(id)
	}
	return true
}

// ListBySession 列出指定会话的定时任务
func (m *Manager) ListBySession(sessionID string) []*ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ScheduledTask
	for _, t := range m.tasks {
		if t.SessionID == sessionID {
			result = append(result, t)
		}
	}
	return result
}

// ListAll 列出所有定时任务
func (m *Manager) ListAll() []*ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ScheduledTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// runLoop 后台检查循环，每分钟触发一次
func (m *Manager) runLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case now := <-ticker.C:
			m.checkAndTrigger(now)
		}
	}
}

func (m *Manager) checkAndTrigger(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, task := range m.tasks {
		if now.After(task.NextRunAt) || now.Equal(task.NextRunAt) {
			slog.Info("cron task triggered", "id", id, "session", task.SessionID)

			// 注入消息到会话
			if m.injectFn != nil {
				m.injectFn(task.SessionID, task.Prompt)
			}

			task.LastRanAt = now

			if task.Recurring {
				next, err := NextRunTime(task.CronExpr, now)
				if err != nil {
					slog.Warn("cron task failed to calc next run, removing", "id", id, "error", err)
					delete(m.tasks, id)
					if m.repo != nil {
						_ = m.repo.Delete(id)
					}
					continue
				}
				task.NextRunAt = next
			} else {
				// 一次性任务，删除
				delete(m.tasks, id)
				if m.repo != nil {
					_ = m.repo.Delete(id)
				}
			}
		}
	}
}

// ── 简易 Cron 解析器（5 字段：min hour dom month dow） ──

// NextRunTime 根据 cron 表达式计算下一次执行时间
func NextRunTime(cronExpr string, after time.Time) (time.Time, error) {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return time.Time{}, fmt.Errorf("cron 表达式需要 5 个字段，得到 %d", len(parts))
	}

	// 从 after 的下分钟开始搜索
	t := after.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 525600; i++ { // 最多搜索一年
		if matchField(parts[0], t.Minute(), 0, 59) &&
			matchField(parts[1], t.Hour(), 0, 23) &&
			matchField(parts[2], t.Day(), 1, 31) &&
			matchField(parts[3], int(t.Month()), 1, 12) &&
			matchField(parts[4], int(t.Weekday()), 0, 6) {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("未能找到 cron 的下一次匹配时间")
}

func matchField(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}

	// 逗号分隔
	for _, part := range strings.Split(field, ",") {
		if matchPart(strings.TrimSpace(part), value, min, max) {
			return true
		}
	}
	return false
}

func matchPart(part string, value, min, max int) bool {
	// */N 步进
	if strings.HasPrefix(part, "*/") {
		step, err := strconv.Atoi(part[2:])
		if err != nil {
			return false
		}
		return (value-min)%step == 0
	}

	// N-M 范围
	if strings.Contains(part, "-") {
		rangeParts := strings.SplitN(part, "-", 2)
		lo, err1 := strconv.Atoi(rangeParts[0])
		hi, err2 := strconv.Atoi(rangeParts[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return value >= lo && value <= hi
	}

	// 精确值
	n, err := strconv.Atoi(part)
	if err != nil {
		return false
	}
	return value == n
}

// DescribeCron 返回 cron 表达式的人类可读描述
func DescribeCron(cronExpr string) string {
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return cronExpr
	}

	// 简单模式匹配
	switch {
	case parts[0] == "*/5" && parts[1] == "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*":
		return "每5分钟"
	case parts[0] == "*/10" && parts[1] == "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*":
		return "每10分钟"
	case parts[0] == "*/30" && parts[1] == "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*":
		return "每30分钟"
	case parts[0] == "0" && parts[1] == "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*":
		return "每小时"
	case parts[0] == "0" && strings.HasPrefix(parts[1], "*/"):
		step := parts[1][2:]
		return "每" + step + "小时"
	default:
		return cronExpr
	}
}
