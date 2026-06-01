#!/bin/bash
# Yunxi Home — 一键发布打包 (Linux + Windows)
# 用法: bash scripts/release.sh [version]
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-4.0.0}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

RELEASE_DIR="./release"
APP_NAME="yunxi-home"

echo "=========================================="
echo "  Yunxi Home 发布打包"
echo "  Version: ${VERSION}"
echo "  Build  : ${BUILD_TIME}"
echo "=========================================="
echo ""

rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"

# ── 前端构建 ──────────────────────────────────────
echo "→ 构建前端 ..."
# Windows 兼容：npm ci 失败时 fallback 到 npm install
cd web && {
    if [ -d "node_modules" ]; then
        npm ci --silent 2>/dev/null || { echo "    尝试 npm install..."; rm -rf node_modules && npm install --silent; }
    else
        npm install --silent
    fi
} && npm run build && cd ..
rm -rf internal/web/static/*
cp -r web/dist/* internal/web/static/
echo "  ✓ 前端已构建并嵌入"

# ── Linux amd64 ────────────────────────────────────
echo "→ linux/amd64 ..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "${LDFLAGS}" \
    -o "${RELEASE_DIR}/${APP_NAME}" \
    ./cmd/yunxi-home/
echo "  ✓ ${RELEASE_DIR}/${APP_NAME}  ($(du -h "${RELEASE_DIR}/${APP_NAME}" | cut -f1))"

# ── Windows amd64 ──────────────────────────────────
echo "→ windows/amd64 ..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
    -ldflags "${LDFLAGS}" \
    -o "${RELEASE_DIR}/${APP_NAME}.exe" \
    ./cmd/yunxi-home/
echo "  ✓ ${RELEASE_DIR}/${APP_NAME}.exe  ($(du -h "${RELEASE_DIR}/${APP_NAME}.exe" | cut -f1))"

echo ""
echo "=========================================="
echo "  打包完成: ${RELEASE_DIR}/"
echo "=========================================="
ls -lh "${RELEASE_DIR}/"
