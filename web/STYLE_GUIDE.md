# Yunxi Home — 前端设计系统文档

## 1. 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 框架 | Vue 3 (Composition API) | ^3.4 |
| 构建 | Vite | ^5.2 |
| 路由 | Vue Router 4 (Hash 模式) | ^4.3 |
| 状态管理 | Pinia | ^2.1 |
| UI 组件库 | Arco Design Vue | ^2.55 |
| 图表 | Chart.js + vue-chartjs | ^4.5 |
| 终端 | xterm.js | ^6.0 |
| Markdown | marked | ^18.0 |
| HTML 清洗 | DOMPurify | ^3.4 |

所有页面通过 `createWebHashHistory` 路由，由 `router.beforeEach` 守卫校验 token；登录页 (`/login`) 不拦截。

---

## 2. 主题系统

### 2.1 品牌色

品牌色为 **Cyan（青色）** 系列，以 Tailwind `cyan-500` (`#06b6d4`) 为主色。

```
--brand-50:  #ecfeff     ← 最浅底色
--brand-100: #cffafe
--brand-200: #a5f3fc
--brand-300: #67e8f9     ← 亮色装饰
--brand-400: #22d3ee     ← 高亮/悬停
--brand-500: #06b6d4     ← 主色 PRIMARY
--brand-600: #0891b2     ← 深色强调
--brand-700: #0e7490
--brand-800: #155e75
--brand-900: #164e63     ← 最深底色
```

**语义色：**
- Success: `#16a34a` (light) / `#22c55e` (dark)
- Danger: `#dc2626` (light) / `#ef4444` (dark)
- Warning: `#d97706` (light) / `#f59e0b` (dark)

### 2.2 亮色 / 暗色切换

`useThemeStore` 管理 `data-theme` 属性，设置在 `<html>` 上，值为 `"light"` 或 `"dark"`。通过 `localStorage` (`yunxi-theme`) 持久化；未显式设置时跟随系统 `prefers-color-scheme`。

暗色主题的 CSS 变量通过 `[data-theme="dark"]` 选择器覆写，定义在 `web/src/styles/dark.css`。

---

## 3. 字体系统

### 3.1 字体栈

```css
--font-sans: "Noto Sans SC", "HarmonyOS Sans SC", "PingFang SC",
             "Microsoft YaHei", system-ui, sans-serif;
--font-mono: "Cascadia Code", "Fira Code", Consolas, "JetBrains Mono", monospace;
--font-body: "Noto Sans SC", "HarmonyOS Sans SC", "PingFang SC",
             "Microsoft YaHei", system-ui, sans-serif;
```

- **默认正文**：`--font-sans`（中英文混合场景用 Noto Sans SC）
- **代码/数据**：`--font-mono`（等宽字体，用于 IP 地址、版本号、终端、代码块）
- **特殊标注**：`.mono-text` 工具类强制等宽字体

### 3.2 字号阶梯

| Token | 值 | 典型用途 |
|-------|-----|---------|
| `--text-2xs` | 10.5px | 版本号、辅助说明 |
| `--text-xs` | 11.5px | 标签、徽标、badge |
| `--text-sm` | 12.5px | 副文字、图标标注 |
| `--text-base` | 14px | 正文、输入框、列表 |
| `--text-md` | 15.5px | 卡片标题、次级标题 |
| `--text-lg` | 18px | 分区标题 |
| `--text-xl` | 22px | 页面标题 (h3) |
| `--text-2xl` | 28px | 大标题 (h2) |
| `--text-3xl` | 36px | 最大标题 (h1) |

### 3.3 字重

```css
--weight-normal: 400;   /* 正文 */
--weight-medium: 500;   /* 标签、按钮文字 */
--weight-semibold: 600; /* 卡片标题、强调 */
--weight-bold: 700;     /* 页面标题、数值 */
```

---

## 4. 布局架构

### 4.1 AppShell 结构

```
┌─────────────────────────────────────────┐
│ .app-header (52px, sticky, glass)       │  ← 菜单按钮 · Logo · 主题切换
├──────────┬──────────────────────────────┤
│ .app-sider│ .app-main (flex: 1)          │
│ (220px)  │  · PageHeader 组件           │
│          │  · 内容区 (router-view)       │
│ 导航项   │  · card-glass 卡片            │
│ 对话树   │                              │
│ 退出     │                              │
└──────────┴──────────────────────────────┘
```

- **Header**：`position: sticky; top: 0; z-index: 110`，高度固定 52px
- **Sidebar**：220px 宽，`position: sticky; top: 52px`，`height: calc(100vh - 52px)`
- **Main**：`flex: 1; min-width: 0`，padding 在桌面端为 `24px`，移动端为 `12px`

### 4.2 响应式断点

