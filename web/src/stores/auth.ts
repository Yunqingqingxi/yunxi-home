import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../services/api'

interface UserInfo {
  username: string
  role: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') || '')
  const user = ref<UserInfo | null>(JSON.parse(localStorage.getItem('user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)

  async function login(username: string, password: string): Promise<void> {
    const res = await api.post('/api/auth/login', { username, password })
    token.value = res.data.data.token
    user.value = { username: res.data.data.username, role: res.data.data.role }
    localStorage.setItem('token', token.value)
    localStorage.setItem('user', JSON.stringify(user.value))
    api.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  function logout(): void {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    delete api.defaults.headers.common['Authorization']
  }

  if (token.value) {
    api.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  return { token, user, isLoggedIn, login, logout }
})
