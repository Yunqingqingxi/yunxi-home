import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import api from '../services/api'
import type { ApiResponse } from '../types/api'
import type { AppConfig, AliyunDNSConfig, NotifyConfig, AIConfig, DetectConfig, NASConfig, DatabaseConfig, ServerConfig, LogConfig, QQBotConfig } from '../types/config'

interface TestResult {
  ok: boolean
  error: string
}

interface SaveResult {
  ok: boolean
  data?: any
  error?: string
}

export const useSettingsStore = defineStore('settings', () => {
  const config = ref<AppConfig | null>(null)
  const loading = ref(false)
  const saving = ref(new Set<string>())     // Set of sections currently saving
  const error = ref('')
  const dirtySections = reactive(new Set<string>())  // sections with unsaved changes
  const localValues = reactive<Record<string, Record<string, any>>>({})          // { section: { ...formFields } }
  const testResults = reactive<Record<string, TestResult>>({})          // { providerKey: { ok, error } }

  const hasDirty = computed(() => dirtySections.size > 0)
  const dirtyList = computed(() => [...dirtySections])

  // Section accessors
  const dnsConfig = computed<AliyunDNSConfig>(() => config.value?.alidns || {})
  const notifyConfig = computed<NotifyConfig>(() => config.value?.notify || {})
  const aiConfig = computed<AIConfig>(() => config.value?.ai || {})
  const detectConfig = computed<DetectConfig>(() => config.value?.detect || {})
  const nasConfig = computed<NASConfig>(() => config.value?.nas || {})
  const databaseConfig = computed<DatabaseConfig>(() => config.value?.database || {})
  const serverConfig = computed<ServerConfig>(() => config.value?.server || {})
  const logConfig = computed<LogConfig>(() => config.value?.log || {})
  const qqbotConfig = computed<QQBotConfig>(() => config.value?.qqbot || {})

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const res = await api.get<ApiResponse<AppConfig>>('/api/config')
      if (res.data.code === 200) {
        config.value = res.data.data || {}
        // Init localValues from config
        for (const [section, data] of Object.entries(config.value)) {
          if (typeof data === 'object' && data !== null) {
            localValues[section] = { ...data }
          }
        }
        dirtySections.clear()
      } else {
        error.value = res.data.message || '加载配置失败'
      }
    } catch (e) {
      error.value = '网络错误'
    } finally {
      loading.value = false
    }
  }

  function setField(section: string, path: string, value: any): void {
    if (!localValues[section]) localValues[section] = { ...(config.value?.[section] || {}) }
    const keys = path.split('.')
    let obj = localValues[section]
    for (let i = 0; i < keys.length - 1; i++) {
      if (!obj[keys[i]]) obj[keys[i]] = {}
      obj = obj[keys[i]]
    }
    obj[keys[keys.length - 1]] = value
    dirtySections.add(section)
  }

  function isDirty(section: string): boolean { return dirtySections.has(section) }

  async function saveSection(section: string): Promise<SaveResult> {
    if (!localValues[section]) return { ok: false, error: '无数据' }
    saving.value.add(section)
    error.value = ''
    try {
      const payload: Record<string, any> = JSON.parse(JSON.stringify(localValues[section])) // deep clone

      // AI section: only clear keys that were explicitly cleared by user.
      if (section === 'ai') {
        const origAi = config.value?.ai || {}
        for (const [key, val] of Object.entries(payload)) {
          if (typeof val === 'object' && val !== null) {
            if (!val.api_key) {
              if (val._cleared) {
                val._clear_key = true
              } else if (typeof origAi[key] === 'object' && (origAi[key] as any)?.has_key) {
                delete val.api_key
              }
            } else if (val.api_key === '••••••••') {
              delete val.api_key
            }
            delete val._cleared
            delete val.has_key
          }
        }
      }
      // Add _clear_secret markers for empty key fields
      if (section === 'dns' && payload.aliyun) {
        if (!payload.aliyun.access_key_secret) {
          if (payload.aliyun._cleared) {
            payload.aliyun._clear_secret = true
          } else if (config.value?.dns?.aliyun?.has_secret) {
            delete payload.aliyun.access_key_secret
          }
        }
        delete payload.aliyun._cleared
        delete payload.aliyun.has_secret
      }
      if (section === 'notify' && payload.email) {
        if (!payload.email.password) {
          if (payload.email._cleared) {
            payload.email._clear_password = true
          } else if (config.value?.notify?.email?.has_password) {
            delete payload.email.password
          }
        }
        delete payload.email._cleared
        delete payload.email.has_password
      }
      if (section === 'qqbot') {
        if (!payload.app_secret) {
          if (payload._cleared) {
            payload._clear_secret = true
          } else if (config.value?.qqbot?.has_secret) {
            delete payload.app_secret
          }
        }
        delete payload._cleared
        delete payload.has_secret
      }
      if (section === 'database' && payload.mysql) {
        if (!payload.mysql.password) {
          if (payload.mysql._cleared) {
            payload.mysql._clear_password = true
          } else if (config.value?.database?.mysql?.has_password) {
            delete payload.mysql.password
          }
        }
        delete payload.mysql._cleared
        delete payload.mysql.has_password
      }

      const res = await api.put<ApiResponse<any>>('/api/config/' + section, payload)
      if (res.data.code === 200) {
        if (config.value) {
          config.value[section] = JSON.parse(JSON.stringify(payload))
        }
        dirtySections.delete(section)
        return { ok: true, data: res.data.data }
      }
      error.value = res.data.message || '保存失败'
      return { ok: false, error: error.value }
    } catch (e) {
      error.value = '网络错误'
      return { ok: false, error: '网络错误' }
    } finally {
      saving.value.delete(section)
    }
  }

  async function testProvider(key: string): Promise<TestResult> {
    if (!localValues.ai?.[key]) return { ok: false, error: '无配置' }
    try {
      const res = await api.post<ApiResponse<any>>('/api/config/ai/test', { [key]: localValues.ai[key] })
      const tests = res.data?.data?.tests || {}
      const t = tests[key]
      testResults[key] = t ? { ok: t.enabled, error: t.error || '' } : { ok: false, error: '无响应' }
      return testResults[key]
    } catch (e) {
      testResults[key] = { ok: false, error: '测试请求失败' }
      return testResults[key]
    }
  }

  async function saveAll(): Promise<SaveResult[]> {
    const sections = [...dirtySections]
    const results: SaveResult[] = []
    for (const s of sections) {
      results.push(await saveSection(s))
    }
    return results
  }

  function resetSection(section: string): void {
    if (config.value?.[section]) {
      localValues[section] = { ...config.value[section] }
    }
    dirtySections.delete(section)
  }

  return {
    config, loading, saving, error, hasDirty, dirtyList, localValues, testResults,
    dnsConfig, notifyConfig, aiConfig, detectConfig, nasConfig,
    databaseConfig, serverConfig, logConfig, qqbotConfig,
    load, setField, isDirty, saveSection, saveAll, resetSection, testProvider,
  }
})
