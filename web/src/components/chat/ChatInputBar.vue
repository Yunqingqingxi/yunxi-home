<template>
  <!-- SVG defs 保留不变 -->
  <svg
    style="position:absolute;width:0;height:0"
    aria-hidden="true"
  >
    <defs>
      <linearGradient id="fg-md" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#6D93FF" /><stop offset="1" stop-color="#5A71F0" />
      </linearGradient>
      <linearGradient id="fg-doc" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#2B7FFF" /><stop offset="1" stop-color="#1A5CD0" />
      </linearGradient>
      <linearGradient id="fg-ppt" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#FF6B35" /><stop offset="1" stop-color="#D9441E" />
      </linearGradient>
      <linearGradient id="fg-xls" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#21A366" /><stop offset="1" stop-color="#147A48" />
      </linearGradient>
      <linearGradient id="fg-pdf" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#E53935" /><stop offset="1" stop-color="#B71C1C" />
      </linearGradient>
      <linearGradient id="fg-txt" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#78909C" /><stop offset="1" stop-color="#546E7A" />
      </linearGradient>
      <linearGradient id="fg-img" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#AB47BC" /><stop offset="1" stop-color="#8E24AA" />
      </linearGradient>
      <linearGradient id="fg-zip" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#FFA726" /><stop offset="1" stop-color="#F57C00" />
      </linearGradient>
      <linearGradient id="fg-default-0" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#6D93FF" /><stop offset="1" stop-color="#5A71F0" />
      </linearGradient>
      <linearGradient id="fg-default-1" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#26C6DA" /><stop offset="1" stop-color="#0097A7" />
      </linearGradient>
      <linearGradient id="fg-default-2" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#66BB6A" /><stop offset="1" stop-color="#388E3C" />
      </linearGradient>
      <linearGradient id="fg-default-3" x1="1.5" y1="-1" x2="23.5" y2="28" gradientUnits="userSpaceOnUse">
        <stop stop-color="#FF7043" /><stop offset="1" stop-color="#E64A19" />
      </linearGradient>
    </defs>
  </svg>

  <!-- 单一容器 -->
  <div
    :class="['input-bar', store.sessionId ? 'floating' : 'bottom', { 'drag-over': dragOver }]"
    @dragover.prevent="onDragOver"
    @dragleave.prevent="onDragLeave"
    @drop.prevent="onDrop"
  >
    <!-- 附件 chips -->
    <div v-if="attachedFiles.length" class="attach-chips">
      <span v-for="(f, i) in attachedFiles" :key="i" class="chip" :class="{ uploading: f.uploading }">
        <span class="chip-name" @click="previewFile = f" title="点击预览">{{ f.name }}</span>
        <span v-if="f.uploading" class="chip-pct">{{ f.progress }}%</span>
        <button class="chip-close" @click="removeFile(i)">×</button>
      </span>
    </div>

    <!-- Textarea 透明嵌入，自动撑高 -->
    <textarea
      ref="inputEl"
      v-model="input"
      class="input-field"
      :class="{ 'cmd-mode': showCommands }"
      :placeholder="inputPlaceholder"
      rows="1"
      maxlength="4000"
      @keydown="onKeydown"
      @input="onInputResize"
    />

    <!-- 底栏控件行 -->
    <div class="input-toolbar">
      <button class="tb-item model-btn" @click="modelMenuOpen = !modelMenuOpen">
        <span class="tb-dot" :class="modelKey"></span>
        <span>{{ currentModelLabel }}</span>
        <svg width="8" height="8" viewBox="0 0 8 8"><path d="M2 3l2 2 2-2" stroke="currentColor" stroke-width="1.2" fill="none"/></svg>
      </button>

      <div class="tb-spacer"></div>

      <button class="tb-item" @click="$refs.fileInput.click()" title="附加文件">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
          <path d="M10 4v7a3 3 0 01-6 0V5a2 2 0 014 0v5.5a1 1 0 01-2 0V5"/>
        </svg>
      </button>

      <button
        class="tb-send"
        :class="{
          stop: isBusy,
          active: !isBusy && (hasInput || attachedFiles.length)
        }"
        :disabled="!isBusy && !hasInput && !attachedFiles.length"
        :title="isBusy ? '停止' : '发送'"
        @click="handleSend"
      >
        <svg v-if="isBusy" width="12" height="12" viewBox="0 0 12 12" fill="currentColor">
          <rect x="2" y="2" width="8" height="8" rx="1.5"/>
        </svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
          <path d="M22 2L11 13"/><path d="M22 2L15 22l-4-9-9-4 20-7z"/>
        </svg>
      </button>
    </div>

    <!-- 模型下拉 -->
    <div v-if="modelMenuOpen" class="model-dropdown" @click.stop>
      <button
        v-for="m in modelOptions" :key="m.key"
        :class="['model-opt', { active: modelKey === m.key }]"
        @click="onModelChange(m.key); modelMenuOpen = false"
      >
        <span class="tb-dot" :class="m.key"></span>
        <div class="model-opt-info">
          <span class="model-opt-label">{{ m.label }}</span>
          <span class="model-opt-desc">{{ m.desc }}</span>
        </div>
      </button>
      <div class="model-dropdown-div"></div>
      <div class="model-reasoning-row">
        <span class="model-reasoning-label">推理</span>
        <button
          v-for="r in reasoningLevels" :key="r.key"
          :class="['seg-item', { active: reasoning === r.key }]"
          @click="onReasoningChange(r.key)"
        >
          {{ r.label }}
        </button>
      </div>
    </div>

    <!-- 命令面板 -->
    <div v-if="showCommands" class="cmd-palette">
      <div
        v-for="(cmd, i) in filteredCommands" :key="cmd.name"
        :class="['cmd-item', { active: i === cmdIndex }]"
        @mousedown.prevent
        @click="selectCommand(cmd)"
      >
        <span class="cmd-name">/{{ cmd.name }}</span>
        <span v-if="cmd.args" class="cmd-args">{{ cmd.args }}</span>
        <span class="cmd-desc">{{ cmd.desc }}</span>
      </div>
      <div v-if="!filteredCommands.length" class="cmd-empty">无匹配指令</div>
    </div>

    <!-- 拖拽上传遮罩 -->
    <div v-if="dragOver" class="drop-overlay">
      <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
        <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
      </svg>
      <span>释放以上传文件</span>
    </div>

    <input ref="fileInput" type="file" hidden multiple @change="onFileAttach" />

    <CronPanel v-if="store.sessionId" />

    <!-- 文件预览弹窗 -->
    <Teleport to="body">
      <div v-if="previewFile" class="preview-overlay" @click.self="previewFile = null" @keydown.escape="previewFile = null">
        <div class="preview-modal">
          <div class="preview-head">
            <span class="preview-title">{{ previewFile.name }}</span>
            <button class="preview-close" @click="previewFile = null">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M3 3l10 10M13 3L3 13"/></svg>
            </button>
          </div>
          <div class="preview-body">
            <!-- Image preview -->
            <img v-if="isPreviewImage" :src="previewSrc" class="preview-img" :alt="previewFile.name" />
            <!-- Text / Markdown preview -->
            <div v-else-if="isPreviewText" class="preview-text-wrap">
              <div v-if="isPreviewMd" class="preview-md" v-html="previewHtml"></div>
              <pre v-else class="preview-text">{{ previewText }}</pre>
            </div>
            <!-- Unsupported binary format -->
            <div v-else class="preview-file-info">
              <div class="preview-file-icon">
                <svg width="48" height="56" viewBox="0 0 24 28" fill="none">
                  <path d="M16.5 0l7 7v15.6c0 2.25 0 3.375-.573 4.164a3 3 0 0 1-.663.663C21.475 28 20.349 28 18.1 28H5.9c-2.25 0-3.375 0-4.164-.573a3 3 0 0 1-.663-.663C.5 25.975.5 24.849.5 22.6V5.4c0-2.25 0-3.375.573-4.164a3 3 0 0 1 .663-.663C2.525 0 3.651 0 5.9 0h10.6z" :fill="fileIconGradient(previewFile.name, 0)"/>
                  <path d="M16.5 0l7 7h-3.8c-1.12 0-1.68 0-2.108-.218a2 2 0 0 1-.874-.874C16.5 5.48 16.5 4.92 16.5 3.8V0z" fill="#fff" fill-opacity=".55"/>
                </svg>
              </div>
              <div class="preview-file-meta">
                <span class="preview-file-name">{{ previewFile.name }}</span>
                <span class="preview-file-size">{{ fileExt(previewFile.name).toUpperCase() }} · {{ fmtFileSize(previewFile.size) }}</span>
                <span class="preview-unsupported">此格式不支持预览</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useChatStore, renderMarkdown } from '../../stores/chat'
