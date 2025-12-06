# Makefile for vshell-firewall

# 变量定义
BINARY_NAME=vshell-firewall
BUILD_DIR=build
SOURCE_FILES=$(wildcard *.go)
INSTALL_PATH=/usr/local/bin
SERVICE_FILE=vshell-firewall.service
SERVICE_PATH=/etc/systemd/system

# Go 编译参数
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
LDFLAGS=-ldflags "-s -w"

# 版本信息
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 默认目标
.PHONY: all
all: clean build

# 编译
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_FILES)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# 编译（带版本信息）
.PHONY: build-with-version
build-with-version:
	@echo "Building $(BINARY_NAME) with version info..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) $(SOURCE_FILES)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# 交叉编译 Linux amd64
.PHONY: build-linux
build-linux:
	@echo "Building for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(SOURCE_FILES)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# 交叉编译 Linux arm64
.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for Linux arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(SOURCE_FILES)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

# 编译所有平台
.PHONY: build-all
build-all: build-linux build-linux-arm64
	@echo "All builds complete"

# 运行
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# 测试
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# 代码格式化
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# 代码检查
.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "Vet complete"

# 代码整理
.PHONY: tidy
tidy:
	@echo "Tidying module dependencies..."
	go mod tidy
	@echo "Tidy complete"

# 安装到系统
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Install complete"

# 安装并配置 systemd 服务
.PHONY: install-service
install-service: install
	@echo "Installing systemd service..."
	@if [ -f $(SERVICE_FILE) ]; then \
		sudo cp $(SERVICE_FILE) $(SERVICE_PATH)/; \
		sudo systemctl daemon-reload; \
		echo "Service installed. Use 'sudo systemctl start $(BINARY_NAME)' to start"; \
	else \
		echo "Warning: $(SERVICE_FILE) not found"; \
	fi

# 启动服务
.PHONY: start
start:
	@echo "Starting $(BINARY_NAME) service..."
	@sudo systemctl start $(BINARY_NAME)
	@sudo systemctl status $(BINARY_NAME) --no-pager

# 停止服务
.PHONY: stop
stop:
	@echo "Stopping $(BINARY_NAME) service..."
	@sudo systemctl stop $(BINARY_NAME)

# 重启服务
.PHONY: restart
restart:
	@echo "Restarting $(BINARY_NAME) service..."
	@sudo systemctl restart $(BINARY_NAME)
	@sudo systemctl status $(BINARY_NAME) --no-pager

# 查看服务状态
.PHONY: status
status:
	@sudo systemctl status $(BINARY_NAME) --no-pager

# 查看服务日志
.PHONY: logs
logs:
	@sudo journalctl -u $(BINARY_NAME) -f

# 启用开机自启
.PHONY: enable
enable:
	@echo "Enabling $(BINARY_NAME) service on boot..."
	@sudo systemctl enable $(BINARY_NAME)

# 禁用开机自启
.PHONY: disable
disable:
	@echo "Disabling $(BINARY_NAME) service on boot..."
	@sudo systemctl disable $(BINARY_NAME)

# 卸载
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo systemctl stop $(BINARY_NAME) 2>/dev/null || true
	@sudo systemctl disable $(BINARY_NAME) 2>/dev/null || true
	@sudo rm -f $(SERVICE_PATH)/$(SERVICE_FILE)
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo systemctl daemon-reload
	@echo "Uninstall complete"

# 清理编译产物
.PHONY: clean
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# 完全清理
.PHONY: distclean
distclean: clean
	@echo "Removing all generated files..."
	@go clean -cache -modcache -testcache
	@echo "Distclean complete"

# 显示帮助
.PHONY: help
help:
	@echo "vshell-firewall Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              - Clean and build (default)"
	@echo "  build            - Build the binary"
	@echo "  build-with-version - Build with version information"
	@echo "  build-linux      - Cross compile for Linux amd64"
	@echo "  build-linux-arm64 - Cross compile for Linux arm64"
	@echo "  build-all        - Build for all platforms"
	@echo "  run              - Build and run the proxy"
	@echo "  test             - Run tests"
	@echo "  fmt              - Format source code"
	@echo "  vet              - Run go vet"
	@echo "  tidy             - Tidy module dependencies"
	@echo "  install          - Install binary to $(INSTALL_PATH)"
	@echo "  install-service  - Install and configure systemd service"
	@echo "  start            - Start the systemd service"
	@echo "  stop             - Stop the systemd service"
	@echo "  restart          - Restart the systemd service"
	@echo "  status           - Show service status"
	@echo "  logs             - Show service logs (follow mode)"
	@echo "  enable           - Enable service on boot"
	@echo "  disable          - Disable service on boot"
	@echo "  uninstall        - Uninstall binary and service"
	@echo "  clean            - Clean build directory"
	@echo "  distclean        - Clean all generated files"
	@echo "  help             - Show this help message"
	@echo ""
