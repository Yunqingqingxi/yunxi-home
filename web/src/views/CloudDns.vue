<template>
  <div class="cloud-page">
    <PageHeader title="云解析记录">
      <template #actions>
        <div class="header-controls">
          <a-select v-model="selectedDomain" placeholder="选择域名" style="width:220px" @change="loadRecords" allow-search>
            <a-option v-for="d in domains" :key="d.domain_name" :value="d.domain_name">{{ d.domain_name }} ({{ d.record_count }}条)</a-option>
          </a-select>
          <a-button size="small" @click="loadDomains" :loading="loadingDomains">刷新</a-button>
          <a-button type="primary" size="small" @click="openAdd" v-if="selectedDomain">添加记录</a-button>
        </div>
      </template>
    </PageHeader>

    <a-alert v-if="error" type="error" closable @close="error=''">{{ error }}</a-alert>
    <a-alert v-if="msg" type="success" closable @close="msg=''">{{ msg }}</a-alert>

    <a-table v-if="selectedDomain" :data="records" :columns="columns" :pagination="pagination" row-key="RecordId" size="small" :loading="loading" @page-change="onPageChange">
      <template #Type="{ record }">
        <a-tag :color="typeColor(record.Type)" size="small">{{ record.Type }}</a-tag>
      </template>
      <template #Status="{ record }">
        <a-tag :color="record.Status === 'Enable' ? 'green' : 'red'" size="small">{{ record.Status === 'Enable' ? '启用' : '停用' }}</a-tag>
      </template>
      <template #actions="{ record }">
        <a-space>
          <a-button size="mini" @click="openEdit(record)">编辑</a-button>
          <a-popconfirm content="确认删除？" @ok="doDelete(record.RecordId)">
            <a-button size="mini" status="danger">删除</a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </a-table>
    <a-empty v-if="selectedDomain && !loading && !records.length" description="暂无记录" />
    <a-empty v-if="!selectedDomain" description="请先选择一个域名" />

    <a-modal v-model:visible="modalVisible" :title="editing ? '编辑记录' : '添加记录'" @ok="doSave" :ok-loading="saving" width="460px">
      <a-form :model="form" layout="vertical" size="medium">
        <a-form-item field="domain" label="域名">
          <a-input :model-value="selectedDomain" disabled />
        </a-form-item>
        <a-form-item field="rr" label="主机记录 (RR)" required>
          <a-input v-model="form.rr" placeholder="@ 或 www" :disabled="!!editing" />
        </a-form-item>
        <a-form-item field="type" label="记录类型" required>
          <a-select v-model="form.type" :disabled="!!editing">
            <a-option v-for="t in recordTypes" :key="t" :value="t">{{ t }}</a-option>
          </a-select>
        </a-form-item>
        <a-form-item field="value" label="记录值" required>
          <a-input v-model="form.value" :placeholder="typePlaceholder" />
        </a-form-item>
        <a-form-item field="ttl" label="TTL">
          <a-select v-model="form.ttl">
            <a-option :value="60">60s</a-option>
            <a-option :value="120">120s</a-option>
            <a-option :value="300">300s</a-option>
            <a-option :value="600">600s</a-option>
            <a-option :value="1800">1800s</a-option>
            <a-option :value="3600">3600s</a-option>
            <a-option :value="86400">86400s</a-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../services/api'
import PageHeader from '../components/ui/PageHeader.vue'

const recordTypes = ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA']
const columns = [
  { title: '类型', slotName: 'Type', width: 80 },
  { title: '主机记录', dataIndex: 'RR', width: 120 },
  { title: '记录值', dataIndex: 'Value' },
  { title: 'TTL', dataIndex: 'TTL', width: 80 },
  { title: '线路', dataIndex: 'Line', width: 90 },
  { title: '状态', slotName: 'Status', width: 70 },
  { title: '操作', slotName: 'actions', width: 140 },
]

function typeColor(type) {
  if (typeof document === 'undefined') return '#909399'
  const style = getComputedStyle(document.documentElement)
  const m = {
    A: style.getPropertyValue('--brand-500').trim() || '#06b6d4',
    AAAA: style.getPropertyValue('--color-success').trim() || '#16a34a',
    CNAME: style.getPropertyValue('--color-warning').trim() || '#d97706',
    MX: style.getPropertyValue('--color-danger').trim() || '#dc2626',
    TXT: style.getPropertyValue('--text-muted').trim() || '#94a3b8',
    NS: style.getPropertyValue('--thinking-color').trim() || '#818cf8',
    SRV: style.getPropertyValue('--color-warning').trim() || '#d97706',
    CAA: style.getPropertyValue('--brand-400').trim() || '#22d3ee',
  }
  return m[type] || style.getPropertyValue('--text-muted').trim() || '#94a3b8'
}

