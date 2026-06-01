# 云兮之家 — 核心系统分析

## 1. 架构总览

```
┌─────────────────────────────────────────────────────┐
│                   Web UI (Vue 3)                     │
│  Dashboard │ Chat │ Domains │ Files │ Settings │ ... │
├─────────────────────────────────────────────────────┤
│              HTTP API (Echo, :9981)                  │
│  Auth │ Config │ DNS │ AI │ Files │ Docker │ Admin   │
├─────────────────────────────────────────────────────┤
│                    Core Services                     │
│  AI Engine │ Scheduler │ Notifier │ QQ Bot │ NAS     │
├─────────────────────────────────────────────────────┤
│                   Data Layer                         │
│              SQLite (encrypted)                      │
└─────────────────────────────────────────────────────┘
```

**构建产物**: 单二进制文件（~17MB），Go 后端编译时通过 `//go:embed` 嵌入 Vue 前端 dist 目录。

---

## 2. 配置系统

### 三层覆盖机制

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1 (最低) | `config.DefaultConfig()` | 代码内默认值 |
| 2 | 加密 SQLite | Web 设置页写入，AES-256 加密存储 |
| 3 (最高) | 环境变量 `DNS_UPDATER_*` | 密钥类敏感字段 |

### 核心配置结构

```go
Config {
    Server     // 端口、限流
    Database   // SQLite 路径
    DNS        // dns.aliyun (AccessKey)
    Detect     // IP 检测间隔、数据源
    Notify     // email、webhook、dingtalk
    Auth       // 用户名、密码、JWT 密钥
    AI         // deepseek (enabled, api_key, base_url, model, reasoning)
    QQBots     // bots[] (app_id, app_secret)
    NAS        // 沙箱根目录
    Log        // 日志级别、目录、保留天数
}
```

---

## 3. AI 引擎

### 3.1 核心架构

```
用户消息 → StreamChat()
  ├── Session Manager (会话持久化 + Token 预算管理)
  ├── System Prompt (模块化提示词，7 个独立组件)
  ├── LLM Provider (DeepSeek v4 Flash/Pro，SSE 流式)
  ├── Tool Registry (49 个注册工具)
  ├── Middleware Chain (工具执行 → 确认 → 超时 → 重试)
  └── Event Bus (SSE → 前端 + 会话缓冲区)
```

### 3.2 工具系统 (49 Tools)

| 类别 | 工具 | 用途 |
|------|------|------|
| 系统 | `get_system_status`, `get_network_info`, `gc_memory` | CPU/内存/网络/GC |
| DNS | `list_domains`, `create_domain`, `delete_domain`, `query_dns_records`, `trigger_dns_update`, `list_cloud_records`, `create_cloud_record`, `update_cloud_record`, `delete_cloud_record` | DNS 管理 |
| Docker | `docker_list_containers`, `docker_get_logs`, `docker_container_action`, `docker_compose_action`, `docker_stats` | 容器管理 |
| 文件 | `file_list`, `file_read`, `file_write`, `file_delete`, `file_copy`, `file_move`, `file_mkdir`, `file_info`, `file_download` | 沙箱文件操作 |
| Agent | `spawn_agent` | 派生并行子 Agent |
| 技能 | `list_skills`, `run_skill` | YAML 技能 |
| 任务 | `todo_write`, `cron_create`, `cron_delete`, `cron_list` | 任务管理 |
| 运维 | `run_command` | Shell 命令执行 |
| MCP | `mcp_*` | MCP 协议工具 |

### 3.3 子 Agent 系统

- `spawn_agent` 工具：接收 `[{goal, tool_filter}]`，并行执行
- 独立 LLM 上下文，受限工具集
- 状态机: `pending → running → done/error`
- 支持链式传播（agent > sub-agent）

### 3.4 技能系统 (Skills)

- YAML 声明式定义 (`skills/*.yaml`)
- DAG 依赖执行（`depends` 字段实现步骤排序）
- `/healthcheck`, `/docker-cleanup` 等可通过 QQ Bot 指令调用
- `/skills-create <描述>` 用 AI 自动生成新技能
- `/reload-skills` 热重载

### 3.5 MCP 工具

- `mcp.json` 配置 MCP 服务器
- JSON-RPC over stdio 协议
- 自动注册 `mcp_` 前缀工具
- `/reload-mcp` 热重载

### 3.6 提示词系统

模块化设计，7 个独立组件组合为完整系统提示词:

| 模块 | 内容 |
|------|------|
| `IdentityRules` | 身份铁律（不暴露模型名、不讨论架构） |
| `CoreRules` | 核心行为（必须调工具、先确认再执行） |
| `FilesystemRules` | 沙箱文件系统限制 |
| `FileSendingRules` | 文件发送标记格式 `[文件: name (path)]` |
| `ToolStrategy` | 工具选择策略 |
| `TimeoutGuide` | 超时估算指南 |

---

## 4. QQ Bot 系统

### 4.1 多机器人架构

- 支持配置多个 Bot (不同 AppID/Token)
- 链式 WebSocket 事件处理器（每个 Bot 包装前一个）
- 每个 Bot 独立 API 客户端和会话管理

### 4.2 指令系统

| 指令 | 功能 |
|------|------|
| `/help` | 帮助 |
| `/status` | 系统状态 |
| `/domains` | 域名列表 |
| `/add /delete /enable /disable` | DNS 记录管理 |
| `/history [n]` | 更新历史 |
| `/trigger` | 触发 DNS 检测 |
| `/gc` | 内存清理 |
| `/clear` | 清除对话 |
| `/compact` | 压缩上下文 |
| `/list-skills` | 列出技能 |
| `/skills-create <描述>` | AI 创建技能 |
| `/reload-skills` | 热重载技能 |
| `/reload-mcp` | 热重载 MCP |

