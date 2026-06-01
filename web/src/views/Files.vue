<template>
  <div class="files-page" @dragover.prevent="dragOver = true" @dragleave.prevent="dragOver = false" @drop.prevent="onDrop" @keydown="onKeydown" tabindex="-1" ref="pageEl">
    <!-- Toast -->
    <Transition name="toast"><div v-if="toastMsg" :class="['toast', 'toast-' + toastType]">{{ toastMsg }}</div></Transition>
    <!-- Drag overlay -->
    <Transition name="fade"><div v-if="dragOver" class="drop-zone"><div class="drop-zone-inner"><svg width="48" height="48" viewBox="0 0 48 48" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M24 32V10M16 18l8-8 8 8M10 32v8a4 4 0 004 4h20a4 4 0 004-4v-8"/></svg><span>释放文件以上传</span></div></div></Transition>

    <!-- Top bar: disk + sandbox -->
    <div class="files-header">
      <div class="header-meta">
        <div v-if="diskInfo" class="disk-mini" :class="{ warn: diskInfo.used_pct > 80, danger: diskInfo.used_pct > 90 }">
          <div class="disk-mini-bar"><div class="disk-mini-fill" :style="{ width: diskInfo.used_pct + '%' }"></div></div>
          <span class="disk-mini-text">{{ fmtBytes(diskInfo.used) }}/{{ fmtBytes(diskInfo.total) }}</span>
        </div>
        <div v-if="sandboxInfo?.sandbox_root" class="sandbox-tag" :title="sandboxInfo.sandbox_root">{{ sandboxDisplayName }}</div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="files-toolbar">
      <div class="toolbar-left">
        <button class="tb-btn" :class="{ on: clickMode === 'select' }" @click="toggleClickMode">
          <svg v-if="clickMode === 'select'" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 3h10M2 7h10M2 11h7"/><circle cx="11" cy="11" r="1" fill="currentColor" stroke="none"/></svg>
          <svg v-else width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4.5v7a1 1 0 001 1h8a1 1 0 001-1V6a1 1 0 00-1-1H6.5L5.5 3H3a1 1 0 00-1 1v.5"/><path d="M8 8.5l2-2-2-2"/></svg>
        </button>
        <label class="tb-btn accent" title="上传文件"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M7 10V2M4 5l3-3 3 3M2 9v3a1 1 0 001 1h8a1 1 0 001-1V9"/></svg><input type="file" multiple @change="doUpload" hidden /></label>
        <label class="tb-btn accent" title="上传文件夹"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M2 4v8a1 1 0 001 1h8a1 1 0 001-1V5a1 1 0 00-1-1H6.5L5.5 2.5H3a1 1 0 00-1 1v.5"/><path d="M7 7v3M5.5 8.5h3"/></svg><input type="file" webkitdirectory directory multiple @change="doFolderUpload" hidden /></label>
        <button class="tb-btn" @click="showMkdir = true" title="新建文件夹"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 4v8a1 1 0 001 1h8a1 1 0 001-1V5a1 1 0 00-1-1H6.5L5.5 2.5H3a1 1 0 00-1 1v.5"/><path d="M7 7v3M5.5 8.5h3"/></svg></button>
        <button class="tb-btn" @click="loadFiles" title="刷新"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M12 7a5 5 0 11-1.5-3.5M12 2v4h-4"/></svg></button>
        <button class="tb-btn" :class="{ on: viewMode === 'grid' }" @click="viewMode = viewMode === 'list' ? 'grid' : 'list'" title="切换视图">
          <svg v-if="viewMode === 'grid'" width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1" y="1" width="5" height="5" rx="1"/><rect x="8" y="1" width="5" height="5" rx="1"/><rect x="1" y="8" width="5" height="5" rx="1"/><rect x="8" y="8" width="5" height="5" rx="1"/></svg>
          <svg v-else width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M2 3h10M2 7h10M2 11h7"/></svg>
        </button>
      </div>
      <div class="toolbar-right">
        <template v-if="selectedFiles.length">
          <span class="sel-count">{{ selectedFiles.length }} 个已选</span>
          <button class="tb-btn" @click="cutSelected">剪切</button>
          <button class="tb-btn" @click="copySelected">复制</button>
          <button class="tb-btn danger" @click="batchDelete">删除</button>
        </template>
        <button v-if="clipboard.files.length" class="tb-btn accent" @click="doPaste">粘贴({{ clipboard.files.length }})</button>
        <!-- type filter -->
        <div class="type-filter">
          <button v-for="ft in fileTypeFilters" :key="ft.key" :class="['type-pill', { active: fileTypeFilter === ft.key }]" @click="fileTypeFilter = ft.key">{{ ft.label }}</button>
        </div>
        <div class="search-wrap">
          <svg class="search-icon" width="13" height="13" viewBox="0 0 13 13" fill="none" stroke="currentColor" stroke-width="1.6"><circle cx="5.5" cy="5.5" r="3.5"/><path d="M8.5 8.5L12 12"/></svg>
          <input v-model="searchQuery" placeholder="搜索" class="search-inp" @input="doSearch" @keyup.escape="clearSearch" />
          <button v-if="searchQuery" class="search-clr" @click="clearSearch">&times;</button>
        </div>
        <div class="sort-group">
          <button v-for="opt in sortOptions" :key="opt.key" :class="['sort-pill', { active: sortBy === opt.key }]" @click="toggleSort(opt.key)">{{ opt.label }}<svg v-if="sortBy === opt.key" :class="['sort-arrow', { flip: sortOrder === -1 }]" width="8" height="8" viewBox="0 0 8 8" fill="currentColor"><path d="M4 1v6M1.5 4.5L4 7l2.5-2.5"/></svg></button>
        </div>
      </div>
    </div>

    <!-- Three-panel body -->
    <div class="files-body">
      <!-- Left: Directory Tree -->
      <div class="file-tree-panel">
        <div class="tree-head">目录</div>
        <div class="tree-list">
          <div :class="['tree-item', 'tree-root', { active: currentPath === '/' }]" @click="navigateTo('/')">
            <span class="tree-arrow" :class="{ expanded: expandedDirs.has('/') }" @click.stop="toggleExpand('/')">▾</span>
            <span class="tree-icon">📁</span><span class="tree-name">/</span>
          </div>
          <template v-for="(node, i) in visibleNodes" :key="node.path">
            <div :class="['tree-item', { active: currentPath === node.path }]" @click="navigateTo(node.path)" @contextmenu.prevent="onTreeContext($event, node)">
              <span class="tree-indent">
                <span v-for="d in node.depth - 1" :key="d" class="tree-line" :class="{ 'line-through': hasSiblingAfter(visibleNodes, i, d) }"></span>
                <span class="tree-line" :class="node._last ? 'line-end' : 'line-branch'"></span>
              </span>
              <span v-if="node.children > 0" class="tree-arrow" :class="{ expanded: expandedDirs.has(node.path) }" @click.stop="toggleExpand(node.path)">▸</span>
              <span v-else class="tree-arrow-spacer"></span>
              <span class="tree-icon">{{ expandedDirs.has(node.path) ? '📂' : '📁' }}</span>
              <span class="tree-name">{{ node.name }}</span>
            </div>
          </template>
          <div v-if="treeLoading" class="tree-loading">...</div>
        </div>
      </div>

      <!-- Center: File list -->
      <div class="files-center">
        <div class="file-list-wrap" ref="fileTableEl"
          @mousedown="onFileAreaMouseDown" @mousemove="onFileAreaMouseMove" @mouseup="onFileAreaMouseUp" @mouseleave="onFileAreaMouseUp">
          <div v-if="dragSelecting && dragRect" class="drag-select-box" :style="{ left: dragRect.left + 'px', top: dragRect.top + 'px', width: dragRect.width + 'px', height: dragRect.height + 'px' }"></div>

          <!-- Grid view -->
          <div v-if="viewMode === 'grid' && displayFiles.length" class="file-grid">
            <div v-for="f in sortedFiles" :key="f.path" :class="['grid-item', { dir: f.is_dir, selected: isSelected(f) }]" @click="onFileClick(f)" @dblclick="clickMode === 'select' ? onFileOpen(f) : null" @contextmenu.prevent="onContextMenu($event, f)">
              <div class="grid-icon">
                <img v-if="f.isUploading" src="data:image/svg+xml,..." style="display:none" />
                <svg v-if="f.is_dir" width="36" height="36" viewBox="0 0 22 22" fill="var(--brand-500)" stroke="var(--brand-600)" stroke-width="0.7"><path d="M2.5 6.5v9.5a1.5 1.5 0 001.5 1.5h14a1.5 1.5 0 001.5-1.5V8a1 1 0 00-1-1h-7L9.5 5H4a1.5 1.5 0 00-1.5 1.5z"/></svg>
                <svg v-else width="32" height="32" viewBox="0 0 20 20" fill="none" stroke="var(--text-muted)" stroke-width="1.2"><path d="M5.5 2h6l3 3v10.5a1.5 1.5 0 01-1.5 1.5H5.5a1.5 1.5 0 01-1.5-1.5V3.5A1.5 1.5 0 015.5 2z"/><path d="M11.5 2v3h3"/></svg>
              </div>
              <div class="grid-name">{{ f.name }}</div>
              <div v-if="f.isUploading" class="mini-progress"><div :style="{ width: f.progress + '%' }"></div></div>
            </div>
          </div>

          <!-- List view -->
          <div v-else-if="displayFiles.length" class="file-list">
            <div v-for="f in sortedFiles" :key="f.path"
              class="file-row" :class="{ dir: f.is_dir, selected: isSelected(f), uploading: f.isUploading, 'cut-mark': clipboard.mode === 'cut' && clipboard.files.some(c => c.path === f.path) }"
              :data-path="f.path" :draggable="true"
              @dragstart.self="onDragStart($event, f)" @dragover.prevent="f.is_dir ? (dragOverDir = f.path, true) : false" @dragleave="dragOverDir = null"
              @drop.prevent="f.is_dir ? onDropToDir($event, f) : null" @click="onFileClick(f)" @dblclick="clickMode === 'select' ? onFileOpen(f) : null" @contextmenu.prevent="onContextMenu($event, f)">
              <span class="file-check" @click.stop="toggleSelect(f)">
                <svg v-if="isSelected(f)" width="15" height="15" viewBox="0 0 15 15" fill="var(--brand-500)" stroke="var(--brand-500)" stroke-width="1.2"><rect x="1.5" y="1.5" width="12" height="12" rx="3.5"/><path d="M4.5 7.5l2 2 4-4" stroke="#fff" stroke-width="1.6" fill="none"/></svg>
                <svg v-else width="15" height="15" viewBox="0 0 15 15" fill="none" stroke="var(--text-muted)" stroke-width="1.2"><rect x="1.5" y="1.5" width="12" height="12" rx="3.5"/></svg>
              </span>
              <span class="file-icon">
                <svg v-if="f.is_dir" width="22" height="22" viewBox="0 0 22 22" fill="var(--brand-500)" stroke="var(--brand-600)" stroke-width="0.7"><path d="M2.5 6.5v9.5a1.5 1.5 0 001.5 1.5h14a1.5 1.5 0 001.5-1.5V8a1 1 0 00-1-1h-7L9.5 5H4a1.5 1.5 0 00-1.5 1.5z"/></svg>
                <svg v-else-if="f.isUploading" width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="var(--brand-400)" stroke-width="1.2"><path d="M5.5 2h6l3 3v10.5a1.5 1.5 0 01-1.5 1.5H5.5a1.5 1.5 0 01-1.5-1.5V3.5A1.5 1.5 0 015.5 2z"/><path d="M11.5 2v3h3"/></svg>
                <svg v-else width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="var(--text-muted)" stroke-width="1.2"><path d="M5.5 2h6l3 3v10.5a1.5 1.5 0 01-1.5 1.5H5.5a1.5 1.5 0 01-1.5-1.5V3.5A1.5 1.5 0 015.5 2z"/><path d="M11.5 2v3h3"/></svg>
              </span>
              <span class="file-name">{{ f.name }}</span>
              <span class="file-meta">
                <span v-if="f.isUploading" class="file-progress">{{ f.progress }}%</span>
                <span v-else class="file-size">{{ fmtBytes(f.size) }}</span>
                <span class="file-time">{{ fmtTime(f.mod_time) }}</span>
              </span>
              <span class="file-actions" @click.stop>
                <button v-if="!f.is_dir && !f.isUploading && isTextExt(f)" class="act-btn" @click="startEdit(f)" title="编辑">✎</button>
                <button v-if="!f.is_dir && !f.isUploading" class="act-btn" @click="downloadFile(f)" title="下载">↓</button>
                <button v-if="!f.isUploading" class="act-btn" @click="startRename(f)" title="重命名">✐</button>
                <button v-if="!f.isUploading" class="act-btn" @click="startShare(f)" title="分享">↗</button>
                <button v-if="!f.isUploading" class="act-btn del" @click="deleteFile(f)" title="删除">✕</button>
              </span>
            </div>
          </div>

          <div v-else-if="!loading" class="empty-list">
            <svg width="40" height="40" viewBox="0 0 40 40" fill="none" stroke="var(--text-muted)" stroke-width="1"><path d="M5 8h12l3 3h15a3 3 0 013 3v16a3 3 0 01-3 3H5a3 3 0 01-3-3V11a3 3 0 013-3z"/></svg>
            <p>此目录为空</p>
            <label class="empty-upload-btn"><svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M7 10V2M4 5l3-3 3 3M2 9v3a1 1 0 001 1h8a1 1 0 001-1V9"/></svg>上传文件<input type="file" multiple @change="doUpload" hidden /></label>
          </div>
          <div v-else class="loading-state">加载中...</div>
        </div>
      </div>

      <!-- Right: Upload panel -->
      <div v-if="uploadStore.tasks.length" class="upload-panel" :class="{ collapsed: upCollapsed }">
        <div class="up-head" @click="upCollapsed = !upCollapsed">
          <span>{{ uploadStore.hasActive ? '上传中' : '已完成' }} ({{ uploadStore.tasks.length }})</span>
          <button v-if="!uploadStore.hasActive" class="up-dismiss" @click.stop="uploadStore.clearDone()">✕</button>
        </div>
        <div v-if="!upCollapsed" class="up-body">
          <div v-for="t in uploadStore.tasks" :key="t.id" class="up-item">
            <div class="up-name">{{ t.name }}</div>
            <div class="up-track"><div class="up-fill" :class="t.status" :style="{ width: t.progress + '%' }"></div></div>
            <div class="up-meta">
              <span :class="['up-status', t.status]">{{ t.status === 'uploading' ? t.progress + '%' : t.status === 'done' ? '✓' : '✗' }}</span>
              <button v-if="t.status === 'uploading'" class="up-cancel" @click="uploadStore.cancelTask(t.id)">✕</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <div v-if="showMkdir" class="modal-overlay" @click.self="showMkdir = false"><div class="modal-card"><h4>新建文件夹</h4><input v-model="newDirName" placeholder="文件夹名称" class="modal-input" @keyup.enter="doMkdir" /><div class="modal-actions"><button class="btn-cancel" @click="showMkdir = false">取消</button><button class="btn-ok" @click="doMkdir">创建</button></div></div></div>
    <div v-if="showRename" class="modal-overlay" @click.self="showRename = false"><div class="modal-card"><h4>重命名</h4><input v-model="renameName" placeholder="新名称" class="modal-input" @keyup.enter="doRename" /><div class="modal-actions"><button class="btn-cancel" @click="showRename = false">取消</button><button class="btn-ok" @click="doRename">确定</button></div></div></div>
    <div v-if="showEdit" class="modal-overlay" @click.self="showEdit = false"><div class="modal-card wide"><h4>编辑: {{ editFile?.name }}</h4><textarea v-model="editContent" class="modal-textarea" rows="14" spellcheck="false"></textarea><div class="modal-actions"><button class="btn-cancel" @click="showEdit = false">取消</button><button class="btn-ok" @click="doEdit" :disabled="editSaving">保存</button></div></div></div>
    <div v-if="previewFile" class="modal-overlay preview-overlay" @click.self="previewFile = null"><div class="preview-modal">
      <div class="preview-head"><span class="preview-title">{{ previewFile.name }}</span><span class="preview-size">{{ fmtBytes(previewFile.size) }}</span><button v-if="!previewFile.is_dir" class="btn-ok" @click="downloadFile(previewFile)">下载</button><button class="preview-close" @click="previewFile = null">&times;</button></div>
      <div class="preview-body">
        <div v-if="isVideo(previewFile)" class="video-wrap"><video ref="videoPlayer" controls autoplay :src="streamUrl(previewFile)"></video></div>
        <div v-else-if="isImage(previewFile)" class="img-wrap"><img :src="streamUrl(previewFile)" :alt="previewFile.name" /></div>
        <div v-else-if="isTextExt(previewFile)" class="text-wrap">
          <div v-if="(previewFile.ext || '').toLowerCase() === '.md' && previewContent !== null" class="md-preview-content" v-html="renderMarkdown(previewContent)"></div>
          <pre v-else-if="previewContent !== null" class="preview-text">{{ previewContent }}</pre>
          <div v-else class="preview-loading">加载中...</div>
        </div>
        <div v-else class="preview-unsupported"><p>不支持预览此类型</p><button class="btn-ok" @click="downloadFile(previewFile)">下载</button></div>
      </div>
    </div></div>
    <div v-if="showShare" class="modal-overlay" @click.self="showShare = false"><div class="modal-card"><h4>分享</h4>
      <div class="modal-field"><label>文件</label><span class="mono">{{ sharePath }}</span></div><div class="modal-field"><label>过期天数</label><input v-model.number="shareDays" type="number" min="0" class="modal-input" /></div><div class="modal-field"><label>密码 (可选)</label><input v-model="sharePass" placeholder="留空无密码" class="modal-input" /></div>
      <div v-if="shareResult" class="share-link"><code>{{ shareResult }}</code><button class="btn-ok" @click="copyShareUrl">复制</button></div>
      <div class="modal-actions"><button class="btn-cancel" @click="showShare = false">关闭</button><button class="btn-ok" @click="doShare" :disabled="sharing">{{ sharing ? '创建中...' : '创建' }}</button></div>
    </div></div>
    <div v-if="showStatFile" class="modal-overlay" @click.self="showStatFile = null"><div class="modal-card" v-if="statInfo"><h4>{{ showStatFile.name }}</h4>
      <div class="stat-list"><div class="stat-line"><span class="sl">路径</span><span class="sv">{{ statInfo.path }}</span></div><div class="stat-line"><span class="sl">类型</span><span class="sv">{{ statInfo.is_dir ? '文件夹' : '文件' }}</span></div><div class="stat-line"><span class="sl">大小</span><span class="sv">{{ statInfo.size ? fmtBytes(statInfo.size) : '--' }}</span></div><div class="stat-line"><span class="sl">权限</span><span class="sv">{{ statInfo.mode }} ({{ statInfo.permissions }})</span></div><div class="stat-line"><span class="sl">修改时间</span><span class="sv">{{ statInfo.mod_time }}</span></div></div>
      <div class="modal-actions"><button class="btn-cancel" @click="showStatFile = null">关闭</button></div>
    </div></div>

    <ContextMenu :visible="ctxMenu.visible" :x="ctxMenu.x" :y="ctxMenu.y" :items="ctxItems" @close="ctxMenu.visible = false" @action="onCtxAction" />
    <ConfirmDialog :visible="confirmDialog.visible" :title="confirmDialog.title" :message="confirmDialog.message" :confirm-text="confirmDialog.confirmText" :variant="confirmDialog.variant" icon="warn" @confirm="confirmDialog.visible = false; confirmDialog.resolve(true)" @cancel="confirmDialog.visible = false; confirmDialog.resolve(false)" />
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
import { ref, computed, reactive, onMounted } from 'vue'
import api from '../services/api'
import { useUploadStore } from '../stores/upload'
import { useToast } from '../composables/useToast'
import { renderMarkdown } from '../stores/chat'
import ContextMenu from '../components/ui/ContextMenu.vue'
import ConfirmDialog from '../components/ui/ConfirmDialog.vue'

