import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const STORAGE_KEY = 'yunxi-theme'

type Theme = 'light' | 'dark'

function getSystemPreference(): Theme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function getStored(): Theme | null {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'dark' || v === 'light') return v
  } catch (_) { /* noop */ }
  return null
}

export const useThemeStore = defineStore('theme', () => {
  const stored = getStored()
  const theme = ref<Theme>(stored || getSystemPreference())

  function apply(themeValue: Theme): void {
    document.documentElement.setAttribute('data-theme', themeValue)
  }

  function toggle(): void {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
  }

  function set(newTheme: Theme): void {
    if (newTheme !== 'light' && newTheme !== 'dark') return
    theme.value = newTheme
  }

  watch(theme, (val: Theme) => {
    apply(val)
    try { localStorage.setItem(STORAGE_KEY, val) } catch (_) { /* noop */ }
  }, { immediate: true })

  if (typeof window !== 'undefined') {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e: MediaQueryListEvent) => {
      if (!getStored()) {
        theme.value = e.matches ? 'dark' : 'light'
      }
    })
  }

  return { theme, toggle, set }
})
