package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
)

// Manager 管理子 Agent 的创建、执行和清理
type Manager struct {
	mu       sync.RWMutex
	agents   map[string]*SubAgent
	config   ManagerConfig
	nextID   int64
	sem      chan struct{} // 并发控制信号量
}

// NewManager 创建 AgentManager
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 100
	}
	if cfg.AgentTimeout <= 0 {
		cfg.AgentTimeout = 10 * time.Minute
	}
	return &Manager{
		agents: make(map[string]*SubAgent),
		config: cfg,
		sem:    make(chan struct{}, cfg.MaxConcurrent),
	}
}

// SetProgressFn replaces the progress callback and returns the previous one.
func (m *Manager) SetProgressFn(fn ProgressFunc) ProgressFunc {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.config.ProgressFn
	m.config.ProgressFn = fn
	return old
}

// Spawn 创建并启动一个子 Agent。
// toolFilter 限制可用工具（空/nil=当前注册的全部工具）。
func (m *Manager) Spawn(goal string, toolFilter []string, parentID string) *SubAgent {
	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("agent_%d", m.nextID)
	agent := &SubAgent{
		ID:         id,
		Goal:       goal,
		ToolFilter: toolFilter,
		Status:     StatusPending,
		StartedAt:  time.Now(),
		progressFn: m.config.ProgressFn, // capture at spawn time
	}
	m.agents[id] = agent
	m.mu.Unlock()

	// 异步启动
	go m.runAgent(agent, parentID)
	return agent
}

// SpawnParallel 批量并行派生多个子 Agent，等待全部完成。
func (m *Manager) SpawnParallel(tasks []SpawnTask, parentID string) []*Result {
	if len(tasks) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	results := make([]*Result, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t SpawnTask) {
			defer wg.Done()
			agent := m.Spawn(t.Goal, t.ToolFilter, parentID)
			// 等待这个 agent 完成
			m.waitFor(agent)
			results[idx] = &Result{
				AgentID: agent.ID,
				Goal:    agent.Goal,
				Status:  agent.Status,
				Summary: agent.Summary,
				Error:   agent.Error,
				Rounds:  agent.Round,
			}
		}(i, task)
	}
	wg.Wait()
	return results
}

// SpawnAsync 异步派生多个子 Agent，立即返回 Agent ID 列表。
// 所有 Agent 完成后通过 CompletionFn 回调注入结果到父会话。
func (m *Manager) SpawnAsync(tasks []SpawnTask, sessionID string) []string {
	if len(tasks) == 0 {
		return nil
	}

	ids := make([]string, len(tasks))
	for i, task := range tasks {
		agent := m.Spawn(task.Goal, task.ToolFilter, "")
		ids[i] = agent.ID
	}

	// 后台等待所有 Agent 完成，然后回调
	go func() {
		for _, id := range ids {
			m.waitForByID(id)
		}
		results := make([]*Result, len(tasks))
		for i, id := range ids {
			a := m.Get(id)
			if a != nil {
				results[i] = &Result{
					AgentID: a.ID, Goal: a.Goal, Status: a.Status,
					Summary: a.Summary, Error: a.Error, Rounds: a.Round,
				}
			}
		}
		if m.config.CompletionFn != nil {
			m.config.CompletionFn(sessionID, results)
		}
	}()

	return ids
}

// waitForByID is like waitFor but takes an agent ID.
func (m *Manager) waitForByID(id string) {
	if a := m.Get(id); a != nil {
		m.waitFor(a)
	}
}

// SpawnTask 批量派生任务的输入
type SpawnTask struct {
	Goal       string   `json:"goal"`
	ToolFilter []string `json:"tool_filter"`
}

// Get 获取指定 Agent
func (m *Manager) Get(id string) *SubAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[id]
}

// ListAll 列出所有 Agent
func (m *Manager) ListAll() []*SubAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SubAgent, 0, len(m.agents))
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result
}

// Cleanup 清理已完成/已过期的 Agent
func (m *Manager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for id, a := range m.agents {
		if a.Status == StatusDone || a.Status == StatusError {
			if a.FinishedAt.Before(cutoff) {
				delete(m.agents, id)
			}
		}
	}
}

// waitFor 轮询等待子 Agent 完成（带超时保护）
func (m *Manager) waitFor(agent *SubAgent) {
	timeout := m.config.AgentTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	deadline := time.After(timeout + 30*time.Second) // 比 agent 超时多 30s

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		m.mu.RLock()
		a := m.agents[agent.ID]
		m.mu.RUnlock()

		if a == nil || a.Status == StatusDone || a.Status == StatusError {
			return
		}

		select {
		case <-ticker.C:
			// continue polling
		case <-deadline:
			slog.Warn("waitFor timed out waiting for agent", "agent", agent.ID, "status", agent.Status)
			return
		}
	}
}

