<template>
  <svg style="position:absolute;width:0;height:0" aria-hidden="true">
    <defs>
      <linearGradient id="fg-md" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#6D93FF"/><stop offset="1" stop-color="#5A71F0"/></linearGradient>
      <linearGradient id="fg-doc" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#2B7FFF"/><stop offset="1" stop-color="#1A5CD0"/></linearGradient>
      <linearGradient id="fg-ppt" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#FF6B35"/><stop offset="1" stop-color="#D9441E"/></linearGradient>
      <linearGradient id="fg-xls" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#21A366"/><stop offset="1" stop-color="#147A48"/></linearGradient>
      <linearGradient id="fg-pdf" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#E53935"/><stop offset="1" stop-color="#B71C1C"/></linearGradient>
      <linearGradient id="fg-txt" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#78909C"/><stop offset="1" stop-color="#546E7A"/></linearGradient>
      <linearGradient id="fg-img" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#AB47BC"/><stop offset="1" stop-color="#8E24AA"/></linearGradient>
      <linearGradient id="fg-zip" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#FFA726"/><stop offset="1" stop-color="#F57C00"/></linearGradient>
      <linearGradient id="fg-default-0" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#6D93FF"/><stop offset="1" stop-color="#5A71F0"/></linearGradient>
      <linearGradient id="fg-default-1" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#26C6DA"/><stop offset="1" stop-color="#0097A7"/></linearGradient>
      <linearGradient id="fg-default-2" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#66BB6A"/><stop offset="1" stop-color="#388E3C"/></linearGradient>
      <linearGradient id="fg-default-3" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse"><stop stop-color="#FF7043"/><stop offset="1" stop-color="#E64A19"/></linearGradient>
    </defs>
  </svg>
  <div :class="store.sessionId ? 'input-bar-floating' : 'input-bar-bottom'">
    <!-- Attachments & hints -->
    <div v-if="attachedFiles.length" class="attach-bar">
      <div v-for="(f, i) in attachedFiles" :key="i" class="file-card" :class="{ uploading: f.uploading }">
        <div class="file-card-icon">
          <svg width="24" height="28" viewBox="0 0 24 28" fill="none">
            <path d="M16.5 0l7 7v15.6c0 2.25 0 3.375-.573 4.164a3 3 0 0 1-.663.663C21.475 28 20.349 28 18.1 28H5.9c-2.25 0-3.375 0-4.164-.573a3 3 0 0 1-.663-.663C.5 25.975.5 24.849.5 22.6V5.4c0-2.25 0-3.375.573-4.164a3 3 0 0 1 .663-.663C2.525 0 3.651 0 5.9 0h10.6z" :fill="fileIconGradient(f.name, i)"/><path d="M16.5 0l7 7h-3.8c-1.12 0-1.68 0-2.108-.218a2 2 0 0 1-.874-.874C16.5 5.48 16.5 4.92 16.5 3.8V0z" fill="#fff" fill-opacity=".55"/><path d="M6 11.784c0-.433.351-.784.784-.784h10.432a.784.784 0 1 1 0 1.568H6.784A.784.784 0 0 1 6 11.784zM6 15.784c0-.433.351-.784.784-.784h10.432a.784.784 0 1 1 0 1.568H6.784A.784.784 0 0 1 6 15.784zM6.114 19.817c0-.433.35-.784.784-.784h6.318a.784.784 0 1 1 0 1.568H6.898a.784.784 0 0 1-.784-.784z" fill="#fff"/>
          </svg>
        </div>
        <div class="file-card-info">
          <span class="file-card-name">{{ f.name }}</span>
          <span class="file-card-meta">{{ fileExt(f.name).toUpperCase() }} {{ fmtFileSize(f.size) }}</span>
        </div>
        <span v-if="f.uploading" class="file-card-progress">{{ f.progress }}%</span>
        <div class="file-card-close" @click.stop="removeFile(i)" tabindex="0">
          <svg width="12" height="12" viewBox="0 0 14 14" fill="none"><path d="M10.6 4.4L7 8L3.4 4.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><path d="M10.6 10.6L7 7L3.4 10.6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </div>
      </div>
    </div>

    <!-- Main input row: textarea + actions inline -->
    <div class="input-main">
      <input ref="fileInput" type="file" style="display:none" @change="onFileAttach" multiple />
      <button class="act-btn" title="附加文件" @click="$refs.fileInput.click()">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M10 4v7a3 3 0 01-6 0V5a2 2 0 014 0v5.5a1 1 0 01-2 0V5"/></svg>
      </button>

      <button class="act-btn" :class="{ active: planMode }" @click="planMode = !planMode" title="Plan 多步规划">
        <svg width="14" height="14" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.3"><rect x="1.5" y="1.5" width="4" height="3" rx="0.8"/><rect x="7.5" y="1.5" width="4" height="3" rx="0.8"/><rect x="1.5" y="8.5" width="4" height="3" rx="0.8"/><rect x="7.5" y="8.5" width="4" height="3" rx="0.8"/></svg>
      </button>

      <div class="input-wrapper">
        <textarea ref="inputEl" v-model="input" class="input-field" :class="['model-'+modelKey, { focused: inputFocused, 'cmd-mode': showCommands }]" :placeholder="inputPlaceholder"
          :rows="1"
          @keydown="onKeydown" @focus="inputFocused = true" @blur="inputFocused = false"
          @input="onInputResize" maxlength="4000"></textarea>
        <button type="button" class="model-dot" :class="modelKey" @mousedown.prevent @click.stop="modelMenuOpen = !modelMenuOpen" :title="currentModelName + ' ▼'"></button>
        <div v-if="modelMenuOpen" class="model-dropdown" @click.stop>
          <button v-for="m in modelOptions" :key="m.key"
            :class="['model-opt', { active: modelKey === m.key }]"
            @click="onModelChange(m.key); modelMenuOpen = false">
            <span class="seg-dot" :class="m.key"></span>
            <div class="model-opt-info">
              <span class="model-opt-label">{{ m.label }}</span>
              <span class="model-opt-desc">{{ m.desc }}</span>
            </div>
          </button>
          <div class="model-dropdown-div"></div>
          <div class="model-reasoning-row">
            <span class="model-reasoning-label">推理</span>
            <button v-for="r in reasoningLevels" :key="r.key"
              :class="['seg-item', { active: reasoning === r.key }]"
              @click="reasoning = r.key">
              {{ r.label }}
            </button>
          </div>
        </div>
      </div>

      <button
        :class="['send-btn', isBusy && !input.trim() ? 'stop' : '', hasAgent ? 'agent-running' : '', input.trim() || attachedFiles.length ? 'active' : '']"
        :disabled="!isBusy && !input.trim() && !attachedFiles.length"
        @click="isBusy && !input.trim() ? stopGeneration() : doSend()"
        :title="hasAgent ? 'Agent 执行中，输入消息将注入不中断' : isBusy ? '停止' : '发送'"
      >
        <svg v-if="isBusy && !input.trim()" width="14" height="14" viewBox="0 0 13 13" fill="currentColor"><rect x="2.5" y="2.5" width="8" height="8" rx="1.5"/></svg>
        <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M22 2L11 13"/><path d="M22 2L15 22l-4-9-9-4 20-7z"/></svg>
      </button>
    </div>

    <!-- Command palette -->
    <div v-if="showCommands" class="cmd-palette">
      <div v-for="(cmd, i) in filteredCommands" :key="cmd.name"
        :class="['cmd-item', { active: i === cmdIndex }]"
        @mousedown.prevent @click="selectCommand(cmd)">
        <span class="cmd-name">/{{ cmd.name }}</span>
        <span v-if="cmd.args" class="cmd-args">{{ cmd.args }}</span>
        <span class="cmd-desc">{{ cmd.desc }}</span>
      </div>
      <div v-if="!filteredCommands.length" class="cmd-empty">无匹配指令</div>
    </div>

    <CronPanel v-if="store.sessionId" />
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, nextTick, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useChatStore } from '../../stores/chat'
import { useSettingsStore } from '../../stores/settings'
import CronPanel from './CronPanel.vue'