| 断点 | 行为 |
|------|------|
| > 768px | 侧栏固定可见，主内容区 padding: 24px |
| ≤ 767px | 侧栏变为抽屉式（`transform: translateX(-100%)`），汉堡菜单按钮显示 |

### 4.3 布局规则

- **全部使用 Flexbox**，不混用 Grid 做整体布局
- 页面内部内容区使用 `display: flex; flex-direction: column;`
- 卡片内部使用 flex row 或 flex column 排版
- 表单使用 Arco 默认的 `layout="vertical"`（标签在上，输入框在下）

---

## 5. Glass Morphism（玻璃质感）设计

### 5.1 核心玻璃 token

```css
--glass-bg: linear-gradient(165deg,
  rgba(255,255,255,0.48), rgba(255,255,255,0.26) 30%,
  rgba(236,254,255,0.32) 70%, rgba(255,255,255,0.42));
--glass-blur: blur(38px) saturate(190%);
--glass-border: 1px solid rgba(255,255,255,0.38);
--glass-highlight: linear-gradient(180deg,
  rgba(255,255,255,0.50) 0%, transparent 100%);
```

暗色模式下：
```css
--glass-bg: linear-gradient(165deg,
  rgba(18,26,44,0.58), rgba(18,26,44,0.34) 30%,
  rgba(6,182,212,0.06) 70%, rgba(18,26,44,0.46));
--glass-blur: blur(42px) saturate(170%);
--glass-border: 1px solid rgba(255,255,255,0.09);
```

### 5.2 卡片类 `.card-glass`

全局 CSS 类，所有视图页面统一使用：

```css
.card-glass {
  background: var(--surface-card);
  backdrop-filter: var(--glass-blur);
  border: var(--glass-border);
  border-radius: var(--radius-lg); /* 16px */
  box-shadow: var(--shadow-glass), inset 0 0 0 1px rgba(255,255,255,0.30);
  position: relative;
  overflow: hidden;
}
.card-glass::before {
  /* 顶部高光线 */
  content: "";
  position: absolute; top: 0; left: 0; right: 0; height: 1px;
  background: linear-gradient(180deg, rgba(255,255,255,0.50), transparent);
}
.card-glass:hover {
  box-shadow: var(--shadow-glass-hover);
  transform: translateY(-1px);
}

/* 紧凑变体 */
.card-glass-sm { border-radius: var(--radius-md); /* 12px */ }
```

**使用规则：** 每个独立数据区块必须包裹在 `.card-glass` 内。

---

## 6. 表面与阴影

### 6.1 表面层级 (Surface)

| Token | 用途 | Light 值 |
|-------|------|---------|
| `--surface-page` | 页面背景 | `#e4e9f2` |
| `--surface-card` | 玻璃卡片 | `rgba(255,255,255,0.48)` |
| `--surface-header` | 顶栏 | `rgba(255,255,255,0.55)` |
| `--surface-sidebar` | 侧栏 | `rgba(255,255,255,0.40)` |
| `--surface-modal` | 模态框 | `rgba(255,255,255,0.65)` |
| `--surface-input` | 输入框 | `rgba(255,255,255,0.55)` |
| `--surface-raised` | 浮层面板 | `rgba(255,255,255,0.65)` |
| `--surface-footer` | 底部栏 | `rgba(255,255,255,0.36)` |

### 6.2 阴影

```css
--shadow-glass: 0 8px 44px rgba(0,0,0,0.06),
                inset 0 1px 0 rgba(255,255,255,0.55);
--shadow-glass-hover: 0 16px 56px rgba(0,0,0,0.09),
                       inset 0 1px 0 rgba(255,255,255,0.65);
--shadow-modal: 0 25px 80px rgba(0,0,0,0.12),
                0 8px 24px rgba(6,182,212,0.07);
--shadow-btn: 0 2px 10px rgba(6,182,212,0.32);
--shadow-btn-hover: 0 4px 18px rgba(6,182,212,0.44);
```

---

## 7. 组件样式规范

### 7.1 按钮

- 主按钮：`--gradient-brand-btn` 渐变 + `box-shadow: var(--shadow-btn)`
- hover 时：渐变加深 + `translateY(-1px)` + 光泽扫过（`::after` pseudo-element）
- 圆角：12px
- 字重：Semibold

### 7.2 表格

- 全局玻璃化：`background: var(--surface-card)` + `backdrop-filter`
- 表头背景：`rgba(6,182,212,0.05)`，底部 2px 品牌色下划线
- 行 hover：`var(--surface-card-hover)`
- 条纹行：`nth-child(even)` 背景 `rgba(6,182,212,0.03)`
- 行入场动画：`tableRowIn` (0.3s ease-out-expo)，每行依次延迟 0.02s

