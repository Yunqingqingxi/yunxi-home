<template>
  <svg
    style="position:absolute;width:0;height:0"
    aria-hidden="true"
  >
    <defs>
      <linearGradient
        id="fg-md"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#6D93FF" /><stop
        offset="1"
        stop-color="#5A71F0"
      /></linearGradient>
      <linearGradient
        id="fg-doc"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#2B7FFF" /><stop
        offset="1"
        stop-color="#1A5CD0"
      /></linearGradient>
      <linearGradient
        id="fg-ppt"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#FF6B35" /><stop
        offset="1"
        stop-color="#D9441E"
      /></linearGradient>
      <linearGradient
        id="fg-xls"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#21A366" /><stop
        offset="1"
        stop-color="#147A48"
      /></linearGradient>
      <linearGradient
        id="fg-pdf"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#E53935" /><stop
        offset="1"
        stop-color="#B71C1C"
      /></linearGradient>
      <linearGradient
        id="fg-txt"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#78909C" /><stop
        offset="1"
        stop-color="#546E7A"
      /></linearGradient>
      <linearGradient
        id="fg-img"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#AB47BC" /><stop
        offset="1"
        stop-color="#8E24AA"
      /></linearGradient>
      <linearGradient
        id="fg-zip"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#FFA726" /><stop
        offset="1"
        stop-color="#F57C00"
      /></linearGradient>
      <linearGradient
        id="fg-default-0"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#6D93FF" /><stop
        offset="1"
        stop-color="#5A71F0"
      /></linearGradient>
      <linearGradient
        id="fg-default-1"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#26C6DA" /><stop
        offset="1"
        stop-color="#0097A7"
      /></linearGradient>
      <linearGradient
        id="fg-default-2"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#66BB6A" /><stop
        offset="1"
        stop-color="#388E3C"
      /></linearGradient>
      <linearGradient
        id="fg-default-3"
        x1="1.5"
        y1="-1"
        x2="23.5"
        y2="28"
        gradientUnits="userSpaceOnUse"
      ><stop stop-color="#FF7043" /><stop
        offset="1"
        stop-color="#E64A19"
      /></linearGradient>
    </defs>
  </svg>
  <!-- User message -->
  <div
    v-if="msg.role === 'user'"
    class="msg-row user"
  >
    <div class="user-msg-wrap">
      <!-- Attachments as rich file cards -->
      <div v-if="attachments.length" class="msg-attachments">
        <FileAttachment
          v-for="(att, i) in attachments"
          :key="i"
          :name="att.name"
          :path="att.path"
          :error="att._error"
          @preview="openLightbox(i)"
        />
      </div>
      <!-- Text content (without attachment markers) -->
      <div
        v-if="cleanContent"
        class="user-bubble"
      >
        <ContentBlock :content="cleanContent" />
      </div>
    </div>
    <div class="avatar user-avatar">
      <svg
        width="14"
        height="14"
        viewBox="0 0 16 16"
        fill="none"
      ><circle
        cx="8"
        cy="5.5"
        r="2.5"
        stroke="currentColor"
        stroke-width="1.2"
        fill="none"
      /><path
        d="M3.5 13.5c0-2.5 2-4.5 4.5-4.5s4.5 2 4.5 4.5"
        stroke="currentColor"
        stroke-width="1.2"
        fill="none"
        stroke-linecap="round"
      /></svg>
    </div>
  </div>

  <!-- AI消息：每个block独立气泡 -->
  <template v-if="msg.role === 'assistant' || msg.role === 'agent'">
    <div v-for="(block, bi) in (msg.blocks || [])" :key="'blk'+bi+'-'+block.type">
      <div v-if="block.type === 'thinking'" class="msg-row assistant thinking-row">
        <div class="avatar ai-avatar"><svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg></div>
        <div class="block-body thinking-body-plain"><span class="role-tag">云兮</span><ThinkingBlock :reasoning="block.content" :streaming="msg.streaming" /></div>
      </div>
      <div v-else-if="block.type === 'tool'" class="msg-row assistant">
        <div class="avatar ai-avatar"><svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg></div>
        <div class="block-body"><span class="role-tag">云兮</span><ToolCallBlock :name="block.name" :args="block.args" :result="block.result" :status="block.status" :progress="block.progress" :streaming="msg.streaming" /></div>
      </div>
      <template v-else-if="block.type === 'content'">
        <div v-for="(seg, si) in splitSegments(block.content || '')" :key="'seg'+si" class="msg-row assistant">
          <div class="avatar ai-avatar"><svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg></div>
          <div v-if="seg.type === 'text'" class="block-body content-body-bubble"><span class="role-tag">云兮</span><ContentBlock :content="seg.content" :streaming="false" /></div>
          <div v-else class="block-body"><FileAttachment :name="seg.name" :path="seg.path" @preview="openLightbox(findImageIdx(seg.path))" /></div>
        </div>
      </template>
    </div>
    <div v-if="!msg.blocks?.length && (msg.content || msg.streaming)" class="msg-row assistant">
      <div class="avatar ai-avatar"><svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg></div>
      <div :class="['block-body', (msg.streaming && !msg.content) ? '' : 'content-body-bubble']"><span class="role-tag">云兮</span><div v-if="msg.streaming && !msg.content" class="ai-empty"><span class="dot"/><span class="dot"/><span class="dot"/></div><ContentBlock v-else :content="msg.content" :streaming="false" /></div>
    </div>
    <div v-if="msg.status === 'error' && !msg.blocks?.length" class="msg-row assistant">
      <div class="avatar ai-avatar"><svg width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2" fill="none"/><circle cx="5.5" cy="7" r="1" fill="currentColor"/><circle cx="10.5" cy="7" r="1" fill="currentColor"/><path d="M5.5 10.5c0 0 1 2 2.5 2s2.5-2 2.5-2" stroke="currentColor" stroke-width="1" fill="none" stroke-linecap="round"/></svg></div>
      <div class="block-body"><span class="error-text">请求失败</span></div>
    </div>
  </template>

  <!-- Safety fallback: unknown role not rendered -->
  <div
    v-else
    style="display:none"
  ></div>

  <!-- 图片灯箱 -->
  <ImageLightbox
    :images="lightboxImages"
    :index="lightboxIdx"
    :visible="lightboxVisible"
    @close="lightboxVisible = false"
  />
