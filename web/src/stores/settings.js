import { defineStore } from 'pinia'
import { ref, computed, reactive } from 'vue'
import api from '../services/api'

export const useSettingsStore = defineStore('settings', () => {
  const config = ref(null)
  const loading = ref(false)
  const saving = ref(new Set())     // Set of sections currently saving
  const error = ref('')
  const dirtySections = reactive(new Set())  // sections with unsaved changes
  const localValues = reactive({})          // { section: { ...formFields } }
  const testResults = reactive({})          // { providerKey: { ok, error } }

  const hasDirty = computed(() => dirtySections.size > 0)
  const dirtyList = computed(() => [...dirtySections])

  // Section accessors
  const dnsConfig = computed(() => config.value?.alidns || {})
  const notifyConfig = computed(() => config.value?.notify || {})
  const aiConfig = computed(() => config.value?.ai || {})
  const detectConfig = computed(() => config.value?.detect || {})
  const nasConfig = computed(() => config.value?.nas || {})
  const databaseConfig = computed(() => config.value?.database || {})
  const serverConfig = computed(() => config.value?.server || {})
  const logConfig = computed(() => config.value?.log || {})
  const qqbotConfig = computed(() => config.value?.qqbot || {})

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const res = await api.get('/api/config')
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

  function setField(section, path, value) {
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

  function isDirty(section) { return dirtySections.has(section) }

  async function saveSection(section) {
    if (!localValues[section]) return { ok: false, error: '无数据' }
    saving.value.add(section)
    error.value = ''
    try {
      const payload = JSON.parse(JSON.stringify(localValues[section])) // deep clone

      // AI section: only clear keys that were explicitly cleared by user.
      // If has_key was true in the original config and api_key is empty,
      // the user didn't modify it — just omit the field (skip).
      if (section === 'ai') {
        const origAi = config.value?.ai || {}
        for (const [key, val] of Object.entries(payload)) {
          if (typeof val === 'object' && val !== null) {
            if (!val.api_key) {
              if (val._cleared) {
                // User explicitly clicked clear → send clear
                val._clear_key = true
              } else if (origAi[key]?.has_key) {
                // Masked, unchanged → omit api_key, don't clear
                delete val.api_key
              }
              // else: never had a key and still empty → nothing to do
            } else if (val.api_key === '••••••••') {
              delete val.api_key
            }
            delete val._cleared  // clean up internal marker
            delete val.has_key   // clean up read-only flag
          }
        }
      }
      // Add _clear_secret markers for empty key fields
      if (section === 'dns' && payload.aliyun) {
        if (!payload.aliyun.access_key_secret) {
          if (payload.aliyun._cleared) {
            payload.aliyun._clear_secret = true
          } else if (config.value?.dns?.aliyun?.has_secret) {
            delete payload.aliyun.access_key_secret  // masked, skip
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
            delete payload.email.password  // masked, skip
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
            delete payload.app_secret  // masked, skip
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
            delete payload.mysql.password  // masked, skip
          }
        }
        delete payload.mysql._cleared
        delete payload.mysql.has_password
      }

      const res = await api.put('/api/config/' + section, payload)
      if (res.data.code === 200) {
        config.value[section] = JSON.parse(JSON.stringify(payload))
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

  async function testProvider(key) {
    if (!localValues.ai?.[key]) return { ok: false, error: '无配置' }
    try {
      const res = await api.post('/api/config/ai/test', { [key]: localValues.ai[key] })
      const tests = res.data?.data?.tests || res.data?.tests || {}
      const t = tests[key]
      testResults[key] = t ? { ok: t.enabled, error: t.error || '' } : { ok: false, error: '无响应' }
      return testResults[key]
    } catch (e) {
      testResults[key] = { ok: false, error: '测试请求失败' }
      return testResults[key]
    }
  }

  async function saveAll() {
    const sections = [...dirtySections]
    const results = []
    for (const s of sections) {
      results.push(await saveSection(s))
    }
    return results
  }

  function resetSection(section) {
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
