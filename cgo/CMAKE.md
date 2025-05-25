# WebDAV CGO 库 CMake 构建指南

本文档介绍如何使用 CMake 构建 WebDAV CGO 库和示例程序。

## 前提条件

1. **CMake 3.16+**
   ```bash
   # macOS
   brew install cmake
   
   # Ubuntu/Debian
   sudo apt-get install cmake
   
   # CentOS/RHEL
   sudo yum install cmake
   ```

2. **Go 库文件**
   在使用 CMake 之前，必须先构建 Go 库文件：
   ```bash
   ./build.sh -t darwin/arm64 -s -d  # 构建静态库和动态库
   ```

## 快速开始

### 1. 使用构建脚本（推荐）

```bash
# 基本构建
./cmake_build.sh

# 调试模式构建
./cmake_build.sh -t Debug -v

# 清理并重新构建
./cmake_build.sh -c

# 构建并安装
./cmake_build.sh --install
```

### 2. 手动使用 CMake

```bash
# 创建构建目录
mkdir build && cd build

# 配置项目
cmake ..

# 构建
cmake --build .

# 安装（可选）
cmake --install .
```

## 构建选项

### CMake 变量

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `CMAKE_BUILD_TYPE` | Release | 构建类型 (Debug/Release/RelWithDebInfo/MinSizeRel) |
| `BUILD_STATIC` | ON | 构建静态库示例 |
| `BUILD_DYNAMIC` | ON | 构建动态库示例 |
| `BUILD_EXAMPLES` | ON | 构建示例程序 |
| `BUILD_TESTS` | ON | 构建测试 |
| `CMAKE_INSTALL_PREFIX` | /usr/local | 安装前缀 |

### 示例用法

```bash
# 只构建静态库示例
cmake -DBUILD_DYNAMIC=OFF ..

# 不构建示例程序
cmake -DBUILD_EXAMPLES=OFF ..

# 设置安装前缀
cmake -DCMAKE_INSTALL_PREFIX=/opt/webdav ..

# 调试模式
cmake -DCMAKE_BUILD_TYPE=Debug ..
```

## 构建目标

### 可执行文件

- `c_example_static` - 静态库交互式示例
- `c_example_daemon_static` - 静态库守护进程示例
- `c_example_dynamic` - 动态库交互式示例
- `c_example_daemon_dynamic` - 动态库守护进程示例

### 自定义目标

- `run_static` - 运行静态库示例
- `run_dynamic` - 运行动态库示例
- `test` - 运行功能测试

### 使用示例

```bash
# 构建特定目标
make c_example_static

# 运行示例
make run_static

# 运行测试
make test
```

## 项目结构

```
cgo/
├── CMakeLists.txt              # 主 CMake 文件
├── cmake_build.sh              # 构建脚本
├── CMAKE.md                    # 本文档
├── cmake/
│   └── webdav.pc.in           # pkg-config 模板
├── examples/
│   ├── CMakeLists.txt         # 示例 CMake 文件
│   ├── c_example.c            # C 示例源码
│   └── c_example_daemon.c     # 守护进程示例源码
└── build/                     # 构建输出目录
    ├── examples/
    │   ├── c_example_static
    │   ├── c_example_dynamic
    │   └── webdav_root/       # 测试目录
    └── webdav.pc              # pkg-config 文件
```

## 平台支持

### 支持的平台

- **Linux** (x86_64, ARM64)
- **macOS** (x86_64, ARM64)
- **Windows** (x86_64, ARM64)

### 平台特定配置

#### macOS
自动链接必要的框架：
- CoreFoundation
- Security
- pthread

#### Linux
自动链接：
- pthread
- m (数学库)

#### Windows
自动链接：
- ws2_32
- winmm

## 安装

### 系统安装

```bash
# 构建并安装到系统目录
./cmake_build.sh --install

# 或手动安装
cd build
cmake --install .
```

### 自定义安装位置

```bash
# 安装到自定义目录
./cmake_build.sh -p /opt/webdav --install

# 或使用 CMake
cmake -DCMAKE_INSTALL_PREFIX=/opt/webdav ..
cmake --install .
```

### 安装组件

- `static` - 静态库文件
- `dynamic` - 动态库文件
- `headers` - 头文件
- `examples` - 示例程序
- `tests` - 测试脚本
- `pkgconfig` - pkg-config 文件

## 在其他项目中使用

### 使用 find_package

```cmake
# 在你的 CMakeLists.txt 中
find_package(PkgConfig REQUIRED)
pkg_check_modules(WEBDAV REQUIRED webdav)

target_link_libraries(your_target ${WEBDAV_LIBRARIES})
target_include_directories(your_target PRIVATE ${WEBDAV_INCLUDE_DIRS})
```

### 直接链接

```cmake
# 静态库
target_link_libraries(your_target /path/to/libwebdav_darwin_arm64.a)
target_include_directories(your_target PRIVATE /path/to/include)

# macOS 还需要链接框架
if(APPLE)
    target_link_libraries(your_target 
        "-framework CoreFoundation"
        "-framework Security"
        pthread
    )
endif()
```

## 故障排除

### 常见问题

1. **找不到 webdav.h**
   ```
   解决方案: 先运行 ./build.sh 构建 Go 库
   ```

2. **链接错误**
   ```
   解决方案: 确保库文件存在于 dist/lib/ 目录
   ```

3. **权限错误**
   ```
   解决方案: 给构建脚本添加执行权限
   chmod +x cmake_build.sh
   ```

### 调试构建

```bash
# 详细输出
./cmake_build.sh -v

# 查看 CMake 变量
cmake -LAH ..

# 查看生成的 Makefile
make VERBOSE=1
```

## 与传统 Makefile 的比较

| 特性 | Makefile | CMake |
|------|----------|-------|
| 跨平台支持 | 有限 | 优秀 |
| 依赖管理 | 手动 | 自动 |
| IDE 集成 | 有限 | 优秀 |
| 配置灵活性 | 中等 | 高 |
| 学习曲线 | 简单 | 中等 |

## 总结

CMake 构建系统为 WebDAV CGO 库提供了：

- ✅ 跨平台构建支持
- ✅ 灵活的配置选项
- ✅ 自动依赖管理
- ✅ IDE 集成支持
- ✅ 标准化的安装流程
- ✅ pkg-config 支持

推荐在需要跨平台支持或复杂构建配置的项目中使用 CMake。 