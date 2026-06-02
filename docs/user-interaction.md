# 用户交互与体验

> 版本：1.0  
> 适用范围：SSE 流式响应、会话管理、中断恢复、通知机制、前端架构  
> 更新日期：2026-06-02

---

## 1. 概述

AI Agent 应用的用户体验与传统 Web 应用有本质差异：用户需要**实时看到 AI 的思考过程**、**理解正在执行的操作**、**感知系统进度**，并在需要时**介入控制**。本文档定义了系统的用户交互模型。

### 1.1 核心体验目标

- **透明性**：用户能看到 AI 每一步在做什么（思考、调用工具、委派子 Agent）
- **可控性**：用户可以暂停、恢复、取消 AI 的执行
- **即时性**：流式输出，减少等待焦虑
- **可恢复性**：刷新页面或断网后能恢复之前的会话状态

---

## 2. 通信协议

### 2.1 SSE（Server-Sent Events）— 主要通道

```
客户端                         服务端
  │                              │
  │── POST /api/chat ──────────▶│  发起对话
  │                              │
  │◀── SSE: text/event-stream ──│  流式响应
  │    data: {"type":"thinking"} │
  │    data: {"type":"content"}  │
  │    data: {"type":"tool_call"}│
  │    data: {"type":"done"}     │
  │                              │
  │── GET /api/chat/stream/:id ─▶│  断线重连
  │◀── SSE: 历史事件回放 ────────│
```

**SSE 格式**：
```
data: {"type":"thinking","content":"正在分析需求...","session_id":"sess_abc"}\n\n
data: {"type":"tool_call","name":"file_read","args":{"path":"/etc/hosts"}}\n\n
data: {"type":"tool_result","success":true}\n\n
data: {"type":"content","delta":"根据分析结果..."}\n\n
data: {"type":"done","session_id":"sess_abc","rounds":5,"tokens":1234}\n\n
```

### 2.2 事件类型定义

| 事件类型 | 方向 | 说明 | 携带数据 |
|----------|------|------|----------|
| `thinking` | S→C | AI 正在进行推理 | `content`, `round` |
| `content` | S→C | AI 输出增量文本 | `delta` |
| `tool_call` | S→C | AI 发起工具调用 | `name`, `args`, `call_id` |
| `tool_result` | S→C | 工具执行结果 | `call_id`, `success`, `output` |
| `tool_start` | S→C | 工具开始执行 | `name`, `started_at` |
| `tool_progress` | S→C | 长时间工具进度 | `elapsed_s`, `status` |
| `agent_progress` | S→C | 子 Agent 进度更新 | `agent_id`, `goal`, `status` |
| `agent_result` | S→C | 子 Agent 完成 | `agent_id`, `summary` |
| `confirm_required` | S→C | 需要用户确认 | `message`, `confirm_id` |
| `interactive_request` | S→C | AI 请求用户输入 | `prompt`, `timeout_s` |
| `interrupted` | S→C | 会话被中断 | `reason`, `mode` |
| `session_status` | S→C | 会话状态快照 | 完整 SessionState |
| `cross_session` | S→C | 跨会话通知 | `source_session`, `message` |
| `topology_update` | S→C | 拓扑状态变化 | `coordinate`, `trajectory` |
| `done` | S→C | 本轮对话完成 | `rounds`, `tokens` |
| `error` | S→C | 错误发生 | `code`, `message` |

### 2.3 前端 SSE 消费

```typescript
// web/src/stores/chat.ts
class ChatStore {
    private eventSource: EventSource | null = null;
    
    async sendMessage(content: string) {
        const response = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
            body: JSON.stringify({ session_id: this.sessionId, message: content })
        });
        
        const reader = response.body!.getReader();
        const decoder = new TextDecoder();
        
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            
            const text = decoder.decode(value, { stream: true });
            for (const line of text.split('\n')) {
                if (line.startsWith('data: ')) {
                    const event = JSON.parse(line.slice(6));
                    this.handleEvent(event);
                }
            }
        }
    }
    
    // 断线重连
    async reconnect(sessionId: string) {
        const response = await fetch(`/api/chat/stream/${sessionId}`);
        // 同样的 SSE 消费逻辑
    }
}
```

---

## 3. 流式响应与思考可见性

### 3.1 三级流式粒度

```
┌──────────────────────────────────────────────┐
│ Level 1: Token 级                            │
│ AI 逐 token 输出 → content delta 事件          │
│ 延迟: ~50ms                                  │
├──────────────────────────────────────────────┤
│ Level 2: 步骤级                              │
│ 工具调用/结果 → tool_call + tool_result 事件   │
│ 延迟: ~1-5s（取决于工具）                     │
├──────────────────────────────────────────────┤
│ Level 3: 任务级                              │
│ 子 Agent 进度 → agent_progress 事件          │
│ 延迟: ~10-60s（取决于子任务复杂度）           │
└──────────────────────────────────────────────┘
```