import { useSettingsStore } from '../../stores/settings'
import CronPanel from './CronPanel.vue'

const router = useRouter()
const store = useChatStore()

// ── 发送按钮状态计算 ──
const isBusy = computed(() => store.isStreaming || store.hasRunningAgents || !!store.currentToolName)
const hasAgent = computed(() => store.hasRunningAgents && !store.isStreaming)
const hasInput = computed(() => !!input.value.trim())

const input = ref('')
const inputEl = ref(null)
const fileInput = ref(null)

// ── File attach ──
const attachedFiles = ref([])

// ── Drag & drop ──
const dragOver = ref(false)
let dragCounter = 0
function onDragOver(e: DragEvent) {
  dragCounter++
  dragOver.value = true
}
function onDragLeave(e: DragEvent) {
  dragCounter--
  if (dragCounter <= 0) { dragCounter = 0; dragOver.value = false }
}
function onDrop(e: DragEvent) {
  dragCounter = 0
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files?.length) return
  addFiles(files)
}

// ── File preview ──
const previewFile = ref(null)
const previewSrc = ref('')
const previewText = ref('')
const previewHtml = ref('')
const imageExtensions = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico']
const textExtensions = ['txt', 'md', 'json', 'xml', 'csv', 'log', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf',
  'js', 'ts', 'jsx', 'tsx', 'py', 'go', 'java', 'rs', 'rb', 'php', 'c', 'cpp', 'h', 'hpp',
  'css', 'scss', 'less', 'html', 'vue', 'svelte', 'sh', 'bat', 'ps1', 'zsh', 'bash',
  'env', 'gitignore', 'dockerfile', 'makefile', 'sql', 'graphql', 'proto']
