# WebDAV CGO 库

这是一个通过 CGO 导出的 WebDAV 服务器库，提供 C 兼容的接口，可以被其他语言（如 C/C++、Python、Java、C#、Rust 等）通过静态库或动态库的形式调用。

## 特性

- ✅ C 兼容的 API 接口
- ✅ 支持静态库和动态库
- ✅ 跨平台支持 (Linux, macOS, Windows)
- ✅ 多架构支持 (AMD64, ARM64)
- ✅ 完整的 WebDAV 协议支持
- ✅ 用户认证和权限管理
- ✅ TLS/SSL 支持
- ✅ 可配置的日志级别
- ✅ 线程安全

## 支持的平台

| 操作系统 | 架构 | 静态库 | 动态库 |
|---------|------|--------|--------|
| Linux   | AMD64 | ✅ | ✅ |
| Linux   | ARM64 | ✅ | ✅ |
| macOS   | AMD64 | ✅ | ✅ |
| macOS   | ARM64 | ✅ | ✅ |
| Windows | AMD64 | ✅ | ✅ |

## 构建

### 方法一：使用 Shell 脚本（推荐）

```bash
# 构建所有平台的静态库和动态库
./build.sh -a

# 构建特定平台
./build.sh -t linux/amd64 -s -d
./build.sh -t darwin/arm64 -s -d
./build.sh -t windows/amd64 -s -d

# 只构建静态库
./build.sh -t darwin/arm64 -s

# 只构建动态库
./build.sh -t darwin/arm64 -d
```

### 方法二：使用 CMake

CMake 提供了更现代的构建体验，支持 IDE 集成和更灵活的配置。

#### 前提条件

1. **安装 CMake 3.16+**
   ```bash
   # macOS
   brew install cmake
   
   # Ubuntu/Debian
   sudo apt-get install cmake
   
   # CentOS/RHEL
   sudo yum install cmake
   ```

2. **先构建 Go 库**
   ```bash
   ./build.sh -t darwin/arm64 -s -d  # 构建静态库和动态库
   ```

#### 使用 CMake 构建脚本

```bash
# 基本构建
./cmake_build.sh

# 调试模式构建
./cmake_build.sh -t Debug -v

# 清理并重新构建
./cmake_build.sh -c

# 构建并安装
./cmake_build.sh --install

# 只构建静态库示例
./cmake_build.sh --no-dynamic

# 查看所有选项
./cmake_build.sh --help
```

#### 手动使用 CMake

```bash
# 创建构建目录
mkdir build && cd build

# 配置项目
cmake ..

# 构建
cmake --build .

# 运行测试
make test

# 安装（可选）
cmake --install .
```

#### CMake 构建选项

| 选项 | 默认值 | 描述 |
|------|--------|------|
| `BUILD_STATIC` | ON | 构建静态库示例 |
| `BUILD_DYNAMIC` | ON | 构建动态库示例 |
| `BUILD_EXAMPLES` | ON | 构建示例程序 |
| `BUILD_TESTS` | ON | 构建测试 |
| `CMAKE_BUILD_TYPE` | Release | 构建类型 |

详细的 CMake 使用说明请参考 [CMAKE.md](CMAKE.md)。

## 快速开始

### 1. 构建库

```bash
# 进入 cgo 目录
cd cgo

# 构建所有平台的库
./build.sh

# 或者只构建当前平台
./build.sh -t $(uname -s | tr '[:upper:]' '[:lower:]')/amd64

# 只构建静态库
./build.sh -s

# 只构建动态库
./build.sh -d
```

构建完成后，库文件将位于 `dist/` 目录：

```
dist/
├── lib/                    # 库文件
│   ├── libwebdav_linux_amd64.a
│   ├── libwebdav_linux_amd64.so
│   ├── libwebdav_darwin_amd64.a
│   ├── libwebdav_darwin_amd64.dylib
│   └── webdav.pc          # pkg-config 文件
├── include/               # 头文件
│   └── webdav.h
└── examples/              # 示例代码
```

### 2. C/C++ 使用示例