### 7.3 输入框

- 玻璃背景：`var(--surface-input)` + `backdrop-filter: blur(12px) saturate(160%)`
- 聚焦：品牌色边框 + `box-shadow: 0 0 0 3px var(--focus-ring)`
- 圆角：12px

### 7.4 模态框 (Modal)

- 玻璃面板 + 顶部品牌色渐变光线（cyan 从左到右 0→50%→100%→0）
- 入场动画：`modalIn` (0.3s ease-out-back, scale 0.92→1 + translateY)

### 7.5 标签 (Tag)

- 绿色：`--tag-success-bg` / `--tag-success-border` / `--color-success`
- 红色：`--tag-danger-bg` / `--tag-danger-border` / `--color-danger`
- 品牌色：`rgba(6,182,212,0.09)` / `rgba(6,182,212,0.16)`
- 入场动画：`tagIn` (0.25s ease-out-back)

### 7.6 选项卡 (Tabs)

- 激活态：品牌色文字 + 品牌色下划线
- 暗色模式：文字 `--text-muted` → hover `--text-primary` → active `--brand-400`

---

## 8. 动效与微交互

### 8.1 缓动函数

```css
--ease-out-expo: cubic-bezier(0.16, 1, 0.3, 1);   /* 平滑减速 */
--ease-out-back: cubic-bezier(0.34, 1.56, 0.64, 1); /* 带回弹 */
--ease-in-out:   cubic-bezier(0.4, 0, 0.2, 1);       /* 标准缓入缓出 */
```

### 8.2 动画时长

```css
--duration-fast:   0.15s;  /* 微交互 */
--duration-normal: 0.25s;  /* 常规过渡 */
--duration-slow:   0.4s;   /* 入场动画 */
```

### 8.3 页面级动画

| 动画 | 效果 | 时长 | 缓动 |
|------|------|------|------|
| 页面切换 | opacity + translateY(6px→0) | 0.22s | ease-out-expo |
| 表格行入场 | opacity + translateX(-6px→0) | 0.3s | ease-out-expo |
| 模态框入场 | scale(0.92→1) + translateY | 0.3s | ease-out-back |
| 标签入场 | scale(0.8→1) | 0.25s | ease-out-back |
| 卡片 hover | translateY(-1px) | 0.2s | ease |
| 按钮光泽扫过 | translateX(-100%→100%) | 0.6s | ease |

### 8.4 背景微动

页面背景包含多个 `radial-gradient` 椭圆，通过 `bgShimmer` 关键帧动画缓慢交替透明度（14s 周期），营造呼吸感。

---

## 9. 间距与圆角

### 9.1 间距阶梯

| Token | 值 | 用途 |
|-------|-----|------|
| `--space-1` | 4px | 微距 |
| `--space-2` | 8px | 图标间距 |
| `--space-3` | 12px | 组件内 gap |
| `--space-4` | 16px | 区块内 padding |
| `--space-5` | 20px | 卡片 padding |
| `--space-6` | 24px | 页面外距 |
| `--space-8` | 32px | 大间距 |

### 9.2 圆角

| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | 8px | 小按钮、输入框 |
| `--radius-md` | 12px | 标准卡片、浮层 |
| `--radius-lg` | 16px | 玻璃卡片 |
| `--radius-xl` | 22px | 模态框、登录卡片 |

---

## 10. 各页面风格

### 10.1 仪表盘 (Dashboard)

- 布局：单列 flex column，`gap: 16px`
- 每块数据一个 `.card-glass` 卡片
- 统计条 `.stat-strip`：flex row wrap，每项 `.stat-badge`（带 hover 微上浮 + 径向光晕）
- CPU/内存：`Doughnut` 圆环图（cutout: 78%），中心叠加百分比文字
- 系统负载：`Bar` 水平柱状图（indexAxis: "y"）
- 网络接口：树形结构（可折叠），IP 地址用等宽字体，IPv4/IPv6 分色标签

### 10.2 文件管理 (Files)

- 工具栏：面包屑 + 搜索框 + 操作按钮（flex row）
- 文件列表：Arco Table 全玻璃化
- 内联模态框：新建文件夹、重命名、分享（自建 overlay + modal-card，不使用 Arco Modal）

### 10.3 设置 (Settings)

- Arco Tabs 切换 DNS / 通知 / 认证 / AI / 权限
- 每个 tab 内为 `.setting-group` 分组列表
- 密码类字段使用原生 `<input type="password">` 防复制

### 10.4 Chat (AI 助手)

