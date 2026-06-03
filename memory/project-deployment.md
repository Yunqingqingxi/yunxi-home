---
name: project-deployment
description: yunxi-home 的本地开发路径和生产服务器部署信息
type: project
---

# 项目部署信息

## 本地开发环境
- 路径：`d:\code\dns-updater-go`
- 系统：Windows 11 Pro
- 用途：代码开发和本地测试

## 生产服务器
- 路径：`/opt/yunxi-home`
- 系统：Ubuntu 22.04 LTS（远程 Linux 服务器）
- 服务管理：systemd（服务名：yunxi-home）
- 端口：9981

## 关键事实
- **`/opt/yunxi-home` 是远程服务器路径，不在本地开发机上**
- 需要通过 SSH 连接到服务器才能操作该路径
- 服务器上的 `/opt/yunxi-home` 是一个 git 仓库
- 本地开发机无法直接访问 `/opt/yunxi-home`

## 数据库
- 类型：SQLite（加密存储）
- 路径：`./data/yunxi-home.db`（相对于工作目录）
- 加密：AES-256-GCM

## Why
会话 chat_178 中 AI 误以为 `/opt/yunxi-home` 在本地，浪费多轮尝试 `cd /opt/yunxi-home && git pull`。必须明确区分本地和远程环境。

## How to apply
当被要求"拉取最新代码"或"更新服务器"时，首先确认操作的是哪个环境。操作生产服务器需要 SSH 凭证。
