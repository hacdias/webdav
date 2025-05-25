#!/bin/bash

echo "WebDAV 服务器连接测试"
echo "===================="

# 启动服务器
echo "1. 启动 WebDAV 服务器..."
./c_example_static &
SERVER_PID=$!
echo "服务器 PID: $SERVER_PID"

# 等待服务器启动
echo "2. 等待服务器启动..."
sleep 3

# 检查端口是否在监听
echo "3. 检查端口 8080 是否在监听..."
if command -v lsof >/dev/null 2>&1; then
    lsof -i :8080
else
    netstat -an | grep 8080
fi

# 测试基本连接
echo "4. 测试基本 HTTP 连接..."
curl -v --connect-timeout 5 http://127.0.0.1:8080/ 2>&1 | head -20

# 测试 WebDAV PROPFIND 请求
echo "5. 测试 WebDAV PROPFIND 请求..."
curl -v -X PROPFIND \
     -H "Depth: 1" \
     -H "Content-Type: text/xml" \
     --user admin:password \
     --connect-timeout 5 \
     http://127.0.0.1:8080/ 2>&1 | head -20

# 测试文件访问
echo "6. 测试文件访问..."
curl -v --user admin:password \
     --connect-timeout 5 \
     http://127.0.0.1:8080/test.txt 2>&1 | head -20

# 清理
echo "7. 停止服务器..."
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo "测试完成！" 