#!/bin/bash
# 跨平台编译脚本
set -euo pipefail

VERSION=${1:-"3.0.0"}
BUILD_TIME=${2:-$(date -u '+%Y-%m-%d_%H:%M:%S')}
GIT_COMMIT=${3:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
APP_NAME="yunxi-home"
OUTPUT_DIR="./build"

LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm/7"
    "windows/amd64"
    "darwin/amd64"
    "darwin/arm64"
)

echo "========================================="
echo "  Yunxi Home 跨平台编译"
echo "  Version: ${VERSION}"
echo "  Build  : ${BUILD_TIME}"
echo "========================================="

mkdir -p "${OUTPUT_DIR}"

for target in "${TARGETS[@]}"; do
    IFS='/' read -r GOOS GOARCH GOARM <<< "${target}"

    if [ "${GOOS}" = "windows" ]; then
        OUTPUT="${OUTPUT_DIR}/${APP_NAME}-${GOOS}-${GOARCH}.exe"
    else
        OUTPUT="${OUTPUT_DIR}/${APP_NAME}-${GOOS}-${GOARCH}"
    fi

    ARM_VARIANT=""
    [ -n "${GOARM:-}" ] && ARM_VARIANT="GOARM=${GOARM}"

    echo "编译: ${GOOS}/${GOARCH}${GOARM:+ v${GOARM}} ..."

    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} ${ARM_VARIANT} \
        go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" ./cmd/yunxi-home/

    echo "  -> ${OUTPUT} ($(du -h "${OUTPUT}" | cut -f1))"
done

echo ""
echo "所有平台编译完成！"
echo "输出目录: ${OUTPUT_DIR}/"
ls -lh "${OUTPUT_DIR}/"
