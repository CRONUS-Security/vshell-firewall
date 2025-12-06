package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config 主配置结构
type Config struct {
	Global    GlobalConfig     `toml:"global"`
	Listeners []ListenerConfig `toml:"listeners"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	BufferSize   int              `toml:"buffer_size"`
	LogLevel     string           `toml:"log_level"`
	GeoIP        GeoIPConfig      `toml:"geoip"`
	TimeWindow   TimeWindowConfig `toml:"time_window"`
}

// GeoIPConfig GeoIP 配置
type GeoIPConfig struct {
	Enabled      bool     `toml:"enabled"`       // 是否启用 GeoIP 检查
	DatabasePath string   `toml:"database_path"` // GeoLite2 数据库路径
	BlockRegions []string `toml:"block_regions"` // 要拦截的地区列表，如 ["US", "GB", "EU"]
}

// TimeWindowConfig 时间窗口配置
type TimeWindowConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用时间窗口过滤
	Timezone  string `toml:"timezone"`   // 时区，如 "UTC", "Asia/Shanghai"
	StartTime string `toml:"start_time"` // 开始时间，格式 HH:MM，如 "00:00"
	EndTime   string `toml:"end_time"`   // 结束时间，格式 HH:MM，如 "11:00"
}

// ListenerConfig 监听器配置
type ListenerConfig struct {
	Name        string              `toml:"name"`
	ListenPort  string              `toml:"listen_port"`
	BackendAddr string              `toml:"backend_addr"`
	Protocol    string              `toml:"protocol"` // tcp
	Timeout     TimeoutConfig       `toml:"timeout"`
	HTTP        HTTPProcessorConfig `toml:"http"`
	TCP         TCPProcessorConfig  `toml:"tcp"`
	Routes      []RouteRule         `toml:"routes"` // 兼容旧配置，优先使用 HTTP/TCP processor
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Enabled        bool `toml:"enabled"`
	InitialRead    int  `toml:"initial_read"`
	ConnectBackend int  `toml:"connect_backend"`
}

// HTTPProcessorConfig HTTP 处理器配置
type HTTPProcessorConfig struct {
	Processors []Processor `toml:"processor"`
}

// TCPProcessorConfig TCP 处理器配置
type TCPProcessorConfig struct {
	Processors []Processor `toml:"processor"`
}

// Processor 处理器规则
type Processor struct {
	Path       interface{} `toml:"path"`      // string 或 []string
	MatchMode  string      `toml:"match_mode"` // prefix (前缀), exact (精确), regex (正则)
	Action     string      `toml:"action"`     // allow, drop, rewrite, file, proxy
	Response   string      `toml:"response"`   // 404, 403, 502, close (用于 drop)
	RewriteTo  string      `toml:"rewrite_to"` // 路径重写目标 (用于 rewrite)
	File       string      `toml:"file"`       // 文件路径 (用于 file)
	ProxyTo    string      `toml:"proxy_to"`   // 代理目标 (用于 proxy)
}

// RouteRule 路由规则（兼容旧配置）
type RouteRule struct {
	Path      string `toml:"path"`
	Action    string `toml:"action"`     // drop, allow
	Response  string `toml:"response"`   // 404, 403, 502, close
	RewriteTo string `toml:"rewrite_to"` // 路径重写目标（可选）
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	var config Config

	// 读取配置文件
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 TOML
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证全局配置
	if c.Global.BufferSize <= 0 {
		return fmt.Errorf("global.buffer_size must be positive")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Global.LogLevel] {
		return fmt.Errorf("global.log_level must be one of: debug, info, warn, error")
	}

	// 验证 GeoIP 配置
	if c.Global.GeoIP.Enabled {
		if c.Global.GeoIP.DatabasePath == "" {
			return fmt.Errorf("global.geoip.database_path is required when geoip is enabled")
		}
		if len(c.Global.GeoIP.BlockRegions) == 0 {
			return fmt.Errorf("global.geoip.block_regions must contain at least one region when geoip is enabled")
		}
	}

	// 验证时间窗口配置
	if c.Global.TimeWindow.Enabled {
		if err := validateTimeWindowConfig(&c.Global.TimeWindow); err != nil {
			return fmt.Errorf("global.time_window: %w", err)
		}
	}

	// 验证监听器配置
	if len(c.Listeners) == 0 {
		return fmt.Errorf("at least one listener must be configured")
	}

	listenerNames := make(map[string]bool)
	listenPorts := make(map[string]bool)

	for i, listener := range c.Listeners {
		// 检查名称唯一性
		if listener.Name == "" {
			return fmt.Errorf("listener[%d]: name is required", i)
		}
		if listenerNames[listener.Name] {
			return fmt.Errorf("listener[%d]: duplicate name '%s'", i, listener.Name)
		}
		listenerNames[listener.Name] = true

		// 检查端口唯一性
		if listener.ListenPort == "" {
			return fmt.Errorf("listener[%d]: listen_port is required", i)
		}
		if listenPorts[listener.ListenPort] {
			return fmt.Errorf("listener[%d]: duplicate listen_port '%s'", i, listener.ListenPort)
		}
		listenPorts[listener.ListenPort] = true

		// 检查后端地址
		if listener.BackendAddr == "" {
			return fmt.Errorf("listener[%d]: backend_addr is required", i)
		}

	// 检查协议类型
	validProtocols := map[string]bool{"tcp": true}
	if !validProtocols[listener.Protocol] {
		return fmt.Errorf("listener[%d]: protocol must be: tcp", i)
	}		// 检查超时配置
		if listener.Timeout.InitialRead < 0 {
			return fmt.Errorf("listener[%d]: timeout.initial_read cannot be negative", i)
		}
		if listener.Timeout.ConnectBackend < 0 {
			return fmt.Errorf("listener[%d]: timeout.connect_backend cannot be negative", i)
		}

		// 验证 HTTP 处理器
		for j, proc := range listener.HTTP.Processors {
			if err := validateProcessor(proc, i, "http", j); err != nil {
				return err
			}
		}

		// 验证 TCP 处理器
		for j, proc := range listener.TCP.Processors {
			if err := validateProcessor(proc, i, "tcp", j); err != nil {
				return err
			}
		}

		// 检查旧的路由规则（兼容性）
		for j, route := range listener.Routes {
			if route.Path == "" {
				return fmt.Errorf("listener[%d].route[%d]: path is required", i, j)
			}

			validActions := map[string]bool{"drop": true, "allow": true}
			if !validActions[route.Action] {
				return fmt.Errorf("listener[%d].route[%d]: action must be one of: drop, allow", i, j)
			}

			if route.Action == "drop" {
				validResponses := map[string]bool{"404": true, "403": true, "502": true, "close": true}
				if !validResponses[route.Response] {
					return fmt.Errorf("listener[%d].route[%d]: response must be one of: 404, 403, 502, close", i, j)
				}
			}
		}
	}

	return nil
}

// validateProcessor 验证处理器配置
func validateProcessor(proc Processor, listenerIdx int, processorType string, procIdx int) error {
	validActions := map[string]bool{"allow": true, "drop": true, "rewrite": true, "file": true, "proxy": true}
	if !validActions[proc.Action] {
		return fmt.Errorf("listener[%d].%s.processor[%d]: action must be one of: allow, drop, rewrite, file, proxy", 
			listenerIdx, processorType, procIdx)
	}

	// 验证匹配模式
	if proc.MatchMode != "" {
		validModes := map[string]bool{"prefix": true, "exact": true, "regex": true}
		if !validModes[proc.MatchMode] {
			return fmt.Errorf("listener[%d].%s.processor[%d]: match_mode must be one of: prefix, exact, regex", 
				listenerIdx, processorType, procIdx)
		}
	}

	// 验证 action 特定的配置
	switch proc.Action {
	case "drop":
		validResponses := map[string]bool{"404": true, "403": true, "502": true, "close": true}
		if proc.Response != "" && !validResponses[proc.Response] {
			return fmt.Errorf("listener[%d].%s.processor[%d]: response must be one of: 404, 403, 502, close", 
				listenerIdx, processorType, procIdx)
		}
	case "rewrite":
		if proc.RewriteTo == "" {
			return fmt.Errorf("listener[%d].%s.processor[%d]: rewrite_to is required for rewrite action", 
				listenerIdx, processorType, procIdx)
		}
	case "file":
		if proc.File == "" {
			return fmt.Errorf("listener[%d].%s.processor[%d]: file is required for file action", 
				listenerIdx, processorType, procIdx)
		}
	case "proxy":
		if proc.ProxyTo == "" {
			return fmt.Errorf("listener[%d].%s.processor[%d]: proxy_to is required for proxy action", 
				listenerIdx, processorType, procIdx)
		}
	}

	return nil
}

// GetPaths 获取处理器的路径列表
func (p *Processor) GetPaths() []string {
	if p.Path == nil {
		return []string{}
	}

	switch v := p.Path.(type) {
	case string:
		return []string{v}
	case []interface{}:
		paths := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				paths = append(paths, str)
			}
		}
		return paths
	default:
		return []string{}
	}
}

// MatchProcessor 匹配 HTTP 处理器
func (l *ListenerConfig) MatchHTTPProcessor(path string) *Processor {
	for _, proc := range l.HTTP.Processors {
		if matchPath(path, proc) {
			return &proc
		}
	}
	return nil
}

// MatchTCPProcessor 获取 TCP 处理器
func (l *ListenerConfig) MatchTCPProcessor() *Processor {
	if len(l.TCP.Processors) > 0 {
		return &l.TCP.Processors[0]
	}
	return nil
}

// matchPath 匹配路径
func matchPath(path string, proc Processor) bool {
	paths := proc.GetPaths()
	if len(paths) == 0 {
		return true // 无路径限制，匹配所有
	}

	matchMode := proc.MatchMode
	if matchMode == "" {
		matchMode = "prefix" // 默认前缀匹配
	}

	for _, pattern := range paths {
		switch matchMode {
		case "exact":
			if path == pattern {
				return true
			}
		case "prefix":
			if strings.HasPrefix(path, pattern) {
				return true
			}
		case "regex":
			// TODO: 实现正则匹配
			if strings.HasPrefix(path, pattern) {
				return true
			}
		}
	}
	return false
}

// MatchRoute 匹配路由规则
func (l *ListenerConfig) MatchRoute(path string) *RouteRule {
	for _, route := range l.Routes {
		// 前缀匹配
		if strings.HasPrefix(path, route.Path) {
			return &route
		}
	}
	return nil
}

// validateTimeWindowConfig 验证时间窗口配置
func validateTimeWindowConfig(tw *TimeWindowConfig) error {
	if tw.Timezone == "" {
		return fmt.Errorf("timezone is required")
	}
	
	// 验证时区
	if _, err := time.LoadLocation(tw.Timezone); err != nil {
		return fmt.Errorf("invalid timezone '%s': %w", tw.Timezone, err)
	}
	
	if tw.StartTime == "" {
		return fmt.Errorf("start_time is required")
	}
	if tw.EndTime == "" {
		return fmt.Errorf("end_time is required")
	}
	
	// 验证时间格式
	if _, err := time.Parse("15:04", tw.StartTime); err != nil {
		return fmt.Errorf("invalid start_time format '%s', expected HH:MM", tw.StartTime)
	}
	if _, err := time.Parse("15:04", tw.EndTime); err != nil {
		return fmt.Errorf("invalid end_time format '%s', expected HH:MM", tw.EndTime)
	}
	
	return nil
}

// IsInTimeWindow 检查当前时间是否在允许的时间窗口内
func (tw *TimeWindowConfig) IsInTimeWindow() (bool, error) {
	if !tw.Enabled {
		return true, nil
	}
	
	// 加载时区
	loc, err := time.LoadLocation(tw.Timezone)
	if err != nil {
		return false, fmt.Errorf("failed to load timezone: %w", err)
	}
	
	// 获取当前时间（在指定时区）
	now := time.Now().In(loc)
	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeInMinutes := currentHour*60 + currentMinute
	
	// 解析开始和结束时间
	startTime, _ := time.Parse("15:04", tw.StartTime)
	endTime, _ := time.Parse("15:04", tw.EndTime)
	
	startMinutes := startTime.Hour()*60 + startTime.Minute()
	endMinutes := endTime.Hour()*60 + endTime.Minute()
	
	// 检查是否在时间窗口内
	if startMinutes <= endMinutes {
		// 正常情况：start_time < end_time (如 00:00 - 11:00)
		return currentTimeInMinutes >= startMinutes && currentTimeInMinutes < endMinutes, nil
	} else {
		// 跨天情况：start_time > end_time (如 23:00 - 02:00)
		return currentTimeInMinutes >= startMinutes || currentTimeInMinutes < endMinutes, nil
	}
}
