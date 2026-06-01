#!/bin/bash
# Yunxi Home — SSH 部署脚本
# 用法: bash scripts/deploy.sh [--dry-run|--rollback]
set -euo pipefail

cd "$(dirname "$0")/.."

# ── 配置 ────────────────────────────────────────────
SSH_HOST="yxdthird.top"
REMOTE_DIR="/opt/yunxi-home"
SERVICE_NAME="yunxi-home"
KEEP_RELEASES=3
BINARY="yunxi-home"
BUILD_DIR="./build"

# ── 参数解析 ────────────────────────────────────────
DRY_RUN=false
ROLLBACK=false

for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=true ;;
        --rollback) ROLLBACK=true ;;
        *) echo "未知参数: $arg"; echo "用法: bash scripts/deploy.sh [--dry-run|--rollback]"; exit 1 ;;
    esac
done

# ── 回滚模式 ────────────────────────────────────────
if $ROLLBACK; then
    echo ">>> 列出服务器上的备份版本..."
    ssh "$SSH_HOST" "ls -1t ${REMOTE_DIR}/${BINARY}.* 2>/dev/null" || {
        echo "没有找到备份文件"
        exit 1
    }

    echo ""
    echo "可用的备份版本:"
    ssh "$SSH_HOST" "ls -1t ${REMOTE_DIR}/${BINARY}.* 2>/dev/null" | cat -n

    echo ""
    read -rp "输入要恢复的编号 (回车取消): " choice
    if [ -z "$choice" ]; then
        echo "已取消"
        exit 0
    fi

    backup=$(ssh "$SSH_HOST" "ls -1t ${REMOTE_DIR}/${BINARY}.* 2>/dev/null" | sed -n "${choice}p")
    if [ -z "$backup" ]; then
        echo "无效的编号"
        exit 1
    fi

    echo ">>> 恢复: $backup"
    ssh "$SSH_HOST" <<SSH_ROLLBACK
        set -e
        sudo systemctl stop ${SERVICE_NAME}
        cp "$backup" "${REMOTE_DIR}/${BINARY}"
        sudo systemctl start ${SERVICE_NAME}
        echo "已恢复并重启服务"
SSH_ROLLBACK
    echo ">>> 回滚完成"
    exit 0
fi

# ── 构建前端 ────────────────────────────────────────
echo ">>> 构建前端..."
if [ -d "web" ] && [ -f "web/package.json" ]; then
    cd web
    # Windows 兼容：优先用 npm ci，权限失败时 fallback 到 npm install
    if [ -d "node_modules" ]; then
        npm ci --silent 2>/dev/null || {
            echo "    npm ci 失败（可能是 Windows 文件锁定），尝试 npm install..."
            rm -rf node_modules
            npm install --silent
        }
    else
        npm install --silent
    fi
    npm run build
    cd ..
    rm -rf internal/web/static
    mkdir -p internal/web/static
    cp -r web/dist/* internal/web/static/
    echo "    前端已构建并嵌入"
else
    echo "    未找到 web 目录，跳过前端构建"
fi

# ── 构建后端 ────────────────────────────────────────
echo ">>> 编译 linux/amd64 二进制..."
mkdir -p "$BUILD_DIR"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o "${BUILD_DIR}/${BINARY}" ./cmd/yunxi-home/
echo "    ${BUILD_DIR}/${BINARY}  ($(du -h "${BUILD_DIR}/${BINARY}" | cut -f1))"

# ── Dry-run 模式 ────────────────────────────────────
if $DRY_RUN; then
    echo ""
    echo ">>> Dry-run 完成，未上传到服务器"
    exit 0
fi

# ── SSH 连接检查 ────────────────────────────────────
echo ">>> 检查 SSH 连接..."
if ! ssh -o ConnectTimeout=5 "$SSH_HOST" "echo ok" &>/dev/null; then
    echo "无法连接到 $SSH_HOST，请检查 SSH 配置"
    exit 1
fi
echo "    SSH 连接正常"

# ── 上传 ────────────────────────────────────────────
echo ">>> 上传二进制到 ${SSH_HOST}..."
scp "${BUILD_DIR}/${BINARY}" "${SSH_HOST}:${REMOTE_DIR}/${BINARY}.new"
echo "    上传完成"

# ── 远程部署 ────────────────────────────────────────
echo ">>> 远程部署..."
ssh "$SSH_HOST" <<'SSH_DEPLOY'
    set -e

    REMOTE_DIR="/opt/yunxi-home"
    BINARY="yunxi-home"
    SERVICE_NAME="yunxi-home"
    KEEP_RELEASES=3

    # 备份旧版本
    if [ -f "${REMOTE_DIR}/${BINARY}" ]; then
        backup_name="${REMOTE_DIR}/${BINARY}.$(date +%Y%m%d%H%M%S)"
        cp "${REMOTE_DIR}/${BINARY}" "$backup_name"
        echo "    已备份: $backup_name"
    fi

    # 替换二进制
    mv "${REMOTE_DIR}/${BINARY}.new" "${REMOTE_DIR}/${BINARY}"
    chmod +x "${REMOTE_DIR}/${BINARY}"
    echo "    二进制已替换"

    # 重启服务
    sudo systemctl restart ${SERVICE_NAME}
    echo "    服务已重启"

    # 清理旧备份（保留最近 N 个）
    backups=($(ls -1t ${REMOTE_DIR}/${BINARY}.* 2>/dev/null || true))
    if [ ${#backups[@]} -gt ${KEEP_RELEASES} ]; then
        for old in "${backups[@]:${KEEP_RELEASES}}"; do
            rm -f "$old"
            echo "    清理旧备份: $old"
        done
    fi

    # 检查服务状态
    sleep 2
    sudo systemctl is-active --quiet ${SERVICE_NAME} && echo "    ✓ 服务运行正常" || echo "    ✗ 服务可能未正常启动，请检查"
SSH_DEPLOY

echo ""
echo "=========================================="
echo "  部署完成: ${SSH_HOST}"
echo "=========================================="
