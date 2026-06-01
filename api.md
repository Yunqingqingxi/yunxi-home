# Yunxi Home API 文档

Base URL: `http://{host}:9981/api`

## 认证

所有 `/api/*` 接口（除 auth 登录相关外）需携带 JWT Token：`Authorization: Bearer <token>`

### POST /api/auth/login
登录获取 Token。
```
Body: { "username": "admin", "password": "xxx" }
Response: { "code": 200, "data": { "token": "...", "username": "admin", "role": "admin" } }
```

### GET /api/auth/status
检查是否需要初始化设置。
```
Response: { "code": 200, "data": { "needs_setup": false } }
```

### POST /api/auth/setup
首次初始化管理员密码（仅未设置时可用）。
```
Body: { "password": "xxx" }
```

### POST /api/auth/refresh
刷新 Token。

### POST /api/auth/change-password
修改当前用户密码。
```
Body: { "current": "old", "new": "new" }
```

---

## 系统状态

### GET /api/status
获取系统状态（CPU、内存、调度器、网络接口、运行时间、版本）。
```
Response: {
  "code": 200,
  "data": {
    "cpu_cores": 4, "cpu_usage": 12.5, "mem_usage": 45.2,
    "mem_total": "8GB", "mem_used": "3.6GB",
    "goroutines": 24, "go_version": "go1.26.0",
    "version": "v4.0.0", "uptime": "2h 30m",
    "scheduler": { "running": true, "total": 2, "notifiers": 1, "cron_entries": 2, "records": [...] },
    "system": { "interfaces": [...], "load_avg": {...} }
  }
}
```

### POST /api/trigger
手动触发 DNS 检测更新。

### POST /api/system/gc
触发 Go GC 释放内存。

### GET /api/system/setup-status
获取系统初始化状态。

### POST /api/system/run-setup
运行系统初始化。

---

## DNS 管理

### GET /api/domains/cloud
列出阿里云域名。

### GET /api/domains/cloud/records
列出云解析记录。Query: `?domain=example.com&rr=www&type=A`

### POST /api/domains/cloud/records
创建云解析记录。
```
Body: { "domain": "example.com", "rr": "www", "type": "A", "value": "1.2.3.4", "ttl": 600 }
```

### PUT /api/domains/cloud/records/:recordId
更新云解析记录。

### DELETE /api/domains/cloud/records/:recordId
删除云解析记录。

### GET /api/domains
列出本地域名记录。

### GET /api/domains/:id
获取单条记录。

### POST /api/domains
创建本地记录。

### PUT /api/domains/:id
更新本地记录。

### DELETE /api/domains/:id
删除本地记录。

---

## 更新历史

### GET /api/history
分页查询更新历史。Query: `?page=1&size=20`

### GET /api/history/stats
获取历史统计。

### DELETE /api/history/clean
清理旧历史记录。

---

## 配置管理

### GET /api/config
获取全部配置（加密字段脱敏）。

### GET /api/config/:section
获取单个配置节（alidns / notify / ai / detect / database / server / nas / qqbot / auth / log）。

### PUT /api/config
批量更新配置。

### PUT /api/config/:section
更新单个配置节。
```
Body: { "enabled": true, "host": "smtp.qq.com", ... }
```

---

## AI 对话

### POST /api/chat
发送消息，SSE 流式返回。
```
Body: { "message": "你好", "session_id": "chat_xxx", "model": "deepseek-v4-pro", "plan_mode": false, "reasoning_intensity": "medium" }
Response: text/event-stream
Events: thinking | content | tool_call | tool_start | tool_progress | tool_result | confirm_required | agent_progress | agent_result | todo_update | error | done
```

### GET /api/chat/stream/:id
重连活跃会话的事件流（页面刷新后恢复）。

### POST /api/chat/inject
在流式对话中注入消息（不中断 AI）。
```
Body: { "session_id": "chat_xxx", "message": "继续" }
```

### POST /api/chat/confirm
确认/取消危险操作。
```
Body: { "confirm_id": "confirm_1_1", "approved": true }
```

### GET /api/chat/sessions
列出所有会话。

### GET /api/chat/sessions/:id
获取会话详情（含消息历史）。

### DELETE /api/chat/sessions/:id
删除会话。

### POST /api/chat/clear
清除单个会话。Body: `{ "session_id": "..." }`

### POST /api/chat/clear-all
清除所有会话。

### GET /api/chat/tools
返回可用工具列表（JSON Schema）。

---

## 定时任务

### GET /api/cron/tasks
列出定时任务。Query: `?session_id=chat_xxx`

### DELETE /api/cron/tasks/:id
删除定时任务。

---

## NAS 文件管理

### GET /api/nas/diskinfo
磁盘信息。Query: `?path=/`

### GET /api/nas/files
列出目录。Query: `?path=/`

### POST /api/nas/files/upload
上传文件（multipart/form-data）。

### POST /api/nas/files/mkdir
创建目录。Body: `{ "path": "/newdir" }`

### POST /api/nas/files/rename
重命名。Body: `{ "path": "/old", "name": "new" }`

### POST /api/nas/files/delete
删除文件/目录。Body: `{ "path": "/file" }`

### POST /api/nas/files/copy
复制。Body: `{ "src": "/a", "dst": "/b" }`

### POST /api/nas/files/move
移动。Body: `{ "src": "/a", "dst": "/b" }`

### POST /api/nas/files/write
写入文件。Body: `{ "path": "/f.txt", "content": "..." }`

### GET /api/nas/files/read
读取文件。Query: `?path=/f.txt`

### GET /api/nas/files/download
下载文件。Query: `?path=/f.txt`

### 分块上传
- POST /api/nas/files/upload/init — 初始化
- POST /api/nas/files/upload/chunk — 上传分块
- POST /api/nas/files/upload/complete — 合并完成
- GET /api/nas/files/upload/status — 查询状态
- POST /api/nas/files/upload/abort — 取消上传

### 分享
- POST /api/nas/shares — 创建分享
- GET /api/nas/shares — 列出分享
- DELETE /api/nas/shares/:id — 删除分享

---

## 沙箱

### GET /api/sandbox/status
获取文件沙箱状态（根目录、已用/总量）。

---

## Docker

### GET /api/docker/containers
列出容器。

### POST /api/docker/containers/:name/:action
容器操作（start/stop/restart）。

### GET /api/docker/containers/:name/logs
获取容器日志。

### GET /api/docker/containers/:name/stats
获取容器资源统计。

### POST /api/docker/compose/:action
Docker Compose 操作（up/down/restart）。

---

## 其他

### GET /health
健康检查。

### GET /ready
就绪检查。

### GET /api/terminal
WebSocket 终端。

### GET /*
静态文件服务（前端 SPA）。
