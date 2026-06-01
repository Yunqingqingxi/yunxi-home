@echo off
REM Yunxi Home — Windows 快捷命令 (替代 Makefile)
REM 用法: make.bat [deploy|deploy-dry|deploy-rollback|build|release|test|clean]

setlocal

if "%1"=="" goto :usage
goto :%1 2>nul || goto :unknown

:deploy
    echo === 部署到 Linux 服务器 ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\deploy.ps1"
    goto :eof

:deploy-dry
    echo === 部署 (Dry-Run) ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\deploy.ps1" -DryRun
    goto :eof

:deploy-rollback
    echo === 回滚 ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\deploy.ps1" -Rollback
    goto :eof

:deploy-win
    echo === 部署到本机 Windows ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\deploy.ps1" -TargetOS windows
    goto :eof

:deploy-win-remote
    echo === 部署到远程 Windows (需指定计算机名) ===
    if "%2"=="" (echo 用法: make.bat deploy-win-remote COMPUTER_NAME && goto :eof)
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\deploy.ps1" -TargetOS windows -ComputerName "%2"
    goto :eof

:build
    echo === 构建 (本地) ===
    cd /d "%~dp0web"
    call npm run build
    cd /d "%~dp0"
    if exist "internal\web\static\*" rmdir /s /q "internal\web\static"
    mkdir "internal\web\static"
    xcopy /e /y "web\dist\*" "internal\web\static\"
    if not exist "build" mkdir build
    set CGO_ENABLED=0
    go build -ldflags="-s -w" -o build\yunxi-home.exe .\cmd\yunxi-home\
    echo 构建完成: build\yunxi-home.exe
    goto :eof

:build-all
    echo === 跨平台编译 ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\build.ps1"
    goto :eof

:release
    echo === 发布打包 (Linux + Windows) ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\release.ps1"
    echo.
    echo 发布文件在 release\ 目录下:
    dir /b release\*.zip release\*.tar.gz 2>nul
    goto :eof

:test
    echo === 运行测试 ===
    go test -v -race -coverprofile=coverage.out .\...
    goto :eof

:install
    echo === 安装 Windows 服务 ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\install.ps1"
    goto :eof

:uninstall
    echo === 卸载 Windows 服务 ===
    powershell -ExecutionPolicy Bypass -File "%~dp0scripts\uninstall.ps1"
    goto :eof

:clean
    echo === 清理 ===
    if exist "build" rmdir /s /q "build"
    if exist "release" rmdir /s /q "release"
    if exist "internal\web\static" rmdir /s /q "internal\web\static"
    if exist "coverage.out" del "coverage.out"
    echo 清理完成
    goto :eof

:deploy-bash
    echo === 部署到服务器 (Bash) ===
    bash scripts\deploy.sh
    goto :eof

:usage
    echo Yunxi Home — Windows 构建工具
    echo.
    echo 用法: make.bat [目标]
    echo.
    echo 部署:
    echo   deploy           部署到服务器 (PowerShell)
    echo   deploy-dry       构建但不部署 (PowerShell)
    echo   deploy-rollback  回滚到之前版本 (PowerShell)
    echo   deploy-bash      部署到服务器 (Bash)
    echo.
    echo 构建:
    echo   build            本地构建 (Windows)
    echo   build-all        跨平台编译
    echo   release          发布打包 (Linux + Windows)
    echo.
    echo 其他:
    echo   test             运行测试
    echo   clean            清理构建产物
    goto :eof

:unknown
    echo 未知目标: %1
    echo 运行 make.bat 查看可用目标
    exit /b 1
