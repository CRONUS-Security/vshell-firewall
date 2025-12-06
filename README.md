# vshell-firewall

一个灵活、高性能的 TCP 代理服务，支持 HTTP 路径过滤和多端口监听。

## 特性

- 🚀 **高性能** - 高效的 TCP 代理转发
- 🔌 **多端口监听** - 支持同时监听多个端口，各自独立配置
- 🔒 **灵活的路由规则** - 基于路径的访问控制（允许/拒绝）
- 🌍 **GeoIP 支持** - 基于 IP 地理位置的访问控制，可拦截特定国家或地区
- 🔄 **TCP 协议支持** - 支持 raw TCP 后端服务（包括 HTTP 和长连接）
- ⚡ **长连接支持** - 可配置的超时策略，支持长期 TCP 连接
- 🛡️ **恶意连接防护** - 可选的初始超时防止空连接占用资源
- 📊 **详细日志** - 可配置的日志级别和连接跟踪
- 📝 **TOML 配置** - 人性化的配置文件格式

## 支持的后端协议

**当前支持：**
- ✅ **TCP** - 支持所有基于 TCP 的后端服务（包括 HTTP over TCP、长连接等）

**未来计划（TODO）：**
- ⏳ **KCP/UDP** - 基于 KCP 协议的 UDP 通信
- ⏳ **WebSocket** - WebSocket 协议支持

## 架构

```
Client --> vshell-firewall (多端口) --> Backend Services
           Port 8880 -----> Backend:9991
           Port 9880 -----> Backend:9992
           Port 7880 -----> Backend:8000
```

代理服务可以同时监听多个端口，每个端口都有独立的：
- 后端服务地址
- 协议类型（auto/http/tcp）
- 超时配置
- 路由规则

## 快速开始

### 1. 编译

```bash
# 构建二进制文件
make build

# 或者构建带版本信息的
make build-with-version
```

### 2. 配置

复制示例配置文件并编辑：

```bash
cp config.toml.example config.toml
vim config.toml
```

基本配置示例：

```toml
[global]
buffer_size = 32768
log_level = "info"

# GeoIP 配置（可选）
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["US", "EU"]  # 拦截美国和欧洲地区

[[listeners]]
name = "my_proxy"
listen_port = ":8880"
backend_addr = "127.0.0.1:9991"
protocol = "tcp"

[listeners.timeout]
enabled = true
initial_read = 30
connect_backend = 5

[[listeners.routes]]
path = "/admin"
action = "drop"
response = "403"

[[listeners.routes]]
path = "/"
action = "allow"
```

### 3. 运行

```bash
# 直接运行
./build/vshell-firewall

# 指定配置文件
./build/vshell-firewall -config /path/to/config.toml

# 查看版本
./build/vshell-firewall -version
```

### 4. 安装为系统服务

```bash
# 安装二进制文件和 systemd 服务
sudo make install-service

# 启动服务
sudo make start

# 开机自启
sudo make enable
```

## 配置说明

### 全局配置

```toml
[global]
buffer_size = 32768  # 缓冲区大小（字节）
log_level = "info"   # 日志级别：debug, info, warn, error
```

#### GeoIP 配置（可选）

```toml
[global.geoip]
enabled = true                               # 是否启用 GeoIP 检查
database_path = "./GeoLite2-Country.mmdb"    # GeoIP 数据库路径
block_regions = ["US", "EU", "JP"]           # 要拦截的地区列表
```

支持国家代码（如 `US`, `CN`, `GB`）和大洲代码（如 `EU`, `AS`, `NA`）。
详细说明请参阅 [GEOIP.md](GEOIP.md)。

### 监听器配置

每个监听器可以独立配置：

```toml
[[listeners]]
name = "listener_name"           # 监听器名称（用于日志）
listen_port = ":8880"            # 监听端口
backend_addr = "127.0.0.1:9991"  # 后端服务地址
protocol = "tcp"                 # 协议类型：tcp
```

**协议类型说明：**
- `tcp` - TCP 协议（支持所有 TCP 后端，包括 HTTP over TCP 和长连接）
- 未来支持：`kcp/udp`、`websocket`（见上方 TODO 列表）

### 超时配置

```toml
[listeners.timeout]
enabled = true       # 是否启用超时
initial_read = 30    # 初始读取超时（秒），0 = 无限制
connect_backend = 5  # 连接后端超时（秒），0 = 无限制
```

**超时策略：**
- `enabled = true` - 初始读取有超时，数据到达后移除超时（防护 + 长连接）
- `enabled = false` - 完全无超时（纯长连接）

### 路由规则

路由规则按顺序匹配，支持前缀匹配：

```toml
[[listeners.routes]]
path = "/admin"      # 路径（前缀匹配）
action = "drop"      # 动作：drop 或 allow
response = "403"     # drop 时的响应：404, 403, 502, close
```

**响应类型：**
- `404` - 返回 404 Not Found
- `403` - 返回 403 Forbidden
- `502` - 返回 502 Bad Gateway
- `close` - 直接关闭连接（不响应）

**规则示例：**

```toml
# 拒绝特定路径
[[listeners.routes]]
path = "/admin"
action = "drop"
response = "403"

# 允许 API
[[listeners.routes]]
path = "/api"
action = "allow"

# 默认拒绝其他所有请求
[[listeners.routes]]
path = "/"
action = "drop"
response = "404"
```

