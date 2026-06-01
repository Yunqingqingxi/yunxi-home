<template>
  <div class="setup-card">
    <!-- Step progress bar -->
    <div class="progress-bar">
      <div class="progress-step" :class="{ active: wizardStep <= 1, done: wizardStep > 1 }">
        <span class="progress-dot">1</span>
        <span class="progress-label">设置密码</span>
      </div>
      <span class="progress-line" :class="{ fill: wizardStep > 1 }"></span>
      <div class="progress-step" :class="{ active: wizardStep >= 2, done: wizardStep > 2 }">
        <span class="progress-dot">2</span>
        <span class="progress-label">系统配置</span>
      </div>
    </div>

    <!-- Step transition wrapper -->
    <Transition :name="transitionName" mode="out-in">
      <!-- Step 1: Set admin password -->
      <div v-if="wizardStep === 1" key="step1" class="step-panel">
        <p class="setup-desc">首次使用，请为管理员 <strong>admin</strong> 设置密码</p>
        <div class="field">
          <input type="password" v-model="setupPassword" class="form-input" placeholder="输入密码（至少 6 位）" @keyup.enter="doSetupPassword" />
        </div>
        <div class="field">
          <input type="password" v-model="setupPassword2" class="form-input" placeholder="确认密码" @keyup.enter="doSetupPassword" />
        </div>
        <Transition name="slide-down">
          <div v-if="error" class="error-alert">
            <svg viewBox="0 0 16 16" fill="none" class="error-icon"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M8 5v3.5M8 11.5v.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            <span>{{ error }}</span>
          </div>
        </Transition>
        <button class="submit-btn" :disabled="!canProceed || settingUp" @click="doSetupPassword">
          {{ settingUp ? '设置中...' : '下一步' }}
        </button>
      </div>

      <!-- Step 2: System check -->
      <div v-else-if="wizardStep === 2" key="step2" class="step-panel">
        <p class="setup-desc">系统环境配置</p>
        <p class="setup-info">将创建 <strong>yunxi</strong> 系统用户和用户组，用于隔离服务权限，保障系统安全。</p>
        <div class="check-list">
          <TransitionGroup name="check-stagger">
            <div v-for="s in visibleSteps" :key="s.name" class="check-item" :class="{ ok: s.success, fail: s.status === 'fail' }">
              <span class="check-icon">
                <svg v-if="s.success" viewBox="0 0 14 14" fill="none"><path d="M3 7L6 10L11 4" stroke="#22c55e" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
                <svg v-else-if="s.status === 'fail'" viewBox="0 0 14 14" fill="none"><path d="M4 4l6 6M10 4l-6 6" stroke="#ef4444" stroke-width="1.8" stroke-linecap="round"/></svg>
                <span class="check-spinner" v-else></span>
              </span>
              <span class="check-name">{{ s.name }}</span>
              <span class="check-msg">{{ s.message }}</span>
            </div>
          </TransitionGroup>
        </div>
        <Transition name="slide-down">
          <div v-if="error" class="error-alert">
            <svg viewBox="0 0 16 16" fill="none" class="error-icon"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.2"/><path d="M8 5v3.5M8 11.5v.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            <span>{{ error }}</span>
          </div>
        </Transition>
        <div v-if="allOk" class="success-banner">
          <svg viewBox="0 0 16 16" fill="none"><path d="M3 8.5L6.5 12L13 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          <span>配置完成！{{ countdown }} 秒后自动跳转...</span>
        </div>
        <button v-if="!allOk" class="submit-btn" :disabled="runningSetup" @click="runSetup">
          {{ runningSetup ? '配置中...' : '一键配置' }}
        </button>
        <button v-if="!allOk" class="submit-btn secondary" @click="goBack">返回上一步</button>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import api from '../../services/api'
import { useAuthStore } from '../../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const wizardStep = ref(1)
const transitionName = ref('slide-left')
const setupPassword = ref('')
const setupPassword2 = ref('')
const settingUp = ref(false)
const runningSetup = ref(false)
const error = ref('')
const allSetupSteps = ref([])
const allOk = ref(false)
const countdown = ref(3)
let errorTimer = null
let countdownTimer = null

const canProceed = computed(() => setupPassword.value && setupPassword.value === setupPassword2.value && setupPassword.value.length >= 6)

// Staggered step visibility for animation
const visibleSteps = ref([])
let staggerTimer = null

function showError(msg) {
  error.value = msg
  if (errorTimer) clearTimeout(errorTimer)
  errorTimer = setTimeout(() => { error.value = '' }, 3000)
}

function goBack() {
  transitionName.value = 'slide-right'
  wizardStep.value = 1
}

async function doSetupPassword() {
  if (!canProceed.value) {
    if (setupPassword.value.length < 6) { showError('密码至少 6 位'); return }
    if (setupPassword.value !== setupPassword2.value) { showError('两次密码不一致'); return }
    return
  }
  settingUp.value = true; error.value = ''
  try {
    await api.post('/api/auth/setup', { password: setupPassword.value })
    await auth.login('admin', setupPassword.value)
    transitionName.value = 'slide-left'
    wizardStep.value = 2
    await loadSysStatus()
  } catch (e) {
    showError(e.response?.data?.message || '设置失败')
  } finally {
    settingUp.value = false
  }
}

