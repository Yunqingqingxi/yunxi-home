# Contributing to Yunxi Home

Thank you for considering contributing! This document outlines the basics.

## Development Setup

### Prerequisites

- **Go 1.24+** — [Download](https://go.dev/dl/)
- **Node.js 18+** — [Download](https://nodejs.org/)
- Linux server (Ubuntu 22.04+ recommended) or WSL2 on Windows

### Getting Started

```bash
git clone https://github.com/Yunqingqingxi/yunxi-home.git
cd yunxi-home

# Build everything (frontend + backend)
make build
```

## Code Style

- **Go** — Run `go fmt ./...` before committing. Follow standard Go conventions (`gofmt`, `go vet`).
- **Frontend** — Run `npx eslint web/src --fix` for Vue/JS files.
- Ensure all tests pass: `make test` (if available).

## How to Submit a PR

1. Fork the repository and create a feature branch from `main`.
2. Make your changes, keeping commits small and descriptive.
3. Run code formatting and lint checks.
4. Open a pull request against `main` with a clear title and description.
5. A maintainer will review your PR — expect feedback or approval within a few days.

## Reporting Issues

Report bugs and request features via [GitHub Issues](https://github.com/Yunqingqingxi/yunxi-home/issues). Include as much detail as possible (OS, Go version, steps to reproduce).