## 使用场景

### 场景 1: HTTP 反向代理 + 路径过滤

```toml
[[listeners]]
name = "web_proxy"
listen_port = ":8880"
backend_addr = "127.0.0.1:8000"
protocol = "tcp"

[listeners.timeout]
enabled = true
initial_read = 30
connect_backend = 5

[[listeners.routes]]
path = "/slt"
action = "drop"
response = "404"

[[listeners.routes]]
path = "/admin"
action = "drop"
response = "403"

[[listeners.routes]]
path = "/"
action = "allow"
```

### 场景 2: 纯 TCP 长连接转发（无超时）

```toml
[[listeners]]
name = "tcp_longconn"
listen_port = ":9880"
backend_addr = "127.0.0.1:9992"
protocol = "tcp"

[listeners.timeout]
enabled = false  # 完全无超时

[[listeners.routes]]
path = "/"
action = "allow"
```

### 场景 3: GeoIP 地区拦截

```toml
[global]
buffer_size = 32768
log_level = "info"

# 启用 GeoIP，拦截美国和欧洲地区
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["US", "EU"]

[[listeners]]
name = "protected_service"
listen_port = ":8880"
backend_addr = "127.0.0.1:8000"
protocol = "tcp"

[listeners.timeout]
enabled = true
initial_read = 30

[[listeners.routes]]
path = "/"
action = "allow"
```

### 场景 4: 多端口，混合模式

```toml
# HTTP 代理（端口 8880）
[[listeners]]
name = "http_proxy"
listen_port = ":8880"
backend_addr = "127.0.0.1:9991"
protocol = "tcp"

[listeners.timeout]
enabled = true
initial_read = 30

[[listeners.routes]]
path = "/blocked"
action = "drop"
response = "404"

[[listeners.routes]]
path = "/"
action = "allow"

# TCP 长连接（端口 9880）
[[listeners]]
name = "tcp_proxy"
listen_port = ":9880"
backend_addr = "127.0.0.1:9992"
protocol = "tcp"

[listeners.timeout]
enabled = false

[[listeners.routes]]
path = "/"
action = "allow"
```

## Makefile 命令

```bash
make help          # 显示所有可用命令
make build         # 编译二进制文件
make run           # 编译并运行
make test          # 运行测试
make fmt           # 格式化代码
make vet           # 代码检查
make tidy          # 整理依赖

# 安装和服务管理
make install       # 安装到 /usr/local/bin
make install-service  # 安装 systemd 服务
make start         # 启动服务
make stop          # 停止服务
make restart       # 重启服务
make status        # 查看服务状态
make logs          # 查看服务日志
make enable        # 开机自启
make disable       # 禁用自启
make uninstall     # 卸载

# 交叉编译
make build-linux       # Linux amd64
make build-linux-arm64 # Linux arm64
make build-all         # 所有平台

# 清理
make clean         # 清理构建产物
```

## 系统服务管理

安装为服务后：

```bash
# 启动
sudo systemctl start vshell-firewall

# 停止
sudo systemctl stop vshell-firewall

# 重启
sudo systemctl restart vshell-firewall

# 状态
sudo systemctl status vshell-firewall

# 查看日志
sudo journalctl -u vshell-firewall -f

# 开机自启
sudo systemctl enable vshell-firewall

# 禁用自启
sudo systemctl disable vshell-firewall
```

## 日志示例

```
2025/12/06 04:00:00 Loaded config with 2 listener(s)
2025/12/06 04:00:00 All listeners started
2025/12/06 04:00:00 [http_proxy] Listening on :8880, forwarding to 127.0.0.1:9991 (protocol: auto, timeout: true)
2025/12/06 04:00:00 [tcp_proxy] Listening on :9880, forwarding to 127.0.0.1:9992 (protocol: tcp, timeout: false)
2025/12/06 04:00:10 [http_proxy] Blocked request to '/admin' from 192.168.1.100:45678 (response: 403)
2025/12/06 04:00:15 [http_proxy] Forwarding HTTP request: GET /api/data HTTP/1.1 from 192.168.1.101:45679
2025/12/06 04:00:20 [tcp_proxy] Forwarding raw TCP connection from 192.168.1.102:45680
```

## 工作原理

1. **连接建立** - 客户端连接到指定端口
2. **初始超时** - 如果启用，设置初始读取超时（防止空连接）
3. **数据读取** - 读取第一块数据（最多 4KB）
4. **协议处理** - 使用 TCP 协议处理（支持 HTTP over TCP）
5. **路由匹配** - 检测 HTTP 请求并匹配路径规则；纯 TCP 使用默认规则
6. **动作执行** - drop（拒绝）或 allow（转发到后端）
7. **双向转发** - 建立客户端 ↔ 后端的双向流式传输
8. **长连接支持** - 数据传输后移除超时限制

## 依赖

- Go 1.21+
- [github.com/BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML 配置解析

## 开发

```bash
# 格式化代码
make fmt

# 运行检查
make vet

# 整理依赖
make tidy

# 本地测试
make run
```

## 文件说明

- `main.go` - 主程序逻辑
- `config.go` - 配置解析和验证
- `config.toml` - 默认配置文件
- `config.toml.example` - 完整配置示例
- `Makefile` - 构建和部署脚本
- `vshell-firewall.service` - systemd 服务配置

## License

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
