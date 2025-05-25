package main

/*
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
	"unsafe"

	"github.com/hacdias/webdav/v5/lib"
	"go.uber.org/zap"
)

// 全局变量管理服务器实例
var (
	servers = make(map[int]*http.Server)
	handlers = make(map[int]http.Handler)
	serverConfigs = make(map[int]*lib.Config)
	serverMutex sync.RWMutex
	nextServerID = 1
)

// WebDAVConfig C 结构体对应的 Go 结构体
type WebDAVConfig struct {
	Address     string
	Port        int
	Directory   string
	Username    string
	Password    string
	TLS         bool
	CertFile    string
	KeyFile     string
	Prefix      string
	NoPassword  bool
	BehindProxy bool
	Debug       bool
}

//export webdav_create_server
func webdav_create_server(
	address *C.char,
	port C.int,
	directory *C.char,
	username *C.char,
	password *C.char,
	tls C.int,
	cert_file *C.char,
	key_file *C.char,
	prefix *C.char,
	no_password C.int,
	behind_proxy C.int,
	debug C.int,
) C.int {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	// 转换 C 字符串到 Go 字符串
	config := &lib.Config{
		UserPermissions: lib.UserPermissions{
			Directory: C.GoString(directory),
			Permissions: lib.Permissions{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			},
			RulesBehavior: lib.RulesOverwrite,
		},
		Address:     C.GoString(address),
		Port:        int(port),
		TLS:         int(tls) != 0,
		Cert:        C.GoString(cert_file),
		Key:         C.GoString(key_file),
		Prefix:      C.GoString(prefix),
		NoPassword:  int(no_password) != 0,
		BehindProxy: int(behind_proxy) != 0,
		Debug:       int(debug) != 0,
		Log: lib.Log{
			Format:  "console",
			Colors:  true,
			Outputs: []string{"stderr"},
		},
	}

	// 如果提供了用户名和密码，添加用户
	if username != nil && password != nil {
		user := lib.User{
			UserPermissions: lib.UserPermissions{
				Directory: config.UserPermissions.Directory,
				Permissions: lib.Permissions{
					Create: true,
					Read:   true,
					Update: true,
					Delete: true,
				},
				RulesBehavior: lib.RulesOverwrite,
			},
			Username: C.GoString(username),
			Password: C.GoString(password),
		}
		config.Users = []lib.User{user}
	}

	// 设置默认值
	if config.Address == "" {
		config.Address = "0.0.0.0"
	}
	if config.Port == 0 {
		config.Port = 6065
	}
	if config.UserPermissions.Directory == "" {
		config.UserPermissions.Directory = "."
	}
	if config.Prefix == "" {
		config.Prefix = "/"
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		// 添加调试信息
		fmt.Printf("Config validation error: %v\n", err)
		fmt.Printf("Directory: %s\n", config.UserPermissions.Directory)
		fmt.Printf("Address: %s\n", config.Address)
		fmt.Printf("Port: %d\n", config.Port)
		return -1
	}

	// 设置日志
	logger, err := config.GetLogger()
	if err != nil {
		return -2
	}
	zap.ReplaceGlobals(logger)

	// 创建处理器
	handler, err := lib.NewHandler(config)
	if err != nil {
		return -3
	}

	// 创建服务器
	addr := fmt.Sprintf("%s:%d", config.Address, config.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	serverID := nextServerID
	nextServerID++

	servers[serverID] = server
	handlers[serverID] = handler
	
	// 存储配置信息以便在启动时使用
	serverConfigs[serverID] = config

	return C.int(serverID)
}

//export webdav_start_server
func webdav_start_server(server_id C.int) C.int {
	serverMutex.RLock()
	server, exists := servers[int(server_id)]
	config, configExists := serverConfigs[int(server_id)]
	serverMutex.RUnlock()

	if !exists || !configExists {
		fmt.Printf("Server ID %d not found\n", int(server_id))
		return -1
	}

	fmt.Printf("Starting server on %s\n", server.Addr)
	fmt.Printf("TLS enabled: %v\n", config.TLS)
	fmt.Printf("Directory: %s\n", config.UserPermissions.Directory)

	go func() {
		var err error
		if config.TLS {
			fmt.Printf("Starting TLS server with cert: %s, key: %s\n", config.Cert, config.Key)
			err = server.ListenAndServeTLS(config.Cert, config.Key)
		} else {
			fmt.Printf("Starting HTTP server on %s\n", server.Addr)
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
			zap.L().Error("server error", zap.Error(err))
		}
	}()

	// 给服务器一点时间启动
	time.Sleep(100 * time.Millisecond)
	return 0
}

//export webdav_stop_server
func webdav_stop_server(server_id C.int) C.int {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	server, exists := servers[int(server_id)]
	if !exists {
		return -1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return -2
	}

	delete(servers, int(server_id))
	delete(handlers, int(server_id))
	delete(serverConfigs, int(server_id))

	return 0
}

//export webdav_get_server_info
func webdav_get_server_info(server_id C.int, info_buffer *C.char, buffer_size C.int) C.int {
	serverMutex.RLock()
	server, exists := servers[int(server_id)]
	serverMutex.RUnlock()

	if !exists {
		return -1
	}

	info := fmt.Sprintf("Server Address: %s, Status: Running", server.Addr)
	infoBytes := []byte(info)

	if len(infoBytes) >= int(buffer_size) {
		return -2 // 缓冲区太小
	}

	// 将信息复制到 C 缓冲区
	infoPtr := C.CString(info)
	defer C.free(unsafe.Pointer(infoPtr))
	
	// 使用 C.memcpy 复制数据
	C.memcpy(unsafe.Pointer(info_buffer), unsafe.Pointer(infoPtr), C.size_t(len(infoBytes)+1))
	
	return C.int(len(infoBytes))
}

//export webdav_set_log_level
func webdav_set_log_level(level C.int) C.int {
	var zapLevel zap.AtomicLevel

	switch int(level) {
	case 0: // DEBUG
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case 1: // INFO
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case 2: // WARN
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case 3: // ERROR
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return -1
	}

	// 重新配置全局日志器
	config := zap.NewProductionConfig()
	config.Level = zapLevel
	logger, err := config.Build()
	if err != nil {
		return -2
	}

	zap.ReplaceGlobals(logger)
	return 0
}

//export webdav_add_user
func webdav_add_user(server_id C.int, username *C.char, password *C.char, directory *C.char) C.int {
	// 注意：这个功能需要重新设计处理器，因为当前的处理器在创建时就固定了用户
	// 这里返回不支持的操作
	return -1
}

//export webdav_remove_user
func webdav_remove_user(server_id C.int, username *C.char) C.int {
	// 注意：这个功能需要重新设计处理器，因为当前的处理器在创建时就固定了用户
	// 这里返回不支持的操作
	return -1
}

//export webdav_get_version
func webdav_get_version() *C.char {
	version := "WebDAV CGO Library v1.0.0"
	return C.CString(version)
}

//export webdav_cleanup
func webdav_cleanup() {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	// 停止所有服务器
	for id, server := range servers {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		server.Shutdown(ctx)
		cancel()
		delete(servers, id)
		delete(handlers, id)
		delete(serverConfigs, id)
	}
}

// 辅助函数：将 Go 字符串转换为 C 字符串（调用者负责释放内存）
func goStringToCString(s string) *C.char {
	return C.CString(s)
}

// 辅助函数：释放 C 字符串内存
//export webdav_free_string
func webdav_free_string(str *C.char) {
	C.free(unsafe.Pointer(str))
}

func main() {
	// 这个函数是必需的，但不会被调用
	// CGO 导出的函数将作为库函数使用
} 