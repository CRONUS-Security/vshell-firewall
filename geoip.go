package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// GeoIPManager GeoIP 管理器
type GeoIPManager struct {
	enabled      bool
	db           *geoip2.Reader
	blockRegions map[string]bool
	mu           sync.RWMutex
}

// NewGeoIPManager 创建 GeoIP 管理器
func NewGeoIPManager(config GeoIPConfig) (*GeoIPManager, error) {
	manager := &GeoIPManager{
		enabled:      config.Enabled,
		blockRegions: make(map[string]bool),
	}

	if !config.Enabled {
		log.Println("[GeoIP] GeoIP checking is disabled")
		return manager, nil
	}

	// 打开数据库
	db, err := geoip2.Open(config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}
	manager.db = db

	// 解析拦截地区列表
	for _, region := range config.BlockRegions {
		normalizedRegion := strings.ToUpper(strings.TrimSpace(region))
		manager.blockRegions[normalizedRegion] = true
	}

	log.Printf("[GeoIP] Initialized with database: %s", config.DatabasePath)
	log.Printf("[GeoIP] Blocking regions: %v", config.BlockRegions)

	return manager, nil
}

// Close 关闭 GeoIP 数据库
func (g *GeoIPManager) Close() error {
	if g.db != nil {
		return g.db.Close()
	}
	return nil
}

// IsBlocked 检查 IP 是否被拦截
func (g *GeoIPManager) IsBlocked(ip string) (bool, string, error) {
	if !g.enabled {
		return false, "", nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	// 解析 IP 地址
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, "", fmt.Errorf("invalid IP address: %s", ip)
	}

	// 查询 IP 信息
	record, err := g.db.Country(parsedIP)
	if err != nil {
		return false, "", fmt.Errorf("failed to lookup IP: %w", err)
	}

	countryCode := record.Country.IsoCode
	continentCode := record.Continent.Code

	// 检查是否在拦截列表中
	// 支持国家代码（如 US, CN）和大洲代码（如 EU, AS）
	if g.blockRegions[countryCode] || g.blockRegions[continentCode] {
		return true, countryCode, nil
	}

	// 特殊处理欧洲地区
	if g.blockRegions["EU"] && isEuropeanCountry(countryCode) {
		return true, countryCode, nil
	}

	return false, countryCode, nil
}

// GetCountryInfo 获取 IP 的国家信息（用于日志）
func (g *GeoIPManager) GetCountryInfo(ip string) (string, string, error) {
	if !g.enabled {
		return "", "", nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", "", fmt.Errorf("invalid IP address: %s", ip)
	}

	record, err := g.db.Country(parsedIP)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup IP: %w", err)
	}

	return record.Country.IsoCode, record.Country.Names["zh-CN"], nil
}

// isEuropeanCountry 判断是否为欧洲国家
func isEuropeanCountry(code string) bool {
	// 欧盟成员国和其他欧洲国家
	europeanCountries := map[string]bool{
		"AT": true, // 奥地利
		"BE": true, // 比利时
		"BG": true, // 保加利亚
		"HR": true, // 克罗地亚
		"CY": true, // 塞浦路斯
		"CZ": true, // 捷克
		"DK": true, // 丹麦
		"EE": true, // 爱沙尼亚
		"FI": true, // 芬兰
		"FR": true, // 法国
		"DE": true, // 德国
		"GR": true, // 希腊
		"HU": true, // 匈牙利
		"IE": true, // 爱尔兰
		"IT": true, // 意大利
		"LV": true, // 拉脱维亚
		"LT": true, // 立陶宛
		"LU": true, // 卢森堡
		"MT": true, // 马耳他
		"NL": true, // 荷兰
		"PL": true, // 波兰
		"PT": true, // 葡萄牙
		"RO": true, // 罗马尼亚
		"SK": true, // 斯洛伐克
		"SI": true, // 斯洛文尼亚
		"ES": true, // 西班牙
		"SE": true, // 瑞典
		"GB": true, // 英国
		"NO": true, // 挪威
		"CH": true, // 瑞士
		"IS": true, // 冰岛
		"LI": true, // 列支敦士登
	}
	return europeanCountries[code]
}