const toast = useToast()
const uploadStore = useUploadStore()

const clickMode = ref(localStorage.getItem('files_click_mode') || 'select')
const viewMode = ref(localStorage.getItem('files_view_mode') || 'list')
function toggleClickMode() { clickMode.value = clickMode.value === 'select' ? 'open' : 'select'; localStorage.setItem('files_click_mode', clickMode.value) }
const currentPath = ref('/')
const files = ref([])
const loading = ref(false)
const diskInfo = ref(null)
const sandboxInfo = ref(null)
const showMkdir = ref(false); const newDirName = ref('')
const showRename = ref(false); const renameTarget = ref(null); const renameName = ref('')
const showShare = ref(false); const sharePath = ref(''); const shareDays = ref(7); const sharePass = ref(''); const shareResult = ref(''); const sharing = ref(false)
const sortBy = ref('name'); const sortOrder = ref(1)
const selectedFiles = ref([])
const sortOptions = [{ key: 'name', label: '名称' }, { key: 'size', label: '大小' }, { key: 'time', label: '时间' }]
const dragOver = ref(false); const pageEl = ref(null)
const searchQuery = ref(''); const searchResults = ref(null)
const showEdit = ref(false); const editFile = ref(null); const editContent = ref(''); const editSaving = ref(false)
const showStatFile = ref(null); const statInfo = ref(null)
const previewFile = ref(null); const previewContent = ref(null); const videoPlayer = ref(null)
const dragSelecting = ref(false); const fileTableEl = ref(null)
const dragStart = ref({ x: 0, y: 0 }); const dragRect = ref(null)
const dragOverDir = ref(null)
const ctxMenu = ref({ visible: false, x: 0, y: 0, target: null })
const clipboard = ref(loadClipboard())
const upCollapsed = ref(false)
const fileTypeFilter = ref('all')
const fileTypeFilters = [{ key: 'all', label: '全部' }, { key: 'image', label: '图片' }, { key: 'video', label: '视频' }, { key: 'doc', label: '文档' }, { key: 'other', label: '其他' }]
const treeNodes = ref([])
const treeLoading = ref(false)
const expandedDirs = ref(new Set(['/']))