技能自动注册为 `/<技能名>` 指令。

### 4.3 消息处理

```
C2C/群聊消息 → parseCommand()
  ├── 匹配指令 → handler(ctx, args) → 直接回复
  └── 非指令 → StreamChat() → replyMarkdown() → 分片/降级
```

- Markdown 发送（MsgType=2），失败自动降级纯文本
- 文件传输: `[文件: name (sandbox_path)]` 标记 → base64 上传 → msg_type=7 发送
- 消息限流: 2s/条，最多 3 条突发
- 超时控制: AI 对话 45s 超时

---

## 5. API 系统

### 5.1 路由总览

| 模块 | 路由前缀 | 端点数量 |
|------|----------|----------|
| Auth | `/api/auth` | 5 |
| DNS | `/api/domains` | 13 |
| History | `/api/history` | 3 |
| Config | `/api/config` | 3 |
| Status | `/api/status /api/trigger /api/system` | 5 |
| Chat | `/api/chat` | 10 |
| Files | `/api/nas/files` | 14 |
| Shares | `/api/nas/shares` | 3 |
| Docker | `/api/docker` | 5 |
| Admin | `/api/admin` | 6 |
| Terminal | `/api/terminal` (WS) | 1 |
| Cron | `/api/cron` | 2 |
| Sandbox | `/api/sandbox` | 1 |
| Public | `/dl` (签名下载), `/s/:token` (分享), `/health` | 3 |

总计 **~74 个端点**。详细文档见 [api.md](api.md)。

### 5.2 中间件链

```
Request → Logger → Recover → CORS → RateLimiter → [JWT Auth] → Handler
```

- JWT 24h 过期，支持刷新
- 限流: 20 req/s (可配置)
- FileAccess 中间件: 文件操作权限检查

---

## 6. 前端系统

### 6.1 页面结构

| 页面 | 路由 | 功能 |
|------|------|------|
| 登录 | `/login` | JWT 登录/初始化 |
| 仪表盘 | `/` | 系统状态实时监控 |
| 聊天 | `/chat` `/chat/:id` | AI 对话 (SSE 流式) |
| 域名 | `/domains` | DNS 记录管理 |
| 文件 | `/files` | NAS 文件管理 |
| 设置 | `/settings` | 全配置管理 |
| 系统 | `/system` | 进程/服务/终端 |

### 6.2 Chat 组件架构

```
Chat.vue (orchestrator)
├── Sidebar.vue          # 会话列表 + 子 Agent 状态
├── HomeState.vue        # 空状态占位
├── ChatMessage.vue      # 消息气泡
│   ├── ContentBlock.vue # Markdown 渲染 (marked + DOMPurify)
│   ├── ThinkingBlock.vue # 思考过程
│   ├── ToolCallBlock.vue # 工具调用卡片
│   └── AgentBubble.vue  # 子 Agent 气泡
├── ChatInputBar.vue     # 输入栏 (文件/模型/推理/指令面板)
│   └── CronPanel.vue    # 定时任务面板
├── ConfirmDialog.vue    # 危险操作确认
├── TodoPanel.vue        # 任务列表
└── AgentPanel.vue       # Agent 面板
```

### 6.3 状态管理 (Pinia)

- `chat.js`: 会话、消息、SSE 流处理、子 Agent 跟踪
- `settings.js`: 全局配置缓存、按 section 保存

---

## 7. 数据模型

### 核心表

| 表 | 用途 |
|----|------|
| `config` | 加密配置 (AES-256) |
| `domains` | DNS 记录 |
| `histories` | DNS 更新历史 |
| `chat_sessions` | AI 对话会话 |
| `users` | 用户 |
| `file_permissions` | 文件权限 |
| `shares` | 文件分享 |
| `cron_tasks` | 定时任务 |
| `ai_event_log` | AI 事件日志 |
| `ai_metrics_hourly` | AI 指标 |

---

## 8. 部署与运维

### 构建脚本

| 脚本 | 功能 |
|------|------|
| `scripts/deploy.sh` | 前端构建 + Go 交叉编译 + SSH 上传 + 自动重启 |
| `scripts/deploy.sh --dry-run` | 仅构建 |
| `scripts/deploy.sh --rollback` | 回滚 |

### 服务器配置

- **服务管理**: systemd (`yunxi-home.service`)
- **反向代理**: nginx (端口 80 → 9981)
- **文件存储**: `/opt/yunxi-home/data/`
- **日志**: `/opt/yunxi-home/log/`
- **沙箱**: `/opt/yunxi-home/data/yunxiFiles/`

---

## 9. 安全

- 配置加密: AES-256-GCM，32 字节密钥
- 密钥管理: 环境变量 `DNS_UPDATER_ENCRYPTION_KEY` 或自动生成
- API 密钥掩码: 前端展示 `••••••••`，保存时后端自动保留真实值
- 文件沙箱: 所有 AI 文件操作限制在沙箱目录内
- JWT: 24h 过期 + 刷新机制
- 操作确认: 危险操作 (删除、启停) 需要用户确认

---

## 10. 扩展能力

- **技能**: YAML 模板扩展 AI 能力，支持热重载
- **MCP**: JSON-RPC 连接外部工具服务器
- **QQ Bot**: 多机器人、动态添加/删除
- **通知**: 插件式通知渠道
- **前端**: 组件化 Vue 3，易于扩展新页面
