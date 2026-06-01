<template>
  <Transition name="modal">
    <div v-if="visible" class="interact-overlay" @click.self="cancel">
      <div class="interact-card" :class="'variant-' + (req.variant || 'info')">
        <div class="interact-head">
          <span class="interact-icon">
            <svg v-if="req.type === 'confirm' && req.variant === 'danger'" width="20" height="20" viewBox="0 0 20 20" fill="none"><path d="M10 2a8 8 0 100 16 8 8 0 000-16zM10 6v4M10 13v1" stroke="var(--color-danger)" stroke-width="1.5" stroke-linecap="round"/></svg>
            <svg v-else-if="req.type === 'form' || req.type === 'input'" width="20" height="20" viewBox="0 0 20 20" fill="none"><rect x="3" y="3" width="14" height="14" rx="2" stroke="var(--brand-500)" stroke-width="1.5"/><path d="M7 8h6M7 12h4" stroke="var(--brand-500)" stroke-width="1.5" stroke-linecap="round"/></svg>
            <svg v-else width="20" height="20" viewBox="0 0 20 20" fill="none"><circle cx="10" cy="10" r="8" stroke="var(--brand-500)" stroke-width="1.5"/><path d="M10 6v5M10 13.5v.5" stroke="var(--brand-500)" stroke-width="1.5" stroke-linecap="round"/></svg>
          </span>
          <span class="interact-title">{{ req.title || 'AI 请求' }}</span>
        </div>

        <div v-if="req.message" class="interact-msg">{{ req.message }}</div>

        <!-- confirm 模式 -->
        <div v-if="req.type === 'confirm'" class="interact-body"></div>

        <!-- input 模式 -->
        <div v-if="req.type === 'input'" class="interact-body">
          <div v-for="f in (req.fields || [])" :key="f.name" class="interact-field">
            <label>{{ f.label }} <span v-if="f.required" class="req">*</span></label>
            <input v-model="values[f.name]" :type="f.type || 'text'" :placeholder="f.placeholder" class="interact-input" />
          </div>
        </div>

        <!-- form 模式 -->
        <div v-if="req.type === 'form'" class="interact-body">
          <div v-for="f in (req.fields || [])" :key="f.name" class="interact-field">
            <label>{{ f.label }} <span v-if="f.required" class="req">*</span></label>
            <input v-model="values[f.name]" :type="f.type || 'text'" :placeholder="f.placeholder || f.label" class="interact-input" />
            <span class="interact-hint">{{ f.default ? '默认: ' + f.default : '' }}</span>
          </div>
        </div>

        <!-- select 模式 -->
        <div v-if="req.type === 'select'" class="interact-body">
          <button v-for="opt in (req.options || [])" :key="opt"
            :class="['interact-option', { active: selected === opt }]"
            @click="selected = opt">{{ opt }}</button>
        </div>

        <div class="interact-actions">
          <button class="interact-btn cancel" @click="cancel">
            {{ req.cancel_text || '取消' }}
          </button>
          <button class="interact-btn confirm" :class="variantClass" @click="submit" :disabled="submitting">
            {{ submitting ? '提交中...' : (req.confirm_text || getDefaultConfirm()) }}
          </button>
        </div>
        <div v-if="req.timeout_sec" class="interact-timeout">⏱ {{ timeoutLeft }}s 后自动取消</div>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, reactive, watch, onUnmounted, computed } from 'vue'

const props = defineProps({
  request: { type: Object, default: null },
  visible: { type: Boolean, default: false }
})
const emit = defineEmits(['respond', 'close'])

const req = ref({})
const values = reactive({})
const selected = ref('')
const submitting = ref(false)
const timeoutLeft = ref(120)
let timer = null

const variantClass = computed(() => {
  if (req.value.variant === 'danger') return 'danger'
  if (req.value.variant === 'warning') return 'warning'
  return 'primary'
})

function getDefaultConfirm() {
  if (req.value.type === 'input' || req.value.type === 'form') return '提交'
  if (req.value.type === 'select') return '选择'
  return '确认'
}