function toggleExpand(path) {
  if (expandedDirs.value.has(path)) expandedDirs.value.delete(path)
  else expandedDirs.value.add(path)
  // trigger reactivity
  expandedDirs.value = new Set(expandedDirs.value)
}

const visibleNodes = computed(() => {
  const result = []
  for (const node of treeNodes.value) {
    // Check if all ancestors are expanded
    const parts = node.path.split('/').filter(Boolean)
    let visible = true
    let acc = ''
    for (let i = 0; i < parts.length - 1; i++) {
      acc += '/' + parts[i]
      if (!expandedDirs.value.has(acc)) { visible = false; break }
    }
    // Root-level dirs always visible if root is expanded
    if (node.depth === 1 && !expandedDirs.value.has('/')) visible = false
    if (visible) result.push(node)
  }
  return result
})

function hasSiblingAfter(nodes, idx, depth) {
  for (let j = idx + 1; j < nodes.length; j++) {
    if (nodes[j].depth < depth) return false
    if (nodes[j].depth === depth) return true
  }
  return false
}

function loadClipboard() { try { const raw = sessionStorage.getItem('files_clipboard'); if (raw) return JSON.parse(raw) } catch (_) {} return { files: [], mode: '' } }
function saveClipboard() { sessionStorage.setItem('files_clipboard', JSON.stringify(clipboard.value)) }
function clearClipboard() { clipboard.value = { files: [], mode: "" }; saveClipboard() }
function copySelected() { clipboard.value = { files: [...selectedFiles.value], mode: 'copy' }; saveClipboard(); showToast('已复制 ' + selectedFiles.value.length + ' 项') }
function cutSelected() { clipboard.value = { files: [...selectedFiles.value], mode: 'cut' }; saveClipboard(); showToast('已剪切 ' + selectedFiles.value.length + ' 项') }
const toastMsg = ref(''); const toastType = ref('info'); let toastTimer = null
function showToast(msg, type = 'info') { toastMsg.value = msg; toastType.value = type; clearTimeout(toastTimer); toastTimer = setTimeout(() => { toastMsg.value = '' }, 3000) }
const confirmDialog = reactive({ visible: false, title: '', message: '', confirmText: '确定', variant: 'danger', resolve: (v) => {} })
function showConfirm(title, message, opts = {}) { return new Promise((resolve) => { Object.assign(confirmDialog, { visible: true, title, message, confirmText: opts.confirmText || '确定', variant: opts.variant || 'danger', resolve }) }) }

