#!/bin/bash
# GeoIP 数据库下载脚本

set -e

echo "=========================================="
echo "  vshell-firewall GeoIP 数据库下载工具"
echo "=========================================="
echo ""

# 数据库文件名
DB_FILE="GeoLite2-Country.mmdb"

# 检查是否已存在
if [ -f "$DB_FILE" ]; then
    echo "⚠️  检测到已存在的数据库文件: $DB_FILE"
    read -p "是否覆盖？(y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "取消下载。"
        exit 0
    fi
    rm -f "$DB_FILE"
fi

echo "📥 正在下载 GeoLite2-Country 数据库..."
echo ""
echo "注意：由于 MaxMind 的政策变更，现在需要注册免费账号才能下载。"
echo ""
echo "请按照以下步骤操作："
echo "1. 访问 https://dev.maxmind.com/geoip/geolite2-free-geolocation-data"
echo "2. 注册免费账号（Sign up for GeoLite2）"
echo "3. 登录后下载 GeoLite2 Country (MMDB 格式)"
echo "4. 将下载的 GeoLite2-Country.mmdb 文件放到当前目录"
echo ""
echo "或者使用以下命令（如果你已有账号）："
echo "  curl -o GeoLite2-Country.mmdb 'YOUR_DOWNLOAD_URL'"
echo ""

# 尝试从常见的镜像下载（可能已过期）
echo "正在尝试从镜像下载..."

MIRROR_URLS=(
    "https://git.io/GeoLite2-Country.mmdb"
    "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
)

DOWNLOADED=0
for URL in "${MIRROR_URLS[@]}"; do
    echo "尝试: $URL"
    if curl -L -f -o "$DB_FILE" "$URL" 2>/dev/null; then
        DOWNLOADED=1
        break
    fi
done

if [ $DOWNLOADED -eq 1 ]; then
    echo ""
    echo "✅ 下载成功！"
    echo "📁 文件位置: $(pwd)/$DB_FILE"
    echo "📊 文件大小: $(du -h $DB_FILE | cut -f1)"
    echo ""
    echo "现在可以在 config.toml 中启用 GeoIP 功能："
    echo ""
    echo "[global.geoip]"
    echo "enabled = true"
    echo "database_path = \"./$DB_FILE\""
    echo "block_regions = [\"US\", \"EU\"]"
    echo ""
else
    echo ""
    echo "❌ 自动下载失败。"
    echo ""
    echo "请手动下载："
    echo "1. 访问 https://dev.maxmind.com/geoip/geolite2-free-geolocation-data"
    echo "2. 注册并下载 GeoLite2-Country.mmdb"
    echo "3. 将文件放到当前目录: $(pwd)"
    echo ""
    exit 1
fi
