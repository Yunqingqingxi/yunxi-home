<template>
  <div class="dns-page">
    <div class="dns-header">
      <h3>DNS 管理</h3>
      <div class="dns-actions">
        <a-button
          size="small"
          :loading="triggering"
          @click="doTrigger"
        >
          触发更新
        </a-button>
        <a-button
          type="primary"
          size="small"
          @click="openAdd"
        >
          添加记录
        </a-button>
      </div>
    </div>

    <!-- Quick stat strip -->
    <div
      v-if="records.length || history.length"
      class="stat-strip"
    >
      <div class="strip-item">
        记录 <strong>{{ records.length }}</strong>
      </div>
      <div class="strip-item">
        IPv6 <strong>{{ records.filter(r=>r.type==='AAAA').length }}</strong>
      </div>
      <div class="strip-item">
        已启用 <strong>{{ records.filter(r=>r.enabled).length }}</strong>
      </div>
      <div class="strip-item">
        更新 <strong>{{ historyPagination.total }}</strong> 次
      </div>
    </div>

    <!-- Tabs -->
    <div class="tab-bar">
      <button
        :class="['tab', { active: activeTab === 'records' }]"
        @click="activeTab = 'records'"
      >
        域名记录
      </button>
      <button
        :class="['tab', { active: activeTab === 'history' }]"
        @click="activeTab = 'history'"
      >
        更新历史
      </button>
    </div>

    <!-- Records tab -->
    <Transition
      :name="slideDirection"
      mode="out-in"
    >
      <div
        v-if="activeTab === 'records'"
        v-show="records.length || !loading"
        key="records"
        class="table-card"
      >
        <table
          v-if="records.length"
          class="native-table"
        >
          <thead>
            <tr>
              <th>类型</th><th>域名</th><th>RR</th><th>记录值</th><th class="col-hide-mobile">
                TTL
              </th><th class="col-hide-mobile">
                频率
              </th><th>状态</th><th>更新时间</th><th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="r in records"
              :key="r.id"
            >
              <td>
                <span
                  class="tag"
                  :class="r.type==='AAAA'?'tag-green':'tag-blue'"
                >{{ r.type }}</span>
              </td>
              <td>{{ r.domain }}</td><td>{{ r.rr }}</td>
              <td><code class="cell-code">{{ r.value || '等待更新' }}</code></td>
              <td class="col-hide-mobile">
                {{ r.ttl }}s
              </td>
              <td class="col-hide-mobile">
                {{ formatCron(r.cron_expr) }}
              </td>
              <td>
                <span
                  class="tag"
                  :class="r.enabled?'tag-green':'tag-gray'"
                >{{ r.enabled?'启用':'停用' }}</span>
              </td>
              <td class="text-muted">
                {{ formatTime(r.updated_at) }}
              </td>
              <td class="row-actions">
                <button
                  class="btn-sm"
                  @click="openEdit(r)"
                >
                  编辑
                </button>
                <button
                  class="btn-sm btn-warn"
                  @click="toggleEnable(r)"
                >
                  {{ r.enabled?'停用':'启用' }}
                </button>
                <button
                  class="btn-sm btn-danger"
                  @click="doDelete(r.id)"
                >
                  删除
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <a-empty
          v-if="!loading && !records.length"
          description="暂无记录"
        />
      </div>

      <!-- History tab -->
      <div
        v-if="activeTab === 'history'"
        v-show="history.length || !loadingHistory"
        key="history"
        class="table-card"
      >
        <table
          v-if="history.length"
          class="native-table"
        >
          <thead>
            <tr>
              <th>类型</th><th>域名</th><th>RR</th><th>旧IP</th><th>新IP</th><th>状态</th><th class="col-hide-mobile">
                时间
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="r in history"
              :key="r.id"
            >
              <td>
                <span
                  class="tag"
                  :class="r.type==='AAAA'?'tag-green':'tag-blue'"
                >{{ r.type }}</span>
              </td>
              <td>{{ r.domain }}</td><td>{{ r.rr || '@' }}</td>
              <td><code class="cell-code">{{ r.old_ip || '-' }}</code></td>
              <td><code class="cell-code">{{ r.new_ip || '-' }}</code></td>
              <td>
                <span
                  class="tag"
                  :class="r.status==='success'?'tag-green':'tag-red'"
                >{{ r.status==='success'?'成功':'失败' }}</span>
              </td>
              <td class="text-muted col-hide-mobile">
                {{ formatTime(r.created_at) }}
              </td>
            </tr>
          </tbody>
        </table>
        <a-empty
          v-if="!loadingHistory && !history.length"
          description="暂无更新记录"
        />
        <div
          v-if="historyPagination.total > historyPagination.pageSize"
          class="pagination"
        >
          <button
            :disabled="historyPagination.current <= 1"
            @click="loadHistory(historyPagination.current - 1)"
          >
            上一页
          </button>
          <span>{{ historyPagination.current }} / {{ Math.ceil(historyPagination.total / historyPagination.pageSize) }}</span>
          <button
            :disabled="historyPagination.current >= Math.ceil(historyPagination.total / historyPagination.pageSize)"
            @click="loadHistory(historyPagination.current + 1)"
          >
            下一页
          </button>
        </div>
      </div>
    </Transition>

    <!-- Add/Edit Modal -->
    <a-modal
      v-model:visible="modalVisible"
      :title="editing ? '编辑记录' : '添加记录'"
      :ok-loading="saving"
      :width="modalWidth"
      :simple="isMobile"
      @ok="doSave"
    >
      <a-form
        :model="form"
        layout="vertical"
        size="medium"
      >
        <a-form-item
          field="domain"
          label="域名"
          required
        >
          <a-select
            v-model="form.domain"
            placeholder="选择或输入域名"
            allow-search
            allow-create
            :loading="loadingCloud"
            @popup-visible-change="onDomainPopupChange"
            @change="onDomainChange"
          >
            <a-option
              v-for="d in cloudDomains"
              :key="d.domain_name"
              :value="d.domain_name"
            >
              {{ d.domain_name }} ({{ d.record_count }}条)
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item
          v-if="cloudRecords.length > 0"
          label="已有记录"
        >
          <div class="cloud-records-hint">
            <span
              v-for="r in cloudRecords"
              :key="r.record_id"
              class="cloud-record-tag"
            >{{ r.rr || '@' }} → {{ r.value }} <em>{{ r.type }}</em></span>
          </div>
        </a-form-item>
        <a-form-item
          field="rr"
          label="主机记录 (RR)"
          required
        >
          <a-input
            v-model="form.rr"
            placeholder="@ 或 www"
          />
        </a-form-item>
        <a-form-item
          field="type"
          label="记录类型"
          required
        >
          <a-radio-group v-model="form.type">
            <a-radio value="AAAA">
              AAAA (IPv6)
            </a-radio><a-radio value="A">
              A (IPv4)
            </a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item
          field="ttl"
          label="TTL"
        >
          <a-select v-model="form.ttl">
            <a-option :value="60">
              60s
            </a-option><a-option :value="120">
              120s
            </a-option><a-option :value="300">
              300s
            </a-option><a-option :value="600">
              600s
            </a-option><a-option :value="1800">
              1800s
            </a-option><a-option :value="3600">
              3600s
            </a-option><a-option :value="86400">
              86400s
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item
          field="cron_expr"
          label="检测频率"
        >
          <a-select v-model="form.cron_expr">
            <a-option value="0 */1 * * * *">
              每分钟
            </a-option><a-option value="0 */5 * * * *">
              每5分钟
            </a-option><a-option value="0 */10 * * * *">
              每10分钟
            </a-option><a-option value="0 */30 * * * *">
              每30分钟
            </a-option><a-option value="0 0 * * * *">
              每小时
            </a-option><a-option value="0 0 */6 * * *">
              每6小时
            </a-option><a-option value="0 0 0 * * *">
              每天
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item
          field="enabled"
          label="启用"
        >
          <a-switch v-model="form.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, reactive, onMounted, computed } from 'vue'
