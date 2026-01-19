#!/bin/bash

# CI/CD 构建脚本
# 用法: ./build_ci.sh <version> <platform> <arch>
# 示例: ./build_ci.sh 1.0.0 linux amd64

set -e

VERSION="$1"
PLATFORM="$2"
ARCH="$3"

if [ -z "$VERSION" ] || [ -z "$PLATFORM" ] || [ -z "$ARCH" ]; then
    echo "Usage: $0 <version> <platform> <arch>"
    echo "Example: $0 1.0.0 linux amd64"
    exit 1
fi

echo "=========================================="
echo "CI/CD Build Script"
echo "=========================================="
echo "Version: $VERSION"
echo "Platform: $PLATFORM"
echo "Architecture: $ARCH"
echo "=========================================="

# 获取脚本所在目录（agent 根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_DIR"

echo "Agent directory: $SCRIPT_DIR"

# 更新版本号
echo ""
echo "Updating version in source files..."

# 更新 cmd/cli/root.go 中的版本号
if [ -f "cmd/cli/root.go" ]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/Version: \".*\"/Version: \"$VERSION\"/" cmd/cli/root.go
    else
        # Linux
        sed -i "s/Version: \".*\"/Version: \"$VERSION\"/" cmd/cli/root.go
    fi
    echo "Version updated in cmd/cli/root.go: $VERSION"
fi

# 更新 internal/collector/collector.go 中的版本号
if [ -f "internal/collector/collector.go" ]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/const agentVersion = \".*\"/const agentVersion = \"$VERSION-release\"/" internal/collector/collector.go
    else
        # Linux
        sed -i "s/const agentVersion = \".*\"/const agentVersion = \"$VERSION-release\"/" internal/collector/collector.go
    fi
    echo "Version updated in internal/collector/collector.go: $VERSION-release"
fi

# 设置构建环境变量
export GOOS="$PLATFORM"
export GOARCH="$ARCH"
export CGO_ENABLED=0

# 确定输出文件名
if [ "$PLATFORM" = "windows" ]; then
    BINARY_NAME="agent-${PLATFORM}-${ARCH}.exe"
    OUTPUT_NAME="agent.exe"
    ARCHIVE_NAME="agent-${PLATFORM}-${ARCH}.zip"
    ARCHIVE_CMD="zip"
else
    BINARY_NAME="agent-${PLATFORM}-${ARCH}"
    OUTPUT_NAME="agent"
    ARCHIVE_NAME="agent-${PLATFORM}-${ARCH}.tar.gz"
    ARCHIVE_CMD="tar -czf"
fi

# 构建
echo ""
echo "=========================================="
echo "Building agent"
echo "=========================================="
echo "GOOS=$GOOS"
echo "GOARCH=$GOARCH"
echo "CGO_ENABLED=$CGO_ENABLED"
echo "Command: go build -ldflags \"-s -w\" -trimpath -o $OUTPUT_NAME ./cmd/agent/main.go"
echo ""

# 执行构建
echo "Starting build..."
BUILD_OUTPUT=$(go build -ldflags "-s -w" -trimpath -o "$OUTPUT_NAME" ./cmd/agent/main.go 2>&1)
BUILD_EXIT_CODE=$?

if [ $BUILD_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "=========================================="
    echo "ERROR: Build failed!"
    echo "=========================================="
    echo "$BUILD_OUTPUT"
    echo ""
    exit 1
fi

# 显示构建输出（如果有警告或信息）
if [ -n "$BUILD_OUTPUT" ]; then
    echo "Build output:"
    echo "$BUILD_OUTPUT"
fi

if [ ! -f "$OUTPUT_NAME" ]; then
    echo "Error: Build failed - $OUTPUT_NAME not found"
    exit 1
fi

echo "Build successful!"

# 验证构建结果
BINARY_SIZE=$(stat -f%z "$OUTPUT_NAME" 2>/dev/null || stat -c%s "$OUTPUT_NAME" 2>/dev/null || echo "0")
echo ""
echo "Binary size: $BINARY_SIZE bytes ($(numfmt --to=iec-i --suffix=B $BINARY_SIZE 2>/dev/null || echo "N/A"))"

# 重命名二进制文件
if [ -f "$BINARY_NAME" ]; then
    rm "$BINARY_NAME"
fi
mv "$OUTPUT_NAME" "$BINARY_NAME"
echo "Binary renamed to: $BINARY_NAME"

# 打包
echo ""
echo "Creating archive..."
if [ -f "$ARCHIVE_NAME" ]; then
    rm "$ARCHIVE_NAME"
fi

if [ "$PLATFORM" = "windows" ]; then
    if command -v zip &> /dev/null; then
        zip "$ARCHIVE_NAME" "$BINARY_NAME"
    elif command -v 7z &> /dev/null; then
        7z a "$ARCHIVE_NAME" "$BINARY_NAME"
    else
        echo "Error: zip or 7z command not found"
        exit 1
    fi
else
    tar -czf "$ARCHIVE_NAME" -C . "$BINARY_NAME"
fi

if [ ! -f "$ARCHIVE_NAME" ]; then
    echo "Error: Failed to create archive"
    exit 1
fi

echo "Archive created: $ARCHIVE_NAME"

# 生成 SHA256
echo ""
echo "Generating SHA256 checksum..."
SHA256_FILE="agent-${PLATFORM}-${ARCH}.sha256"

if command -v sha256sum &> /dev/null; then
    sha256sum "$ARCHIVE_NAME" > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
elif command -v shasum &> /dev/null; then
    shasum -a 256 "$ARCHIVE_NAME" > "$SHA256_FILE"
    echo "SHA256 file generated: $SHA256_FILE"
else
    echo "Error: sha256sum or shasum not found, cannot generate checksum"
    exit 1
fi

if [ ! -f "$SHA256_FILE" ]; then
    echo "Error: Failed to generate SHA256 file"
    exit 1
fi

# 显示 SHA256 内容用于验证
echo "SHA256 checksum:"
cat "$SHA256_FILE"

echo ""
echo "=========================================="
echo "Build completed successfully!"
echo "=========================================="
echo "Files created:"
echo "  - $BINARY_NAME"
echo "  - $ARCHIVE_NAME"
if [ -f "$SHA256_FILE" ]; then
    echo "  - $SHA256_FILE"
fi
echo "=========================================="
