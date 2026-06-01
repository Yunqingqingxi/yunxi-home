<template>
  <code class="cell-code"><slot /></code>
</template>

<style scoped>
.cell-code {
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  background: var(--code-bg);
  padding: 2px 7px;
  border-radius: var(--radius-sm);
  color: var(--code-color);
  white-space: nowrap;
}
</style>