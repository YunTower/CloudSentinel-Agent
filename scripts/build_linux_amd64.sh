#!/bin/bash

set -e

cd "$(dirname "$0")/.."

echo "Building agent for Linux AMD64..."

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

BINARY_NAME="agent-linux-amd64"
go build -o "$BINARY_NAME" ./cmd/agent/main.go

if [ ! -f "$BINARY_NAME" ]; then
    echo "Error: $BINARY_NAME file not found, build failed"
    exit 1
fi
echo "Binary file created: $BINARY_NAME"

# Package as tar.gz
TAR_GZ_NAME="agent-linux-amd64.tar.gz"
if [ -f "$TAR_GZ_NAME" ]; then
    rm "$TAR_GZ_NAME"
fi
tar -czf "$TAR_GZ_NAME" -C . "$BINARY_NAME"
if [ $? -ne 0 ]; then
    echo "Error: Failed to create tar.gz file"
    exit 1
fi
echo "Archive created: $TAR_GZ_NAME"

# Generate SHA256 file (for tar.gz file)
SHA256_FILE="agent-linux-amd64.sha256"
sha256sum "$TAR_GZ_NAME" | awk '{print $1}' > "$SHA256_FILE"
if [ $? -ne 0 ]; then
    echo "Error: Failed to generate SHA256 file"
    exit 1
fi
echo "SHA256 file generated: $SHA256_FILE"

echo "Build completed successfully!"
echo "Files created:"
echo "  - $BINARY_NAME"
echo "  - $TAR_GZ_NAME"
echo "  - $SHA256_FILE"

