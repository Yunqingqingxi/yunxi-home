---
name: github-info
description: GitHub 仓库地址、Token 存放位置和认证检查方法
type: reference
---

# GitHub 信息

## 仓库
- 地址：`github.com/Yunqingqingxi/yunxi-home`
- 默认分支：`main`
- Git 用户：yunxi
- **仓库是公开的（public）！** `git clone https://github.com/Yunqingqingxi/yunxi-home.git` 不需要任何认证
- **只有 push 才需要认证**；clone/pull 公开仓库直接用 HTTPS 即可

## Token 存放位置
按优先级顺序查找：
1. 环境变量：`$GITHUB_TOKEN` 或 `$GH_TOKEN`
2. 凭据文件：`~/.git-credentials`
3. Windows 凭据管理器
4. GitHub CLI 登录状态：运行 `gh auth status` 检查

## 操作规范
- **clone/pull 公开仓库不需要认证**——直接 `git clone https://github.com/Yunqingqingxi/yunxi-home.git` 即可
- **只有 push 才需要 Token 或 SSH 密钥**
- 项目代码中不存储 Token
- 不要将 Token 写入任何会被 git 追踪的文件

## Why
AI 之前多次向用户索要 GitHub Token 但跨会话遗忘。记录 Token 位置策略可以让 AI 先自行检查认证状态。

## How to apply
操作 git 远程仓库前先验证认证状态。如果找不到 Token，明确告知用户需要设置哪个环境变量。
