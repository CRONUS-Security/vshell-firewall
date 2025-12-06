package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config 主配置结构
type Config struct {
	Global    GlobalConfig     `toml:"global"`
	Listeners []ListenerConfig `toml:"listeners"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	BufferSize int    `toml:"buffer_size"`
	LogLevel   string `toml:"log_level"`
}

// ListenerConfig 监听器配置
type ListenerConfig struct {
	Name        string        `toml:"name"`
	ListenPort  string        `toml:"listen_port"`
	BackendAddr string        `toml:"backend_addr"`
	Protocol    string        `toml:"protocol"` // auto, http, tcp
	Timeout     TimeoutConfig `toml:"timeout"`
	Routes      []RouteRule   `toml:"routes"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	Enabled        bool `toml:"enabled"`
	InitialRead    int  `toml:"initial_read"`
	ConnectBackend int  `toml:"connect_backend"`
}

// RouteRule 路由规则
type RouteRule struct {
	Path     string `toml:"path"`
	Action   string `toml:"action"`   // drop, allow
	Response string `toml:"response"` // 404, 403, 502, close
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
		validProtocols := map[string]bool{"auto": true, "http": true, "tcp": true}
		if !validProtocols[listener.Protocol] {
			return fmt.Errorf("listener[%d]: protocol must be one of: auto, http, tcp", i)
		}

		// 检查超时配置
		if listener.Timeout.InitialRead < 0 {
			return fmt.Errorf("listener[%d]: timeout.initial_read cannot be negative", i)
		}
		if listener.Timeout.ConnectBackend < 0 {
			return fmt.Errorf("listener[%d]: timeout.connect_backend cannot be negative", i)
		}

		// 检查路由规则
		if len(listener.Routes) == 0 {
			return fmt.Errorf("listener[%d]: at least one route must be configured", i)
		}

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

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			BufferSize: 32768,
			LogLevel:   "info",
		},
		Listeners: []ListenerConfig{
			{
				Name:        "default_proxy",
				ListenPort:  ":8880",
				BackendAddr: "127.0.0.1:9991",
				Protocol:    "auto",
				Timeout: TimeoutConfig{
					Enabled:        true,
					InitialRead:    30,
					ConnectBackend: 5,
				},
				Routes: []RouteRule{
					{
						Path:     "/slt",
						Action:   "drop",
						Response: "404",
					},
					{
						Path:   "/",
						Action: "allow",
					},
				},
			},
		},
	}
}
