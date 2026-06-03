import { defineStore } from 'pinia'
import { ref, computed, ComputedRef } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import type { ChatMessage, ChatBlock, Conversation, SSEEvent, AgentInfo, ToolCall, AgentState, AgentRole, LockConflict } from '../types/chat'
import type { TopologyState, TopologyUpdate } from '../types/topology'

// ── Session Connection Lifecycle (v2.0) ──
type Lifecycle = 'idle' | 'connecting' | 'streaming' | 'reconnecting' | 'closed'
const STORAGE_KEY = 'yunxi_lifecycle'

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
      ALLOWED_TAGS: [
        'p',
        'br',
        'strong',
        'em',
        'del',
        'a',
        'ul',
        'ol',
        'li',
        'h1',
        'h2',
        'h3',
        'h4',
        'h5',
        'h6',
        'pre',
        'code',
        'blockquote',
        'hr',
        'table',
        'thead',
        'tbody',
        'tr',
        'th',
        'td',
        'span',
        'img',
        'input',
      ],
      ALLOWED_ATTR: ['href', 'target', 'class', 'src', 'alt', 'type', 'checked', 'disabled'],
    })
  } catch (e) {
    return DOMPurify.sanitize(text)
  }
}

// ── Store ─────────────────────────────────

export const useChatStore = defineStore('chat', () => {
  // Per-session message storage — isolates SSE writes per session, prevents cross-session corruption
  const _sessionMsgs: Record<string, ChatMessage[]> = {}
  const messages = ref<ChatMessage[]>([])
  const sessionId = ref<string>('')
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
  const confirmRequest = ref<ConfirmRequest | null>(null)
  const interactiveRequest = ref<InteractiveRequest | null>(null)
  const topology = ref<TopologyState | null>(null)

  // ── v2.0 Session connection lifecycle ──
  // Persisted across tab switches + page refreshes via sessionStorage.
  const lifecycles = ref<Record<string, Lifecycle>>(loadLifecycles())
  const sessionAgents = ref<Record<string, AgentInfo[]>>({})
  const agentActiveSessions = ref<Record<string, boolean>>({})
  // Legacy alias for backward compat
  const streamingSessions = ref<Record<string, boolean>>({})

  // Computed: current session is busy (streaming or sending)
  const isStreaming = computed<boolean>(() => {
    const sid = sessionId.value
    if (!sid) return false
    const lc = getLifecycle(sid)
    return lc === 'streaming' || lc === 'reconnecting' || lc === 'connecting'
  })

  const hasRunningAgents = computed<boolean>(() => {
    const sid = sessionId.value
    if (!sid) return false
    if (agentActiveSessions.value[sid]) return true
    const list = sessionAgents.value[sid] || []
    return list.some((a) => a.agent_status === 'running' || a.status === 'running')
  })

  // ── Lifecycle helpers ──
  function loadLifecycles(): Record<string, Lifecycle> {
    try { const raw = sessionStorage.getItem(STORAGE_KEY); return raw ? JSON.parse(raw) : {} } catch { return {} }
  }
  function saveLifecycles(): void {
    try { sessionStorage.setItem(STORAGE_KEY, JSON.stringify(lifecycles.value)) } catch {}
  }
  function setLifecycle(sid: string, lc: Lifecycle): void {
    if (lc === 'closed' || lc === 'idle') { delete lifecycles.value[sid]; delete streamingSessions.value[sid] }
    else { lifecycles.value[sid] = lc; streamingSessions.value[sid] = true }
    saveLifecycles()
  }
  function getLifecycle(sid: string): Lifecycle {
    return lifecycles.value[sid] || 'idle'
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', saveLifecycles)
  }

  // ── v2.0 Lock conflict & meta report storage ──
  const _lockConflicts: Record<string, LockConflict[]> = {}
  const _metaReports: Record<string, any> = {}
  const lockConflicts = computed<LockConflict[]>(() => {
    const sid = sessionId.value
    return sid ? (_lockConflicts[sid] || []) : []
  })
  const metaReport = computed<any>(() => {
    const sid = sessionId.value
    return sid ? (_metaReports[sid] || null) : null
  })

  // ── Debug injection (dev only) ──
  const agents = computed<AgentInfo[]>(() => {
    const sid = sessionId.value
    if (!sid) return []
    return sessionAgents.value[sid] || []
  })

  // Tool progress tracking
  const currentToolName = ref('')
  const currentToolProgress = ref('')

  // Interrupt state (for InterruptBanner)
  const interruptSnapshot = ref<{ progress: number; last_task: string } | null>(null)

  // Current turn's streaming assistant message indices
  const streamingPlaceholders = ref<number[]>([])

  let _msgVersion = 0
  let _contentFlushTimer: ReturnType<typeof setTimeout> | null = null

  function resetStreaming(): void {
    if (_contentFlushTimer) {
      clearTimeout(_contentFlushTimer)
      _contentFlushTimer = null
    }
    confirmRequest.value = null
    todoList.value = []
    streamingPlaceholders.value = []
    currentToolName.value = ''
    currentToolProgress.value = ''
    loading.value = false
    const sid = sessionId.value
    if (sid) {
      setLifecycle(sid, 'idle')
      sessionAgents.value[sid] = []
    }
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
      createdAt: Date.now(),
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
      createdAt: Date.now(),
    }
    messages.value.push(msg)
    return messages.value.length - 1
  }

  async function sendMessage(
    text: string,
    model: string = '',
    opts: { reasoning_intensity?: string; plan_mode?: boolean } = {},
  ): Promise<void> {
    const token = localStorage.getItem('token')
    if (!text.trim()) return

    const sendSid = sessionId.value || ('chat_' + Date.now())
    _sending[sendSid] = true

    // If streaming or agent running, inject into current session
    if (isStreaming.value || hasRunningAgents.value) {
      console.log('[chat] sendMessage: taking INJECT path')
      addUserMessage(text)
      try {
        await fetch('/api/chat/inject', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
          body: JSON.stringify({ session_id: sessionId.value, message: text }),
        })
      } catch (e) {
        /* ignore */
      }
      if (hasRunningAgents.value) {
        const idx = addAssistantPlaceholder()
        streamingPlaceholders.value = [...streamingPlaceholders.value, idx]
      }
      delete _sending[sendSid]
      return
    }

    loading.value = true
    if (!sessionId.value) {
      sessionId.value = sendSid
      activeConversationId.value = sessionId.value  // keep in sync
    }

    addUserMessage(text)
    const firstIdx = addAssistantPlaceholder()
    streamingPlaceholders.value = [firstIdx]
    setLifecycle(sessionId.value, 'streaming')

    try {
      const body: Record<string, any> = { message: text, session_id: sessionId.value }
      if (model) body.model = model
      if (opts.reasoning_intensity) body.reasoning_intensity = opts.reasoning_intensity
      if (opts.plan_mode) body.plan_mode = true
      const res = await fetch('/api/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + token,
        },
        body: JSON.stringify(body),
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
            const evt = JSON.parse(trimmed.slice(6))
            // Redirect writes to correct session's message store if user switched away
            if (sessionId.value !== sendSid && sendSid) {
              const origMsgs = messages.value
              const origPlaceholders = [...streamingPlaceholders.value]
              messages.value = _sessionMsgs[sendSid] || []
              streamingPlaceholders.value = []
              processSSEEvent(evt, sendSid) // ← pass correct session ID
              _sessionMsgs[sendSid] = [...messages.value]
              messages.value = origMsgs
              streamingPlaceholders.value = origPlaceholders
            } else {
              processSSEEvent(evt, sendSid)
            }
          } catch (e) { /* skip */ }
        }
      }
      if (buf.trim().startsWith('data: ')) {
        try {
          const evt = JSON.parse(buf.trim().slice(6))
          if (sessionId.value !== sendSid && sendSid) {
            const origMsgs = messages.value
            const origPlaceholders = [...streamingPlaceholders.value]
            messages.value = _sessionMsgs[sendSid] || []
            streamingPlaceholders.value = []
            processSSEEvent(evt, sendSid)
            _sessionMsgs[sendSid] = [...messages.value]
            messages.value = origMsgs
            streamingPlaceholders.value = origPlaceholders
          } else {
            processSSEEvent(evt, sendSid)
          }
        } catch (e) {}
      }
    } catch (e: any) {
      const msgs = (sendSid && sessionId.value !== sendSid) ? (_sessionMsgs[sendSid] || messages.value) : messages.value
      const idx = streamingPlaceholders.value[0]
      const msg = msgs[idx]
      if (msg) {
        msg.status = 'error'
        msg.content = e.message
        msg.contentHtml = '<p class="error-text">' + (e.message || '请求失败') + '</p>'
      }
    } finally {
      delete _sending[sendSid]
      if (sessionId.value === sendSid) {
        finalizeStream()
      } else {
        // Session changed — finalize the send session's messages in the per-session store
        console.log('[chat] sendMessage: finalizing for different session', sendSid)
        if (sendSid && _sessionMsgs[sendSid]) {
          const savedMsgs = messages.value
          const savedPlaceholders = [...streamingPlaceholders.value]
          messages.value = _sessionMsgs[sendSid]
          streamingPlaceholders.value = []
          finalizeStream()
          _sessionMsgs[sendSid] = [...messages.value]
          messages.value = savedMsgs
          streamingPlaceholders.value = savedPlaceholders
        }
      }
      // Reconnect stream after send completes
      if (sessionId.value) debouncedReconnect(sessionId.value)
    }
  }

  function currentStreamingMsg(): ChatMessage | null {
    const list = streamingPlaceholders.value
    if (!list.length) return null
    return messages.value[list[list.length - 1]]
  }

  // Helper: get mutable agents array for a specific session
  function currentAgents(sid?: string): AgentInfo[] {
    const target = sid || sessionId.value
    if (!target) return []
    if (!sessionAgents.value[target]) sessionAgents.value[target] = []
    return sessionAgents.value[target]
  }

  // Sync messages.value from per-session store after switching
  function _syncMessages(sid: string) {
    if (_sessionMsgs[sid]) {
      messages.value = _sessionMsgs[sid]
    } else {
      messages.value = []
    }
  }

  // ── Event dedup state ──
  const _lastSeq: Record<string, number> = {}

  function processSSEEvent(ev: SSEEvent, sid?: string): void {
    const targetSid = sid || sessionId.value
    const t = ev.type

    // v2.0: Dedup by _seq to prevent replay overlap
    const seq = (ev as any)._seq as number | undefined
    if (seq != null && targetSid) {
      if (_lastSeq[targetSid] && seq <= _lastSeq[targetSid]) return
      _lastSeq[targetSid] = seq
    }

    // Handle session_status for reconnection sync
    if (t === 'session_status') {
      try {
        const st = JSON.parse(ev.content || '{}')
        console.log('[chat] session_status event:', st)
        if (st.session_id) {
          streamingSessions.value[st.session_id] = !!st.streaming
          // Persist agent-active flag across page refreshes
          if (st.has_agents) {
            agentActiveSessions.value[st.session_id] = true
            // Restore agent details from reconnection event
            if (st.agents && Array.isArray(st.agents)) {
              sessionAgents.value[st.session_id] = st.agents.map((a: any) => ({
                agent_id: a.agent_id || a.id,
                agent_goal: a.goal,
                agent_status: a.status,
                agent_round: a.round,
                content: a.summary || a.progress || '',
              }))
            }
          } else {
            delete agentActiveSessions.value[st.session_id]
          }
        }
      } catch (_) {}
      return
    }

    // Handle interrupted event
    if (t === 'interrupted') {
      console.log('[chat] interrupted event:', ev.content)
      const content = ev.content || ''
      const pm = content.match(/进度\s*(\d+)%/)
      const tm = content.match(/最后执行：(.+)/)
      interruptSnapshot.value = {
        progress: pm ? parseInt(pm[1]) : 0,
        last_task: tm ? tm[1] : '',
      }
      setLifecycle(targetSid, 'idle')
      return
    }

    // ── v2.0 event handlers ──
    if (t === 'state_change' && ev.state_change) {
      const sc = ev.state_change
      const list = currentAgents(targetSid)
      const idx = list.findIndex(a => a.agent_id === sc.agent_id)
      if (idx >= 0) list[idx].state = sc.to
      return
    }
    if (t === 'role_change' && ev.role_change) {
      const rc = ev.role_change
      const list = currentAgents(targetSid)
      const idx = list.findIndex(a => a.agent_id === rc.agent_id)
      if (idx >= 0) list[idx].role = rc.new_role
      return
    }
    if (t === 'lock_conflict' && ev.lock_conflict) {
      // Store for LockConflictNotice component
      if (!_lockConflicts[targetSid]) _lockConflicts[targetSid] = []
      _lockConflicts[targetSid].push(ev.lock_conflict)
      return
    }
    if (t === 'meta_report' && ev.meta_report) {
      _metaReports[targetSid] = ev.meta_report
      return
    }

    // todo_update, agent events don't need a streaming msg
    if (t === 'todo_update') {
      todoList.value = (ev.todos || []) as Todo[]
      return
    }
    if (t === 'agent_progress') {
      const list = currentAgents(targetSid)
      const idx = list.findIndex((a) => a.agent_id === ev.agent_id)
      if (idx >= 0) Object.assign(list[idx], ev)
      else list.push({ ...ev } as AgentInfo)
      updateSubAgent(sessionId.value, ev as AgentInfo)
      return
    }
    if (t === 'agent_result') {
      const list = currentAgents(targetSid)
      const idx = list.findIndex((a) => a.agent_id === ev.agent_id)
      const merged = { ...ev, status: ev.agent_status || ev.status || 'done' }
      if (idx >= 0) Object.assign(list[idx], merged)
      else list.push(merged as AgentInfo)
      updateSubAgent(sessionId.value, merged as AgentInfo)
      // Only push a message bubble if NOT in active sendMessage streaming.
      // During streaming, pushing messages corrupts the ReAct loop flow.
      // AgentPanel already shows the agent status card.
      const lc = getLifecycle(targetSid)
      if (lc !== 'streaming') {
        const isOk = merged.status === 'done'
        const resultContent = ev.content || ev.summary || ''
        const isQQ = targetSid?.startsWith('qqbot_')
        const agentContent = isQQ
          ? `${isOk ? '✓' : '✗'} ${ev.agent_goal || ev.goal || ''}: ${resultContent.slice(0, 200)}`
          : isOk
            ? `子Agent 完成\n> ${ev.agent_goal || ev.goal || ''}\n\n${resultContent}`
            : `子Agent 失败\n> ${ev.agent_goal || ev.goal || ''}\n\n${resultContent}`
        messages.value.push({
          id: 'agr_' + (ev.agent_id || Date.now()),
          role: 'assistant' as const,
          content: agentContent,
          contentHtml: renderMarkdown(agentContent),
          blocks: [{ type: 'content' as const, content: agentContent }],
          status: 'done' as const,
          streaming: false,
          _v: ++_msgVersion,
          createdAt: Date.now(),
        } as ChatMessage)
      }
      return
    }
    if (t === 'topology_update') {
      const tu = ev.topology_update as TopologyUpdate | undefined
      if (tu) {
        topology.value = {
          session_id: tu.session_id,
          current_coord: tu.coord,
          start_coord: { x: 0, y: 0, z: 0 },
          constraint: tu.constraint,
          trajectory: (tu.trajectory || []).map((c, i) => ({
            x: c.x, y: c.y, z: c.z, round: i,
            tool_call: '', status: 'committed',
          })),
          reject_count: tu.reject_count,
          trust_lies: tu.trust_lies,
          trust_locked: tu.trust_locked,
          closed_loop: tu.closed_loop,
          closed_distance: tu.closed_distance,
          warning: tu.warning,
          active: true,
        }
      }
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
      // Track current tool for AgentStatusBar
      if (ev.tool) currentToolName.value = ev.tool
      currentToolProgress.value = ev.content || ''
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
    // (streaming state is managed by session_status event, not reconnection)
    if (!msg && _listeningForEvents) {
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
      msg.blocks = [
        ...msg.blocks!,
        {
          type: 'tool' as const,
          name: ev.tool || 'unknown',
          args: ev.args || '',
          result: '',
        },
      ]
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

      const hasPendingTools = blocks.some((b) => b.type === 'tool' && !b.result)
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
    msg.content = contentBlocks
      .map((b) => b.content)
      .filter(Boolean)
      .join('\n')
    msg.contentHtml = renderMarkdown(msg.content)
    msg.reasoning = thinkingBlocks
      .map((b) => b.content)
      .filter(Boolean)
      .join('\n')
    msg.tools = toolBlocks.map((b) => ({ name: b.name || '', args: b.args || '', result: b.result || '' }))
    msg.status = 'done'
    msg.streaming = false
    msg._v = ++_msgVersion
  }

  const _titleRequested = new Set<string>()

  async function requestTitle(sid: string) {
    if (_titleRequested.has(sid)) return
    _titleRequested.add(sid)
    // Find first user message
    const msgs = _sessionMsgs[sid] || messages.value
    const firstUser = msgs.find((m: ChatMessage) => m.role === 'user' && m.content)
    if (!firstUser) return
    try {
      const resp = await fetch('/api/chat/title', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify({ session_id: sid, message: firstUser.content }),
      })
      const data = await resp.json()
      const title = data?.data?.title || data?.title
      if (title && title !== '新对话') {
        const conv = conversations.value.find((c: any) => c.id === sid)
        if (conv) conv.title = title
      }
    } catch (_) { /* silent */ }
  }

  function finalizeStream(): void {
    const last = currentStreamingMsg()
    if (last && last.streaming) {
      finalizeOne(last)
    }
    messages.value = messages.value.filter((m) => {
      if (!m) return false // safety: filter null entries
      if (m.streaming && m.role === 'assistant') return false
      if (m.role === 'assistant' && m.status === 'done' && (!m.blocks || !m.blocks.length)) return false
      return true
    })
    resetStreaming()

    // Auto-generate title after first exchange
    const sid = sessionId.value
    if (sid) {
      const conv = conversations.value.find((c: any) => c.id === sid)
      if (!conv || conv.title === '新对话') {
        requestTitle(sid)
      }
    }
  }

  function buildBlocksLegacy(role: string, content: string, reasoning: string, toolCalls: ToolCall[]): ChatBlock[] {
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
    const type = b.type === 'tool_call' || b.type === 'tool_result' ? 'tool' : b.type
    return {
      type,
      content: b.content || b.tool_result || '',
      name: b.name || b.tool_name || '',
      args: b.args || b.tool_args || '',
      result: b.result || b.tool_result || '',
    }
  }

  // mergeToolBlocks merges adjacent tool_call+tool_result pairs into a single tool block.
  function mergeToolBlocks(blocks: ChatBlock[]): ChatBlock[] {
    const merged: ChatBlock[] = []
    for (let i = 0; i < blocks.length; i++) {
      const b = blocks[i]
      if (b.type === 'tool' && !b.result && b.name && i + 1 < blocks.length) {
        const next = blocks[i + 1]
        // Next block is a tool_result for the same tool (no name, no args, has result)
        if (next.type === 'tool' && !next.name && !next.args && next.result) {
          merged.push({ ...b, result: next.result })
          i++ // skip the tool_result block
          continue
        }
      }
      merged.push(b)
    }
    return merged
  }

  async function loadSession(sid: string, msgs: any[]): Promise<void> {
    sessionId.value = sid
    messages.value = (msgs || [])
      .filter((m) => m.role !== 'tool')
      .map((m, i) => {
        // Synthesize content for agent messages from their structured fields
        let content = m.content || ''
        let role = m.role
        if (role === 'agent' && !content) {
          const status = m.agent_status || m.status || ''
          const icon = status === 'done' ? '✅' : status === 'error' ? '❌' : '🔄'
          content = `${icon} 子Agent: ${m.agent_goal || m.goal || ''}\n${m.agent_summary || m.summary || ''}`
          role = 'assistant' // render as normal bubble
        }
        const msg: ChatMessage = {
          id: 'h_' + i + '_' + Math.random().toString(36).slice(2, 6),
          role,
          content,
          contentHtml: renderMarkdown(content),
          reasoning: m.reasoning_content || '',
          tools: (m.tool_calls || []).map((tc: any) => ({
            name: tc.name || '',
            args: tc.args || '',
            result: tc.result || '',
          })),
          status: 'done',
          streaming: false,
          _v: i,
          createdAt: m.created_at ? new Date(m.created_at).getTime() : Date.now(),
        }

        if (m.blocks && m.blocks.length > 0) {
          msg.blocks = mergeToolBlocks(m.blocks.map(normalizeBlock))
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
          headers: { Authorization: 'Bearer ' + token },
        })
        const data = await res.json()
        if (data.code === 200 && data.data && data.data.messages) {
          await loadSession(sid, data.data.messages)
          // 同步获取拓扑状态
          fetchTopology(sid)
          return true
        }
        if (data.code !== 404) break
      } catch (e) {
        /* ignore */
      }
      if (attempt < 2) await new Promise((r) => setTimeout(r, 500))
    }
    return false
  }

  async function fetchTopology(sid: string): Promise<void> {
    const token = localStorage.getItem('token')
    try {
      const res = await fetch('/api/chat/sessions/' + sid + '/topology', {
        headers: { Authorization: 'Bearer ' + token },
      })
      const data = await res.json()
      if (data.code === 200 && data.data) {
        topology.value = data.data as TopologyState
      }
    } catch (e) {
      /* ignore */
    }
  }

  function clearCurrent(): void {
    const sid = sessionId.value
    if (sid && messages.value.length > 0) {
      _sessionMsgs[sid] = [...messages.value]
    }
    // Bug 2 fix: clean per-session send flag
    if (sid) {
      delete _sending[sid]
      delete agentActiveSessions.value[sid]
      setLifecycle(sid, 'closed')
    }
    messages.value = []
    resetStreaming()
    sessionId.value = ''
    activeConversationId.value = ''
  }

  let _streamAbort: AbortController | null = null
  let _listeningForEvents = false
  let _streamPromise: Promise<void> | null = null
  // Bug 2 fix: per-session send flag — prevents cross-session stickiness
  const _sending: Record<string, boolean> = {}
  let _switchToken = 0
  // Bug 1 fix: debounced reconnect timer
  let _reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let _pendingReconnectSid: string | null = null

  async function connectStream(sid: string): Promise<void> {
    console.log('[chat] connectStream: start', sid)
    const token = localStorage.getItem('token')
    const controller = new AbortController()
    _streamAbort = controller
    _listeningForEvents = true

    // Track promise so disconnectStream can await full cleanup.
    _streamPromise = (async () => {
      try {
        const res = await fetch('/api/chat/stream/' + sid, {
          headers: { Authorization: 'Bearer ' + token },
          signal: controller.signal,
        })
        if (!res.ok) {
          streamingSessions.value[sessionId.value] = false
          return
        }

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
              processSSEEvent(JSON.parse(trimmed.slice(6)), sid)
            } catch (e) {}
          }
        }
      } catch (e: any) {
        if (e.name !== 'AbortError') {
          console.warn('[chat] stream disconnected:', e.message)
        }
      } finally {
        _listeningForEvents = false
        _streamAbort = null
        _streamPromise = null
        // v2.0: Only clean up if session is truly closed, not just reconnecting
        const lc = getLifecycle(sid)
        if (lc === 'closed') {
          if (sessionId.value === sid) {
            streamingPlaceholders.value = []
          }
          return
        }
        // Don't prematurely clear streaming — let switch/send manage the lifecycle
        if (_sending[sid]) return
        if (sessionId.value === sid && lc !== 'streaming') {
          const idx = streamingPlaceholders.value[0]
          if (idx != null) {
            const m = messages.value[idx]
            if (m && m.streaming && (!m.blocks || !m.blocks.length) && !m.content) {
              messages.value.splice(idx, 1)
            }
          }
          streamingPlaceholders.value = []
        }
      }
    })()

    return _streamPromise
  }

  async function disconnectStream(): Promise<void> {
    // Bug 3 fix: synchronously clear streaming states for current session
    const sid = sessionId.value
    if (sid) {
      delete streamingSessions.value[sid]
    }
    // Cancel any pending reconnect
    if (_reconnectTimer) {
      clearTimeout(_reconnectTimer)
      _reconnectTimer = null
      _pendingReconnectSid = null
    }
    console.log('[chat] disconnectStream: abort:', !!_streamAbort, '| hasPromise:', !!_streamPromise)
    // Bug 1 fix: idempotent abort — don't re-abort an already-aborted controller
    if (_streamAbort) {
      try { _streamAbort.abort() } catch (_) { /* already aborted */ }
      _streamAbort = null
    }
    _listeningForEvents = false
    // Wait for the old stream's finally block to finish
    if (_streamPromise) {
      try { await _streamPromise } catch (_) {
        console.log('[chat] disconnectStream: promise rejected:', _)
      }
    }
    console.log('[chat] disconnectStream: done')
  }

  // Bug 1 fix: debounced reconnect — prevents SSE thrashing on rapid switches
  function debouncedReconnect(targetSid: string): void {
    if (_reconnectTimer) {
      clearTimeout(_reconnectTimer)
      _reconnectTimer = null
    }
    _pendingReconnectSid = targetSid
    _reconnectTimer = setTimeout(() => {
      _reconnectTimer = null
      if (_pendingReconnectSid === targetSid && sessionId.value === targetSid) {
        _pendingReconnectSid = null
        connectStream(targetSid)
      }
    }, 150)
  }

  // Bug 3 fix: clean up ALL streaming sessions (used on route leave / page unload)
  function cleanupAllStreams(): void {
    console.log('[chat] cleanupAllStreams: keys:', Object.keys(streamingSessions.value))
    // Abort current stream reader
    if (_streamAbort) {
      try { _streamAbort.abort() } catch (_) { /* already aborted */ }
      _streamAbort = null
    }
    _listeningForEvents = false
    if (_reconnectTimer) {
      clearTimeout(_reconnectTimer)
      _reconnectTimer = null
      _pendingReconnectSid = null
    }
    // Clear all streaming session flags
    const keys = Object.keys(streamingSessions.value)
    for (const k of keys) {
      delete streamingSessions.value[k]
      delete agentActiveSessions.value[k]
    }
    console.log('[chat] cleanupAllStreams: done, cleared', keys.length, 'sessions')
  }

  // Bug 2 fix: force-clear all per-session send flags (used on unmount)
  function forceClearSending(): void {
    const keys = Object.keys(_sending)
    for (const k of keys) {
      delete _sending[k]
    }
    if (keys.length > 0) {
      console.log('[chat] forceClearSending: cleared', keys.length, 'flags')
    }
  }

  // ── Conversation management actions ──

  async function renameConversation(id: string, title: string): Promise<boolean> {
    const token = localStorage.getItem('token')
    try {
      const res = await fetch('/api/chat/sessions/' + id, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify({ title }),
      })
      const data = await res.json()
      if (data.code === 200) {
        // Optimistically update local state
        const conv = conversations.value.find(c => c.id === id)
        if (conv) conv.title = title
        return true
      }
    } catch (e) { /* ignore */ }
    return false
  }

  async function deleteConversation(id: string): Promise<boolean> {
    const token = localStorage.getItem('token')
    try {
      const res = await fetch('/api/chat/sessions/' + id, {
        method: 'DELETE',
        headers: { Authorization: 'Bearer ' + token },
      })
      const data = await res.json()
      if (data.code === 200) {
        // Remove from list
        conversations.value = conversations.value.filter(c => c.id !== id)
        // Clean up per-session cache
        delete _sessionMsgs[id]
        delete _sending[id]
        delete streamingSessions.value[id]
        delete sessionAgents.value[id]
        delete agentActiveSessions.value[id]
        // If currently viewing this session, jump to home
        if (sessionId.value === id) {
          clearCurrent()
        }
        return true
      }
    } catch (e) { /* ignore */ }
    return false
  }

  async function togglePin(id: string, pinned: boolean): Promise<boolean> {
    const token = localStorage.getItem('token')
    // Optimistic update
    const conv = conversations.value.find(c => c.id === id)
    if (conv) (conv as any).pinned = pinned
    try {
      const res = await fetch('/api/chat/sessions/' + id, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify({ pinned }),
      })
      const data = await res.json()
      if (data.code === 200) {
        // Reload to get correct sort order
        await loadConversations()
        return true
      }
    } catch (e) { /* ignore */ }
    // Rollback on failure
    if (conv) (conv as any).pinned = !pinned
    return false
  }

  // ── Multi-Conversation State ──
  const conversations = ref<Conversation[]>([])
  const activeConversationId = ref<string>('')
  const subAgents = ref<Record<string, SubAgentEntry[]>>({})

  async function loadConversations(): Promise<void> {
    const token = localStorage.getItem('token')
    try {
      const res = await fetch('/api/chat/sessions', {
        headers: { Authorization: 'Bearer ' + token },
      })
      const data = await res.json()
      if (data.code === 200 && data.data) {
        // Backend returns sorted by pinned DESC, updated_at DESC — preserve that order
        // Client-side sort as safety net: pinned first, then by updated_at descending
        conversations.value = (data.data || [])
          .map((s: any) => ({
            id: s.id,
            title: s.title || '新对话',
            createdAt: s.created_at,
            updatedAt: s.updated_at,
            messageCount: s.message_count || 0,
            pinned: s.pinned || false,
            isActive: s.is_active || false,
          }))
          .sort((a: any, b: any) => {
            if (a.pinned !== b.pinned) return a.pinned ? -1 : 1
            return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
          })
      }
    } catch (e) {
      /* ignore */
    }
  }

  async function switchConversation(id: string): Promise<boolean> {
    const token = ++_switchToken

    if (activeConversationId.value === id) return true

    // Save current messages to per-session store before leaving
    const oldSid = sessionId.value
    if (oldSid) {
      _sessionMsgs[oldSid] = [...messages.value]
      delete _sending[oldSid]
    }

    // Stop listening to the OLD session's SSE, but don't close it — it keeps running
    await disconnectStream()

    if (token !== _switchToken) return false

    // Switch to new session
    activeConversationId.value = id
    sessionId.value = id

    // Preserve target session's lifecycle — don't overwrite with 'idle'
    const targetLc = getLifecycle(id)
    const wasStreaming = targetLc === 'streaming' || targetLc === 'reconnecting' || targetLc === 'connecting'

    // Only clear these if the target is NOT streaming (otherwise keep them)
    if (!wasStreaming) {
      interruptSnapshot.value = null
      confirmRequest.value = null
      todoList.value = []
      streamingPlaceholders.value = []
      currentToolName.value = ''
      currentToolProgress.value = ''
      loading.value = false
    }

    // ALWAYS restore from per-session cache first (it has the latest messages from SSE redirects)
    const cached = _sessionMsgs[id]
    if (cached && cached.length > 0) {
      messages.value = [...cached]
      if (token === _switchToken) debouncedReconnect(id)
      return true
    }

    // No cache — load from DB
    messages.value = []
    const ok = await fetchSessionMessages(id)
    if (token !== _switchToken) return false
    if (!ok) {
      clearCurrent()
      sessionId.value = id
      activeConversationId.value = ''
      return false
    }
    _sessionMsgs[id] = [...messages.value]
    if (token === _switchToken) debouncedReconnect(id)
    return ok
  }

  function updateSubAgent(convId: string, agent: AgentInfo): void {
    if (!subAgents.value[convId]) subAgents.value[convId] = []
    const list = subAgents.value[convId]
    const idx = list.findIndex((a) => a.id === agent.agent_id)
    if (idx >= 0) {
      list[idx] = {
        ...list[idx],
        status: agent.agent_status || agent.status || '',
        summary: agent.content || list[idx].summary,
      }
    } else {
      list.push({
        id: agent.agent_id!,
        goal: agent.agent_goal || '',
        status: agent.agent_status || 'running',
        summary: '',
      })
    }
  }

  // ── v2.0 debug injection (dev only) ──
  if (typeof window !== 'undefined' && import.meta.env.DEV) {
    (window as any).__debug = {
      injectSSE: (e: any) => processSSEEvent(e),
      get store() { return useChatStore() },
      simulateConflict: () => processSSEEvent({ type: 'lock_conflict', lock_conflict: { resource_id: 'file:/etc/test.txt', agents: ['agent_1','agent_2'], decision: 'yield', winner: 'agent_1', reason: 'priority' } }),
      simulatePromotion: () => processSSEEvent({ type: 'role_change', role_change: { agent_id: 'agent_1', old_role: 'executor', new_role: 'supervisor', reason: 'test' } }),
      simulateStateChange: (to: any) => processSSEEvent({ type: 'state_change', state_change: { agent_id: 'agent_1', from: 'reasoning' as any, to, event: 'debug' } }),
      runScenario(name: string) {
        const s: any = useChatStore()
        const sid = s.sessionId || 'debug_' + Date.now()
        if (!s.sessionId) { s.sessionId = sid; s.activeConversationId = sid }
        switch (name) {
          case 'greeting': { const m = s.addUserMessage('你好'); s.messages.push({ id: 'a_debug', role: 'assistant', content: '你好！有什么可以帮你的？', blocks: [{ type: 'content', content: '你好！有什么可以帮你的？' }], status: 'done', streaming: false }); break }
          case 'short': { const m = s.addUserMessage('列出 /tmp 目录'); s.sessionAgents[sid] = [{ agent_id: 'agent_1', agent_goal: '列出文件', agent_status: 'running', agent_round: 1, state: 'executing' }]; s.messages.push({ id: 'a_short', role: 'assistant', content: '', blocks: [{ type: 'tool', name: 'file_list', args: '{}', result: 'file1.txt\nfile2.log' }], status: 'done', streaming: false }); break }
          case 'long': { s.addUserMessage('同时检查DNS和Docker'); s.sessionAgents[sid] = [{ agent_id: 'agent_1', agent_goal: '检查DNS', agent_status: 'running', agent_round: 3, state: 'executing', role: 'supervisor' }, { agent_id: 'agent_2', agent_goal: '检查Docker', agent_status: 'running', agent_round: 2, state: 'executing', role: 'executor' }, { agent_id: 'agent_3', agent_goal: '读配置', agent_status: 'running', agent_round: 1, state: 'waiting_lock', role: 'executor' }]; s.messages.push({ id: 'a_long', role: 'assistant', content: '正在并行检查...', blocks: [{ type: 'content', content: '正在并行检查系统状态...' }], status: 'streaming', streaming: true }); break }
          case 'compound': { s.addUserMessage('全面巡检服务器'); s.sessionAgents[sid] = [{ agent_id: 'agent_1', agent_goal: '备份数据库', agent_status: 'running', agent_round: 5, state: 'executing', role: 'supervisor' }, { agent_id: 'agent_2', agent_goal: '检查磁盘', agent_status: 'running', agent_round: 3, state: 'executing', role: 'executor' }, { agent_id: 'agent_3', agent_goal: '查日志', agent_status: 'running', agent_round: 2, state: 'waiting_lock', role: 'executor' }]; s._lockConflicts[sid] = [{ resource_id: 'file:/etc/hosts', agents: ['agent_1','agent_3'], decision: 'yield', winner: 'agent_1', reason: 'priority preempt' }]; s._metaReports[sid] = { agent_id: 'agent_1', success_rate: 0.95, avg_latency_ms: 230, conflict_count: 1, task_completed: 10, task_failed: 1, current_load: 0.3, role: 'supervisor' }; break }
          case 'switch': { s.addUserMessage('长任务执行中'); s.messages.push({ id: 'a_switch', role: 'assistant', content: '', blocks: [{ type: 'tool', name: 'long_task', args: '{}', result: '', status: 'running' }], status: 'streaming', streaming: true }); (s as any).setLifecycle(sid, 'streaming'); break }
        }
      },
    }
  }

  return {
    messages,
    sessionId,
    isStreaming,
    loading,
    hasRunningAgents,
    streamingPlaceholders,
    hintTexts,
    todoList,
    agents,
    confirmRequest,
    interactiveRequest,
    topology,
    currentToolName,
    currentToolProgress,
    streamingSessions,
    sessionAgents,
    agentActiveSessions,
    interruptSnapshot,
    // v2.0
    lifecycles,
    lockConflicts,
    metaReport,
    resetStreaming,
    sendMessage,
    loadSession,
    fetchSessionMessages,
    clearCurrent,
    addUserMessage,
    addAssistantPlaceholder,
    processSSEEvent,
    finalizeStream,
    connectStream,
    disconnectStream,
    cleanupAllStreams,
    forceClearSending,
    conversations,
    activeConversationId,
    subAgents,
    loadConversations,
    switchConversation,
    renameConversation,
    deleteConversation,
    togglePin,
    updateSubAgent,
  }
})
