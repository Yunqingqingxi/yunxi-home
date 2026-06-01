# DNS Updater Go — API 文档

## 认证

除登录和健康检查外，所有 `/api/*` 接口需要 JWT 认证。

```
Authorization: Bearer <jwt_token>
```

## 通用响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

- `code: 0` 表示成功，`code: -1` 表示失败

---

## 公开接口

### GET /health

存活探针。

**响应**：`{"status": "alive"}`

### GET /ready

就绪探针。

**响应**：`{"status": "ready"}` 或 HTTP 503

### POST /api/auth/login

用户登录。

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
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOi...",
    "username": "admin",
    "role": "admin"
  }
}
```

---

## 认证接口

### POST /api/auth/refresh

刷新 JWT Token（需要认证）。

**响应**：同登录接口

---

## 域名管理（需要认证）

### GET /api/domains

获取所有域名记录。

**响应**：
```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "domain": "example.com",
      "rr": "@",
      "type": "AAAA",
      "value": "2409:8a38:...",
      "ttl": 600,
      "enabled": true,
      "cron_expr": "*/5 * * * *",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:05:00Z"
    }
  ]
}
```

### GET /api/domains/:id

获取单条记录。

### POST /api/domains

创建域名记录。

**请求**：
```json
{
  "domain": "example.com",
  "rr": "@",
  "type": "AAAA",
  "ttl": 600,
  "cron_expr": "*/5 * * * *",
  "enabled": true
}
```

### PUT /api/domains/:id

更新域名记录（部分字段可选）。

### DELETE /api/domains/:id

删除域名记录。

---

## 历史记录（需要认证）

### GET /api/history?page=1&size=20&domain=example.com

分页查询更新历史。

**响应**：
```json
{
  "code": 0,
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

### DELETE /api/history/clean?days=90

清理旧历史记录。

---

## 状态与控制（需要认证）

### GET /api/status

获取系统状态。

**响应**：
```json
{
  "code": 0,
  "data": {
    "version": "3.0.0",
    "uptime": "2h30m15s",
    "go_version": "go1.22.0",
    "goroutines": 12,
    "scheduler": {
      "running": true,
      "interval": "*/5 * * * *",
      "total": 2,
      "notifiers": 1
    }
  }
}
```

### POST /api/trigger

手动触发 DNS 更新检测。

**响应**：
```json
{
  "code": 0,
  "data": { "message": "更新任务已触发" }
}
```

---

## 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 400 | 请求参数错误 |
| 401 | 未认证（Token 过期或无效） |
| 404 | 资源不存在 |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |
