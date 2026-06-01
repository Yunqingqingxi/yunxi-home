<template>
  <div class="brand-panel">
    <div class="brand-content">
      <div class="brand-logo">
        <!-- Cloud + House SVG -->
        <svg
          viewBox="0 0 48 48"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <defs>
            <linearGradient
              id="blg"
              x1="4"
              y1="4"
              x2="44"
              y2="44"
              gradientUnits="userSpaceOnUse"
            >
              <stop
                offset="0%"
                stop-color="#fff"
                stop-opacity="0.95"
              />
              <stop
                offset="100%"
                stop-color="#e0f2fe"
                stop-opacity="0.7"
              />
            </linearGradient>
          </defs>
          <!-- Cloud -->
          <path
            d="M8 28 C4 26 3 20 7 17 C4 11 10 6 18 8 C16 2 28 2 30 8 C35 4 40 9 39 15 C43 14 44 22 39 26 L39 27 C40 29 39 32 35 32 L11 32 C6 32 5 30 8 28Z"
            fill="url(#blg)"
            opacity="0.9"
          />
          <!-- House -->
          <polygon
            points="16,24 24,16 32,24"
            fill="#fff"
            opacity="0.85"
          />
          <rect
            x="19"
            y="24"
            width="10"
            height="8"
            rx="0.5"
            fill="#fff"
            opacity="0.85"
          />
          <rect
            x="22"
            y="27"
            width="4"
            height="5"
            rx="1.5"
            fill="url(#blg)"
          />
          <!-- Ring -->
          <circle
            cx="24"
            cy="24"
            r="22"
            stroke="url(#blg)"
            stroke-width="1"
            opacity="0.2"
          />
        </svg>
      </div>
      <h1 class="brand-title">
        云兮之家
      </h1>
      <p class="brand-subtitle">
        Yunxi Home
      </p>
      <p
        v-if="version"
        class="brand-version"
      >
        v{{ version }}
      </p>
    </div>
    <button
      class="theme-toggle"
      :title="isDark ? '切换亮色主题' : '切换暗色主题'"
      @click="toggleTheme"
    >
      <!-- Sun icon -->
      <svg
        v-if="!isDark"
        viewBox="0 0 20 20"
        fill="none"
      ><circle
        cx="10"
        cy="10"
        r="4"
        stroke="currentColor"
        stroke-width="1.5"
      /><path
        d="M10 1v1.5M10 17.5V19M1 10h1.5M17.5 10H19M3.6 3.6l1.1 1.1M15.3 15.3l1.1 1.1M3.6 16.4l1.1-1.1M15.3 4.7l1.1-1.1"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
      /></svg>
      <!-- Moon icon -->
      <svg
        v-else
        viewBox="0 0 20 20"
        fill="none"
      ><path
        d="M16 12.5A6.5 6.5 0 017.5 4c.5-.1.8-.2 1-.1-3 .6-5 3.5-5 6.6a6.5 6.5 0 0011 5c-.3-.3.4-1.6.1-1.3.3-.2.6-.4 1.2-.5.4-.1.2-.2 0-.2z"
        fill="currentColor"
      /></svg>
    </button>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, onMounted } from 'vue'
import { useThemeStore } from '../../stores/theme'
import api from '../../services/api'

const themeStore = useThemeStore()
const isDark = computed(() => themeStore.theme === 'dark')
const version = ref('')

function toggleTheme() {
  themeStore.toggle()
}

onMounted(async () => {
  try {
    const r = await api.get('/api/status')
    version.value = r.data?.data?.version || ''
  } catch (_) { /* ignore */ }
})
</script>

<style scoped>
.brand-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0;
  height: 100%;
  padding: var(--space-8);
  position: relative;
  background: linear-gradient(180deg, #0c4a6e 0%, #155e75 40%, #0e7490 100%);
}
[data-theme="dark"] .brand-panel {
  background: linear-gradient(180deg, #020617 0%, #0b1121 40%, #0f172a 100%);
}

.brand-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  text-align: center;
}

.brand-logo {
  width: 72px;
  height: 72px;
  border-radius: 18px;
  background: rgba(255,255,255,0.08);
  border: 1px solid rgba(255,255,255,0.12);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
}
.brand-logo svg {
  width: 42px;
  height: 42px;
}

.brand-title {
  font-size: 28px;
  font-weight: var(--weight-bold);
  letter-spacing: 0.06em;
  margin: 0;
  color: #fff;
}
.brand-subtitle {
  font-size: var(--text-sm);
  color: rgba(255,255,255,0.55);
  margin: 0;
  letter-spacing: 0.05em;
}
.brand-version {
  font-size: var(--text-2xs);
  color: rgba(255,255,255,0.35);
  margin-top: 20px;
  letter-spacing: 0.03em;
}

.theme-toggle {
  position: absolute;
  bottom: 24px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(255,255,255,0.08);
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: 100px;
  color: rgba(255,255,255,0.6);
  cursor: pointer;
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}
.theme-toggle:hover {
  background: rgba(255,255,255,0.14);
  color: #fff;
}
.theme-toggle svg {
  width: 18px;
  height: 18px;
}

@media (max-width: 767px) {
  .brand-panel {
    padding: var(--space-6) var(--space-4);
    height: auto;
    min-height: auto;
  }
  .brand-logo {
    width: 56px;
    height: 56px;
    border-radius: 14px;
  }
  .brand-logo svg {
    width: 32px;
    height: 32px;
  }
  .brand-title {
    font-size: 22px;
  }
  .brand-version {
    margin-top: 12px;
  }
  .theme-toggle {
    bottom: 12px;
  }
}
</style>
