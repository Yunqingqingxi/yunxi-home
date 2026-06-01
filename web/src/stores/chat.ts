import { defineStore } from 'pinia'
import { ref, computed, ComputedRef } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import type { ChatMessage, ChatBlock, Conversation, SSEEvent, AgentInfo, ToolCall } from '../types/chat'

// ── Additional types ──────────────────────

interface Todo {
  id: string
  content: string
  status: string
}

interface ConfirmRequest {
  title?: string
  message?: string
  [key: string]: any
}

interface InteractiveRequest {
  type?: string
  [key: string]: any
}

interface SubAgentEntry {
  id: string
  goal: string
  status: string
  summary: string
}

// ── Regex ─────────────────────────────────

const fileMarkerRe: RegExp = /\[文件:\s*[^\]]+?\s*\([^)]+?\)\]/g

function stripFileMarkers(text: string): string {
  return text.replace(fileMarkerRe, '').replace(/\n{3,}/g, '\n\n')
}

// ── Render Markdown ───────────────────────

export function renderMarkdown(text: string): string {
  if (!text) return ''
  try {
    let raw = marked.parse(text, { gfm: true, breaks: true }) as string
    raw = raw.replace(/<a\s+href=/g, '<a target="_blank" rel="noopener" href=')
    return DOMPurify.sanitize(raw, {
      ALLOWED_TAGS: ['p','br','strong','em','del','a','ul','ol','li','h1','h2','h3','h4','h5','h6','pre','code','blockquote','hr','table','thead','tbody','tr','th','td','span','img','input'],
      ALLOWED_ATTR: ['href','target','class','src','alt','type','checked','disabled'],
    })
  } catch (e) {
    return DOMPurify.sanitize(text)
  }
}

// ── Store ─────────────────────────────────

