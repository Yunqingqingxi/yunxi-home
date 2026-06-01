<template>
  <div class="sysctl-page">
    <h3>系统控制</h3>

    <div class="tab-bar">
      <button
        :class="['tab', { active: tab === 'docker' }]"
        @click="tab = 'docker'"
      >
        Docker 容器
      </button>
      <button
        :class="['tab', { active: tab === 'process' }]"
        @click="tab = 'process'"
      >
        进程管理
      </button>
      <button
        :class="['tab', { active: tab === 'service' }]"
        @click="tab = 'service'"
      >
        系统服务
      </button>
    </div>

    <div
      v-if="tab === 'docker'"
      class="card-bento"
      style="padding: 0"
    >
      <div class="card-head">
        <h4>容器列表</h4>
        <a-button
          size="small"
          @click="loadContainers"
        >
          刷新
        </a-button>
      </div>
      <div
        v-if="containers.length"
        class="row-table"
      >
        <div
          v-for="c in containers"
          :key="c.id"
          class="row-item"
        >
          <span class="col-name">{{ c.name }}</span>
          <span class="col-image">{{ c.image }}</span>
          <a-tag
            :color="c.state === 'running' ? 'green' : 'red'"
            size="small"
          >
            {{ c.state }}
          </a-tag>
          <span
            v-if="c.ports"
            class="col-ports"
          >{{ c.ports }}</span>
          <span class="col-actions">
            <a-button
              v-if="c.state !== 'running'"
              size="mini"
              @click="containerAction(c.name, 'start')"
            >启动</a-button>
            <a-button
              v-if="c.state === 'running'"
              size="mini"
              @click="containerAction(c.name, 'stop')"
            >停止</a-button>
            <a-button
              size="mini"
              @click="containerAction(c.name, 'restart')"
            >重启</a-button>
            <a-button
              size="mini"
              @click="viewLogs(c)"
            >日志</a-button>
          </span>
        </div>
      </div>
      <div
        v-else
        class="empty-state"
      >
        暂无容器
      </div>
    </div>

    <div
      v-if="tab === 'process'"
      class="card-bento"
      style="padding: 0"
    >
      <div class="card-head">
        <h4>进程列表</h4><a-button
          size="small"
          @click="loadProcesses"
        >
          刷新
        </a-button>
      </div>
      <div
        v-if="processes.length"
        class="row-table"
      >
        <div
          v-for="p in processes"
          :key="p.pid"
          class="row-item"
        >
          <span class="col-pid">{{ p.pid }}</span><span class="col-name">{{ p.name || p.command }}</span>
          <span
            v-if="p.cpu !== undefined"
            class="col-cpu"
          >{{ p.cpu }}% CPU</span>
          <span
            v-if="p.mem_pct !== undefined"
            class="col-mem"
          >{{ p.mem_pct }}% MEM</span>
          <span
            v-else-if="p.mem_mb"
            class="col-mem"
          >{{ p.mem_mb.toFixed(1) }} MB</span>
          <span
            v-if="p.user"
            class="col-user"
          >{{ p.user }}</span>
        </div>
      </div>
      <div
        v-else
        class="empty-state"
      >
        暂无进程数据
      </div>
    </div>

    <div
      v-if="tab === 'service'"
      class="card-bento"
      style="padding: 0"
    >
      <div class="card-head">
        <h4>服务列表</h4><a-button
          size="small"
          @click="loadServices"
        >
          刷新
        </a-button>
      </div>
      <div
        v-if="services.length"
        class="row-table"
      >
        <div
          v-for="s in services"
          :key="s.name"
          class="row-item"
        >
          <span class="col-name">{{ s.name }}</span>
          <a-tag
            :color="s.status === 'active' ? 'green' : s.status === 'failed' ? 'red' : 'orange'"
            size="small"
          >
            {{ s.status }}
          </a-tag>
          <span class="col-actions">
            <a-button
              size="mini"
              @click="serviceAction(s.name, 'start')"
            >启动</a-button>
            <a-button
              size="mini"
              @click="serviceAction(s.name, 'stop')"
            >停止</a-button>
            <a-button
              size="mini"
              @click="serviceAction(s.name, 'restart')"
            >重启</a-button>
          </span>
        </div>
      </div>
      <div
        v-else
        class="empty-state"
      >
        暂无服务数据 (仅 Linux systemd)
      </div>
    </div>

    <div
      v-if="showLogs"
      class="modal-overlay"
      @click.self="showLogs = false"
    >
      <div class="modal-card wide">
        <div class="log-head">
          <h4>{{ logsContainer }} 日志</h4><a-button
            size="small"
            @click="showLogs = false"
          >
            关闭
          </a-button>
        </div>
        <pre class="log-content">{{ logsText || '加载中...' }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import api from '../services/api'

const tab = ref('docker')
const containers = ref([])
const processes = ref([])
const services = ref([])
const showLogs = ref(false)
const logsContainer = ref('')
const logsText = ref('')

async function loadContainers() { try { const res = await api.get('/api/docker/containers', { params: { all: 'true' } }); containers.value = res.data.data || [] } catch (e) { containers.value = [] } }
async function containerAction(name, action) {
  try { await api.post('/api/docker/containers/' + name + '/' + action); Message.success(`容器 ${name} ${action} 成功`); loadContainers() }
  catch (e) { Message.error(`容器 ${name} ${action} 失败: ` + (e.response?.data?.message || e.message)) }
}
async function viewLogs(c) { logsContainer.value = c.name; logsText.value = ''; showLogs.value = true; try { const res = await api.get('/api/docker/containers/' + c.name + '/logs', { params: { tail: 200 } }); logsText.value = res.data.data?.logs || '(无日志)' } catch (e) { logsText.value = '获取日志失败' } }
async function loadProcesses() { try { const res = await api.get('/api/sysctl/processes', { params: { limit: 100 } }); processes.value = res.data.data || [] } catch (e) { processes.value = [] } }
async function loadServices() { try { const res = await api.get('/api/sysctl/services'); services.value = res.data.data || [] } catch (e) { services.value = [] } }
async function serviceAction(name, action) {
  try { await api.post('/api/sysctl/services/' + name + '/' + action); Message.success(`服务 ${name} ${action} 成功`); loadServices() }
  catch (e) { Message.error(`服务 ${name} ${action} 失败: ` + (e.response?.data?.message || e.message)) }
}
onMounted(() => { loadContainers() })
</script>

<style scoped>
.sysctl-page { display: flex; flex-direction: column; gap: 12px; }
.sysctl-page h3 { margin: 0; font-size: var(--text-xl); font-weight: var(--weight-bold); color: var(--text-primary); }
.card-head { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border-subtle); }
.card-head h4 { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); color: var(--text-primary); }
.row-table { display: flex; flex-direction: column; }
.row-item { display: flex; align-items: center; gap: 10px; padding: 9px 16px; border-bottom: 1px solid var(--border-subtle); font-size: var(--text-sm); transition: background 0.1s; }
.row-item:last-child { border-bottom: none; }
.row-item:hover { background: rgba(0,0,0,0.02); }
.col-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-primary); font-family: var(--font-mono); font-size: var(--text-xs); }
.col-image { width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-muted); font-size: var(--text-xs); }
.col-pid { width: 56px; color: var(--text-muted); font-family: var(--font-mono); font-size: var(--text-xs); }
.col-cpu { width: 72px; text-align: right; font-size: var(--text-xs); color: var(--text-muted); }
.col-mem { width: 80px; text-align: right; font-size: var(--text-xs); color: var(--text-muted); }
.col-user { width: 72px; font-size: var(--text-xs); color: var(--text-muted); }
.col-ports { width: 100px; font-size: var(--text-xs); color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.col-actions { display: flex; gap: 4px; flex-shrink: 0; }
.log-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.log-head h4 { margin: 0; font-size: var(--text-md); font-weight: var(--weight-semibold); }
.log-content { background: var(--code-block-bg); color: var(--code-block-color); padding: 12px; border-radius: var(--radius-sm); font-family: var(--font-mono); font-size: 11px; white-space: pre-wrap; overflow: auto; max-height: 50vh; margin: 0; }
@media (max-width: 767px) {
  .sysctl-page { gap: 8px; }
  .col-image, .col-ports, .col-user { display: none; }
  .col-cpu { width: 54px; font-size: 11px; }
  .col-mem { width: 58px; font-size: 11px; }
  .row-item { padding: 7px 12px; gap: 6px; }
  .col-actions { flex-wrap: wrap; gap: 2px; }
  .card-head { padding: 8px 12px; }
  .modal-card.wide { width: 95%; max-width: 95vw; padding: 12px; }
  .log-content { max-height: 40vh; font-size: 10px; }
}
</style>