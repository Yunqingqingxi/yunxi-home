import { defineStore } from 'pinia'
import { ref, computed, ComputedRef, watch } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { fetchEventSource } from '@microsoft/fetch-event-source'
import type { ChatMessage, Conversation, SSEEvent, AgentInfo, ToolCall, AgentState, AgentRole, LockConflict, StateTransition } from '../types/chat'
import type { TopologyState, TopologyUpdate } from '../types/topology'

// ── Debug logger ──
const LOG_ENABLED = true
function dbg(tag: string, ...args: any[]) {
  if (!LOG_ENABLED) return
  const ts = new Date().toISOString().slice(11, 23)
  console.log(`%c[${ts}][${tag}]`, 'color:#888', ...args)
}

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

// Convert bare file references (filename.ext or /path/to/file.ext) in agent results to clickable links
const knownExt = '(?:html?|json|txt|log|yaml|yml|md|csv|xml|pdf|png|jpg|jpeg|gif|svg|zip|tar\\.gz|sh|py|go|js|ts|css|java|class|jar|war|vue|svelte|rs|rb|php|c|cpp|h|hpp|sql|proto|toml|ini|cfg|conf|env|bat|ps1|dockerfile|makefile)'
const bareFileRe = new RegExp(`\\b([\\w\\-+.]+?)\\.${knownExt}\\b`, 'gi')
const pathFileRe = new RegExp(`(/[\\w\\-+./]+?)\\.${knownExt}\\b`, 'gi')

function convertFileRefs(text: string): string {
  if (!text) return text
  // Don't double-convert
  if (fileMarkerRe.test(text)) return text
  // Convert paths like /DeepSeekExample.java or /data/.../file.py
  text = text.replace(pathFileRe, (_match, path) => {
    const name = path.split('/').pop() || path
    return `[文件: ${name} (${path})]`
  })
  // Convert bare filenames like DeepSeekExample.java
  text = text.replace(bareFileRe, (_match, name) => {
    return `[文件: ${name} (/sandbox/${name})]`
  })
  return text
}

// ── Render Markdown ───────────────────────

