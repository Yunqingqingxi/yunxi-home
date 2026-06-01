# Yunxi Home — Windows PowerShell 部署脚本
# 用法:
#   Linux 部署:   .\scripts\deploy.ps1 [-DryRun] [-Rollback]
#   Windows 部署: .\scripts\deploy.ps1 -TargetOS windows [-ComputerName SERVER] [-DryRun]
param(
    [switch]$DryRun,
    [switch]$Rollback,
    [ValidateSet("linux", "windows")]
    [string]$TargetOS = "linux",
    [string]$ComputerName = "",    # Windows 远程部署目标
    [string]$WindowsInstallDir = "C:\Program Files\yunxi-home"
)

$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

# ── 配置 ────────────────────────────────────────────
$SSH_HOST      = if ($env:YUNXI_SSH_HOST) { $env:YUNXI_SSH_HOST } else { throw "错误: SSH_HOST 为空。请设置 YUNXI_SSH_HOST 环境变量。" }
$REMOTE_DIR    = if ($env:YUNXI_REMOTE_DIR) { $env:YUNXI_REMOTE_DIR } else { "/opt/yunxi-home" }
$SERVICE_NAME  = "yunxi-home"
$BINARY        = if ($TargetOS -eq "windows") { "yunxi-home.exe" } else { "yunxi-home" }
$KEEP_RELEASES = 3
$BUILD_DIR     = "build"

# ── 回滚模式 ────────────────────────────────────────
if ($Rollback) {
    Write-Host ">>> 列出服务器上的备份版本..." -ForegroundColor Cyan
    $backups = ssh $SSH_HOST "ls -1t ${REMOTE_DIR}/${BINARY}.* 2>/dev/null"
    if (-not $backups) {
        Write-Host "没有找到备份文件" -ForegroundColor Red
        exit 1
    }

    $backupArray = $backups -split "`n"
    for ($i = 0; $i -lt $backupArray.Count; $i++) {
        Write-Host "  [$($i + 1)] $($backupArray[$i])"
    }

    $choice = Read-Host "输入要恢复的编号 (回车取消)"
    if (-not $choice) {
        Write-Host "已取消"
        exit 0
    }

    $idx = [int]$choice - 1
    if ($idx -lt 0 -or $idx -ge $backupArray.Count) {
        Write-Host "无效的编号" -ForegroundColor Red
        exit 1
    }

    $backup = $backupArray[$idx].Trim()
    Write-Host ">>> 恢复: $backup" -ForegroundColor Yellow
    ssh $SSH_HOST @"
        set -e
        sudo systemctl stop ${SERVICE_NAME}
        cp "$backup" "${REMOTE_DIR}/${BINARY}"
        sudo systemctl start ${SERVICE_NAME}
        echo "已恢复并重启服务"
"@
    Write-Host ">>> 回滚完成" -ForegroundColor Green
    exit 0
}

# ── 构建前端 ────────────────────────────────────────
Write-Host ">>> 构建前端..." -ForegroundColor Cyan
if ((Test-Path "web") -and (Test-Path "web\package.json")) {
    Push-Location web
    try {
        # 如果 node_modules 不存在，先 install；否则用 ci
        if (-not (Test-Path "node_modules")) {
            npm install --silent 2>$null
        }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    }
    finally {
        Pop-Location
    }

    # 复制到嵌入目录
    if (Test-Path "internal\web\static") {
        Remove-Item -Recurse -Force "internal\web\static\*"
    }
    else {
        New-Item -ItemType Directory -Force -Path "internal\web\static" | Out-Null
    }
    Copy-Item -Recurse -Force "web\dist\*" "internal\web\static\"
    Write-Host "    前端已构建并嵌入" -ForegroundColor Green
}
else {
    Write-Host "    未找到 web 目录，跳过前端构建" -ForegroundColor Yellow
}

# ── 构建后端 ────────────────────────────────────────
Write-Host (">>> 编译 " + $TargetOS + "/amd64 二进制...") -ForegroundColor Cyan
if (-not (Test-Path $BUILD_DIR)) {
    New-Item -ItemType Directory -Force -Path $BUILD_DIR | Out-Null
}

$env:CGO_ENABLED = "0"
$env:GOOS = $TargetOS
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o "$BUILD_DIR\$BINARY" .\cmd\yunxi-home\
if ($LASTEXITCODE -ne 0) { throw "Go 编译失败" }

