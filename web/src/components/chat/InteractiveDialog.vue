<template>
  <Transition name="modal">
    <div
      v-if="visible"
      class="interact-overlay"
      @click.self="cancel"
    >
      <div
        class="interact-card"
        :class="'variant-' + (req.variant || 'info')"
      >
        <div class="interact-head">
          <span class="interact-icon">
            <svg
              v-if="req.type === 'confirm' && req.variant === 'danger'"
              width="20"
              height="20"
              viewBox="0 0 20 20"
              fill="none"
            ><path
              d="M10 2a8 8 0 100 16 8 8 0 000-16zM10 6v4M10 13v1"
              stroke="var(--color-danger)"
              stroke-width="1.5"
              stroke-linecap="round"
            /></svg>
            <svg
              v-else-if="req.type === 'form' || req.type === 'input'"
              width="20"
              height="20"
              viewBox="0 0 20 20"
              fill="none"
            ><rect
              x="3"
              y="3"
              width="14"
              height="14"
              rx="2"
              stroke="var(--brand-500)"
              stroke-width="1.5"
            /><path
              d="M7 8h6M7 12h4"
              stroke="var(--brand-500)"
              stroke-width="1.5"
              stroke-linecap="round"
            /></svg>
            <svg
              v-else
              width="20"
              height="20"
              viewBox="0 0 20 20"
              fill="none"
            ><circle
              cx="10"
              cy="10"
              r="8"
              stroke="var(--brand-500)"
              stroke-width="1.5"
            /><path
              d="M10 6v5M10 13.5v.5"
              stroke="var(--brand-500)"
              stroke-width="1.5"
              stroke-linecap="round"
            /></svg>
          </span>
          <span class="interact-title">{{ req.title || 'AI 请求' }}</span>
        </div>

        <div
          v-if="req.message"
          class="interact-msg"
        >
          {{ req.message }}
        </div>

        <!-- v3.3: 操作链接（如 GitHub Token 页面） -->
        <a
          v-if="req.action_url"
          :href="req.action_url"
          target="_blank"
          rel="noopener"
          class="interact-action-link"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M11 7.5v4a.5.5 0 01-.5.5h-8a.5.5 0 01-.5-.5v-8A.5.5 0 012.5 3h4M9 2h3v3M6.5 7.5L12 2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          {{ req.action_label || '打开链接' }}
        </a>

        <!-- confirm 模式 -->
        <div
          v-if="req.type === 'confirm'"
          class="interact-body"
        ></div>

        <!-- input 模式 -->
        <div
          v-if="req.type === 'input'"
          class="interact-body"
        >
          <div
            v-for="f in (req.fields || [])"
            :key="f.name"
            class="interact-field"
          >
            <label>{{ f.label }} <span
              v-if="f.required"
              class="req"
            >*</span></label>
            <input
              v-model="values[f.name]"
              :type="f.type || 'text'"
              :placeholder="f.placeholder"
              class="interact-input"
            />
          </div>
        </div>

        <!-- form 模式 -->
        <div
          v-if="req.type === 'form'"
          class="interact-body"
        >
          <div
            v-for="f in (req.fields || [])"
            :key="f.name"
            class="interact-field"
          >
            <label>{{ f.label }} <span
              v-if="f.required"
              class="req"
            >*</span></label>
            <input
              v-model="values[f.name]"
              :type="f.type || 'text'"
              :placeholder="f.placeholder || f.label"
              class="interact-input"
            />
            <span class="interact-hint">{{ f.default ? '默认: ' + f.default : '' }}</span>
          </div>
        </div>

        <!-- select 模式 -->
        <div
          v-if="req.type === 'select'"
          class="interact-body"
        >
          <button
            v-for="opt in (req.options || [])"
            :key="opt"
            :class="['interact-option', { active: selected === opt }]"
            @click="selected = opt"
          >
            {{ opt }}
          </button>
        </div>

        <!-- v3.1 多页向导模式 -->
        <div v-if="isWizard" class="interact-body wizard-body">
          <div class="wizard-step-indicator">
            <span class="wizard-step-label">&lt; {{ currentPage + 1 }}/{{ req.pages.length }} &gt;</span>
            <div class="wizard-dots">
              <span v-for="(_, i) in req.pages" :key="i" class="wizard-dot" :class="{ active: i === currentPage, done: i < currentPage }"></span>
            </div>
          </div>
          <div class="wizard-page-title">{{ req.pages[currentPage].title }}</div>
          <div v-if="req.pages[currentPage].description" class="wizard-page-desc">{{ req.pages[currentPage].description }}</div>
          <!-- 当前页为确认页（无输入字段）时，展示已填写的所有值 -->
          <div v-if="!req.pages[currentPage].fields?.length" class="wizard-summary">
            <div v-for="(_, i) in req.pages" :key="'sum'+i">
              <div v-if="i < currentPage" v-for="f in req.pages[i].fields" :key="'f'+f.name" class="summary-row">
                <span class="sum-label">{{ f.label }}</span>
                <span class="sum-value">{{ values[f.name] || '(未填)' }}</span>
              </div>
            </div>
          </div>
          <div v-for="f in req.pages[currentPage].fields" :key="f.name" class="interact-field">
            <label>{{ f.label }} <span v-if="f.required" class="req">*</span></label>
            <input v-model="values[f.name]" :type="f.type || 'text'" :placeholder="f.placeholder || f.default" class="interact-input" />
          </div>
        </div>

        <div class="interact-actions">
          <button class="interact-btn cancel" @click="cancel">{{ req.cancel_text || '取消' }}</button>
          <template v-if="isWizard">
            <button v-if="currentPage > 0" class="interact-btn prev" @click="currentPage--">上一步</button>
            <button v-if="currentPage < req.pages.length - 1" class="interact-btn confirm" :class="variantClass" @click="wizardNext">下一步</button>
            <button v-else class="interact-btn confirm" :class="variantClass" :disabled="submitting" @click="submit">提交</button>
          </template>
          <button v-else class="interact-btn confirm" :class="variantClass" :disabled="submitting" @click="submit">
            {{ submitting ? '提交中...' : (req.confirm_text || getDefaultConfirm()) }}
          </button>
        </div>
        <div
          v-if="req.timeout_sec"
          class="interact-timeout"
        >
          ⏱ {{ timeoutLeft }}s 后自动取消
        </div>
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
const currentPage = ref(0)
let timer = null

