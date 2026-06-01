<template>
  <form class="login-form" @submit.prevent="handleSubmit" novalidate>
    <!-- Username -->
    <div class="field-group">
      <div class="input-wrap">
        <span class="input-icon">
          <svg viewBox="0 0 20 20" fill="none"><circle cx="10" cy="7" r="3.5" stroke="currentColor" stroke-width="1.5"/><path d="M3 18c0-3.9 3.1-7 7-7s7 3.1 7 7" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </span>
        <input
          ref="usernameRef"
          v-model="username"
          type="text"
          class="form-input"
          placeholder="用户名"
          autocomplete="username"
          @keyup.enter="focusPassword"
        />
        <label class="floating-label" :class="{ active: username }">用户名</label>
      </div>
    </div>

    <!-- Password -->
    <div class="field-group">
      <div class="input-wrap">
        <span class="input-icon">
          <svg viewBox="0 0 20 20" fill="none"><rect x="3.5" y="7.5" width="13" height="10" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M6.5 7.5V6a3.5 3.5 0 117 0v1.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><circle cx="10" cy="12.5" r="1.2" fill="currentColor"/></svg>
        </span>
        <input
          ref="passwordRef"
          v-model="password"
          :type="showPassword ? 'text' : 'password'"
          class="form-input"
          placeholder="密码"
          autocomplete="current-password"
        />
        <label class="floating-label" :class="{ active: password }">密码</label>
        <button type="button" class="toggle-pw" @click="showPassword = !showPassword" tabindex="-1">
          <!-- Eye on -->
          <svg v-if="showPassword" viewBox="0 0 20 20" fill="none"><path d="M10 5C5 5 2 10 2 10s3 5 8 5 8-5 8-5-3-5-8-5z" stroke="currentColor" stroke-width="1.5"/><circle cx="10" cy="10" r="2.5" stroke="currentColor" stroke-width="1.5"/></svg>
          <!-- Eye off -->
          <svg v-else viewBox="0 0 20 20" fill="none"><path d="M10 5C5 5 2 10 2 10s3 5 8 5 8-5 8-5-3-5-8-5z" stroke="currentColor" stroke-width="1.5"/><circle cx="10" cy="10" r="2.5" stroke="currentColor" stroke-width="1.5"/><line x1="3" y1="3" x2="17" y2="17" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </button>
      </div>
    </div>

    <!-- Remember me -->
    <div class="field-meta">
      <label class="remember-label">
        <span class="remember-check" :class="{ checked: rememberMe }" @click="rememberMe = !rememberMe">
          <svg v-if="rememberMe" viewBox="0 0 12 12" fill="none"><path d="M2.5 6L5 8.5L9.5 3.5" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </span>
        <input type="checkbox" v-model="rememberMe" class="sr-only" />
        <span>记住此设备</span>
      </label>
    </div>

    <!-- Error -->
    <Transition name="slide-down">
      <div v-if="error" class="error-alert">
        <svg viewBox="0 0 16 16" fill="none" class="error-icon"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M8 5v3.5M8 11.5v.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        <span>{{ error }}</span>
      </div>
    </Transition>

    <!-- Submit button -->
    <button
      class="submit-btn"
      :class="{ loading: loading, success: success, shake: shakeBtn }"
      :disabled="!canSubmit || loading"
      type="submit"
    >
      <span v-if="loading" class="btn-inner">
        <span class="spinner"></span>
        登录中...
      </span>
      <span v-else-if="success" class="btn-inner">
        <svg viewBox="0 0 16 16" fill="none" class="check-icon"><path d="M3 8.5L6.5 12L13 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
        登录成功
      </span>
      <span v-else>登 录</span>
    </button>
  </form>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const usernameRef = ref(null)
const passwordRef = ref(null)
const username = ref('')
const password = ref('')
const showPassword = ref(false)
const rememberMe = ref(false)
const error = ref('')
const loading = ref(false)
const success = ref(false)
const shakeBtn = ref(false)

let errorTimer = null

const canSubmit = computed(() => username.value && password.value)

function focusPassword() {
  passwordRef.value?.focus()
}

function clearError() {
  error.value = ''
  if (errorTimer) { clearTimeout(errorTimer); errorTimer = null }
}

function showError(msg) {
  error.value = msg
  if (errorTimer) clearTimeout(errorTimer)
  errorTimer = setTimeout(() => { error.value = '' }, 3000)
}