</template>

<script setup lang="ts">
// @ts-nocheck
import { computed, ref } from 'vue'
import ContentBlock from './ContentBlock.vue'
import ThinkingBlock from './ThinkingBlock.vue'
import ToolCallBlock from './ToolCallBlock.vue'
import FileAttachment from './FileAttachment.vue'
import ImageLightbox from './ImageLightbox.vue'
import { formatDuration } from '../../composables/useFormat'


const props = defineProps({
  msg: { type: Object, required: true },
  showAvatar: { type: Boolean, default: true }
})

// Parse [文件: name (path)] patterns from user content
// Split content block by [文件: name (path)] markers into text/file segments
const fileSplitRe = /\[文件:\s*([^\]]+?)\s*\(([^)]+)\)\]/
type Segment = { type: 'text'; content: string } | { type: 'file'; name: string; path: string }

function splitSegments(content: string): Segment[] {
  const segments: Segment[] = []
  let remaining = content || ''
  while (remaining.length > 0) {
    const m = remaining.match(fileSplitRe)
    if (!m || m.index === undefined) {
      const trimmed = remaining.trim()
      if (trimmed) segments.push({ type: 'text', content: trimmed })
      break
    }
    // Text before the file marker
    if (m.index > 0) {
      const before = remaining.slice(0, m.index).trim()
      if (before) segments.push({ type: 'text', content: before })
    }
    // The file
    segments.push({ type: 'file', name: m[1].trim(), path: m[2].trim() })
    remaining = remaining.slice(m.index + m[0].length)
  }
  return segments
}

// Get segments from the main content block (NOT from individual blocks)
const contentSegments = computed(() => {
  // 流式传输期间不切分，避免抖动和重复渲染；完成后才切分
  if (props.msg.streaming) {
    const block = (props.msg.blocks || []).find((b: any) => b.type === 'content')
    return [{ type: 'text' as const, content: block?.content || props.msg.content || '' }]
  }
  const block = (props.msg.blocks || []).find((b: any) => b.type === 'content')
  if (block?.content) return splitSegments(block.content)
  return splitSegments(props.msg.content || '')
})

