<template>
  <div class="page-header">
    <h3>{{ title }}</h3>
    <div
      v-if="$slots.actions"
      class="ph-actions"
    >
      <slot name="actions"></slot>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
defineProps({ title: { type: String, required: true } })
</script>

<style scoped>
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 4px;
}
.page-header h3 {
  margin: 0;
  font-size: var(--text-xl);
  font-weight: var(--weight-bold);
  color: var(--text-primary);
  letter-spacing: -0.01em;
}
.ph-actions { display: flex; align-items: center; gap: 8px; }
@media (max-width: 767px) {
  .page-header { flex-direction: column; align-items: flex-start; gap: 4px; }
  .ph-actions { width: 100%; }
}
</style>