const router = useRouter()
const store = useChatStore()

// ── 发送按钮状态计算 ──
const isBusy = computed(() => store.isStreaming || store.hasRunningAgents)
const hasAgent = computed(() => store.hasRunningAgents && !store.isStreaming)

const input = ref('')
const inputEl = ref(null)
const fileInput = ref(null)
const inputFocused = ref(false)
const planMode = ref(false)

// ── File attach ──
const attachedFiles = ref([])

// ── Hints ──
const quickHints = computed(() => store.hintTexts?.slice(0, 3) || [])

// ── Model & reasoning ──
const settingsStore = useSettingsStore()

function getDefaultModel() {
  const cfg = settingsStore.config
  if (cfg?.ai?.default_model) return cfg.ai.default_model
  return 'deepseek-v4-flash'
}
function getDefaultReasoning() {
  const cfg = settingsStore.config
  if (cfg?.ai?.default_reasoning) return cfg.ai.default_reasoning
  return 'high'
}
function getEnabledModels() {
  const cfg = settingsStore.config
  if (cfg?.ai?.enabled_models?.length) return cfg.ai.enabled_models
  return ['deepseek-v4-flash', 'deepseek-v4-pro', 'qwen-plus', 'qwen-max']
}

const allModelDefs = [
  { key: 'flash', label: 'Flash', desc: 'DeepSeek 轻量', model: 'deepseek-v4-flash', reasoning: 'low' },
  { key: 'pro',   label: 'Pro',   desc: 'DeepSeek 推理', model: 'deepseek-v4-pro',   reasoning: 'high' },
  { key: 'qwen-plus', label: 'Plus', desc: '通义千问 Plus', model: 'qwen-plus', reasoning: 'low' },
  { key: 'qwen-max',  label: 'Max',  desc: '通义千问 Max',  model: 'qwen-max',  reasoning: 'medium' },
]

