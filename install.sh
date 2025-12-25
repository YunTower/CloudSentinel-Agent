#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${CYAN}ℹ${NC} ${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} ${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} ${YELLOW}$1${NC}"
}

print_error() {
    echo -e "${RED}✗${NC} ${RED}$1${NC}"
}

print_step() {
    echo -e "\n${BOLD}${MAGENTA}▶${NC} ${BOLD}$1${NC}"
}

print_separator() {
    echo -e "${CYAN}────────────────────────────────────────────────────────${NC}"
}

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        print_error "$1 命令未找到，请先安装 $1"
        exit 1
    fi
}

# 检查必要的命令
check_command "curl"
check_command "jq"
check_command "sha256sum"

# 获取系统信息
get_system_info() {
    OS_TYPE="linux"
    ARCH=$(uname -m)

    # 标准化架构名称
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l|arm) ARCH="arm" ;;
        i386|i686) ARCH="386" ;;
        *)
            print_error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac

    print_info "系统类型: ${BOLD}$OS_TYPE-$ARCH${NC}"
}

# 获取最新版本信息
get_latest_version() {
    print_info "正在获取最新版本信息..."

    # 默认且仅使用 GitHub
    API_URL="https://api.github.com/repos/YunTower/CloudSentinel-Agent/releases/latest"
    RESPONSE=$(curl -s -H "Accept: application/vnd.github.v3+json" "$API_URL")
    local curl_exit_code=$?

    if [ $curl_exit_code -ne 0 ]; then
        print_error "获取版本信息失败（网络错误）"
        exit 1
    fi

    # 检查响应是否为空
    if [ -z "$RESPONSE" ]; then
        print_error "API 返回空响应"
        exit 1
    fi

    # 检查响应是否为有效的 JSON
    if ! echo "$RESPONSE" | jq . > /dev/null 2>&1; then
        print_error "API 返回无效的 JSON 响应"
        print_info "响应内容（前200字符）："
        echo "$RESPONSE" | head -c 200
        echo ""
        exit 1
    fi

    # 检查响应是否包含错误消息
    if echo "$RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
        ERROR_MSG=$(echo "$RESPONSE" | jq -r '.message // "未知错误"')
        print_error "API 返回错误: $ERROR_MSG"
        exit 1
    fi

    # 提取版本标签
    TAG_NAME=$(echo "$RESPONSE" | jq -r '.tag_name // empty' 2>/dev/null)
    if [ $? -ne 0 ]; then
        print_error "解析版本标签失败"
        exit 1
    fi

    if [ -z "$TAG_NAME" ] || [ "$TAG_NAME" = "null" ] || [ "$TAG_NAME" = "" ]; then
        print_error "无法获取版本标签"
        print_info "API 响应（前500字符）："
        echo "$RESPONSE" | head -c 500
        echo ""
        exit 1
    fi

    # 移除版本号前的 'v' 前缀
    TAG_NAME=${TAG_NAME#v}

    print_success "最新版本: ${BOLD}$TAG_NAME${NC}"

    # 保存 assets 信息
    ASSETS=$(echo "$RESPONSE" | jq -c '.assets // []' 2>/dev/null)
    if [ $? -ne 0 ]; then
        print_warning "解析 assets 信息失败，将使用空数组"
        ASSETS="[]"
    fi
}

# 查找匹配的二进制包
find_asset() {
    local expected_name=$1
    local asset_name=""
    local download_url=""

    # 检查 ASSETS 是否为空或无效
    if [ -z "$ASSETS" ] || [ "$ASSETS" = "null" ] || [ "$ASSETS" = "[]" ]; then
        return 1
    fi

    # 验证 ASSETS 是否为有效的 JSON
    if ! echo "$ASSETS" | jq . > /dev/null 2>&1; then
        print_error "Assets 数据无效"
        return 1
    fi

    # 遍历 assets 查找匹配的文件
    local assets_array
    assets_array=$(echo "$ASSETS" | jq -c '.[]' 2>/dev/null)
    if [ $? -ne 0 ]; then
        print_error "解析 assets 数组失败"
        return 1
    fi

    while IFS= read -r asset; do
        if [ -z "$asset" ]; then
            continue
        fi

        # 提取文件名
        name=$(echo "$asset" | jq -r '.name // empty' 2>/dev/null)
        if [ $? -ne 0 ] || [ -z "$name" ] || [ "$name" = "null" ]; then
            continue
        fi

        if [ "$name" = "$expected_name" ]; then
            asset_name="$name"
            # GitHub 使用 browser_download_url
            download_url=$(echo "$asset" | jq -r '.browser_download_url // empty' 2>/dev/null)
            
            if [ $? -eq 0 ] && [ -n "$download_url" ] && [ "$download_url" != "null" ] && [ "$download_url" != "" ]; then
                echo "$asset_name|$download_url"
                return 0
            fi
        fi
    done <<< "$assets_array"

    return 1
}

# 下载文件
download_file() {
    local url=$1
    local output=$2
    local description=$3

    print_info "正在下载 $description..."
    if curl -L --progress-bar -o "$output" "$url"; then
        local file_size=$(du -h "$output" | cut -f1)
        print_success "$description 下载完成 (大小: $file_size)"
        return 0
    else
        print_error "$description 下载失败"
        return 1
    fi
}

# 计算文件的 SHA256
calculate_sha256() {
    local file=$1
    sha256sum "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

# 读取 SHA256 文件内容
read_sha256_file() {
    local file=$1
    head -n1 "$file" | awk '{print $1}' | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]'
}

# 校验文件完整性
verify_file() {
    local file=$1
    local sha256_file=$2

    print_info "正在校验文件完整性..."

    # 读取期望的哈希值
    local expected_hash=$(read_sha256_file "$sha256_file")
    # 计算实际的哈希值
    local actual_hash=$(calculate_sha256 "$file")

    # 再次确保去除所有空白字符
    expected_hash=$(printf '%s' "$expected_hash" | tr -d '[:space:]')
    actual_hash=$(printf '%s' "$actual_hash" | tr -d '[:space:]')

    # 比较哈希值
    if [ "$expected_hash" = "$actual_hash" ]; then
        print_success "文件校验通过"
        return 0
    else
        print_error "文件校验失败"
        print_error "期望 (长度 ${#expected_hash}): $expected_hash"
        print_error "实际 (长度 ${#actual_hash}): $actual_hash"
        # 使用 od 命令显示十六进制，帮助调试隐藏字符
        if command -v od &> /dev/null; then
            print_info "期望值十六进制: $(printf '%s' "$expected_hash" | od -An -tx1 | tr -d ' \n')"
            print_info "实际值十六进制: $(printf '%s' "$actual_hash" | od -An -tx1 | tr -d ' \n')"
        fi
        return 1
    fi
}

# 解压 tar.gz 文件
extract_tar_gz() {
    local archive=$1
    local dest_dir=$2

    print_info "正在解压文件..."

    if [ ! -d "$dest_dir" ]; then
        mkdir -p "$dest_dir"
    fi

    if tar -xzf "$archive" -C "$dest_dir"; then
        print_success "解压完成"
        return 0
    else
        print_error "解压失败"
        return 1
    fi
}

# 主安装流程
main() {
    clear
    echo -e "${BOLD}${CYAN}"
    echo "CloudSentinel Agent 安装脚本"
    echo -e "${NC}\n"

    # 获取系统信息
    get_system_info

    # 获取最新版本
    get_latest_version

    # 构建期望的文件名
    BINARY_NAME="agent-$OS_TYPE-$ARCH.tar.gz"
    SHA256_NAME="agent-$OS_TYPE-$ARCH.sha256"

    print_info "查找文件: $BINARY_NAME"

    # 查找二进制包
    BINARY_ASSET=$(find_asset "$BINARY_NAME")
    if [ -z "$BINARY_ASSET" ]; then
        print_error "未找到适用于 $OS_TYPE-$ARCH 的二进制包: $BINARY_NAME"
        exit 1
    fi

    BINARY_URL=$(echo "$BINARY_ASSET" | cut -d'|' -f2)

    # 查找 SHA256 文件
    SHA256_ASSET=$(find_asset "$SHA256_NAME")
    if [ -z "$SHA256_ASSET" ]; then
        print_error "未找到 SHA256 校验文件: $SHA256_NAME"
        exit 1
    fi

    SHA256_URL=$(echo "$SHA256_ASSET" | cut -d'|' -f2)

    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    # 下载文件
    BINARY_FILE="$TEMP_DIR/$BINARY_NAME"
    SHA256_FILE="$TEMP_DIR/$SHA256_NAME"

    if ! download_file "$BINARY_URL" "$BINARY_FILE" "二进制包"; then
        exit 1
    fi

    if ! download_file "$SHA256_URL" "$SHA256_FILE" "SHA256 校验文件"; then
        exit 1
    fi

    # 校验文件
    if ! verify_file "$BINARY_FILE" "$SHA256_FILE"; then
        exit 1
    fi

    # 解压文件
    EXTRACT_DIR="$TEMP_DIR/extract"
    if ! extract_tar_gz "$BINARY_FILE" "$EXTRACT_DIR"; then
        exit 1
    fi

    # 查找解压后的二进制文件
    BINARY_EXE="agent-$OS_TYPE-$ARCH"

    EXTRACTED_BINARY="$EXTRACT_DIR/$BINARY_EXE"

    # 如果直接找不到，尝试在子目录中查找
    if [ ! -f "$EXTRACTED_BINARY" ]; then
        FOUND_BINARY=$(find "$EXTRACT_DIR" -name "$BINARY_EXE" -type f | head -n1)
        if [ -n "$FOUND_BINARY" ]; then
            EXTRACTED_BINARY="$FOUND_BINARY"
        else
            print_error "解压后未找到二进制文件: $BINARY_EXE"
            exit 1
        fi
    fi

    # 询问安装目录
    echo ""
    read -p "$(echo -e ${CYAN}请输入安装目录${NC} $(echo -e ${YELLOW}[默认: $(pwd)]${NC}): )" INSTALL_DIR
    INSTALL_DIR=${INSTALL_DIR:-$(pwd)}

    # 创建安装目录
    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR"
    fi

    INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd)
    print_info "安装目录: ${BOLD}$INSTALL_DIR${NC}"

    # 复制二进制文件
    INSTALLED_BINARY="$INSTALL_DIR/agent"

    print_info "正在复制二进制文件..."
    if cp "$EXTRACTED_BINARY" "$INSTALLED_BINARY"; then
        chmod +x "$INSTALLED_BINARY"
        print_success "二进制文件已复制"
    else
        print_error "复制二进制文件失败"
        exit 1
    fi

    # 完成
    echo ""
    print_separator
    echo -e "${BOLD}${GREEN}✓ 安装完成！${NC}"
    print_separator
    echo ""

    # 显示安装信息
    echo -e "${BOLD}安装信息：${NC}"
    echo -e "  版本: ${BOLD}$TAG_NAME${NC}"
    echo -e "  安装目录: ${BOLD}$INSTALL_DIR${NC}"
    echo -e "  二进制文件: ${BOLD}$INSTALLED_BINARY${NC}"
    echo ""
}

# 执行主函数
main "$@"

