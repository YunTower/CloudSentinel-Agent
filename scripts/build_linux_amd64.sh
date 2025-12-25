#!/bin/bash

set -e

cd "$(dirname "$0")/.."

GOOS=linux
GOARCH=amd64
CGO_ENABLED=0

echo "Building agent for Linux AMD64..."
BINARY_NAME="agent-linux-amd64"
go build -o "$BINARY_NAME" ./cmd/agent/main.go

# 生成 SHA256 文件
if [ -f "$BINARY_NAME" ]; then
    SHA256_FILE="agent-linux-amd64.sha256"
    sha256sum "$BINARY_NAME" | awk '{print $1}' > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
    echo "SHA256: $(cat $SHA256_FILE)"
    echo "Binary file: $BINARY_NAME"
else
    echo "Error: $BINARY_NAME file not found, cannot generate SHA256"
    exit 1
fi

