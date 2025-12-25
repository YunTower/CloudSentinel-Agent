set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -o agent ./cmd/agent/main.go

REM 生成 SHA256 文件
if exist agent (
    set SHA256_FILE=agent-linux-arm64.sha256
    powershell -Command "$hash = (Get-FileHash -Path agent -Algorithm SHA256).Hash; Set-Content -Path 'agent-linux-arm64.sha256' -Value $hash"
    echo SHA256 file generated: %SHA256_FILE%
) else (
    echo Error: agent file not found, cannot generate SHA256
    exit /b 1
)

