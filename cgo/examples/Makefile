# WebDAV CGO 库示例 Makefile

# 编译器设置
CC = gcc
CFLAGS = -Wall -Wextra -std=c99 -O2
LDFLAGS = -L../dist/lib
INCLUDES = -I../dist/include
STATIC_LIB = ../dist/lib/libwebdav_darwin_arm64.a
DYNAMIC_LIB = ../dist/lib/libwebdav_darwin_arm64.dylib

# macOS 特定的链接选项
LDFLAGS_STATIC = -lpthread -framework CoreFoundation -framework Security
LDFLAGS_DYNAMIC = -lpthread

# 检测操作系统和架构
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# 映射架构名称
ifeq ($(UNAME_M),x86_64)
    ARCH = amd64
else ifeq ($(UNAME_M),arm64)
    ARCH = arm64
else ifeq ($(UNAME_M),aarch64)
    ARCH = arm64
else
    ARCH = amd64
endif

# 映射操作系统名称
ifeq ($(UNAME_S),Linux)
    OS = linux
    LIB_EXT = .so
    LIBS = -lwebdav_$(OS)_$(ARCH) -lpthread -lm
else ifeq ($(UNAME_S),Darwin)
    OS = darwin
    LIB_EXT = .dylib
    LIBS = -lwebdav_$(OS)_$(ARCH) -lpthread
else ifeq ($(UNAME_S),MINGW64_NT)
    OS = windows
    LIB_EXT = .dll
    LIBS = -lwebdav_$(OS)_$(ARCH)
else
    OS = linux
    LIB_EXT = .so
    LIBS = -lwebdav_$(OS)_$(ARCH) -lpthread -lm
endif

# 目标文件
TARGET = c_example
SOURCE = c_example.c

# 默认目标
all: static dynamic daemon

# 编译 C 示例
$(TARGET): $(SOURCE)
	@echo "编译 C 示例程序..."
	@echo "操作系统: $(OS)"
	@echo "架构: $(ARCH)"
	@echo "库扩展名: $(LIB_EXT)"
	$(CC) $(CFLAGS) $(INCLUDES) -o $(TARGET) $(SOURCE) $(LDFLAGS) $(LIBS)
	@echo "编译完成: $(TARGET)"

# 使用静态库编译
static: c_example_static

dynamic: c_example_dynamic

daemon: c_example_daemon

c_example_static: $(SOURCE)
	@echo "使用静态库编译 C 示例程序..."
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $< $(STATIC_LIB) $(LDFLAGS_STATIC)
	@echo "静态编译完成: $@"

c_example_dynamic: $(SOURCE)
	@echo "使用动态库编译 C 示例程序..."
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $< $(DYNAMIC_LIB) $(LDFLAGS_DYNAMIC)
	@echo "动态编译完成: $@"

c_example_daemon: c_example_daemon.c
	@echo "编译守护进程版本..."
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $< $(STATIC_LIB) $(LDFLAGS_STATIC)
	@echo "守护进程编译完成: $@"

# 运行示例
run: $(TARGET)
	@echo "运行 C 示例程序..."
	@mkdir -p webdav_root
	LD_LIBRARY_PATH=../dist/lib:$$LD_LIBRARY_PATH ./$(TARGET)

# 运行 Python 示例
run-python:
	@echo "运行 Python 示例程序..."
	@mkdir -p webdav_root
	python3 python_example.py

# 清理
clean:
	@echo "清理编译文件..."
	rm -f $(TARGET) c_example_static c_example_dynamic c_example_daemon
	rm -rf webdav_root
	@echo "清理完成"

# 帮助信息
help:
	@echo "WebDAV CGO 库示例 Makefile"
	@echo ""
	@echo "可用目标:"
	@echo "  all      - 编译所有版本"
	@echo "  static   - 编译静态库版本"
	@echo "  dynamic  - 编译动态库版本"
	@echo "  daemon   - 编译守护进程版本"
	@echo "  run       - 运行 C 示例程序"
	@echo "  run-python - 运行 Python 示例程序"
	@echo "  clean     - 清理编译文件"
	@echo "  help      - 显示此帮助信息"
	@echo ""
	@echo "当前检测到的平台:"
	@echo "  操作系统: $(OS)"
	@echo "  架构: $(ARCH)"
	@echo "  库文件: libwebdav_$(OS)_$(ARCH)$(LIB_EXT)"

# 声明伪目标
.PHONY: all static dynamic daemon run run-python clean help 