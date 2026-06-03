<template>
  <div class="file-attach" :class="{ error: !!error, loading: loading }" @click="onClick">
    <!-- 图片缩略图 -->
    <div v-if="isImage(name)" class="fa-thumb fa-image">
      <img v-if="thumbSrc" :src="thumbSrc" :alt="name" @error="onThumbError" @load="loading = false" />
      <div v-if="loading && !thumbError" class="fa-loading">
        <span class="fa-spinner"></span>
      </div>
      <div v-if="thumbError" class="fa-icon-wrap">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>
      </div>
    </div>

    <!-- 视频：直接嵌入播放器 -->
    <div v-else-if="isVideo(name)" class="fa-video-player">
      <video
        v-if="videoOpen"
        ref="videoEl"
        controls
        autoplay
        muted
        playsinline
        preload="metadata"
        :src="streamUrl"
        class="fa-video-el"
        @error="thumbError = true"
        @loadedmetadata="loading = false"
      />
      <div v-else class="fa-thumb fa-video" @click.stop="videoOpen = true; loading = true">
        <svg class="fa-play-icon" width="32" height="32" viewBox="0 0 24 24" fill="rgba(255,255,255,0.9)" stroke="currentColor" stroke-width="1"><circle cx="12" cy="12" r="10" fill="rgba(0,0,0,0.4)"/><polygon points="10,8 16,12 10,16" fill="white"/></svg>
        <span v-if="thumbError" class="fa-error-tag">加载失败</span>
      </div>
    </div>

    <!-- 文档图标 -->
    <div v-else class="fa-thumb fa-doc">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="url(#fg-default-0)" stroke="none"><rect x="4" y="2" width="16" height="20" rx="2" fill="var(--fg-color, url(#fg-default-0))"/><text x="12" y="16" text-anchor="middle" fill="white" font-size="7" font-weight="700">{{ ext.toUpperCase() }}</text></svg>
    </div>

    <!-- 文件信息 -->
    <div class="fa-info">
      <span class="fa-name">{{ name }}</span>
      <span v-if="size" class="fa-size">{{ size }}</span>
      <span v-if="error" class="fa-error-text">{{ error }}</span>
    </div>

    <!-- 下载/预览按钮 -->
    <div class="fa-actions">
      <button v-if="isImage(name)" class="fa-btn" title="放大查看" @click.stop="$emit('preview')">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="7" cy="7" r="4"/><line x1="10" y1="10" x2="15" y2="15"/></svg>
      </button>
      <a :href="downloadUrl" class="fa-btn" title="下载" @click.stop>
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v10"/><polyline points="4,8 8,12 12,8"/><line x1="3" y1="14" x2="13" y2="14"/></svg>
      </a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  name: string
  path: string
  size?: string
  error?: string
}>()

const emit = defineEmits<{ preview: [] }>()

const loading = ref(true)
const thumbError = ref(false)
const videoOpen = ref(false)

const ext = computed(() => {
  const i = props.name.lastIndexOf('.')
  return i >= 0 ? props.name.slice(i + 1) : ''
})

// 图片/视频通过 <img>/<video> 加载时无法带 Authorization header，需 query 参数传递 token
const authToken = localStorage.getItem('token') || ''

const downloadUrl = computed(() => '/api/nas/files/download?path=' + encodeURIComponent(props.path) + '&token=' + authToken)

const streamUrl = computed(() => '/api/nas/files/stream?path=' + encodeURIComponent(props.path) + '&token=' + authToken)

const thumbSrc = computed(() => {
  if (!isImage(props.name)) return ''
  return streamUrl.value
})

const isImage = (n: string) => /\.(png|jpg|jpeg|gif|webp|svg|bmp|ico)$/i.test(n)
const isVideo = (n: string) => /\.(mp4|webm|mov|avi|mkv|flv|wmv|m4v)$/i.test(n)

function onThumbError() {
  thumbError.value = true
  loading.value = false
}

function clickLink(url: string, download?: string) {
  const a = document.createElement('a')
  a.href = url
  a.style.display = 'none'
  if (download) a.download = download
  else { a.target = '_blank'; a.rel = 'noopener' }
  document.body.appendChild(a)
  a.click()
  setTimeout(() => document.body.removeChild(a), 100)
}

function onClick() {
  if (isImage(props.name)) {
    emit('preview')
  } else if (isVideo(props.name)) {
    // 视频点击直接在内联播放器中播放
    videoOpen.value = true
    loading.value = true
  } else {
    const previewExts = ['md','txt','json','xml','csv','log','yaml','yml','html','htm','js','ts','py','go','java','rs','rb','php','c','cpp','h','css','scss','sh','bat','ps1','sql','toml','ini','cfg','conf','env','vue','svelte','proto','dockerfile','makefile']
    if (previewExts.includes(ext.value.toLowerCase())) {
      clickLink('/api/nas/files/stream?path=' + encodeURIComponent(props.path) + '&token=' + authToken + '&inline=1')
    } else {
      clickLink(downloadUrl.value, props.name)
    }
  }
}
</script>

<style scoped>
.file-attach {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  background: var(--surface-card);
  cursor: pointer;
  transition: all 0.15s;
  max-width: 360px;
}
.file-attach:hover { background: var(--surface-hover); border-color: var(--border-strong); }
.file-attach.error { border-color: rgba(239,68,68,0.3); background: rgba(239,68,68,0.04); }
.file-attach.loading { opacity: 0.7; }

.fa-thumb {
  width: 56px; height: 56px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
}
.fa-image { background: #f1f5f9; }
.fa-image img { width: 100%; height: 100%; object-fit: cover; }
.fa-video { background: #1e293b; position: relative; }
.fa-play-icon { position: absolute; z-index: 1; }
.fa-video-player { width: 100%; max-width: 400px; border-radius: 6px; overflow: hidden; }
.fa-video-el { width: 100%; max-height: 300px; display: block; background: #000; }
.fa-error-tag { position: absolute; bottom: 4px; right: 4px; font-size: 9px; color: #ef4444; background: rgba(0,0,0,0.6); padding: 1px 4px; border-radius: 3px; }
.fa-doc {
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  position: relative;
}
.fa-icon-wrap {
  display: flex; align-items: center; justify-content: center;
  color: #94a3b8;
}
.fa-loading {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
  background: #f1f5f9;
}
.fa-spinner {
  width: 16px; height: 16px;
  border: 2px solid #e2e8f0;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.fa-info {
  flex: 1; min-width: 0;
  display: flex; flex-direction: column; gap: 2px;
}
.fa-name {
  font-size: 12.5px; font-weight: 500; color: var(--text-primary);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.fa-size {
  font-size: 10.5px; color: var(--text-muted);
}
.fa-error-text {
  font-size: 10.5px; color: #ef4444;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

.fa-actions { display: flex; gap: 4px; flex-shrink: 0; }
.fa-btn {
  width: 28px; height: 28px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 6px; border: none; background: transparent;
  color: var(--text-muted); cursor: pointer; transition: all 0.15s;
}
.fa-btn:hover { background: var(--surface-hover); color: var(--text-primary); }

@media (prefers-color-scheme: dark) {
  .file-attach { border-color: #334155; background: #1e293b; }
  .file-attach:hover { background: #1e293b; border-color: #475569; }
  .file-attach.error { background: rgba(239,68,68,0.06); }
  .fa-image { background: #0f172a; }
  .fa-doc { background: linear-gradient(135deg, #1e293b, #0f172a); }
  .fa-loading { background: #0f172a; }
}
</style>
