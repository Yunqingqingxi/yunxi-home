# Yunxi Home — Windows PowerShell 跨平台编译
# 用法: .\scripts\build.ps1 [-Version "3.0.0"]
param(
    [string]$Version = "3.0.0"
)

$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot\.."

$BUILD_TIME  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-dd_HH:mm:ss")
$GIT_COMMIT  = try { git rev-parse --short HEAD 2>$null } catch { "unknown" }
$APP_NAME    = "yunxi-home"
$OUTPUT_DIR  = "build"

$LDFLAGS = "-s -w -X 'main.Version=${Version}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

$TARGETS = @(
    @{OS = "linux";   Arch = "amd64"; Arm = ""},
    @{OS = "linux";   Arch = "arm64"; Arm = ""},
    @{OS = "linux";   Arch = "arm";   Arm = "7"},
    @{OS = "windows"; Arch = "amd64"; Arm = ""},
    @{OS = "darwin";  Arch = "amd64"; Arm = ""},
    @{OS = "darwin";  Arch = "arm64"; Arm = ""}
)

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "  Yunxi Home 跨平台编译" -ForegroundColor Cyan
Write-Host "  Version: ${Version}" -ForegroundColor Cyan
Write-Host "  Build  : ${BUILD_TIME}" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

if (-not (Test-Path $OUTPUT_DIR)) {
    New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null
}

foreach ($target in $TARGETS) {
    $GOOS   = $target.OS
    $GOARCH = $target.Arch
    $GOARM  = $target.Arm

    if ($GOOS -eq "windows") {
        $OUTPUT = "${OUTPUT_DIR}\${APP_NAME}-${GOOS}-${GOARCH}.exe"
    }
    else {
        $OUTPUT = "${OUTPUT_DIR}\${APP_NAME}-${GOOS}-${GOARCH}"
    }

    $armStr = if ($GOARM) { " v${GOARM}" } else { "" }
    Write-Host "编译: ${GOOS}/${GOARCH}${armStr} ..." -ForegroundColor Yellow

    $env:CGO_ENABLED = "0"
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    if ($GOARM) { $env:GOARM = $GOARM }

    go build -ldflags $LDFLAGS -o $OUTPUT .\cmd\yunxi-home\
    if ($LASTEXITCODE -ne 0) { throw "编译失败: ${GOOS}/${GOARCH}" }

    $size = (Get-Item $OUTPUT).Length
    $sizeStr = if ($size -gt 1MB) { "{0:N1} MB" -f ($size / 1MB) } else { "{0:N0} KB" -f ($size / 1KB) }
    Write-Host "  -> ${OUTPUT} (${sizeStr})" -ForegroundColor Green
}

Write-Host ""
Write-Host "所有平台编译完成！" -ForegroundColor Green
Write-Host "输出目录: ${OUTPUT_DIR}\"
Get-ChildItem $OUTPUT_DIR | Format-Table Name, @{N="Size";E={if($_.Length -gt 1MB){"{0:N1}MB" -f ($_.Length/1MB)}else{"{0:N0}KB" -f ($_.Length/1KB)}}}
