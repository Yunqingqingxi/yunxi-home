<template>
  <div v-if="visible" class="interrupt-banner">
    <div class="banner-accent"></div>
    <div class="banner-icon">
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
        <rect x="4" y="2" width="3" height="12" rx="1"/>
        <rect x="9" y="2" width="3" height="12" rx="1"/>
      </svg>
    </div>
    <div class="banner-body">
      <span class="banner-title">对话已中断</span>
      <span v-if="progress > 0 || lastTask" class="banner-detail">
        <template v-if="progress > 0">进度 {{ progress }}%</template>
        <template v-if="lastTask">{{ progress > 0 ? '，' : '' }}最后执行：{{ lastTask }}</template>
      </span>
    </div>
    <div class="banner-actions">
      <button class="bn-btn primary" @click="$emit('continue')">继续</button>
      <button class="bn-btn" @click="$emit('retry')">修改做法</button>
      <button class="bn-btn" @click="$emit('dismiss')">新任务</button>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  visible: boolean
  progress: number
  lastTask: string
}>()

defineEmits<{
  continue: []
  retry: []
  dismiss: []
}>()
</script>

<style scoped>
.interrupt-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  margin: 0 4px 8px;
  background: var(--surface-card);
  border: 1px solid var(--border-default);
  border-radius: 10px;
  font-size: 13px;
  overflow: hidden;
  position: relative;
  animation: bannerSlideIn 0.25s var(--ease-out-expo, ease-out);
  box-shadow: var(--shadow-xs, 0 1px 2px rgba(0,0,0,0.04));
}
.banner-accent {
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--color-warning, #d2991d);
  border-radius: 3px 0 0 3px;
}
@keyframes bannerSlideIn {
  from { opacity: 0; transform: translateY(-4px); }
  to   { opacity: 1; transform: translateY(0); }
}
.banner-icon {
  color: var(--color-warning, #d2991d);
  flex-shrink: 0;
  display: flex;
  align-items: center;
}
.banner-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.banner-title {
  font-weight: 600;
  color: var(--text-primary);
  font-size: 13px;
}
.banner-detail {
  font-size: 12px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.banner-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.bn-btn {
  padding: 5px 12px;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
  line-height: 1.4;
}
.bn-btn:hover {
  background: var(--surface-hover);
  color: var(--text-primary);
  border-color: var(--border-strong);
}
.bn-btn.primary {
  background: var(--brand-500);
  color: #fff;
  border-color: transparent;
  font-weight: 500;
}
.bn-btn.primary:hover {
  background: var(--brand-600);
}
</style>
