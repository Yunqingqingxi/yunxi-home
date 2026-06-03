<template>
  <div v-if="visible" class="interrupt-banner" :class="{ locked: trustLocked }">
    <div class="banner-accent" :class="trustLocked ? 'locked' : ''"></div>
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
      <!-- 拓扑信任状态 -->
      <span v-if="(trustLocked ?? false) || (trustLies ?? 0) > 0 || (rejectCount ?? 0) >= 2" class="banner-topo">
        <span v-if="trustLocked" class="topo-tag locked">信任已锁定</span>
        <span v-else-if="(trustLies ?? 0) >= 2" class="topo-tag warn">信任需注意 ({{ trustLies ?? 0 }})</span>
        <span v-if="(rejectCount ?? 0) >= 2 && !trustLocked" class="topo-tag reject">连续拒绝 {{ rejectCount ?? 0 }} 次</span>
        <span v-if="warning && !trustLocked" class="topo-warning">{{ warning }}</span>
      </span>
    </div>
    <div class="banner-actions">
      <!-- 拓扑操作按钮（按会话） -->
      <button v-if="trustLocked" class="bn-btn warn" @click="$emit('unlock-trust')" title="手动解锁信任">解锁信任</button>
      <button v-if="(rejectCount ?? 0) >= 2" class="bn-btn" @click="$emit('override')" title="放行下一轮拓扑校验">放行一次</button>
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
  trustLocked?: boolean
  trustLies?: number
  rejectCount?: number
  warning?: string
}>()

defineEmits<{
  continue: []
  retry: []
  dismiss: []
  'unlock-trust': []
  override: []
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
.interrupt-banner.locked {
  border-color: rgba(239,68,68,0.3);
}
.banner-accent {
  position: absolute;
  left: 0; top: 0; bottom: 0;
  width: 3px;
  background: var(--color-warning, #d2991d);
  border-radius: 3px 0 0 3px;
}
.banner-accent.locked {
  background: #ef4444;
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
.banner-topo {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 2px;
}
.topo-tag {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
  font-weight: 500;
  white-space: nowrap;
}
.topo-tag.locked {
  background: rgba(239,68,68,0.12);
  color: #ef4444;
}
.topo-tag.warn {
  background: rgba(245,158,11,0.12);
  color: #d97706;
}
.topo-tag.reject {
  background: rgba(245,158,11,0.12);
  color: #d97706;
}
.topo-warning {
  font-size: 10px;
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
.bn-btn.warn {
  color: #ef4444;
  border-color: rgba(239,68,68,0.3);
}
.bn-btn.warn:hover {
  background: rgba(239,68,68,0.08);
  border-color: #ef4444;
}
</style>
