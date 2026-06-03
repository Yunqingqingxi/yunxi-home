<template>
  <teleport to="body">
    <div v-if="props.visible" class="lightbox-overlay" @click.self="close">
      <button class="lb-close" @click="close" title="关闭">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
      <button v-if="images.length > 1" class="lb-nav lb-prev" @click="prev">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15,18 9,12 15,6"/></svg>
      </button>
      <div class="lb-content">
        <img :src="currentSrc" :alt="currentName" @error="onError" />
        <div v-if="imgError" class="lb-error">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
          <span>图片加载失败</span>
        </div>
        <div class="lb-info">
          <span class="lb-name">{{ currentName }}</span>
          <span v-if="images.length > 1" class="lb-idx">{{ idx + 1 }} / {{ images.length }}</span>
        </div>
      </div>
      <button v-if="images.length > 1" class="lb-nav lb-next" @click="next">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9,6 15,12 9,18"/></svg>
      </button>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

interface ImageItem { name: string; path: string }

const props = defineProps<{ images: ImageItem[]; index: number; visible: boolean }>()
const emit = defineEmits<{ close: [] }>()

const idx = ref(props.index)
const imgError = ref(false)

// Sync idx when props.index changes
import { watchEffect } from 'vue'
watchEffect(() => { if (props.visible) idx.value = props.index })

const current = computed(() => props.images[idx.value] || props.images[0])
const currentName = computed(() => current.value?.name || '')
const currentSrc = computed(() => {
  if (!current.value) return ''
  const token = localStorage.getItem('token') || ''
  return '/api/nas/files/stream?path=' + encodeURIComponent(current.value.path) + '&token=' + token
})
const images = computed(() => props.images)

function close() {
  document.body.style.overflow = ''
  emit('close')
}
function prev() {
  idx.value = idx.value > 0 ? idx.value - 1 : props.images.length - 1
  imgError.value = false
}
function next() {
  idx.value = idx.value < props.images.length - 1 ? idx.value + 1 : 0
  imgError.value = false
}
function onError() { imgError.value = true }

// Keyboard nav
watch(() => props.visible, (v) => {
  if (v) {
    document.body.style.overflow = 'hidden'
    const h = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
      if (e.key === 'ArrowLeft') prev()
      if (e.key === 'ArrowRight') next()
    }
    document.addEventListener('keydown', h)
    return () => {
      document.removeEventListener('keydown', h)
      document.body.style.overflow = ''
    }
  }
})

defineExpose({ open })
</script>

<style scoped>
.lightbox-overlay {
  position: fixed; inset: 0; z-index: 10000;
  background: rgba(0,0,0,0.92);
  display: flex; align-items: center; justify-content: center;
  animation: lbFadeIn 0.2s ease;
}
@keyframes lbFadeIn { from { opacity: 0; } to { opacity: 1; } }

.lb-close {
  position: absolute; top: 16px; right: 16px; z-index: 10;
  width: 40px; height: 40px; border-radius: 50%; border: none;
  background: rgba(255,255,255,0.1); color: #fff; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
}
.lb-close:hover { background: rgba(255,255,255,0.2); }

.lb-nav {
  position: absolute; top: 50%; transform: translateY(-50%); z-index: 10;
  width: 48px; height: 48px; border-radius: 50%; border: none;
  background: rgba(255,255,255,0.1); color: #fff; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: background 0.15s;
}
.lb-nav:hover { background: rgba(255,255,255,0.25); }
.lb-prev { left: 16px; }
.lb-next { right: 16px; }

.lb-content {
  max-width: 90vw; max-height: 90vh;
  display: flex; flex-direction: column; align-items: center; gap: 12px;
}
.lb-content img {
  max-width: 90vw; max-height: 80vh; object-fit: contain;
  border-radius: 8px; box-shadow: 0 4px 30px rgba(0,0,0,0.5);
}
.lb-error {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  color: #94a3b8; padding: 40px;
}
.lb-info {
  display: flex; align-items: center; gap: 12px;
  color: rgba(255,255,255,0.7); font-size: 13px;
}
.lb-name { font-weight: 500; }
.lb-idx { color: rgba(255,255,255,0.4); }
</style>