watch(() => props.request, (r) => {
  if (!r) return
  req.value = r
  // 重置
  Object.keys(values).forEach(k => delete values[k])
  selected.value = ''
  submitting.value = false
  timeoutLeft.value = r.timeout_sec || 120
  // 填默认值
  for (const f of (r.fields || [])) {
    if (f.default) values[f.name] = f.default
  }
  // 倒计时
  clearInterval(timer)
  if (timeoutLeft.value > 0) {
    timer = setInterval(() => {
      timeoutLeft.value--
      if (timeoutLeft.value <= 0) { clearInterval(timer); cancel() }
    }, 1000)
  }
}, { immediate: true, deep: true })

function cancel() {
  clearInterval(timer)
  emit('respond', { id: req.value.id, approved: false, values: {}, selected: '' })
  emit('close')
}

function submit() {
  clearInterval(timer)
  submitting.value = true
  const resp = {
    id: req.value.id,
    approved: true,
    values: req.value.type === 'confirm' ? {} : { ...values },
    selected: req.value.type === 'select' ? selected.value : ''
  }
  emit('respond', resp)
  setTimeout(() => { emit('close'); submitting.value = false }, 300)
}

onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.interact-overlay { position: fixed; inset: 0; z-index: 600; background: rgba(15,23,42,0.4); backdrop-filter: blur(8px); display: flex; align-items: center; justify-content: center; }
.interact-card { background: var(--glass-bg-elevated); border: 1px solid var(--glass-border-strong); border-radius: 16px; padding: 24px; min-width: 360px; max-width: 500px; width: 90vw; display: flex; flex-direction: column; gap: 14px; box-shadow: 0 20px 60px rgba(0,0,0,0.15); }
.variant-danger { border-color: rgba(220,38,38,0.3); }
.variant-warning { border-color: rgba(245,158,11,0.3); }

.interact-head { display: flex; align-items: center; gap: 10px; }
.interact-title { font-size: 15px; font-weight: 700; color: var(--text-primary); }
.interact-msg { font-size: 13px; color: var(--text-secondary); line-height: 1.6; white-space: pre-wrap; }
.interact-body { display: flex; flex-direction: column; gap: 10px; }

.interact-field { display: flex; flex-direction: column; gap: 3px; }
.interact-field label { font-size: 12px; font-weight: 600; color: var(--text-primary); }
.interact-field .req { color: var(--color-danger); }
.interact-hint { font-size: 10px; color: var(--text-muted); }
.interact-input { width: 100%; padding: 8px 10px; border: 1px solid var(--border-default); border-radius: 8px; background: rgba(255,255,255,0.4); color: var(--text-primary); font-size: 13px; font-family: inherit; outline: none; box-sizing: border-box; }
.interact-input:focus { border-color: var(--border-focus); box-shadow: 0 0 0 3px var(--focus-ring); }

.interact-option { padding: 10px 14px; border: 1px solid var(--border-default); border-radius: 8px; background: transparent; color: var(--text-primary); cursor: pointer; font-size: 13px; font-family: inherit; text-align: left; transition: all 0.1s; }
.interact-option:hover { background: var(--surface-hover); }
.interact-option.active { border-color: var(--brand-400); background: rgba(6,182,212,0.08); color: var(--brand-600); font-weight: 600; }

.interact-actions { display: flex; justify-content: flex-end; gap: 8px; }
.interact-btn { padding: 8px 18px; border-radius: 8px; font-size: 13px; font-family: inherit; font-weight: 500; cursor: pointer; transition: all 0.12s; border: 1px solid var(--border-default); }
.interact-btn.cancel { background: transparent; color: var(--text-secondary); }
.interact-btn.cancel:hover { background: var(--surface-hover); }
.interact-btn.confirm { border: none; color: #fff; }
.interact-btn.primary { background: var(--gradient-brand-btn); }
.interact-btn.warning { background: #f59e0b; }
.interact-btn.danger { background: #dc2626; }
.interact-btn:disabled { opacity: 0.5; }

.interact-timeout { font-size: 10px; color: var(--text-muted); text-align: center; }

.modal-enter-active { transition: all 0.2s ease-out; }
.modal-leave-active { transition: all 0.15s ease-in; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-from .interact-card, .modal-leave-to .interact-card { transform: scale(0.95) translateY(10px); }
</style>
