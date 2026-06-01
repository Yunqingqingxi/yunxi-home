<template>
  <div class="home-state">
    <div class="home-core">
      <svg
        class="core-ring"
        viewBox="0 0 120 120"
        fill="none"
      >
        <circle
          cx="60"
          cy="60"
          r="52"
          stroke="var(--brand-300)"
          stroke-width="0.5"
          opacity="0.3"
        />
        <circle
          cx="60"
          cy="60"
          r="42"
          stroke="var(--brand-400)"
          stroke-width="0.6"
          opacity="0.4"
        />
        <circle
          cx="60"
          cy="60"
          r="24"
          stroke="var(--brand-500)"
          stroke-width="0.8"
          opacity="0.6"
        />
        <circle
          cx="60"
          cy="60"
          r="7"
          fill="var(--brand-500)"
          opacity="0.9"
        >
          <animate
            attributeName="r"
            values="6;8.5;6"
            dur="3s"
            repeatCount="indefinite"
          />
          <animate
            attributeName="opacity"
            values="0.7;1;0.7"
            dur="3s"
            repeatCount="indefinite"
          />
        </circle>
      </svg>
      <div class="core-orbits">
        <span
          v-for="i in 12"
          :key="i"
          class="orbit-dot"
          :style="orbitStyle(i)"
        ></span>
      </div>
    </div>
    <h1 class="home-title">
      云 兮
    </h1>
    <p class="home-desc">
      你的全能家庭服务器运维伙伴
    </p>
    <div class="home-caps">
      <button
        v-for="c in caps"
        :key="c.label"
        class="cap-pill"
        @click="$emit('quickStart', c.prompt)"
      >
        <span
          class="cap-icon"
          v-html="c.icon"
        ></span>
        {{ c.label }}
      </button>
    </div>
    <div class="home-quick-prompts">
      <button
        v-for="p in quickPrompts"
        :key="p"
        class="hint-chip"
        @click="$emit('quickStart', p)"
      >
        {{ p }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
// @ts-nocheck
const emit = defineEmits(['quickStart'])

const caps = [
  { label: '文件管理', prompt: '帮我管理文件，列出根目录', icon: '<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><path d="M2 2h3.5L6.5 3.5H10a1 1 0 011 1v4a1 1 0 01-1 1H2a1 1 0 01-1-1V3a1 1 0 011-1z"/></svg>' },
  { label: 'DNS 域名', prompt: '查看 DNS 域名和更新记录', icon: '<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><circle cx="6" cy="6" r="4.5"/><ellipse cx="6" cy="6" rx="1.8" ry="4.5"/><path d="M1.5 6h9M6 1.5v9"/></svg>' },
  { label: '系统监控', prompt: '查看系统状态和磁盘使用情况', icon: '<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x="1.5" y="2" width="9" height="6.5" rx="1.2"/><path d="M4 10h4M6 8.5V10"/></svg>' },
  { label: 'Docker', prompt: '检查 Docker 容器运行状态', icon: '<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.2"><rect x="1" y="2.5" width="10" height="7" rx="1"/><path d="M3.5 2.5v7M7.5 2.5v7M1 5h10"/></svg>' },
]

const quickPrompts = [
  '搜索最近的日志文件',
  '查看网络连接状态',
  '检查服务运行状态',
]

function orbitStyle(i) {
  const angle = (i - 1) * 30
  const r = 44 + Math.sin(i * 2.3) * 8
  const x = 60 + r * Math.cos(angle * Math.PI / 180)
  const y = 60 + r * Math.sin(angle * Math.PI / 180)
  const size = 2.5 + Math.random() * 2
  const delay = Math.random() * 3
  return {
    left: x + 'px', top: y + 'px',
    width: size + 'px', height: size + 'px',
    animationDelay: delay + 's'
  }
}
</script>

<style scoped>
.home-state {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  gap: 10px; text-align: center; padding-top: min(12vh, 120px);
  overflow-y: auto;
}
.home-core {
  position: relative; width: 120px; height: 120px; margin-bottom: 8px;
}
.core-ring {
  position: absolute; inset: 0; width: 120px; height: 120px;
}
.orbit-dot {
  position: absolute; border-radius: 50%; background: var(--brand-400);
  opacity: 0; animation: orbitPulse 3s ease-in-out infinite;
}
@keyframes orbitPulse {
  0%, 100% { opacity: 0; transform: scale(0); }
  30% { opacity: 0.6; }
  60% { opacity: 0.15; transform: scale(1.2); }
}

.home-title {
  font-size: 28px; font-weight: var(--weight-bold); color: var(--text-primary);
  letter-spacing: 0.08em; margin: 0;
}
.home-desc {
  font-size: var(--text-sm); color: var(--text-muted); margin: 0;
}
.home-caps {
  display: flex; flex-wrap: wrap; gap: 6px; justify-content: center; max-width: 440px; margin-top: 4px;
}
.cap-pill {
  font-size: 11px; padding: 5px 14px; border-radius: 100px;
  background: var(--brand-50); color: var(--brand-600);
  border: 1px solid var(--brand-200); font-weight: var(--weight-medium);
  display: flex; align-items: center; gap: 5px; transition: all 0.2s;
}
.cap-pill:hover { transform: translateY(-1px); box-shadow: 0 2px 8px rgba(6,182,212,0.12); }
.cap-icon { display: flex; align-items: center; }
[data-theme="dark"] .cap-pill { background: rgba(6,182,212,0.08); color: #22d3ee; border-color: rgba(34,211,238,0.18); }

.home-quick-prompts {
  display: flex; flex-wrap: wrap; gap: 6px; justify-content: center;
  max-width: 440px; margin-top: 2px;
}
.hint-chip {
  padding: 5px 14px; border-radius: 100px; font-size: 11px;
  font-family: inherit; border: 1px solid var(--border-default);
  background: transparent; color: var(--text-muted);
  cursor: pointer; transition: all 0.15s;
}
.hint-chip:hover {
  border-color: var(--brand-400); color: var(--brand-500);
  background: var(--brand-50);
}
[data-theme="dark"] .hint-chip:hover {
  background: rgba(6,182,212,0.08); color: #22d3ee;
}

@media (max-width: 767px) {
  .home-title { font-size: 22px; }
  .home-core { width: 100px; height: 100px; }
  .core-ring { width: 100px; height: 100px; }
  .home-caps { max-width: 340px; }
  .cap-pill { font-size: 10px; padding: 4px 10px; }
}
</style>