const isWizard = computed(() => (req.value.pages?.length || 0) > 0)
function wizardNext() {
  const page = req.value.pages[currentPage.value]
  const missing = (page.fields || []).filter((f: any) => f.required && !values[f.name])
  if (missing.length) return
  if (currentPage.value < (req.value.pages?.length || 1) - 1) currentPage.value++
}

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
  // 填默认值（支持 plain fields 和 wizard pages）
  for (const f of (r.fields || [])) {
    if (f.default) values[f.name] = f.default
  }
  for (const p of (r.pages || [])) {
    for (const f of (p.fields || [])) {
      if (f.default) values[f.name] = f.default
    }
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
.interact-action-link { display: inline-flex; align-items: center; gap: 6px; padding: 8px 14px; border-radius: 8px; background: rgba(6,182,212,0.08); border: 1px solid rgba(6,182,212,0.2); color: var(--brand-500); font-size: 13px; font-weight: 500; text-decoration: none; transition: all 0.15s; width: fit-content; }
.interact-action-link:hover { background: rgba(6,182,212,0.15); border-color: var(--brand-400); }
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

/* ── v3.1 多页向导 ── */
.wizard-body { display: flex; flex-direction: column; gap: 10px; }
.wizard-step-indicator { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.wizard-step-label { font-size: 11px; font-weight: 600; color: var(--brand-500); white-space: nowrap; }
.wizard-dots { display: flex; gap: 4px; }
.wizard-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--border-default); transition: all 0.15s; }
.wizard-dot.active { background: var(--brand-500); transform: scale(1.3); }
.wizard-dot.done { background: var(--brand-300); }
.wizard-page-title { font-size: 14px; font-weight: 600; color: var(--text-primary); }
.wizard-summary { background: rgba(6,182,212,0.05); border: 1px solid var(--border-default); border-radius: 8px; padding: 10px 12px; display: flex; flex-direction: column; gap: 6px; max-height: 180px; overflow-y: auto; }
.summary-row { display: flex; justify-content: space-between; align-items: center; font-size: 12px; }
.sum-label { color: var(--text-secondary); }
.sum-value { color: var(--text-primary); font-weight: 500; }
.wizard-page-desc { font-size: 12px; color: var(--text-muted); line-height: 1.5; }
.interact-btn.prev { background: transparent; color: var(--text-secondary); border: 1px solid var(--border-default); }
.interact-btn.prev:hover { background: var(--surface-hover); }
</style>
