<template>
  <div class="chat-session-list">
    <div class="panel-head">会话列表</div>

    <!-- Search -->
    <div class="list-search">
      <input
        v-model="searchQuery"
        placeholder="搜索会话..."
        class="search-input"
      />
    </div>

    <!-- Date filter -->
    <div v-if="dateOptions.length > 1" class="date-filter">
      <select v-model="selectedDate" class="date-select">
        <option value="">全部日期</option>
        <option v-for="d in dateOptions" :key="d" :value="d">{{ d }}</option>
      </select>
    </div>

    <!-- Content -->
    <div v-if="loading" class="empty-list">加载中...</div>
    <div v-else-if="!filteredSessions.length" class="empty-list">暂无日志</div>
    <div v-else class="session-list">
      <template v-for="item in sessionsByDate" :key="item._date || item.session_id">
        <div v-if="item._date" class="session-date-head">{{ item._date }}</div>
        <div
          v-else
          :class="['session-item', { active: selected === item.session_id }]"
          @click="$emit('select', item)"
        >
          <button class="item-delete-btn" title="删除" @click.stop="$emit('delete', item.session_id)">✕</button>
          <div class="si-title">{{ truncate(item.session_id) }}</div>
          <div class="si-meta">
            <span class="si-rounds">{{ item.rounds || '-' }} 轮</span>
            <span class="si-size">{{ fmtSize(item.size) }}</span>
            <span v-if="item.active" class="si-live">● LIVE</span>
          </div>
          <div class="si-date">{{ fmtDate(item.created) }}</div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SessionInfo } from '../../types/logs'
import { fmtSize, fmtDate } from '../../stores/logs'

const props = defineProps<{ sessions: SessionInfo[]; selected: string | null; loading: boolean }>()
defineEmits<{ select: [session: SessionInfo]; delete: [sessionId: string] }>()

const searchQuery = ref('')
const selectedDate = ref('')

const dateOptions = computed(() => {
  const dates = new Set<string>()
  for (const s of props.sessions) dates.add(fmtDate(s.created))
  return [...dates].sort().reverse()
})

const filteredSessions = computed(() => {
  let list = [...props.sessions].sort((a, b) => {
    if (a.active !== b.active) return a.active ? -1 : 1
    return new Date(b.created).getTime() - new Date(a.created).getTime()
  })
  if (selectedDate.value) list = list.filter(s => fmtDate(s.created) === selectedDate.value)
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(s => s.session_id.toLowerCase().includes(q))
  }
  return list
})

const sessionsByDate = computed(() => {
  const groups: any[] = []
  let lastDate = ''
  for (const s of filteredSessions.value) {
    const d = fmtDate(s.created)
    if (d !== lastDate) { groups.push({ _date: d }); lastDate = d }
    groups.push(s)
  }
  return groups
})

function truncate(id: string): string {
  if (!id || id.length <= 28) return id
  return id.slice(0, 20) + '...' + id.slice(-4)
}
</script>

<style scoped>
.chat-session-list {
  width: 220px; flex-shrink: 0; display: flex; flex-direction: column;
  border: 1px solid var(--border-default); border-radius: var(--radius-lg);
  overflow: hidden; background: var(--glass-bg-card);
}
.panel-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; font-size: 13px; font-weight: 600;
  color: var(--text-primary); border-bottom: 1px solid var(--border-subtle); flex-shrink: 0;
}
.list-search { padding: 6px 10px; border-bottom: 1px solid var(--border-subtle); }
.search-input {
  width: 100%; padding: 4px 8px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-primary); font-size: 11px; font-family: inherit; outline: none;
  box-sizing: border-box;
}
.date-filter { padding: 5px 10px; border-bottom: 1px solid var(--border-subtle); }
.date-select {
  width: 100%; padding: 3px 6px; border: 1px solid var(--border-default); border-radius: 4px;
  background: transparent; color: var(--text-primary); font-size: 10.5px; font-family: inherit; outline: none;
}
.empty-list {
  flex: 1; display: flex; align-items: center; justify-content: center;
  color: var(--text-muted); font-size: 13px; padding: 40px;
}
.session-list { flex: 1; overflow-y: auto; }
.session-item {
  padding: 10px 14px; border-bottom: 1px solid var(--border-subtle);
  cursor: pointer; transition: background 0.1s; position: relative;
}
.session-item .item-delete-btn {
  position: absolute; top: 6px; right: 6px; width: 20px; height: 20px;
  border: none; background: transparent; color: var(--text-muted); cursor: pointer;
  font-size: 13px; border-radius: 4px; display: flex; align-items: center; justify-content: center;
  opacity: 0; transition: opacity 0.15s, color 0.15s, background 0.15s;
}
.session-item:hover .item-delete-btn { opacity: 1; }
.session-item .item-delete-btn:hover { color: var(--color-danger); background: rgba(239,68,68,0.1); }
.session-item:hover { background: var(--surface-hover); }
.session-item.active { background: color-mix(in srgb, var(--brand-500) 8%, transparent); }
.si-title { font-size: 12px; font-family: var(--font-mono); color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.si-meta { display: flex; gap: 8px; margin-top: 4px; font-size: 11px; color: var(--text-muted); }
.si-live { color: var(--brand-500); font-weight: 600; }
.si-date { font-size: 10px; color: var(--text-muted); margin-top: 2px; }
.session-date-head {
  padding: 5px 14px; font-size: 10.5px; font-weight: 600; color: var(--brand-600);
  background: var(--brand-50); border-bottom: 1px solid var(--border-subtle);
  position: sticky; top: 0; z-index: 1;
}
[data-theme="dark"] .session-date-head { background: rgba(34,211,238,0.08); color: #67e8f9; }
</style>
