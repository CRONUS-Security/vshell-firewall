# 项目细节

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