import api from '../services/api'
import { useToast } from '../composables/useToast.js'
import { formatTime, formatCron } from '../composables/useFormat.js'

const toast = useToast()
const activeTab = ref('records')
const slideDirection = computed(() => activeTab.value === 'records' ? 'slide-left' : 'slide-right')

// ── Records ──
const records = ref([])
const loading = ref(false)
const triggering = ref(false)
const loadingCloud = ref(false)
const cloudDomains = ref([])
const cloudRecords = ref([])
const modalVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 768)
const modalWidth = computed(() => isMobile.value ? '100%' : '480px')
window.addEventListener('resize', () => { windowWidth.value = window.innerWidth })
const defaultForm = { domain: '', rr: '@', type: 'AAAA', ttl: 600, cron_expr: '0 */5 * * * *', enabled: true }
const form = reactive({ ...defaultForm })
function openAdd() { editing.value = null; Object.assign(form, defaultForm); cloudRecords.value = []; modalVisible.value = true }
function openEdit(rec) { editing.value = rec; Object.assign(form, { domain: rec.domain, rr: rec.rr, type: rec.type, ttl: rec.ttl, cron_expr: rec.cron_expr, enabled: rec.enabled }); modalVisible.value = true }
async function doSave() { if (!form.domain || !form.rr || !form.type) { toast.error('请填写必填字段'); return }; saving.value = true; try { if (editing.value) { await api.put('/api/domains/' + editing.value.id, form); toast.success('已更新') } else { await api.post('/api/domains', form); toast.success('已创建') }; modalVisible.value = false; await loadRecords() } catch (e) { toast.error(e.response?.data?.message || '保存失败') } finally { saving.value = false } }
async function doDelete(id) { try { await api.delete('/api/domains/' + id); toast.success('已删除'); await loadRecords() } catch (e) { toast.error('删除失败') } }
async function toggleEnable(rec) { try { await api.put('/api/domains/' + rec.id, { enabled: !rec.enabled }); toast.success(rec.enabled ? '已停用' : '已启用'); await loadRecords() } catch (e) { toast.error('操作失败') } }
async function loadRecords() { loading.value = true; try { const res = await api.get('/api/domains'); records.value = res.data.data || [] } catch (e) { toast.error('加载失败') } finally { loading.value = false } }
async function onDomainPopupChange(visible) { if (visible) await loadCloudDomains() }
async function onDomainChange(val) { if (val) { try { const res = await api.get("/api/domains/cloud/records", { params: { domain: val, size: 50 } }); cloudRecords.value = res.data.data?.records || [] } catch (e) { cloudRecords.value = [] } } else { cloudRecords.value = [] } }
async function loadCloudDomains() { if (cloudDomains.value.length > 0) return; loadingCloud.value = true; try { const res = await api.get('/api/domains/cloud', { params: { size: 50 } }); cloudDomains.value = res.data.data?.domains || [] } catch (e) {} finally { loadingCloud.value = false } }
async function doTrigger() { triggering.value = true; try { await api.post('/api/trigger'); toast.success('已触发更新'); await loadRecords() } catch (e) { toast.error('触发失败') } finally { triggering.value = false } }

