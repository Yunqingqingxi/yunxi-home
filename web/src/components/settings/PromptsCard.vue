<template>
  <div class="setting-card full-width">
    <div class="card-head">
      <span class="card-icon">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M2 4h12v8a2 2 0 01-2 2H4a2 2 0 01-2-2V4z"/>
          <path d="M5 2h6v2H5z"/><path d="M5 8h6M5 10h4"/>
        </svg>
      </span>
      <span class="card-title">AI 提示词</span>
      <span class="prompt-count">{{ prompts.length }} 条</span>
      <span v-if="dirtyId" class="dirty-tag">● 未保存</span>
    </div>

    <div class="card-body prompt-body">
      <!-- filters -->
      <div class="prompt-filters">
        <button :class="['filter-btn', { active: filter === 'all' }]" @click="filter = 'all'">全部</button>
        <button :class="['filter-btn', { active: filter === 'general' }]" @click="filter = 'general'">通用规则</button>
        <button :class="['filter-btn', { active: filter === 'specialized' }]" @click="filter = 'specialized'">专用规则</button>
      </div>

      <div class="prompt-table-wrap">
        <table class="prompt-table">
          <thead>
            <tr>
              <th class="col-name">名称</th>
              <th class="col-cat">分类</th>
              <th class="col-preview">内容预览</th>
              <th class="col-act"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in filtered" :key="p.id" :class="{ editing: editingId === p.id }" @click="startEdit(p)">
              <td class="col-name"><span class="p-name">{{ p.name }}</span><span class="p-id">{{ p.id }}</span></td>
              <td class="col-cat"><span :class="['cat-tag', p.category]">{{ p.category === 'general' ? '通用' : '专用' }}</span></td>
              <td class="col-preview">{{ previewContent(p.content) }}</td>
              <td class="col-act"><button class="edit-btn" @click.stop="startEdit(p)">编辑</button></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Edit Modal -->
    <Transition name="modal">
      <div v-if="editingId" class="modal-overlay" @click.self="cancelEdit">
        <div class="modal-card prompt-modal">
          <div class="modal-head">
            <span class="modal-title">编辑提示词</span>
            <span class="p-id">{{ editingId }}</span>
            <span :class="['cat-tag', editingCategory]">{{ editingCategory === 'general' ? '通用' : '专用' }}</span>
          </div>
          <div class="modal-body">
            <label class="field-label">提示词名称</label>
            <input v-model="editName" class="field-input" readonly disabled />
            <label class="field-label">内容（Markdown）</label>
            <textarea
              ref="editArea"
              v-model="editContent"
              class="prompt-textarea"
              rows="20"
              spellcheck="false"
            ></textarea>
            <div class="edit-meta">
              <span>字符数: {{ editContent.length }}</span>
              <span v-if="editOrigLen">原始: {{ editOrigLen }} → 当前: {{ editContent.length }}</span>
            </div>
          </div>
          <div class="modal-actions">
            <button class="interact-btn cancel" @click="cancelEdit">取消</button>
            <button
              :class="['interact-btn confirm', saving ? 'disabled' : '']"
              :disabled="saving"
              @click="doSave"
            >
              {{ saving ? '保存中...' : '保存并热重载' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import api from '../../services/api'
import { useToast } from '../../composables/useToast'

const toast = useToast()

interface Prompt {
  id: string; name: string; category: string; content: string; keywords: string; priority: number; enabled: boolean;
}

const prompts = ref<Prompt[]>([])
const filter = ref('all')
const editingId = ref('')
const editName = ref('')
const editContent = ref('')
const editOrigLen = ref(0)
const editingCategory = ref('')
const dirtyId = ref('')
const saving = ref(false)
const editArea = ref<HTMLTextAreaElement>()

const filtered = computed(() => {
  if (filter.value === 'all') return prompts.value
  return prompts.value.filter(p => p.category === filter.value)
})

function previewContent(c: string) {
  const t = (c || '').replace(/\n/g, ' ').trim()
  return t.length > 80 ? t.slice(0, 80) + '…' : t
}

async function loadPrompts() {
  try {
    const r = await api.get('/api/config/prompts')
    const sections = r.data?.data?.sections || r.data?.sections || {}
    const list: Prompt[] = []
    for (const [id, v] of Object.entries(sections)) {
      const s = v as any
      list.push({
        id, name: s.name || id, category: s.category || 'general',
        content: s.content || '', keywords: s.keywords || '',
        priority: s.priority || 0, enabled: s.enabled !== false,
      })
    }
    list.sort((a, b) => a.category.localeCompare(b.category) || a.id.localeCompare(b.id))
    prompts.value = list
  } catch (e) {
    toast.error('加载提示词失败')
  }
}

function startEdit(p: Prompt) {
  editingId.value = p.id
  editName.value = p.name
  editContent.value = p.content
  editOrigLen.value = p.content.length
  editingCategory.value = p.category
  dirtyId.value = ''
  nextTick(() => editArea.value?.focus())
}

function cancelEdit() {
  editingId.value = ''
  dirtyId.value = ''
}

async function doSave() {
  saving.value = true
  try {
    await api.put('/api/config/prompts/' + editingId.value, { data: editContent.value })
    // update local cache
    const p = prompts.value.find(x => x.id === editingId.value)
    if (p) { p.content = editContent.value; editOrigLen.value = editContent.value.length }
    toast.success('已保存并热重载: ' + editingId.value)
    cancelEdit()
  } catch (e) {
    toast.error('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadPrompts)
</script>

<style scoped>
/* card layout — match Settings.vue card styles */
.setting-card { background: var(--glass-bg-card); border: 1px solid var(--glass-border); border-radius: var(--radius-lg); overflow: hidden; }
.setting-card.full-width { grid-column: 1 / -1; }
.card-head { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--border-subtle); }
.card-icon { color: var(--brand-500); display: flex; }
.card-title { font-size: 13px; font-weight: 600; color: var(--text-primary); flex: 1; }
.dirty-tag { font-size: 10px; color: #d97706; font-weight: 600; }

.prompt-body { display: flex; flex-direction: column; gap: 10px; }
.prompt-count { font-size: 11px; color: var(--text-muted); margin-left: auto; margin-right: 8px; }
.prompt-filters { display: flex; gap: 6px; }
.filter-btn { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 6px; background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; font-family: inherit; transition: all 0.12s; }
.filter-btn.active { background: var(--brand-500); color: #fff; border-color: var(--brand-500); }
.filter-btn:hover:not(.active) { background: var(--surface-hover); }

.prompt-table-wrap { max-height: 420px; overflow-y: auto; border: 1px solid var(--border-subtle); border-radius: 8px; }
.prompt-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.prompt-table thead { position: sticky; top: 0; z-index: 2; background: var(--surface-raised); }
.prompt-table th { padding: 8px 10px; text-align: left; font-weight: 600; color: var(--text-secondary); font-size: 11px; border-bottom: 1px solid var(--border-subtle); }
.prompt-table td { padding: 7px 10px; border-bottom: 1px solid var(--border-subtle); color: var(--text-primary); vertical-align: middle; }
.prompt-table tbody tr { cursor: pointer; transition: background 0.1s; }
.prompt-table tbody tr:hover { background: var(--surface-hover); }
.prompt-table tbody tr.editing { background: rgba(6,182,212,0.06); }
.col-name { width: 180px; }
.col-cat { width: 60px; }
.col-preview { }
.col-act { width: 60px; text-align: right; white-space: nowrap; }
.p-name { display: block; font-weight: 600; font-size: 12px; }
.p-id { display: block; font-size: 10px; color: var(--text-muted); font-family: var(--font-mono); }
.cat-tag { display: inline-block; padding: 1px 6px; border-radius: 4px; font-size: 10px; font-weight: 600; }
.cat-tag.general { background: rgba(6,182,212,0.12); color: var(--brand-600); }
.cat-tag.specialized { background: rgba(245,158,11,0.12); color: #d97706; }
.edit-btn { padding: 4px 12px; border: 1px solid var(--border-default); border-radius: 5px; background: transparent; color: var(--text-secondary); font-size: 12px; cursor: pointer; font-family: inherit; white-space: nowrap; }
.edit-btn:hover { background: var(--surface-hover); border-color: var(--brand-400); color: var(--brand-500); }

/* modal */
.modal-overlay { position: fixed; inset: 0; z-index: 500; background: rgba(15,23,42,0.4); backdrop-filter: blur(8px); display: flex; align-items: center; justify-content: center; }
.modal-card { background: var(--glass-bg-elevated); border: 1px solid var(--glass-border-strong); border-radius: 16px; padding: 24px; min-width: 600px; max-width: 800px; width: 90vw; max-height: 85vh; display: flex; flex-direction: column; gap: 14px; box-shadow: 0 20px 60px rgba(0,0,0,0.15); }
.modal-head { display: flex; align-items: center; gap: 10px; }
.modal-title { font-size: 15px; font-weight: 700; color: var(--text-primary); }
.modal-body { display: flex; flex-direction: column; gap: 6px; overflow-y: auto; }
.field-label { font-size: 12px; font-weight: 600; color: var(--text-primary); margin-top: 6px; }
.field-input { width: 100%; padding: 8px 10px; border: 1px solid var(--border-default); border-radius: 8px; background: var(--surface-input); color: var(--text-muted); font-size: 13px; font-family: var(--font-mono); outline: none; box-sizing: border-box; }
.prompt-textarea { width: 100%; padding: 10px; border: 1px solid var(--border-default); border-radius: 8px; background: var(--surface-input); color: var(--text-primary); font-size: 13px; font-family: var(--font-mono); line-height: 1.5; resize: vertical; outline: none; box-sizing: border-box; }
.prompt-textarea:focus { border-color: var(--border-focus); box-shadow: 0 0 0 3px var(--focus-ring); }
.edit-meta { display: flex; gap: 16px; font-size: 11px; color: var(--text-muted); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }

.interact-btn { padding: 8px 18px; border-radius: 8px; font-size: 13px; font-family: inherit; font-weight: 500; cursor: pointer; transition: all 0.12s; border: 1px solid var(--border-default); }
.interact-btn.cancel { background: transparent; color: var(--text-secondary); }
.interact-btn.cancel:hover { background: var(--surface-hover); }
.interact-btn.confirm { border: none; color: #fff; background: var(--gradient-brand-btn); }
.interact-btn.confirm.disabled { opacity: 0.5; pointer-events: none; }

.modal-enter-active { transition: all 0.2s ease-out; }
.modal-leave-active { transition: all 0.15s ease-in; }
.modal-enter-from, .modal-leave-to { opacity: 0; }
.modal-enter-from .modal-card, .modal-leave-to .modal-card { transform: scale(0.95) translateY(10px); }
</style>