const sandboxDisplayName = computed(() => { const root = sandboxInfo.value?.sandbox_root; if (!root) return '沙箱'; const parts = root.replace(/\\/g, '/').split('/').filter(Boolean); return (parts[parts.length - 1] || root) + '/' })

const VIDEO_EXTS = ['.mp4','.mkv','.avi','.mov','.webm','.flv','.wmv']
const IMAGE_EXTS = ['.jpg','.jpeg','.png','.gif','.webp','.bmp','.svg','.ico']
const DOC_EXTS = ['.txt','.md','.log','.json','.xml','.yml','.yaml','.toml','.ini','.cfg','.conf','.sh','.bat','.py','.js','.ts','.vue','.go','.rs','.c','.cpp','.h','.java','.css','.html','.csv','.env','.sql','.pdf','.docx','.pptx','.xlsx']
function isVideo(f) { return VIDEO_EXTS.includes((f.ext||'').toLowerCase()) }
function isImage(f) { return IMAGE_EXTS.includes((f.ext||'').toLowerCase()) }
function isTextExt(f) { return DOC_EXTS.includes((f.ext||'').toLowerCase()) }
function fmtBytes(bytes) { if (!bytes) return '0 B'; const k = 1024; const sizes = ['B','KB','MB','GB']; const i = Math.floor(Math.log(bytes)/Math.log(k)); return parseFloat((bytes/Math.pow(k,i)).toFixed(1))+' '+sizes[i] }
function fmtTime(t) { if (!t) return ''; const d = new Date(t); const now = new Date(); const diff = now - d; if (diff < 86400000) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }); return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' }) }
function streamUrl(f) { const token = localStorage.getItem('token'); return '/api/nas/files/stream?path=' + encodeURIComponent(f.path) + '&token=' + encodeURIComponent(token || '') }
function toggleSort(key) { if (sortBy.value === key) sortOrder.value *= -1; else { sortBy.value = key; sortOrder.value = 1 } }

const uploadingEntries = computed(() => {
  if (!uploadStore.hasActive) return []
  return uploadStore.tasks.filter(t => t.status === 'uploading').map(t => ({ name: t.name, size: t.fileSize || 0, isUploading: true, progress: t.progress, is_dir: false }))
})

const displayFiles = computed(() => {
  let list = searchResults.value || [...files.value]
  if (uploadStore.hasActive) {
    for (const u of uploadingEntries.value) {
      if (!list.find(f => f.name === u.name)) list.unshift(u)
    }
  }
  // File type filter
  if (fileTypeFilter.value === 'image') list = list.filter(f => f.is_dir || isImage(f))
  else if (fileTypeFilter.value === 'video') list = list.filter(f => f.is_dir || isVideo(f))
  else if (fileTypeFilter.value === 'doc') list = list.filter(f => f.is_dir || isTextExt(f))
  else if (fileTypeFilter.value === 'other') list = list.filter(f => f.is_dir || (!isImage(f) && !isVideo(f) && !isTextExt(f)))
  return list
})

const sortedFiles = computed(() => {
  const arr = [...displayFiles.value]
  arr.sort((a, b) => { const d = (b.is_dir ? 1 : 0) - (a.is_dir ? 1 : 0); if (d) return d; let va = a[sortBy.value === 'time' ? 'mod_time' : sortBy.value] ?? ''; let vb = b[sortBy.value === 'time' ? 'mod_time' : sortBy.value] ?? ''; if (sortBy.value === 'size') { va = a.size || 0; vb = b.size || 0 } else if (sortBy.value === 'name') { va = (a.name || '').toLowerCase(); vb = (b.name || '').toLowerCase() }; if (va < vb) return -1 * sortOrder.value; if (va > vb) return 1 * sortOrder.value; return 0 })
  return arr
})

async function loadFiles() {
  loading.value = true
  try {
    const [fRes, dRes] = await Promise.all([api.get('/api/nas/files', { params: { path: currentPath.value } }), api.get('/api/nas/diskinfo', { params: { path: currentPath.value } })])
    files.value = fRes.data.data || []; diskInfo.value = dRes.data.data
    try { const sbRes = await api.get('/api/sandbox/status'); sandboxInfo.value = sbRes.data } catch (_) {}
    loadTree()
  } catch (e) { files.value = [] } finally { loading.value = false }
}

async function loadTree() {
  treeLoading.value = true
  try {
    const res = await api.get('/api/nas/files/tree', { params: { path: '/' } })
    const data = res.data?.data || res.data || []
    const nodes = []
    function walk(children, depth) {
      if (depth > 4 || !children) return
      for (let i = 0; i < children.length; i++) {
        const n = children[i]
        nodes.push({ name: n.name, path: n.path, depth, children: n.children?.length || 0, _last: i === children.length - 1 && (!n.children || n.children.length === 0) })
        if (n.children) {
          const lastChildIdx = n.children.length - 1
          for (let j = 0; j < n.children.length; j++) {
            const c = n.children[j]
            nodes.push({ name: c.name, path: c.path, depth: depth + 1, children: c.children?.length || 0, _last: j === lastChildIdx })
            if (c.children) {
              const lastGcIdx = c.children.length - 1
              for (let k = 0; k < c.children.length; k++) {
                const gc = c.children[k]
                nodes.push({ name: gc.name, path: gc.path, depth: depth + 2, children: gc.children?.length || 0, _last: k === lastGcIdx })
                if (gc.children) {
                  const lastGgcIdx = gc.children.length - 1
                  for (let l = 0; l < gc.children.length; l++) {
                    const ggc = gc.children[l]
                    nodes.push({ name: ggc.name, path: ggc.path, depth: depth + 3, children: 0, _last: l === lastGgcIdx })
                  }
                }
              }
            }
          }
        }
      }
    }
    walk(Array.isArray(data) ? data : [], 1)
    treeNodes.value = nodes
  } catch (e) { treeNodes.value = [] } finally { treeLoading.value = false }
}

