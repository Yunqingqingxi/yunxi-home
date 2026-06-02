<template>
  <div v-if="visible" class="interrupt-banner">
    <div class="banner-icon">⏸</div>
    <div class="banner-body">
      <span class="banner-text">对话已中断</span>
      <span v-if="progress > 0" class="banner-detail">
        — 进度 {{ progress }}%<span v-if="lastTask">，最后执行：{{ lastTask }}</span>
      </span>
    </div>
    <div class="banner-actions">
      <button class="banner-btn primary" @click="$emit('continue')" title="从断点继续执行">继续</button>
      <button class="banner-btn" @click="$emit('retry')" title="修改做法重新执行">修改做法</button>
      <button class="banner-btn" @click="$emit('dismiss')" title="放弃当前任务开始新对话">新任务</button>
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
  gap: 12px;
  padding: 10px 16px;
  margin: 0 16px 8px;
  background: linear-gradient(135deg, rgba(251,191,36,0.08), rgba(245,158,11,0.06));
  border: 1px solid rgba(251,191,36,0.3);
  border-radius: 12px;
  font-size: 13px;
  animation: bannerIn 0.3s ease;
}
@keyframes bannerIn {
  from { opacity: 0; transform: translateY(-6px); }
  to { opacity: 1; transform: translateY(0); }
}
.banner-icon {
  font-size: 18px;
  flex-shrink: 0;
}
.banner-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.banner-text {
  font-weight: 600;
  color: #b45309;
}
.banner-detail {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.banner-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.banner-btn {
  padding: 5px 12px;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  background: var(--surface-card);
  color: var(--text-primary);
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.12s;
  white-space: nowrap;
}
.banner-btn:hover {
  background: var(--surface-hover);
  border-color: var(--brand-300);
}
.banner-btn.primary {
  background: var(--brand-500);
  color: #fff;
  border-color: transparent;
}
.banner-btn.primary:hover {
  background: var(--brand-600);
}
[data-theme="dark"] .interrupt-banner {
  background: linear-gradient(135deg, rgba(251,191,36,0.1), rgba(245,158,11,0.04));
  border-color: rgba(251,191,36,0.2);
}
[data-theme="dark"] .banner-text {
  color: #fbbf24;
}
</style>
