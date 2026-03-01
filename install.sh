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

# 创建 cloudsentinel-agent 用户
create_cloudsentinel_agent_user() {
    local agent_dir=$1

    # 检查是否有 root 权限
    if [ "$EUID" -ne 0 ]; then
        print_warning "创建用户需要 root 权限，将使用当前用户运行"
        return 0
    fi

    # 检查用户是否已存在
    if id "cloudsentinel-agent" &>/dev/null; then
        print_info "用户 cloudsentinel-agent 已存在"
    else
        print_info "正在创建 cloudsentinel-agent 用户..."
        # 创建系统用户（无登录 shell，无主目录）
        if useradd -r -s /bin/false cloudsentinel-agent 2>/dev/null; then
            print_success "用户 cloudsentinel-agent 创建成功"
        else
            print_warning "创建用户失败，将使用当前用户运行"
            return 0
        fi
    fi

    # 设置目录所有权（同时授予 root 组访问权限）
    print_info "正在设置目录权限..."
    if chown -R cloudsentinel-agent:root "$agent_dir"; then
        print_success "目录权限设置成功"
    else
        print_warning "设置目录权限失败"
        return 0
    fi

    # 设置目录权限：
    # - 2770: owner/group 可读写执行，其他无权限；setgid 确保新文件继承 root 组
    chmod 2770 "$agent_dir"

    return 0
}

# 确保 cloudsentinel-agent 用户可以进入安装目录（修复父目录不可 traverse 导致的 cd 失败）
ensure_cloudsentinel_agent_can_access_dir() {
    local base_dir=$1
    local target_dir=$2

    # 仅在 root 且 cloudsentinel-agent 用户存在时处理
    if [ "$EUID" -ne 0 ] || ! id "cloudsentinel-agent" &>/dev/null; then
        return 0
    fi

    # 若已可进入则直接返回
    if sudo -u cloudsentinel-agent test -d "$target_dir" 2>/dev/null && sudo -u cloudsentinel-agent test -x "$target_dir" 2>/dev/null; then
        return 0
    fi

    print_warning "cloudsentinel-agent 用户无法进入安装目录，尝试修复父目录权限（最小化授权）..."

    # 优先使用 ACL（更安全：不需要放开整个 base_dir 的访问权限）
    if command -v setfacl &>/dev/null; then
        # 允许 cloudsentinel-agent traverse base_dir，允许其访问 target_dir
        setfacl -m "u:cloudsentinel-agent:--x" "$base_dir" || true
        setfacl -m "u:cloudsentinel-agent:rwx" "$target_dir" || true
        # 让 target_dir 下新建文件默认也给 cloudsentinel-agent rwx（避免后续写入问题）
        setfacl -d -m "u:cloudsentinel-agent:rwx" "$target_dir" || true
    else
        print_warning "未检测到 setfacl，将退化为 chmod o+x 方式放开父目录可进入权限"
        chmod o+x "$base_dir" || true
    fi

    # 再次校验
    if ! sudo -u cloudsentinel-agent test -d "$target_dir" 2>/dev/null || ! sudo -u cloudsentinel-agent test -x "$target_dir" 2>/dev/null; then
        print_warning "仍无法让 cloudsentinel-agent 进入安装目录：$target_dir"
        print_warning "建议将安装目录选在 /opt 或 /srv 等公共路径，例如：/opt/cloudsentinel-agent"
        return 0
    fi

    return 0
}

# 检查是否为 root 且 cloudsentinel-agent 用户存在
is_root_with_cloudsentinel_agent() {
    [ "$EUID" -eq 0 ] && id "cloudsentinel-agent" &>/dev/null
}

# 显示手动执行命令提示
show_manual_command() {
    local cmd=$1
    if id "cloudsentinel-agent" &>/dev/null; then
        echo -e "  ${CYAN}sudo -u cloudsentinel-agent $INSTALL_DIR/agent $cmd${NC}"
    else
        echo -e "  ${CYAN}cd $INSTALL_DIR${NC}"
        echo -e "  ${CYAN}./agent $cmd${NC}"
    fi
}