const domains = ref([])
const selectedDomain = ref('')
const records = ref([])
const loading = ref(false)
const loadingDomains = ref(false)
const error = ref('')
const msg = ref('')
const pagination = ref({ current:1, pageSize:50, total:0, showTotal:true })
const modalVisible = ref(false)
const saving = ref(false)
const editing = ref(null)
const defaultForm = { rr: '@', type: 'A', value: '', ttl: 600 }
const form = ref({ ...defaultForm })

const typePlaceholder = computed(() => {
  const p = { A:'192.168.1.1', AAAA:'2409::1', CNAME:'example.com', MX:'mail.example.com', TXT:'some text', NS:'ns1.example.com', SRV:'0 5 5060 sip.example.com', CAA:'0 issue "ca.example.com"' }
  return p[form.value.type] || '记录值'
})

function resetForm() { form.value = { ...defaultForm } }
function openAdd() { editing.value = null; resetForm(); modalVisible.value = true }
function openEdit(rec) {
  editing.value = rec
  form.value = { rr: rec.RR, type: rec.Type, value: rec.Value, ttl: rec.TTL }
  modalVisible.value = true
}

async function doSave() {
  if (!form.value.rr || !form.value.type || !form.value.value) { error.value = '请填写必填项'; return }
  saving.value = true
  try {
    if (editing.value) {
      await api.put('/api/domains/cloud/records/' + editing.value.RecordId, form.value)
      msg.value = '记录已更新'
    } else {
      await api.post('/api/domains/cloud/records', { ...form.value, domain: selectedDomain.value })
      msg.value = '记录已创建'
    }
    modalVisible.value = false
    await loadRecords()
  } catch(e) { error.value = e.response?.data?.message || '操作失败' }
  finally { saving.value = false }
}

async function doDelete(recordId) {
  try { await api.delete('/api/domains/cloud/records/' + recordId); msg.value='已删除'; await loadRecords() }
  catch(e) { error.value = '删除失败' }
}

async function loadDomains() {
  loadingDomains.value = true
  try { const res = await api.get('/api/domains/cloud', { params: { size: 50 } }); domains.value = res.data.data?.domains || [] }
  catch(e) { error.value = '获取域名列表失败' }
  finally { loadingDomains.value = false }
}

async function loadRecords() {
  if (!selectedDomain.value) return
  loading.value = true
  try {
    const res = await api.get('/api/domains/cloud/records', { params: { domain: selectedDomain.value, page: pagination.value.current, size: pagination.value.pageSize } })
    const d = res.data.data; records.value = d?.records || []; pagination.value.total = d?.total || 0
  } catch(e) { error.value = '获取记录失败' }
  finally { loading.value = false }
}

function onPageChange(page) { pagination.value.current = page; loadRecords() }

onMounted(loadDomains)
</script>

<style scoped>
.cloud-page { display: flex; flex-direction: column; gap: var(--space-4); }
.header-controls { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-2); }
:deep(.arco-table-td-content) { white-space: nowrap; }
:deep(.arco-select-view-single) { border-radius: var(--radius-md) !important; }

@media (max-width: 767px) {
  .cloud-page :deep(.arco-table) {
    min-width: 600px;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
  .cloud-page :deep(.arco-space) { flex-wrap: wrap; gap: 4px; }
  .cloud-page :deep(.arco-btn-mini) { padding: 0 6px !important; }

  .header-controls {
    width: 100%;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
  .header-controls :deep(.arco-select-view) {
    width: 100% !important;
  }

  /* Modal: full-width */
  .cloud-page :deep(.arco-modal) {
    width: 92vw !important;
  }
  .cloud-page :deep(.arco-modal-body .arco-form-item .arco-input-wrapper),
  .cloud-page :deep(.arco-modal-body .arco-form-item .arco-select-view) {
    width: 100% !important;
  }

  /* Action buttons in table */
  .cloud-page :deep(.arco-table-td:last-child .arco-space) {
    gap: 2px;
  }
}
</style>