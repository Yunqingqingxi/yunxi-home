import axios from 'axios'

const api = axios.create({
  baseURL: '/',
  timeout: 0, // 无全局超时，各接口按需设置
  // Content-Type is set per-request by axios (JSON for objects, multipart for FormData)
})

// Request interceptor: attach auth token from localStorage
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const resp = error.response
    if (resp?.status === 401) {
      const token = localStorage.getItem('token')
      if (!token) return Promise.reject(error)
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      if (window.location.hash !== '#/login' && window.location.hash !== '') {
        window.location.hash = '#/login'
      }
    }
    // Attach structured error_code for callers to handle
    if (resp?.data?.error_code) {
      error.errorCode = resp.data.error_code
      error.serverMessage = resp.data.message
    }
    return Promise.reject(error)
  }
)

export default api