export function renderMarkdown(text: string): string {
  if (!text) return ''
  try {
    let raw = marked.parse(text, { gfm: true, breaks: true }) as string
    raw = raw.replace(/<a\s+href=/g, '<a target="_blank" rel="noopener" href=')
    raw = DOMPurify.sanitize(raw, {
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
    // Post-process: wrap code blocks with copy button (after DOMPurify so div/button survive)
    raw = raw.replace(
      /<pre><code( class="language-(\w+)")?>/g,
      (_m: string, _cls: string, lang: string) =>
        `<div class="md-code-block"><div class="md-code-hdr"><span class="md-code-lang">${lang || 'code'}</span><button class="md-code-copy" data-copy>复制</button></div><pre><code${_cls || ''}>`,
    )
    raw = raw.replace(/<\/code><\/pre>/g, '</code></pre></div>')
    return raw
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
    try { const raw = localStorage.getItem(STORAGE_KEY); return raw ? JSON.parse(raw) : {} } catch { return {} }
  }
  function saveLifecycles(): void {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(lifecycles.value)) } catch {}
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

  // 从当前消息中恢复工具进度（刷新/切换后重连用）
  function restoreToolProgress(): void {
    const msgs = messages.value
    if (!msgs.length) return
    for (let i = msgs.length - 1; i >= 0; i--) {
      const m = msgs[i]
      if (m.role !== 'assistant' && m.role !== 'agent') continue
      if (!m.tools?.length) continue
      const running = m.tools.find((t: any) => !t.result)
      if (running) {
        currentToolName.value = running.name || ''
        currentToolProgress.value = running.progress || ''
        return
      }
    }
  }

  // Interrupt state (for InterruptBanner)
  // 按会话隔离的 interrupt/topology 状态
  const _interruptSnapshots: Record<string, any> = {}
  const _topologies: Record<string, any> = {}

  const interruptSnapshot = ref<any>(null)
  const topology = ref<TopologyState | null>(null)

  // 切换会话时自动保存/恢复状态
  watch(sessionId, (newSid, oldSid) => {
    if (oldSid) {
      _interruptSnapshots[oldSid] = interruptSnapshot.value
      _topologies[oldSid] = topology.value
    }
    interruptSnapshot.value = _interruptSnapshots[newSid] || null
    topology.value = _topologies[newSid] || null
  })

  // Current turn's streaming assistant message indices
  const streamingPlaceholders = ref<number[]>([])

  let _msgVersion = 0

  function resetStreaming(): void {
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
      status: 'done',
      createdAt: Date.now(),
    }
    dbg('addUserMsg', msg.id, text.slice(0, 40))
    console.trace('[addUserMsg] call stack')
    messages.value.push(msg)
    return msg
  }

  function addAssistantPlaceholder(): number {
    const msg: ChatMessage = {
      id: 'a_' + Date.now() + '_' + Math.random().toString(36).slice(2, 6),
      role: 'assistant',
      content: '',
      reasoning: '',
      tools: [],
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
    if (_sending[sendSid]) { dbg('sendMsg', 'skip-already-sending', sendSid.slice(-8)); return }
    dbg('sendMsg', 'start', text.slice(0, 40), 'sid=' + sendSid.slice(-8))
    _sending[sendSid] = true

    if (!sessionId.value) {
      sessionId.value = sendSid
      activeConversationId.value = sessionId.value
    }

    loading.value = true
    addUserMessage(text)
    const idx = addAssistantPlaceholder()
    streamingPlaceholders.value = [...streamingPlaceholders.value, idx]
    setLifecycle(sessionId.value, 'streaming')

    try {
      // Ensure SSE is connected so response events can be received
      connectStream(sessionId.value)
      // Always inject via the persistent SSE stream — no second SSE connection.
      const body: Record<string, any> = { message: text, session_id: sessionId.value }
      if (model) body.model = model
      if (opts.reasoning_intensity) body.reasoning_intensity = opts.reasoning_intensity
      if (opts.plan_mode) body.plan_mode = true
      await fetch('/api/chat/inject', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify(body),
      })
    } catch (e) {
      const msgs = messages.value
      const msg = msgs[idx]
      if (msg) {
        msg.status = 'error'
        msg.content = String(e)
        msg.contentHtml = '<p class="error-text">' + (String(e) || '请求失败') + '</p>'
      }
    } finally {
      delete _sending[sendSid]
      loading.value = false
      setTimeout(() => loadConversations(), 500)
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
    // Log key events (skip high-frequency thinking/content to reduce noise)
    if (t !== 'thinking' && t !== 'content' && t !== 'tool_progress') {
      dbg('SSE', t, 'sid=' + (targetSid || '').slice(-8), 'seq=' + ((ev as any)._seq ?? '?'))
    }

    // v2.0: Dedup by _seq to prevent replay overlap
    const seq = (ev as any)._seq as number | undefined
    if (seq != null && targetSid) {
      if (_lastSeq[targetSid] && seq <= _lastSeq[targetSid]) return
      _lastSeq[targetSid] = seq
    }

    if (t === 'session_created') { try { const sc = JSON.parse(ev.content || '{}'); if (sc.session_id && !sessionId.value) { sessionId.value = sc.session_id; activeConversationId.value = sc.session_id } } catch (_) {} return }
    // Handle session_status for reconnection sync
    if (t === 'session_status') {
      try {
        const st = JSON.parse(ev.content || '{}')
        console.log('[chat] session_status event:', st)
        if (st.session_id) {
          // Busy if main stream is active OR sub-agents are still running
          const busy = !!(st.streaming || st.has_agents)
          streamingSessions.value[st.session_id] = busy
          // Sync lifecycle so isStreaming / isBusy stay consistent
          if (busy) {
            setLifecycle(st.session_id, 'streaming')
          } else {
            setLifecycle(st.session_id, 'idle')
          }
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
      // 捕获当前拓扑状态，用于 InterruptBanner 显示
      const topo = topology.value
      interruptSnapshot.value = {
        progress: pm ? parseInt(pm[1]) : 0,
        last_task: tm ? tm[1] : '',
        trust_locked: topo?.trust_locked ?? false,
        trust_lies: topo?.trust_lies ?? 0,
        reject_count: topo?.reject_count ?? 0,
        warning: topo?.warning ?? '',
      }
      setLifecycle(targetSid, 'idle')
      return
    }

    // ── v2.0 event handlers ──
    if (t === 'state_change' && ev.state_change) {
      const sc = ev.state_change
      const list = currentAgents(targetSid)
      var agent = list.find(a => a.agent_id === sc.agent_id)
      if (!agent) {
        agent = { agent_id: sc.agent_id, state: sc.to, agent_status: sc.to } as AgentInfo
        list.push(agent)
      } else {
        agent.state = sc.to
        agent.agent_status = sc.to
      }
      if (!agent._transitions) agent._transitions = []
      agent._transitions.push({ from: sc.from, to: sc.to, event: sc.event, reason: sc.reason, ts: Date.now() } as StateTransition)
      sessionAgents.value = { ...sessionAgents.value }
      updateSubAgent(targetSid, agent)
      return
    }
    if (t === 'role_change' && ev.role_change) {
      const rc = ev.role_change
      const list = currentAgents(targetSid)
      var roleAgent = list.find(a => a.agent_id === rc.agent_id)
      if (!roleAgent) {
        roleAgent = { agent_id: rc.agent_id, role: rc.new_role } as AgentInfo
        list.push(roleAgent)
      }
      roleAgent.role = rc.new_role as AgentRole
      if (!roleAgent._transitions) roleAgent._transitions = []
      roleAgent._transitions.push({ from: rc.old_role, to: rc.new_role, event: 'role_change', reason: rc.reason, ts: Date.now() } as StateTransition)
      sessionAgents.value = { ...sessionAgents.value }
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
        // 转换文件引用为可点击链接
        const fileRefContent = convertFileRefs(resultContent)
        const agentContent = isQQ
          ? `${isOk ? '✓' : '✗'} ${ev.agent_goal || ev.goal || ''}: ${fileRefContent.slice(0, 200)}`
          : isOk
            ? `子Agent 完成\n> ${ev.agent_goal || ev.goal || ''}\n\n${fileRefContent}`
            : `子Agent 失败\n> ${ev.agent_goal || ev.goal || ''}\n\n${fileRefContent}`
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
        // 子Agent完成时自动恢复主Agent处理结果
        if (isOk && targetSid && !targetSid.startsWith('qqbot_')) {
          setTimeout(() => {
            if (sessionId.value === targetSid && getLifecycle(targetSid) !== 'streaming') {
              sendMessage('继续', '', {})
            }
          }, 500)
        }
      }
      return
    }
    if (t === 'skill_progress') {
      currentToolName.value = ev.skill_name || 'skill'
      currentToolProgress.value = String(ev.skill_current_step || 0) + '/' + String(ev.skill_total_steps || 0)
      return
    }
    if (t === 'cron_notify') { return }
    if (t === 'plan' && ev.plan_result) {
      messages.value.push({
        id: 'plan_' + Date.now(), role: 'system' as ChatMessage['role'],
        content: 'Plan: ' + ev.plan_result.successes + '/' + ev.plan_result.total_steps + ' ok',
        blocks: [{ type: 'content' as const, content: 'Plan completed' }],
        status: 'done' as const, streaming: false, _v: 0, createdAt: Date.now(),
      } as ChatMessage)
      return
    }
    if (t === 'step_result' && ev.step_result) {
      const sr = currentStreamingMsg()
      if (sr) {
        if (!sr.tools) sr.tools = []
        sr.tools.push({ name: ev.step_result.tool, args: '', result: ev.step_result.status === 'success' ? 'OK' : 'FAIL' })
        sr._v = (sr._v || 0) + 1
      }
      return
    }
    if (t === 'goal_progress') { return }
    if (t === 'topology_update') {
      const tu = ev.topology_update as TopologyUpdate | undefined
      // 仅接受当前会话的拓扑事件，防止旧会话 SSE 覆盖新会话状态
      if (tu && tu.session_id === sessionId.value) {
        topology.value = {
          session_id: tu.session_id,
          current_coord: tu.coord,
          start_coord: { x: 0, y: 0, z: 0 },
          constraint: tu.constraint,
          trajectory: (tu.trajectory || []).map((c: any, i: number) => ({
            x: c.x, y: c.y, z: c.z, round: i,
            tool_call: c.tool_call || '', status: c.status || 'committed',
            reason: c.reason || '',
            tool_result: c.tool_result || '',
          })),
          reject_count: tu.reject_count,
          trust_lies: tu.trust_lies,
          trust_locked: tu.trust_locked,
          closed_loop: tu.closed_loop,
          closed_distance: tu.closed_distance,
          warning: tu.warning,
          active: true,
          committed_count: tu.committed_count,
          total_nodes: tu.total_nodes,
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

    // tool_start / tool_progress — update tool status on current streaming msg
    if (t === 'tool_start' || t === 'tool_progress') {
      if (ev.tool) currentToolName.value = ev.tool
      currentToolProgress.value = ev.content || ''
      const pending = currentStreamingMsg()
      if (!pending || !pending.tools) return
      for (let i = 0; i < pending.tools.length; i++) {
        if (!pending.tools[i].result && pending.tools[i].status !== 'running') {
          pending.tools[i] = { ...pending.tools[i], status: 'running', progress: ev.content || '执行中...' }
          break
        }
      }
      pending._v = ++_msgVersion
      return
    }

    let msg = currentStreamingMsg()
    // Auto-create placeholder for reconnection events.
    // Prefer reusing the last assistant message loaded from DB if it's incomplete.
    if (!msg) {
      const msgs = messages.value
      const last = msgs.length > 0 ? msgs[msgs.length - 1] : null
      if (last && last.role === 'assistant' && last.streaming) {
        msg = last
      } else {
        const idx = addAssistantPlaceholder()
        streamingPlaceholders.value = [idx]
        msg = messages.value[idx]
      }
    }
    if (!msg) return

    // ── thinking / content: 直接拼到字符串 ──
    if (t === 'thinking') {
      msg.reasoning = (msg.reasoning || '') + (ev.content || '')
      msg._v = ++_msgVersion
    } else if (t === 'content') {
      msg.content = (msg.content || '') + (ev.content || '')
      msg._v = ++_msgVersion
    } else if (t === 'tool_call') {
      if (!msg.tools) msg.tools = []
      msg.tools.push({ name: ev.tool || 'unknown', args: ev.args || '', result: '', status: 'pending', progress: '' })
      msg._v = ++_msgVersion
    } else if (t === 'tool_result') {
      if (!msg.tools) return
      for (let i = 0; i < msg.tools.length; i++) {
        if (!msg.tools[i].result) {
          msg.tools[i] = { ...msg.tools[i], result: ev.content || '', status: '', progress: '' }
          break
        }
      }
      msg._v = ++_msgVersion
      const hasPending = msg.tools.some((t: any) => !t.result)
      if (!hasPending && msg.tools.length > 0) {
        finalizeOne(msg)
        const newIdx = addAssistantPlaceholder()
        streamingPlaceholders.value = [...streamingPlaceholders.value, newIdx]
      }
    } else if (t === 'error') {
      msg.status = 'error'
      msg.content = (msg.content || '') + '\n\n' + (ev.content || '')
      msg._v = ++_msgVersion
    } else if (t === 'done') {
      const dur = parseInt(ev.content || '') || 0
      if (dur > 0) msg.durationMs = dur
      // Only set idle if no sub-agents are still running.
      // Otherwise the frontend would show idle while agents work in background.
      const hasRunning = agentActiveSessions.value[targetSid] ||
        (sessionAgents.value[targetSid] || []).some(a =>
          a.agent_status === 'running' || a.agent_status === 'pending' ||
          a.status === 'running' || a.status === 'pending')
      if (!hasRunning) {
        setLifecycle(targetSid, 'idle')
      }
      currentToolName.value = ''
      currentToolProgress.value = ''
      if (msg.content || msg.reasoning || (msg.tools && msg.tools.length > 0)) {
        finalizeOne(msg)
      }
      streamingPlaceholders.value = streamingPlaceholders.value.filter(p => p !== messages.value.indexOf(msg))
    }
  }

  function finalizeOne(msg: ChatMessage): void {
    if (msg.status === 'error') {
      msg.contentHtml = '<p class="error-text">' + (msg.content || '请求失败') + '</p>'
      msg.streaming = false
      msg._v = ++_msgVersion
      return
    }
    msg.contentHtml = renderMarkdown(msg.content || '')
    // tools already populated during streaming via tool_call/tool_result
    if (!msg.tools) msg.tools = []
    msg.status = 'done'
    msg.streaming = false
    msg._v = ++_msgVersion
  }

  const _titleRequested = new Set<string>()

  function requestTitle(sid: string) {
    if (_titleRequested.has(sid)) return
    _titleRequested.add(sid)
    const msgs = _sessionMsgs[sid] || messages.value
    const firstUser = msgs.find((m: ChatMessage) => m.role === 'user' && m.content)
    if (!firstUser) return
    // Fire-and-forget: backend generates title asynchronously
    const token = localStorage.getItem('token')
    fetch('/api/chat/title', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ session_id: sid, message: firstUser.content }),
    }).catch(() => {})
    // Refresh sidebar after backend has time to generate title
    setTimeout(() => loadConversations(), 500)
  }

  function finalizeStream(): void {
    const last = currentStreamingMsg()
    if (last && last.streaming) {
      finalizeOne(last)
    }
    messages.value = messages.value.filter((m) => {
      if (!m) return false // safety: filter null entries
      if (m.streaming && m.role === 'assistant') return false
      if (m.role === 'assistant' && m.status === 'done' && !(m.content || m.reasoning || (m.tools && m.tools.length > 0))) return false
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
            name: tc.function?.name || tc.name || '',
            args: tc.function?.arguments || tc.args || '',
            result: tc.result || '',
          })),
          status: 'done',
          streaming: false,
          _v: i,
          createdAt: m.created_at ? new Date(m.created_at).getTime() : Date.now(),
        }

        return msg
      })
    restoreToolProgress()
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
    console.log('[store] clearCurrent called, sid=' + sessionId.value)
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

  // Per-session stream tracking — multiple sessions can have active SSE connections
  type StreamState = { abort: AbortController; promise: Promise<void>; sid: string }
  const _streams: Record<string, StreamState> = {}
  const _sending: Record<string, boolean> = {}
  let _switchToken = 0
  let _reconnectTimer: ReturnType<typeof setTimeout> | null = null

  async function connectStream(sid: string): Promise<void> {
    // Already connected for this session
    if (_streams[sid]) return
    console.log('[chat] connectStream: start', sid)
    const token = localStorage.getItem('token')
    const controller = new AbortController()
    const state: StreamState = { abort: controller, promise: Promise.resolve(), sid }
    _streams[sid] = state

    state.promise = fetchEventSource('/api/chat/stream/' + sid, {
      headers: { Authorization: 'Bearer ' + token },
      signal: controller.signal,
      onmessage(ev) {
        try {
          processSSEEvent(JSON.parse(ev.data), sid)
        } catch (e) {}
      },
      onerror(err) {
        throw err
      },
      onclose() {
        // Only cleanup if this specific stream is still tracked
        if (_streams[sid] && _streams[sid].abort === controller) {
          delete _streams[sid]
        }
        if (sessionId.value === sid) {
          const lc = getLifecycle(sid)
          if (lc !== 'streaming') {
            const idx = streamingPlaceholders.value[0]
            if (idx != null) {
              const m = messages.value[idx]
              if (m && m.streaming && !m.content && !m.reasoning) {
                messages.value.splice(idx, 1)
              }
            }
            streamingPlaceholders.value = []
          }
        }
      },
    })
  }

  async function disconnectStream(sid?: string): Promise<void> {
    const targetSid = sid || sessionId.value
    if (!targetSid) return
    delete streamingSessions.value[targetSid]
    if (_reconnectTimer) { clearTimeout(_reconnectTimer); _reconnectTimer = null }
    const st = _streams[targetSid]
    if (st) {
      console.log('[chat] disconnectStream:', targetSid)
      try { st.abort.abort() } catch (_) {}
      delete _streams[targetSid]
      try { await st.promise } catch (_) {}
    }
  }

  // Simplified debounce — clean setTimeout/clearTimeout pattern
  const debouncedReconnect = (targetSid: string): void => {
    if (_reconnectTimer) clearTimeout(_reconnectTimer)
    _reconnectTimer = setTimeout(() => {
      _reconnectTimer = null
      if (sessionId.value === targetSid) connectStream(targetSid)
    }, 150)
  }

  // Clean up ALL streaming sessions (used on route leave / page unload)
  function cleanupAllStreams(): void {
    console.log('[chat] cleanupAllStreams: streams:', Object.keys(_streams).length)
    if (_reconnectTimer) { clearTimeout(_reconnectTimer); _reconnectTimer = null }
    for (const k of Object.keys(_streams)) {
      try { _streams[k].abort.abort() } catch (_) {}
      delete _streams[k]
    }
    const keys = Object.keys(streamingSessions.value)
    for (const k of keys) {
      delete streamingSessions.value[k]
      delete agentActiveSessions.value[k]
    }
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
    dbg('switchConv', 'to=' + (id || '').slice(-8), 'from=' + (sessionId.value || '').slice(-8))
    const token = ++_switchToken

    if (activeConversationId.value === id) return true

    // Save current messages to per-session store before leaving
    const oldSid = sessionId.value
    if (oldSid) {
      _sessionMsgs[oldSid] = [...messages.value]
      delete _sending[oldSid]
    }

    // Keep old session's SSE alive — it continues receiving events in background
    // Events for non-active sessions go to _sessionMsgs[oldSid] via processSSEEvent

    if (token !== _switchToken) return false

    // Switch to new session
    activeConversationId.value = id
    sessionId.value = id

    // Preserve target session's lifecycle — don't overwrite with 'idle'
    const targetLc = getLifecycle(id)
    const wasStreaming = targetLc === 'streaming' || targetLc === 'reconnecting' || targetLc === 'connecting'

    // Only clear transient UI state if target is NOT streaming
    if (!wasStreaming) {
      confirmRequest.value = null
      todoList.value = []
      streamingPlaceholders.value = []
      currentToolName.value = ''
      currentToolProgress.value = ''
      loading.value = false
    }
    // interruptSnapshot 和 topology 已由 sessionId watch 自动恢复，无需手动清除

    // Always fetch from DB first to get canonical state.
    // Cache may have stale partial messages from interrupted streams.
    messages.value = []
    const dbOk = await fetchSessionMessages(id)
    if (token !== _switchToken) return false

    // Merge DB + cache: dedup by (role + first 80 chars of content)
    // since DB and frontend generate different message IDs
    const seen = new Set<string>()
    for (const m of messages.value) {
      seen.add(m.role + '|' + (m.content || '').slice(0, 80))
    }
    const cached = _sessionMsgs[id]
    if (cached && cached.length > 0) {
      for (const cm of cached) {
        const key = cm.role + '|' + (cm.content || '').slice(0, 80)
        if (!seen.has(key)) {
          seen.add(key)
          messages.value.push(cm)
        }
      }
    }
    _sessionMsgs[id] = [...messages.value]

    if (!dbOk && messages.value.length === 0) {
      clearCurrent()
      sessionId.value = id
      activeConversationId.value = ''
      return false
    }

    // Clean up any stale streaming state from cached messages
    for (const m of messages.value) {
      if (m.streaming && m.status !== 'streaming') m.streaming = false
    }
    streamingPlaceholders.value = []
    restoreToolProgress()
    if (token === _switchToken) debouncedReconnect(id)
    return true
  }

  function updateSubAgent(convId: string, agent: AgentInfo): void {
    // 更新 Sidebar 子 Agent 列表
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
    // 同步到 sessionAgents，使右侧 "活跃助手" 面板也能显示
    if (!sessionAgents.value[convId]) sessionAgents.value[convId] = []
    const saList = sessionAgents.value[convId]
    const saIdx = saList.findIndex((a) => a.agent_id === agent.agent_id)
    const agentEntry: AgentInfo = {
      agent_id: agent.agent_id,
      agent_goal: agent.agent_goal || '',
      agent_status: agent.agent_status || agent.status || 'running',
      agent_round: agent.agent_round || 0,
      content: agent.content || '',
      state: (agent.state || agent.agent_status || 'executing') as import('../types/chat').AgentState,
    }
    if (saIdx >= 0) {
      saList.splice(saIdx, 1, { ...saList[saIdx], ...agentEntry })
    } else {
      saList.push(agentEntry)
    }
    // 触发 ref 响应
    sessionAgents.value = { ...sessionAgents.value }
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
