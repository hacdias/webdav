#ifndef WEBDAV_H
#define WEBDAV_H

#ifdef __cplusplus
extern "C" {
#endif

// 返回值定义
#define WEBDAV_SUCCESS 0
#define WEBDAV_ERROR_INVALID_CONFIG -1
#define WEBDAV_ERROR_LOGGER_INIT -2
#define WEBDAV_ERROR_HANDLER_INIT -3
#define WEBDAV_ERROR_SERVER_NOT_FOUND -1
#define WEBDAV_ERROR_SHUTDOWN_FAILED -2
#define WEBDAV_ERROR_BUFFER_TOO_SMALL -2
#define WEBDAV_ERROR_UNSUPPORTED -1

// 日志级别定义
#define WEBDAV_LOG_DEBUG 0
#define WEBDAV_LOG_INFO 1
#define WEBDAV_LOG_WARN 2
#define WEBDAV_LOG_ERROR 3

/**
 * 创建 WebDAV 服务器
 * 
 * @param address 服务器监听地址 (例如: "0.0.0.0")
 * @param port 服务器监听端口 (例如: 6065)
 * @param directory 服务器根目录路径
 * @param username 用户名 (可以为 NULL，表示无认证)
 * @param password 密码 (可以为 NULL，表示无认证)
 * @param tls 是否启用 TLS (0=否, 1=是)
 * @param cert_file TLS 证书文件路径 (TLS 启用时必需)
 * @param key_file TLS 私钥文件路径 (TLS 启用时必需)
 * @param prefix URL 前缀 (例如: "/webdav")
 * @param no_password 是否禁用密码检查 (0=否, 1=是)
 * @param behind_proxy 是否在代理后面 (0=否, 1=是)
 * @param debug 是否启用调试模式 (0=否, 1=是)
 * 
 * @return 服务器 ID (>0) 或错误码 (<0)
 */
int webdav_create_server(
    const char* address,
    int port,
    const char* directory,
    const char* username,
    const char* password,
    int tls,
    const char* cert_file,
    const char* key_file,
    const char* prefix,
    int no_password,
    int behind_proxy,
    int debug
);

/**
 * 启动 WebDAV 服务器
 * 
 * @param server_id 服务器 ID
 * @return 0 成功，负数表示错误
 */
int webdav_start_server(int server_id);

/**
 * 停止 WebDAV 服务器
 * 
 * @param server_id 服务器 ID
 * @return 0 成功，负数表示错误
 */
int webdav_stop_server(int server_id);

/**
 * 获取服务器信息
 * 
 * @param server_id 服务器 ID
 * @param info_buffer 信息缓冲区
 * @param buffer_size 缓冲区大小
 * @return 信息长度 (>0) 或错误码 (<0)
 */
int webdav_get_server_info(int server_id, char* info_buffer, int buffer_size);

/**
 * 设置日志级别
 * 
 * @param level 日志级别 (WEBDAV_LOG_DEBUG, WEBDAV_LOG_INFO, WEBDAV_LOG_WARN, WEBDAV_LOG_ERROR)
 * @return 0 成功，负数表示错误
 */
int webdav_set_log_level(int level);

/**
 * 添加用户 (当前版本不支持)
 * 
 * @param server_id 服务器 ID
 * @param username 用户名
 * @param password 密码
 * @param directory 用户目录
 * @return 0 成功，负数表示错误
 */
int webdav_add_user(int server_id, const char* username, const char* password, const char* directory);

/**
 * 删除用户 (当前版本不支持)
 * 
 * @param server_id 服务器 ID
 * @param username 用户名
 * @return 0 成功，负数表示错误
 */
int webdav_remove_user(int server_id, const char* username);

/**
 * 获取库版本信息
 * 
 * @return 版本字符串 (调用者需要使用 webdav_free_string 释放内存)
 */
char* webdav_get_version(void);

/**
 * 清理所有资源
 */
void webdav_cleanup(void);

/**
 * 释放字符串内存
 * 
 * @param str 要释放的字符串指针
 */
void webdav_free_string(char* str);

#ifdef __cplusplus
}
#endif

#endif // WEBDAV_H 