// ── History ──
const history = ref([])
const loadingHistory = ref(false)
const historyPagination = ref({ current: 1, pageSize: 20, total: 0 })
async function loadHistory(page = 1) {
  loadingHistory.value = true; historyPagination.value.current = page
  try { const res = await api.get('/api/history', { params: { page, size: historyPagination.value.pageSize } }); const data = res.data.data; history.value = data?.records || []; historyPagination.value.total = data?.total || 0 }
  catch (e) { toast.error('加载历史失败') }
  finally { loadingHistory.value = false }
}

onMounted(() => { loadRecords(); loadHistory() })
</script>

<style scoped>
.dns-page { display: flex; flex-direction: column; gap: 12px; }
.dns-header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; }
.dns-header h3 { margin: 0; font-size: var(--text-xl); font-weight: var(--weight-bold); color: var(--text-primary); }
.stat-strip { display: flex; gap: 12px; flex-wrap: wrap; }
.strip-item { padding: 6px 14px; font-size: var(--text-xs); color: var(--text-secondary); background: var(--glass-bg-card); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); border: 1px solid var(--glass-border); border-radius: 10px; }
.strip-item strong { color: var(--text-primary); font-weight: var(--weight-semibold); margin-left: 2px; }
.tab-bar { display: flex; gap: 2px; background: rgba(0,0,0,0.03); border-radius: var(--radius-lg); padding: 4px; border: 1px solid var(--glass-border); backdrop-filter: blur(12px); }
.tab { flex: 1; padding: 8px 16px; border: none; border-radius: var(--radius-md); background: transparent; color: var(--text-secondary); cursor: pointer; font-size: var(--text-sm); font-weight: var(--weight-medium); font-family: inherit; transition: all 0.15s var(--ease-out-expo); }
.tab.active { background: rgba(255,255,255,0.55); backdrop-filter: blur(8px); color: var(--text-primary); box-shadow: 0 1px 3px rgba(0,0,0,0.06); font-weight: var(--weight-semibold); }
.tab:hover:not(.active) { color: var(--text-primary); }
[data-theme="dark"] .tab-bar { background: rgba(255,255,255,0.04); }
[data-theme="dark"] .tab.active { background: rgba(255,255,255,0.06); }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; padding: 12px; font-size: var(--text-sm); color: var(--text-secondary); }
.pagination button { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: var(--radius-sm); background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 12px; font-family: inherit; }
.pagination button:disabled { opacity: 0.3; cursor: default; }
.pagination button:hover:not(:disabled) { background: var(--surface-hover); }
@media (max-width: 767px) { .dns-page { gap: 8px; } .stat-strip { gap: 6px; } .strip-item { padding: 5px 10px; } }

/* Tab slide animation */
.slide-left-enter-active, .slide-left-leave-active,
.slide-right-enter-active, .slide-right-leave-active { transition: all 0.25s var(--ease-out-expo); }
.slide-left-enter-from { opacity: 0; transform: translateX(24px); }
.slide-left-leave-to { opacity: 0; transform: translateX(-24px); }
.slide-right-enter-from { opacity: 0; transform: translateX(-24px); }
.slide-right-leave-to { opacity: 0; transform: translateX(24px); }
</style>
