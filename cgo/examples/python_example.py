#!/usr/bin/env python3
"""
WebDAV CGO 库 Python 示例
使用 ctypes 调用 C 库
"""

import ctypes
import os
import sys
import time
import platform
from ctypes import c_char_p, c_int, c_void_p, POINTER

class WebDAVServer:
    """WebDAV 服务器 Python 包装类"""
    
    # 常量定义
    WEBDAV_SUCCESS = 0
    WEBDAV_ERROR_INVALID_CONFIG = -1
    WEBDAV_ERROR_LOGGER_INIT = -2
    WEBDAV_ERROR_HANDLER_INIT = -3
    WEBDAV_ERROR_SERVER_NOT_FOUND = -1
    WEBDAV_ERROR_SHUTDOWN_FAILED = -2
    WEBDAV_ERROR_BUFFER_TOO_SMALL = -2
    WEBDAV_ERROR_UNSUPPORTED = -1
    
    # 日志级别
    WEBDAV_LOG_DEBUG = 0
    WEBDAV_LOG_INFO = 1
    WEBDAV_LOG_WARN = 2
    WEBDAV_LOG_ERROR = 3
    
    def __init__(self, library_path=None):
        """初始化 WebDAV 服务器包装类"""
        if library_path is None:
            # 自动检测库文件路径
            library_path = self._detect_library_path()
        
        # 加载动态库
        try:
            self.lib = ctypes.CDLL(library_path)
        except OSError as e:
            raise RuntimeError(f"无法加载 WebDAV 库: {e}")
        
        # 设置函数签名
        self._setup_function_signatures()
        
        self.server_id = None
    
    def _detect_library_path(self):
        """自动检测库文件路径"""
        system = platform.system().lower()
        machine = platform.machine().lower()
        
        # 映射架构名称
        arch_map = {
            'x86_64': 'amd64',
            'amd64': 'amd64',
            'arm64': 'arm64',
            'aarch64': 'arm64'
        }
        
        arch = arch_map.get(machine, 'amd64')
        
        # 确定库文件扩展名
        if system == 'windows':
            ext = '.dll'
        elif system == 'darwin':
            ext = '.dylib'
        else:
            ext = '.so'
        
        # 构建库文件名
        lib_name = f"libwebdav_{system}_{arch}{ext}"
        
        # 查找库文件
        search_paths = [
            os.path.join(os.path.dirname(__file__), '..', 'dist', 'lib', lib_name),
            os.path.join('.', 'dist', 'lib', lib_name),
            lib_name
        ]
        
        for path in search_paths:
            if os.path.exists(path):
                return path
        
        # 添加调试信息
        print(f"Debug: 当前系统: {system}")
        print(f"Debug: 当前架构: {machine} -> {arch}")
        print(f"Debug: 查找的库文件: {lib_name}")
        print(f"Debug: 搜索路径:")
        for path in search_paths:
            print(f"  - {path} (存在: {os.path.exists(path)})")
        
        raise FileNotFoundError(f"找不到 WebDAV 库文件: {lib_name}")
    
    def _setup_function_signatures(self):
        """设置函数签名"""
        # webdav_create_server
        self.lib.webdav_create_server.argtypes = [
            c_char_p, c_int, c_char_p, c_char_p, c_char_p,
            c_int, c_char_p, c_char_p, c_char_p,
            c_int, c_int, c_int
        ]
        self.lib.webdav_create_server.restype = c_int
        
        # webdav_start_server
        self.lib.webdav_start_server.argtypes = [c_int]
        self.lib.webdav_start_server.restype = c_int
        
        # webdav_stop_server
        self.lib.webdav_stop_server.argtypes = [c_int]
        self.lib.webdav_stop_server.restype = c_int
        
        # webdav_get_server_info
        self.lib.webdav_get_server_info.argtypes = [c_int, c_char_p, c_int]
        self.lib.webdav_get_server_info.restype = c_int
        
        # webdav_set_log_level
        self.lib.webdav_set_log_level.argtypes = [c_int]
        self.lib.webdav_set_log_level.restype = c_int
        
        # webdav_get_version
        self.lib.webdav_get_version.argtypes = []
        self.lib.webdav_get_version.restype = c_char_p
        
        # webdav_cleanup
        self.lib.webdav_cleanup.argtypes = []
        self.lib.webdav_cleanup.restype = None
        
        # webdav_free_string
        self.lib.webdav_free_string.argtypes = [c_char_p]
        self.lib.webdav_free_string.restype = None
    
    def get_version(self):
        """获取库版本"""
        version_ptr = self.lib.webdav_get_version()
        version = ctypes.string_at(version_ptr).decode('utf-8')
        self.lib.webdav_free_string(version_ptr)
        return version
    
    def set_log_level(self, level):
        """设置日志级别"""
        result = self.lib.webdav_set_log_level(level)
        if result != self.WEBDAV_SUCCESS:
            raise RuntimeError(f"设置日志级别失败: {result}")
    
    def create_server(self, address="127.0.0.1", port=8080, directory="./webdav_root",
                     username=None, password=None, tls=False, cert_file=None,
                     key_file=None, prefix="/", no_password=False,
                     behind_proxy=False, debug=False):
        """创建 WebDAV 服务器"""
        
        # 转换参数
        address_b = address.encode('utf-8') if address else None
        directory_b = directory.encode('utf-8') if directory else None
        username_b = username.encode('utf-8') if username else None
        password_b = password.encode('utf-8') if password else None
        cert_file_b = cert_file.encode('utf-8') if cert_file else None
        key_file_b = key_file.encode('utf-8') if key_file else None
        prefix_b = prefix.encode('utf-8') if prefix else None
        
        self.server_id = self.lib.webdav_create_server(
            address_b, port, directory_b, username_b, password_b,
            1 if tls else 0, cert_file_b, key_file_b, prefix_b,
            1 if no_password else 0, 1 if behind_proxy else 0, 1 if debug else 0
        )
        
        if self.server_id < 0:
            error_msg = {
                self.WEBDAV_ERROR_INVALID_CONFIG: "无效的配置",
                self.WEBDAV_ERROR_LOGGER_INIT: "日志初始化失败",
                self.WEBDAV_ERROR_HANDLER_INIT: "处理器初始化失败"
            }.get(self.server_id, f"未知错误: {self.server_id}")
            raise RuntimeError(f"创建服务器失败: {error_msg}")
        
        return self.server_id
    
    def start_server(self):
        """启动服务器"""
        if self.server_id is None:
            raise RuntimeError("服务器未创建")
        
        result = self.lib.webdav_start_server(self.server_id)
        if result != self.WEBDAV_SUCCESS:
            raise RuntimeError(f"启动服务器失败: {result}")
    
    def stop_server(self):
        """停止服务器"""
        if self.server_id is None:
            return
        
        result = self.lib.webdav_stop_server(self.server_id)
        if result != self.WEBDAV_SUCCESS:
            raise RuntimeError(f"停止服务器失败: {result}")
        
        self.server_id = None
    
    def get_server_info(self):
        """获取服务器信息"""
        if self.server_id is None:
            raise RuntimeError("服务器未创建")
        
        buffer = ctypes.create_string_buffer(256)
        result = self.lib.webdav_get_server_info(self.server_id, buffer, len(buffer))
        
        if result < 0:
            raise RuntimeError(f"获取服务器信息失败: {result}")
        
        return buffer.value.decode('utf-8')
    
    def cleanup(self):
        """清理资源"""
        self.lib.webdav_cleanup()
        self.server_id = None
    
    def __enter__(self):
        """上下文管理器入口"""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """上下文管理器出口"""
        self.stop_server()
        self.cleanup()


