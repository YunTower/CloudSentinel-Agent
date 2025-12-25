@echo off
cd /d "%~dp0.."

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
set BINARY_NAME=agent-windows-amd64.exe
go build -o %BINARY_NAME% ./cmd/agent/main.go

REM 生成 SHA256 文件
if exist %BINARY_NAME% (
    set SHA256_FILE=agent-windows-amd64.sha256
    powershell -Command "$hash = (Get-FileHash -Path %BINARY_NAME% -Algorithm SHA256).Hash; Set-Content -Path 'agent-windows-amd64.sha256' -Value $hash"
    echo SHA256 file generated: %SHA256_FILE%
    echo Binary file: %BINARY_NAME%
) else (
    echo Error: %BINARY_NAME% file not found, cannot generate SHA256
    exit /b 1
)

