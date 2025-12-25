set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -o agent.exe ./cmd/agent/main.go

REM 生成 SHA256 文件
if exist agent.exe (
    set SHA256_FILE=agent-windows-amd64.sha256
    powershell -Command "$hash = (Get-FileHash -Path agent.exe -Algorithm SHA256).Hash; Set-Content -Path 'agent-windows-amd64.sha256' -Value $hash"
    echo SHA256 file generated: %SHA256_FILE%
) else (
    echo Error: agent.exe file not found, cannot generate SHA256
    exit /b 1
)

