<template>
  <div class="history-page">
    <h3>更新历史</h3>
    <div class="stat-strip" v-if="records.length">
      <div class="strip-item">总记录 <strong>{{ pagination.total }}</strong></div>
      <div class="strip-item">成功 <strong>{{ records.filter(r=>r.status==='success').length }}</strong></div>
      <div class="strip-item">最近更新 <strong>{{ formatTime(records[0]?.created_at) }}</strong></div>
    </div>
    <div class="table-card" v-if="records.length">
      <table class="native-table">
        <thead><tr><th>类型</th><th>域名</th><th>RR</th><th>旧IP</th><th>新IP</th><th>状态</th><th class="col-hide-mobile">时间</th></tr></thead>
        <tbody><tr v-for="r in records" :key="r.id">
          <td><span class="tag" :class="r.type==='AAAA'?'tag-green':'tag-blue'">{{ r.type }}</span></td>
          <td>{{ r.domain }}</td><td>{{ r.rr || '@' }}</td>
          <td><code class="cell-code">{{ r.old_ip || '-' }}</code></td>
          <td><code class="cell-code">{{ r.new_ip || '-' }}</code></td>
          <td><span class="tag" :class="r.status==='success'?'tag-green':'tag-red'">{{ r.status==='success'?'成功':'失败' }}</span></td>
          <td class="text-muted col-hide-mobile">{{ formatTime(r.created_at) }}</td>
        </tr></tbody>
      </table>
    </div>
    <a-empty v-if="!loading && !records.length" description="暂无更新记录" />
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted } from 'vue'
import api from '../services/api'
import { useToast } from '../composables/useToast.js'
import { formatTime } from '../composables/useFormat.js'

const toast = useToast()
const records = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 20, total: 0 })

async function load() {
  loading.value = true
  try { const res = await api.get('/api/history', { params: { page: pagination.value.current, size: pagination.value.pageSize } }); const data = res.data.data; records.value = data?.records || []; pagination.value.total = data?.total || 0 }
  catch (e) { toast.error('获取历史记录失败') }
  finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
.history-page { display: flex; flex-direction: column; gap: 12px; }
.history-page h3 { margin: 0; font-size: var(--text-xl); font-weight: var(--weight-bold); color: var(--text-primary); }
.stat-strip { display: flex; gap: 12px; flex-wrap: wrap; }
.strip-item { padding: 6px 14px; font-size: var(--text-xs); color: var(--text-secondary); background: var(--glass-bg-card); backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px); border: 1px solid var(--glass-border); border-radius: 10px; }
.strip-item strong { color: var(--text-primary); font-weight: var(--weight-semibold); margin-left: 2px; }
@media (max-width: 767px) { .history-page { gap: 8px; } .stat-strip { gap: 6px; } .strip-item { padding: 5px 10px; } }
</style>