const modelOptions = computed(() => {
  const enabled = getEnabledModels()
  return allModelDefs.filter(m => enabled.includes(m.model))
})

function findModelKey(modelName) {
  const m = allModelDefs.find(o => o.model === modelName)
  return m ? m.key : 'flash'
}

const modelKey = ref(findModelKey(getDefaultModel()))
const modelMenuOpen = ref(false)
const reasoning = ref(getDefaultReasoning())

const currentModelLabel = computed(() => {
  const m = allModelDefs.find(o => o.key === modelKey.value)
  return m ? m.label : 'Flash'
})
const reasoningLevels = [
  { key: 'low',    label: '快速' },
  { key: 'medium', label: '均衡' },
  { key: 'high',   label: '深度' },
]

function onModelChange(key) {
  modelKey.value = key
  const m = allModelDefs.find(o => o.key === key)
  if (m) reasoning.value = m.reasoning
}

const currentModelName = computed(() => {
  const m = modelOptions.value.find(o => o.key === modelKey.value)
  return m ? m.model : 'deepseek-v4-flash'
})

// ── Commands ──
const cmdIndex = ref(0)
const commands = ref([])       // 动态从 API 加载
const commandsLoaded = ref(false)

// 页面加载时获取命令列表
async function loadCommands() {
  try {
    const token = localStorage.getItem('token')
    const res = await fetch('/api/chat/commands', { headers: { Authorization: 'Bearer ' + token } })
    if (res.ok) {
      const data = await res.json()
      commands.value = (data.data?.commands || []).map(c => ({
        name: c.name,
        desc: c.description || '',
        args: c.usage ? c.usage.replace('/' + c.name, '').trim() : '',
        type: c.type || 'builtin',
        skillName: c.skill_name || ''
      }))
    }
  } catch (_) {}
  commandsLoaded.value = true
}
loadCommands()

// 仅输入 / 时即展示面板（不需要额外字符）
const showCommands = computed(() => input.value.startsWith('/') && !input.value.includes(' '))
const filteredCommands = computed(() => {
  const q = input.value.slice(1).toLowerCase()
  if (!q) return commands.value  // 只输入 / 显示全部
  // 模糊搜索：名称 + 描述
  return commands.value.filter(c =>
    c.name.toLowerCase().includes(q) ||
    c.desc.toLowerCase().includes(q)
  )
})

function selectCommand(cmd) {
  if (cmd.args) {
    input.value = '/' + cmd.name + ' '
  } else {
    input.value = '/' + cmd.name
    doSend()
  }
  cmdIndex.value = 0
  nextTick(() => inputEl.value?.focus())
}

// ── Placeholder ──
const inputPlaceholder = computed(() => {
  if (!store.sessionId) return '描述你想做什么...'
  return '输入消息，Enter 发送，/ 指令'
})