- 全高 flex column 占满主区域
- 无会话时显示欢迎状态（图标 + 提示语 + 快捷提示按钮）
- 消息列表 `.panel-body`：flex column, `overflow-y: auto`
- 消息气泡 `.ai-blocks`：圆角 14px，微凸起表面
- 输入区：底部固定，圆角胶囊形状
- 流式输出支持：ThinkingBlock + ToolCallBlock + ContentBlock 交替渲染

### 10.5 登录页

- 全屏居中 flex
- 上方品牌标识区（Logo + 渐变标题）
- 下方玻璃化卡片表单（用户名 + 密码 + 错误提示）
- 标题使用 `background-clip: text` 渐变文字
- 多段入场动画（标题→副标题→卡片，依次延迟 0.1s）

### 10.6 终端 (Terminal)

- 使用 xterm.js 全屏终端模拟器
- 暗色背景，连接 WebSocket 实时通信

### 10.7 其他视图

- **Domains**：域名管理列表（表格 + 玻璃卡片）
- **History**：更新历史日志（时间线或表格）
- **System**：系统控制面板（服务启停、配置管理）
- **CloudDns**：阿里云 DNS 记录管理

---

## 11. 暗色模式覆盖

### 11.1 背景

```css
[data-theme="dark"] body {
  background:
    radial-gradient(ellipse 75% 55% at 22% 14%, rgba(6,182,212,0.07), transparent 68%),
    radial-gradient(ellipse 48% 42% at 78% 88%, rgba(34,211,238,0.05), transparent 58%),
    linear-gradient(175deg, #080e1a 0%, #0a1422 35%, #0c1628 70%, #090f1e 100%);
}
```

### 11.2 关键差异

| 属性 | Light | Dark |
|------|-------|------|
| 文字主色 | `#0f172a` | `#e8edf3` |
| 文字次色 | `#475569` | `#94a3b8` |
| 文字禁用 | `#94a3b8` | `#5b6e8a` |
| 卡片背景 | `rgba(255,255,255,0.48)` | `rgba(18,26,44,0.64)` |
| 侧栏背景 | `rgba(255,255,255,0.40)` | `rgba(12,18,32,0.50)` |
| 活动导航 | `rgba(6,182,212,0.13)` | `rgba(6,182,212,0.18)` |
| 活动导航文字 | `--brand-600` | `#22d3ee` |
| 关键阴影 | `rgba(0,0,0,0.06)` | `rgba(0,0,0,0.45)` |

---

## 12. 命名约定

### 12.1 文件

- 视图组件：PascalCase，`Dashboard.vue` / `Settings.vue`
- 子组件：按功能目录组织，`components/chat/` / `components/ui/`
- Store：`stores/auth.js` / `stores/theme.js` / `stores/chat.js`
- 样式文件：`styles/tokens.css` / `base.css` / `dark.css` / `arco.css`
- 服务层：`services/api.js`

### 12.2 CSS 类

- 全局样式类：kebab-case，如 `.card-glass` / `.nav-item` / `.app-header`
- Scoped 样式：描述性 BEM 风格，如 `.stat-strip` / `.iface-tree` / `.chat-page`
- 状态类：`.active` / `.open` / `.expanded` / `.streaming`

---

## 13. 状态处理

### 13.1 加载态

- 骨架屏：`.skeleton` 类，渐变动画 `skeletonPulse`
- Arco 组件：`<a-spin :loading="...">` 包裹内容区
- 按钮：`:loading` prop

### 13.2 空状态

- 图标容器 `.empty-state-icon`（品牌色圆形背景）
- 文字 `.empty-state-text`（灰色居中）
- Arco 组件：`<a-empty>`

### 13.3 错误态

- 警告框：`<a-alert type="error">`，使用 `--alert-error-bg` / `--alert-error-border`
- Chat 页面：错误边界 `onErrorCaptured` + 重试按钮

---

## 14. 可访问性

- `:focus-visible` 全局样式：品牌色 2px outline，offset 2px
- 键盘导航：Tab 键跳转，Enter 触发操作
- 暗色模式：`color-scheme: dark`
- 减少动效：CSS `@media (prefers-reduced-motion: reduce)` 需在后续迭代中添加

---

## 15. 文件索引

| 文件 | 用途 |
|------|------|
| `web/src/styles/tokens.css` | 所有 CSS 自定义属性（颜色、字体、阴影、圆角、间距） |
| `web/src/styles/base.css` | 全局重置、排版、卡片类、过渡动画、工具类 |
| `web/src/styles/dark.css` | 暗色主题变量覆盖 |
| `web/src/styles/arco.css` | Arco Design 组件玻璃化样式覆盖 |
| `web/src/App.vue` | 应用壳（Header、Sidebar、路由出口） |
| `web/src/main.js` | 入口：Pinia + Router + ArcoVue 初始化 |
| `web/src/components/ui/PageHeader.vue` | 页面通用标题栏组件 |
