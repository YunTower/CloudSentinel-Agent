#!/bin/bash

set -e

GOOS=windows
GOARCH=amd64
CGO_ENABLED=0

echo "Building agent for Windows AMD64..."
go build -o agent.exe ./cmd/agent/main.go

# 生成 SHA256 文件
if [ -f agent.exe ]; then
    SHA256_FILE="agent-windows-amd64.sha256"
    sha256sum agent.exe | awk '{print $1}' > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
    echo "SHA256: $(cat $SHA256_FILE)"
else
    echo "Error: agent.exe file not found, cannot generate SHA256"
    exit 1
fi

