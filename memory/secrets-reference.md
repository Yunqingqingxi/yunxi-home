---
name: secrets-reference
description: 各类 API 密钥和凭证的环境变量名称
type: reference
---

# 密钥存放参考

> **绝对禁止在记忆文件或代码中存储明文密钥！**
> 以下仅记录密钥的**环境变量名称**。

## API 密钥
| 用途 | 环境变量 |
|------|---------|
| DeepSeek API（AI 聊天） | `$DEEPSEEK_API_KEY` |
| DashScope API（Qwen VL 视觉） | `$DASHSCOPE_API_KEY` |
| 阿里云 DNS AccessKey ID | `$DNS_UPDATER_ALIDNS_ACCESS_KEY_ID` |
| 阿里云 DNS AccessKey Secret | `$DNS_UPDATER_ALIDNS_ACCESS_KEY_SECRET` |

## 服务密钥
| 用途 | 环境变量 |
|------|---------|
| JWT 签名密钥 | `$DNS_UPDATER_AUTH_JWT_SECRET` |
| 数据库加密密钥 | `./data/.encryption_key`（自动生成） |

## GitHub 认证
- 详见 [[github-info]]

## Why
集中记录所有密钥的环境变量名称，避免 AI 反复猜测或询问。

## How to apply
任务需要某个密钥时，先查此文件找到对应环境变量名，再检查是否已设置。未设置时明确告知用户需要设置哪个变量。
