<template>
  <div class="files-toolbar">
    <div class="toolbar-left">
      <button
        class="tb-btn"
        :class="{ on: clickMode === 'select' }"
        :title="clickMode === 'select' ? '选择模式' : '打开模式'"
        @click="$emit('toggle-mode')"
      >
        <svg
          v-if="clickMode === 'select'"
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
        ><path d="M2 3h10M2 7h10M2 11h7" /><circle
          cx="11"
          cy="11"
          r="1"
          fill="currentColor"
          stroke="none"
        /></svg>
        <svg
          v-else
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
        ><path d="M2 4.5v7a1 1 0 001 1h8a1 1 0 001-1V6a1 1 0 00-1-1H6.5L5.5 3H3a1 1 0 00-1 1v.5" /><path d="M8 8.5l2-2-2-2" /></svg>
        <span class="mode-label">{{ clickMode === 'select' ? '选择' : '打开' }}</span>
      </button>
      <label
        class="tb-btn accent"
        title="上传文件"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
        ><path d="M7 10V2M4 5l3-3 3 3M2 9v3a1 1 0 001 1h8a1 1 0 001-1V9" /></svg>
        <input
          type="file"
          multiple
          hidden
          @change="$emit('upload', $event)"
        />
      </label>
      <label
        class="tb-btn accent"
        title="上传文件夹"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
        ><path d="M2 4v8a1 1 0 001 1h8a1 1 0 001-1V5a1 1 0 00-1-1H6.5L5.5 2.5H3a1 1 0 00-1 1v.5" /><path d="M7 7v3M5.5 8.5h3" /></svg>
        <input
          type="file"
          webkitdirectory
          directory
          multiple
          hidden
          @change="$emit('folder-upload', $event)"
        />
      </label>
      <button
        class="tb-btn"
        title="新建文件夹"
        @click="$emit('mkdir')"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
        ><path d="M2 4v8a1 1 0 001 1h8a1 1 0 001-1V5a1 1 0 00-1-1H6.5L5.5 2.5H3a1 1 0 00-1 1v.5" /><path d="M7 7v3M5.5 8.5h3" /></svg>
      </button>
      <button
        class="tb-btn"
        title="刷新"
        @click="$emit('refresh')"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
          stroke-linecap="round"
        ><path d="M12 7a5 5 0 11-1.5-3.5M12 2v4h-4" /></svg>
      </button>
    </div>
    <div class="toolbar-right">
      <template v-if="selectedCount">
        <span class="sel-count">{{ selectedCount }} 个已选</span>
        <button
          class="tb-btn"
          @click="$emit('cut')"
        >
          剪切
        </button>
        <button
          class="tb-btn"
          @click="$emit('copy')"
        >
          复制
        </button>
        <button
          class="tb-btn danger"
          @click="$emit('batch-delete')"
        >
          删除
        </button>
      </template>
      <button
        v-if="clipCount"
        class="tb-btn accent"
        @click="$emit('paste')"
      >
        粘贴({{ clipCount }})
      </button>
      <div class="sort-group">
        <button
          v-for="opt in sortOptions"
          :key="opt.key"
          :class="['sort-pill', { active: sortBy === opt.key }]"
          @click="$emit('toggle-sort', opt.key)"
        >
          {{ opt.label }}
          <svg
            v-if="sortBy === opt.key"
            :class="['sort-arrow', { flip: sortOrder === -1 }]"
            width="8"
            height="8"
            viewBox="0 0 8 8"
            fill="currentColor"
          ><path d="M4 1v6M1.5 4.5L4 7l2.5-2.5" /></svg>
        </button>
      </div>
      <div class="search-wrap">
        <svg
          class="search-icon"
          width="13"
          height="13"
          viewBox="0 0 13 13"
          fill="none"
          stroke="currentColor"
          stroke-width="1.6"
        ><circle
          cx="5.5"
          cy="5.5"
          r="3.5"
        /><path d="M8.5 8.5L12 12" /></svg>
        <input
          :value="searchQuery"
          placeholder="搜索"
          class="search-inp"
          @input="$emit('update:searchQuery', $event.target.value)"
        />
        <button
          v-if="searchQuery"
          class="search-clr"
          @click="$emit('update:searchQuery', '')"
        >
          &times;
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
defineProps({
  clickMode: String, selectedCount: Number, clipCount: Number,
  sortBy: String, sortOrder: Number, searchQuery: String,
})
defineEmits(['toggle-mode', 'upload', 'folder-upload', 'mkdir', 'refresh', 'cut', 'copy', 'batch-delete', 'paste', 'toggle-sort', 'update:searchQuery'])
const sortOptions = [{ key: 'name', label: '名称' }, { key: 'size', label: '大小' }, { key: 'time', label: '时间' }]
</script>

<style scoped>
.files-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
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
.search-wrap { display: flex; align-items: center; gap: 4px; padding: 2px 8px; border: 1px solid var(--border-default); border-radius: 6px; background: transparent; margin-left: 4px; }
.search-inp { border: none; background: transparent; outline: none; font-size: 11px; color: var(--text-primary); width: 100px; font-family: inherit; }
.search-clr { border: none; background: none; color: var(--text-muted); cursor: pointer; font-size: 14px; padding: 0 2px; }
</style>
