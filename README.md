# 🏠 Yunxi Home (云兮之家)

> Self-hosted home server intelligence hub — DNS, AI Chat, Files, Docker, Monitoring, all-in-one

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Yunxi Home is a single-binary (~18MB) home server management panel with an AI assistant at its core. It wraps DNS automation, file management, Docker control, system monitoring, and a Web Terminal behind a glassmorphism UI — all controllable through natural language via the built-in AI chat.

## Features

| Module | Capabilities |
|--------|-------------|
| 🤖 **AI Assistant** | Streaming chat, parallel sub-agents, YAML-based skill system, MCP protocol support, cron tasks, Plan mode, slash commands (`/help`, `/compact`, etc.) |
| 🌐 **DNS Manager** | Alibaba Cloud (AliDNS) DDNS, IPv4/IPv6 auto-detection, scheduled & manual updates, full record CRUD |
| 💬 **QQ Bot** | Multi-bot support, group/private chat, AI-powered replies, command system, file transfer |
| 📁 **File Manager** | Browse, upload, download, preview, share, rename, copy, delete, chunked uploads, sandbox isolation |
| 🐳 **Docker** | Container list, start/stop/restart, log viewer, resource stats, docker-compose management |
| 📊 **Dashboard** | Real-time CPU, memory, disk, network charts (Chart.js), 750ms refresh |
| 🖥 **Terminal** | In-browser Web Terminal (xterm.js + PTY) |
| 👤 **Auth** | JWT authentication, admin/user roles, per-file access control |
| 📢 **Notifications** | Email, Webhook, DingTalk channels with event-driven throttling |

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24+ · Echo · Viper · SQLite (encrypted) |
| Frontend | Vue 3 · Pinia · Vue Router · Vite · Arco Design · Chart.js · xterm.js |
| AI | DeepSeek v4 (Flash/Pro) · Qwen (Plus/Max) · OpenAI-compatible providers |
| Tools | 50+ built-in tools across system, file, DNS, Docker, MCP integration |

## Quick Start

### Prerequisites
- Go 1.24+
- Node.js 18+ (for frontend builds)
- Linux server (Ubuntu 22.04+ recommended)

### Build from source

```bash
# Clone
git clone https://github.com/Yunqingqingxi/yunxi-home.git
cd yunxi-home

# Build frontend + backend (single binary)
make build
# Binary: ./build/yunxi-home

# Or development mode
cd web && npm install && npm run build && cd ..
go run ./cmd/yunxi-home/
```

### Configuration

Configuration is managed through the Web Settings UI (`/settings`). On first startup, defaults are written to an encrypted SQLite database — no manual YAML editing needed.

Secret fields (API keys, passwords) can be injected via environment variables:
```bash
export DEEPSEEK_API_KEY=sk-xxx
export ALIYUN_ACCESS_KEY_ID=xxx
export ALIYUN_ACCESS_KEY_SECRET=xxx
```

Advanced users can provide a `configs/config.yaml` for custom paths and server options.

### Deploy

```bash
# Full deploy: build + SCP upload + systemd restart
bash scripts/deploy.sh

# Build only, no upload
bash scripts/deploy.sh --dry-run

# Rollback to previous version
bash scripts/deploy.sh --rollback
```

A systemd service template is provided at [`deploy/yunxi-home.service`](deploy/yunxi-home.service).

## Project Structure

```
├── cmd/yunxi-home/          # Entry point
├── internal/
│   ├── ai/                  # AI engine (providers, agents, skills, MCP, cron, sessions)
│   ├── config/              # Config system (encrypted DB + env)
│   ├── crypto/              # AES encryption layer
│   ├── database/            # SQLite data access
│   ├── dns/                 # AliDNS API client
│   ├── docker/              # Docker Engine API
│   ├── ipdetect/            # Public IP detection (multi-source)
│   ├── models/              # Data models
│   ├── nas/                 # File service with sandbox
│   ├── notifier/            # Email/Webhook/DingTalk
│   ├── qqbot/               # QQ Bot integration
│   ├── scheduler/           # DNS update scheduler
│   ├── sysctl/              # System metrics collection
│   ├── terminal/            # PTY WebSocket server
│   ├── toolreg/             # AI tool registration
│   └── web/                 # HTTP server + API handlers
├── web/                     # Vue 3 SPA
│   └── src/
│       ├── components/      # UI components (chat, files, etc.)
│       ├── stores/          # Pinia state management
│       └── views/           # Page components
├── skills/                  # YAML skill templates
├── scripts/                 # Build & deploy shell scripts
├── deploy/                  # Docker compose + systemd configs
└── docs/                    # API documentation
```

## AI Tools

The AI assistant has access to 50+ registered tools across these categories:

| Category | Examples |
|----------|---------|
| **File** | `file_list`, `file_read`, `file_write`, `file_delete`, `file_search` |
| **Command** | `run_command` (sandboxed), `command_status` |
| **DNS** | `dns_list`, `dns_update`, `dns_add`, `dns_delete`, `dns_check` |
| **Docker** | `docker_list`, `docker_start`, `docker_stop`, `docker_logs`, `docker_stats` |
| **System** | `system_status`, `get_processes`, `get_services`, `disk_usage` |
| **Agent** | `spawn_agent`, `todo_write` |
| **Skill** | `run_skill`, `list_skills` |
| **MCP** | `get_mcp_status`, `reload_mcp`, `reload_skills` |

Skills are YAML-defined workflows that chain multiple tool calls. See [`skills/`](skills/) for examples.

## MCP Support

Yunxi Home implements the [Model Context Protocol](https://modelcontextprotocol.io/) as a client. Configure MCP servers in `mcp.json` (copy from `mcp.example.json`):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-filesystem", "/allowed/path"]
    }
  }
}
```

Use `/reload-mcp` or the Settings page to hot-reload MCP connections.

## API

See [docs/API.md](docs/API.md) for the full REST API reference.

## License

[MIT](LICENSE) © 2026 Yunxi Home Contributors