// runAgent 执行子 Agent 的独立 ReAct 循环
func (m *Manager) runAgent(agent *SubAgent, parentID string) {
	// 获取信号量（并发控制）
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	m.updateStatus(agent, StatusRunning, "")
	slog.Info("agent started", "id", agent.ID, "goal", agent.Goal, "tools", len(agent.ToolFilter))

	// 构建独立上下文（带超时）
	agentTimeout := m.config.AgentTimeout
	if agentTimeout <= 0 {
		agentTimeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()
	messages := []base.Message{
		{Role: "system", Content: fmt.Sprintf(
			"你是一个子任务执行 Agent。你的唯一目标是完成以下任务:\n%s\n\n"+
			"规则:\n- 只使用已分配的工具\n- 保持简洁高效\n- 完成后用一句话总结结果\n"+
			"- 最多执行 %d 轮", agent.Goal, m.config.MaxRounds,
		)},
	}

	// 过滤工具
	tools := m.filterTools(agent.ToolFilter)

	success := false
	var lastToolSig string
	loopCount := 0
	for round := 1; round <= m.config.MaxRounds; round++ {
		agent.Round = round
		m.updateProgress(agent, fmt.Sprintf("第 %d/%d 轮", round, m.config.MaxRounds))

		stream, err := m.config.Provider.ChatStream(ctx, messages, tools)
		if err != nil {
			m.updateStatus(agent, StatusError, "LLM 调用失败: "+err.Error())
			return
		}

		var contentBuf string
		var toolCalls []base.ToolCall

		for ev := range stream {
			switch ev.Type {
			case "content":
				contentBuf += ev.Content
			case "tool_call":
				toolCalls = append(toolCalls, base.ToolCall{
					ID: fmt.Sprintf("ca_%d", len(toolCalls)),
					Function: base.FunctionCall{Name: ev.Tool, Arguments: ev.Args},
				})
			case "error":
				m.updateStatus(agent, StatusError, "LLM 错误: "+ev.Content)
				return
			}
		}

		// 无工具调用 → Agent 完成
		if len(toolCalls) == 0 {
			m.updateStatus(agent, StatusDone, contentBuf)
			success = true
			_ = success
			return
		}

		// 有工具调用 → 执行然后继续下一轮
		messages = append(messages, base.Message{
			Role: "assistant", Content: contentBuf,
			ToolCalls: toolCalls, HasToolCalls: true,
		})

		for _, tc := range toolCalls {
		toolName := tc.Function.Name
		args := ParseArgsString(tc.Function.Arguments)

		m.updateProgress(agent, fmt.Sprintf("执行: %s", toolName))

		// Check context timeout before executing
		if ctx.Err() != nil {
		 toolResult := fmt.Sprintf("[%s 执行超时] 错误码: TIMEOUT 详情: Agent 上下文已过期", toolName)
		 messages = append(messages, base.Message{
		 Role: "tool", Content: toolResult, ToolCallID: tc.ID,
		})
		 m.updateStatus(agent, StatusError, "Agent 超时")
		return
		}

		tool, ok := m.config.Registry.Get(toolName)
		var toolResult string
		if !ok {
		toolResult = fmt.Sprintf("[%s 执行失败] 错误码: UNKNOWN_ERROR 详情: 未知工具 %s", toolName, toolName)
		slog.Warn("子Agent未知工具", "agent", agent.ID, "工具", toolName)
		} else if tool.HandlerV2 != nil {
		res := tool.HandlerV2(ctx, args)
		if res.Status == base.StatusError && res.Error != nil {
		toolResult = fmt.Sprintf("[%s 执行失败] 错误码: %s 详情: %s", toolName, res.Error.Code, res.Error.Message)
		 if res.Error.Retryable {
		   toolResult += fmt.Sprintf(" (可重试: %s)", res.Error.RetryHint)
						}
		  if res.Error.Fallback != "" {
		  toolResult += fmt.Sprintf(" 建议降级: %s", res.Error.Fallback)
		  }
						slog.Warn("子Agent工具失败", "agent", agent.ID, "工具", toolName, "错误", res.Error.Message)
		 } else {
		  toolResult = res.Summary
		  if toolResult == "" {
		   toolResult = fmt.Sprintf("[%s 执行成功]", toolName)
						}
					}
				} else {
					data, err := tool.Handler(ctx, args)
					if err != nil {
						toolResult = fmt.Sprintf("[%s 执行失败] 错误码: EXEC_FAILED 详情: %v", toolName, err)
					} else {
						toolResult = data
					}
				}

				messages = append(messages, base.Message{
					Role: "tool", Content: toolResult, ToolCallID: tc.ID,
				})
		}

		// Loop detection: same tool signature 3 consecutive rounds
		sig := ""
		for _, tc := range toolCalls {
			sig += tc.Function.Name + ";"
		}
		if sig == lastToolSig {
			loopCount++
			if loopCount >= 3 {
				m.updateStatus(agent, StatusError, "检测到工具调用循环: "+sig)
				slog.Warn("sub-agent loop detected", "agent", agent.ID, "sig", sig)
				return
			}
		} else {
			lastToolSig = sig
			loopCount = 0
		}
	}

	m.updateStatus(agent, StatusError, fmt.Sprintf("超过最大轮次 %d", m.config.MaxRounds))
}

func (m *Manager) filterTools(filter []string) []base.ToolDef {
	all := m.config.Registry.All()

	if len(filter) == 0 || (len(filter) == 1 && filter[0] == "*") {
		return all
	}

	allow := make(map[string]bool, len(filter))
	for _, f := range filter {
		allow[f] = true
	}

	var filtered []base.ToolDef
	for _, t := range all {
		if allow[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (m *Manager) updateStatus(agent *SubAgent, status Status, summary string) {
	m.mu.Lock()
	agent.Status = status
	agent.Summary = summary
	if status == StatusError {
		agent.Error = summary
	}
	if status == StatusDone || status == StatusError {
		agent.FinishedAt = time.Now()
	}
	m.mu.Unlock()

	eventType := "agent_result"
	if agent != nil && agent.progressFn != nil {
		agent.progressFn(agent, eventType)
	}

	slog.Info("agent finished", "id", agent.ID, "status", status, "rounds", agent.Round)
}

func (m *Manager) updateProgress(agent *SubAgent, progress string) {
	m.mu.Lock()
	agent.Progress = progress
	m.mu.Unlock()

	if agent != nil && agent.progressFn != nil {
		agent.progressFn(agent, "agent_progress")
	}
}