export const useChatStore = defineStore('chat', () => {
  const messages = ref<ChatMessage[]>([])
  const sessionId = ref<string>('')
  const isStreaming = ref<boolean>(false)
  const loading = ref<boolean>(false)
  const hintTexts: string[] = [
    '查看系统状态和磁盘使用情况',
    '帮我管理文件，列出根目录',
    '查看 DNS 域名和更新记录',
    '搜索最近的日志文件',
    '检查 Docker 容器运行状态',
    '创建一个新的项目文件夹',
  ]

  const todoList = ref<Todo[]>([])
  const agents = ref<AgentInfo[]>([])
  const confirmRequest = ref<ConfirmRequest | null>(null)
  const interactiveRequest = ref<InteractiveRequest | null>(null)

  // Whether any agent (subtask) is running
  const hasRunningAgents = computed<boolean>(
    () => agents.value.some(a => a.agent_status === 'running' || a.status === 'running')
  )

  // Current turn's streaming assistant message indices
  const streamingPlaceholders = ref<number[]>([])

  let _msgVersion = 0
  let _contentFlushTimer: ReturnType<typeof setTimeout> | null = null

  function resetStreaming(): void {
    if (_contentFlushTimer) { clearTimeout(_contentFlushTimer); _contentFlushTimer = null }
    confirmRequest.value = null
    todoList.value = []
    agents.value = []
    streamingPlaceholders.value = []
    isStreaming.value = false
    loading.value = false
  }

  function addUserMessage(text: string): ChatMessage {
    const msg: ChatMessage = {
      id: 'u_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6),
      role: 'user',
      content: text,
      contentHtml: renderMarkdown(text),
      blocks: [{ type: 'content' as const, content: text }],
      status: 'done',
      streaming: false,
      _v: 0,
      createdAt: Date.now()
    }
    messages.value.push(msg)
    return msg
  }

  function addAssistantPlaceholder(): number {
    const msg: ChatMessage = {
      id: 'a_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6),
      role: 'assistant',
      content: '',
      contentHtml: '',
      reasoning: '',
      tools: [],
      blocks: [],
      status: 'streaming',
      streaming: true,
      _v: 0,
      createdAt: Date.now()
    }
    messages.value.push(msg)
    return messages.value.length - 1
  }

  async function sendMessage(
    text: string,
    model: string = '',
    opts: { reasoning_intensity?: string; plan_mode?: boolean } = {}
  ): Promise<void> {
    const token = localStorage.getItem('token')
    if (!text.trim()) return

    // If streaming or agent running, inject into current session
    if (isStreaming.value || hasRunningAgents.value) {
      addUserMessage(text)
      try {
        await fetch('/api/chat/inject', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
          body: JSON.stringify({ session_id: sessionId.value, message: text })
        })
      } catch (e) { /* ignore */ }
      if (hasRunningAgents.value) {
        const idx = addAssistantPlaceholder()
        streamingPlaceholders.value = [...streamingPlaceholders.value, idx]
      }
      return
    }

    loading.value = true
    if (!sessionId.value) {
      sessionId.value = 'chat_' + Date.now()
    }

    addUserMessage(text)
    const firstIdx = addAssistantPlaceholder()
    streamingPlaceholders.value = [firstIdx]
    isStreaming.value = true

    try {
      const body: Record<string, any> = { message: text, session_id: sessionId.value }
      if (model) body.model = model
      if (opts.reasoning_intensity) body.reasoning_intensity = opts.reasoning_intensity
      if (opts.plan_mode) body.plan_mode = true
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token
        },
        body: JSON.stringify(body)
      })
      if (!res.ok) throw new Error('HTTP ' + res.status)

      const reader = res.body!.getReader()
      const dec = new TextDecoder()
      let buf = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || ''
        for (const l of lines) {
          const trimmed = l.trim()
          if (!trimmed.startsWith('data: ')) continue
          try {
            processSSEEvent(JSON.parse(trimmed.slice(6)))
          } catch (e) { /* skip */ }
        }
      }
      if (buf.trim().startsWith('data: ')) {
        try { processSSEEvent(JSON.parse(buf.trim().slice(6))) } catch (e) {}
      }
    } catch (e: any) {
      const idx = streamingPlaceholders.value[0]
      const msg = messages.value[idx]
      if (msg) {
        msg.status = 'error'
        msg.content = e.message
        msg.contentHtml = '<p class="error-text">' + (e.message || '请求失败') + '</p>'
      }
    } finally {
      finalizeStream()
    }
  }

  function currentStreamingMsg(): ChatMessage | null {
    const list = streamingPlaceholders.value
    if (!list.length) return null
    return messages.value[list[list.length - 1]]
  }

  function processSSEEvent(ev: SSEEvent): void {
    const t = ev.type

    // todo_update, agent events don't need a streaming msg
    if (t === 'todo_update') {
      todoList.value = (ev.todos || []) as Todo[]
      return
    }
    if (t === 'agent_progress') {
      const idx = agents.value.findIndex(a => a.agent_id === ev.agent_id)
      if (idx >= 0) Object.assign(agents.value[idx], ev)
      else {
        agents.value.push({ ...ev } as AgentInfo)
        messages.value.push({
          id: 'ag_' + (ev.agent_id || Date.now()),
          role: 'agent' as const,
          agentId: ev.agent_id || '',
          agentGoal: ev.agent_goal || ev.goal || '',
          agentStatus: ev.agent_status || 'running',
          agentRound: ev.agent_round || 0,
          agentSummary: ev.content || '',
          createdAt: Date.now()
        } as ChatMessage)
      }
      updateSubAgent(sessionId.value, ev as AgentInfo)
      return
    }
    if (t === 'agent_result') {
      const idx = agents.value.findIndex(a => a.agent_id === ev.agent_id)
      const merged = { ...ev, status: ev.agent_status || ev.status || 'done' }
      if (idx >= 0) Object.assign(agents.value[idx], merged)
      const msgIdx = messages.value.findIndex(m => m.role === 'agent' && m.agentId === ev.agent_id)
      if (msgIdx >= 0) {
        messages.value[msgIdx] = {
          ...messages.value[msgIdx],
          agentStatus: merged.status,
          agentSummary: ev.content || ev.summary || messages.value[msgIdx].agentSummary,
          agentRound: ev.agent_round || messages.value[msgIdx].agentRound,
        } as ChatMessage
      } else {
        messages.value.push({
          id: 'ag_' + (ev.agent_id || Date.now()),
          role: 'agent' as const,
          agentId: ev.agent_id || '',
          agentGoal: ev.agent_goal || ev.goal || '',
          agentStatus: merged.status,
          agentRound: ev.agent_round || 0,
          agentSummary: ev.content || ev.summary || '',
          createdAt: Date.now()
        } as ChatMessage)
      }
      updateSubAgent(sessionId.value, merged as AgentInfo)
      return
    }
    if (t === 'confirm_required') {
      confirmRequest.value = ev.confirm_request || null
      return
    }
    if (t === 'interactive_request') {
      interactiveRequest.value = ev.interactive_request || null
      return
    }

    // tool_start / tool_progress
    if (t === 'tool_start' || t === 'tool_progress') {
      const pending = currentStreamingMsg()
      if (!pending) return
      const blocks = [...pending.blocks!]
      for (let i = 0; i < blocks.length; i++) {
        if (blocks[i].type === 'tool' && !blocks[i].result && blocks[i].status !== 'running') {
          blocks[i] = { ...blocks[i], status: 'running', progress: ev.content || '执行中...' }
          break
        }
      }
      pending.blocks = blocks
      pending._v = ++_msgVersion
      return
    }

    let msg = currentStreamingMsg()
    // Auto-create placeholder for reconnection events
    if (!msg && _listeningForEvents) {
      isStreaming.value = true
      const idx = addAssistantPlaceholder()
      streamingPlaceholders.value = [idx]
      msg = messages.value[idx]
    }
    if (!msg) return

    if (t === 'thinking' || t === 'content') {
      const raw = ev.content || ''
      const clean = t === 'content' ? stripFileMarkers(raw) : raw
      if (t === 'content' && !clean) return
      const blocks = [...msg.blocks!]
      const last = blocks.length > 0 ? blocks[blocks.length - 1] : null
      if (last && last.type === t) {
        blocks[blocks.length - 1] = { type: t, content: (last.content || '') + clean }
      } else {
        if (clean) blocks.push({ type: t, content: clean })
      }
      msg.blocks = blocks
      msg._v = ++_msgVersion
    } else if (t === 'tool_call') {
      msg.blocks = [...msg.blocks!, {
        type: 'tool' as const,
        name: ev.tool || 'unknown',
        args: ev.args || '',
        result: ''
      }]
      msg._v = ++_msgVersion
    } else if (t === 'tool_result') {
      const blocks = [...msg.blocks!]
      for (let i = 0; i < blocks.length; i++) {
        if (blocks[i].type === 'tool' && !blocks[i].result) {
          blocks[i] = { ...blocks[i], result: ev.content || '', status: '', progress: '' }
          break
        }
      }
      msg.blocks = blocks
      msg._v = ++_msgVersion

      const hasPendingTools = blocks.some(b => b.type === 'tool' && !b.result)
      if (!hasPendingTools && blocks.length > 0) {
        finalizeOne(msg)
        const newIdx = addAssistantPlaceholder()
        streamingPlaceholders.value = [...streamingPlaceholders.value, newIdx]
      }
    } else if (t === 'error') {
      msg.status = 'error'
      msg.content += '\n\n' + (ev.content || '')
    } else if (t === 'done') {
      const dur = parseInt(ev.content || '') || 0
      if (dur > 0) msg.durationMs = dur
    }
  }

  function finalizeOne(msg: ChatMessage): void {
    if (msg.status === 'error') {
      msg.contentHtml = '<p class="error-text">' + (msg.content || '请求失败') + '</p>'
      msg.streaming = false
      msg._v = ++_msgVersion
      return
    }
    const contentBlocks = msg.blocks!.filter((b: ChatBlock) => b.type === 'content')
    const thinkingBlocks = msg.blocks!.filter((b: ChatBlock) => b.type === 'thinking')
    const toolBlocks = msg.blocks!.filter((b: ChatBlock) => b.type === 'tool')
    msg.content = contentBlocks.map(b => b.content).filter(Boolean).join('\n')
    msg.contentHtml = renderMarkdown(msg.content)
    msg.reasoning = thinkingBlocks.map(b => b.content).filter(Boolean).join('\n')
    msg.tools = toolBlocks.map(b => ({ name: b.name || '', args: b.args || '', result: b.result || '' }))
    msg.status = 'done'
    msg.streaming = false
    msg._v = ++_msgVersion
  }

  function finalizeStream(): void {
    const last = currentStreamingMsg()
    if (last && last.streaming) {
      finalizeOne(last)
    }
    messages.value = messages.value.filter(m => {
      if (m.streaming && m.role === 'assistant') return false
      if (m.role === 'assistant' && m.status === 'done' && (!m.blocks || !m.blocks.length)) return false
      return true
    })
    resetStreaming()
  }

  function buildBlocksLegacy(
    role: string,
    content: string,
    reasoning: string,
    toolCalls: ToolCall[]
  ): ChatBlock[] {
    if (role === 'user') {
      return content ? [{ type: 'content' as const, content }] : []
    }
    const blocks: ChatBlock[] = []
    if (reasoning) blocks.push({ type: 'thinking' as const, content: reasoning })
    if (content) blocks.push({ type: 'content' as const, content })
    for (const tc of toolCalls) {
      blocks.push({ type: 'tool' as const, name: tc.name || '', args: tc.args || '', result: tc.result || '' })
    }
    return blocks
  }

  function normalizeBlock(b: any): ChatBlock {
    const type = (b.type === 'tool_call' || b.type === 'tool_result') ? 'tool' : b.type
    return {
      type,
      content: b.content || b.tool_result || '',
      name: b.name || b.tool_name || '',
      args: b.args || b.tool_args || '',
      result: b.result || b.tool_result || ''
    }
  }

  async function loadSession(sid: string, msgs: any[]): Promise<void> {
    sessionId.value = sid
    messages.value = (msgs || []).filter(m => m.role !== 'tool').map((m, i) => {
      const msg: ChatMessage = {
        id: 'h_' + i + '_' + Math.random().toString(36).slice(2, 6),
        role: m.role,
        content: m.content || '',
        contentHtml: renderMarkdown(m.content || ''),
        reasoning: m.reasoning_content || '',
        tools: (m.tool_calls || []).map((tc: any) => ({
          name: tc.name || '',
          args: tc.args || '',
          result: tc.result || ''
        })),
        status: 'done',
        streaming: false,
        _v: i,
        createdAt: m.created_at ? new Date(m.created_at).getTime() : Date.now()
      }

      if (m.blocks && m.blocks.length > 0) {
        msg.blocks = m.blocks.map(normalizeBlock)
      } else {
        msg.blocks = buildBlocksLegacy(m.role, m.content || '', m.reasoning_content || '', msg.tools || [])
      }

      return msg
    })
  }

  async function fetchSessionMessages(sid: string): Promise<boolean> {
    const token = localStorage.getItem('token')
    for (let attempt = 0; attempt < 3; attempt++) {
      try {
        const res = await fetch('/api/chat/sessions/' + sid, {
          headers: { 'Authorization': 'Bearer ' + token }
        })
        const data = await res.json()
        if (data.code === 200 && data.data && data.data.messages) {
          await loadSession(sid, data.data.messages)
          return true
        }
        if (data.code !== 404) break
      } catch (e) { /* ignore */ }
      if (attempt < 2) await new Promise(r => setTimeout(r, 500))
    }
    return false
  }

  function clearCurrent(): void {
    messages.value = []
    resetStreaming()
    sessionId.value = ''
  }

  let _streamAbort: AbortController | null = null
  let _listeningForEvents = false

  async function connectStream(sid: string): Promise<void> {
    const token = localStorage.getItem('token')
    const controller = new AbortController()
    _streamAbort = controller

    _listeningForEvents = true

    try {
      const res = await fetch('/api/chat/stream/' + sid, {
        headers: { 'Authorization': 'Bearer ' + token },
        signal: controller.signal,
      })
      if (!res.ok) { isStreaming.value = false; _listeningForEvents = false; return }

      const reader = res.body!.getReader()
      const dec = new TextDecoder()
      let buf = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || ''
        for (const l of lines) {
          const trimmed = l.trim()
          if (!trimmed.startsWith('data: ')) continue
          try { processSSEEvent(JSON.parse(trimmed.slice(6))) } catch (e) {}
        }
      }
    } catch (e: any) {
      if (e.name !== 'AbortError') {
        console.warn('[chat] stream disconnected:', e.message)
      }
    } finally {
      _listeningForEvents = false
      const idx = streamingPlaceholders.value[0]
      if (idx != null) {
        const m = messages.value[idx]
        if (m && m.streaming && (!m.blocks || !m.blocks.length) && !m.content) {
          messages.value.splice(idx, 1)
        }
      }
      isStreaming.value = false
      finalizeStream()
    }
  }

  function disconnectStream(): void {
    if (_streamAbort) { _streamAbort.abort(); _streamAbort = null }
  }

  // ── Multi-Conversation State ──
  const conversations = ref<Conversation[]>([])
  const activeConversationId = ref<string>('')
  const subAgents = ref<Record<string, SubAgentEntry[]>>({})

  async function loadConversations(): Promise<void> {
    const token = localStorage.getItem('token')
    try {
      const res = await fetch('/api/chat/sessions', {
        headers: { 'Authorization': 'Bearer ' + token }
      })
      const data = await res.json()
      if (data.code === 200 && data.data) {
        conversations.value = (data.data || []).map((s: any) => ({
          id: s.id,
          title: s.title || '新对话',
          createdAt: s.created_at,
          updatedAt: s.updated_at,
          messageCount: s.message_count || 0,
        })).sort((a: Conversation, b: Conversation) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
      }
    } catch (e) { /* ignore */ }
  }

  async function switchConversation(id: string): Promise<boolean> {
    if (activeConversationId.value === id) return true
    disconnectStream()
    activeConversationId.value = id
    sessionId.value = id
    messages.value = []
    agents.value = []
    resetStreaming()
    const ok = await fetchSessionMessages(id)
    if (!ok) {
      clearCurrent()
      sessionId.value = id
      activeConversationId.value = ''
      return false
    }
    connectStream(id)
    return ok
  }

  function updateSubAgent(convId: string, agent: AgentInfo): void {
    if (!subAgents.value[convId]) subAgents.value[convId] = []
    const list = subAgents.value[convId]
    const idx = list.findIndex(a => a.id === agent.agent_id)
    if (idx >= 0) {
      list[idx] = { ...list[idx], status: agent.agent_status || agent.status || '', summary: agent.content || list[idx].summary }
    } else {
      list.push({ id: agent.agent_id!, goal: agent.agent_goal || '', status: agent.agent_status || 'running', summary: '' })
    }
  }

  return {
    messages, sessionId, isStreaming, loading, hasRunningAgents,
    streamingPlaceholders, hintTexts, todoList, agents, confirmRequest, interactiveRequest,
    resetStreaming, sendMessage, loadSession,
    fetchSessionMessages, clearCurrent,
    addUserMessage, addAssistantPlaceholder, processSSEEvent, finalizeStream,
    connectStream, disconnectStream,
    conversations, activeConversationId, subAgents,
    loadConversations, switchConversation, updateSubAgent,
  }
})
