# vshell-firewall

VShell 基础设施指纹隐藏与访问控制代理 - 通过流量过滤和指纹阻断，保护 VShell C2 基础设施免受公开扫描和威胁情报追踪。

## 项目背景

随着 NVISO 等安全厂商公开 [VShell 后渗透工具分析报告](https://www.nviso.eu/blog/nviso-analyzes-vshell-post-exploitation-tool)，VShell 的网络特征指纹、通信模式和基础设施标识已被广泛公开。威胁情报机构如 Team Cymru、ThreatFox 等正在全球范围内追踪和标记 VShell C2 服务器。

**vshell-firewall** 是一个专门设计的反向代理防护系统，旨在：
- 🛡️ **隐藏 VShell 指纹** - 阻断已知的指纹识别请求，防止 C2 基础设施被自动化扫描发现
- 🔒 **访问控制** - 基于地理位置、时间窗口、路径特征的多维度访问过滤
- 🎭 **流量混淆** - 通过路由规则和响应定制，混淆真实的后端服务特征
- 📊 **威胁感知** - 记录所有扫描和探测行为，提供威胁情报反馈

## 核心特性

### 🛡️ 指纹隐藏与防护
- **网络特征过滤** - 拦截针对 VShell stager（Windows/Linux/MacOS）的指纹探测
- **行为模式隐藏** - 阻断 beaconing 活动的特征识别请求
- **路径级访问控制** - 精细化的 HTTP 路径白名单/黑名单机制
- **自定义响应** - 为扫描请求返回伪装响应，混淆真实服务指纹

### 🌍 地理位置与时间控制
- **GeoIP 过滤** - 拦截来自特定国家/地区的威胁情报扫描（如美国、欧洲等威胁追踪热点）
- **时间窗口限制** - 仅在指定时间段内允许连接，降低暴露时长
- **时区自定义** - 支持全球时区配置，匹配目标地区的活动时间

### 🚀 高性能代理架构
- **多端口监听** - 同时保护多个 VShell 实例，各自独立配置
- **TCP 全协议支持** - 支持 VShell 的所有 TCP 通信模式（HTTP over TCP、长连接等）
- **智能超时策略** - 防护空连接攻击，同时保持合法长连接稳定
- **高效转发** - 低延迟的透明代理，不影响 VShell 正常通信性能

### 📊 审计与监控
- **详细连接日志** - 记录所有连接尝试、地理位置、匹配规则
- **威胁行为追踪** - 识别并记录扫描器特征和探测模式
- **双重日志输出** - 同时输出到控制台和文件，支持持久化分析

## 支持的后端协议

**当前支持：**
- ✅ **TCP** - 支持所有基于 TCP 的后端服务（包括**TCP长连接**、**WebSocket**等）

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

## 部署指南

### 1. 编译

```bash
# 克隆项目
git clone https://github.com/CRONUS-Security/vshell-firewall.git
cd vshell-firewall

# 构建二进制文件
make build

# 或构建带版本信息
make build-with-version
```

### 2. 准备 GeoIP 数据库

为了启用地理位置过滤（强烈推荐），需要下载 MaxMind GeoIP2 数据库：

```bash
# 使用提供的脚本自动下载
./download-geoip.sh

# 或手动下载并放置到项目目录
# 下载地址：https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
```

详细的 GeoIP 配置说明请参阅 [GEOIP.md](./docs/GEOIP.md)。

### 3. 配置防护策略

复制示例配置并根据需求定制：

```bash
cp config.toml.example config.toml
vim config.toml
```

**典型防护配置示例：**

```toml
[global]
buffer_size = 32768
log_level = "info"
log_file = "./vshell-firewall.log"  # 记录所有访问尝试

# 地理位置防护：拦截威胁情报热点地区
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["US", "EU", "JP"]  # 美国、欧洲、日本的威胁追踪活跃

# 时间窗口：仅在目标活动时间开放（UTC+8 示例）
[global.time_window]
enabled = true
timezone = "Asia/Shanghai"
start_time = "09:00"  # 上午 9 点开始
end_time = "18:00"    # 下午 6 点结束

# VShell C2 监听器
[[listeners]]
name = "vshell_c2_main"
listen_port = ":443"              # 对外端口（伪装成 HTTPS）
backend_addr = "127.0.0.1:9991"   # VShell 实际监听端口
protocol = "tcp"

[listeners.timeout]
enabled = true
initial_read = 30    # 30 秒初始超时，防止空连接扫描
connect_backend = 5  # 5 秒后端连接超时

# 路径防护规则（针对 HTTP 探测）
[[listeners.routes]]
path = "/favicon.ico"
action = "drop"
response = "404"  # 拦截常见的指纹探测请求

[[listeners.routes]]
path = "/robots.txt"
action = "drop"
response = "404"

[[listeners.routes]]
path = "/"
action = "allow"  # 允许根路径（VShell 正常通信）
```

### 4. 运行与部署

**手动运行：**

```bash
# 前台运行（测试）
./build/vshell-firewall

# 指定配置文件
./build/vshell-firewall -config /path/to/config.toml

# 查看版本信息
./build/vshell-firewall -version
```

**系统服务部署（生产环境推荐）：**

```bash
# 安装为 systemd 服务
sudo make install-service

# 启动服务
sudo systemctl start vshell-firewall

# 设置开机自启
sudo systemctl enable vshell-firewall

# 查看运行状态
sudo systemctl status vshell-firewall

# 查看实时日志
sudo journalctl -u vshell-firewall -f
```

## 配置详解

### 全局配置

```toml
[global]
buffer_size = 32768  # TCP 缓冲区大小（字节）
log_level = "info"   # 日志级别：debug, info, warn, error
log_file = "./vshell-firewall.log"  # 日志文件路径（强烈建议配置）
```

**日志记录：**
- 配置 `log_file` 后，日志会**同时**输出到控制台和文件
- 包含时间戳、来源 IP、地理位置、匹配规则等详细信息
- 用于事后审计和威胁情报分析

#### GeoIP 地理位置防护（核心功能）

```toml
[global.geoip]
enabled = true                               # 强烈建议启用
database_path = "./GeoLite2-Country.mmdb"    # GeoIP2 数据库路径
block_regions = ["US", "EU", "JP", "GB"]     # 拦截的国家/地区列表
```

**推荐拦截地区：**
- `US` - 美国（大量威胁情报机构和安全研究机构）
- `EU` - 欧洲（NVISO、Team Cymru 等追踪活跃地区）
- `GB` - 英国（网络安全研究热点）
- `JP` - 日本（APT 研究活跃）
- `AU` - 澳大利亚（五眼联盟成员）

支持国家代码（ISO 3166-1 alpha-2）和大洲代码。详细配置参阅 [GEOIP.md](./docs/GEOIP.md)。

#### 时间窗口限制（降低暴露）

```toml
[global.time_window]
enabled = true           # 启用时间窗口控制
timezone = "UTC"         # 时区：UTC, Asia/Shanghai, America/New_York 等
start_time = "09:00"     # 开放开始时间（HH:MM）
end_time = "18:00"       # 开放结束时间（HH:MM）
```

**使用策略：**
- 根据目标的活动时间设置窗口（如工作时间）
- 非活动时段完全拒绝新连接，降低被扫描概率
- 支持跨天配置（如 `23:00` - `02:00`）
- 窗口外不影响已建立的连接

### 监听器配置（VShell 实例）

每个 VShell 实例可以独立配置防护策略：

```toml
[[listeners]]
name = "vshell_instance_1"       # 监听器名称（用于日志标识）
listen_port = ":443"             # 对外暴露端口
backend_addr = "127.0.0.1:9991"  # VShell 实际监听地址
protocol = "tcp"                 # 协议：tcp（VShell 标准）
```

**多实例示例：**
```toml
# 主 C2 服务器
[[listeners]]
name = "primary_c2"
listen_port = ":443"
backend_addr = "127.0.0.1:9991"
protocol = "tcp"

# 备用 C2 服务器
[[listeners]]
name = "backup_c2"
listen_port = ":8443"
backend_addr = "127.0.0.1:9992"
protocol = "tcp"
```

### 超时与防护配置

```toml
[listeners.timeout]
enabled = true       # 启用超时防护
initial_read = 30    # 初始读取超时（秒），0 = 无限制
connect_backend = 5  # 后端连接超时（秒），0 = 无限制
```

**超时策略说明：**
- `enabled = true` + `initial_read > 0` - **推荐配置**
  - 防止扫描器使用空连接探测
  - 真实流量到达后自动移除超时
  - 支持 VShell 的长连接通信
  
- `enabled = false` - 完全无超时
  - 适用于确定无扫描威胁的内网环境
  - 节省资源但失去空连接防护

### 路径过滤规则（HTTP 指纹防护）

路由规则按顺序匹配，支持前缀匹配和精确匹配：

```toml
[[listeners.routes]]
path = "/slt"      # 路径（前缀匹配）
action = "drop"      # 动作：drop（拦截）或 allow（放行）
response = "404"     # 拦截时的响应类型
```

**响应类型：**
- `404` - 返回 404 Not Found（伪装成不存在）
- `403` - 返回 403 Forbidden（显示禁止访问）
- `502` - 返回 502 Bad Gateway（伪装成网关错误）
- `close` - 直接关闭连接（无响应，更隐蔽）

**VShell 指纹防护规则示例：**

```toml
# 拦截常见的 Web 指纹探测
[[listeners.routes]]
path = "/favicon.ico"
action = "drop"
response = "404"

[[listeners.routes]]
path = "/robots.txt"
action = "drop"
response = "404"

# 拦截已知的 VShell 探测路径（根据威胁情报更新）
[[listeners.routes]]
path = "/slt"
action = "drop"
response = "404"

[[listeners.routes]]
path = "/swt"
action = "drop"
response = "404"
```

**规则匹配顺序：**
- 规则按配置顺序从上到下匹配
- 第一个匹配的规则生效
- 最后建议添加兜底规则（如拦截所有其他路径）

## 威胁情报与检测对抗

### 已知的 VShell 检测方法

根据 NVISO 报告，以下是威胁情报机构追踪 VShell 的主要手段：

1. **网络指纹检测规则**
   - Windows/Linux/MacOS stager 的网络特征
   - Beaconing 活动的流量模式
   - 特定的 HTTP 头部和响应特征

2. **基础设施追踪**
   - Team Cymru 的全球网络监控
   - ThreatFox 的 IOC 数据库
   - 被动 DNS 和证书透明度日志

3. **自动化扫描**
   - Shodan、Censys、Zoomeye 等搜索引擎
   - 安全厂商的主动探测
   - 蜜罐和诱捕系统

### vshell-firewall 的对抗策略

| 检测手段 | 防护措施 | 配置项 |
|---------|---------|--------|
| 网络指纹扫描 | 路径过滤 + 自定义响应 | `listeners.routes` |
| 地理位置追踪 | GeoIP 拦截热点地区 | `global.geoip` |
| 持续监控 | 时间窗口限制暴露 | `global.time_window` |
| 空连接探测 | 初始超时防护 | `listeners.timeout.initial_read` |
| 批量扫描 | 连接日志分析 | `global.log_file` |

### 最佳实践建议

1. **多层防御组合**
   ```toml
   启用 GeoIP + 时间窗口 + 路径过滤 + 超时防护
   ```

2. **定期更新拦截规则**
   - 关注威胁情报更新（NVISO、Google Cloud 等报告）
   - 根据日志分析调整路径过滤规则
   - 更新 GeoIP 数据库（每月）

3. **日志审计与分析**
   - 定期检查 `log_file` 中的异常访问
   - 识别新的扫描器特征和探测模式
   - 统计被拦截的地区和路径分布

4. **隐藏部署**
   - 使用常见端口（443、8080）伪装
   - 配置 TLS 证书（通过 nginx/caddy 前置）
   - 避免使用默认配置和明显的服务名称

5. **基础设施隔离**
   - VShell 后端仅监听 127.0.0.1
   - 通过 vshell-firewall 统一对外暴露
   - 使用独立的网络和防火墙策略

## 监控与维护

### 日志分析

vshell-firewall 的日志包含丰富的连接信息：

```
2025/12/09 10:23:45.123456 [INFO] [vshell_c2] New connection from 203.0.113.10:54321
2025/12/09 10:23:45.234567 [INFO] [vshell_c2] GeoIP: US (United States) - BLOCKED
2025/12/09 10:23:45.345678 [INFO] [vshell_c2] Connection dropped: blocked region

2025/12/09 10:24:10.123456 [INFO] [vshell_c2] New connection from 192.168.1.100:45678
2025/12/09 10:24:10.234567 [INFO] [vshell_c2] GeoIP: CN (China) - ALLOWED
2025/12/09 10:24:10.345678 [INFO] [vshell_c2] HTTP Request: GET / HTTP/1.1
2025/12/09 10:24:10.456789 [INFO] [vshell_c2] Route matched: / -> allow
2025/12/09 10:24:10.567890 [INFO] [vshell_c2] Connected to backend 127.0.0.1:9991
```

**关键指标：**
- 被拦截的地区分布（识别扫描来源）
- 被拦截的路径统计（发现新的探测模式）
- 连接时间分布（优化时间窗口配置）

### 性能优化

```toml
# 根据实际负载调整缓冲区大小
[global]
buffer_size = 65536  # 高流量场景增大缓冲区

# 降低日志级别减少 I/O
log_level = "warn"   # 生产环境仅记录警告和错误
```

### 故障排查

**问题：无法连接到后端 VShell**
```bash
# 检查后端是否监听
netstat -tlnp | grep 9991

# 检查 vshell-firewall 日志
tail -f /var/log/vshell-firewall.log
```

**问题：合法流量被拦截**
```bash
# 临时禁用 GeoIP（调试）
[global.geoip]
enabled = false

# 检查路由规则匹配顺序
# 确保 allow 规则在 drop 规则之前
```

**问题：服务无法启动**
```bash
# 检查端口占用
sudo lsof -i :443

# 检查配置文件语法
./build/vshell-firewall -config config.toml
```

## 安全注意事项

⚠️ **重要提醒：**

1. **合法使用**
   - vshell-firewall 设计用于合法的红队演练和渗透测试
   - 使用前确保获得明确的授权和许可
   - 遵守当地法律法规和行业规范

2. **数据保护**
   - 日志文件可能包含敏感信息，注意权限管理
   - 定期清理或加密历史日志
   - 使用 `log_level = "warn"` 减少敏感数据记录

3. **配置安全**
   - 保护 `config.toml` 文件（chmod 600）
   - 定期更新 GeoIP 数据库
   - 审查和更新拦截规则

4. **网络隔离**
   - VShell 后端应仅监听本地回环地址
   - 使用防火墙限制 vshell-firewall 的访问来源
   - 避免在公共云环境直接暴露

## 相关资源

### VShell 威胁情报

- [NVISO VShell 分析报告](https://www.nviso.eu/blog/nviso-analyzes-vshell-post-exploitation-tool)
- [ThreatFox VShell IOC](https://threatfox.abuse.ch/browse/malware/win.vshell/)

### GeoIP 数据库

- [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)
- [GeoIP 配置文档](./docs/GEOIP.md)

### 项目信息

- **GitHub**: [CRONUS-Security/vshell-firewall](https://github.com/CRONUS-Security/vshell-firewall)
- **许可证**: MIT License
- **维护者**: CRONUS-Security

## 贡献与反馈

欢迎提交 Issue 和 Pull Request：
- 报告 Bug 和安全漏洞
- 建议新功能和改进
- 分享部署经验和最佳实践
- 提供新的威胁情报和拦截规则
---

**免责声明：** 本工具仅供安全研究和合法授权的渗透测试使用。使用者应对自己的行为负责，开发团队不承担任何滥用导致的法律责任。