function navigateTo(path) { currentPath.value = path; selectedFiles.value = []; loadFiles() }
function toggleSelect(f) { const idx = selectedFiles.value.findIndex(s => s.path === f.path); if (idx >= 0) selectedFiles.value.splice(idx, 1); else selectedFiles.value.push(f) }
function isSelected(f) { return selectedFiles.value.some(s => s.path === f.path) }
function onFileClick(f) { if (clickMode.value === 'open') onFileOpen(f); else toggleSelect(f) }
function onFileOpen(f) { if (f.is_dir) navigateTo(f.path); else openPreview(f) }

async function doPaste() {
  if (!clipboard.value.files.length) return
  try {
    for (const f of clipboard.value.files) {
      const destPath = currentPath.value + '/' + (f.name || f.path.split('/').pop())
      await api.post('/api/nas/files/copy', { src: f.path, dst: destPath })
    }
    if (clipboard.value.mode === 'cut') { for (const f of clipboard.value.files) { await api.delete('/api/nas/files', { data: { path: f.path } }) }; clearClipboard() }
    loadFiles(); showToast('粘贴完成')
  } catch (e) { showToast('粘贴失败', 'error') }
}

async function doMkdir() { if (!newDirName.value.trim()) return; try { await api.post('/api/nas/files/mkdir', { path: currentPath.value + '/' + newDirName.value.trim() }); showMkdir.value = false; newDirName.value = ''; clearClipboard(); loadFiles() } catch (e) { showToast('创建失败', 'error') } }
function startRename(f) { renameTarget.value = f; renameName.value = f.name; showRename.value = true }
async function doRename() { if (!renameName.value.trim()) return; try { const oldPath = renameTarget.value.path; const parts = oldPath.split('/'); parts[parts.length-1] = renameName.value.trim(); await api.put('/api/nas/files/rename', { old_path: oldPath, new_path: parts.join('/') }); showRename.value = false; clearClipboard(); loadFiles() } catch (e) { showToast('重命名失败', 'error') } }
function startShare(f) { sharePath.value = f.path; shareResult.value = ''; sharePass.value = ''; shareDays.value = 7; showShare.value = true }
async function doShare() { sharing.value = true; try { const res = await api.post('/api/nas/shares', { file_path: sharePath.value, expire_days: shareDays.value, password: sharePass.value || null }); shareResult.value = window.location.origin + '/s/' + res.data.data.token } catch (e) { showToast('创建失败', 'error') } finally { sharing.value = false } }
function copyShareUrl() { navigator.clipboard.writeText(shareResult.value); showToast('已复制') }

async function deleteFile(f) { const ok = await showConfirm('删除', '确认删除 ' + f.name + '？'); if (!ok) return; try { await api.delete('/api/nas/files', { data: { path: f.path } }); clearClipboard(); loadFiles(); showToast('已删除') } catch (e) { showToast('删除失败', 'error') } }
async function batchDelete() { if (!selectedFiles.value.length) return; const ok = await showConfirm('批量删除', '确认删除 ' + selectedFiles.value.length + ' 项？'); if (!ok) return; try { const res = await api.post('/api/nas/files/batch-delete', { paths: selectedFiles.value.map(f => f.path) }); const results = res.data?.data?.results || []; const failed = results.filter(r => !r.success); selectedFiles.value = []; clearClipboard(); loadFiles(); if (failed.length) showToast(failed.length + ' 项失败', 'error') } catch (e) { showToast('批量删除失败', 'error') } }

async function startEdit(f) { editFile.value = f; editContent.value = ''; editSaving.value = false; showEdit.value = true; try { const token = localStorage.getItem('token'); const res = await fetch('/api/nas/files/preview?path=' + encodeURIComponent(f.path), { headers: { 'Authorization': 'Bearer ' + token } }); editContent.value = await res.text() } catch (_) { editContent.value = '[加载失败]' } }
async function doEdit() { if (!editFile.value) return; editSaving.value = true; try { const token = localStorage.getItem('token'); await fetch('/api/nas/files/save?path=' + encodeURIComponent(editFile.value.path), { method: 'PUT', headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'text/plain' }, body: editContent.value }); showEdit.value = false; loadFiles(); showToast('已保存') } catch (e) { showToast('保存失败', 'error') } finally { editSaving.value = false } }

function downloadFile(f) { const token = localStorage.getItem('token'); const a = document.createElement('a'); a.href = streamUrl(f); a.download = f.name; a.click() }
async function showStat(f) { showStatFile.value = f; try { const res = await api.get('/api/nas/files/stat', { params: { path: f.path } }); statInfo.value = res.data.data } catch (e) { showToast('获取属性失败', 'error') } }
function openPreview(f) { previewFile.value = f; previewContent.value = null; if (isTextExt(f)) { fetch('/api/nas/files/preview?path=' + encodeURIComponent(f.path), { headers: { 'Authorization': 'Bearer ' + localStorage.getItem('token') } }).then(r => r.text()).then(t => { previewContent.value = t }).catch(() => { previewContent.value = '[加载失败]' }) } }

function doUpload(e) { const files = e.target.files; if (!files?.length) return; for (const f of files) uploadStore.startUpload(f, currentPath.value); e.target.value = ''; showToast('已添加 ' + files.length + ' 个文件') }
function doFolderUpload(e) { const files = e.target.files; if (!files?.length) { e.target.value = ''; return }; const fileList = []; for (const f of files) { const relPath = f.webkitRelativePath || f.name; const parts = relPath.split('/'); parts.pop(); const subDir = parts.length ? '/' + parts.join('/') : ''; fileList.push({ file: f, dir: (currentPath.value + subDir).replace(/\/+/g, '/') }) }; e.target.value = ''; uploadStore.startFolderUpload(fileList, onFolderProgress); showToast('已添加文件夹 ' + fileList[0]?.file.webkitRelativePath?.split('/')[0] + ' (' + fileList.length + ' 个文件)') }

function onFolderProgress(done, total, name, failed) {
  if (failed !== undefined) {
    if (failed.length) { toast.warning(`上传完成：${done} 个成功，${failed.length} 个失败`); console.warn('Upload failures:', failed) }
    else { toast.success(`已上传 ${done} 个文件`) }
    loadFiles()
  }
}

async function traverseDropEntry(entry, baseDir, fileList) {
  if (entry.isFile) { return new Promise((resolve) => { entry.file((file) => { const relPath = entry.fullPath || ('/' + file.name); const parts = relPath.replace(/\\/g, '/').split('/').filter(Boolean); parts.pop(); const subDir = parts.length ? '/' + parts.join('/') : ''; fileList.push({ file, dir: (baseDir + subDir).replace(/\/+/g, '/') }); resolve() }) }) }
  else if (entry.isDirectory) { const reader = entry.createReader(); return new Promise((resolve) => { const readAll = () => { reader.readEntries(async (entries) => { if (!entries.length) { resolve(); return }; for (const e of entries) { await traverseDropEntry(e, baseDir, fileList) }; readAll() }) }; readAll() }) }
}

