export interface TopologyCoord {
  x: number
  y: number
  z: number
}

export interface TopologyConstraint {
  a: number
  r: number
  t: boolean
  force_tools: string[]
}

export interface TopologyNode {
  x: number
  y: number
  z: number
  round: number
  tool_call: string
  status: string
  reason?: string
}

export interface TopologyUpdate {
  session_id: string
  coord: TopologyCoord
  trajectory: TopologyCoord[]
  constraint: TopologyConstraint
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

export interface TopologyState {
  session_id: string
  current_coord: TopologyCoord
  start_coord: TopologyCoord
  constraint: TopologyConstraint
  trajectory: TopologyNode[]
  reject_count: number
  trust_lies: number
  trust_locked: boolean
  closed_loop: boolean
  closed_distance?: number
  warning?: string
  active: boolean
}

export interface PromptSection {
  name: string
  value: string
  source: 'db' | 'builtin'
  has_default: boolean
}
