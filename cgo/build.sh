#!/bin/bash

# WebDAV CGO 库构建脚本
# 支持构建静态库和动态库，适用于多种平台

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    print_info "检查构建依赖..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go 编译器未找到，请安装 Go"
        exit 1
    fi
    
    if ! command -v gcc &> /dev/null; then
        print_error "GCC 编译器未找到，请安装 GCC"
        exit 1
    fi
    
    print_success "依赖检查完成"
}

# 创建输出目录
create_output_dirs() {
    print_info "创建输出目录..."
    
    mkdir -p dist/lib
    mkdir -p dist/include
    mkdir -p dist/examples
    
    print_success "输出目录创建完成"
}

# 构建静态库
build_static_lib() {
    local target_os=$1
    local target_arch=$2
    
    print_info "构建静态库 (${target_os}/${target_arch})..."
    
    # 设置环境变量
    export CGO_ENABLED=1
    export GOOS=$target_os
    export GOARCH=$target_arch
    
    # 构建静态库
    go build -buildmode=c-archive -o "dist/lib/libwebdav_${target_os}_${target_arch}.a" webdav_export.go
    
    # 复制头文件
    cp "dist/lib/libwebdav_${target_os}_${target_arch}.h" "dist/include/" 2>/dev/null || true
    
    print_success "静态库构建完成: libwebdav_${target_os}_${target_arch}.a"
}

# 构建动态库
build_shared_lib() {
    local target_os=$1
    local target_arch=$2
    
    print_info "构建动态库 (${target_os}/${target_arch})..."
    
    # 设置环境变量
    export CGO_ENABLED=1
    export GOOS=$target_os
    export GOARCH=$target_arch
    
    # 根据操作系统设置动态库扩展名
    local ext=""
    case $target_os in
        "windows")
            ext=".dll"
            ;;
        "darwin")
            ext=".dylib"
            ;;
        *)
            ext=".so"
            ;;
    esac
    
    # 构建动态库
    go build -buildmode=c-shared -o "dist/lib/libwebdav_${target_os}_${target_arch}${ext}" webdav_export.go
    
    print_success "动态库构建完成: libwebdav_${target_os}_${target_arch}${ext}"
}

# 复制头文件
copy_headers() {
    print_info "复制头文件..."
    
    cp webdav.h dist/include/
    
    print_success "头文件复制完成"
}

# 生成 pkg-config 文件
generate_pkgconfig() {
    print_info "生成 pkg-config 文件..."
    
    cat > dist/lib/webdav.pc << EOF
prefix=\${pcfiledir}/..
exec_prefix=\${prefix}
libdir=\${prefix}/lib
includedir=\${prefix}/include

Name: WebDAV CGO Library
Description: WebDAV server library with C interface
Version: 1.0.0
Libs: -L\${libdir} -lwebdav
Cflags: -I\${includedir}
EOF
    
    print_success "pkg-config 文件生成完成"
}

# 显示帮助信息
show_help() {
    echo "WebDAV CGO 库构建脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help              显示此帮助信息"
    echo "  -s, --static            只构建静态库"
    echo "  -d, --shared            只构建动态库"
    echo "  -a, --all               构建所有平台的库 (默认)"
    echo "  -t, --target OS/ARCH    指定目标平台 (例如: linux/amd64)"
    echo "  --clean                 清理构建输出"
    echo ""
    echo "支持的平台:"
    echo "  linux/amd64, linux/arm64"
    echo "  darwin/amd64, darwin/arm64"
    echo "  windows/amd64, windows/arm64"
    echo ""
    echo "示例:"
    echo "  $0                      # 构建所有平台的静态库和动态库"
    echo "  $0 -s                   # 只构建静态库"
    echo "  $0 -t linux/amd64       # 只构建 Linux AMD64 平台"
    echo "  $0 --clean              # 清理构建输出"
}

# 清理构建输出
clean_build() {
    print_info "清理构建输出..."
    rm -rf dist/
    print_success "清理完成"
}

# 主构建函数
main_build() {
    local build_static=true
    local build_shared=true
    local target_platforms=()
    
    # 默认支持的平台
    local default_platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -s|--static)
                build_shared=false
                shift
                ;;
            -d|--shared)
                build_static=false
                shift
                ;;
            -a|--all)
                target_platforms=("${default_platforms[@]}")
                shift
                ;;
            -t|--target)
                target_platforms=("$2")
                shift 2
                ;;
            --clean)
                clean_build
                exit 0
                ;;
            *)
                print_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 如果没有指定平台，使用默认平台
    if [[ ${#target_platforms[@]} -eq 0 ]]; then
        target_platforms=("${default_platforms[@]}")
    fi
    
    # 开始构建
    print_info "开始构建 WebDAV CGO 库..."
    
    check_dependencies
    create_output_dirs
    copy_headers
    
    # 构建每个平台
    for platform in "${target_platforms[@]}"; do
        IFS='/' read -r os arch <<< "$platform"
        
        print_info "构建平台: $platform"
        
        if [[ "$build_static" == true ]]; then
            build_static_lib "$os" "$arch"
        fi
        
        if [[ "$build_shared" == true ]]; then
            build_shared_lib "$os" "$arch"
        fi
    done
    
    generate_pkgconfig
    
    print_success "所有构建任务完成！"
    print_info "输出目录: $(pwd)/dist/"
    print_info "库文件: dist/lib/"
    print_info "头文件: dist/include/"
}

# 如果没有参数，显示帮助
if [[ $# -eq 0 ]]; then
    main_build
else
    main_build "$@"
fi 