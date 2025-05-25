#!/usr/bin/env python3
"""
简化的 WebDAV CGO 库测试
"""

import ctypes
import os
import platform

def main():
    print("WebDAV CGO 库简化测试")
    print("====================\n")
    
    # 检测库文件路径
    system = platform.system().lower()
    machine = platform.machine().lower()
    
    arch_map = {
        'x86_64': 'amd64',
        'amd64': 'amd64',
        'arm64': 'arm64',
        'aarch64': 'arm64'
    }
    
    arch = arch_map.get(machine, 'amd64')
    
    if system == 'darwin':
        ext = '.dylib'
    else:
        ext = '.so'
    
    lib_name = f"libwebdav_{system}_{arch}{ext}"
    lib_path = f"../dist/lib/{lib_name}"
    
    print(f"系统: {system}")
    print(f"架构: {machine} -> {arch}")
    print(f"库文件: {lib_name}")
    print(f"路径: {lib_path}")
    print(f"存在: {os.path.exists(lib_path)}\n")
    
    if not os.path.exists(lib_path):
        print("错误: 库文件不存在")
        return 1
    
    try:
        # 加载库
        lib = ctypes.CDLL(lib_path)
        print("✓ 库加载成功")
        
        # 设置函数签名
        lib.webdav_get_version.argtypes = []
        lib.webdav_get_version.restype = ctypes.c_char_p
        
        lib.webdav_create_server.argtypes = [
            ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p,
            ctypes.c_char_p, ctypes.c_char_p, ctypes.c_int,
            ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p,
            ctypes.c_int, ctypes.c_int, ctypes.c_int
        ]
        lib.webdav_create_server.restype = ctypes.c_int
        
        lib.webdav_start_server.argtypes = [ctypes.c_int]
        lib.webdav_start_server.restype = ctypes.c_int
        
        lib.webdav_stop_server.argtypes = [ctypes.c_int]
        lib.webdav_stop_server.restype = ctypes.c_int
        
        lib.webdav_cleanup.argtypes = []
        lib.webdav_cleanup.restype = None
        
        # 获取版本
        version_ptr = lib.webdav_get_version()
        version = ctypes.string_at(version_ptr).decode('utf-8')
        print(f"✓ 库版本: {version}")
        
        # 创建服务器
        print("✓ 创建 WebDAV 服务器...")
        server_id = lib.webdav_create_server(
            b"127.0.0.1",      # 地址
            8081,              # 端口
            b"./webdav_root",  # 目录
            b"admin",          # 用户名
            b"password",       # 密码
            0,                 # 不使用 TLS
            None,              # 证书文件
            None,              # 密钥文件
            b"/",              # 前缀
            0,                 # 不禁用密码
            0,                 # 不在代理后面
            1                  # 启用调试
        )
        
        if server_id < 0:
            print(f"✗ 创建服务器失败: {server_id}")
            return 1
        
        print(f"✓ 服务器创建成功，ID: {server_id}")
        
        # 启动服务器
        result = lib.webdav_start_server(server_id)
        if result != 0:
            print(f"✗ 启动服务器失败: {result}")
            return 1
        
        print("✓ 服务器启动成功")
        print("✓ WebDAV 服务器运行在: http://127.0.0.1:8081/")
        print("✓ 用户名: admin, 密码: password")
        
        # 停止服务器
        print("✓ 停止服务器...")
        lib.webdav_stop_server(server_id)
        
        # 清理资源
        lib.webdav_cleanup()
        print("✓ 资源清理完成")
        
        print("\n🎉 所有测试通过！")
        return 0
        
    except Exception as e:
        print(f"✗ 错误: {e}")
        return 1

if __name__ == "__main__":
    exit(main()) 