async function onDrop(e) { dragOver.value = false; const items = e.dataTransfer?.items; if (!items?.length) return; const fileList = []; for (let i = 0; i < items.length; i++) { const item = items[i]; if (item.kind === 'file') { const entry = item.webkitGetAsEntry ? item.webkitGetAsEntry() : null; if (entry) { await traverseDropEntry(entry, currentPath.value, fileList) } else { const file = item.getAsFile(); if (file) fileList.push({ file, dir: currentPath.value }) } } }; if (fileList.length) { uploadStore.startFolderUpload(fileList, onFolderProgress); const folderName = fileList[0]?.file.webkitRelativePath?.split('/')[0]; const label = folderName ? '文件夹 ' + folderName : fileList.length + ' 个文件'; showToast('已添加 ' + label + ' (' + fileList.length + ' 项)') } }
async function onDropToDir(e, dir) { e.preventDefault(); dragOverDir.value = null; const items = e.dataTransfer?.items; if (!items?.length) return; const fileList = []; for (let i = 0; i < items.length; i++) { const item = items[i]; if (item.kind === 'file') { const entry = item.webkitGetAsEntry ? item.webkitGetAsEntry() : null; if (entry) { await traverseDropEntry(entry, dir.path, fileList) } else { const file = item.getAsFile(); if (file) fileList.push({ file, dir: dir.path }) } } }; if (fileList.length) { uploadStore.startFolderUpload(fileList, onFolderProgress); showToast('已添加到 ' + dir.name + ' (' + fileList.length + ' 项)') } }

function onKeydown(e) { if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return; if (e.key === 'Escape') { previewFile.value = null; showEdit.value = false; showMkdir.value = false; showRename.value = false; showShare.value = false; selectedFiles.value = []; clearSearch() } if (e.key === 'Delete' && selectedFiles.value.length) batchDelete() }
function doSearch() { const q = searchQuery.value.trim().toLowerCase(); if (!q) { searchResults.value = null; return }; searchResults.value = files.value.filter(f => f.name.toLowerCase().includes(q)) }
function clearSearch() { searchQuery.value = ''; searchResults.value = null }

function onFileAreaMouseDown(e) { if (e.target.closest('.file-actions,.file-check')) return; if (e.button !== 0) return; const rect = fileTableEl.value.getBoundingClientRect(); dragStart.value = { x: e.clientX - rect.left, y: e.clientY - rect.top }; dragSelecting.value = true; dragRect.value = null }
function onFileAreaMouseMove(e) { if (!dragSelecting.value) return; const rect = fileTableEl.value.getBoundingClientRect(); const x = e.clientX - rect.left; const y = e.clientY - rect.top; const dx = Math.abs(x - dragStart.value.x); const dy = Math.abs(y - dragStart.value.y); if (dx < 8 && dy < 8) return; dragRect.value = { left: Math.min(dragStart.value.x, x), top: Math.min(dragStart.value.y, y), width: Math.abs(x - dragStart.value.x), height: Math.abs(y - dragStart.value.y) } }
function onFileAreaMouseUp() { if (!dragSelecting.value) return; dragSelecting.value = false; if (!dragRect.value) return; const sel = []; const rows = fileTableEl.value?.querySelectorAll('.file-row, .grid-item'); if (!rows) return; const r = dragRect.value; for (const row of rows) { const rr = row.getBoundingClientRect(); const ftRect = fileTableEl.value.getBoundingClientRect(); const rl = rr.left - ftRect.left; const rt = rr.top - ftRect.top; if (rl + rr.width > r.left && rl < r.left + r.width && rt + rr.height > r.top && rt < r.top + r.height) { const f = files.value.find(x => x.path === row.dataset.path); if (f) { if (!sel.some(s => s.path === f.path)) sel.push(f) } } }; selectedFiles.value = sel; dragRect.value = null }
function onDragStart(e, f) { if (!isSelected(f)) selectedFiles.value = [f]; e.dataTransfer.setData('text/plain', JSON.stringify(selectedFiles.value.map(x => x.path))); e.dataTransfer.effectAllowed = 'move' }

const ctxItems = computed(() => { const target = ctxMenu.value.target; if (!target) return []
  if (ctxMenu.value.isTree) { return [{ key: 'open', label: '打开' }, { key: 'mkdir', label: '新建文件夹' }, { key: 'delete', label: '删除目录', danger: true }] }
  const f = target; const items = []; items.push({ key: 'preview', label: '预览' }); if (!f.is_dir && !f.isUploading && isTextExt(f)) items.push({ key: 'edit', label: '编辑' }); items.push({ key: 'rename', label: '重命名' }); items.push({ key: 'copy', label: '复制' }); items.push({ key: 'cut', label: '剪切' }); if (!f.is_dir && !f.isUploading) { items.push({ key: 'download', label: '下载' }); items.push({ key: 'share', label: '分享' }) }; items.push({ key: 'stat', label: '属性' }); items.push({ key: 'delete', label: '删除', danger: true }); return items })
function onTreeContext(e, node) { ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: node, isTree: true } }
function onContextMenu(e, f) { if (!isSelected(f)) selectedFiles.value = [f]; ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: f } }
function onCtxAction(key) { const target = ctxMenu.value.target; const isTree = ctxMenu.value.isTree; ctxMenu.value.visible = false
  if (isTree) { switch (key) { case 'open': navigateTo(target.path); break; case 'mkdir': currentPath.value = target.path; showMkdir = true; break; case 'delete': deleteTreeDir(target); break } return }
  const f = target
  switch (key) { case 'preview': openPreview(f); break; case 'edit': startEdit(f); break; case 'rename': startRename(f); break; case 'copy': clipboard.value = { files: [f], mode: 'copy' }; saveClipboard(); showToast('已复制'); break; case 'cut': clipboard.value = { files: [f], mode: 'cut' }; saveClipboard(); showToast('已剪切'); break; case 'download': downloadFile(f); break; case 'share': startShare(f); break; case 'stat': showStat(f); break; case 'delete': deleteFile(f); break } }
async function deleteTreeDir(node) { const ok = await showConfirm('删除目录', '确认删除 ' + node.path + ' 及其所有内容？'); if (!ok) return; try { await api.delete('/api/nas/files', { data: { path: node.path } }); loadFiles(); showToast('已删除') } catch (e) { showToast('删除失败', 'error') } }
function onVideoError() { showToast('视频加载失败', 'error') }

uploadStore.onAllComplete(() => { setTimeout(() => loadFiles(), 500) })
onMounted(() => { loadFiles(); uploadStore.resumeFromStorage() })
</script>

<style scoped>
.files-page { display: flex; flex-direction: column; flex: 1; min-height: 0; position: relative; outline: none; }

