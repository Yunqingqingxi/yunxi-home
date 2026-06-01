import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import ArcoVue from '@arco-design/web-vue'
import '@arco-design/web-vue/dist/arco.css'
import './styles/arco.css'
import './styles/tokens.css'
import './styles/base.css'
import './styles/dark.css'
import App from './App.vue'
import { useThemeStore } from './stores/theme'

const routes = [
  { path: '/', name: 'dashboard', component: () => import('./views/Dashboard.vue'), meta: { requiresAuth: true } },
  { path: '/domains', name: 'domains', component: () => import('./views/Domains.vue'), meta: { requiresAuth: true } },
  { path: '/files', name: 'files', component: () => import('./views/Files.vue'), meta: { requiresAuth: true } },
  { path: '/history', name: 'history', component: () => import('./views/History.vue'), meta: { requiresAuth: true } },
  { path: '/system', name: 'system', component: () => import('./views/System.vue'), meta: { requiresAuth: true } },
  { path: '/terminal', name: 'terminal', component: () => import('./views/Terminal.vue'), meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: () => import('./views/Settings.vue'), meta: { requiresAuth: true } },
  { path: '/market', name: 'market', component: () => import('./views/SkillsMarket.vue'), meta: { requiresAuth: true } },
  { path: '/logs', name: 'logs', component: () => import('./views/Logs.vue'), meta: { requiresAuth: true } },
  { path: '/chat', name: 'chat', component: () => import('./views/Chat.vue'), meta: { requiresAuth: true } },
  { path: '/chat/:sessionId', name: 'chat-session', component: () => import('./views/Chat.vue'), meta: { requiresAuth: true } },
  { path: '/login', name: 'login', component: () => import('./views/Login.vue') },
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) { next('/login') }
  else if (to.path === '/login' && token) { next('/') }
  else { next() }
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(ArcoVue)
app.mount('#app')
useThemeStore()