// ── Helpers ──
function fileExt(name) {
  const i = (name || '').lastIndexOf('.')
  return i >= 0 ? name.slice(i + 1) : ''
}
function fmtFileSize(bytes) {
  if (!bytes || bytes < 0) return '0 B'
  const k = 1024, sizes = ['B','KB','MB','GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
const gradients = ['fg-md','fg-doc','fg-ppt','fg-xls','fg-pdf','fg-txt','fg-img','fg-zip']
function fileIconGradient(name, i) {
  const ext = fileExt(name).toLowerCase()
  const map = { md:'fg-md', doc:'fg-doc', docx:'fg-doc', ppt:'fg-ppt', pptx:'fg-ppt', xls:'fg-xls', xlsx:'fg-xls', pdf:'fg-pdf', txt:'fg-txt', jpg:'fg-img', png:'fg-img', gif:'fg-img', webp:'fg-img', svg:'fg-img', zip:'fg-zip', rar:'fg-zip', '7z':'fg-zip', tar:'fg-zip', gz:'fg-zip' }
  return `url(#${map[ext] || gradients[i % gradients.length]})`
}

// ── Input resize ──
function onInputResize() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 140) + 'px'
}

// ── Key handler ──
function onKeydown(e) {
  if (showCommands.value) {
    if (e.key === 'ArrowDown') { e.preventDefault(); cmdIndex.value = Math.min(cmdIndex.value + 1, filteredCommands.value.length - 1); return }
    if (e.key === 'ArrowUp') { e.preventDefault(); cmdIndex.value = Math.max(cmdIndex.value - 1, 0); return }
    if (e.key === 'Tab' || e.key === 'Enter') { e.preventDefault(); const cmd = filteredCommands.value[cmdIndex.value]; if (cmd) selectCommand(cmd); return }
    if (e.key === 'Escape') { input.value = ''; cmdIndex.value = 0; return }
  }
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); store.isStreaming && !input.trim() ? stopGeneration() : doSend() }
}
function stopGeneration() { store.resetStreaming() }

// ── Send ──
async function doSend() {
  const t = input.value.trim()
  const files = attachedFiles.value.filter(f => f.file)
  if (!t && !files.length) return

  // / 指令通过 AI 处理（自然持久化到会话历史，AI 可调用工具执行）
  if (t.startsWith('/') && !files.length) {
    store.sendMessage(t, currentModelName.value, {
      reasoning_intensity: reasoning.value,
      plan_mode: planMode.value
    })
    input.value = ''
    inputEl.value && (inputEl.value.style.height = 'auto')
    return
  }

  let msg = t
  // Upload files first
  const token = localStorage.getItem('token')
  const uploadedPaths = []
  for (const f of files) {
    f.uploading = true
    try {
      const form = new FormData()
      form.append('file', f.file)
      form.append('path', '/upload/' + f.name)
      const res = await fetch('/api/nas/files/upload', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token },
        body: form
      })
      if (res.ok) {
        const data = await res.json()
        uploadedPaths.push(data.data?.path || '/upload/' + f.name)
        f.uploaded = true
      }
    } catch (e) { /* skip failed */ }
    f.uploading = false
  }
  // Build message with file references
  if (uploadedPaths.length) {
    const fileRefs = uploadedPaths.map(p => '[文件: ' + p.split('/').pop() + ' (' + p + ')]').join('\n')
    msg = t ? t + '\n\n' + fileRefs : fileRefs
  }

  input.value = ''
  inputEl.value && (inputEl.value.style.height = 'auto')
  attachedFiles.value = []
  store.sendMessage(msg, currentModelName.value, {
    reasoning_intensity: reasoning.value,
    plan_mode: planMode.value
  })
}

function sendHint(t) { input.value = t; nextTick(() => inputEl.value?.focus()); doSend() }

function removeFile(i) { attachedFiles.value.splice(i, 1) }
function onFileAttach(e) {
  const files = e.target.files
  if (!files?.length) return
  for (const f of files) {
    attachedFiles.value.push({ name: f.name, size: f.size, file: f, uploaded: false, progress: 0 })
  }
  e.target.value = ''
}

function onDocClick(e) {
  if (modelMenuOpen.value && !e.target.closest('.model-select')) modelMenuOpen.value = false
}
onMounted(() => {
  nextTick(() => inputEl.value?.focus())
  document.addEventListener('click', onDocClick)
})
</script>