$size = (Get-Item "$BUILD_DIR\$BINARY").Length
$sizeMB = "{0:N1}" -f ($size / 1MB)
Write-Host ("    " + $BUILD_DIR + "\" + $BINARY + "  (" + $sizeMB + " MB)") -ForegroundColor Green

# ── Dry-run 模式 ────────────────────────────────────
if ($DryRun) {
    Write-Host ""
    Write-Host ">>> Dry-run 完成，未部署到目标" -ForegroundColor Yellow
    exit 0
}

# ── 分支：Linux 部署 vs Windows 部署 ────────────────

if ($TargetOS -eq "windows") {
    # ====================================================
    #  Windows 部署
    # ====================================================
    $TargetComputer = if ($ComputerName) { $ComputerName } else { $env:COMPUTERNAME }
    $targetDir = $WindowsInstallDir

    Write-Host (">>> 部署到 Windows: " + $TargetComputer) -ForegroundColor Cyan

    if ($TargetComputer -eq $env:COMPUTERNAME) {
        # ── 本地部署 ──
        Write-Host "    本地部署模式" -ForegroundColor Yellow

        # 创建目标目录
        if (-not (Test-Path $targetDir)) {
            New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
            New-Item -ItemType Directory -Force -Path "$targetDir\data" | Out-Null
            New-Item -ItemType Directory -Force -Path "$targetDir\log" | Out-Null
            New-Item -ItemType Directory -Force -Path "$targetDir\configs" | Out-Null
        }

        # 停止服务
        Write-Host "    停止服务..."
        Stop-Service $SERVICE_NAME -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2

        # 备份旧版本
        $destBinary = "$targetDir\$BINARY"
        if (Test-Path $destBinary) {
            $backupName = $targetDir + "\" + $BINARY + "." + (Get-Date -Format 'yyyyMMddHHmmss')
            Copy-Item $destBinary $backupName
            Write-Host "    已备份: $backupName" -ForegroundColor Green
        }

        # 复制新二进制
        Copy-Item "$BUILD_DIR\$BINARY" $destBinary -Force
        Write-Host "    二进制已复制到 $destBinary" -ForegroundColor Green

        # 复制配置（如果存在且目标没有）
        if ((Test-Path "configs\config.yaml") -and -not (Test-Path "$targetDir\configs\config.yaml")) {
            Copy-Item "configs\config.yaml" "$targetDir\configs\config.yaml"
            Write-Host "    配置文件已复制" -ForegroundColor Green
        }

        # 清理旧备份
        $backups = Get-ChildItem ($targetDir + "\" + $BINARY + ".*") -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending
        if ($backups.Count -gt $KEEP_RELEASES) {
            $backups | Select-Object -Skip $KEEP_RELEASES | Remove-Item -Force
            Write-Host "    清理了 $($backups.Count - $KEEP_RELEASES) 个旧备份" -ForegroundColor Green
        }

        # 启动服务
        Write-Host "    启动服务..."
        Start-Service $SERVICE_NAME -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 3

        $svc = Get-Service $SERVICE_NAME -ErrorAction SilentlyContinue
        if ($svc.Status -eq 'Running') {
            Write-Host "    ✓ 服务运行正常" -ForegroundColor Green
        } else {
            Write-Host "    ✗ 服务状态: $($svc.Status)" -ForegroundColor Red
        }
    }
    else {
        # ── 远程部署（WinRM / PowerShell Remoting）──
        Write-Host "    远程部署模式 (WinRM)" -ForegroundColor Yellow

        # 测试连接
        $testResult = Test-WSMan -ComputerName $TargetComputer -ErrorAction SilentlyContinue
        if (-not $testResult) {
            Write-Host "    无法连接到 $TargetComputer，尝试启用 PSRemoting..." -ForegroundColor Yellow
            Write-Host "    请在目标机器上以管理员运行: Enable-PSRemoting -Force" -ForegroundColor Yellow
            Write-Host "    回退：尝试通过管理员共享复制..." -ForegroundColor Yellow

            # 通过 \\server\C$ 复制
            $adminShare = "\\$TargetComputer\C`$\Program Files\yunxi-home"
            $driveLetter = $targetDir.Substring(0, 1)
            $restPath = $targetDir.Substring(2)
            $adminShare = "\\$TargetComputer\$driveLetter`$\$restPath"

            if (-not (Test-Path $adminShare)) {
                New-Item -ItemType Directory -Force -Path $adminShare | Out-Null
            }

            Copy-Item "$BUILD_DIR\$BINARY" "$adminShare\$BINARY" -Force
            Write-Host "    二进制已复制到 $adminShare" -ForegroundColor Green

            # 远程重启服务
            Write-Host "    远程重启服务..."
            sc.exe "\\$TargetComputer" stop $SERVICE_NAME 2>$null
            Start-Sleep -Seconds 2
            sc.exe "\\$TargetComputer" start $SERVICE_NAME 2>$null
            Write-Host "    服务重启命令已发送" -ForegroundColor Green
        }
        else {
            # WinRM 可用
            Invoke-Command -ComputerName $TargetComputer -ScriptBlock {
                param($srcBinary, $destDir, $serviceName)

                Stop-Service $serviceName -Force -ErrorAction SilentlyContinue
                Start-Sleep -Seconds 2

                if (Test-Path "$destDir\$srcBinary") {
                    Copy-Item "$destDir\$srcBinary" "$destDir\$srcBinary.$(Get-Date -Format 'yyyyMMddHHmmss')"
                }

                Copy-Item $srcBinary $destDir -Force
                Start-Service $serviceName

                Get-Service $serviceName
            } -ArgumentList "$BUILD_DIR\$BINARY", $targetDir, $SERVICE_NAME
        }
    }

    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host ("  Windows 部署完成: " + $TargetComputer) -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green

} else {
    # ====================================================
    #  Linux 部署 (SSH + systemctl)
    # ====================================================

    # ── SSH 连接检查 ──
    Write-Host ">>> 检查 SSH 连接..." -ForegroundColor Cyan
    $sshTest = ssh -o ConnectTimeout=5 $SSH_HOST "echo ok" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "无法连接到 $SSH_HOST，请检查 SSH 配置" -ForegroundColor Red
        exit 1
    }
    Write-Host "    SSH 连接正常" -ForegroundColor Green

    # ── 上传 ──
    Write-Host (">>> 上传二进制到 " + $SSH_HOST + "...") -ForegroundColor Cyan
    $remotePath = $SSH_HOST + ":" + $REMOTE_DIR + "/" + $BINARY + ".new"
scp "$BUILD_DIR\$BINARY" $remotePath
    if ($LASTEXITCODE -ne 0) { throw "上传失败" }
    Write-Host "    上传完成" -ForegroundColor Green

    # ── 远程部署 ──
    Write-Host ">>> 远程部署..." -ForegroundColor Cyan
    $deployScript = @"
set -e

REMOTE_DIR="${REMOTE_DIR}"
BINARY="${BINARY}"
SERVICE_NAME="${SERVICE_NAME}"
KEEP_RELEASES=${KEEP_RELEASES}

if [ -f "\${REMOTE_DIR}/\${BINARY}" ]; then
    backup_name="\${REMOTE_DIR}/\${BINARY}.\$(date +%Y%m%d%H%M%S)"
    cp "\${REMOTE_DIR}/\${BINARY}" "\$backup_name"
    echo "    已备份: \$backup_name"
fi

mv "\${REMOTE_DIR}/\${BINARY}.new" "\${REMOTE_DIR}/\${BINARY}"
chmod +x "\${REMOTE_DIR}/\${BINARY}"
echo "    二进制已替换"

sudo systemctl restart \${SERVICE_NAME}
echo "    服务已重启"

backups=(\$(ls -1t \${REMOTE_DIR}/\${BINARY}.* 2>/dev/null || true))
if [ \${#backups[@]} -gt \${KEEP_RELEASES} ]; then
    for old in "\${backups[@]:\${KEEP_RELEASES}}"; do
        rm -f "\$old"
        echo "    清理旧备份: \$old"
    done
fi

sleep 2
sudo systemctl is-active --quiet \${SERVICE_NAME} && echo "    ✓ 服务运行正常" || echo "    ✗ 服务可能未正常启动，请检查"
"@

    ssh $SSH_HOST $deployScript
    if ($LASTEXITCODE -ne 0) { throw "远程部署失败" }

    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host ("  部署完成: " + $SSH_HOST) -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
}
