# vshell-firewall

VShell 基础设施指纹隐藏与访问控制代理 - 通过流量过滤和指纹伪装，保护 VShell C2 基础设施免受公开扫描和威胁情报追踪。

## 项目背景

随着 NVISO 等安全厂商公开 [VShell 后渗透工具分析报告](https://www.nviso.eu/blog/nviso-analyzes-vshell-post-exploitation-tool)，VShell 的网络特征、通信模式和基础设施标识已被广泛公开，威胁情报机构（Team Cymru、ThreatFox 等）正在全球范围内追踪和标记 VShell C2 服务器。

vshell-firewall 作为 VShell 服务端的前置反向代理，核心目标是**隐匿而非拦截**：

- **指纹隐藏** - 行为模拟真实 nginx：根路径返回伪装页面，其余路径（含 `/swt`、`/slt`）一律 404，与 nginx 默认行为无指纹差异
- **访问控制** - 基于 GeoIP、时间窗口的多维度访问过滤，屏蔽威胁情报热点地区
- **流量伪装** - 根路径 `/` 与 `/index.html` 返回编译内建的 nginx 伪装页面
- **虚拟路径** - 可选配置对外虚拟路径重写转发到后端真实路径

## 快速开始

```bash
# 构建
make build

# 下载 GeoIP 数据库（可选，启用 GeoIP 过滤时需要）
./download-geoip.sh

# 配置
cp config.toml.example config.toml
vim config.toml

# 运行
./build/vshell-firewall -config config.toml
```

## 配置

完整带有说明注释的示例见 [docs\config.toml.example](docs\config.toml.example)

但是在实际环境中，尽可能做好隐蔽，推荐配置见 [config.toml.example](config.toml.example)

### 全局配置

```toml
[global]
buffer_size = 32768              # TCP 缓冲区大小（字节）
log_level = "info"               # 日志级别: debug, info, warn, error
log_file = "./vshell-firewall.log"  # 日志文件路径，为空则只输出到控制台

[global.geoip]
enabled = true                               # 是否启用 GeoIP 过滤
database_path = "./GeoLite2-Country.mmdb"    # GeoIP2 数据库路径
block_regions = ["US", "EU"]                 # 拦截的国家/地区列表
                                             # 支持 ISO 3166-1 alpha-2 国家代码和大洲代码（EU, AS, NA 等）
                                             # 详细说明见 docs/GEOIP.md

[global.time_window]
enabled = true            # 是否启用时间窗口限制
timezone = "UTC"          # 时区: UTC, Asia/Shanghai, America/New_York 等
start_time = "09:00"      # 开放开始时间（HH:MM）
end_time = "18:00"        # 开放结束时间（HH:MM），支持跨天（如 23:00 - 02:00）
                          # 窗口外拒绝新连接，不影响已建立的连接
```

### 监听器

每个监听器对应一个 VShell 实例，可独立配置：

```toml
[[listeners]]
name = "vshell_c2_main"           # 监听器名称（用于日志标识）
listen_port = ":443"              # 对外暴露端口
backend_addr = "127.0.0.1:9991"   # VShell 实际监听地址（建议仅监听 127.0.0.1）
protocol = "tcp"                  # 后端协议：tcp

[listeners.timeout]
enabled = true       # 是否启用超时
initial_read = 30    # 初始读取超时（秒），防止空连接扫描；真实流量到达后自动移除
connect_backend = 5  # 后端连接超时（秒），0 = 无限制
```

HTTP 请求按处理器规则伪装处理；非 HTTP 的 raw TCP 流量直接转发到后端（VShell 通信依赖 raw TCP）。

### HTTP 处理器

处理器按配置顺序匹配（前缀匹配，`path` 支持 string 或数组），第一个匹配的生效：

| 动作 | 说明 |
| --- | --- |
| `drop` | 直接拦截，返回指定响应（`404` / `403` / `502` / `close`） |
| `rewrite` | 将虚拟路径重写为 `rewrite_to` 指定的真实路径后转发 |
| `file` | 返回静态页面；未指定 `file` 参数时默认返回编译内建的伪装页面 |
| `allow` | 不做修改，直接转发到后端 |

默认行为模拟真实 nginx：根路径 `/` 与 `/index.html` 返回内建伪装页面，其余未匹配任何处理器的路径一律返回 404。因此 `/swt`、`/slt` 等真实路径无需任何规则即返回 404，与 nginx 默认行为无指纹差异，无需显式 `drop` 规则（直接 drop 反而可能因响应差异暴露特征）。

除非一定要用一键上线的脚本，否则建议不要启用 `rewrite` 处理模式，而是不做处理，让真实路径统一返回 404。

典型配置：

```toml
# 默认行为模拟真实 nginx：
#   - 根路径 / 与 /index.html 返回内建伪装页面
#   - 其余路径（含 /swt、/slt）一律 404，无需任何规则

# 可选：对外虚拟路径重写转发（仅需保留 stage 模式一键上线时启用）
[[listeners.http.processor]]
path = "/patch_swt"
action = "rewrite"
rewrite_to = "/swt"

# 非 HTTP 的 raw TCP 流量无需配置，直接转发到后端（VShell 通信依赖 raw TCP）
```

### 对抗策略

| 检测手段 | 防护措施 | 配置项 |
| --- | --- | --- |
| 网络指纹扫描 | nginx 行为模拟（根路径伪装页，其余 404） | `listeners.http.processor` |
| 地理位置追踪 | GeoIP 拦截热点地区 | `global.geoip` |
| 持续监控 | 时间窗口限制暴露 | `global.time_window` |
| 空连接探测 | 初始读取超时 | `listeners.timeout.initial_read` |

**部署建议：**

- VShell 后端仅监听 127.0.0.1，通过 vshell-firewall 统一对外暴露
- 使用常见端口（443、8080）并前置 TLS 证书降低可疑度
- 根据日志定期调整路径过滤规则，每月更新 GeoIP 数据库

## 故障排查

### 无法连接到后端 VShell

```bash
netstat -tlnp | grep 9991              # 检查后端是否监听
tail -f vshell-firewall.log            # 检查代理日志
```

### 合法流量被拦截

```bash
[global.geoip]
enabled = false                        # 临时禁用 GeoIP 定位问题
```

### 服务无法启动

```bash
lsof -i :443                           # 检查端口占用
./build/vshell-firewall -config config.toml   # 检查配置语法
```

## 相关资源

- [NVISO VShell 分析报告](https://www.nviso.eu/blog/nviso-analyzes-vshell-post-exploitation-tool)
- [ThreatFox VShell IOC](https://threatfox.abuse.ch/browse/malware/win.vshell/)
- [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data)
- [GeoIP 配置文档](./docs/GEOIP.md)

## 贡献与反馈

欢迎提交 Issue 和 Pull Request：报告 Bug、建议新功能、分享部署经验。

---

**免责声明：** 本工具仅供安全研究和合法授权的渗透测试使用。使用者应对自己的行为负责，开发团队不承担任何滥用导致的法律责任。
