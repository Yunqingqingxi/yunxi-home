<template>
  <div class="terminal-page">
    <h3>Web 终端</h3>
    <div class="glass-card terminal-card">
      <div class="term-bar">
        <span class="term-dot red"></span><span class="term-dot yellow"></span><span class="term-dot green"></span>
        <span class="term-title">Terminal</span>
        <div class="term-controls">
          <button
            class="term-btn"
            title="Zoom out"
            @click="zoomOut"
          >
            A-
          </button>
          <button
            class="term-btn"
            title="Zoom in"
            @click="zoomIn"
          >
            A+
          </button>
          <button
            class="term-btn"
            title="Search"
            @click="toggleSearch"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 12 12"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
            ><circle
              cx="5"
              cy="5"
              r="3.5"
            /><path d="M8 8l3 3" /></svg>
          </button>
          <button
            class="term-btn"
            title="Clear"
            @click="clearTerminal"
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 12 12"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
            ><path d="M2 2l8 8M10 2l-8 8" /></svg>
          </button>
        </div>
        <span
          class="term-status"
          :class="{ connected }"
        >{{ connected ? '已连接' : '连接中...' }}</span>
      </div>
      <div
        ref="termEl"
        class="term-body"
      ></div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'

const termEl = ref(null)
const connected = ref(false)
const fontSize = ref(14)
let term = null, fitAddon = null, searchAddon = null, ws = null
const MIN_FONT = 10, MAX_FONT = 22

function zoomIn() { if (fontSize.value >= MAX_FONT) return; fontSize.value += 1; if (term) { term.options.fontSize = fontSize.value; fitAddon?.fit() } }
function zoomOut() { if (fontSize.value <= MIN_FONT) return; fontSize.value -= 1; if (term) { term.options.fontSize = fontSize.value; fitAddon?.fit() } }
function clearTerminal() { term?.clear() }
function toggleSearch() { searchAddon?.findNext('') }

function connect() {
  if (connected.value) return
  if (term) { try { term.dispose() } catch (_) {}; term = null }
  if (ws) { try { ws.close() } catch (_) {}; ws = null }
  term = new Terminal({ cursorBlink: true, fontSize: fontSize.value, fontFamily: '"Geist Mono", "Cascadia Code", Consolas, monospace', theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#0ea5e9', selectionBackground: 'rgba(14,165,233,0.25)' }, allowProposedApi: true, copyOnSelect: true })
  fitAddon = new FitAddon(); searchAddon = new SearchAddon()
  term.loadAddon(fitAddon); term.loadAddon(new WebLinksAddon()); term.loadAddon(searchAddon)
  term.open(termEl.value); fitAddon.fit()
  const token = localStorage.getItem('token')
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = proto + '://' + location.host + '/api/terminal'
  ws = new WebSocket(url + '?token=' + token)
  let reconnectDelay = 1000
  const MAX_RECONNECT_DELAY = 30000
  ws.onopen = () => { reconnectDelay = 1000; connected.value = true; term.focus(); term.onData(data => { if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data })) }); term.onResize(({ cols, rows }) => { if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', data: { cols, rows } })) }) }
  term.element?.addEventListener('contextmenu', async (e) => { e.preventDefault(); if (!connected.value) return; try { const text = await navigator.clipboard.readText(); if (text && ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data: text })) } catch (_) {} })
  ws.onmessage = (ev) => { if (typeof ev.data === 'string' && term) term.write(ev.data) }
  ws.onclose = () => { if (!connected.value) return; connected.value = false; if (term) term.write('\r\n\x1b[31m连接已断开，' + (reconnectDelay / 1000).toFixed(0) + '秒后重试...\x1b[0m\r\n'); if (ws) { try { ws.close() } catch (_) {}; ws = null }; setTimeout(() => { if (!connected.value && termEl.value) { reconnectDelay = Math.min(reconnectDelay * 1.5, MAX_RECONNECT_DELAY); connect() } }, reconnectDelay) }
  ws.onerror = () => { connected.value = false }
  term.attachCustomKeyEventHandler((e) => { if (e.ctrlKey && e.shiftKey && e.key === 'F') { searchAddon.findNext(''); return false }; if (e.ctrlKey && e.shiftKey && e.key === 'V') { if (!connected.value) return true; navigator.clipboard.readText().then(text => { if (text && ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'input', data: text })) }).catch(() => {}); return false }; if (e.ctrlKey && e.shiftKey && e.key === 'C') { const sel = term.getSelection(); if (sel && navigator.clipboard) navigator.clipboard.writeText(sel).catch(() => {}); return false }; return true })
}
function disconnect() { if (ws) { ws.onclose = null; ws.onerror = null; ws.onmessage = null; ws.close(); ws = null }; if (term) { term.dispose(); term = null }; connected.value = false }
const resizeObserver = new ResizeObserver(() => { if (fitAddon) try { fitAddon.fit() } catch (e) {} })
onMounted(async () => { await nextTick(); if (termEl.value && termEl.value instanceof Element) { try { resizeObserver.observe(termEl.value) } catch (e) {} }; connect() })
onUnmounted(() => { disconnect(); resizeObserver.disconnect() })
</script>

<style scoped>
.terminal-page { display: flex; flex-direction: column; gap: 12px; height: calc(100vh - 120px); }
.terminal-page h3 { margin: 0; font-size: var(--text-xl); font-weight: var(--weight-bold); color: var(--text-primary); flex-shrink: 0; }
.terminal-card { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 0; }
.term-bar { display: flex; align-items: center; gap: 6px; padding: 7px 12px; background: rgba(0,0,0,0.03); border-bottom: 1px solid var(--border-default); flex-shrink: 0; border-radius: var(--radius-lg) var(--radius-lg) 0 0; }
.term-dot { width: 10px; height: 10px; border-radius: 50%; }
.term-dot.red { background: #ef4444; } .term-dot.yellow { background: #f59e0b; } .term-dot.green { background: #22c55e; }
.term-title { font-size: var(--text-xs); color: var(--text-muted); flex: 1; }
.term-controls { display: flex; align-items: center; gap: 1px; }
.term-btn { width: 26px; height: 22px; display: flex; align-items: center; justify-content: center; font-size: 10.5px; padding: 0; border: none; border-radius: var(--radius-xs); background: transparent; color: var(--text-muted); cursor: pointer; font-family: inherit; transition: all 0.1s; }
.term-btn:hover { background: rgba(0,0,0,0.05); color: var(--text-primary); }
.term-status { font-size: 10.5px; font-weight: var(--weight-semibold); color: #ef4444; white-space: nowrap; }
.term-status.connected { color: #22c55e; }
.term-body { flex: 1; min-height: 0; }
.term-body :deep(.xterm) { height: 100%; padding: 6px; }
.term-body :deep(.xterm-viewport) { overflow-y: auto; scrollbar-width: thin; }
@media (max-width: 767px) { .terminal-page { height: calc(100vh - 100px); } .term-bar { padding: 5px 8px; gap: 4px; } .term-title { font-size: 10.5px; } .term-btn { width: 24px; height: 20px; } .term-dot { width: 8px; height: 8px; } }
</style>