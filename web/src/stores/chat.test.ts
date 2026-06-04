import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock DOMPurify before importing the store module
vi.mock('dompurify', () => ({
  default: {
    sanitize: (text: string) => text,
  },
}))

// Mock marked
vi.mock('marked', () => ({
  marked: {
    parse: (text: string) => `<p>${text}</p>`,
  },
}))

// Mock storage
const storage: Record<string, string> = {}
beforeEach(() => {
  setActivePinia(createPinia())
  for (const key of Object.keys(storage)) delete storage[key]
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => storage[k] || null,
    setItem: (k: string, v: string) => { storage[k] = v },
    removeItem: (k: string) => { delete storage[k] },
  })
  vi.stubGlobal('sessionStorage', {
    getItem: (k: string) => storage[k] || null,
    setItem: (k: string, v: string) => { storage[k] = v },
    removeItem: (k: string) => { delete storage[k] },
  })
  vi.stubGlobal('fetch', vi.fn())
  vi.stubGlobal('AbortController', vi.fn(() => ({ abort: vi.fn(), signal: {} })))
  vi.stubGlobal('EventSource', vi.fn())
})

import { useChatStore } from './chat'

// Helper: get internal store state for white-box testing
function sm(store: any) {
  return store
}

// ── Conversation + Message Tests ──

describe('ChatStore — Session Lifecycle', () => {
  it('starts with idle lifecycle', () => {
    const store = useChatStore()
    expect(store.sessionId).toBe('')
    expect(store.isStreaming).toBe(false)
  })

  it('addUserMessage creates a message with correct shape', () => {
    const store: any = useChatStore()
    store.sessionId = 'test-sid'
    const msg = store.addUserMessage('你好')
    expect(msg.role).toBe('user')
    expect(msg.content).toBe('你好')
    expect(msg.status).toBe('done')
    expect(msg.blocks).toHaveLength(1)
    expect(msg.blocks[0].type).toBe('content')
  })

  it('addAssistantPlaceholder creates streaming message', () => {
    const store: any = useChatStore()
    store.sessionId = 'test-sid'
    const idx = store.addAssistantPlaceholder()
    const msg = store.messages[idx]
    expect(msg.role).toBe('assistant')
    expect(msg.streaming).toBe(true)
    expect(msg.status).toBe('streaming')
    expect(msg.blocks).toEqual([])
  })

  it('lifecycle object is defined', () => {
    const store = useChatStore()
    expect(store.lifecycles).toBeDefined()
  })
})

// ── SSE Event Processing ──

