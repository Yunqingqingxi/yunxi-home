# Yunxi Home Windows Uninstall Script
# Run as Administrator: powershell -ExecutionPolicy Bypass -File uninstall.ps1

param(
    [string]$InstallDir = "$env:ProgramFiles\yunxi-home",
    [switch]$RemoveData
)

$ErrorActionPreference = "Stop"
$serviceName = "yunxi-home"

Write-Host "=== Yunxi Home Windows Uninstaller ===" -ForegroundColor Cyan

# 1. Stop and remove service
Write-Host "[1/3] Removing Windows Service..." -ForegroundColor Yellow
$svc = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($svc) {
    Stop-Service $serviceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    sc.exe delete $serviceName | Out-Null
    Write-Host "  Service removed"
} else {
    Write-Host "  Service not found, skipping"
}

# 2. Remove install directory
Write-Host "[2/3] Removing files..." -ForegroundColor Yellow
if (Test-Path $InstallDir) {
    Remove-Item -Recurse -Force $InstallDir
    Write-Host "  Removed: $InstallDir"
} else {
    Write-Host "  Install dir not found, skipping"
}

# 3. Done
Write-Host "[3/3] Uninstall complete" -ForegroundColor Green
if ($RemoveData) {
    Write-Host "  (data directory was removed with install dir)"
} else {
    Write-Host "  (use -RemoveData to delete all data)"
}