const isPreviewImage = computed(() => {
  if (!previewFile.value) return false
  return imageExtensions.includes(fileExt(previewFile.value.name).toLowerCase())
})
const isPreviewText = computed(() => {
  if (!previewFile.value) return false
  return textExtensions.includes(fileExt(previewFile.value.name).toLowerCase())
})
const isPreviewMd = computed(() => {
  if (!previewFile.value) return false
  return fileExt(previewFile.value.name).toLowerCase() === 'md'
})

// Read file content for preview
watch(previewFile, (f) => {
  // Cleanup previous
  if (previewSrc.value) { URL.revokeObjectURL(previewSrc.value); previewSrc.value = '' }
  previewText.value = ''
  previewHtml.value = ''
  if (!f || !f.file) return
  if (isPreviewImage.value) {
    previewSrc.value = URL.createObjectURL(f.file)
  } else if (isPreviewText.value) {
    const reader = new FileReader()
    reader.onload = () => {
      const text = reader.result as string
      // Truncate very large files
      previewText.value = text.length > 50000 ? text.slice(0, 50000) + '\n\n... (文件过大，仅显示前 50000 字符)' : text
      if (isPreviewMd.value) {
        previewHtml.value = renderMarkdown(previewText.value)
      }
    }
    reader.readAsText(f.file)
  }
})

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

// Persist model/reasoning choice across page refreshes
const STORAGE_KEY_MODEL = 'yunxi_chat_model'
const STORAGE_KEY_REASONING = 'yunxi_chat_reasoning'

function getStoredOrDefault(key, defaultVal) {
  try { return localStorage.getItem(key) || defaultVal } catch { return defaultVal }
}
function setStored(key, val) {
  try { localStorage.setItem(key, val) } catch { /* ignore */ }
}

