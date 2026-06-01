# Yunxi Home — Windows PowerShell 一键发布打包 (Linux + Windows)
# 用法: .\scripts\release.ps1 [-Version "4.0.0"]
param(
    [string]$Version = "4.0.0"
)

$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

$BUILD_TIME  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd_HH:mm:ss")
$GIT_COMMIT  = try { git rev-parse --short HEAD 2>$null } catch { "unknown" }
$LDFLAGS     = "-s -w -X 'main.Version=${Version}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

$RELEASE_DIR = "release"
$APP_NAME    = "yunxi-home"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Yunxi Home 发布打包" -ForegroundColor Cyan
Write-Host "  Version: ${Version}" -ForegroundColor Cyan
Write-Host "  Build  : ${BUILD_TIME}" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# 清理并创建发布目录
if (Test-Path $RELEASE_DIR) {
    Remove-Item -Recurse -Force $RELEASE_DIR
}
New-Item -ItemType Directory -Force -Path $RELEASE_DIR | Out-Null

# ── 前端构建 ──────────────────────────────────────
Write-Host "→ 构建前端 ..." -ForegroundColor Yellow
Push-Location web
try {
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
Write-Host "  ✓ 前端已构建并嵌入" -ForegroundColor Green

# ── Linux amd64 ────────────────────────────────────
Write-Host "→ linux/amd64 ..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags $LDFLAGS -o "${RELEASE_DIR}\${APP_NAME}" .\cmd\yunxi-home\
if ($LASTEXITCODE -ne 0) { throw "Linux 编译失败" }

$size = (Get-Item "${RELEASE_DIR}\${APP_NAME}").Length
$sizeMB = "{0:N1}" -f ($size / 1MB)
Write-Host "  ✓ ${RELEASE_DIR}\${APP_NAME}  (${sizeMB} MB)" -ForegroundColor Green

# ── Windows amd64 ──────────────────────────────────
Write-Host "→ windows/amd64 ..." -ForegroundColor Yellow
$env:GOOS = "windows"
go build -ldflags $LDFLAGS -o "${RELEASE_DIR}\${APP_NAME}.exe" .\cmd\yunxi-home\
if ($LASTEXITCODE -ne 0) { throw "Windows 编译失败" }

$size = (Get-Item "${RELEASE_DIR}\${APP_NAME}.exe").Length
$sizeMB = "{0:N1}" -f ($size / 1MB)
Write-Host "  ✓ ${RELEASE_DIR}\${APP_NAME}.exe  (${sizeMB} MB)" -ForegroundColor Green

# ── 打包 Windows 发布包 ──────────────────────────────
Write-Host "→ 打包 Windows 发布包..." -ForegroundColor Yellow
$winPkgDir = "${RELEASE_DIR}\yunxi-home-windows-amd64"
if (Test-Path $winPkgDir) { Remove-Item -Recurse -Force $winPkgDir }
New-Item -ItemType Directory -Force -Path $winPkgDir | Out-Null
New-Item -ItemType Directory -Force -Path "$winPkgDir\data" | Out-Null
New-Item -ItemType Directory -Force -Path "$winPkgDir\log" | Out-Null
New-Item -ItemType Directory -Force -Path "$winPkgDir\configs" | Out-Null

# 复制二进制
Copy-Item "${RELEASE_DIR}\${APP_NAME}.exe" "$winPkgDir\" -Force
# 复制安装脚本
Copy-Item "scripts\install.ps1" "$winPkgDir\" -Force
Copy-Item "scripts\uninstall.ps1" "$winPkgDir\" -Force
# 复制示例配置（如果存在）
if (Test-Path "configs\config.yaml") {
    Copy-Item "configs\config.yaml" "$winPkgDir\configs\" -Force
}
# 创建 README
@"
Yunxi Home v${Version} — Windows 安装说明
============================================

快速安装:
  1. 以管理员身份打开 PowerShell
  2. 运行: .\install.ps1
  3. 服务将自动注册并启动

管理服务:
  启动: Start-Service yunxi-home
  停止: Stop-Service yunxi-home
  状态: Get-Service yunxi-home
  日志: Get-Content .\log\*.log -Tail 50

卸载:
  以管理员身份运行: .\uninstall.ps1

目录结构:
  yunxi-home.exe    主程序
  configs\          配置文件目录
  data\             数据目录（数据库等）
  log\              日志目录
"@ | Out-File -FilePath "$winPkgDir\README.txt" -Encoding UTF8

# 创建 zip
$zipPath = "${RELEASE_DIR}\yunxi-home-v${Version}-windows-amd64.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -Path "$winPkgDir\*" -DestinationPath $zipPath -Force
Remove-Item -Recurse -Force $winPkgDir
$zipSize = (Get-Item $zipPath).Length
$zipSizeMB = "{0:N1}" -f ($zipSize / 1MB)
Write-Host "  ✓ ${zipPath} (${zipSizeMB} MB)" -ForegroundColor Green

# ── 打包 Linux 发布包 ──────────────────────────────────
Write-Host "→ 打包 Linux 发布包..." -ForegroundColor Yellow
$linuxPkg = "${RELEASE_DIR}\yunxi-home-v${Version}-linux-amd64.tar.gz"
if (Test-Path $linuxPkg) { Remove-Item $linuxPkg -Force }
$tarDir = "${RELEASE_DIR}\yunxi-home-linux-amd64"
if (Test-Path $tarDir) { Remove-Item -Recurse -Force $tarDir }
New-Item -ItemType Directory -Force -Path $tarDir | Out-Null
New-Item -ItemType Directory -Force -Path "$tarDir\data" | Out-Null
New-Item -ItemType Directory -Force -Path "$tarDir\log" | Out-Null
New-Item -ItemType Directory -Force -Path "$tarDir\configs" | Out-Null

Copy-Item "${RELEASE_DIR}\${APP_NAME}" "$tarDir\" -Force
Copy-Item "scripts\install.sh" "$tarDir\" -Force
if (Test-Path "configs\config.yaml") {
    Copy-Item "configs\config.yaml" "$tarDir\configs\" -Force
}
if (Test-Path "deploy\yunxi-home.service") {
    Copy-Item "deploy\yunxi-home.service" "$tarDir\" -Force
}

# 用 tar 打包
Push-Location $RELEASE_DIR
try {
    tar -czf "yunxi-home-v${Version}-linux-amd64.tar.gz" "yunxi-home-linux-amd64\"
    Remove-Item -Recurse -Force "yunxi-home-linux-amd64"
}
finally { Pop-Location }

$tarSize = (Get-Item $linuxPkg).Length
$tarSizeMB = "{0:N1}" -f ($tarSize / 1MB)
Write-Host "  ✓ ${linuxPkg} (${tarSizeMB} MB)" -ForegroundColor Green

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  打包完成: ${RELEASE_DIR}\" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Get-ChildItem $RELEASE_DIR | Format-Table Name, @{N="Size";E={if($_.Length -gt 1MB){"{0:N1}MB" -f ($_.Length/1MB)}else{"{0:N0}KB" -f ($_.Length/1KB)}}}
