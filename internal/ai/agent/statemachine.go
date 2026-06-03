// Package agent 提供子 Agent 派生和并行执行能力。
package agent

import (
	"fmt"

	"sync"
	"time"
)

// AgentState 统一状态枚举（主Agent、子Agent、工具共用）
type AgentState string

const (
	StateStart        AgentState = "start"         // 初始态：实例已创建，尚未开始执行
	StateReasoning    AgentState = "reasoning"      // 推理态：正在分析/规划
	StateExecuting    AgentState = "executing"      // 执行态：正在调用工具
	StateWaitingLock  AgentState = "waiting_lock"   // 等待锁：资源被占用，等待释放
	StateWaitingHuman AgentState = "waiting_human"  // 等待用户：等待确认或交互输入
	StateDelegate     AgentState = "delegate"       // 委派态：有活跃子Agent异步执行
	StateSuspended    AgentState = "suspended"      // 暂停态：用户暂停，上下文已保存
	StateTimeout      AgentState = "timeout"        // 超时暂态：执行超时，可重试或转错误
	StateRetry        AgentState = "retry"          // 重试态：自动重试中
	StateDone         AgentState = "done"           // 正常终态：任务成功完成
	StateFailed       AgentState = "failed"         // 错误终态：不可恢复的错误
	StateCancel       AgentState = "cancel"         // 取消终态：被强制终止
)

// AgentRole 角色枚举
type AgentRole string

const (
	RoleExecutor   AgentRole = "executor"   // 执行者：执行分配的任务
	RoleSupervisor AgentRole = "supervisor" // 监督者：可调度子任务、查看其他Agent
	RoleManager    AgentRole = "manager"    // 管理者：可协调多个Agent、分配资源
)

// TransitionEvent 状态转换事件
type TransitionEvent string

const (
	EvTaskAssigned    TransitionEvent = "task_assigned"    // INIT → REASONING
	EvPlanReady       TransitionEvent = "plan_ready"       // REASONING → EXECUTING
	EvNeedInput       TransitionEvent = "need_input"       // REASONING → WAITING_HUMAN
	EvInputReceived   TransitionEvent = "input_received"   // WAITING_HUMAN → REASONING
	EvResourceConflict TransitionEvent = "resource_conflict" // EXECUTING → WAITING_LOCK
	EvLockAcquired    TransitionEvent = "lock_acquired"    // WAITING_LOCK → EXECUTING
	EvDelegate        TransitionEvent = "delegate"         // EXECUTING → DELEGATE (标记)
	EvSubagentDone    TransitionEvent = "subagent_done"    // DELEGATE → EXECUTING
	EvInterrupt       TransitionEvent = "interrupt"        // → SUSPENDED
	EvResume          TransitionEvent = "resume"           // SUSPENDED → REASONING
	EvTimeout         TransitionEvent = "timeout"          // → TIMEOUT
	EvRetry           TransitionEvent = "retry"            // TIMEOUT/FAILED → RETRY
	EvRetryComplete   TransitionEvent = "retry_complete"   // RETRY → EXECUTING
	EvTaskComplete    TransitionEvent = "task_complete"    // → DONE
	EvError           TransitionEvent = "error"            // → FAILED
	EvCancel          TransitionEvent = "cancel"           // → CANCEL
)