```c
#include "webdav.h"
#include <stdio.h>

int main() {
    // 创建服务器
    int server_id = webdav_create_server(
        "127.0.0.1",     // 地址
        8080,            // 端口
        "./webdav_root", // 根目录
        "admin",         // 用户名
        "password",      // 密码
        0,               // 不使用 TLS
        NULL, NULL,      // 证书和密钥文件
        "/",             // URL 前缀
        0, 0, 1          // 配置选项
    );
    
    if (server_id < 0) {
        printf("创建服务器失败\n");
        return 1;
    }
    
    // 启动服务器
    if (webdav_start_server(server_id) != 0) {
        printf("启动服务器失败\n");
        return 1;
    }
    
    printf("WebDAV 服务器运行在 http://127.0.0.1:8080/\n");
    getchar(); // 等待用户输入
    
    // 停止服务器
    webdav_stop_server(server_id);
    webdav_cleanup();
    
    return 0;
}
```

编译：

```bash
# 使用动态库
gcc -I./dist/include -L./dist/lib -o example example.c -lwebdav_linux_amd64

# 使用静态库
gcc -I./dist/include -o example example.c ./dist/lib/libwebdav_linux_amd64.a -lpthread
```

### 3. Python 使用示例

```python
import ctypes

# 加载库
lib = ctypes.CDLL('./dist/lib/libwebdav_linux_amd64.so')

# 设置函数签名
lib.webdav_create_server.argtypes = [
    ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p,
    ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int,
    ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p,
    ctypes.c_int, ctypes.c_int, ctypes.c_int
]
lib.webdav_create_server.restype = ctypes.c_int

# 创建服务器
server_id = lib.webdav_create_server(
    b"127.0.0.1", 8080, b"./webdav_root",
    b"admin", b"password", 0,
    None, None, b"/",
    0, 0, 1
)

# 启动服务器
lib.webdav_start_server(server_id)
print("WebDAV 服务器运行在 http://127.0.0.1:8080/")

input("按 Enter 键停止...")

# 停止服务器
lib.webdav_stop_server(server_id)
lib.webdav_cleanup()
```

## API 参考

### 核心函数

#### `webdav_create_server`

创建 WebDAV 服务器实例。

```c
int webdav_create_server(
    const char* address,      // 监听地址
    int port,                 // 监听端口
    const char* directory,    // 根目录
    const char* username,     // 用户名 (可为 NULL)
    const char* password,     // 密码 (可为 NULL)
    int tls,                  // 是否启用 TLS
    const char* cert_file,    // 证书文件路径
    const char* key_file,     // 私钥文件路径
    const char* prefix,       // URL 前缀
    int no_password,          // 是否禁用密码检查
    int behind_proxy,         // 是否在代理后面
    int debug                 // 是否启用调试
);
```

**返回值：**
- `> 0`: 服务器 ID
- `< 0`: 错误码

#### `webdav_start_server`

启动服务器。

```c
int webdav_start_server(int server_id);
```

#### `webdav_stop_server`

停止服务器。

```c
int webdav_stop_server(int server_id);
```

#### `webdav_cleanup`

清理所有资源。

```c
void webdav_cleanup(void);
```

### 辅助函数

#### `webdav_get_version`

获取库版本信息。

```c
char* webdav_get_version(void);
```

#### `webdav_set_log_level`

设置日志级别。

```c
int webdav_set_log_level(int level);
```

日志级别：
- `WEBDAV_LOG_DEBUG` (0)
- `WEBDAV_LOG_INFO` (1)
- `WEBDAV_LOG_WARN` (2)
- `WEBDAV_LOG_ERROR` (3)

#### `webdav_get_server_info`

获取服务器信息。

```c
int webdav_get_server_info(int server_id, char* buffer, int buffer_size);
```

### 错误码

| 错误码 | 常量 | 描述 |
|--------|------|------|
| 0 | `WEBDAV_SUCCESS` | 成功 |
| -1 | `WEBDAV_ERROR_INVALID_CONFIG` | 无效配置 |
| -2 | `WEBDAV_ERROR_LOGGER_INIT` | 日志初始化失败 |
| -3 | `WEBDAV_ERROR_HANDLER_INIT` | 处理器初始化失败 |

## 示例程序

### 运行 C 示例

```bash
cd cgo/examples
make
make run
```

### 运行 Python 示例

```bash
cd cgo/examples
make run-python
```

## 其他语言绑定