# 创建全局命令
create_global_command() {
    local install_dir=$1
    local binary_path=$2
    
    # 确定全局命令路径
    local global_cmd_path=""
    
    if [ "$EUID" -eq 0 ]; then
        # root 用户：使用 /usr/local/bin
        global_cmd_path="/usr/local/bin/cloudsentinel-agent"
    else
        # 非 root 用户：使用 ~/.local/bin
        global_cmd_path="$HOME/.local/bin/cloudsentinel-agent"
        
        # 确保 ~/.local/bin 目录存在
        if [ ! -d "$HOME/.local/bin" ]; then
            mkdir -p "$HOME/.local/bin"
        fi
        
        # 检查 ~/.local/bin 是否在 PATH 中
        if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
            print_warning "~/.local/bin 不在 PATH 中，请将以下内容添加到 ~/.bashrc 或 ~/.zshrc："
            echo -e "  ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
            echo ""
            print_info "添加后请执行: ${CYAN}source ~/.bashrc${NC} 或 ${CYAN}source ~/.zshrc${NC}"
        fi
    fi
    
    # 检查是否已存在
    if [ -f "$global_cmd_path" ] || [ -L "$global_cmd_path" ]; then
        print_warning "全局命令已存在: $global_cmd_path"
        read -p "$(echo -e ${YELLOW}是否覆盖？${NC} [y/N]): " overwrite
        if [ "$overwrite" != "y" ] && [ "$overwrite" != "Y" ]; then
            print_info "跳过创建全局命令"
            return 0
        fi
        # 删除旧文件
        rm -f "$global_cmd_path"
    fi
    
    # 创建包装脚本
    print_info "正在创建全局命令: ${BOLD}$global_cmd_path${NC}"
    
    # 生成包装脚本内容
    cat > "$global_cmd_path" << 'WRAPPER_EOF'
#!/bin/bash
# CloudSentinel Agent 全局命令包装脚本
# 此脚本由安装程序自动生成

INSTALL_DIR="INSTALL_DIR_PLACEHOLDER"
BINARY_PATH="$INSTALL_DIR/agent"

# 检查安装目录是否存在
if [ ! -d "$INSTALL_DIR" ]; then
    echo "错误: CloudSentinel Agent 安装目录不存在: $INSTALL_DIR" >&2
    echo "请重新运行安装脚本或手动设置正确的安装目录" >&2
    exit 1
fi

# 检查二进制文件是否存在
if [ ! -f "$BINARY_PATH" ]; then
    echo "错误: CloudSentinel Agent 二进制文件不存在: $BINARY_PATH" >&2
    exit 1
fi

# 切换到安装目录并执行命令
cd "$INSTALL_DIR" || exit 1

# 如果有 cloudsentinel-agent 用户且当前是 root，使用该用户执行
# 这样可以确保权限一致性
if [ "$EUID" -eq 0 ] && id "cloudsentinel-agent" &>/dev/null 2>&1; then
    # root 用户且存在 cloudsentinel-agent 用户：使用 sudo -u
    exec sudo -u cloudsentinel-agent "$BINARY_PATH" "$@"
else
    # 其他情况：直接执行（非 root 用户或不存在 cloudsentinel-agent 用户）
    exec "$BINARY_PATH" "$@"
fi
WRAPPER_EOF
    
    # 替换安装目录占位符
    sed -i "s|INSTALL_DIR_PLACEHOLDER|$install_dir|g" "$global_cmd_path"
    
    # 设置执行权限
    chmod +x "$global_cmd_path"
    
    # 如果是 root 用户，设置所有权
    if [ "$EUID" -eq 0 ]; then
        chown root:root "$global_cmd_path"
    fi
    
    print_success "全局命令已创建: ${BOLD}$global_cmd_path${NC}"
    print_info "现在可以在任意位置使用 ${BOLD}cloudsentinel-agent${NC} 命令"
    
    return 0
}

# 以 cloudsentinel-agent 用户执行命令
run_as_cloudsentinel_agent() {
    local command=$1
    local working_dir=$2

    if is_root_with_cloudsentinel_agent; then
        if [ -n "$working_dir" ]; then
            sudo -u cloudsentinel-agent sh -c "cd '$working_dir' && $command"
        else
            sudo -u cloudsentinel-agent sh -c "$command"
        fi
    else
        if [ -n "$working_dir" ]; then
            (cd "$working_dir" && eval "$command")
        else
            eval "$command"
        fi
    fi
}

