<template>
  <div v-if="tasks.length > 0" class="cron-panel">
    <div class="cron-list">
      <div
        v-for="task in tasks"
        :key="task.id"
        class="cron-tag"
        :title="task.prompt"
      >
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5">
          <circle cx="5" cy="5" r="4"/>
          <path d="M5 2.5v3L7 6.5"/>
        </svg>
        <span class="cron-desc">{{ describeCron(task.cron_expr) }}</span>
        <span class="cron-prompt-text">{{ truncate(task.prompt, 20) }}</span>
        <button class="cron-delete" @click.stop="deleteTask(task.id)" title="删除定时任务">
          <svg width="8" height="8" viewBox="0 0 8 8" fill="none" stroke="currentColor" stroke-width="1.6">
            <path d="M1.5 1.5l5 5M6.5 1.5l-5 5"/>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, onMounted } from 'vue'
import { useChatStore } from '../../stores/chat'

const store = useChatStore()
const tasks = ref([])

function describeCron(cron) {
  const descs = {
    '*/5 * * * *': '每5分钟',
    '*/10 * * * *': '每10分钟',
    '*/30 * * * *': '每30分钟',
    '0 * * * *': '每小时',
    '*/1 * * * *': '每分钟',
  }
  return descs[cron] || cron
}

function truncate(s, n) {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '…' : s
}

async function fetchTasks() {
  if (!store.sessionId) return
  try {
    const token = localStorage.getItem('token')
    const res = await fetch(`/api/cron/tasks?session_id=${store.sessionId}`, {
      headers: { 'Authorization': 'Bearer ' + token }
    })
    const data = await res.json()
    if (data.code === 200) {
      tasks.value = data.data || []
    }
  } catch (e) { /* ignore */ }
}

async function deleteTask(id) {
  try {
    const token = localStorage.getItem('token')
    await fetch(`/api/cron/tasks/${id}`, {
      method: 'DELETE',
      headers: { 'Authorization': 'Bearer ' + token }
    })
    tasks.value = tasks.value.filter(t => t.id !== id)
  } catch (e) { /* ignore */ }
}

// Watch session changes to refetch
import { watch } from 'vue'
watch(() => store.sessionId, () => { fetchTasks() })
onMounted(() => { fetchTasks() })
</script>

<style scoped>
.cron-panel {
  flex-shrink: 0;
  margin-bottom: 6px;
}
.cron-list {
  display: flex; flex-wrap: wrap; gap: 4px;
}
.cron-tag {
  display: flex; align-items: center; gap: 4px;
  padding: 3px 8px;
  border-radius: 100px;
  background: rgba(6,182,212,0.06);
  border: 1px solid rgba(6,182,212,0.14);
  font-size: 10.5px; color: var(--brand-600);
  cursor: default;
  transition: all 0.2s;
}
.cron-tag:hover {
  background: rgba(6,182,212,0.1);
  border-color: rgba(6,182,212,0.25);
}
.cron-desc {
  font-weight: var(--weight-medium);
  white-space: nowrap;
}
.cron-prompt-text {
  color: var(--text-muted);
  white-space: nowrap;
  max-width: 120px; overflow: hidden; text-overflow: ellipsis;
}
.cron-delete {
  display: flex; align-items: center; justify-content: center;
  width: 14px; height: 14px; border-radius: 50%; border: none;
  background: transparent; color: var(--text-muted);
  cursor: pointer; opacity: 0; transition: all 0.15s;
}
.cron-tag:hover .cron-delete { opacity: 1; }
.cron-delete:hover { background: rgba(239,68,68,0.12); color: var(--color-danger); }

[data-theme="dark"] .cron-tag {
  background: rgba(34,211,238,0.08);
  border-color: rgba(34,211,238,0.16);
  color: #22d3ee;
}
</style>
