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

## Token 存放位置
按优先级顺序查找：
1. 环境变量：`$GITHUB_TOKEN` 或 `$GH_TOKEN`
2. 凭据文件：`~/.git-credentials`
3. Windows 凭据管理器
4. GitHub CLI 登录状态：运行 `gh auth status` 检查

## 操作规范
- **任何 git push/pull 操作前，先用 `gh auth status` 检查认证状态**
- 如果未认证，告知用户并建议运行 `gh auth login`
- 项目代码中不存储 Token
- 不要将 Token 写入任何会被 git 追踪的文件

## Why
AI 之前多次向用户索要 GitHub Token 但跨会话遗忘。记录 Token 位置策略可以让 AI 先自行检查认证状态。

## How to apply
操作 git 远程仓库前先验证认证状态。如果找不到 Token，明确告知用户需要设置哪个环境变量。