# 检测 systemd 是否已安装
check_systemd() {
    # 检查 systemctl 命令是否存在
    if command -v systemctl &> /dev/null; then
        # 检查 systemd 是否正在运行
        if systemctl --version &> /dev/null; then
            return 0
        fi
    fi
    return 1
}

# 检测 Linux 发行版
detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO_ID="$ID"
        DISTRO_VERSION_ID="$VERSION_ID"
    elif [ -f /etc/redhat-release ]; then
        # 旧版 CentOS/RHEL
        if grep -q "CentOS" /etc/redhat-release; then
            DISTRO_ID="centos"
        elif grep -q "Red Hat" /etc/redhat-release; then
            DISTRO_ID="rhel"
        fi
    elif [ -f /etc/debian_version ]; then
        DISTRO_ID="debian"
    fi
}

# 安装 systemd
install_systemd() {
    if [ "$EUID" -ne 0 ]; then
        print_error "安装 systemd 需要 root 权限，请使用 sudo 运行安装脚本"
        return 1
    fi

    detect_distro

    print_info "检测到 Linux 发行版: ${BOLD}$DISTRO_ID${NC}"

    case "$DISTRO_ID" in
        ubuntu|debian)
            print_info "正在安装 systemd..."
            if apt-get update && apt-get install -y systemd; then
                print_success "systemd 安装成功"
                return 0
            else
                print_error "systemd 安装失败"
                return 1
            fi
            ;;
        centos|rhel|fedora|rocky|almalinux)
            print_info "正在安装 systemd..."
            if command -v dnf &> /dev/null; then
                if dnf install -y systemd; then
                    print_success "systemd 安装成功"
                    return 0
                else
                    print_error "systemd 安装失败"
                    return 1
                fi
            elif command -v yum &> /dev/null; then
                if yum install -y systemd; then
                    print_success "systemd 安装成功"
                    return 0
                else
                    print_error "systemd 安装失败"
                    return 1
                fi
            else
                print_error "未找到包管理器 (dnf/yum)"
                return 1
            fi
            ;;
        *)
            print_warning "未识别的 Linux 发行版: $DISTRO_ID"
            print_info "请手动安装 systemd，或使用以下命令："
            echo -e "  ${CYAN}Ubuntu/Debian:${NC} sudo apt-get install systemd"
            echo -e "  ${CYAN}CentOS/RHEL:${NC} sudo yum install systemd"
            echo -e "  ${CYAN}Fedora:${NC} sudo dnf install systemd"
            return 1
            ;;
    esac
}

# 安装并启用 systemd 服务
install_agent_service() {
    local install_dir=$1
    local binary_path="$install_dir/agent"

    if [ "$EUID" -ne 0 ]; then
        print_warning "安装 systemd 服务需要 root 权限"
        print_info "请使用以下命令安装服务："
        echo -e "  ${CYAN}sudo $binary_path install${NC}"
        return 1
    fi

    # 检查二进制文件是否存在
    if [ ! -f "$binary_path" ]; then
        print_error "二进制文件不存在: $binary_path"
        return 1
    fi

    print_step "正在安装 systemd 服务..."

    # 使用 agent 的 install 命令安装服务
    if "$binary_path" install; then
        print_success "systemd 服务安装成功"
        
        # 重新加载 systemd daemon
        if systemctl daemon-reload; then
            print_success "systemd daemon 已重新加载"
        else
            print_warning "重新加载 systemd daemon 失败"
        fi

        return 0
    else
        print_error "systemd 服务安装失败"
        return 1
    fi
}

