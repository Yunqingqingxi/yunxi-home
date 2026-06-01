import axios, { AxiosInstance, InternalAxiosRequestConfig, AxiosResponse } from 'axios'

declare module 'axios' {
  interface AxiosError {
    errorCode?: string
    serverMessage?: string
  }
}

const api: AxiosInstance = axios.create({
  baseURL: '/',
  timeout: 0,
})

// Request interceptor: attach auth token from localStorage
api.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = localStorage.getItem('token')
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: any) => {
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
  },
)

export default api