### 3.2 思考过程可视化

用户界面展示 AI 思考过程的三个区域：

```
┌──────────────────────────────────────────────┐
│  💭 思考区域（可折叠）                         │
│  "我需要先读取配置文件，然后分析其中的关键参数"  │
├──────────────────────────────────────────────┤
│  🔧 工具调用（内联展示）                       │
│  ⚙ file_read /etc/config.yaml  ✓ (0.3s)     │
│  ⚙ run_command systemctl status  ⏳ (进行中)  │
├──────────────────────────────────────────────┤
│  📝 最终回答                                 │
│  "根据配置文件分析，您的系统状态如下：..."       │
└──────────────────────────────────────────────┘
```

### 3.3 长时间工具的心跳

```go
// 当工具执行超过 2s 时，每 2s 发送心跳
func (s *Service) executeWithProgress(ctx context.Context, call ToolCall) {
    progressTicker := time.NewTicker(2 * time.Second)
    defer progressTicker.Stop()
    
    done := make(chan *ToolResult, 1)
    go func() {
        done <- s.exec.Execute(ctx, call)
    }()
    
    for {
        select {
        case result := <-done:
            return result
        case <-progressTicker.C:
            s.emitSSE("tool_progress", map[string]any{
                "call_id":   call.ID,
                "elapsed_s": time.Since(start).Seconds(),
                "status":    "执行中...",
            })
        }
    }
}
```

---

## 4. 中断与恢复

### 4.1 三种取消模式

```go
// interrupt.go
type CancelMode string
const (
    CancelSoft     CancelMode = "soft"     // 优雅：完成当前工具后停止
    CancelHard     CancelMode = "hard"     // 强制：立即中断（cancel context）
    CancelModeSnapshot CancelMode = "snapshot" // 快照：暂停并保存完整上下文
)
```

**模式对比**：

| 模式 | 行为 | 恢复方式 | 适用场景 |
|------|------|----------|----------|
| Soft | 等待当前工具完成，不发起新推理 | （不可恢复） | 用户想"等一下再继续" |
| Hard | 立即 cancel context，不等待 | （不可恢复） | 用户想中止并开始新话题 |
| Snapshot | 保存完整上下文后暂停 | 调用 resume API | 用户想暂停后稍后继续 |

### 4.2 中断流程

```
用户点击暂停
    │
    ▼
POST /api/chat/interrupt { session_id, mode: "snapshot" }
    │
    ▼
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
服务端：
  1. 注入 interrupt 信号到会话的 injectCh
  2. ReAct 循环在下一轮开始前检测到信号
  3. 序列化完整消息历史到 DB
  4. 将 Session 标记为 interrupted
  5. 发送 interrupted SSE 事件
  6. 保存 Topology 状态
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    │
    ▼
用户恢复
    │
    ▼
POST /api/chat/resume { session_id }
    │
    ▼
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
服务端：
  1. 从 DB 加载 interrupted 会话
  2. 恢复消息历史、拓扑状态
  3. 重新进入 ReAct 循环
  4. 发送 session_status SSE 事件
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### 4.3 中断横幅 UI

当会话处于中断状态时，在聊天界面顶部显示横幅：

```
┌──────────────────────────────────────────────────┐
│  ⏸ 会话已于 15:04 暂停                            │
│  已完成 12/20 轮，可随时恢复                        │
│  [▶ 恢复]  [✕ 放弃]                               │
└──────────────────────────────────────────────────┘
```

---

## 5. 确认与交互请求

### 5.1 确认流程

当 AI 需要执行高风险操作时，请求用户确认：

```
AI → SSE → 前端:
{
  "type": "confirm_required",
  "confirm_id": "confirm_abc123",
  "message": "即将删除文件 /etc/config/backup.yaml。确认执行？",
  "tool": "file_delete",
  "risk_level": "dangerous"
}

前端展示确认对话框:
┌─────────────────────────────┐
│  ⚠ 确认操作                  │
│                              │
│  即将删除文件:                │
│  /etc/config/backup.yaml     │
│                              │
│  风险等级: 危险               │
│                              │
│  [确认执行]  [取消]           │
└─────────────────────────────┘

用户点击 [确认执行] →
POST /api/chat/confirm { confirm_id, approved: true }

服务端:
  waitForConfirm 收到 approved → 继续执行工具
