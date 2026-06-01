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
}

export interface SSEEvent {
  type: string
  content?: string
  tool?: string
  args?: string
  result?: string
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
}