const attachments = computed(() => contentSegments.value.filter((s): s is { type: 'file'; name: string; path: string } => s.type === 'file'))
const fileMarkerReGlobal = /\[文件:\s*[^\]]+?\s*\([^)]*\)\]/g
const cleanContent = computed(() => {
  return (props.msg.content || '')
    .replace(fileMarkerReGlobal, '')
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
function isImage(name) { return /\.(png|jpg|jpeg|gif|webp|svg|bmp|ico)$/i.test(name) }

const lightboxImages = ref<{ name: string; path: string }[]>([])
const lightboxIdx = ref(0)
const lightboxVisible = ref(false)

function findImageIdx(path: string): number {
  // Find this path's index among image attachments for lightbox navigation
  const imgAtts = attachments.value.filter(a => isImage(a.name))
  return imgAtts.findIndex(a => a.path === path)
}

const allImageAtts = computed(() => attachments.value.filter(a => isImage(a.name)))

function openLightbox(imgIdx: number) {
  if (!allImageAtts.value.length) return
  lightboxImages.value = allImageAtts.value.map(a => ({ name: a.name, path: a.path }))
  lightboxIdx.value = imgIdx >= 0 ? imgIdx : 0
  lightboxVisible.value = true
}

function fmtDur(ms) {
  return formatDuration(ms)
}
</script>

<style scoped>
/* ── Layout ── */
.msg-row { display: flex; gap: 8px; align-items: flex-start; }
.msg-row.user { justify-content: flex-end; }
.avatar {
  width: 28px; height: 28px; min-width: 28px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: var(--surface-card, #fff); border: 1px solid var(--border-subtle, #e2e8f0);
}
.ai-avatar { color: var(--brand-500, #06b6d4); }
.user-avatar { color: var(--text-muted, #94a3b8); }

/* ── User message ── */
.user-msg-wrap { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; max-width: 78%; }
.user-attachments { display: flex; flex-direction: column; gap: 6px; width: 100%; }
.file-card-msg {
  display: flex; align-items: center; gap: 10px; padding: 10px 14px; border-radius: 8px;
  background: var(--surface-card, #fff); border: 1px solid var(--border-subtle, #e2e8f0);
  color: var(--text-primary); cursor: pointer; max-width: 300px;
}
.file-card-msg:hover { border-color: var(--brand-300, #67e8f9); }
.file-card-icon { flex-shrink: 0; width: 24px; height: 28px; }
.file-card-icon svg { width: 24px; height: 28px; display: block; }
.file-card-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.file-card-name { font-size: 13px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-primary); }
.file-card-meta { font-size: 10px; color: var(--text-muted); }

.file-standalone {
  flex: 1; min-width: 0;
}
.file-standalone .file-attach {
  max-width: 230px;
  margin-bottom: 8px;
}
/* 图片气泡更大 */
.file-standalone:has(.fa-image) .file-attach {
  max-width: 400px;
}
.file-standalone:has(.fa-image) .fa-thumb {
  width: 400px;
  height: 225px; /* 16:9 */
}
/* 行间距 */
.msg-row.assistant + .msg-row.assistant {
  margin-top: 6px;
}

.user-bubble {
  width: 100%; padding: 10px 14px; border-radius: 8px;
  background: var(--surface-card, #fff);
  border: 1px solid var(--border-subtle, #e2e8f0);
  color: var(--text-primary); line-height: 1.6;
}
.user-bubble :deep(.content-body p) { margin: 0; }

/* ── AI message ── */
.block-body {
  flex: 1; min-width: 0;
}
/* 思考块：无背景 */
.thinking-body-plain {
  opacity: 0.75; font-size: 13px;
}
.thinking-body-plain .thinking-body { background: transparent; border: none; padding: 0; }
/* 内容块：保持卡片背景 */
.content-body-bubble {
  border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px; padding: 12px 14px;
  background: var(--surface-card, #fff);
}
/* 思考行 */
.thinking-row { margin: 6px 0; }

.ai-blocks {
  display: flex; flex-direction: column; gap: 12px;
  border: 1px solid var(--border-subtle, #e2e8f0); border-radius: 8px; padding: 12px 14px;
  background: var(--surface-card, #fff);
}
/* Block type separators */
.ai-blocks :deep(.thinking-block + .tool-block),
.ai-blocks :deep(.content-block + .tool-block) {
  margin-top: 4px; padding-top: 8px;
  border-top: 1px solid var(--border-subtle, #e2e8f0);
}
.ai-blocks :deep(.tool-block + .content-block) {
  margin-top: 4px; padding-top: 8px;
  border-top: 1px solid var(--border-subtle, #e2e8f0);
}
.role-tag {
  display: inline-block; font-size: 11px; font-weight: 600;
  color: var(--brand-500); padding-bottom: 4px; user-select: none;
}
.msg-duration { display: inline-block; font-size: 10px; color: var(--text-muted); margin-left: 6px; user-select: none; }

/* ── Streaming dots ── */
.ai-empty { display: flex; align-items: center; gap: 6px; padding: 4px 0; min-height: 20px; }
.ai-empty.error { color: #ef4444; font-size: 12px; }
.ai-empty .dot { width: 5px; height: 5px; border-radius: 50%; background: var(--text-muted); animation: dotBounce 1.2s ease-in-out infinite; }
.ai-empty .dot:nth-child(2) { animation-delay: 0.2s; }
.ai-empty .dot:nth-child(3) { animation-delay: 0.4s; }
@keyframes dotBounce { 0%,60%,100% { transform: translateY(0); opacity: 0.25; } 30% { transform: translateY(-5px); opacity: 1; } }

/* ── Animations (minimal) ── */
.msg-row { animation: msgIn 0.2s ease-out; }
@keyframes msgIn { from { opacity: 0; } to { opacity: 1; } }

/* ── Dark mode ── */
@media (prefers-color-scheme: dark) {
  .ai-blocks { background: #1e293b; border-color: #334155; }
  .user-bubble { background: #1e293b; border-color: #334155; }
  .file-card-msg { background: #1e293b; border-color: #334155; }
  .file-card-msg:hover { border-color: #22d3ee; }
  .avatar { background: #1e293b; border-color: #334155; }
}

/* ── Mobile ── */
@media (max-width: 1023px) {
  .ai-blocks, .user-bubble { font-size: 15px; }
  .avatar { width: 24px; height: 24px; min-width: 24px; }
}
</style>
