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
  <!-- User message -->
  <div v-if="msg.role === 'user'" class="msg-row user">
    <div class="user-msg-wrap">
      <!-- Attachments as separate bubbles -->
      <div v-if="attachments.length" class="user-attachments">
        <div v-for="(att, i) in attachments" :key="i"
          class="file-card-msg"
          @click="previewAttachment(att)"
        >
          <div class="file-card-icon">
            <svg width="24" height="28" viewBox="0 0 24 28" fill="none">
              <path d="M16.5 0l7 7v15.6c0 2.25 0 3.375-.573 4.164a3 3 0 0 1-.663.663C21.475 28 20.349 28 18.1 28H5.9c-2.25 0-3.375 0-4.164-.573a3 3 0 0 1-.663-.663C.5 25.975.5 24.849.5 22.6V5.4c0-2.25 0-3.375.573-4.164a3 3 0 0 1 .663-.663C2.525 0 3.651 0 5.9 0h10.6z" :fill="fileIconGradientUrl(att.name, i)"/><path d="M16.5 0l7 7h-3.8c-1.12 0-1.68 0-2.108-.218a2 2 0 0 1-.874-.874C16.5 5.48 16.5 4.92 16.5 3.8V0z" fill="#fff" fill-opacity=".55"/><path d="M6 11.784c0-.433.351-.784.784-.784h10.432a.784.784 0 1 1 0 1.568H6.784A.784.784 0 0 1 6 11.784zM6 15.784c0-.433.351-.784.784-.784h10.432a.784.784 0 1 1 0 1.568H6.784A.784.784 0 0 1 6 15.784zM6.114 19.817c0-.433.35-.784.784-.784h6.318a.784.784 0 1 1 0 1.568H6.898a.784.784 0 0 1-.784-.784z" fill="#fff"/>
            </svg>
          </div>
          <div class="file-card-info">
            <span class="file-card-name">{{ att.name }}</span>
            <span class="file-card-meta">{{ fileExt(att.name).toUpperCase() }} {{ fmtFileSize(att.path) }}</span>
          </div>
        </div>
      </div>
      <!-- Text content (without attachment markers) -->
      <div v-if="cleanContent" class="user-bubble">
        <ContentBlock :content="cleanContent" />
      </div>
    </div>
    <div class="avatar user-avatar">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="5.5" r="2.5" stroke="currentColor" stroke-width="1.2" fill="none"/><path d="M3.5 13.5c0-2.5 2-4.5 4.5-4.5s4.5 2 4.5 4.5" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/></svg>
    </div>
  </div>

  <!-- Assistant message -->
  <div v-else-if="msg.role === 'assistant'" class="msg-row assistant">
    <div class="avatar ai-avatar">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg>
    </div>
    <div class="ai-blocks">
      <span class="role-tag">云兮之家</span>
      <span v-if="!msg.streaming && msg.durationMs >= 1000" class="msg-duration">{{ fmtDur(msg.durationMs) }}</span>

      <!-- Blocks preserving temporal reply chain order -->
      <template v-for="(block, bi) in msg.blocks" :key="'blk'+bi">
        <ThinkingBlock v-if="block.type === 'thinking'" :reasoning="block.content" :streaming="msg.streaming" />
        <ContentBlock v-if="block.type === 'content'" :content="block.content" :streaming="msg.streaming" />
        <ToolCallBlock v-if="block.type === 'tool'" :name="block.name" :args="block.args" :result="block.result" :status="block.status" :progress="block.progress" :streaming="msg.streaming" />
      </template>

      <!-- Streaming loading dots (before any blocks arrive) -->
      <div v-if="msg.streaming && !msg.blocks?.length" class="ai-empty">
        <span class="dot"></span><span class="dot"></span><span class="dot"></span>
      </div>
      <!-- Error fallback -->
      <div v-else-if="msg.status === 'error' && !msg.blocks?.length" class="ai-empty error">
        <span class="error-text">请求失败</span>
      </div>
    </div>
  </div>

  <!-- Agent message -->
  <div v-else-if="msg.role === 'agent'" class="msg-row agent">
    <div class="avatar ai-avatar">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg>
    </div>
    <AgentBubble
      :agent-id="msg.agentId"
      :goal="msg.agentGoal"
      :status="msg.agentStatus"
      :round="msg.agentRound"
      :summary="msg.agentSummary"
    />
  </div>
  <!-- Safety fallback: unknown role not rendered -->
  <div v-else style="display:none"></div>
