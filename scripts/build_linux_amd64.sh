#!/bin/bash

set -e

GOOS=linux
GOARCH=amd64
CGO_ENABLED=0

echo "Building agent for Linux AMD64..."
go build -o agent ./cmd/agent/main.go

# 生成 SHA256 文件
if [ -f agent ]; then
    SHA256_FILE="agent-linux-amd64.sha256"
    sha256sum agent | awk '{print $1}' > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
    echo "SHA256: $(cat $SHA256_FILE)"
else
    echo "Error: agent file not found, cannot generate SHA256"
    exit 1
fi

