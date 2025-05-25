#!/bin/bash

echo "WebDAV 服务器快速测试"
echo "==================="

# 启动服务器（后台运行）
echo "启动 WebDAV 服务器..."
./c_example_daemon > server.log 2>&1 &
SERVER_PID=$!
echo "服务器 PID: $SERVER_PID"

# 等待服务器启动
echo "等待服务器启动..."
sleep 3

# 测试基本连接
echo "测试基本连接..."
curl -q -s --user admin:password http://127.0.0.1:8080/ > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ 基本连接成功"
else
    echo "❌ 基本连接失败"
fi

# 测试 WebDAV PROPFIND
echo "测试 WebDAV PROPFIND..."
curl -q -s -X PROPFIND \
     -H "Depth: 1" \
     -H "Content-Type: text/xml" \
     --user admin:password \
     http://127.0.0.1:8080/ > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ WebDAV PROPFIND 成功"
else
    echo "❌ WebDAV PROPFIND 失败"
fi

# 测试文件上传
echo "测试文件上传..."
echo "Hello WebDAV!" > test_upload.txt
curl -q -s -T test_upload.txt \
     --user admin:password \
     http://127.0.0.1:8080/test_upload.txt > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ 文件上传成功"
else
    echo "❌ 文件上传失败"
fi

# 测试文件下载
echo "测试文件下载..."
curl -q -s --user admin:password \
     http://127.0.0.1:8080/test_upload.txt > downloaded.txt
if [ $? -eq 0 ] && [ -f downloaded.txt ]; then
    echo "✅ 文件下载成功"
    echo "下载内容: $(cat downloaded.txt)"
else
    echo "❌ 文件下载失败"
fi

# 清理
echo "停止服务器..."
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
rm -f test_upload.txt downloaded.txt

echo "测试完成！"
echo "详细日志请查看 server.log" 