</template>

<script setup>
import { computed } from 'vue'
import ContentBlock from './ContentBlock.vue'
import ThinkingBlock from './ThinkingBlock.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import AgentBubble from './AgentBubble.vue'

const props = defineProps({
  msg: { type: Object, required: true },
  showAvatar: { type: Boolean, default: true }
})

// Parse [文件: name (path)] patterns from user content
const attachmentRe = /\[文件:\s*([^\]]+?)\s*\(([^)]+)\)\]/g
const attachments = computed(() => {
  const content = props.msg.content || ''
  const matches = []
  let m
  while ((m = attachmentRe.exec(content)) !== null) {
    matches.push({ name: m[1].trim(), path: m[2].trim() })
  }
  return matches
})
const cleanContent = computed(() => {
  return (props.msg.content || '')
    .replace(attachmentRe, '')
    // Also strip bare [文件: name] without path
    .replace(/\[文件:\s*[^\]]+\]/g, '')
    .trim()
})

function fileExt(name) { const i = name.lastIndexOf('.'); return i >= 0 ? name.slice(i+1) : '' }
function fileIconGradientUrl(name, idx) {
  const ext = fileExt(name).toLowerCase()
  const map = { md:'fg-md', docx:'fg-doc', doc:'fg-doc', pptx:'fg-ppt', ppt:'fg-ppt', xlsx:'fg-xls', xls:'fg-xls', pdf:'fg-pdf', txt:'fg-txt', png:'fg-img', jpg:'fg-img', jpeg:'fg-img', gif:'fg-img', webp:'fg-img', zip:'fg-zip', rar:'fg-zip', '7z':'fg-zip' }
  return `url(#${map[ext] || 'fg-default-' + (idx % 4)})`
}
function fmtFileSize(path) {
  // Try to get size from available info; fallback to type display
  return ''
}
function isImage(name) { return /\.(png|jpg|jpeg|gif|webp|svg|bmp)$/i.test(name) }
function isDoc(name) { return /\.(doc|docx|pdf|txt|md)$/i.test(name) }

function previewAttachment(att) {
  const url = '/api/nas/files/download?path=' + encodeURIComponent(att.path)
  // Images: open in new tab for preview. Others: download
  if (isImage(att.name)) {
    window.open(url, '_blank')
  } else {
    const a = document.createElement('a')
    a.href = url; a.download = att.name; a.click()
  }
}

function fmtDur(ms) {
  if (ms < 1000) return (ms / 1000).toFixed(1) + 's'
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's'
  const m = Math.floor(ms / 60000)
  const s = ((ms % 60000) / 1000).toFixed(1)
  return m + 'm' + s + 's'
}
</script>

<style scoped>
.msg-row { display: flex; gap: 10px; align-items: flex-start; }
.msg-row.user { justify-content: flex-end; }

.avatar {
  width: 30px; height: 30px; min-width: 30px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: var(--surface-card); border: 1px solid var(--border-default);
}
.ai-avatar { color: var(--brand-500); }
.user-avatar { color: var(--text-muted); }

.user-msg-wrap {
  display: flex; flex-direction: column; align-items: flex-end; gap: 6px;
  max-width: 78%;
}
.user-attachments {
  display: flex; flex-direction: column; gap: 6px; width: 100%;
}
.file-card-msg {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 14px; border-radius: 12px;
  background: var(--surface-card); border: 1px solid var(--border-default);
  color: var(--text-primary); transition: all 0.15s; cursor: pointer;
  max-width: 300px;
}
.file-card-msg:hover {
  border-color: var(--brand-300); background: var(--brand-50);
  box-shadow: 0 2px 8px rgba(6,182,212,0.08);
}
.file-card-icon { flex-shrink: 0; width: 24px; height: 28px; }
.file-card-icon svg { width: 24px; height: 28px; display: block; }
.file-card-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.file-card-name { font-size: 13px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-primary); }
.file-card-meta { font-size: 10px; color: var(--text-muted); }
[data-theme="dark"] .file-card-msg { background: rgba(18,26,44,0.55); border-color: rgba(255,255,255,0.07); }
[data-theme="dark"] .file-card-msg:hover { border-color: rgba(34,211,238,0.25); background: rgba(6,182,212,0.06); }

