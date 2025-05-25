# 示例程序 CMakeLists.txt

# 创建 webdav_root 目录
file(MAKE_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}/webdav_root)

# C 示例程序源文件
set(C_EXAMPLE_SOURCES c_example.c)
set(C_DAEMON_SOURCES c_example_daemon.c)

# 静态库示例
if(BUILD_STATIC AND TARGET webdav_static)
    # 交互式示例
    add_executable(c_example_static ${C_EXAMPLE_SOURCES})
    target_link_libraries(c_example_static webdav_static)
    
    # 守护进程示例
    add_executable(c_example_daemon_static ${C_DAEMON_SOURCES})
    target_link_libraries(c_example_daemon_static webdav_static)
    
    # 设置输出目录
    set_target_properties(c_example_static c_example_daemon_static
        PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
    )
    
    message(STATUS "Added static library examples: c_example_static, c_example_daemon_static")
endif()

# 动态库示例
if(BUILD_DYNAMIC AND TARGET webdav_dynamic)
    # 交互式示例
    add_executable(c_example_dynamic ${C_EXAMPLE_SOURCES})
    target_link_libraries(c_example_dynamic webdav_dynamic)
    
    # 守护进程示例
    add_executable(c_example_daemon_dynamic ${C_DAEMON_SOURCES})
    target_link_libraries(c_example_daemon_dynamic webdav_dynamic)
    
    # 设置输出目录
    set_target_properties(c_example_dynamic c_example_daemon_dynamic
        PROPERTIES
        RUNTIME_OUTPUT_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
    )
    
    # 复制动态库到构建目录以便运行时找到
    add_custom_command(TARGET c_example_dynamic POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        ${WEBDAV_DYNAMIC_LIB}
        ${CMAKE_CURRENT_BINARY_DIR}
    )
    
    add_custom_command(TARGET c_example_daemon_dynamic POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
        ${WEBDAV_DYNAMIC_LIB}
        ${CMAKE_CURRENT_BINARY_DIR}
    )
    
    message(STATUS "Added dynamic library examples: c_example_dynamic, c_example_daemon_dynamic")
endif()

# 测试目标
if(BUILD_TESTS)
    # 添加测试脚本的复制
    configure_file(
        ${CMAKE_CURRENT_SOURCE_DIR}/quick_test.sh
        ${CMAKE_CURRENT_BINARY_DIR}/quick_test.sh
        COPYONLY
    )
    
    configure_file(
        ${CMAKE_CURRENT_SOURCE_DIR}/test_server.sh
        ${CMAKE_CURRENT_BINARY_DIR}/test_server.sh
        COPYONLY
    )
    
    # 设置脚本可执行权限
    if(UNIX)
        file(COPY ${CMAKE_CURRENT_SOURCE_DIR}/quick_test.sh
             DESTINATION ${CMAKE_CURRENT_BINARY_DIR}
             FILE_PERMISSIONS OWNER_READ OWNER_WRITE OWNER_EXECUTE
                             GROUP_READ GROUP_EXECUTE
                             WORLD_READ WORLD_EXECUTE)
        
        file(COPY ${CMAKE_CURRENT_SOURCE_DIR}/test_server.sh
             DESTINATION ${CMAKE_CURRENT_BINARY_DIR}
             FILE_PERMISSIONS OWNER_READ OWNER_WRITE OWNER_EXECUTE
                             GROUP_READ GROUP_EXECUTE
                             WORLD_READ WORLD_EXECUTE)
    endif()
    
    message(STATUS "Added test scripts to build directory")
endif()

# 自定义目标：运行静态库示例
if(TARGET c_example_static)
    add_custom_target(run_static
        COMMAND ${CMAKE_CURRENT_BINARY_DIR}/c_example_static
        DEPENDS c_example_static
        WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
        COMMENT "Running static library example"
    )
endif()

# 自定义目标：运行动态库示例
if(TARGET c_example_dynamic)
    add_custom_target(run_dynamic
        COMMAND ${CMAKE_CURRENT_BINARY_DIR}/c_example_dynamic
        DEPENDS c_example_dynamic
        WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
        COMMENT "Running dynamic library example"
    )
endif()

# 自定义目标：运行快速测试
if(BUILD_TESTS AND (TARGET c_example_daemon_static OR TARGET c_example_daemon_dynamic))
    # 优先使用静态库版本，如果不存在则使用动态库版本
    if(TARGET c_example_daemon_static)
        set(TEST_EXECUTABLE "./c_example_daemon_static")
        set(TEST_DEPENDS c_example_daemon_static)
    else()
        set(TEST_EXECUTABLE "./c_example_daemon_dynamic")
        set(TEST_DEPENDS c_example_daemon_dynamic)
    endif()
    
    # 修改测试脚本以使用正确的可执行文件
    configure_file(
        ${CMAKE_CURRENT_SOURCE_DIR}/quick_test.sh
        ${CMAKE_CURRENT_BINARY_DIR}/quick_test_cmake.sh
        COPYONLY
    )
    
    # 创建一个修改版本的测试脚本
    file(READ ${CMAKE_CURRENT_SOURCE_DIR}/quick_test.sh QUICK_TEST_CONTENT)
    string(REPLACE "./c_example_daemon" "${TEST_EXECUTABLE}" QUICK_TEST_CONTENT "${QUICK_TEST_CONTENT}")
    file(WRITE ${CMAKE_CURRENT_BINARY_DIR}/quick_test_cmake.sh "${QUICK_TEST_CONTENT}")
    
    # 设置可执行权限
    if(UNIX)
        file(COPY ${CMAKE_CURRENT_BINARY_DIR}/quick_test_cmake.sh
             DESTINATION ${CMAKE_CURRENT_BINARY_DIR}
             FILE_PERMISSIONS OWNER_READ OWNER_WRITE OWNER_EXECUTE
                             GROUP_READ GROUP_EXECUTE
                             WORLD_READ WORLD_EXECUTE)
    endif()
    
    add_custom_target(test
        COMMAND ${CMAKE_CURRENT_BINARY_DIR}/quick_test_cmake.sh
        DEPENDS ${TEST_DEPENDS}
        WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
        COMMENT "Running WebDAV functionality tests"
    )
endif()

# 安装示例程序
if(TARGET c_example_static)
    install(TARGETS c_example_static c_example_daemon_static
        RUNTIME DESTINATION bin
        COMPONENT examples
    )
endif()

if(TARGET c_example_dynamic)
    install(TARGETS c_example_dynamic c_example_daemon_dynamic
        RUNTIME DESTINATION bin
        COMPONENT examples
    )
endif()

# 安装测试脚本
if(BUILD_TESTS)
    install(FILES 
        ${CMAKE_CURRENT_SOURCE_DIR}/quick_test.sh
        ${CMAKE_CURRENT_SOURCE_DIR}/test_server.sh
        DESTINATION bin
        PERMISSIONS OWNER_READ OWNER_WRITE OWNER_EXECUTE
                   GROUP_READ GROUP_EXECUTE
                   WORLD_READ WORLD_EXECUTE
        COMPONENT tests
    )
endif() 