<template>
  <div v-if="visible" class="ctx-menu-backdrop" @click.self="close" @contextmenu.prevent="close">
    <div class="ctx-menu" :style="{ left: x + 'px', top: y + 'px' }" ref="menuEl">
      <div v-for="item in items" :key="item.key" class="ctx-item"
        :class="{ disabled: item.disabled, danger: item.danger, divider: item.divider }"
        @click="item.disabled ? null : onClick(item)">
        <span class="ctx-icon" v-if="item.icon" v-html="item.icon"></span>
        <span class="ctx-label">{{ item.label }}</span>
        <span class="ctx-shortcut" v-if="item.shortcut">{{ item.shortcut }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
  visible: Boolean,
  x: Number,
  y: Number,
  items: Array, // { key, label, icon, shortcut, danger, disabled, divider, action }
})

const emit = defineEmits(['close', 'action'])

const menuEl = ref(null)

function onClick(item) {
  emit('action', item.key)
  emit('close')
}

function close() {
  emit('close')
}

// 确保菜单不超出屏幕
watch(() => props.visible, async (v) => {
  if (!v) return
  await nextTick()
  if (!menuEl.value) return
  const rect = menuEl.value.getBoundingClientRect()
  if (rect.right > window.innerWidth) {
    menuEl.value.style.left = (props.x - rect.width) + 'px'
  }
  if (rect.bottom > window.innerHeight) {
    menuEl.value.style.top = (props.y - rect.height) + 'px'
  }
})
</script>

<style scoped>
.ctx-menu-backdrop { position: fixed; inset: 0; z-index: 500; }
.ctx-menu {
  position: fixed;
  min-width: 180px;
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 32px rgba(0,0,0,0.18);
  padding: 4px;
  z-index: 501;
  animation: ctxFadeIn 0.12s var(--ease-out-expo);
}
@keyframes ctxFadeIn { from { opacity: 0; transform: scale(0.95); } to { opacity: 1; transform: scale(1); } }

.ctx-item {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 8px; border-radius: var(--radius-sm);
  font-size: var(--text-sm); color: var(--text-primary);
  cursor: pointer; transition: background 0.1s;
}
.ctx-item:hover { background: var(--surface-hover); }
.ctx-item.disabled { color: var(--text-muted); cursor: default; }
.ctx-item.disabled:hover { background: transparent; }
.ctx-item.danger { color: var(--color-danger, #ef4444); }
.ctx-item.danger:hover { background: rgba(239,68,68,0.08); }
.ctx-item.divider { border-bottom: 1px solid var(--border-hr); margin-bottom: 4px; padding-bottom: 8px; border-radius: 0; }

.ctx-icon { width: 16px; text-align: center; flex-shrink: 0; }
.ctx-icon :deep(svg) { width: 14px; height: 14px; display: block; }
.ctx-label { flex: 1; }
.ctx-shortcut { font-size: var(--text-xs); color: var(--text-muted); }
</style>