<style scoped>
/* ── Input bar container ── */
.input-bar-bottom {
  flex-shrink: 0; padding: 12px 20px 14px;
  border-top: 1px solid var(--border-subtle);
  background: transparent;
}
.input-bar-floating {
  position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%);
  z-index: 200; max-width: 740px; width: calc(100% - 32px);
  background: rgba(255,255,255,0.72);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid rgba(0,0,0,0.06);
  border-radius: 20px;
  box-shadow:
    0 0 0 1px rgba(0,0,0,0.03),
    0 2px 8px rgba(0,0,0,0.04),
    0 8px 32px rgba(0,0,0,0.06);
  padding: 12px 14px 10px;
  display: flex; flex-direction: column; gap: 8px;
  transition: box-shadow 0.2s, border-color 0.2s;
}
[data-theme="dark"] .input-bar-floating {
  background: rgba(15,23,42,0.78);
  border-color: rgba(255,255,255,0.08);
  box-shadow:
    0 0 0 1px rgba(255,255,255,0.03),
    0 2px 8px rgba(0,0,0,0.15),
    0 8px 32px rgba(0,0,0,0.25);
}

/* ── Main input row ── */
.input-main {
  display: flex; align-items: flex-end; gap: 8px;
}
.input-field {
  flex: 1; border: 1px solid rgba(0,0,0,0.06); background: rgba(0,0,0,0.02); outline: none;
  font-size: 14px; color: var(--text-primary); font-family: inherit;
  line-height: 1.5; resize: none; padding: 8px 12px;
  min-height: 38px; max-height: 140px; border-radius: 12px;
  transition: border-color 0.15s, box-shadow 0.15s, height 0.15s ease;
}
.input-field.focused {
  border-color: rgba(6,182,212,0.25);
  box-shadow: 0 0 0 3px rgba(6,182,212,0.06);
}
.input-field::placeholder { color: var(--text-muted); }
[data-theme="dark"] .input-field { background: rgba(255,255,255,0.03); border-color: rgba(255,255,255,0.06); }
[data-theme="dark"] .input-field.focused { border-color: rgba(34,211,238,0.2); }

/* ── Action buttons row ── */
.input-actions {
  display: flex; align-items: center; gap: 4px; flex-shrink: 0;
}
.act-btn {
  width: 32px; height: 32px; border-radius: 8px; border: none;
  background: transparent; color: var(--text-muted); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.12s;
}
.act-btn:hover { background: var(--surface-hover); color: var(--text-primary); }
.act-btn.active { color: var(--brand-500); background: var(--brand-50); }

/* ── Input wrapper (textarea + model dot) ── */
.input-wrapper { flex: 1; position: relative; display: flex; }
.input-wrapper .input-field { width: 100%; padding-left: 28px; }

