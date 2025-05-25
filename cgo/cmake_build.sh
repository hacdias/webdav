#!/bin/bash

# WebDAV CGO 库 CMake 构建脚本
# 使用方法: ./cmake_build.sh [选项]

set -e

# 默认配置
BUILD_TYPE="Release"
BUILD_DIR="build"
INSTALL_PREFIX=""
BUILD_STATIC=ON
BUILD_DYNAMIC=ON
BUILD_EXAMPLES=ON
BUILD_TESTS=ON
CLEAN_BUILD=false
VERBOSE=false

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

# 显示帮助信息
show_help() {
    cat << EOF
WebDAV CGO 库 CMake 构建脚本

使用方法: $0 [选项]

选项:
  -h, --help              显示此帮助信息
  -t, --type TYPE         构建类型 (Debug|Release|RelWithDebInfo|MinSizeRel)
                          默认: Release
  -d, --build-dir DIR     构建目录，默认: build
  -p, --prefix PREFIX     安装前缀，默认: /usr/local
  --no-static             不构建静态库示例
  --no-dynamic            不构建动态库示例
  --no-examples           不构建示例程序
  --no-tests              不构建测试
  -c, --clean             清理构建目录
  -v, --verbose           详细输出
  --install               构建后安装

示例:
  $0                      # 使用默认设置构建
  $0 -t Debug -v          # 调试模式构建，详细输出
  $0 -c                   # 清理构建目录
  $0 --no-examples        # 不构建示例程序
  $0 --install            # 构建并安装

注意:
  在运行此脚本之前，请先运行 './build.sh' 构建 Go 库文件。

EOF
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -t|--type)
            BUILD_TYPE="$2"
            shift 2
            ;;
        -d|--build-dir)
            BUILD_DIR="$2"
            shift 2
            ;;
        -p|--prefix)
            INSTALL_PREFIX="$2"
            shift 2
            ;;
        --no-static)
            BUILD_STATIC=OFF
            shift
            ;;
        --no-dynamic)
            BUILD_DYNAMIC=OFF
            shift
            ;;
        --no-examples)
            BUILD_EXAMPLES=OFF
            shift
            ;;
        --no-tests)
            BUILD_TESTS=OFF
            shift
            ;;
        -c|--clean)
            CLEAN_BUILD=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        --install)
            INSTALL_AFTER_BUILD=true
            shift
            ;;
        *)
            print_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查 CMake
if ! command -v cmake &> /dev/null; then
    print_error "CMake 未安装，请先安装 CMake"
    exit 1
fi

# 检查 Go 库是否已构建
if [ ! -f "dist/include/webdav.h" ]; then
    print_warning "WebDAV 头文件未找到"
    print_info "请先运行 './build.sh' 构建 Go 库文件"
    exit 1
fi

print_info "开始 CMake 构建..."

# 清理构建目录
if [ "$CLEAN_BUILD" = true ]; then
    print_info "清理构建目录: $BUILD_DIR"
    rm -rf "$BUILD_DIR"
fi

# 创建构建目录
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

# 准备 CMake 参数
CMAKE_ARGS=(
    "-DCMAKE_BUILD_TYPE=$BUILD_TYPE"
    "-DBUILD_STATIC=$BUILD_STATIC"
    "-DBUILD_DYNAMIC=$BUILD_DYNAMIC"
    "-DBUILD_EXAMPLES=$BUILD_EXAMPLES"
    "-DBUILD_TESTS=$BUILD_TESTS"
)

if [ -n "$INSTALL_PREFIX" ]; then
    CMAKE_ARGS+=("-DCMAKE_INSTALL_PREFIX=$INSTALL_PREFIX")
fi

if [ "$VERBOSE" = true ]; then
    CMAKE_ARGS+=("-DCMAKE_VERBOSE_MAKEFILE=ON")
fi

# 运行 CMake 配置
print_info "配置项目..."
cmake "${CMAKE_ARGS[@]}" ..

if [ $? -ne 0 ]; then
    print_error "CMake 配置失败"
    exit 1
fi

# 构建项目
print_info "构建项目..."
if [ "$VERBOSE" = true ]; then
    cmake --build . --config "$BUILD_TYPE" -- -v
else
    cmake --build . --config "$BUILD_TYPE"
fi

if [ $? -ne 0 ]; then
    print_error "构建失败"
    exit 1
fi

print_success "构建完成！"

# 安装（如果请求）
if [ "$INSTALL_AFTER_BUILD" = true ]; then
    print_info "安装项目..."
    cmake --install . --config "$BUILD_TYPE"
    
    if [ $? -eq 0 ]; then
        print_success "安装完成！"
    else
        print_error "安装失败"
        exit 1
    fi
fi

# 显示构建结果
print_info "构建结果:"
echo "  构建目录: $(pwd)"
echo "  构建类型: $BUILD_TYPE"

if [ -f "examples/c_example_static" ]; then
    echo "  静态库示例: examples/c_example_static"
fi

if [ -f "examples/c_example_dynamic" ]; then
    echo "  动态库示例: examples/c_example_dynamic"
fi

if [ -f "examples/quick_test.sh" ]; then
    echo "  测试脚本: examples/quick_test.sh"
fi

print_info "可用的 make 目标:"
echo "  make                    # 构建所有目标"
echo "  make c_example_static   # 构建静态库示例"
echo "  make c_example_dynamic  # 构建动态库示例"
echo "  make run_static         # 运行静态库示例"
echo "  make run_dynamic        # 运行动态库示例"
echo "  make test               # 运行测试"
echo "  make install            # 安装"

print_success "CMake 构建脚本执行完成！" 