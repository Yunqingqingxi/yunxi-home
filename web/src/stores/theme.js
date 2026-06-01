import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const STORAGE_KEY = 'yunxi-theme'

function getSystemPreference() {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function getStored() {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'dark' || v === 'light') return v
  } catch (_) { /* noop */ }
  return null
}

export const useThemeStore = defineStore('theme', () => {
  const stored = getStored()
  const theme = ref(stored || getSystemPreference())

  function apply(themeValue) {
    document.documentElement.setAttribute('data-theme', themeValue)
  }

  function toggle() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
  }

  function set(newTheme) {
    if (newTheme !== 'light' && newTheme !== 'dark') return
    theme.value = newTheme
  }

  watch(theme, (val) => {
    apply(val)
    try { localStorage.setItem(STORAGE_KEY, val) } catch (_) { /* noop */ }
  }, { immediate: true })

  if (typeof window !== 'undefined') {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
      if (!getStored()) {
        theme.value = e.matches ? 'dark' : 'light'
      }
    })
  }

  return { theme, toggle, set }
})