/* ── Model dot inside input ── */
.model-dot {
  position: absolute; left: 10px; top: 50%; transform: translateY(-50%);
  width: 10px; height: 10px; min-width: 10px; min-height: 10px; flex-shrink: 0;
  border-radius: 50%; border: none; padding: 0;
  cursor: pointer; z-index: 2; background: #6D93FF;
  box-shadow: 0 0 0 2px rgba(0,0,0,0.04);
  transition: transform 0.15s;
}
.model-dot:hover { transform: translateY(-50%) scale(1.3); }
.model-dot.flash { background: #6D93FF; }
.model-dot.pro { background: #A78BFA; }
.model-dot.qwen-plus { background: #34d399; }
.model-dot.qwen-max { background: #f472b6; }

/* ── Model dropdown ── */
.model-dropdown {
  position: absolute; bottom: calc(100% + 8px); left: -4px;
  min-width: 180px; background: var(--surface-raised);
  border: 1px solid var(--border-default); border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12); z-index: 60;
  padding: 6px; display: flex; flex-direction: column; gap: 2px;
}
.model-opt {
  display: flex; align-items: center; gap: 8px;
  padding: 7px 10px; border-radius: 8px; border: none;
  background: transparent; cursor: pointer; font-family: inherit;
  font-size: 12px; color: var(--text-primary); text-align: left;
  transition: all 0.12s;
}
.model-opt:hover { background: var(--surface-hover); }
.model-opt.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }
.model-opt-info { display: flex; flex-direction: column; gap: 1px; flex: 1; }
.model-opt-label { font-weight: 500; }
.model-opt-desc { font-size: 10px; color: var(--text-muted); }
.model-dropdown-div { height: 1px; background: var(--border-subtle); margin: 4px 0; }
.model-reasoning-row {
  display: flex; align-items: center; gap: 4px;
  padding: 4px 6px;
}
.model-reasoning-label { font-size: 10px; color: var(--text-muted); margin-right: 4px; }
.model-reasoning-row .seg-item {
  padding: 2px 6px; border-radius: 4px; border: none;
  background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 10px; font-family: inherit;
}
.model-reasoning-row .seg-item:hover { color: var(--text-primary); }
.model-reasoning-row .seg-item.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }

/* ── Send button ── */
.send-btn {
  width: 36px; height: 36px; border-radius: 10px; border: none;
  background: var(--brand-500); color: #fff; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s; flex-shrink: 0; margin-left: 2px;
  box-shadow: 0 2px 8px rgba(6,182,212,0.25);
}
.send-btn:not(.active) { background: var(--surface-hover); color: var(--text-muted); box-shadow: none; }
.send-btn:hover:not(:disabled) { background: var(--brand-600); color: #fff; box-shadow: 0 4px 12px rgba(6,182,212,0.35); }
.send-btn:disabled { opacity: 0.3; cursor: default; }
.send-btn.stop { background: var(--color-danger); color: #fff; }

.model-opt-icon { font-size: 12px; width: 16px; text-align: center; flex-shrink: 0; }

/* Command mode highlight */
.input-field.cmd-mode {
  border-color: rgba(6,182,212,0.4);
  background: rgba(6,182,212,0.04);
}

/* ── Attach bar ── */
.attach-bar { display: flex; flex-wrap: wrap; gap: 6px; }
.file-card {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; border-radius: 10px;
  background: var(--surface-card); border: 1px solid var(--border-default);
  color: var(--text-primary); max-width: 260px;
  transition: all 0.15s; cursor: default; position: relative;
}
.file-card:hover { border-color: var(--brand-300); box-shadow: 0 2px 8px rgba(6,182,212,0.06); }
.file-card.uploading { opacity: 0.7; }
.file-card-icon { width: 28px; height: 32px; flex-shrink: 0; }
.file-card-info { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
.file-card-name { font-size: 12px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-card-meta { font-size: 10px; color: var(--text-muted); }
.file-card-progress { font-size: 10px; color: var(--brand-500); font-weight: 600; }
.file-card-close { width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; border-radius: 4px; cursor: pointer; color: var(--text-muted); flex-shrink: 0; }
.file-card-close:hover { background: var(--surface-hover); color: var(--color-danger); }
.hint-chip { padding: 4px 11px; border-radius: 100px; font-size: 11px; font-family: inherit; border: 1px solid var(--border-default); background: transparent; color: var(--text-muted); cursor: pointer; transition: all 0.12s; }
.hint-chip:hover { border-color: var(--brand-400); color: var(--brand-500); background: var(--brand-50); }
[data-theme="dark"] .hint-chip:hover { background: rgba(6,182,212,0.08); color: #22d3ee; }

/* ── Command palette ── */
.cmd-palette {
  position: absolute; bottom: 100%; left: 0; right: 0;
  margin-bottom: 4px; max-height: 200px; overflow-y: auto;
  background: var(--surface-raised); border: 1px solid var(--border-default);
  border-radius: 10px; box-shadow: 0 4px 20px rgba(0,0,0,0.12);
  z-index: 50; padding: 4px;
}
.cmd-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; border-radius: 6px; cursor: pointer;
}
.cmd-item.active { background: var(--brand-50); }
.cmd-name { font-size: 12px; font-weight: 600; color: var(--brand-500); }
.cmd-args { font-size: 11px; color: var(--text-muted); }
.cmd-desc { font-size: 11px; color: var(--text-muted); flex: 1; text-align: right; }
.cmd-empty { font-size: 11px; color: var(--text-muted); padding: 8px; text-align: center; }

/* ── Mobile ── */
@media (max-width: 767px) {
  .input-bar-floating {
    max-width: calc(100% - 12px); width: calc(100% - 12px);
    border-radius: 14px; padding: 8px 10px 6px; bottom: 16px;
  }
  .input-bar-bottom { padding: 6px 8px 8px; }
  .input-field { font-size: 14px; padding: 8px 10px; min-height: 36px; }
  .seg-item { padding: 3px 5px; font-size: 10px; }
  .seg-item .seg-dot { display: none; }
  .reasoning-seg .seg-item { padding: 3px 5px; font-size: 10px; }
  .act-btn { width: 28px; height: 28px; }
  .send-btn { width: 32px; height: 32px; }
}
</style>
