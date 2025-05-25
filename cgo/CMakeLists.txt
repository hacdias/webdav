cmake_minimum_required(VERSION 3.16)
project(WebDAV_CGO 
    VERSION 1.0.0
    DESCRIPTION "WebDAV CGO Library - C bindings for Go WebDAV server"
    LANGUAGES C)

# 设置 C 标准
set(CMAKE_C_STANDARD 99)
set(CMAKE_C_STANDARD_REQUIRED ON)

# 设置构建类型
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

# 编译选项
set(CMAKE_C_FLAGS_DEBUG "-g -O0 -Wall -Wextra")
set(CMAKE_C_FLAGS_RELEASE "-O2 -Wall -Wextra -DNDEBUG")

# 检测操作系统和架构
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    set(OS_NAME "linux")
elseif(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
    set(OS_NAME "darwin")
elseif(CMAKE_SYSTEM_NAME STREQUAL "Windows")
    set(OS_NAME "windows")
else()
    set(OS_NAME "unknown")
endif()

if(CMAKE_SYSTEM_PROCESSOR MATCHES "x86_64|AMD64")
    set(ARCH_NAME "amd64")
elseif(CMAKE_SYSTEM_PROCESSOR MATCHES "arm64|aarch64")
    set(ARCH_NAME "arm64")
else()
    set(ARCH_NAME "unknown")
endif()

# 库文件名
set(LIB_NAME "webdav_${OS_NAME}_${ARCH_NAME}")

# 查找预构建的库文件
set(WEBDAV_LIB_DIR "${CMAKE_CURRENT_SOURCE_DIR}/dist/lib")
set(WEBDAV_INCLUDE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/dist/include")

# 静态库路径
if(CMAKE_SYSTEM_NAME STREQUAL "Windows")
    set(WEBDAV_STATIC_LIB "${WEBDAV_LIB_DIR}/lib${LIB_NAME}.a")
    set(WEBDAV_DYNAMIC_LIB "${WEBDAV_LIB_DIR}/lib${LIB_NAME}.dll")
else()
    set(WEBDAV_STATIC_LIB "${WEBDAV_LIB_DIR}/lib${LIB_NAME}.a")
    if(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
        set(WEBDAV_DYNAMIC_LIB "${WEBDAV_LIB_DIR}/lib${LIB_NAME}.dylib")
    else()
        set(WEBDAV_DYNAMIC_LIB "${WEBDAV_LIB_DIR}/lib${LIB_NAME}.so")
    endif()
endif()

# 选项
option(BUILD_STATIC "Build static library examples" ON)
option(BUILD_DYNAMIC "Build dynamic library examples" ON)
option(BUILD_EXAMPLES "Build example programs" ON)
option(BUILD_TESTS "Build test programs" ON)

# 检查库文件是否存在
if(NOT EXISTS ${WEBDAV_INCLUDE_DIR}/webdav.h)
    message(WARNING "WebDAV header file not found at ${WEBDAV_INCLUDE_DIR}/webdav.h")
    message(WARNING "Please run './build.sh' first to build the Go library")
endif()

# 创建导入的静态库目标
if(EXISTS ${WEBDAV_STATIC_LIB})
    add_library(webdav_static STATIC IMPORTED)
    set_target_properties(webdav_static PROPERTIES
        IMPORTED_LOCATION ${WEBDAV_STATIC_LIB}
        INTERFACE_INCLUDE_DIRECTORIES ${WEBDAV_INCLUDE_DIR}
    )
    
    # 平台特定的链接库
    if(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
        target_link_libraries(webdav_static INTERFACE
            "-framework CoreFoundation"
            "-framework Security"
            pthread
        )
    elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")
        target_link_libraries(webdav_static INTERFACE
            pthread
            m
        )
    elseif(CMAKE_SYSTEM_NAME STREQUAL "Windows")
        target_link_libraries(webdav_static INTERFACE
            ws2_32
            winmm
        )
    endif()
    
    message(STATUS "Found WebDAV static library: ${WEBDAV_STATIC_LIB}")
else()
    message(WARNING "WebDAV static library not found: ${WEBDAV_STATIC_LIB}")
    set(BUILD_STATIC OFF)
endif()

# 创建导入的动态库目标
if(EXISTS ${WEBDAV_DYNAMIC_LIB})
    add_library(webdav_dynamic SHARED IMPORTED)
    set_target_properties(webdav_dynamic PROPERTIES
        IMPORTED_LOCATION ${WEBDAV_DYNAMIC_LIB}
        INTERFACE_INCLUDE_DIRECTORIES ${WEBDAV_INCLUDE_DIR}
    )
    
    # 动态库通常只需要 pthread
    if(NOT CMAKE_SYSTEM_NAME STREQUAL "Windows")
        target_link_libraries(webdav_dynamic INTERFACE pthread)
    endif()
    
    message(STATUS "Found WebDAV dynamic library: ${WEBDAV_DYNAMIC_LIB}")
else()
    message(WARNING "WebDAV dynamic library not found: ${WEBDAV_DYNAMIC_LIB}")
    set(BUILD_DYNAMIC OFF)
endif()

# 添加示例子目录
if(BUILD_EXAMPLES)
    add_subdirectory(examples)
endif()

# 安装规则
if(BUILD_STATIC AND EXISTS ${WEBDAV_STATIC_LIB})
    install(FILES ${WEBDAV_STATIC_LIB}
        DESTINATION lib
        COMPONENT static
    )
endif()

if(BUILD_DYNAMIC AND EXISTS ${WEBDAV_DYNAMIC_LIB})
    install(FILES ${WEBDAV_DYNAMIC_LIB}
        DESTINATION lib
        COMPONENT dynamic
    )
endif()

if(EXISTS ${WEBDAV_INCLUDE_DIR}/webdav.h)
    install(FILES ${WEBDAV_INCLUDE_DIR}/webdav.h
        DESTINATION include
        COMPONENT headers
    )
endif()

# 创建 pkg-config 文件
if(EXISTS ${WEBDAV_INCLUDE_DIR}/webdav.h)
    configure_file(
        "${CMAKE_CURRENT_SOURCE_DIR}/cmake/webdav.pc.in"
        "${CMAKE_CURRENT_BINARY_DIR}/webdav.pc"
        @ONLY
    )
    
    install(FILES "${CMAKE_CURRENT_BINARY_DIR}/webdav.pc"
        DESTINATION lib/pkgconfig
        COMPONENT pkgconfig
    )
endif()

# 打印配置信息
message(STATUS "=== WebDAV CGO Library Configuration ===")
message(STATUS "Build type: ${CMAKE_BUILD_TYPE}")
message(STATUS "OS: ${OS_NAME}")
message(STATUS "Architecture: ${ARCH_NAME}")
message(STATUS "Library name: ${LIB_NAME}")
message(STATUS "Build static examples: ${BUILD_STATIC}")
message(STATUS "Build dynamic examples: ${BUILD_DYNAMIC}")
message(STATUS "Build examples: ${BUILD_EXAMPLES}")
message(STATUS "Build tests: ${BUILD_TESTS}")
message(STATUS "=========================================") 