describe('ChatStore — processSSEEvent', () => {
  it('processes thinking event into streaming message blocks', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    const idx = s.addAssistantPlaceholder()
    s.streamingPlaceholders = [idx]

    s.processSSEEvent({ type: 'thinking', content: '正在分析...' })

    const msg = s.messages[idx]
    expect(msg.blocks).toHaveLength(1)
    expect(msg.blocks[0].type).toBe('thinking')
    expect(msg.blocks[0].content).toBe('正在分析...')
  })

  it('processes content event appends to blocks', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    const idx = s.addAssistantPlaceholder()
    s.streamingPlaceholders = [idx]

    s.processSSEEvent({ type: 'content', content: '你好！' })
    s.processSSEEvent({ type: 'content', content: '有什么可以帮你的？' })

    const msg = s.messages[idx]
    expect(msg.blocks).toHaveLength(1)
    expect(msg.blocks[0].content).toBe('你好！有什么可以帮你的？')
  })

  it('processes tool_call and tool_result events', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    const idx = s.addAssistantPlaceholder()
    s.streamingPlaceholders = [idx]

    s.processSSEEvent({ type: 'tool_call', tool: 'file_read', args: '{"path":"/tmp"}' })
    s.processSSEEvent({ type: 'tool_result', tool: 'file_read', content: 'file1.txt\nfile2.log' })

    const msg = s.messages[idx]
    expect(msg.blocks).toHaveLength(1)
    expect(msg.blocks[0].type).toBe('tool')
    expect(msg.blocks[0].name).toBe('file_read')
    expect(msg.blocks[0].result).toBe('file1.txt\nfile2.log')
  })

  it('processes agent_progress event updates sessionAgents', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'

    s.processSSEEvent({
      type: 'agent_progress',
      agent_id: 'agent_1',
      agent_goal: '检查DNS',
      agent_status: 'running',
      agent_round: 2,
    })

    const agents = s.sessionAgents['test-sid']
    expect(agents).toHaveLength(1)
    expect(agents[0].agent_id).toBe('agent_1')
    expect(agents[0].agent_status).toBe('running')
  })

  it('processes agent_result event adds to messages', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'

    s.processSSEEvent({
      type: 'agent_result',
      agent_id: 'agent_1',
      agent_goal: '检查DNS',
      agent_status: 'done',
      content: 'DNS 配置正常',
    })

    const agents = s.sessionAgents['test-sid']
    expect(agents).toHaveLength(1)
    expect(agents[0].agent_status).toBe('done')

    // Should also create a message bubble
    const agentMsgs = s.messages.filter((m: any) => m.id && m.id.startsWith('agr_'))
    expect(agentMsgs.length).toBeGreaterThanOrEqual(1)
  })

  it('processes state_change event updates agent state', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    s.sessionAgents['test-sid'] = [{ agent_id: 'agent_1', agent_status: 'running' }]

    s.processSSEEvent({
      type: 'state_change',
      state_change: { agent_id: 'agent_1', from: 'reasoning', to: 'executing', event: 'plan_ready' },
    })

    expect(s.sessionAgents['test-sid'][0].state).toBe('executing')
  })

  it('processes role_change event updates agent role', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    s.sessionAgents['test-sid'] = [{ agent_id: 'agent_1', agent_status: 'running' }]

    s.processSSEEvent({
      type: 'role_change',
      role_change: { agent_id: 'agent_1', old_role: 'executor', new_role: 'supervisor', reason: 'test' },
    })

    expect(s.sessionAgents['test-sid'][0].role).toBe('supervisor')
  })

  it('processes lock_conflict event stores in lockConflicts', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'

    s.processSSEEvent({
      type: 'lock_conflict',
      lock_conflict: {
        resource_id: 'file:/etc/test.txt',
        agents: ['agent_1', 'agent_2'],
        decision: 'yield', winner: 'agent_1', reason: 'priority',
      },
    })

    expect(s.lockConflicts).toHaveLength(1)
    expect(s.lockConflicts[0].resource_id).toBe('file:/etc/test.txt')
  })

  it('processes interrupted event saves snapshot', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'

    s.processSSEEvent({
      type: 'interrupted',
      content: '进度 55%，最后执行：file_read',
    })

    expect(s.interruptSnapshot).toBeTruthy()
    expect(s.interruptSnapshot.progress).toBe(55)
    expect(s.interruptSnapshot.last_task).toBe('file_read')
  })

  it('finalizeStream finalizes streaming messages', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    const idx = s.addAssistantPlaceholder()
    s.streamingPlaceholders = [idx]

    s.processSSEEvent({ type: 'content', content: '完成！' })
    s.finalizeStream()

    const msg = s.messages[idx]
    expect(msg.status).toBe('done')
    expect(msg.streaming).toBe(false)
  })
})

// ── Computed Properties ──

describe('ChatStore — computed', () => {
  it('agents returns session agents for current session', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    s.sessionAgents['test-sid'] = [{ agent_id: 'a1' }, { agent_id: 'a2' }]

    expect(s.agents).toHaveLength(2)
  })

  it('hasRunningAgents detects running agents', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    s.sessionAgents['test-sid'] = [
      { agent_id: 'a1', agent_status: 'done' },
      { agent_id: 'a2', agent_status: 'running' },
    ]
    s.agentActiveSessions['test-sid'] = false

    expect(s.hasRunningAgents).toBe(true)
  })

  it('hasRunningAgents uses agentActiveSessions flag', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'
    s.sessionAgents['test-sid'] = []
    s.agentActiveSessions['test-sid'] = true

    expect(s.hasRunningAgents).toBe(true)
  })

  it('lockConflicts returns current session conflicts via SSE event', () => {
    const s: any = useChatStore()
    s.sessionId = 'test-sid'

    // Populate via SSE event (public API)
    s.processSSEEvent({
      type: 'lock_conflict',
      lock_conflict: { resource_id: 'f1', agents: ['a1'], decision: 'yield', reason: 'test' },
    })

    expect(s.lockConflicts).toHaveLength(1)
    expect(s.lockConflicts[0].resource_id).toBe('f1')
  })
})