const modelKey = ref(findModelKey(getStoredOrDefault(STORAGE_KEY_MODEL, getDefaultModel())))
const modelMenuOpen = ref(false)
const reasoning = ref(getStoredOrDefault(STORAGE_KEY_REASONING, getDefaultReasoning()))

const currentModelLabel = computed(() => {
  const m = allModelDefs.find(o => o.key === modelKey.value)
  return m ? m.label : 'Flash'
})
const reasoningLevels = [
  { key: 'low',    label: '快速' },
  { key: 'medium', label: '均衡' },
  { key: 'high',   label: '深度' },
]

function onReasoningChange(key) {
  reasoning.value = key
  setStored(STORAGE_KEY_REASONING, key)
}

function onModelChange(key) {
  modelKey.value = key
  setStored(STORAGE_KEY_MODEL, key)
  const m = allModelDefs.find(o => o.key === key)
  if (m) { reasoning.value = m.reasoning; setStored(STORAGE_KEY_REASONING, m.reasoning) }
}

const currentModelName = computed(() => {
  const m = modelOptions.value.find(o => o.key === modelKey.value)
  return m ? m.model : 'deepseek-v4-flash'
})

// ── Commands ──
const cmdIndex = ref(0)
const commands = ref([])
const commandsLoaded = ref(false)

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

const showCommands = computed(() => input.value.startsWith('/') && !input.value.includes(' '))
const filteredCommands = computed(() => {
  const q = input.value.slice(1).toLowerCase()
  if (!q) return commands.value
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
  const h = Math.min(Math.max(el.scrollHeight, 40), 320)
  el.style.height = h + 'px'
  el.style.overflowY = el.scrollHeight > 320 ? 'auto' : 'hidden'
}

// ── Key handler ──
function onKeydown(e) {
  if (showCommands.value) {
    if (e.key === 'ArrowDown') { e.preventDefault(); cmdIndex.value = Math.min(cmdIndex.value + 1, filteredCommands.value.length - 1); return }
    if (e.key === 'ArrowUp') { e.preventDefault(); cmdIndex.value = Math.max(cmdIndex.value - 1, 0); return }
    if (e.key === 'Tab' || e.key === 'Enter') { e.preventDefault(); const cmd = filteredCommands.value[cmdIndex.value]; if (cmd) selectCommand(cmd); return }
    if (e.key === 'Escape') { input.value = ''; cmdIndex.value = 0; return }
  }
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend() }
}

// ── Unified send/stop handler ──
function handleSend() {
  if (isBusy.value) {
    stopGeneration()
  } else {
    doSend()
  }
}

async function stopGeneration() {
  const sid = store.sessionId
  if (!sid) { store.resetStreaming(); return }
  const token = localStorage.getItem('token')
  try {
    await fetch('/api/chat/sessions/' + sid + '/interrupt', {
      method: 'POST',
      headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' },
      body: JSON.stringify({ mode: 'soft' }),
    })
  } catch (_) {}
  store.disconnectStream()
  store.resetStreaming()
}

// ── Send ──
async function doSend() {
  const t = input.value.trim()
  const files = attachedFiles.value.filter(f => f.file)
  if (!t && !files.length) return

  if (t.startsWith('/') && !files.length) {
    store.sendMessage(t, currentModelName.value, {
      reasoning_intensity: reasoning.value,
      plan_mode: false
    })
    input.value = ''
    inputEl.value && (inputEl.value.style.height = 'auto', inputEl.value.style.overflowY = 'hidden')
    return
  }

  let msg = t
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
  if (uploadedPaths.length) {
    const fileRefs = uploadedPaths.map(p => '[文件: ' + p.split('/').pop() + ' (' + p + ')]').join('\n')
    msg = t ? t + '\n\n' + fileRefs : fileRefs
  }

  input.value = ''
  inputEl.value && (inputEl.value.style.height = 'auto', inputEl.value.style.overflowY = 'hidden')
  attachedFiles.value = []
  store.sendMessage(msg, currentModelName.value, {
    reasoning_intensity: reasoning.value,
    plan_mode: false
  })
}

function removeFile(i) { attachedFiles.value.splice(i, 1) }
function addFiles(files: FileList) {
  for (const f of files) {
    attachedFiles.value.push({ name: f.name, size: f.size, file: f, uploaded: false, progress: 0 })
  }
}
function onFileAttach(e) {
  const files = e.target.files
  if (!files?.length) return
  addFiles(files)
  e.target.value = ''
}

