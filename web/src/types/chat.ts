export interface ChatBlock {
  type: 'content' | 'thinking' | 'tool' | 'tool_call' | 'tool_result'
  content?: string
  name?: string
  args?: string
  result?: string
  status?: string
  progress?: string
}

export interface ToolCall {
  name: string
  args: string
  result: string
  status?: string
  progress?: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'agent' | 'system'
  content: string
  contentHtml?: string
  reasoning?: string
  tools?: ToolCall[]
  blocks?: ChatBlock[]
  status?: 'streaming' | 'done' | 'error'
  streaming?: boolean
  _v?: number
  createdAt?: number
  durationMs?: number
  agentId?: string
  agentGoal?: string
  agentStatus?: string
  agentRound?: number
  agentSummary?: string
}

export interface Conversation {
  id: string
  title: string
  createdAt: string
  updatedAt: string
  messageCount: number
  pinned?: boolean
  isActive?: boolean
}

// ── Agent State & Role (v2.0 state machine) ──

export type AgentState = 'start' | 'reasoning' | 'executing' | 'waiting_lock'
  | 'waiting_human' | 'delegate' | 'suspended' | 'timeout'
  | 'retry' | 'done' | 'failed' | 'cancel' | 'running' | 'pending' | string

export type AgentRole = 'executor' | 'supervisor' | 'manager' | string

export interface StateTransition {
  from: string
  to: string
  event: string
  reason?: string
  ts: number
}

export interface LockConflict {
  resource_id: string
  agents: string[]
  decision: string
  winner?: string
  reason: string
}

export interface MetaReport {
  agent_id: string
  success_rate: number
  avg_latency_ms: number
  conflict_count: number
  task_completed: number
  task_failed: number
  current_load: number
  role?: AgentRole
  role_since?: string
  role_ttl?: string
}

// ── SSEEvent — complete set matching backend ──

export interface SSEEvent {
  type: string
  content?: string
  tool?: string
  args?: string
  result?: string
  tool_name?: string
  tool_progress?: string
  // Agent
  agent_id?: string
  agent_goal?: string
  agent_status?: string
  agent_round?: number
  goal?: string
  status?: string
  summary?: string
  // Todo
  todos?: Array<{ id: string; content: string; status: string }>
  // Confirm / Interactive
  confirm_request?: any
  interactive_request?: any
  // Plan
  plan_result?: { steps: any[]; total_steps: number; successes: number; failures: number; duration_ms: number }
  step_result?: { id: number; tool: string; status: string; result: any }
  // Goal
  goal_id?: string
  goal_title?: string
  goal_progress?: number
  // Cross-session
  cross_session?: { type: string; session_id: string; resource?: string; message: string }
  // Background tasks
  task_id?: string
  task_progress?: number
  task_status?: string
  task_message?: string
  // Skill
  skill_name?: string
  skill_current_step?: number
  skill_total_steps?: number
  skill_step_status?: string
  // Cron
  cron_task_id?: string
  cron_action?: string
  // Topology
  topology_update?: {
    session_id: string
    coord: { x: number; y: number; z: number }
    trajectory: Array<{ x: number; y: number; z: number; tool_call?: string; status?: string; reason?: string; tool_result?: string }>
    constraint: { a: number; r: number; t: boolean; force_tools: string[] }
    rejected: boolean
    reject_reason?: string
    reject_count: number
    trust_lies: number
    trust_locked: boolean
    closed_loop: boolean
    closed_distance?: number
    warning?: string
    oscillation: boolean
    override: boolean
    committed_count?: number
    total_nodes?: number
  }
  // v2.0 state machine
  state_change?: {
    agent_id: string
    from: string
    to: string
    event: string
    reason?: string
  }
  role_change?: {
    agent_id: string
    old_role: string
    new_role: string
    reason: string
  }
  lock_conflict?: LockConflict
  meta_report?: MetaReport
  // Usage
  usage?: { prompt_tokens: number; completion_tokens: number; total_tokens: number; cost: number }
  // Dedup
  _seq?: number
}

// ── AgentInfo — API + SSE merged ──

export interface AgentInfo {
  agent_id: string
  agent_goal?: string
  agent_status?: string
  agent_round?: number
  content?: string
  goal?: string
  status?: string
  summary?: string
  task?: string
  error?: string
  state?: AgentState
  role?: AgentRole
  _transitions?: StateTransition[]
}