/* Toast & Drop */
.toast { position: fixed; top: 80px; left: 50%; transform: translateX(-50%); z-index: 500; padding: 8px 20px; border-radius: 100px; font-size: 12.5px; font-weight: 500; pointer-events: none; }
.toast-info { background: var(--brand-600); color: #fff; }
.toast-error { background: var(--color-danger); color: #fff; }
.drop-zone { position: fixed; inset: 0; z-index: 400; background: rgba(6,182,212,0.08); backdrop-filter: blur(4px); display: flex; align-items: center; justify-content: center; }
.drop-zone-inner { display: flex; flex-direction: column; align-items: center; gap: 12px; color: var(--brand-500); font-size: 16px; font-weight: 600; padding: 48px; border: 2px dashed var(--brand-400); border-radius: 24px; background: rgba(255,255,255,0.5); }

/* Header */
.files-header { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-bottom: 8px; }
.header-meta { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.disk-mini { display: flex; align-items: center; gap: 6px; }
.disk-mini-bar { width: 48px; height: 4px; border-radius: 2px; background: var(--border-default); overflow: hidden; }
.disk-mini-fill { height: 100%; border-radius: 2px; background: var(--brand-500); transition: width 0.3s; }
.disk-mini.warn .disk-mini-fill { background: var(--color-warning); }
.disk-mini.danger .disk-mini-fill { background: var(--color-danger); }
.disk-mini-text { font-size: 10.5px; color: var(--text-muted); font-family: var(--font-mono); }
.sandbox-tag { font-size: 10.5px; padding: 3px 8px; border-radius: 100px; background: var(--brand-50); color: var(--brand-600); border: 1px solid var(--brand-200); }

/* Toolbar */
.files-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 6px; margin-bottom: 8px; flex-wrap: wrap; }
.toolbar-left, .toolbar-right { display: flex; align-items: center; gap: 3px; flex-wrap: wrap; }
.tb-btn { height: 28px; padding: 0 8px; border-radius: 6px; border: 1px solid var(--border-default); background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 11px; font-family: inherit; display: flex; align-items: center; gap: 4px; transition: all 0.12s; white-space: nowrap; }
.tb-btn:hover { background: var(--surface-hover); color: var(--text-primary); }
.tb-btn.on { background: var(--brand-50); color: var(--brand-600); border-color: var(--brand-300); }
.tb-btn.accent { background: var(--brand-50); color: var(--brand-600); border-color: var(--brand-200); }
.tb-btn.danger { color: var(--color-danger); }
.sel-count { font-size: 10.5px; color: var(--brand-600); font-weight: 600; margin: 0 4px; }
.sort-group { display: flex; gap: 1px; border-radius: 6px; border: 1px solid var(--border-default); overflow: hidden; margin-left: 4px; }
.sort-pill { border: none; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 10.5px; padding: 4px 8px; font-family: inherit; display: flex; align-items: center; gap: 2px; }
.sort-pill.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }
.sort-arrow.flip { transform: rotate(180deg); }
.type-filter { display: flex; gap: 1px; border-radius: 6px; border: 1px solid var(--border-default); overflow: hidden; margin-left: 4px; }
.type-pill { border: none; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 10.5px; padding: 4px 6px; font-family: inherit; }
.type-pill.active { background: var(--brand-50); color: var(--brand-600); font-weight: 600; }
.search-wrap { display: flex; align-items: center; gap: 4px; padding: 2px 8px; border: 1px solid var(--border-default); border-radius: 6px; background: transparent; margin-left: 4px; }
.search-inp { border: none; background: transparent; outline: none; font-size: 11px; color: var(--text-primary); width: 100px; font-family: inherit; }
.search-clr { border: none; background: none; color: var(--text-muted); cursor: pointer; font-size: 14px; padding: 0 2px; }

/* Three-panel body */
.files-body { display: flex; gap: 10px; flex: 1; min-height: 0; }
.file-tree-panel { width: 200px; flex-shrink: 0; border: 1px solid var(--border-default); border-radius: var(--radius-lg); overflow: hidden; background: var(--glass-bg-card); display: flex; flex-direction: column; }
.tree-head { padding: 8px 12px; font-size: 12px; font-weight: 600; color: var(--text-primary); border-bottom: 1px solid var(--border-subtle); }
.tree-list { flex: 1; overflow-y: auto; padding: 4px 0; }
.tree-item { display: flex; align-items: center; gap: 6px; padding: 5px 12px; cursor: pointer; font-size: 12px; color: var(--text-secondary); transition: background 0.1s; }
.tree-item:hover { background: var(--surface-hover); }
.tree-item.active { background: color-mix(in srgb, var(--brand-500) 8%, transparent); color: var(--brand-600); font-weight: 600; }
.tree-icon { font-size: 13px; }
.tree-arrow { width: 14px; height: 14px; display: flex; align-items: center; justify-content: center; font-size: 8px; color: var(--text-muted); cursor: pointer; border-radius: 3px; flex-shrink: 0; transition: transform 0.15s; }
.tree-arrow:hover { background: var(--surface-hover); color: var(--text-primary); }
.tree-arrow.expanded { transform: rotate(90deg); }
.tree-arrow-spacer { width: 14px; flex-shrink: 0; }
.tree-root .tree-arrow { margin-left: -14px; }
.tree-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tree-indent { display: inline-flex; flex-shrink: 0; height: 22px; position: relative; vertical-align: middle; }
.tree-line { display: inline-block; width: 14px; height: 22px; position: relative; flex-shrink: 0; }
.tree-line::before { content: ''; position: absolute; left: 6px; top: 0; bottom: 0; border-left: 1px solid var(--border-default); }
.tree-line.line-end::before { height: 9px; }
.tree-line.line-branch::before { height: 9px; }
.tree-line.line-end::after { content: ''; position: absolute; left: 6px; top: 9px; width: 8px; height: 0; border-top: 1px solid var(--border-default); }
.tree-line.line-branch::after { content: ''; position: absolute; left: 6px; top: 9px; width: 8px; height: 0; border-top: 1px solid var(--border-default); }
[data-theme="dark"] .tree-line::before, [data-theme="dark"] .tree-line::after { border-color: rgba(255,255,255,0.1); }
.tree-loading { text-align: center; padding: 10px; color: var(--text-muted); font-size: 11px; }
.files-center { flex: 1; min-width: 0; }
.file-list-wrap { flex: 1; overflow-y: auto; border-radius: var(--radius-lg); background: var(--glass-bg-card); border: 1px solid var(--glass-border); position: relative; min-height: 200px; }

/* List view */
.file-list { min-height: 0; }
.file-row { display: flex; align-items: center; gap: 10px; padding: 9px 14px; cursor: default; transition: background 0.1s; border-bottom: 1px solid var(--border-subtle); }
.file-row:last-child { border-bottom: none; }
.file-row:hover { background: rgba(0,0,0,0.02); }
.file-row.selected { background: color-mix(in srgb, var(--brand-500) 8%, transparent); }
.file-row.uploading { opacity: 0.7; background: rgba(6,182,212,0.03); }
.file-row.uploading .file-name { color: var(--brand-500); font-style: italic; }
.file-row.cut-mark { opacity: 0.45; }
.file-check { flex-shrink: 0; cursor: pointer; display: flex; }
.file-icon { flex-shrink: 0; width: 24px; display: flex; justify-content: center; }
.file-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; color: var(--text-primary); }
.file-meta { display: flex; align-items: center; gap: 16px; flex-shrink: 0; }
.file-progress { font-size: 11px; color: var(--brand-500); font-family: var(--font-mono); }
.file-size { font-size: 11px; color: var(--text-muted); font-family: var(--font-mono); min-width: 52px; text-align: right; }
.file-time { font-size: 11px; color: var(--text-muted); min-width: 72px; text-align: right; }
.file-actions { display: flex; gap: 1px; opacity: 0; transition: opacity 0.1s; flex-shrink: 0; }
.file-row:hover .file-actions { opacity: 1; }
.act-btn { width: 26px; height: 26px; border-radius: 5px; border: none; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 12px; display: flex; align-items: center; justify-content: center; }
.act-btn:hover { background: var(--surface-hover); color: var(--text-primary); }
.act-btn.del:hover { background: rgba(220,38,38,0.08); color: var(--color-danger); }

