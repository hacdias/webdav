#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <signal.h>
#include "../webdav.h"

static int server_id = -1;
static volatile int running = 1;

void signal_handler(int sig) {
    printf("\n收到信号 %d，正在停止服务器...\n", sig);
    running = 0;
}

int main() {
    printf("WebDAV CGO 库守护进程示例\n");
    printf("========================\n\n");

    // 设置信号处理器
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

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
    server_id = webdav_create_server(
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
        0               // 不启用调试（减少日志输出）
    );

    if (server_id < 0) {
        printf("创建服务器失败，错误码: %d\n", server_id);
        return 1;
    }

    printf("服务器创建成功，ID: %d\n", server_id);

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
    printf("按 Ctrl+C 停止服务器\n\n");

    // 给服务器一些时间来启动
    sleep(1);

    // 主循环
    while (running) {
        sleep(1);
    }

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