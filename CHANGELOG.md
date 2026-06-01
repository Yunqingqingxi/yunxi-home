# Changelog

## v3.0.0 (2026-06-01)

Initial public release of Yunxi Home (云兮之家) — a self-hosted home server intelligence hub.

### Key Features

- **AI Assistant** — Streaming chat with DeepSeek v4 (Flash/Pro), Qwen (Plus/Max), and OpenAI-compatible providers. Supports parallel sub-agents, YAML-based skill system, MCP protocol, cron tasks, Plan mode, and slash commands (`/help`, `/compact`, etc.).
- **Sub-agents** — Spawn parallel AI agents for concurrent task execution with isolated contexts.
- **Skills** — YAML-defined workflow templates that chain multiple tool calls for repeatable automation.
- **Slash Commands** — Built-in commands (`/help`, `/compact`, `/reload-mcp`, `/reload-skills`, etc.) for quick operations.
- **MCP Support** — Model Context Protocol client integration, hot-reloadable MCP server connections.
- **DNS Automation** — Alibaba Cloud (AliDNS) DDNS with IPv4/IPv6 auto-detection, scheduled and manual updates, full record CRUD.
- **File Management** — Browse, upload, download, preview, share, rename, copy, delete, chunked uploads, sandbox isolation.
- **Docker Control** — Container list, start/stop/restart, log viewer, resource stats, docker-compose management.
- **System Monitoring** — Real-time CPU, memory, disk, network charts (Chart.js) with 750ms refresh.
- **QQ Bot** — Multi-bot support, group/private chat, AI-powered replies, command system, file transfer.
- **Web Terminal** — In-browser terminal via xterm.js + PTY WebSocket.
- **Glassmorphism UI** — Modern Vue 3 SPA with Arco Design, dark/light theme, responsive layout.
- **Authentication** — JWT-based auth with admin/user roles and per-file access control.
- **Notifications** — Email, Webhook, DingTalk channels with event-driven throttling.
- **Single Binary** — ~18MB self-contained binary with embedded frontend assets.
