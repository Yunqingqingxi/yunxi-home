# Yunxi Home Windows Install Script
# Run as Administrator: powershell -ExecutionPolicy Bypass -File install.ps1

param(
    [string]$InstallDir = "$env:ProgramFiles\yunxi-home",
    [string]$BinaryPath = ".\build\yunxi-home.exe",
    [string]$ConfigPath = ".\configs\config.yaml",
    [switch]$SkipService
)

$ErrorActionPreference = "Stop"
Write-Host "=== Yunxi Home Windows Installer ===" -ForegroundColor Cyan

# 1. Create directories
Write-Host "[1/5] Creating directories..." -ForegroundColor Yellow
$dirs = @(
    $InstallDir,
    "$InstallDir\data",
    "$InstallDir\log",
    "$InstallDir\configs"
)
foreach ($d in $dirs) {
    if (-not (Test-Path $d)) {
        New-Item -ItemType Directory -Path $d -Force | Out-Null
        Write-Host "  Created: $d"
    } else {
        Write-Host "  Exists: $d"
    }
}

# 2. Copy binary
Write-Host "[2/5] Installing binary..." -ForegroundColor Yellow
if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found at $BinaryPath. Build first: make.bat build"
    exit 1
}
Copy-Item $BinaryPath "$InstallDir\yunxi-home.exe" -Force
Write-Host "  Copied to $InstallDir\yunxi-home.exe"

# 3. Copy config
Write-Host "[3/5] Installing config..." -ForegroundColor Yellow
if (Test-Path $ConfigPath) {
    Copy-Item $ConfigPath "$InstallDir\configs\config.yaml" -Force
    Write-Host "  Config copied"
} else {
    Write-Host "  No config found, skipping (create manually at $InstallDir\configs\config.yaml)"
}

# 4. Register Windows Service
if (-not $SkipService) {
    Write-Host "[4/5] Registering Windows Service..." -ForegroundColor Yellow
    $serviceName = "yunxi-home"
    $existing = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "  Service already exists, stopping..."
        Stop-Service $serviceName -Force -ErrorAction SilentlyContinue
        sc.exe delete $serviceName | Out-Null
        Start-Sleep -Seconds 2
    }
    $binPath = "`"$InstallDir\yunxi-home.exe`" -config `"$InstallDir\configs\config.yaml`""
    sc.exe create $serviceName binPath= $binPath start= auto DisplayName= "Yunxi Home Server" | Out-Null
    sc.exe description $serviceName "Yunxi Home - All-in-one home server management" | Out-Null
    sc.exe failure $serviceName reset= 86400 actions= restart/5000/restart/10000/restart/30000 | Out-Null
    Write-Host "  Service '$serviceName' registered (auto-start, auto-restart on failure)"

    # 5. Start service
    Write-Host "[5/5] Starting service..." -ForegroundColor Yellow
    Start-Service $serviceName
    Start-Sleep -Seconds 3
    $svc = Get-Service $serviceName -ErrorAction SilentlyContinue
    if ($svc.Status -eq 'Running') {
        Write-Host "  Service is RUNNING" -ForegroundColor Green
    } else {
        Write-Host "  Service status: $($svc.Status) — check logs at $InstallDir\log\" -ForegroundColor Red
    }
} else {
    Write-Host "[4/5] Skipping service registration (--SkipService)" -ForegroundColor Yellow
    Write-Host "[5/5] Done" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "  Install dir : $InstallDir"
Write-Host "  Data dir    : $InstallDir\data"
Write-Host "  Log dir     : $InstallDir\log"
Write-Host "  Config      : $InstallDir\configs\config.yaml"
Write-Host "  Service     : yunxi-home"
Write-Host ""
Write-Host "Manage service:"
Write-Host "  Start  : Start-Service yunxi-home"
Write-Host "  Stop   : Stop-Service yunxi-home"
Write-Host "  Status : Get-Service yunxi-home"
Write-Host "  Logs   : Get-Content $InstallDir\log\*.log -Tail 50"