function onDocClick(e) {
  if (modelMenuOpen.value && !e.target.closest('.input-bar')) modelMenuOpen.value = false
}
onMounted(() => {
  nextTick(() => inputEl.value?.focus())
  document.addEventListener('click', onDocClick)
})
</script>

<style scoped>
/* ── 单一容器 ── */
.input-bar {
  position: relative;
  border: 1px solid var(--border-default);
  border-radius: 12px;
  padding: 10px 14px 8px;
  display: flex; flex-direction: column; gap: 6px;
  transition: box-shadow 0.2s, border-color 0.2s;
}
.input-bar.floating {
  flex-shrink: 0; padding: 10px 14px;
  border: 1px solid var(--border-subtle, #e2e8f0); border-bottom: none;
  background: var(--surface-card, #fff); border-radius: 8px 8px 0 0;
  margin: 0 12px;
}
.input-bar.bottom {
  flex-shrink: 0; padding: 10px 14px;
  border: 1px solid var(--border-subtle, #e2e8f0); border-bottom: none;
  background: var(--surface-card, #fff); border-radius: 8px 8px 0 0;
  margin: 0 12px;
}

/* ── 拖拽上传 ── */
.input-bar.drag-over {
  border-color: var(--brand-400);
  box-shadow: 0 0 0 3px rgba(6,182,212,0.12), 0 4px 20px rgba(6,182,212,0.1);
}
.drop-overlay {
  position: absolute; inset: 0; z-index: 10;
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px;
  background: rgba(6,182,212,0.06);
  border-radius: inherit;
  color: var(--brand-500);
  font-size: 13px; font-weight: 500;
  pointer-events: none;
  animation: dropPulse 0.2s ease;
}
@keyframes dropPulse { from { opacity: 0; } to { opacity: 1; } }

/* ── Textarea 透明化，自动撑高 ── */
.input-field {
  width: 100%;
  border: none;
  background: transparent;
  outline: none;
  font-size: 14px;
  color: var(--text-primary);
  font-family: inherit;
  line-height: 1.6;
  resize: none;
  padding: 6px 0;
  min-height: 40px;
  max-height: 320px;
  overflow-y: auto;
  transition: height 0.1s ease;
  box-sizing: border-box;
}
.input-field:focus { min-height: 40px; }
.input-field::placeholder { color: var(--text-muted); }
.input-field.cmd-mode { color: var(--brand-500); }

/* ── 附件 Chips ── */
.attach-chips {
  display: flex; flex-wrap: wrap; gap: 6px;
  padding: 0 0 4px 0;
}
.chip {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  color: var(--text-secondary);
  border: 1px solid var(--border-subtle);
  background: transparent;
  max-width: 200px;
}
.chip.uploading { opacity: 0.6; }
.chip-name {
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  max-width: 120px; cursor: pointer;
}
.chip-name:hover { color: var(--brand-500); }
.chip-pct { font-size: 10px; color: var(--brand-500); flex-shrink: 0; }
.chip-close {
  width: 14px; height: 14px; border: none; background: none;
  color: var(--text-muted); cursor: pointer; font-size: 12px;
  padding: 0; flex-shrink: 0; display: flex; align-items: center; justify-content: center;
  border-radius: 3px;
}
.chip-close:hover { background: var(--surface-hover); color: var(--color-danger); }

/* ── 底栏控件行 ── */
.input-toolbar {
  display: flex; align-items: center; gap: 2px;
  height: 28px; padding: 0 2px;
}
.tb-item {
  display: flex; align-items: center; gap: 4px;
  height: 26px; padding: 0 8px;
  border-radius: 6px; border: none;
  background: transparent;
  color: var(--text-muted);
  font-size: 11px; font-family: inherit;
  cursor: pointer;
  transition: all 0.12s;
}
.tb-item:hover { background: var(--surface-hover); color: var(--text-primary); }
.tb-spacer { flex: 1; }

/* 模型圆点 */
.tb-dot {
  width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0;
}
.tb-dot.flash { background: #6D93FF; }
.tb-dot.pro   { background: #A78BFA; }
.tb-dot.qwen-plus { background: #34d399; }
.tb-dot.qwen-max  { background: #f472b6; }

/* 发送按钮 */
.tb-send {
  width: 28px; height: 28px; border-radius: 8px; border: none;
  background: #dcfce7; color: #16a34a; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s; flex-shrink: 0;
}
.tb-send:hover { background: #bbf7d0; }
.tb-send.active {
  background: #16a34a; color: #fff;
  box-shadow: 0 2px 8px rgba(22,163,74,0.25);
}
.tb-send.active:hover {
  background: #15803d;
  box-shadow: 0 4px 12px rgba(22,163,74,0.35);
}
.tb-send.stop { background: var(--color-danger); color: #fff; }
.tb-send:disabled { opacity: 0.5; cursor: default; }

/* ── 模型下拉 ── */
.model-dropdown {
  position: absolute; bottom: calc(100% + 6px); left: 0;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.08);
  padding: 4px; min-width: 170px; z-index: 60;
}
.model-opt {
  padding: 6px 8px; border-radius: 6px; font-size: 12px;
  background: transparent;
  border: none; cursor: pointer;
  display: flex; align-items: center; gap: 8px;
  color: var(--text-primary);
  font-family: inherit;
  width: 100%; text-align: left;
  transition: background 0.1s;
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
  font-size: 10px; font-family: inherit; transition: all 0.1s;
}
.model-reasoning-row .seg-item:hover { color: var(--text-primary); }
.model-reasoning-row .seg-item.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }

/* ── 命令面板 ── */
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
  .input-bar.floating {
    max-width: calc(100% - 12px); width: calc(100% - 12px);
    border-radius: 14px; padding: 8px 10px 6px; bottom: 16px;
  }
  .input-bar.bottom { padding: 10px 16px 12px; margin: 0 16px 0 0; border-radius: 14px 14px 0 0; }
  .input-field { font-size: 14px; padding: 4px 0; min-height: 36px; }
  .tb-item.model-btn { max-width: 80px; overflow: hidden; }
  .tb-item.model-btn span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}

/* ── 预览弹窗 (unscoped — teleported to body) ── */
</style>

<style>
.preview-overlay {
  position: fixed; inset: 0; z-index: 10000;
  background: rgba(0,0,0,0.45);
  display: flex; align-items: center; justify-content: center;
  animation: previewFadeIn 0.15s ease;
  padding: 40px;
}
.preview-modal {
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  border-radius: 14px;
  max-width: 640px; width: 100%;
  max-height: 80vh;
  display: flex; flex-direction: column;
  box-shadow: 0 16px 48px rgba(0,0,0,0.18);
  overflow: hidden;
  animation: previewSlideUp 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.preview-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-subtle);
}
.preview-title {
  font-size: 13px; font-weight: 600; color: var(--text-primary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex: 1; margin-right: 12px;
}
.preview-close {
  width: 30px; height: 30px; border-radius: 8px; border: none;
  background: transparent; color: var(--text-muted); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; transition: all 0.12s;
}
.preview-close:hover { background: var(--surface-hover); color: var(--text-primary); }
.preview-body {
  flex: 1; overflow: auto; display: flex; align-items: center; justify-content: center;
  padding: 24px; min-height: 200px;
}
.preview-img {
  max-width: 100%; max-height: 60vh;
  border-radius: 8px; object-fit: contain;
}
.preview-file-info {
  display: flex; flex-direction: column; align-items: center; gap: 16px;
  padding: 32px;
}
.preview-file-icon { width: 48px; height: 56px; }
.preview-file-meta {
  display: flex; flex-direction: column; align-items: center; gap: 4px;
}
.preview-file-name { font-size: 14px; font-weight: 500; color: var(--text-primary); }
.preview-file-size { font-size: 12px; color: var(--text-muted); }

@keyframes previewFadeIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes previewSlideUp { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: translateY(0); } }

@media (max-width: 767px) {
  .preview-overlay { padding: 16px; }
  .preview-modal { max-width: 100%; max-height: 90vh; }
}
</style>