```

### 5.2 确认超时

```go
// 确认等待超时
const ConfirmTimeout = 60 * time.Second

func (s *Service) waitForConfirm(confirmID string) (bool, error) {
    select {
    case result := <-s.confirmChannels[confirmID]:
        return result.Approved, nil
    case <-time.After(ConfirmTimeout):
        return false, ErrConfirmTimeout
    }
}
```

### 5.3 交互式请求

AI 在执行中可以请求用户提供额外信息：

```
AI → SSE:
{
  "type": "interactive_request",
  "prompt": "请提供需要修改的域名",
  "timeout_s": 120
}

前端展示输入框 → 用户输入 → 
POST /api/chat/interact { response: "example.com" } →
AI 继续执行
```

---

## 6. 会话管理

### 6.1 会话生命周期

```
创建会话
  │
  ▼
active ──────────────────────────────────────────
  │                                               │
  ├─ 用户发送消息                                   │
  ├─ AI 流式响应                                   │
  ├─ 消息追加到历史                                 │
  │                                               │
  ├─ 用户暂停 → interrupted → resume → active       │
  ├─ 用户取消 → canceled                           │
  ├─ 任务完成 → closed                             │
  └─ 超时无活动 → expired                           │
```

### 6.2 消息历史管理

```go
// 消息历史存储在 DB 中
type ChatSession struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`         // 自动生成（首条消息摘要）
    Messages     []Message `json:"messages"`
    Status       string    `json:"status"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// 消息历史加载策略
// 初次加载：最近 50 条消息
// 滚动加载：按需加载更早的消息
// 上下文裁剪：BudgetManager 自动压缩超出 token 预算的历史
```

### 6.3 会话列表 UI

```
┌──────────────────────────────┐
│  📋 会话列表                  │
│  ┌──────────────────────────┐│
│  │ 🟢 系统健康检查           ││  ← 活跃
│  │    12 轮 · 5 分钟前       ││
│  ├──────────────────────────┤│
│  │ ⏸ 配置 Nginx 反向代理     ││  ← 暂停
│  │    8 轮 · 1 小时前        ││
│  ├──────────────────────────┤│
│  │ ✓ 备份数据库              ││  ← 完成
│  │    25 轮 · 昨天           ││
│  ├──────────────────────────┤│
│  │ ✕ 分析日志错误            ││  ← 取消
│  │    3 轮 · 昨天            ││
│  └──────────────────────────┘│
│                              │
│  [+ 新建会话]                │
└──────────────────────────────┘
```

---

## 7. 通知机制

### 7.1 通知类型

| 类型 | 触发条件 | 传递方式 |
|------|----------|----------|
| 即时通知 | 工具完成、错误、确认请求 | SSE 事件（当前会话内） |
| 跨会话通知 | 子 Agent 完成（异步模式） | SSE + WebSocket |
| 离线通知 | 任务完成（用户已离开） | 暂未实现（推荐：Web Push API） |

### 7.2 跨会话通知

```
场景：用户在会话 A 中启动了异步子 Agent，然后切换到会话 B

实现：
  1. 子 Agent 完成 → 发送 cross_session 事件
  2. 前端监听 cross_session 事件
  3. 在会话 B 界面显示通知徽章

事件格式：
{
  "type": "cross_session",
  "source_session": "sess_abc",
  "message": "子任务「备份数据库」已完成",
  "agent_id": "agent_5"
}
```

### 7.3 通知渠道

```go
// notifier/manager.go — 当前架构
type Notifier struct {
    channels map[string]chan Notification  // 按通知类型分发
}

// 推荐扩展：多渠道通知
type NotificationService struct {
    sse      *SSENotifier      // 浏览器内 SSE
    webpush  *WebPushNotifier  // Web Push API（离线通知）
    email    *EmailNotifier    // 邮件（长时间任务）
    webhook  *WebhookNotifier  // 企业 IM（Slack/钉钉）
}

