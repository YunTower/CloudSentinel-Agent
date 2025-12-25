@echo off
cd /d "%~dp0.."

echo Building agent for Linux ARM64...

set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
set BINARY_NAME=agent-linux-arm64
go build -o %BINARY_NAME% ./cmd/agent/main.go

if not exist %BINARY_NAME% (
    echo Error: %BINARY_NAME% file not found, build failed
    exit /b 1
)
echo Binary file created: %BINARY_NAME%

REM Package as tar.gz
set TAR_GZ_NAME=agent-linux-arm64.tar.gz
if exist %TAR_GZ_NAME% (
    del %TAR_GZ_NAME%
)
REM Use -C . to specify current directory, ensuring only the file itself is packaged without path
tar -czf %TAR_GZ_NAME% -C . %BINARY_NAME%
if errorlevel 1 (
    echo Error: Failed to create tar.gz file
    exit /b 1
)
echo Archive created: %TAR_GZ_NAME%

REM Generate SHA256 file (for tar.gz file)
set SHA256_FILE=agent-linux-arm64.sha256
powershell -Command "$hash = (Get-FileHash -Path '%TAR_GZ_NAME%' -Algorithm SHA256).Hash; Set-Content -Path '%SHA256_FILE%' -Value $hash"
if errorlevel 1 (
    echo Error: Failed to generate SHA256 file
    exit /b 1
)
echo SHA256 file generated: %SHA256_FILE%

echo Build completed successfully!
echo Files created:
echo   - %BINARY_NAME%
echo   - %TAR_GZ_NAME%
echo   - %SHA256_FILE%