### Java (JNI)

```java
public class WebDAVServer {
    static {
        System.loadLibrary("webdav_linux_amd64");
    }
    
    public native int createServer(String address, int port, String directory,
                                  String username, String password, boolean tls,
                                  String certFile, String keyFile, String prefix,
                                  boolean noPassword, boolean behindProxy, boolean debug);
    public native int startServer(int serverId);
    public native int stopServer(int serverId);
    public native void cleanup();
}
```

### C# (P/Invoke)

```csharp
using System;
using System.Runtime.InteropServices;

public class WebDAVServer
{
    [DllImport("libwebdav_windows_amd64.dll")]
    public static extern int webdav_create_server(
        string address, int port, string directory,
        string username, string password, int tls,
        string certFile, string keyFile, string prefix,
        int noPassword, int behindProxy, int debug);
    
    [DllImport("libwebdav_windows_amd64.dll")]
    public static extern int webdav_start_server(int serverId);
    
    [DllImport("libwebdav_windows_amd64.dll")]
    public static extern int webdav_stop_server(int serverId);
    
    [DllImport("libwebdav_windows_amd64.dll")]
    public static extern void webdav_cleanup();
}
```

### Rust (FFI)

```rust
use std::ffi::CString;
use std::os::raw::{c_char, c_int};

#[link(name = "webdav_linux_amd64")]
extern "C" {
    fn webdav_create_server(
        address: *const c_char,
        port: c_int,
        directory: *const c_char,
        username: *const c_char,
        password: *const c_char,
        tls: c_int,
        cert_file: *const c_char,
        key_file: *const c_char,
        prefix: *const c_char,
        no_password: c_int,
        behind_proxy: c_int,
        debug: c_int,
    ) -> c_int;
    
    fn webdav_start_server(server_id: c_int) -> c_int;
    fn webdav_stop_server(server_id: c_int) -> c_int;
    fn webdav_cleanup();
}

pub struct WebDAVServer {
    server_id: c_int,
}

impl WebDAVServer {
    pub fn new(address: &str, port: i32, directory: &str) -> Result<Self, i32> {
        let address_c = CString::new(address).unwrap();
        let directory_c = CString::new(directory).unwrap();
        
        let server_id = unsafe {
            webdav_create_server(
                address_c.as_ptr(),
                port,
                directory_c.as_ptr(),
                std::ptr::null(),
                std::ptr::null(),
                0,
                std::ptr::null(),
                std::ptr::null(),
                CString::new("/").unwrap().as_ptr(),
                0, 0, 1,
            )
        };
        
        if server_id < 0 {
            Err(server_id)
        } else {
            Ok(WebDAVServer { server_id })
        }
    }
    
    pub fn start(&self) -> Result<(), i32> {
        let result = unsafe { webdav_start_server(self.server_id) };
        if result == 0 { Ok(()) } else { Err(result) }
    }
}
```

## 构建选项

### 环境变量

- `CGO_ENABLED`: 启用 CGO (必须设置为 1)
- `GOOS`: 目标操作系统 (linux, darwin, windows)
- `GOARCH`: 目标架构 (amd64, arm64)

### 构建模式

- `c-archive`: 生成静态库 (.a) 和头文件 (.h)
- `c-shared`: 生成动态库 (.so/.dylib/.dll) 和头文件 (.h)

## 故障排除

### 常见问题

1. **库加载失败**
   - 确保库文件路径正确
   - 检查库文件权限
   - 在 Linux/macOS 上设置 `LD_LIBRARY_PATH` 或 `DYLD_LIBRARY_PATH`

2. **编译错误**
   - 确保安装了 Go 和 GCC
   - 检查 CGO 是否启用
   - 验证目标平台支持

3. **运行时错误**
   - 检查目录权限
   - 确保端口未被占用
   - 查看日志输出

### 调试

启用调试模式：

```c
webdav_set_log_level(WEBDAV_LOG_DEBUG);
```

## 许可证

本项目采用与原 WebDAV 项目相同的许可证。

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更多信息

- [原始 WebDAV 项目](https://github.com/hacdias/webdav)
- [Go CGO 文档](https://golang.org/cmd/cgo/)
- [WebDAV 协议规范](https://tools.ietf.org/html/rfc4918) 