# Yunxi Home v4.0 — 项目模块结构

> 生成时间: 2026-05-30  
> Go 91 files | Vue 19 files | 总 ~12,000 行代码

---

## 目录树

```
yunxi-home/
│
├── cmd/yunxi-home/main.go           # 入口: 300行, 全部初始化+组装 [P0: 拆分]
│
├── configs/
│   ├── config.yaml                   # 运行配置
│   └── config.example.yaml           # 配置模板
│
├── deploy/docker/                    # Docker Compose 全家桶部署
│   ├── docker-compose.yml            # 20+ 服务栈
│   ├── configs/authelia/             # SSO 配置
│   └── *.md                          # 部署文档
│
├── internal/
│   │
│   ├── ai/                           # [待重组] AI 模块 — 4 files
│   │   ├── provider.go               # Message/ToolCall/ToolDef 类型 + AIProvider 接口
│   │   ├── deepseek.go               # DeepSeekProvider 实现 (API调用/SSE解析/消息转换)
│   │   ├── chat.go                   # Service: 会话管理/system prompt/工具执行/截断/循环检测
│   │   └── registry.go               # Registry: 工具注册表
│   │   ★ 目标: ai/ 放基座, ai/deepseek/ 放具体实现
│   │
│   ├── alidns/                       # 阿里云 DNS API — 5 files
│   │   ├── api.go                     # DNS API 调用
│   │   ├── signer.go/signer_test.go   # HMAC-SHA1 签名
│   │   ├── types.go                   # 请求/响应类型
│   │   └── errors.go                  # 错误处理
│   │
│   ├── config/                       # 配置系统 — 3 files
│   │   ├── config.go                  # 全部配置结构体
│   │   ├── loader.go                  # 加载: YAML → 环境变量 → DB
│   │   └── validator.go               # 默认值 + 校��
│   │
│   ├── crypto/                       # 加密 — 2 files
│   │   ├── aes.go                     # AES 加密/解密
│   │   └── repo.go                    # EncryptedConfigRepo 装饰器
│   │
│   ├── database/                     # 数据层 — 16 files [最分散]
│   │   ├── interfaces.go             # Domain/History/User/Config/FilePerm/ChatSession Repo 接口
│   │   ├── sqlite.go                 # SQLite 迁移 (全部 CREATE TABLE)
│   │   ├── mysql.go                  # MySQL 迁移 (全部 CREATE TABLE)
│   │   ├── domain_repo.go + _test.go # SQLite 域名仓库
│   │   ├── history_repo.go + _test.go# SQLite 历史仓库
│   │   ├── user_repo.go              # SQLite 用户仓库
│   │   ├── file_perm_repo.go         # SQLite 文件权限仓库
│   │   ├── share_repo.go             # SQLite 分享仓库
│   │   ├── chat_repo.go              # SQLite + MySQL 聊天会话仓库
│   │   ├── config_repo.go            # SQLite + MySQL 配置仓库
│   │   ├── dual_domain.go            # DualDomainRepo: SQLite↔MySQL 双写
│   │   ├── dual_history.go           # DualHistoryRepo: SQLite↔MySQL 双写
│   │   └── syncer.go                 # Syncer: 后台同步 SQLite→MySQL
│   │
│   ├── docker/                       # Docker 管理 — 1 file
│   │   └── manager.go                # Docker API 客户端
│   │
│   ├── ipdetect/                     # IP 检测 — 3 files
│   │   ├── detector.go               # 多源 IPv4/IPv6 检测
│   │   ├── validator.go              # IP 格式校验
│   │   └── cache.go                  # 检测结果缓存
│   │
│   ├── logger/                       # 日志 — 1 file
│   │   └── logger.go                 # slog 初始化 + 日志轮转
│   │
│   ├── middleware/                    # [P0: 合并到 web/middleware/]
│   │   └── file_access.go            # 文件权限中间件 (单独一个文件在一个目录)
│   │
│   ├── models/                       # 共享类型 — 4 files
│   │   ├── domain.go                 # DomainRecord
│   │   ├── history.go                # HistoryRecord
│   │   ├── user.go                   # User, FilePermission, FilePermMask, ChatSession
│   │   └── config.go                 # NASConfig
│   │
│   ├── nas/                          # 文件系统 — 7 files
│   │   ├── filesystem.go             # FileService: 增删改查/沙箱/搜索/复制/分片上传
│   │   ├── filesystem_test.go        # 27 个测试
│   │   ├── share.go                  # ShareService: 分享链接
│   │   ├── diskinfo_windows.go       # Windows 磁盘信息
│   │   ├── diskinfo_unix.go          # Unix 磁盘信息
│   │   └── debug_test.go             # 沙箱调试测试
│   │
│   ├── notifier/                     # 通知 — 5 files
│   │   ├── interface.go              # Notifier 接口
│   │   ├── manager.go                # Manager: 多渠道/节流/并发发送
│   │   ├── mail.go                   # EmailNotifier: SMTP 邮件
│   │   ├── webhook.go                # WebhookNotifier: HTTP webhook
│   │   └── throttler.go + _test.go   # 节流器: 10分钟窗口
│   │
│   ├── qqbot/                        # QQ Bot — 3 files
│   │   ├── bot.go                    # QQ Bot WebSocket 客户端
│   │   ├── commands.go               # 指令注册
│   │   └── notifier.go               # 通知适配器
│   │
│   ├── scheduler/                    # 调度器 — 1 file
│   │   └── scheduler.go              # Cron DNS 更新调度
│   │
│   ├── sysctl/                       # 系统控制 — 3 files
│   │   ├── sysctl.go                 # 接口定义
│   │   ├── sysctl_unix.go            # Unix 实现
│   │   └── sysctl_windows.go         # Windows 实现
│   │
│   ├── terminal/                     # Web SSH — 4 files
│   │   ├── terminal.go               # WebSocket ↔ PTY
│   │   ├── pty_unix.go               # Unix PTY
│   │   ├── pty_windows.go            # Windows PTY
│   │   └── terminal_test.go
│   │
│   ├── toolreg/                      # AI 工具注册 — 4 files [跨模块硬依赖]
│   │   ├── register.go               # DNS/系统/网络工具 (依赖 alidns/database/scheduler/config)
│   │   ├── files.go                  # 文件工具 (依赖 nas)
│   │   ├── extended.go               # Docker/资源工具 (依赖 docker/config)
│   │   └── ops.go                    # SSH/备份/快照/本地命令 (依赖 config)
│   │
│   └── web/
│       ├── server.go                 # Echo 路由 + 静态文件 + CORS + 缓存
│       ├── handlers/                 # HTTP 处理器 — 12 files
│       │   ├── auth.go               # 登录/刷新
│       │   ├── chat.go               # AI 聊天 SSE
│       │   ├── common.go             # APIResponse/errorResp/successResp
│       │   ├── config.go             # 系统配置 CRUD
│       │   ├── docker.go             # Docker 管理
│       │   ├── domain.go             # 域名管理
│       │   ├── files.go              # 文件管理 (最复杂, ~500行)
│       │   ├── history.go            # 更新历史
│       │   ├── admin.go              # 管理员: 用户/文件权限
│       │   ├── shares.go             # 文件分享
│       │   ├── status.go             # 系统状态
│       │   └── sysctl.go             # 系统控制
│       ├── middleware/
│       │   └── auth.go               # JWT 认证中间件
│       └── static/                   # 前端构建产物 (Vite, go:embed)
│
└── web/src/                           # Vue 3 前端 — 19 source files
    ├── main.js                        # 路由 + Pinia
    ├── App.vue                        # 全局上传进度条
    ├── services/api.js                # Axios 实例 (全局 timeout=0)
    ├── stores/
    │   ├── auth.js                    # 认证状态
    │   ├── chat.js                    # 聊天: SSE解析/消息管理/会话切换/队列 [P2: 拆分]
    │   ├── theme.js                   # 主题切换
    │   └── upload.js                  # 上传: 分片/并发/进度
    ├── views/
    │   ├── Chat.vue                   # AI 聊天页
    │   ├── Files.vue                  # 文件管理 (~1400行, 最复杂)
    │   ├── Dashboard.vue              # 仪表盘
    │   ├── Domains.vue                # 域名管理
    │   ├── CloudDns.vue               # 云端 DNS
    │   ├── History.vue                # 更新历史
    │   ├── System.vue                 # 系统监控
    │   ├── Terminal.vue               # Web 终端
    │   ├── Settings.vue               # 系统设置
    │   └── Login.vue                  # 登录页
    ├── components/
    │   ├── chat/
    │   │   ├── ChatMessage.vue        # 消息气泡
    │   │   ├── ContentBlock.vue       # Markdown 渲染
    │   │   ├── ThinkingBlock.vue      # 思考过程折叠
    │   │   └── ToolCallBlock.vue      # 工具调用展开
    │   └── ui/
    │       ├── PageHeader.vue         # 页面标题
    │       ├── ContextMenu.vue        # 右键菜单
    │       ├── ConfirmDialog.vue      # 确认对话框
    │       └── CodeCell.vue           # 代码块渲染
    └── composables/
        ├── useFormat.js               # 格式化工具
        └── useToast.js                # Toast 通知
```

---
