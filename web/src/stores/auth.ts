import { defineStore } from 'pinia'
import { computed } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import api from '../services/api'

interface UserInfo {
  username: string
  role: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = useLocalStorage<string>('token', '')
  const user = useLocalStorage<UserInfo | null>('user', null, {
    serializer: {
      read: (v: any) => (v ? JSON.parse(v) : null),
      write: (v: any) => JSON.stringify(v),
    },
  })

  const isLoggedIn = computed(() => !!token.value)

  async function login(username: string, password: string): Promise<void> {
    const res = await api.post('/api/auth/login', { username, password })
    token.value = res.data.data.token
    user.value = { username: res.data.data.username, role: res.data.data.role }
    api.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  function logout(): void {
    token.value = ''
    user.value = null
    delete api.defaults.headers.common['Authorization']
  }

  if (token.value) {
    api.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  return { token, user, isLoggedIn, login, logout }
})