// TransitionRecord 单次状态转换记录
type TransitionRecord struct {
	From      AgentState      `json:"from"`
	To        AgentState      `json:"to"`
	Event     TransitionEvent `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	Reason    string          `json:"reason,omitempty"`
}

// StateChangeCallback 状态变更回调
type StateChangeCallback func(from, to AgentState, event TransitionEvent, reason string)

// ── 转换规则表 ──

// validTransitions 定义合法的状态转换
// key = 当前状态，value = map[事件]目标状态
var validTransitions = map[AgentState]map[TransitionEvent]AgentState{
	StateStart: {
		EvTaskAssigned: StateReasoning,
		EvCancel:       StateCancel,
	},
	StateReasoning: {
		EvPlanReady:    StateExecuting,
		EvNeedInput:    StateWaitingHuman,
		EvDelegate:     StateDelegate,
		EvTaskComplete: StateDone,
		EvError:        StateFailed,
		EvCancel:       StateCancel,
		EvTimeout:      StateTimeout,
		EvInterrupt:    StateSuspended,
	},
	StateExecuting: {
		EvResourceConflict: StateWaitingLock,
		EvDelegate:         StateDelegate, // 进入 delegate 标记
		EvTaskComplete:     StateDone,
		EvError:            StateFailed,
		EvCancel:           StateCancel,
		EvTimeout:          StateTimeout,
		EvInterrupt:        StateSuspended,
		EvNeedInput:        StateWaitingHuman,
	},
	StateWaitingLock: {
		EvLockAcquired: StateExecuting,
		EvTimeout:      StateTimeout,
		EvCancel:       StateCancel,
		EvError:        StateFailed,
	},
	StateWaitingHuman: {
		EvInputReceived: StateReasoning, // 收到输入后重新推理
		EvTimeout:       StateTimeout,
		EvCancel:        StateCancel,
		EvError:         StateFailed,
	},
	StateDelegate: {
		EvSubagentDone: StateExecuting, // 子Agent完成后继续
		EvTaskComplete: StateDone,      // 委派 + 无更多任务
		EvError:        StateFailed,
		EvCancel:       StateCancel,
		EvTimeout:      StateTimeout,
		EvInterrupt:    StateSuspended,
	},
	StateSuspended: {
		EvResume: StateReasoning, // 恢复后重新推理
		EvCancel: StateCancel,
		EvError:  StateFailed,
	},
	StateTimeout: {
		EvRetry:      StateRetry,
		EvCancel:     StateCancel,
		EvError:      StateFailed,
		EvInterrupt:  StateSuspended,
	},
	StateRetry: {
		EvRetryComplete: StateExecuting,
		EvError:         StateFailed,
		EvCancel:        StateCancel,
	},
	// 终态不允许任何转换
	StateDone:   {},
	StateFailed: {},
	StateCancel: {},
}

// isTerminal 判断是否为终态
func (s AgentState) IsTerminal() bool {
	return s == StateDone || s == StateFailed || s == StateCancel
}

// isActive 判断是否为活动态（正在执行中）
func (s AgentState) IsActive() bool {
	switch s {
	case StateReasoning, StateExecuting, StateRetry, StateDelegate:
		return true
	default:
		return false
	}
}

// isWaiting 判断是否为等待态
func (s AgentState) IsWaiting() bool {
	return s == StateWaitingLock || s == StateWaitingHuman
}

// StateMachine 统一状态机
type StateMachine struct {
	mu          sync.RWMutex
	current     AgentState
	role        AgentRole
	history     []TransitionRecord
	maxHistory  int
	callbacks   []StateChangeCallback
	startedAt   time.Time
	lastTransition time.Time
}

// NewStateMachine 创建新的状态机
func NewStateMachine(role AgentRole) *StateMachine {
	now := time.Now()
	return &StateMachine{
		current:    StateStart,
		role:       role,
		history:    make([]TransitionRecord, 0, 32),
		maxHistory: 100,
		callbacks:  make([]StateChangeCallback, 0, 4),
		startedAt:  now,
		lastTransition: now,
	}
}

// NewSubAgentStateMachine 创建子Agent状态机（默认 EXECUTOR 角色）
func NewSubAgentStateMachine() *StateMachine {
	return NewStateMachine(RoleExecutor)
}

// NewMainAgentStateMachine 创建主Agent状态机（默认 MANAGER 角色）
func NewMainAgentStateMachine() *StateMachine {
	return NewStateMachine(RoleManager)
}

// CurrentState 返回当前状态
func (sm *StateMachine) CurrentState() AgentState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// Role 返回当前角色
func (sm *StateMachine) Role() AgentRole {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.role
}

// SetRole 更新角色
func (sm *StateMachine) SetRole(role AgentRole) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	old := sm.role
	sm.role = role
	if old != role {
		log.Info("agent role changed", "from", old, "to", role)
	}
}

// CanTransition 检查指定事件是否可以触发转换
func (sm *StateMachine) CanTransition(event TransitionEvent) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	transitions, ok := validTransitions[sm.current]
	if !ok {
		return false
	}
	_, ok = transitions[event]
	return ok
}

// Transition 执行状态转换
func (sm *StateMachine) Transition(event TransitionEvent, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	transitions, ok := validTransitions[sm.current]
	if !ok {
		return &InvalidTransitionError{
			From:  sm.current,
			Event: event,
			Cause: fmt.Sprintf("当前状态 %s 无可用转换", sm.current),
		}
	}

	nextState, ok := transitions[event]
	if !ok {
		return &InvalidTransitionError{
			From:  sm.current,
			Event: event,
			Cause: fmt.Sprintf("状态 %s 不支持事件 %s", sm.current, event),
		}
	}

	from := sm.current
	now := time.Now()
	sm.current = nextState
	sm.lastTransition = now

	record := TransitionRecord{
		From:      from,
		To:        nextState,
		Event:     event,
		Timestamp: now,
		Reason:    reason,
	}
	sm.history = append(sm.history, record)
	if len(sm.history) > sm.maxHistory {
		// 保留最近的一半
		keep := sm.maxHistory / 2
		sm.history = sm.history[len(sm.history)-keep:]
	}

	log.Debug("agent state transition",
		"from", string(from),
		"to", string(nextState),
		"event", string(event),
		"reason", reason,
	)

	// 触发回调
	for _, cb := range sm.callbacks {
		cb(from, nextState, event, reason)
	}

	return nil
}

// TransitionIfValid 仅在转换合法时执行，否则静默忽略
func (sm *StateMachine) TransitionIfValid(event TransitionEvent, reason string) bool {
	if err := sm.Transition(event, reason); err != nil {
		log.Debug("state transition skipped", "event", string(event), "error", err.Error())
		return false
	}
	return true
}

// OnTransition 注册状态变更回调
func (sm *StateMachine) OnTransition(cb StateChangeCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, cb)
}

// History 返回转换历史（副本）
func (sm *StateMachine) History() []TransitionRecord {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]TransitionRecord, len(sm.history))
	copy(result, sm.history)
	return result
}

// LastTransition 返回上次转换时间
func (sm *StateMachine) LastTransition() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastTransition
}

// StartedAt 返回创建时间
func (sm *StateMachine) StartedAt() time.Time {
	return sm.startedAt
}

// Reset 重置状态机到初始态（保留角色和历史）
func (sm *StateMachine) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = StateStart
	sm.lastTransition = time.Now()
}

// Snapshot 返回状态机快照（用于持久化或调试）
func (sm *StateMachine) Snapshot() StateMachineSnapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return StateMachineSnapshot{
		Current:  sm.current,
		Role:     sm.role,
		StartedAt: sm.startedAt,
		LastTransition: sm.lastTransition,
		HistoryLen: len(sm.history),
	}
}

// StateMachineSnapshot 状态机快照（只读）
type StateMachineSnapshot struct {
	Current        AgentState `json:"current"`
	Role           AgentRole  `json:"role"`
	StartedAt      time.Time  `json:"started_at"`
	LastTransition time.Time  `json:"last_transition"`
	HistoryLen     int        `json:"history_len"`
}

// InvalidTransitionError 非法状态转换错误
type InvalidTransitionError struct {
	From  AgentState
	Event TransitionEvent
	Cause string
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid transition: %s -[%s]-> ?: %s", e.From, e.Event, e.Cause)
}

// ── 角色字符串转枚举 ──

// ParseRole 解析角色字符串（大小写不敏感）
func ParseRole(s string) AgentRole {
	switch s {
	case "executor", "EXECUTOR", "Executor":
		return RoleExecutor
	case "supervisor", "SUPERVISOR", "Supervisor":
		return RoleSupervisor
	case "manager", "MANAGER", "Manager":
		return RoleManager
	default:
		return RoleExecutor
	}
}

// ParseState 解析状态字符串
func ParseState(s string) AgentState {
	switch s {
	case "start", "START":
		return StateStart
	case "reasoning", "REASONING":
		return StateReasoning
	case "executing", "EXECUTING":
		return StateExecuting
	case "waiting_lock", "WAITING_LOCK":
		return StateWaitingLock
	case "waiting_human", "WAITING_HUMAN":
		return StateWaitingHuman
	case "delegate", "DELEGATE":
		return StateDelegate
	case "suspended", "SUSPENDED":
		return StateSuspended
	case "timeout", "TIMEOUT":
		return StateTimeout
	case "retry", "RETRY":
		return StateRetry
	case "done", "DONE":
		return StateDone
	case "failed", "FAILED":
		return StateFailed
	case "cancel", "CANCEL":
		return StateCancel
	default:
		return StateStart
	}
}
