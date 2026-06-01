#!/bin/bash
# Yunxi Home — 首次安装脚本（在 Linux 服务器上以 root 运行）
set -e

INSTALL_DIR="/opt/yunxi-home"
SERVICE_NAME="yunxi-home"

echo "=== Yunxi Home 安装 ==="

# 1. 创建 yunxi 系统用户和组
if ! getent group yunxi >/dev/null; then
    groupadd --system yunxi
    echo "[✓] 创建 yunxi 用户组"
else
    echo "[✓] yunxi 用户组已存在"
fi

if ! id -u yunxi >/dev/null 2>&1; then
    useradd --system -g yunxi -s /usr/sbin/nologin -d "$INSTALL_DIR" yunxi
    echo "[✓] 创建 yunxi 系统用户"
else
    echo "[✓] yunxi 用户已存在"
fi

# 2. sudo 免密权限
echo "yunxi ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/yunxi
chmod 440 /etc/sudoers.d/yunxi
echo "[✓] yunxi sudo 免密权限已配置"

# 3. 创建必要目录并设置权限
mkdir -p "$INSTALL_DIR/data" "$INSTALL_DIR/log" "$INSTALL_DIR/data/yunxiFiles"
chown -R yunxi:yunxi "$INSTALL_DIR"
chmod 750 "$INSTALL_DIR"
chmod -R 770 "$INSTALL_DIR/data" "$INSTALL_DIR/log"
echo "[✓] 目录权限已设置"

# 3. 安装 systemd 服务
if [ -f "deploy/yunxi-home.service" ]; then
    cp deploy/yunxi-home.service /etc/systemd/system/
    systemctl daemon-reload
    echo "[✓] systemd 服务已安装"
fi

# 4. 创建首次设置标记
touch "$INSTALL_DIR/data/.needs_setup"
chown yunxi:yunxi "$INSTALL_DIR/data/.needs_setup"
echo "[✓] 首次设置标记已创建"

echo ""
echo "=== 安装完成 ==="
echo "启动服务: sudo systemctl start $SERVICE_NAME"
echo "开机自启: sudo systemctl enable $SERVICE_NAME"
echo "查看状态: sudo systemctl status $SERVICE_NAME"