async function loadSysStatus() {
  try {
    const r = await api.get('/api/system/setup-status')
    const s = r.data.data || { commands: [] }
    allSetupSteps.value = [
      { name: '管理员密码', success: !!s.admin_password_set, status: s.admin_password_set ? 'ok' : 'pending' },
      { name: '创建 yunxi 用户', success: !!s.yunxi_user_exists, status: s.yunxi_user_exists ? 'ok' : 'pending' },
      { name: '创建 yunxi 用户组', success: !!s.yunxi_group_exists, status: s.yunxi_group_exists ? 'ok' : 'pending' },
      { name: '沙箱环境', success: !!s.sandbox_ok, status: s.sandbox_ok ? 'ok' : 'pending' },
    ]
  } catch (e) {
    allSetupSteps.value = [
      { name: '管理员密码', success: false, status: 'pending' },
      { name: '创建 yunxi 用户', success: false, status: 'pending' },
      { name: '创建 yunxi 用户组', success: false, status: 'pending' },
      { name: '沙箱环境', success: false, status: 'pending' },
    ]
  }
}

async function runSetup() {
  runningSetup.value = true; error.value = ''
  visibleSteps.value = []
  try {
    const r = await api.post('/api/system/run-setup')
    const steps = r.data.data?.steps || []
    // Map steps from API response
    allSetupSteps.value = steps.map((s, i) => ({
      name: s.name || `步骤 ${i + 1}`,
      success: !!s.success,
      message: s.message || '',
      status: s.success ? 'ok' : 'fail',
    }))

    // Stagger in the results
    for (let i = 0; i < allSetupSteps.value.length; i++) {
      await new Promise(r => setTimeout(r, 350))
      visibleSteps.value = allSetupSteps.value.slice(0, i + 1)
    }

    if (r.data.data?.all_ok) {
      allOk.value = true
      countdown.value = 3
      countdownTimer = setInterval(() => {
        countdown.value--
        if (countdown.value <= 0) {
          clearInterval(countdownTimer)
          router.push('/')
        }
      }, 1000)
    }
  } catch (e) {
    showError('配置失败: ' + (e.response?.data?.message || e.message))
  } finally {
    runningSetup.value = false
  }
}

onMounted(async () => {
  await nextTick()
  await loadSysStatus()
})

onBeforeUnmount(() => {
  if (errorTimer) clearTimeout(errorTimer)
  if (countdownTimer) clearInterval(countdownTimer)
  if (staggerTimer) clearTimeout(staggerTimer)
})
</script>

<style scoped>
.setup-card {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  max-width: 380px;
}

/* ── Progress bar ── */
.progress-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  margin-bottom: 4px;
}
.progress-step {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
}
.progress-dot {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--surface-input);
  border: 2px solid var(--border-default);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: var(--weight-semibold);
  color: var(--text-muted);
  transition: all 0.3s;
}
.progress-step.active .progress-dot {
  background: var(--brand-500);
  border-color: var(--brand-500);
  color: #fff;
}
.progress-step.done .progress-dot {
  background: #22c55e;
  border-color: #22c55e;
  color: #fff;
}
.progress-label {
  font-size: var(--text-2xs);
  color: var(--text-muted);
  white-space: nowrap;
}
.progress-step.active .progress-label {
  color: var(--brand-500);
  font-weight: var(--weight-medium);
}
.progress-step.done .progress-label {
  color: #22c55e;
}
.progress-line {
  width: 48px;
  height: 2px;
  background: var(--border-default);
  margin: 0 8px;
  margin-bottom: 18px;
  transition: background 0.3s;
  border-radius: 1px;
}
.progress-line.fill {
  background: var(--brand-400);
}

/* ── Step panels ── */
.step-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.setup-desc {
  text-align: center;
  font-size: var(--text-sm);
  color: var(--text-secondary);
  margin: 0;
}
.setup-desc strong { color: var(--text-primary); }

.setup-info {
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
  margin: 0;
  line-height: 1.6;
}
.setup-info strong { color: var(--text-primary); }

/* ── Form fields ── */
.field { width: 100%; }
.form-input {
  width: 100%;
  height: 44px;
  padding: 0 14px;
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
.form-input::placeholder { color: var(--text-muted); }

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
  letter-spacing: 0.04em;
  transition: all 0.2s var(--ease-out-expo);
}
.submit-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 16px rgba(6,182,212,0.35);
}
.submit-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.submit-btn.secondary {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border-default);
}
.submit-btn.secondary:hover {
  background: var(--surface-hover);
  transform: none;
  box-shadow: none;
}

/* ── Error ── */
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

/* ── Check list ── */
.check-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.check-item {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--text-muted);
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-input);
  transition: all 0.2s;
}
.check-item.ok {
  color: var(--text-primary);
}
.check-item.fail {
  color: var(--color-danger);
}
.check-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.check-icon svg {
  width: 16px;
  height: 16px;
}
.check-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-default);
  border-top-color: var(--brand-400);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.check-name {
  font-weight: var(--weight-medium);
}
.check-msg {
  font-size: 10px;
  color: var(--text-muted);
  margin-left: auto;
  max-width: 200px;
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ── Success banner ── */
.success-banner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--alert-success-bg);
  border: 1px solid var(--alert-success-border);
  color: var(--color-success);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
}
.success-banner svg {
  width: 16px;
  height: 16px;
}

/* ── Transitions ── */
.slide-left-enter-active,
.slide-left-leave-active,
.slide-right-enter-active,
.slide-right-leave-active {
  transition: all 0.3s var(--ease-out-expo);
}
.slide-left-enter-from,
.slide-right-leave-to {
  opacity: 0;
  transform: translateX(24px);
}
.slide-left-leave-to,
.slide-right-enter-from {
  opacity: 0;
  transform: translateX(-24px);
}

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

/* Staggered check items */
.check-stagger-enter-active {
  transition: all 0.35s var(--ease-out-expo);
}
.check-stagger-enter-from {
  opacity: 0;
  transform: translateX(-8px);
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
