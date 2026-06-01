<template>
  <Transition name="confirm">
    <div v-if="visible" class="confirm-overlay" @click.self="onCancel" @keydown.esc="onCancel">
      <div class="confirm-card" role="dialog" aria-modal="true">
        <div class="confirm-icon" v-if="icon">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
            <circle cx="12" cy="12" r="10" v-if="icon === 'warn'"/>
            <path d="M12 8v4M12 16h.01" v-if="icon === 'warn'"/>
            <path d="M5 13l4 4L19 7" v-if="icon === 'success'"/>
          </svg>
        </div>
        <h3 class="confirm-title">{{ title }}</h3>
        <p class="confirm-msg" v-if="message">{{ message }}</p>
        <div class="confirm-actions">
          <button class="confirm-btn confirm-btn-cancel" @click="onCancel" ref="cancelBtn">
            {{ cancelText }}
          </button>
          <button class="confirm-btn" :class="'confirm-btn-' + variant" @click="onConfirm" ref="confirmBtn">
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
  visible: Boolean,
  title: { type: String, default: '确认操作' },
  message: { type: String, default: '' },
  confirmText: { type: String, default: '确定' },
  cancelText: { type: String, default: '取消' },
  variant: { type: String, default: 'primary' }, // 'primary' | 'danger'
  icon: { type: String, default: '' }, // 'warn' | 'success' | ''
})

const emit = defineEmits(['confirm', 'cancel'])
const confirmBtn = ref(null)

function onConfirm() { emit('confirm') }
function onCancel() { emit('cancel') }

watch(() => props.visible, async (v) => {
  if (v) {
    await nextTick()
    confirmBtn.value?.focus()
  }
})
</script>

<style scoped>
.confirm-overlay {
  position: fixed; inset: 0; z-index: 600;
  display: flex; align-items: center; justify-content: center;
  background: rgba(0,0,0,0.35); backdrop-filter: blur(6px);
}
.confirm-card {
  background: var(--surface-card); border: 1px solid var(--border-default);
  border-radius: var(--radius-lg); padding: var(--space-6);
  min-width: 320px; max-width: 420px;
  box-shadow: 0 16px 48px rgba(0,0,0,0.2);
  display: flex; flex-direction: column; align-items: center; gap: var(--space-3);
  text-align: center;
}
.confirm-icon { color: var(--brand-500); margin-bottom: 4px; }
.confirm-title { margin: 0; font-size: var(--text-lg); font-weight: var(--weight-semibold); color: var(--text-primary); }
.confirm-msg { margin: 0; font-size: var(--text-sm); color: var(--text-secondary); line-height: 1.5; }
.confirm-actions { display: flex; gap: var(--space-3); margin-top: var(--space-3); }
.confirm-btn {
  padding: 8px 24px; border-radius: var(--radius-md); border: 1px solid transparent;
  font-size: var(--text-sm); font-family: inherit; font-weight: var(--weight-medium);
  cursor: pointer; transition: all 0.15s; outline: none;
}
.confirm-btn-primary { background: var(--brand-500); color: #fff; border-color: var(--brand-500); }
.confirm-btn-primary:hover, .confirm-btn-primary:focus-visible { background: var(--brand-600); box-shadow: 0 0 0 3px rgba(6,182,212,0.25); }
.confirm-btn-danger { background: var(--color-danger, #ef4444); color: #fff; border-color: var(--color-danger, #ef4444); }
.confirm-btn-danger:hover, .confirm-btn-danger:focus-visible { background: #dc2626; box-shadow: 0 0 0 3px rgba(239,68,68,0.25); }
.confirm-btn-cancel { background: var(--surface-hover); color: var(--text-secondary); border-color: var(--border-default); }
.confirm-btn-cancel:hover, .confirm-btn-cancel:focus-visible { background: var(--border-hr); color: var(--text-primary); }

.confirm-enter-active { transition: all 0.2s var(--ease-out-expo); }
.confirm-leave-active { transition: all 0.15s var(--ease-in-expo); }
.confirm-enter-from, .confirm-leave-to { opacity: 0; }
.confirm-enter-from .confirm-card, .confirm-leave-to .confirm-card { transform: scale(0.9); }
.confirm-enter-to .confirm-card, .confirm-leave-from .confirm-card { transform: scale(1); }
</style>