// 通知级别路由
// 即时 → SSE
// 5分钟后用户不在线 → Web Push
// 30分钟后 → 邮件
```

---

## 8. 前端架构

### 8.1 技术栈

| 层 | 技术 |
|------|------|
| 框架 | Vue 3 (Composition API) |
| 状态管理 | Pinia |
| 构建 | Vite |
| Markdown | marked + DOMPurify |
| HTTP | Fetch API + SSE |
| 终端 | xterm.js (WebSocket) |

### 8.2 组件架构

```
Chat.vue（主视图）
├── Sidebar.vue                    ← 会话列表
│   ├── 新建会话按钮
│   ├── 会话列表（按状态分组）
│   └── 搜索/过滤
│
├── ChatMessage.vue                ← 消息气泡
│   ├── 用户消息（右侧）
│   ├── AI 回答（左侧，Markdown）
│   ├── 思考过程（可折叠）
│   └── 工具调用（内联卡片）
│
├── ChatInputBar.vue               ← 输入区域
│   ├── 文本输入
│   ├── 发送/停止按钮
│   ├── /compact 命令支持
│   └── 文件附加
│
├── InterruptBanner.vue            ← 中断横幅
│   ├── 恢复按钮
│   └── 放弃按钮
│
├── ConfirmDialog.vue              ← 确认对话框
│   └── 确认/取消按钮（含超时倒计时）
│
├── InteractiveInput.vue           ← 交互式输入
│   └── AI 提问 → 用户回答
│
└── TopologyPanel.vue              ← 拓扑可视化
    ├── 3D 坐标展示
    ├── 轨迹图
    └── 信任状态指示
```

### 8.3 状态管理

```typescript
// web/src/stores/chat.ts
interface ChatState {
    sessions: Map<string, Session>;
    activeSessionId: string | null;
    messages: Message[];
    isStreaming: boolean;
    topologyState: TopologyState | null;
    pendingConfirm: ConfirmRequest | null;
    interactivePrompt: InteractiveRequest | null;
}

// 关键 actions
const useChatStore = defineStore('chat', {
    actions: {
        async sendMessage(content: string),
        async stopGeneration(),
        async pauseSession(),
        async resumeSession(sessionId: string),
        async loadHistory(sessionId: string),
        async reconnect(sessionId: string),
        async confirmAction(confirmId: string, approved: boolean),
        async respondToInteractive(response: string),
        handleSSEEvent(event: ChatEvent),
    }
});
```

---

## 9. 离线与恢复

### 9.1 前端断线重连策略

```
SSE 连接断开
  │
  ├── 立即重试 × 1
  ├── 2s 后重试
  ├── 5s 后重试
  ├── 15s 后重试
  └── 30s 后放弃 → 提示用户手动重连

重连时：
  POST /api/chat/stream/:id
  → 服务端回放事件缓冲区中的事件
  → 发送 session_status 事件（完整状态快照）
  → 前端重建 UI 状态
```

### 9.2 离线队列

```
推荐实现（当前未支持）：

用户离线时：
  - 新消息加入本地队列（IndexedDB）
  - 显示 "等待网络恢复" 指示器

网络恢复时：
  - 发送队列中的消息
  - 重新建立 SSE 连接
  - 恢复会话状态
```

---

## 10. PWA 支持

### 10.1 缓存策略

```typescript
// web/vite.config.ts
// Service Worker 缓存策略
{
    // HTML: no-cache（始终获取最新版本）
    '/': { strategy: 'NetworkFirst' },
    
    // 静态资源: immutable（带 hash 的文件名）
    'assets/*.js': { strategy: 'CacheFirst', maxAge: 365 * 24 * 60 * 60 },
    'assets/*.css': { strategy: 'CacheFirst', maxAge: 365 * 24 * 60 * 60 },
    
    // API: network only（不缓存）
    '/api/*': { strategy: 'NetworkOnly' },
}
```

### 10.2 安装体验

```
首次访问：
  1. 加载 HTML（~5KB）
  2. 加载 JS 入口（~200KB，gzipped）
  3. 注册 Service Worker
  4. 预缓存静态资源

后续访问（PWA）：
  1. Service Worker 拦截请求
  2. 从 Cache Storage 返回静态资源
  3. 后台检查更新
```

---

## 11. 测试策略

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| UT-UI-1 | SSE 事件解析 | 接收完整 data: 行 | 解析为正确的 ChatEvent |
| UT-UI-2 | SSE 断行处理 | 接收不完整的 data: 行 | 缓冲并等待后续数据 |
| UT-UI-3 | 确认超时 | 60s 内无响应 | 返回 ConfirmTimeout |
| UT-UI-4 | 会话恢复 | 调用 resume API | 消息历史完整恢复 |
| IT-UI-1 | 完整对话流程 | 用户发消息 → AI 回复 → 工具调用 | 所有 SSE 事件正确渲染 |
| IT-UI-2 | 中断与恢复 | 暂停 → 恢复 | 状态一致，上下文不丢 |
| IT-UI-3 | 断线重连 | 断开 SSE → 重连 | 回放缓冲事件，恢复 UI |
| IT-UI-4 | 多会话并发 | 切换会话 | 各会话消息隔离 |
| E2E-1 | 首次使用流程 | 打开页面 → 登录 → 发消息 | 完整的首次体验 |

---

**文档结束**
