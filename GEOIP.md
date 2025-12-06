# GeoIP 功能说明

## 功能概述

vshell-firewall 现在支持基于 IP 地理位置的访问控制功能，可以自动拦截来自特定国家或地区的连接请求。

## 快速开始

### 1. 下载 GeoIP 数据库

使用 MaxMind 的 GeoLite2 免费数据库：

```bash
# 访问 MaxMind 官网下载（需要免费注册账号）
# https://dev.maxmind.com/geoip/geolite2-free-geolocation-data

# 下载 GeoLite2-Country.mmdb 文件到项目目录
wget https://git.io/GeoLite2-Country.mmdb -O GeoLite2-Country.mmdb
```

### 2. 配置 GeoIP

在 `config.toml` 中启用 GeoIP 功能：

```toml
[global.geoip]
enabled = true                               # 启用 GeoIP 检查
database_path = "./GeoLite2-Country.mmdb"    # 数据库路径
block_regions = ["US", "EU"]                 # 拦截的地区列表
```

### 3. 支持的地区代码

#### 国家代码（ISO 3166-1 alpha-2）
- `US` - 美国
- `CN` - 中国
- `GB` - 英国
- `JP` - 日本
- `KR` - 韩国
- `DE` - 德国
- `FR` - 法国
- 等... （所有 ISO 3166-1 alpha-2 国家代码）

#### 大洲代码
- `AF` - 非洲 (Africa)
- `AS` - 亚洲 (Asia)
- `EU` - 欧洲 (Europe) - **自动匹配所有欧洲国家**
- `NA` - 北美洲 (North America)
- `SA` - 南美洲 (South America)
- `OC` - 大洋洲 (Oceania)
- `AN` - 南极洲 (Antarctica)

#### 特殊处理：欧洲地区

当配置 `EU` 时，系统会自动拦截以下欧洲国家的连接：

```
奥地利(AT), 比利时(BE), 保加利亚(BG), 克罗地亚(HR), 塞浦路斯(CY), 
捷克(CZ), 丹麦(DK), 爱沙尼亚(EE), 芬兰(FI), 法国(FR), 德国(DE), 
希腊(GR), 匈牙利(HU), 爱尔兰(IE), 意大利(IT), 拉脱维亚(LV), 
立陶宛(LT), 卢森堡(LU), 马耳他(MT), 荷兰(NL), 波兰(PL), 
葡萄牙(PT), 罗马尼亚(RO), 斯洛伐克(SK), 斯洛文尼亚(SI), 
西班牙(ES), 瑞典(SE), 英国(GB), 挪威(NO), 瑞士(CH), 
冰岛(IS), 列支敦士登(LI)
```

## 配置示例

### 示例 1: 拦截美国和欧洲

```toml
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["US", "EU"]
```

### 示例 2: 只拦截特定国家

```toml
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["US", "GB", "FR", "DE"]
```

### 示例 3: 拦截整个大洲

```toml
[global.geoip]
enabled = true
database_path = "./GeoLite2-Country.mmdb"
block_regions = ["NA", "EU", "OC"]  # 拦截北美、欧洲、大洋洲
```

### 示例 4: 禁用 GeoIP

```toml
[global.geoip]
enabled = false  # 关闭 GeoIP 功能
```

## 日志输出

根据 `log_level` 配置，GeoIP 会输出不同级别的日志：

### Debug 模式
```toml
[global]
log_level = "debug"
```
输出所有连接的地理位置信息：
```
[http_proxy] Allowed connection from 1.2.3.4 (Country: CN)
[http_proxy] Blocked connection from 5.6.7.8 (Country: US)
```

### Info 模式
```toml
[global]
log_level = "info"
```
只输出被拦截的连接：
```
[http_proxy] Blocked connection from 5.6.7.8 (Country: US)
```

### Warn/Error 模式
不输出 GeoIP 相关日志（除非发生错误）

## 工作原理

1. 客户端连接到防火墙
2. 提取客户端 IP 地址
3. 查询 GeoIP 数据库获取国家/地区信息
4. 检查是否在拦截列表中
5. 如果匹配，直接关闭连接；否则继续处理请求

## 性能说明

- GeoIP 查询使用内存映射文件，查询速度极快（微秒级）
- 数据库在程序启动时加载到内存
- 对正常流量几乎没有性能影响

## 注意事项

1. **数据库更新**：建议定期更新 GeoLite2 数据库（每月更新一次）
2. **本地 IP**：本地 IP（127.0.0.1, 192.168.x.x 等）可能无法正确识别地理位置
3. **代理/VPN**：如果客户端使用代理或 VPN，识别的是代理服务器的位置
4. **IPv6 支持**：完全支持 IPv6 地址查询

## 故障排除

### 问题 1: 数据库文件找不到
```
Failed to initialize GeoIP manager: failed to open GeoIP database: ...
```
**解决方案**：检查 `database_path` 配置是否正确，确保文件存在

### 问题 2: 所有连接都被拦截
**解决方案**：检查 `block_regions` 配置，确保没有误配置成拦截所有地区

### 问题 3: 特定 IP 无法识别
**解决方案**：可能是内网 IP 或特殊 IP，这类 IP 不会被拦截

## 技术细节

- 使用库：`github.com/oschwald/geoip2-golang`
- 数据库格式：MaxMind DB (MMDB)
- 查询方式：国家级别查询（GeoLite2-Country）
- 线程安全：使用读写锁保护并发访问