def main():
    """主函数示例"""
    print("WebDAV CGO 库 Python 示例")
    print("=========================\n")
    
    try:
        # 创建 WebDAV 服务器实例
        with WebDAVServer() as server:
            # 获取版本信息
            version = server.get_version()
            print(f"库版本: {version}\n")
            
            # 设置日志级别
            print("设置日志级别为 INFO...")
            server.set_log_level(WebDAVServer.WEBDAV_LOG_INFO)
            
            # 创建服务器
            print("创建 WebDAV 服务器...")
            server_id = server.create_server(
                address="127.0.0.1",
                port=8080,
                directory="./webdav_root",
                username="admin",
                password="password",
                debug=True
            )
            print(f"服务器创建成功，ID: {server_id}")
            
            # 获取服务器信息
            info = server.get_server_info()
            print(f"服务器信息: {info}")
            
            # 启动服务器
            print("启动 WebDAV 服务器...")
            server.start_server()
            print("服务器启动成功！")
            print("WebDAV 服务器运行在: http://127.0.0.1:8080/")
            print("用户名: admin")
            print("密码: password")
            print("\n按 Enter 键停止服务器...")
            
            # 等待用户输入
            input()
            
            # 停止服务器
            print("停止服务器...")
            server.stop_server()
            print("服务器已停止")
    
    except Exception as e:
        print(f"错误: {e}")
        return 1
    
    print("资源清理完成")
    return 0


if __name__ == "__main__":
    sys.exit(main()) 