<template>
  <Teleport to="body">
    <!-- Overlay: subtle, no blur, bottom-aligned -->
    <Transition name="slide-up">
      <div
        v-if="visible"
        class="danger-overlay"
      >
        <!-- Scrim behind the card -->
        <div class="danger-scrim"></div>

        <div class="danger-card">
          <!-- Stripe indicator -->
          <div class="danger-stripe"></div>

          <div class="danger-body">
            <div class="danger-icon-wrap">
              <svg
                class="danger-icon"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
              >
                <circle
                  cx="12"
                  cy="12"
                  r="10"
                />
                <line
                  x1="12"
                  y1="8"
                  x2="12"
                  y2="12"
                />
                <line
                  x1="12"
                  y1="16"
                  x2="12.01"
                  y2="16"
                />
              </svg>
            </div>

            <div class="danger-content">
              <h3 class="danger-title">
                确认执行危险操作
              </h3>

              <div class="danger-details">
                <div class="detail-line">
                  <span class="detail-key">工具</span>
                  <code class="detail-val-tool">{{ request?.tool || '未知' }}</code>
                </div>
                <div class="detail-line">
                  <span class="detail-key">操作</span>
                  <span class="detail-val-msg">{{ request?.message || '确认执行此操作？' }}</span>
                </div>
              </div>

              <div
                v-if="request?.fields?.length"
                class="danger-fields"
              >
                <div
                  v-for="f in request.fields"
                  :key="f.name"
                  class="field-row"
                >
                  <label>{{ f.label }}<span
                    v-if="f.required"
                    class="star"
                  >*</span></label>
                  <input
                    v-model="fieldValues[f.name]"
                    :placeholder="f.value || ''"
                    class="field-input"
                  />
                </div>
              </div>
            </div>
          </div>

          <div class="danger-actions">
            <button
              class="btn-cancel"
              :disabled="sending"
              @click="cancel"
            >
              取消
            </button>
            <button
              class="btn-execute"
              :disabled="sending"
              @click="confirm"
            >
              <svg
                v-if="sending"
                class="spin-icon"
                viewBox="0 0 16 16"
                fill="none"
                stroke="currentColor"
                stroke-width="1.5"
              ><circle
                cx="8"
                cy="8"
                r="6"
                stroke-dasharray="30"
                stroke-linecap="round"
              /></svg>
              {{ sending ? '提交中' : '确认执行' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, reactive, watch } from 'vue'

const props = defineProps({
  request: { type: Object, default: null },
  sessionId: { type: String, default: '' }
})

const emit = defineEmits(['done'])

const visible = ref(false)
const sending = ref(false)
const fieldValues = reactive({})

watch(() => props.request, (req) => {
  if (req) {
    visible.value = true
    for (const k in fieldValues) delete fieldValues[k]
    if (req.fields) {
      for (const f of req.fields) {
        fieldValues[f.name] = f.value || ''
      }
    }
  }
}, { deep: true })

async function confirm() {
  sending.value = true
  try {
    await fetch('/api/chat/confirm', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + (localStorage.getItem('token') || '')
      },
      body: JSON.stringify({
        confirm_id: props.request.id,
        approved: true,
        fields: { ...fieldValues }
      })
    })
  } catch (e) { /* ignore */ }
  visible.value = false
  emit('done')
}

async function cancel() {
  sending.value = true
  try {
    await fetch('/api/chat/confirm', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': 'Bearer ' + (localStorage.getItem('token') || '')
      },
      body: JSON.stringify({
        confirm_id: props.request.id,
        approved: false,
        fields: {}
      })
    })
  } catch (e) { /* ignore */ }
  visible.value = false
  emit('done')
}
</script>

<style scoped>
/* ── Overlay ── */
.danger-overlay {
  position: fixed; inset: 0; z-index: 300;
  display: flex; align-items: flex-end; justify-content: center;
  padding-bottom: 120px;
  pointer-events: none;
}
.danger-scrim {
  position: absolute; inset: 0;
  background: rgba(15, 23, 42, 0.25);
  pointer-events: auto;
}

/* ── Card ── */
.danger-card {
  position: relative; z-index: 301;
  max-width: 480px; width: 92vw;
  background: var(--surface-raised, #fff);
  border-radius: 16px;
  box-shadow:
    0 -4px 24px rgba(220, 38, 38, 0.12),
    0 8px 40px rgba(0, 0, 0, 0.18),
    0 1px 0 rgba(220, 38, 38, 0.2);
  overflow: hidden;
  pointer-events: auto;
}
[data-theme="dark"] .danger-card {
  background: #1e293b;
  box-shadow:
    0 -4px 24px rgba(239, 68, 68, 0.15),
    0 8px 40px rgba(0, 0, 0, 0.4);
}

/* ── Top stripe ── */
.danger-stripe {
  height: 3px;
  background: linear-gradient(90deg, #ef4444, #f97316, #ef4444);
}

/* ── Body ── */
.danger-body {
  display: flex; gap: 14px; padding: 20px 22px 0;
}
.danger-icon-wrap {
  flex-shrink: 0; width: 36px; height: 36px;
  display: flex; align-items: center; justify-content: center;
  background: rgba(239, 68, 68, 0.1); border-radius: 50%;
}
.danger-icon {
  width: 20px; height: 20px; color: #ef4444;
}
.danger-content {
  flex: 1; min-width: 0;
  display: flex; flex-direction: column; gap: 10px;
}
.danger-title {
  margin: 0; font-size: 15px; font-weight: 700;
  color: #ef4444;
}
[data-theme="dark"] .danger-title { color: #f87171; }

/* ── Details ── */
.danger-details {
  display: flex; flex-direction: column; gap: 6px;
  padding: 10px 12px;
  background: var(--surface-card, #f8fafc);
  border: 1px solid var(--border-default, #e2e8f0);
  border-radius: 10px;
}
[data-theme="dark"] .danger-details {
  background: rgba(255,255,255,0.04);
  border-color: rgba(255,255,255,0.08);
}
.detail-line {
  display: flex; flex-direction: column; gap: 1px;
}
.detail-key {
  font-size: 10px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.6px;
  color: var(--text-muted, #94a3b8);
}
.detail-val-tool {
  font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', var(--font-mono), monospace;
  font-size: 12px; font-weight: 600;
  color: #ef4444;
  background: rgba(239, 68, 68, 0.06);
  padding: 2px 8px; border-radius: 4px;
  display: inline-block; align-self: flex-start;
}
.detail-val-msg {
  font-size: 13px; color: var(--text-primary, #1e293b);
  line-height: 1.5; word-break: break-all;
}

/* ── Fields ── */
.danger-fields {
  display: flex; flex-direction: column; gap: 8px;
}
.field-row {
  display: flex; flex-direction: column; gap: 3px;
}
.field-row label {
  font-size: 11px; font-weight: 500; color: var(--text-muted, #94a3b8);
}
.star { color: #ef4444; margin-left: 1px; }
.field-input {
  padding: 7px 10px;
  border: 1px solid var(--border-default, #e2e8f0);
  border-radius: 8px;
  background: var(--surface-input, #fff);
  color: var(--text-primary, #1e293b);
  font-size: 13px; outline: none;
}
.field-input:focus {
  border-color: #ef4444;
  box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.12);
}

/* ── Actions ── */
.danger-actions {
  display: flex; gap: 10px; justify-content: flex-end;
  padding: 16px 22px;
}
.btn-cancel {
  padding: 8px 22px; border-radius: 10px;
  border: 1px solid var(--border-default, #e2e8f0);
  background: transparent;
  color: var(--text-secondary, #64748b);
  font-size: 13px; font-weight: 500;
  cursor: pointer; transition: all 0.15s;
}
.btn-cancel:hover { background: var(--surface-hover, #f1f5f9); }
.btn-execute {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 22px; border-radius: 10px; border: none;
  background: linear-gradient(135deg, #ef4444, #dc2626);
  color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: all 0.15s;
}
.btn-execute:hover { background: linear-gradient(135deg, #dc2626, #b91c1c); transform: translateY(-1px); }
.btn-execute:disabled, .btn-cancel:disabled { opacity: 0.4; cursor: default; transform: none; }
.spin-icon { width: 14px; height: 14px; animation: spin 0.8s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* ── Transition: slide up from bottom ── */
.slide-up-enter-active { transition: all 0.2s ease-out; }
.slide-up-leave-active { transition: all 0.15s ease-in; }
.slide-up-enter-from { opacity: 0; }
.slide-up-enter-from .danger-card { transform: translateY(20px); opacity: 0; }
.slide-up-leave-to { opacity: 0; }
.slide-up-leave-to .danger-card { transform: translateY(10px); opacity: 0; }
</style>