async function handleSubmit() {
  if (!canSubmit.value || loading.value) return
  clearError()
  loading.value = true
  try {
    await auth.login(username.value, password.value)
    if (rememberMe.value) {
      localStorage.setItem('yunxi-remember-me', '1')
    }
    success.value = true
    setTimeout(() => router.push('/'), 600)
  } catch (e) {
    showError(e.response?.data?.message || '登录失败')
    shakeBtn.value = true
    setTimeout(() => { shakeBtn.value = false }, 500)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await nextTick()
  usernameRef.value?.focus()
})
</script>

<style scoped>
.login-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: 100%;
  max-width: 380px;
}

.field-group {
  width: 100%;
}

.input-wrap {
  position: relative;
  width: 100%;
}

.input-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-muted);
  display: flex;
  align-items: center;
  z-index: 1;
  pointer-events: none;
  transition: color 0.15s;
}
.input-icon svg {
  width: 16px;
  height: 16px;
}

.form-input {
  width: 100%;
  height: 44px;
  padding: 0 40px 0 38px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--surface-input);
  color: var(--text-primary);
  font-size: 14px;
  font-family: inherit;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.form-input:focus {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 3px var(--focus-ring);
}
.form-input::placeholder {
  color: transparent;
}
.form-input:focus::placeholder {
  color: var(--text-muted);
}

.floating-label {
  position: absolute;
  left: 38px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 14px;
  color: var(--text-muted);
  pointer-events: none;
  transition: all 0.2s var(--ease-out-expo);
  background: transparent;
  padding: 0 2px;
}
.form-input:focus + .floating-label,
.floating-label.active {
  top: 0;
  transform: translateY(-50%);
  font-size: 11px;
  color: var(--border-focus);
  background: var(--surface-input, rgba(255,255,255,0.4));
  padding: 0 4px;
}

.toggle-pw {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  border-radius: 4px;
  transition: color 0.15s;
}
.toggle-pw:hover {
  color: var(--text-secondary);
}
.toggle-pw svg {
  width: 18px;
  height: 18px;
}

.field-meta {
  display: flex;
  align-items: center;
}

.remember-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--text-sm);
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
}

.sr-only {
  position: absolute;
  width: 1px; height: 1px;
  overflow: hidden;
  clip: rect(0,0,0,0);
  white-space: nowrap;
  border: 0;
}

.remember-check {
  width: 16px;
  height: 16px;
  border-radius: 3px;
  border: 1.5px solid var(--border-strong);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
  flex-shrink: 0;
}
.remember-check.checked {
  background: var(--brand-500);
  border-color: var(--brand-500);
}
.remember-check svg {
  width: 10px;
  height: 10px;
}

/* ── Error alert ── */
.error-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--alert-error-bg);
  border: 1px solid var(--alert-error-border);
  font-size: var(--text-sm);
  color: var(--color-danger);
}
.error-icon {
  width: 15px;
  height: 15px;
  flex-shrink: 0;
}

/* ── Submit button ── */
.submit-btn {
  width: 100%;
  height: 44px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--gradient-brand);
  color: #fff;
  font-size: 14px;
  font-weight: var(--weight-semibold);
  cursor: pointer;
  font-family: inherit;
  letter-spacing: 0.06em;
  transition: all 0.2s var(--ease-out-expo);
  position: relative;
  overflow: hidden;
}
.submit-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 16px rgba(6,182,212,0.35);
}
.submit-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.submit-btn.loading {
  opacity: 0.85;
}
.submit-btn.success {
  background: linear-gradient(135deg, #16a34a, #22c55e);
}

.btn-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

.check-icon {
  width: 16px;
  height: 16px;
}

.shake {
  animation: shake 0.5s ease-in-out;
}
@keyframes shake {
  0%, 100% { transform: translateX(0); }
  15% { transform: translateX(-6px); }
  30% { transform: translateX(6px); }
  45% { transform: translateX(-4px); }
  60% { transform: translateX(4px); }
  75% { transform: translateX(-2px); }
  90% { transform: translateX(2px); }
}

/* ── Transitions ── */
.slide-down-enter-active {
  transition: all 0.25s var(--ease-out-expo);
}
.slide-down-leave-active {
  transition: all 0.2s var(--ease-in-out);
}
.slide-down-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}
.slide-down-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

@media (max-width: 767px) {
  .form-input {
    height: 44px;
  }
  .submit-btn {
    height: 44px;
  }
}
</style>
