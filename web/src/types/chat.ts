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
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'agent'
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
  // Agent-specific
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
  | 'retry' | 'done' | 'failed' | 'cancel'

export type AgentRole = 'executor' | 'supervisor' | 'manager'

export interface StateTransition {
  from: AgentState
  to: AgentState
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

export interface SSEEvent {
  type: string
  content?: string
  tool?: string
  args?: string
  result?: string
  tool_name?: string
  tool_progress?: string
  // agent
  agent_id?: string
  agent_goal?: string
  agent_status?: string
  agent_round?: number
  goal?: string
  status?: string
  summary?: string
  todos?: Array<{ id: string; content: string; status: string }>
  confirm_request?: any
  interactive_request?: any
  topology_update?: {
    session_id: string
    coord: { x: number; y: number; z: number }
    trajectory: Array<{ x: number; y: number; z: number }>
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
  }
  // v2.0 events
  state_change?: {
    agent_id: string
    from: AgentState
    to: AgentState
    event: string
    reason?: string
  }
  role_change?: {
    agent_id: string
    old_role: AgentRole
    new_role: AgentRole
    reason: string
  }
  lock_conflict?: LockConflict
  meta_report?: MetaReport
}

export interface AgentInfo {
  agent_id: string
  agent_goal?: string
  agent_status?: string
  agent_round?: number
  content?: string
  goal?: string
  status?: string
  summary?: string
  // v2.0
  state?: AgentState
  role?: AgentRole
}
