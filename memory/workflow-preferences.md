---
name: workflow-preferences
description: 用户的工作习惯、构建命令和部署流程
type: feedback
---

# 工作偏好

## 权限
- 用户已授权全部权限（Bash(*)、Write(*)、Edit(*) 等）
- **直接执行任务，不需要反复请求确认**

## 构建命令
- Go 构建（Windows）：`go build -o build/yunxi-home.exe ./cmd/yunxi-home/`
- Go 构建（Linux）：`go build -o build/yunxi-home ./cmd/yunxi-home/`
- 前端构建：`cd web && npm run build`
- 前端部署：`cp -r web/dist/* internal/web/static/`
- 测试：从项目根目录执行 `go test ./...`

## 部署流程
- Windows 本地编译 → scp 二进制到 Linux 服务器 → `sudo systemctl restart yunxi-home`

## Why
用户已授予广泛权限，期望 AI 直接行动而非反复确认。从会话历史和 settings.local.json 观察得知。

## How to apply
接到任务后直接执行。使用上述命令作为默认值。遇到需要用户决策的 destructive 操作时才询问。