# 主安装流程
main() {
    # 解析命令行参数
    SERVER_URL=""
    AGENT_KEY=""
    DAEMON_MODE=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            --server=*)
                SERVER_URL="${1#*=}"
                shift
                ;;
            --key=*)
                AGENT_KEY="${1#*=}"
                shift
                ;;
            --daemon)
                DAEMON_MODE=true
                shift
                ;;
            *)
                print_warning "未知参数: $1"
                shift
                ;;
        esac
    done

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
    read -p "$(echo -e ${CYAN}请输入安装目录${NC} $(echo -e ${YELLOW}[默认: $(pwd)]${NC}): )" BASE_INSTALL_DIR
    BASE_INSTALL_DIR=${BASE_INSTALL_DIR:-$(pwd)}

    # 创建基础安装目录
    if [ ! -d "$BASE_INSTALL_DIR" ]; then
        mkdir -p "$BASE_INSTALL_DIR"
    fi

    BASE_INSTALL_DIR=$(cd "$BASE_INSTALL_DIR" && pwd)
    
    # 创建 cloudsentinel-agent 子目录
    INSTALL_DIR="$BASE_INSTALL_DIR/cloudsentinel-agent"
    print_info "基础安装目录: ${BOLD}$BASE_INSTALL_DIR${NC}"
    print_info "CloudSentinel Agent 目录: ${BOLD}$INSTALL_DIR${NC}"

    # 创建 cloudsentinel-agent 目录
    if [ ! -d "$INSTALL_DIR" ]; then
        mkdir -p "$INSTALL_DIR"
    fi

    # 创建 cloudsentinel-agent 用户并设置权限
    if ! create_cloudsentinel_agent_user "$INSTALL_DIR"; then
        print_warning "用户创建失败，将使用当前用户运行"
    fi

    if ! ensure_cloudsentinel_agent_can_access_dir "$BASE_INSTALL_DIR" "$INSTALL_DIR"; then
        print_warning "目录访问修复失败，后续操作可能需要手动执行"
    fi

    # 复制二进制文件
    INSTALLED_BINARY="$INSTALL_DIR/agent"

    print_info "正在复制二进制文件..."
    if cp "$EXTRACTED_BINARY" "$INSTALLED_BINARY"; then
        chmod +x "$INSTALLED_BINARY"
        is_root_with_cloudsentinel_agent && chown cloudsentinel-agent:root "$INSTALLED_BINARY"
        print_success "二进制文件已复制"
    else
        print_error "复制二进制文件失败"
        exit 1
    fi
    
    # 设置所有文件的所有权
    if is_root_with_cloudsentinel_agent; then
        print_info "正在设置文件所有权..."
        chown -R cloudsentinel-agent:root "$INSTALL_DIR"
        print_success "文件所有权设置完成"
    fi

    # 如果提供了配置参数，则写入配置文件
    if [ -n "$SERVER_URL" ] && [ -n "$AGENT_KEY" ]; then
        print_info "正在写入配置文件..."
        CONFIG_FILE="$INSTALL_DIR/agent.lock.json"
        
        # 使用 jq 创建 JSON 配置，否则使用 printf
        if command -v jq &> /dev/null; then
            jq -n \
                --arg server "$SERVER_URL" \
                --arg key "$AGENT_KEY" \
                --arg log_path "logs" \
                '{server: $server, key: $key, log_path: $log_path}' > "$CONFIG_FILE"
        else
            # 使用 printf 直接写入 JSON
            printf '{\n  "server": "%s",\n  "key": "%s",\n  "log_path": "logs"\n}' "$SERVER_URL" "$AGENT_KEY" > "$CONFIG_FILE"
        fi
        
        # 设置文件权限
        chmod 600 "$CONFIG_FILE"
        if is_root_with_cloudsentinel_agent; then
            chown cloudsentinel-agent:root "$CONFIG_FILE"
        fi
        
        print_success "配置文件已写入: $CONFIG_FILE"
    elif [ -n "$SERVER_URL" ] || [ -n "$AGENT_KEY" ]; then
        print_warning "配置参数不完整（缺少 server 或 key），将跳过配置文件写入"
    fi

    # 创建全局命令
    echo ""
    if create_global_command "$INSTALL_DIR" "$INSTALLED_BINARY"; then
        print_success "全局命令创建成功"
    else
        print_warning "全局命令创建失败，您仍可以使用完整路径执行命令"
    fi

    # 检测并安装 systemd 服务
    SYSTEMD_INSTALLED=false
    echo ""
    print_step "检查 systemd 支持..."
    
    if check_systemd; then
        print_success "systemd 已安装"
        SYSTEMD_INSTALLED=true
        
        # 尝试安装 systemd 服务
        if install_agent_service "$INSTALL_DIR"; then
            SYSTEMD_INSTALLED=true
        else
            print_warning "systemd 服务安装失败，将使用传统方式启动"
        fi
    else
        print_warning "未检测到 systemd"
        
        # 询问是否安装 systemd
        if [ "$EUID" -eq 0 ]; then
            echo ""
            read -p "$(echo -e ${YELLOW}是否自动安装 systemd？${NC} [Y/n]): " install_systemd_choice
            install_systemd_choice=${install_systemd_choice:-Y}
            
            if [ "$install_systemd_choice" = "Y" ] || [ "$install_systemd_choice" = "y" ]; then
                if install_systemd; then
                    SYSTEMD_INSTALLED=true
                    # 安装成功后，安装 systemd 服务
                    if install_agent_service "$INSTALL_DIR"; then
                        print_success "systemd 服务已安装并配置"
                    fi
                else
                    print_warning "systemd 安装失败，将使用传统方式启动"
                fi
            else
                print_info "跳过 systemd 安装，将使用传统方式启动"
            fi
        else
            print_info "需要 root 权限才能安装 systemd"
            print_info "您可以稍后使用以下命令安装 systemd 服务："
            echo -e "  ${CYAN}sudo $INSTALLED_BINARY install${NC}"
        fi
    fi

    # 自动启动 agent
    AUTO_STARTED=false
    if [ -n "$SERVER_URL" ] && [ -n "$AGENT_KEY" ]; then
        echo ""
        print_step "正在启动 agent..."
        
        # 如果 systemd 服务已安装，优先使用 systemd 启动
        if [ "$SYSTEMD_INSTALLED" = true ] && [ "$EUID" -eq 0 ]; then
            if systemctl start cloudsentinel-agent.service; then
                print_success "agent 已通过 systemd 启动"
                AUTO_STARTED=true
                sleep 1
                # 检查服务状态
                if systemctl is-active --quiet cloudsentinel-agent.service; then
                    print_success "agent 服务运行正常"
                else
                    print_warning "agent 服务可能未正常启动，请检查日志: systemctl status cloudsentinel-agent.service"
                fi
            else
                print_warning "systemd 启动失败，尝试传统方式启动"
                # 回退到传统启动方式
                START_CMD="./agent start"
                if [ "$DAEMON_MODE" = true ]; then
                    START_CMD="$START_CMD --daemon"
                fi
                
                if run_as_cloudsentinel_agent "$START_CMD" "$INSTALL_DIR"; then
                    print_success "agent 已启动"
                    AUTO_STARTED=true
                    if [ "$DAEMON_MODE" = true ]; then
                        print_info "agent 正在后台运行（守护进程模式）"
                    fi
                else
                    print_warning "自动启动失败，请手动执行: cd $INSTALL_DIR && $START_CMD"
                fi
            fi
        else
            # 传统启动方式
            START_CMD="./agent start"
            if [ "$DAEMON_MODE" = true ]; then
                START_CMD="$START_CMD --daemon"
            fi
            
            if run_as_cloudsentinel_agent "$START_CMD" "$INSTALL_DIR"; then
                print_success "agent 已启动"
                AUTO_STARTED=true
                if [ "$DAEMON_MODE" = true ]; then
                    print_info "agent 正在后台运行（守护进程模式）"
                fi
            else
                print_warning "自动启动失败，请手动执行: cd $INSTALL_DIR && $START_CMD"
            fi
        fi
    fi

    # 完成
    echo ""
    print_separator
    echo -e "${BOLD}${GREEN}✓ 安装完成！${NC}"
    print_separator
    echo ""

    # 显示安装信息
    echo -e "${BOLD}${GREEN}安装信息：${NC}"
    echo -e "  版本: ${BOLD}$TAG_NAME${NC}"
    echo -e "  安装目录: ${BOLD}$INSTALL_DIR${NC}"
    echo -e "  二进制文件: ${BOLD}$INSTALLED_BINARY${NC}"
    if [ -n "$SERVER_URL" ] && [ -n "$AGENT_KEY" ]; then
        echo -e "  配置文件: ${BOLD}$INSTALL_DIR/agent.lock.json${NC}"
    fi
    echo ""
    
    # 显示使用提示
    if [ "$AUTO_STARTED" = true ]; then
        echo -e "${BOLD}${GREEN}启动状态：${NC}"
        if [ "$SYSTEMD_INSTALLED" = true ]; then
            echo -e "  ${GREEN}✓${NC} agent 已通过 systemd 服务启动"
        elif [ "$DAEMON_MODE" = true ]; then
            echo -e "  ${GREEN}✓${NC} agent 已使用守护进程模式启动"
        else
            echo -e "  ${GREEN}✓${NC} agent 已启动"
        fi
        echo ""
        if [ "$SYSTEMD_INSTALLED" = true ]; then
            echo -e "${BOLD}${GREEN}systemd 服务管理：${NC}"
            echo -e "  启动服务: ${CYAN}sudo systemctl start cloudsentinel-agent${NC}"
            echo -e "  停止服务: ${CYAN}sudo systemctl stop cloudsentinel-agent${NC}"
            echo -e "  重启服务: ${CYAN}sudo systemctl restart cloudsentinel-agent${NC}"
            echo -e "  查看状态: ${CYAN}sudo systemctl status cloudsentinel-agent${NC}"
            echo -e "  查看日志: ${CYAN}sudo journalctl -u cloudsentinel-agent -f${NC}"
            echo ""
        fi
        if command -v cloudsentinel-agent &>/dev/null; then
            echo -e "${BOLD}${GREEN}使用提示：${NC}"
            echo -e "  你可以在任意位置使用 ${BOLD}cloudsentinel-agent${NC} 命令来管理 agent"
            echo -e "  查看状态: ${CYAN}cloudsentinel-agent status${NC}"
            echo -e "  查看日志: ${CYAN}cloudsentinel-agent logs${NC}"
            echo ""
        fi
    elif [ -n "$SERVER_URL" ] || [ -n "$AGENT_KEY" ]; then
        echo -e "${BOLD}${YELLOW}配置提示：${NC}"
        echo -e "  配置参数不完整，请手动配置后启动 agent"
        echo ""
        if [ "$SYSTEMD_INSTALLED" = true ]; then
            echo -e "${BOLD}${GREEN}systemd 服务管理：${NC}"
            echo -e "  启动服务: ${CYAN}sudo systemctl start cloudsentinel-agent${NC}"
            echo -e "  查看状态: ${CYAN}sudo systemctl status cloudsentinel-agent${NC}"
            echo ""
        fi
        if command -v cloudsentinel-agent &>/dev/null; then
            echo -e "${BOLD}${GREEN}使用提示：${NC}"
            echo -e "  你可以在任意位置使用 ${BOLD}cloudsentinel-agent${NC} 命令来查看cli帮助"
            if [ "$SYSTEMD_INSTALLED" = true ]; then
                echo -e "  或使用 systemd: ${CYAN}sudo systemctl start cloudsentinel-agent${NC}"
            else
                echo -e "  配置完成后，执行 ${CYAN}cloudsentinel-agent start${NC} 来启动Agent"
            fi
            echo ""
        fi
    else
        if [ "$SYSTEMD_INSTALLED" = true ]; then
            echo -e "${BOLD}${GREEN}systemd 服务管理：${NC}"
            echo -e "  启动服务: ${CYAN}sudo systemctl start cloudsentinel-agent${NC}"
            echo -e "  查看状态: ${CYAN}sudo systemctl status cloudsentinel-agent${NC}"
            echo ""
        fi
        if command -v cloudsentinel-agent &>/dev/null; then
            echo -e "${BOLD}${GREEN}使用提示：${NC}"
            echo -e "  你可以在任意位置使用 ${BOLD}cloudsentinel-agent${NC} 命令来查看cli帮助"
            if [ "$SYSTEMD_INSTALLED" = true ]; then
                echo -e "  或使用 systemd: ${CYAN}sudo systemctl start cloudsentinel-agent${NC}"
            else
                echo -e "  接下来，你需要执行 ${CYAN}cloudsentinel-agent start${NC} 来启动Agent"
            fi
            echo ""
        fi
    fi
}

# 执行主函数
main "$@"