.user-bubble {
  width: 100%; background: rgba(6,182,212,0.08);
  border: 1px solid rgba(6,182,212,0.18);
  color: var(--text-primary);
  padding: 10px 16px; border-radius: 16px;
  box-shadow: 0 2px 12px rgba(6,182,212,0.06); line-height: 1.5;
}
.user-bubble :deep(.content-body p) { margin: 0; color: var(--text-primary); }
.user-bubble :deep(.content-body a) { color: var(--brand-600); }

.ai-blocks {
  display: flex; flex-direction: column; gap: 4px;
  background: var(--surface-raised); border: 1px solid var(--border-default);
  border-radius: 14px; padding: 14px 16px;
}

.role-tag {
  display: inline-block; font-size: 10px; font-weight: 600;
  color: var(--brand-600); background: var(--brand-50);
  padding: 2px 8px; border-radius: 100px; text-transform: uppercase;
  letter-spacing: 0.5px; margin-bottom: 4px; user-select: none;
}
[data-theme="dark"] .role-tag { color: var(--brand-400); }

.msg-duration {
  display: inline-block; font-size: 10px; font-weight: 400;
  color: var(--text-muted); margin-left: 6px; user-select: none;
}

.ai-empty { display: flex; align-items: center; gap: 6px; padding: 4px 0; min-height: 20px; }
.ai-empty.error { color: var(--color-danger); font-size: 12px; }
.ai-empty .dot { width: 5px; height: 5px; border-radius: 50%; background: var(--brand-400); animation: dotBounce 1.2s ease-in-out infinite; }
.ai-empty .dot:nth-child(2) { animation-delay: 0.2s; }
.ai-empty .dot:nth-child(3) { animation-delay: 0.4s; }
@keyframes dotBounce { 0%,60%,100% { transform: translateY(0); opacity: 0.25; } 30% { transform: translateY(-5px); opacity: 1; } }
.muted-text { font-size: 12px; color: var(--text-muted); }
.error-text { font-size: 12px; color: var(--color-danger); }

@media (max-width: 767px) {
  .msg-row { padding: 0 4px; }
  .ai-blocks { max-width: calc(100% - 44px); }
  .user-msg-wrap { max-width: calc(100% - 44px); }
  .avatar { width: 24px; height: 24px; min-width: 24px; flex-shrink: 0; }
}


/* Extra polish */
.user-bubble {
  animation: userMsgIn 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes userMsgIn {
  from { opacity: 0; transform: translateX(12px); }
  to   { opacity: 1; transform: translateX(0); }
}

.ai-blocks {
  animation: aiMsgIn 0.28s cubic-bezier(0.16, 1, 0.3, 1);
}
@keyframes aiMsgIn {
  from { opacity: 0; transform: translateX(-8px); }
  to   { opacity: 1; transform: translateX(0); }
}

.role-tag {
  animation: tagFadeIn 0.3s var(--ease-out-expo);
}
@keyframes tagFadeIn {
  from { opacity: 0; transform: translateY(-4px); }
  to   { opacity: 1; transform: translateY(0); }
}

[data-theme="dark"] .ai-blocks {
  background: rgba(18,26,44,0.55); border-color: rgba(255,255,255,0.07);
}
.msg-row.agent { padding-left: 0; }
.msg-row.agent :deep(.agent-bubble) { max-width: 82%; }
[data-theme="dark"] .user-bubble {
  background: rgba(34,211,238,0.10); border-color: rgba(34,211,238,0.20);
  box-shadow: 0 2px 12px rgba(34,211,238,0.04);
}
[data-theme="dark"] .user-bubble :deep(.content-body a) { color: #22d3ee; }
[data-theme="dark"] .role-tag {
  color: #22d3ee; background: rgba(6,182,212,0.12);
}

</style>
