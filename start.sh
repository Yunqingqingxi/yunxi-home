#!/bin/bash
# Yunxi Home — 一键启动脚本
set -e

cd "$(dirname "$0")"

PORT=${PORT:-9981}
CONFIG="${CONFIG:-./configs/config.yaml}"
BINARY="./build/yunxi-home"

# 自动编译（如需）
if [ ! -f "$BINARY" ]; then
    echo ">>> 未找到二进制文件，正在编译..."
    export GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
    export PATH=$PATH:/usr/local/go/bin
    cd web && npm install --silent && npm run build && cd ..
    rm -rf internal/web/static && mkdir -p internal/web/static
    cp -r web/dist/* internal/web/static/
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY" ./cmd/yunxi-home/
    echo ">>> 编译完成"
fi

# 确保目录
mkdir -p ./data ./log

echo ">>> 启动 Yunxi Home (端口: $PORT)..."
exec "$BINARY" -config "$CONFIG"
