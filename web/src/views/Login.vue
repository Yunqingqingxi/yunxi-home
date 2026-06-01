<template>
  <div class="login-page">
    <!-- Left: Brand panel -->
    <div class="login-left">
      <LoginBrand />
    </div>

    <!-- Right: Form / Setup -->
    <div class="login-right">
      <div class="login-form-card glass-elevated">
        <Transition name="slide-fade" mode="out-in">
          <LoginSetup
            v-if="step === 1 || step === 2"
            key="setup"
          />
          <LoginForm v-else key="login" />
        </Transition>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted } from 'vue'
import api from '../services/api'
import LoginBrand from './login/LoginBrand.vue'
import LoginForm from './login/LoginForm.vue'
import LoginSetup from './login/LoginSetup.vue'

const step = ref(0) // 0=login, 1/2=setup wizard

async function checkStatus() {
  try {
    const r = await api.get('/api/auth/status')
    if (r.data.data?.needs_setup) {
      step.value = 1
    } else {
      step.value = 0
    }
  } catch (e) {
    step.value = 0
  }
}

onMounted(checkStatus)
</script>

<style scoped>
.login-page {
  display: flex;
  width: 100%;
  min-height: 100%;
}

/* ── Left brand panel ── */
.login-left {
  width: 40%;
  display: flex;
  align-items: stretch;
}

/* ── Right form panel ── */
.login-right {
  width: 60%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  background: var(--surface-page);
}
[data-theme="dark"] .login-right {
  background: var(--surface-page);
}

.login-form-card {
  width: 100%;
  max-width: 420px;
  padding: 36px 32px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

/* ── Transition: slide-fade ── */
.slide-fade-enter-active {
  transition: all 0.3s var(--ease-out-expo);
}
.slide-fade-leave-active {
  transition: all 0.2s var(--ease-in-out);
}
.slide-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}
.slide-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

/* ── Mobile: single column ── */
@media (max-width: 767px) {
  .login-page {
    flex-direction: column;
  }

  .login-left {
    width: 100%;
  }

  .login-right {
    width: 100%;
    padding: var(--space-5) var(--space-4);
    flex: 1;
    align-items: flex-start;
  }

  .login-form-card {
    max-width: 100%;
    padding: 24px 18px;
  }
}
</style>
