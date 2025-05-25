#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "../webdav.h"

int main() {
    printf("WebDAV CGO 库 C 语言示例\n");
    printf("========================\n\n");

    // 获取库版本
    char* version = webdav_get_version();
    printf("库版本: %s\n\n", version);
    webdav_free_string(version);

    // 设置日志级别为 INFO
    printf("设置日志级别为 INFO...\n");
    int result = webdav_set_log_level(WEBDAV_LOG_INFO);
    if (result != WEBDAV_SUCCESS) {
        printf("设置日志级别失败: %d\n", result);
        return 1;
    }

    // 创建 WebDAV 服务器
    printf("创建 WebDAV 服务器...\n");
    int server_id = webdav_create_server(
        "127.0.0.1",    // 地址
        8080,           // 端口
        "./webdav_root", // 根目录
        "admin",        // 用户名
        "password",     // 密码
        0,              // 不使用 TLS
        NULL,           // 证书文件
        NULL,           // 密钥文件
        "/",            // 前缀
        0,              // 不禁用密码
        0,              // 不在代理后面
        1               // 启用调试
    );

    if (server_id < 0) {
        printf("创建服务器失败，错误码: %d\n", server_id);
        switch (server_id) {
            case WEBDAV_ERROR_INVALID_CONFIG:
                printf("错误: 无效的配置\n");
                break;
            case WEBDAV_ERROR_LOGGER_INIT:
                printf("错误: 日志初始化失败\n");
                break;
            case WEBDAV_ERROR_HANDLER_INIT:
                printf("错误: 处理器初始化失败\n");
                break;
            default:
                printf("错误: 未知错误\n");
                break;
        }
        return 1;
    }

    printf("服务器创建成功，ID: %d\n", server_id);

    // 获取服务器信息
    char info_buffer[256];
    int info_len = webdav_get_server_info(server_id, info_buffer, sizeof(info_buffer));
    if (info_len > 0) {
        printf("服务器信息: %s\n", info_buffer);
    }

    // 启动服务器
    printf("启动 WebDAV 服务器...\n");
    result = webdav_start_server(server_id);
    if (result != WEBDAV_SUCCESS) {
        printf("启动服务器失败: %d\n", result);
        webdav_stop_server(server_id);
        return 1;
    }

    printf("服务器启动成功！\n");
    printf("WebDAV 服务器运行在: http://127.0.0.1:8080/\n");
    printf("用户名: admin\n");
    printf("密码: password\n");
    
    // 给服务器一些时间来启动
    printf("等待服务器完全启动...\n");
    sleep(2);
    
    printf("\n按 Enter 键停止服务器...\n");

    // 等待用户输入
    getchar();

    // 停止服务器
    printf("停止服务器...\n");
    result = webdav_stop_server(server_id);
    if (result != WEBDAV_SUCCESS) {
        printf("停止服务器失败: %d\n", result);
    } else {
        printf("服务器已停止\n");
    }

    // 清理资源
    webdav_cleanup();
    printf("资源清理完成\n");

    return 0;
} 