/* Grid view */
.file-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 10px; padding: 12px; }
.grid-item { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 12px 8px; border-radius: 8px; cursor: pointer; transition: background 0.1s; text-align: center; }
.grid-item:hover { background: var(--surface-hover); }
.grid-item.selected { background: color-mix(in srgb, var(--brand-500) 8%, transparent); }
.grid-icon { width: 40px; height: 40px; display: flex; align-items: center; justify-content: center; }
.grid-name { font-size: 11px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }
.mini-progress { width: 100%; height: 2px; background: var(--surface-hover); border-radius: 1px; overflow: hidden; }
.mini-progress div { height: 100%; background: var(--brand-500); }

.drag-select-box { position: absolute; background: rgba(6,182,212,0.10); border: 1px solid var(--brand-400); border-radius: 4px; pointer-events: none; z-index: 10; }

/* Empty */
.empty-list { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 60px 20px; color: var(--text-muted); }
.empty-list p { margin: 0; font-size: 13px; }
.empty-upload-btn { display: flex; align-items: center; gap: 6px; padding: 8px 18px; border-radius: 8px; border: 1px dashed var(--brand-300); background: var(--brand-50); color: var(--brand-600); cursor: pointer; font-size: 12.5px; font-weight: 500; }
.loading-state { padding: 40px; text-align: center; color: var(--text-muted); }

/* Upload panel */
.upload-panel { width: 260px; flex-shrink: 0; border: 1px solid var(--border-default); border-radius: var(--radius-lg); overflow: hidden; background: var(--glass-bg-card); display: flex; flex-direction: column; }
.upload-panel.collapsed { width: auto; }
.up-head { display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; font-size: 12px; font-weight: 600; cursor: pointer; border-bottom: 1px solid var(--border-subtle); }
.up-dismiss { border: none; background: none; cursor: pointer; font-size: 14px; color: var(--text-muted); }
.up-body { flex: 1; overflow-y: auto; padding: 8px; }
.up-item { padding: 6px 0; border-bottom: 1px solid var(--border-subtle); }
.up-item:last-child { border-bottom: none; }
.up-name { font-size: 11px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.up-track { height: 3px; border-radius: 2px; background: var(--surface-hover); margin: 3px 0; overflow: hidden; }
.up-fill { height: 100%; background: var(--brand-500); transition: width 0.2s; }
.up-fill.done { background: #22c55e; }
.up-fill.error { background: var(--color-danger); }
.up-meta { display: flex; align-items: center; justify-content: space-between; font-size: 10px; }
.up-status { color: var(--text-muted); }
.up-status.done { color: #22c55e; }
.up-status.error { color: var(--color-danger); }
.up-cancel { border: none; background: none; cursor: pointer; color: var(--text-muted); font-size: 10px; }

/* Modals */
.modal-overlay { position: fixed; inset: 0; z-index: 500; background: rgba(15,23,42,0.3); backdrop-filter: blur(6px); display: flex; align-items: center; justify-content: center; }
.preview-overlay { align-items: flex-start; padding-top: 72px; overflow-y: auto; }
.modal-card { background: var(--glass-bg-elevated); border: 1px solid var(--glass-border-strong); border-radius: var(--radius-lg); padding: 20px; min-width: 320px; max-width: 420px; display: flex; flex-direction: column; gap: 12px; box-shadow: var(--glass-shadow-elevated); }
.modal-card.wide { max-width: 720px; width: 90vw; }
.modal-card h4 { margin: 0; font-size: 15px; font-weight: 600; color: var(--text-primary); }
.modal-input { width: 100%; padding: 8px 10px; border: 1px solid var(--border-default); border-radius: 8px; background: rgba(255,255,255,0.3); color: var(--text-primary); font-size: 13px; font-family: inherit; outline: none; box-sizing: border-box; }
.modal-input:focus { border-color: var(--border-focus); box-shadow: 0 0 0 3px var(--focus-ring); }
.modal-textarea { width: 100%; padding: 10px; border: 1px solid var(--border-default); border-radius: 8px; background: rgba(255,255,255,0.3); color: var(--text-primary); font-size: 13px; font-family: var(--font-mono); resize: vertical; outline: none; line-height: 1.5; }
.modal-textarea:focus { border-color: var(--border-focus); box-shadow: 0 0 0 3px var(--focus-ring); }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
.modal-field { display: flex; flex-direction: column; gap: 4px; }
.modal-field label { font-size: 11px; color: var(--text-muted); }
.modal-field .mono { font-family: var(--font-mono); font-size: 12px; color: var(--text-secondary); }
.btn-cancel { padding: 7px 16px; border-radius: 8px; border: 1px solid var(--border-default); background: transparent; color: var(--text-secondary); cursor: pointer; font-size: 12.5px; font-family: inherit; }
.btn-cancel:hover { background: var(--surface-hover); }
.btn-ok { padding: 7px 16px; border-radius: 8px; border: none; background: var(--gradient-brand-btn); color: #fff; cursor: pointer; font-size: 12.5px; font-family: inherit; font-weight: 500; }
.btn-ok:disabled { opacity: 0.4; cursor: default; }

/* Preview */
.preview-modal { background: var(--glass-bg-elevated); backdrop-filter: blur(var(--glass-blur-elevated)); border: 1px solid var(--glass-border-strong); border-radius: var(--radius-xl); width: min(900px, 92vw); max-height: 88vh; display: flex; flex-direction: column; overflow: hidden; box-shadow: var(--glass-shadow-elevated); animation: pvIn 0.2s ease-out; }
@keyframes pvIn { from { opacity: 0; transform: scale(0.95); } to { opacity: 1; transform: scale(1); } }
.preview-head { display: flex; align-items: center; gap: 10px; padding: 14px 20px; border-bottom: 1px solid var(--border-subtle); flex-shrink: 0; }
.preview-title { flex: 1; font-size: 14px; font-weight: 600; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-size { font-size: 11px; color: var(--text-muted); font-family: var(--font-mono); }
.preview-close { width: 28px; height: 28px; border: none; border-radius: 6px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 18px; display: flex; align-items: center; justify-content: center; }
.preview-body { flex: 1; overflow: auto; }
.preview-body video { max-width: 100%; max-height: 75vh; display: block; margin: 0 auto; }
.preview-body img { max-width: 100%; max-height: 75vh; object-fit: contain; border-radius: 4px; display: block; margin: 0 auto; padding: 12px; }
.preview-body .text-wrap { padding: 24px 28px; }
.preview-body .preview-text { margin: 0; font-family: var(--font-mono); font-size: 13px; line-height: 1.6; color: var(--text-primary); white-space: pre-wrap; word-break: break-all; max-height: 70vh; overflow: auto; }
.preview-body .preview-unsupported { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 48px; color: var(--text-muted); }

[data-theme="dark"] .sandbox-tag { background: rgba(6,182,212,0.10); color: #22d3ee; border-color: rgba(34,211,238,0.18); }
[data-theme="dark"] .tb-btn.on, [data-theme="dark"] .tb-btn.accent { background: rgba(6,182,212,0.12); color: #22d3ee; border-color: rgba(34,211,238,0.25); }

@media (max-width: 1023px) {
  .file-tree-panel { display: none; }
  .upload-panel { width: 220px; }
}
@media (max-width: 767px) {
  .files-body { flex-direction: column; }
  .upload-panel { width: 100%; max-height: 200px; }
  .files-page { padding-bottom: 80px; }
  .tb-btn { height: 36px; min-width: 36px; padding: 0 10px; font-size: 12px; }
  .file-row { padding: 10px 8px; gap: 8px; }
  .file-name { font-size: 12.5px; }
  .file-time, .file-size { display: none; }
  .file-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
