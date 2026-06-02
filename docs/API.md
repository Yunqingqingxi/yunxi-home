# DNS Updater Go — API 文档

> 版本：4.0.0
> 基础路径：`/`
> 更新日期：2026-06-02

---

## 目录

1. [认证](#认证)
2. [通用规范](#通用规范)
3. [公开接口](#公开接口)
4. [认证接口](#认证接口)
5. [域名管理](#域名管理)
6. [历史记录](#历史记录)
7. [配置管理](#配置管理)
8. [系统状态](#系统状态)
9. [AI 聊天](#ai-聊天)
10. [技能与 MCP 市场](#技能与-mcp-市场)
11. [日志管理](#日志管理)
12. [定时任务](#定时任务)
13. [文件管理 (NAS)](#文件管理-nas)
14. [文件分享](#文件分享)
15. [Docker 管理](#docker-管理)
16. [系统控制](#系统控制)
17. [终端 WebSocket](#终端-websocket)
18. [管理员](#管理员)
19. [错误码](#错误码)

---

## 认证

除登录、健康检查和公开分享外，所有 `/api/*` 接口需要 JWT 认证。

```
Authorization: Bearer <jwt_token>
```

Token 有效期 24 小时，可通过 `/api/auth/refresh` 刷新。

---

## 通用规范

### 响应格式

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

- `code: 200` 表示成功，其他值表示错误
- `message` 为人类可读的描述
- `data` 为响应数据，可能为 `null`、对象或数组

### SSE 流式响应

聊天和部分实时接口使用 Server-Sent Events (SSE)：

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: {"type":"...","content":"..."}

```

---

## 公开接口

无需认证即可访问。

### GET /health

存活探针。

**响应**：

```json
{"status": "alive"}
```

### GET /ready

就绪探针。

**响应**：`{"status": "ready"}` 或 HTTP 503

### GET /dl

公开文件下载（HMAC 签名，用于 QQ Bot 等场景）。

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `p` | string | 文件路径 |
| `t` | string | HMAC 签名（前 32 位） |
| `e` | string | 过期时间戳 |

### GET /s/:token

公开分享访问。详见 [文件分享](#文件分享)。

---

## 认证接口

### POST /api/auth/login

用户登录（公开）。

**请求**：

```json
{
  "username": "admin",
  "password": "your-password"
}
```

**响应**：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "eyJhbGciOi...",
    "username": "admin",
    "role": "admin"
  }
}
```

### GET /api/auth/status

检查系统是否需要初始化设置（公开）。

**响应**：

```json
{
  "code": 200,
  "data": { "needs_setup": true }
}
```

### POST /api/auth/setup

首次初始化管理员密码（公开）。

**请求**：

```json
{
  "password": "new-admin-password"
}
```

**响应**：

```json
{
  "code": 200,
  "data": { "message": "管理员密码已设置" }
}
```

### POST /api/auth/refresh

刷新 JWT Token（需要认证）。

**响应**：同登录接口。

### POST /api/auth/change-password

修改当前用户密码（需要认证）。

**请求**：

```json
{
  "current": "old-password",
  "new": "new-password"
}
```

---

## 域名管理

所有接口需要认证。

### GET /api/domains

获取所有本地域名记录。

**响应**：

```json
{
  "code": 200,
  "data": [
    {
      "id": 1,
      "domain": "example.com",
      "rr": "@",
      "type": "AAAA",
      "value": "2409:8a38:...",
      "ttl": 600,
      "enabled": true,
      "cron_expr": "0 */5 * * * *",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:05:00Z"
    }
  ]
}
```

### GET /api/domains/:id

获取单条域名记录。

### POST /api/domains

创建域名记录并自动注册调度任务。

**请求**：

```json
{
  "domain": "example.com",
  "rr": "@",
  "type": "AAAA",
  "ttl": 600,
  "cron_expr": "0 */5 * * * *",
  "enabled": true
}
```

- `type` 仅支持 `A` 或 `AAAA`
- `ttl` 默认为 600
- `cron_expr` 默认为 `0 */5 * * * *`

### PUT /api/domains/:id

更新域名记录（部分字段可选）。

**请求**：

```json
{
  "domain": "example.com",
  "rr": "www",
  "type": "A",
  "ttl": 300,
  "cron_expr": "0 */10 * * * *",
  "enabled": true
}
```

### DELETE /api/domains/:id

删除域名记录。

### 云 DNS（阿里云）

#### GET /api/domains/cloud

获取阿里云账号下的域名列表。

**参数**：`keyword`（可选）、`page`、`size`

#### GET /api/domains/cloud/records

获取阿里云上某域名的解析记录。

**参数**：`domain`（必填）、`page`、`size`

#### POST /api/domains/cloud/records

在阿里云上添加解析记录。

**请求**：同 `POST /api/domains`

#### PUT /api/domains/cloud/records/:recordId

更新阿里云解析记录。

#### DELETE /api/domains/cloud/records/:recordId

删除阿里云解析记录。

---

## 历史记录

所有接口需要认证。

### GET /api/history

分页查询 DNS 更新历史。

**参数**：`page`、`size`（默认 20，最大 100）、`domain`（可选过滤）

**响应**：

```json
{
  "code": 200,
  "data": {
    "records": [
      {
        "id": 1,
        "domain": "example.com",
        "old_ip": "2001::1",
        "new_ip": "2001::2",
        "type": "AAAA",
        "status": "success",
        "error_msg": "",
        "created_at": "2026-01-01T00:05:00Z"
      }
    ],
    "total": 100,
    "page": 1,
    "size": 20
  }
}
```

### GET /api/history/stats

获取历史统计（供图表使用）。

**参数**：`days`（默认 7，最大 365）

### DELETE /api/history/clean

清理旧历史记录。

**参数**：`days`（默认 30，保留最近 N 天的记录）

**响应**：

```json
{
  "code": 200,
  "data": { "deleted": 42 }
}
```

---

## 配置管理

所有接口需要认证。敏感字段（密码、密钥、secret）在 GET 响应中会被掩码处理。

### GET /api/config

获取完整配置（密钥已掩码）。

### GET /api/config/:section

获取指定配置分类。

**有效 section**：`server`、`database`、`detect`、`notify`、`auth`、`ai`、`qqbot`、`log`、`dns`、`nas`、`terminal`、`sysctl`、`dynamic_records`

### PUT /api/config/:section

更新指定配置分类。

- 空值或掩码值不会被覆盖（保留原有密钥）。
- 更新 `ai` section 时会先测试连接，通过后才保存。
- 更新 `log` section 时日志级别即时生效。
- 更新 `qqbot` section 时触发 Bot 重启。

### PUT /api/config

批量更新多个配置分类。

**请求**：

```json
{
  "server": { "port": 8080 },
  "log": { "level": "debug" }
}
```

### POST /api/config/ai/test

测试 AI 提供商连接（不保存配置）。

**请求**：完整的 AI 配置 JSON。

**响应**：

```json
{
  "code": 200,
  "data": {
    "tests": {
      "deepseek": { "enabled": true },
      "qwen": { "enabled": false, "error": "HTTP 401" }
    }
  }
}
```

### 提示词管理

#### GET /api/config/prompts

获取所有 Prompt 模板及其来源（默认值 / 数据库覆盖）。

#### PUT /api/config/prompts/:section

更新指定 Prompt 模板并热重载。

**请求**：

```json
{
  "data": "你是一个有帮助的助手..."
}
```

#### POST /api/config/prompts/:section/reset

恢复指定 Prompt 模板为默认值。

---

## 系统状态

所有接口需要认证。

### GET /api/status

获取系统综合状态。

**响应**：

```json
{
  "code": 200,
  "data": {
    "version": "4.0.0",
    "uptime": "2h30m15s",
    "go_version": "go1.22.0",
    "goroutines": 12,
    "scheduler": {
      "running": true,
      "interval": "*/5 * * * *",
      "interval_human": "每 5 分钟",
      "total": 2,
      "notifiers": 1
    },
    "system": {
      "hostname": "myserver",
      "platform": "linux/amd64",
      "cpu_cores": 4,
      "cpu_usage": 15.2,
      "mem_total": "8.0 GB",
      "mem_used": "3.2 GB",
      "mem_usage": 40.0,
      "load_avg": "0.5 0.3 0.2",
      "local_ipv4": "192.168.1.100",
      "local_ipv6": "2409:8a38:...",
      "interfaces": [
        {"name": "eth0", "addr": "192.168.1.100", "mac": "aa:bb:cc:dd:ee:ff", "rx_bytes": 1234567, "tx_bytes": 987654}
      ]
    },
    "ai": {
      "requests": 42,
      "errors": 2,
      "tool_calls": 158,
      "tool_errors": 3,
      "input_tokens": 123456,
      "output_tokens": 78901,
      "cost_usd": 0.123,
      "top_tools": [
        {"name": "read_file", "count": 50, "avg_ms": 12.3}
      ],
      "models": ["deepseek-chat", "qwen-max"]
    },
    "mcp": {
      "total": 2,
      "connected": 2,
      "tools": 15,
      "servers": [...]
    },
    "go_runtime": {
      "goroutines": 12,
      "heap_alloc_mb": 45,
      "heap_sys_mb": 80,
      "num_gc": 120,
      "gc_pause_us": 250
    },
    "notify": {
      "email_enabled": true,
      "webhook_enabled": false,
      "dingtalk_enabled": false
    }
  }
}
```

### POST /api/trigger

手动触发 DNS 更新检测。

**响应**：

```json
{
  "code": 200,
  "data": { "message": "更新任务已触发" }
}
```

### POST /api/system/gc

触发全局内存清理（Go GC + 系统缓存）。

**响应**：

```json
{
  "code": 200,
  "data": {
    "message": "全局内存已清理",
    "heap_freed_kb": 1024,
    "system_freed_kb": 20480,
    "before_free_kb": 1048576,
    "after_free_kb": 1069056,
    "total_mem_kb": 8192000,
    "drop_caches": true
  }
}
```

### GET /api/system/setup-status

获取系统初始化状态（用户/组/沙箱）。

### POST /api/system/run-setup

执行系统初始化（创建用户、组、沙箱目录、sudo 配置）。

### GET /api/sandbox/status

获取 AI 沙箱状态。

**响应**：

```json
{
  "code": 200,
  "data": {
    "sandbox": true,
    "root_dir": "/opt/yunxi-home/data/yunxiFiles"
  }
}
```

---

## AI 聊天

所有接口需要认证。聊天主接口使用 SSE 流式响应。

### POST /api/chat

发起聊天（SSE 流式）。

**请求**：

```json
{
  "message": "你好，帮我检查 DNS 状态",
  "session_id": "chat_1234567890",
  "model": "deepseek-chat",
  "plan_mode": false,
  "reasoning_intensity": "medium"
}
```

**响应**：SSE 事件流，每个事件为 JSON：

```json
{"type":"thinking","content":"正在分析..."}
{"type":"text","content":"DNS 状态正常..."}
{"type":"tool_call","tool":"get_status","args":"{}"}
{"type":"done","content":""}
```

### GET /api/chat/stream/:id

重连到活跃会话的 SSE 事件流。用于网络断开后恢复会话状态。

### POST /api/chat/confirm

确认危险操作。

**请求**：

```json
{
  "confirm_id": "confirm_xxx",
  "approved": true,
  "fields": {"reason": "已经确认安全"}
}
```

### POST /api/chat/respond

响应 AI 的交互式请求（如文件选择、表单填写）。

**请求**：

```json
{
  "request_id": "req_xxx",
  "session_id": "chat_xxx",
  "response": {"file_path": "/data/config.yaml"}
}
```

### POST /api/chat/inject

在流式输出中注入用户消息（不中断当前流）。

**请求**：

```json
{
  "session_id": "chat_xxx",
  "message": "再帮我检查一下 IPv4"
}
```

### POST /api/chat/command

执行指令（不经过 AI 推理，直接分发）。

**请求**：

```json
{
  "session_id": "chat_xxx",
  "command": "/reload-skills"
}
```

**内置指令**：`/help`、`/clear`、`/compact`、`/topology`、`/get-mcp`、`/reload-skills`、`/reload-mcp`

### GET /api/chat/commands

获取所有可用命令（内置 + 技能 + MCP）。

### POST /api/chat/clear

清除单个会话。

**请求**：

```json
{
  "session_id": "chat_xxx"
}
```

### POST /api/chat/clear-all

清除所有会话。

### GET /api/chat/sessions

列出所有会话。

**参数**：`type`（可选，如 `chat`）

### GET /api/chat/sessions/:id

获取会话详情（元数据 + 消息历史）。

### PATCH /api/chat/sessions/:id

更新会话元数据（标题、置顶）。

**请求**：

```json
{
  "title": "DNS 配置讨论",
  "pinned": true
}
```

### DELETE /api/chat/sessions/:id

删除单个会话。

### GET /api/chat/tools

获取已注册的工具列表。

### GET /api/chat/hints

获取上下文快捷提示。

**参数**：`session_id`（可选）

### 拓扑约束 API

#### GET /api/chat/sessions/:id/topology

获取会话的拓扑约束状态。

#### PUT /api/chat/sessions/:id/topology/constraint

更新拓扑约束参数。

**请求**：

```json
{
  "a": 0.8,
  "r": 0.5,
  "t": true,
  "force_tools": ["read_file", "write_file"]
}
```

#### POST /api/chat/sessions/:id/topology/override

强制接受下一次拓扑检查。

**请求**：

```json
{
  "target_coord": {"x": 0, "y": 0, "z": 0}
}
```

### 消息编辑 API

#### PUT /api/chat/sessions/:id/messages/:messageIndex

编辑或插入消息。

**请求**：

```json
{
  "content": "修正后的消息内容",
  "insert_mode": false
}
```

#### DELETE /api/chat/sessions/:id/messages/:messageIndex

删除指定消息。

### POST /api/chat/sessions/:id/interrupt

中断活跃会话。

**请求**：

```json
{
  "mode": "soft"
}
```

`mode` 可选值：`soft`（优雅中断）、`hard`（强制中断）。

---

## 技能与 MCP 市场

所有接口需要认证。

### POST /api/market/search-skills

在线搜索技能市场。

**请求**：

```json
{
  "query": "docker management"
}
```

### POST /api/market/install-skill

下载并安装技能。

**请求**：

```json
{
  "download_url": "https://example.com/skill.tar.gz",
  "skills_dir": "skills"
}
```

### POST /api/market/install-mcp

同步安装 MCP 服务器。

**请求**：

```json
{
  "package": "@anthropic/mcp-server-filesystem",
  "env": { "ROOT_DIR": "/data" }
}
```

若包需要必填参数且未提供 `env`，返回 `status: "need_params"`。

### POST /api/market/install-mcp-stream

SSE 流式安装 MCP（实时进度）。

**请求**：同 `install-mcp`，额外支持 `task_id`。

**响应**：SSE 流，每步返回进度：

```json
{"task_id":"mcp_xxx","step":"download","status":"running","message":"正在下载包...","progress":10}
```

### GET /api/market/install-tasks

获取当前安装任务列表（刷新不丢失）。

### GET /api/market/popular-mcp

获取热门 MCP 服务器推荐。

### GET /api/market/installed

获取已安装的技能和 MCP 服务器。

---

## 日志管理

所有接口需要认证。

### 聊天日志

#### GET /api/logs/chat/sessions

列出所有聊天会话日志。

#### GET /api/logs/chat/:id

获取聊天日志事件列表（支持过滤、排序、搜索、分页）。

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `type` | string | 按类型过滤，逗号分隔（如 `error,tool_call`） |
| `search` | string | 关键词搜索（匹配内容、工具名、错误信息） |
| `order` | string | `asc` 或 `desc`（默认 desc，最新在前） |
| `offset` | int | 分页偏移 |
| `limit` | int | 每页数量（默认 2000） |

#### GET /api/logs/chat/:id/errors

仅获取会话错误事件。

#### GET /api/logs/chat/:id/text

获取会话日志的纯文本视图。

#### GET /api/logs/chat/:id/tail

SSE 实时订阅会话日志。

#### GET /api/logs/chat/:id/download

下载原始日志文件（.log）。

#### DELETE /api/logs/chat/:id

删除聊天日志（运行中的会话不可删除，返回 409）。

### 系统日志

#### GET /api/logs/system

列出可用系统日志文件。

**参数**：`order`（`asc` / `desc`，默认 desc）

#### GET /api/logs/system/:date

获取指定日期的系统日志。

**参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `tail` | int | 获取最后 N 行 |
| `offset` | int | 分页偏移 |
| `limit` | int | 每页数量（默认 500） |
| `level` | string | 按级别过滤，逗号分隔（如 `ERROR,WARN`） |
| `search` | string | 关键词搜索 |
| `order` | string | `asc` 或 `desc` |

#### GET /api/logs/system/:date/download

下载原始系统日志文件。

#### DELETE /api/logs/system/:date

删除指定日期的系统日志。

---

## 定时任务

所有接口需要认证。

### GET /api/cron/tasks

列出指定会话的定时任务。

**参数**：`session_id`（必填）

### DELETE /api/cron/tasks/:id

删除单个定时任务。

---

## 文件管理 (NAS)

所有接口需要认证 + 文件访问权限检查。

### GET /api/nas/files

列出目录内容。

**参数**：`path`（默认 `/`）

### GET /api/nas/files/download

下载文件。

**参数**：`path`（必填）

### POST /api/nas/files/upload

上传文件（multipart/form-data）。

**参数**：`dir`（目标目录，默认 `/`），`file`（文件字段）

### POST /api/nas/files/mkdir

创建目录。

**请求**：

```json
{ "path": "/new-dir" }
```

### DELETE /api/nas/files

删除文件或目录。

**请求**：

```json
{ "path": "/file-to-delete.txt" }
```

### PUT /api/nas/files/rename

重命名 / 移动。

**请求**：

```json
{
  "old_path": "/old-name.txt",
  "new_path": "/new-name.txt"
}
```

### POST /api/nas/files/copy

复制文件或目录。

**请求**：

```json
{
  "src": "/source.txt",
  "dst": "/dest.txt"
}
```

### POST /api/nas/files/move

移动文件（同 rename）。

**请求**：

```json
{
  "src": "/source.txt",
  "dst": "/target/dest.txt"
}
```

### POST /api/nas/files/batch-delete

批量删除（最多 200 项）。

**请求**：

```json
{
  "paths": ["/file1.txt", "/file2.txt"]
}
```

### PUT /api/nas/files/save

保存文本文件内容。

**参数**：`path`（必填），请求体为纯文本内容。

### POST /api/nas/files/batch-download

打包下载多个文件为 ZIP。

**请求**：

```json
{
  "paths": ["/file1.txt", "/file2.txt"]
}
```

### GET /api/nas/diskinfo

获取磁盘信息。

**参数**：`path`（默认 `/`）

### GET /api/nas/search

搜索文件。

**参数**：`q`（必填）、`path`（默认 `/`）、`recursive`（`true`/`false`）、`depth`（默认 3，最大 10）

### GET /api/nas/files/stat

获取文件元数据。

**参数**：`path`（必填）

### GET /api/nas/files/tree

获取目录树结构（仅目录，最大深度 5 层）。

**参数**：`path`（默认 `/`）

### 流媒体与预览

#### GET /api/nas/files/stream

视频/音频流播放，支持 HTTP Range（Seeking）。

**参数**：`path`（必填）

#### GET /api/nas/files/preview

文件预览（限制 4MB）。

**参数**：`path`（必填）

### 分片上传

用于大文件上传（> 100MB）。

#### POST /api/nas/files/upload/init

初始化分片上传。

**请求**：

```json
{
  "filename": "large-video.mp4",
  "dir": "/videos",
  "total_size": 524288000,
  "chunk_size": 5242880
}
```

#### POST /api/nas/files/upload/chunk

保存分片（multipart/form-data）。

**参数**：`upload_id`、`chunk_index`、`file`（分片文件字段）

#### POST /api/nas/files/upload/complete

合并分片，完成上传。

**请求**：

```json
{ "upload_id": "upload_xxx" }
```

#### GET /api/nas/files/upload/status

查询上传进度。

**参数**：`upload_id`（必填）

#### POST /api/nas/files/upload/abort

中止上传并清理临时文件。

**请求**：

```json
{ "upload_id": "upload_xxx" }
```

---

## 文件分享

所有管理接口需要认证。访问接口公开但可能需要密码。

### POST /api/nas/shares

创建分享链接。

**请求**：

```json
{
  "file_path": "/docs/report.pdf",
  "expire_days": 7,
  "password": "share-password"
}
```

**响应**：

```json
{
  "code": 200,
  "data": {
    "id": 1,
    "token": "abc123def456",
    "file_path": "/docs/report.pdf",
    "share_url": "/s/abc123def456",
    "has_pass": true,
    "expires": "2026-01-08T00:00:00Z"
  }
}
```

### GET /api/nas/shares

列出所有分享（分页）。

**参数**：`page`、`size`

### DELETE /api/nas/shares/:id

删除分享。

### GET /s/:token

通过 token 访问分享（公开）。

**参数**：`pass`（分享设置了密码时必填）

---

## Docker 管理

所有接口需要认证。

### GET /api/docker/containers

列出容器。

**参数**：`all`（`true` 包含已停止的容器）

### POST /api/docker/containers/:name/:action

容器操作。

**action 可选值**：`start`、`stop`、`restart`、`pause`、`unpause`

**响应**：

```json
{
  "code": 200,
  "data": {
    "message": "操作成功",
    "output": "..."
  }
}
```

### GET /api/docker/containers/:name/logs

获取容器日志。

**参数**：`tail`（默认 100 行）

### GET /api/docker/containers/:name/stats

获取容器实时资源统计（CPU、内存、网络）。

### POST /api/docker/compose/:action

Docker Compose 操作。

**参数**：`dir`（项目目录，默认 `/app/deploy`）

**action 可选值**：`up`、`down`、`restart`、`pull`、`ps`

---

## 系统控制

所有接口需要认证。

### GET /api/sysctl/info

获取系统信息（CPU、内存、磁盘、网络接口）。

### GET /api/sysctl/processes

列出系统进程。

**参数**：`limit`（默认 50）

### POST /api/sysctl/processes/:pid/kill

终止进程。

**请求**：

```json
{ "force": true }
```

### GET /api/sysctl/services

列出系统服务。

### POST /api/sysctl/services/:name/:action

控制系统服务。

**action 可选值**：`start`、`stop`、`restart`

---

## 终端 WebSocket

### GET /api/terminal

WebSocket 终端连接（需要认证）。

Upgrade 到 WebSocket 后可进行交互式终端操作。仅管理员可访问（`terminal.admin_only` 为 true 时）。

---

## 管理员

所有接口需要认证 + admin 角色。

### 用户管理

#### GET /api/admin/users

列出所有用户。

#### POST /api/admin/users

创建用户。

**请求**：

```json
{
  "username": "newuser",
  "password": "user-password",
  "role": "user",
  "storage_quota": 1073741824
}
```

- `role` 可选 `admin` 或 `user`
- `storage_quota` 为存储配额（字节），0 表示无限制

#### PUT /api/admin/users/:id

更新用户（密码、角色、配额均可选）。

**请求**：

```json
{
  "password": "new-password",
  "role": "admin",
  "storage_quota": 2147483648
}
```

#### DELETE /api/admin/users/:id

删除用户。

### 文件权限管理

#### GET /api/admin/file-permissions

列出文件权限。

**参数**：`user_id`（可选，不传则列出全部）

#### POST /api/admin/file-permissions

创建或更新文件权限。

**请求**：

```json
{
  "user_id": 2,
  "path": "/data/private",
  "can_read": true,
  "can_write": false,
  "can_delete": false,
  "can_share": false
}
```

#### DELETE /api/admin/file-permissions/:id

删除文件权限。

---

## 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证（Token 过期或无效） |
| 403 | 权限不足（非 admin 访问管理接口） |
| 404 | 资源不存在 |
| 409 | 资源冲突（如删除运行中的会话日志） |
| 413 | 请求体过大 / 存储配额不足 |
| 429 | 请求过于频繁（触发限流） |
| 500 | 服务器内部错误 |
| 503 | 服务未就绪 |
