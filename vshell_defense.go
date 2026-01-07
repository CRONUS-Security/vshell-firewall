package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// VShellDefense VShell 防御模块
type VShellDefense struct {
	config VShellDefenseConfig
	mu     sync.RWMutex

	// 连接追踪
	connections map[string]*VShellConnectionTracker

	// 统计信息
	stats VShellDefenseStats
}

// VShellConnectionTracker 连接追踪器
type VShellConnectionTracker struct {
	FirstSeen         time.Time
	LastSeen          time.Time
	WebSocketUpgrade  bool
	HandshakeDetected bool
	SuspiciousScore   int
	BlockedReason     string
}

// VShellDefenseStats 统计信息
type VShellDefenseStats struct {
	TotalChecked     int64
	WebSocketBlocked int64
	HandshakeBlocked int64
	PayloadBlocked   int64
	RateLimited      int64
}

// VShell 协议特征常量
const (
	// VShell 版本标识
	VShellVersion493 = "4.9.3"

	// VShell 命令标识 (Little Endian)
	VShellCmdConf = 0x636f6e66 // "conf"
	VShellCmdFile = 0x66696c65 // "file"
	VShellCmdSucc = 0x73756373 // "sucs"

	// WebSocket 升级特征
	WebSocketUpgradeHeader    = "Upgrade: websocket"
	WebSocketConnectionHeader = "Connection: Upgrade"
	WebSocketVersionHeader    = "Sec-WebSocket-Version: 13"

	// 响应特征
	VShellResponseSwitching = "101 Switching"
)

// VShellDefenseResult 检测结果
type VShellDefenseResult struct {
	IsBlocked   bool
	BlockReason string
	ThreatLevel string // "none", "low", "medium", "high", "critical"
	Details     map[string]interface{}
}

// NewVShellDefense 创建新的 VShell 防御实例
func NewVShellDefense(config VShellDefenseConfig) *VShellDefense {
	defense := &VShellDefense{
		config:      config,
		connections: make(map[string]*VShellConnectionTracker),
	}

	// 启动清理协程
	if config.Enabled {
		go defense.cleanupRoutine()
	}

	return defense
}

// CheckRequest 检查请求数据
func (v *VShellDefense) CheckRequest(clientIP string, data []byte, path string) *VShellDefenseResult {
	if !v.config.Enabled {
		return &VShellDefenseResult{IsBlocked: false, ThreatLevel: "none"}
	}

	v.mu.Lock()
	v.stats.TotalChecked++
	v.mu.Unlock()

	result := &VShellDefenseResult{
		IsBlocked:   false,
		ThreatLevel: "none",
		Details:     make(map[string]interface{}),
	}

	// 1. 检查 WebSocket 升级请求特征
	if v.config.BlockWebSocketUpgrade && v.isVShellWebSocketUpgrade(data) {
		result.IsBlocked = true
		result.BlockReason = "VShell WebSocket upgrade pattern detected"
		result.ThreatLevel = "high"
		result.Details["pattern"] = "websocket_upgrade"
		v.mu.Lock()
		v.stats.WebSocketBlocked++
		v.mu.Unlock()
		return result
	}

	// 2. 检查可疑路径
	if v.config.BlockSuspiciousPaths && v.isSuspiciousPath(path) {
		result.IsBlocked = true
		result.BlockReason = fmt.Sprintf("Suspicious VShell path detected: %s", path)
		result.ThreatLevel = "medium"
		result.Details["path"] = path
		return result
	}

	// 3. 检查 VShell 版本握手
	if v.config.BlockVersionHandshake && v.isVShellVersionHandshake(data) {
		result.IsBlocked = true
		result.BlockReason = "VShell version handshake detected"
		result.ThreatLevel = "critical"
		result.Details["pattern"] = "version_handshake"
		v.mu.Lock()
		v.stats.HandshakeBlocked++
		v.mu.Unlock()
		return result
	}

	// 4. 检查 VShell 命令特征
	if v.config.BlockCommandPatterns && v.isVShellCommand(data) {
		result.IsBlocked = true
		result.BlockReason = "VShell command pattern detected"
		result.ThreatLevel = "critical"
		result.Details["pattern"] = "command"
		v.mu.Lock()
		v.stats.PayloadBlocked++
		v.mu.Unlock()
		return result
	}

	// 5. 检查加密载荷特征
	if v.config.BlockEncryptedPayloads && v.isVShellEncryptedPayload(data) {
		result.IsBlocked = true
		result.BlockReason = "VShell encrypted payload pattern detected"
		result.ThreatLevel = "high"
		result.Details["pattern"] = "encrypted_payload"
		v.mu.Lock()
		v.stats.PayloadBlocked++
		v.mu.Unlock()
		return result
	}

	// 6. 检查 Vkey 哈希特征
	if v.config.BlockVkeyPatterns && v.isVKeyHash(data) {
		result.IsBlocked = true
		result.BlockReason = "VShell Vkey hash pattern detected"
		result.ThreatLevel = "critical"
		result.Details["pattern"] = "vkey_hash"
		v.mu.Lock()
		v.stats.HandshakeBlocked++
		v.mu.Unlock()
		return result
	}

	// 7. 更新连接追踪
	v.updateConnectionTracker(clientIP, data)

	return result
}

// isVShellWebSocketUpgrade 检查是否为 VShell 的 WebSocket 升级请求
func (v *VShellDefense) isVShellWebSocketUpgrade(data []byte) bool {
	dataStr := string(data)

	// 检查 VShell 特有的 WebSocket 升级请求模式
	// VShell 使用特定格式: GET /ws HTTP/1.1 + 标准 WebSocket 头
	hasUpgrade := strings.Contains(dataStr, WebSocketUpgradeHeader)
	hasConnection := strings.Contains(dataStr, WebSocketConnectionHeader)
	hasVersion := strings.Contains(dataStr, WebSocketVersionHeader)

	// 检查是否是对 /ws 或其他可疑路径的请求
	isWsPath := strings.Contains(dataStr, "GET /ws ") ||
		strings.Contains(dataStr, "GET /ws/ ") ||
		strings.Contains(dataStr, "GET /websocket ")

	// VShell 特征: 所有头部都存在且路径为 /ws
	if hasUpgrade && hasConnection && hasVersion && isWsPath {
		return true
	}

	// 额外检查: WebSocket Key 格式 (Base64 编码的 16 字节)
	if hasUpgrade && hasConnection {
		wsKeyPattern := regexp.MustCompile(`Sec-WebSocket-Key:\s*([A-Za-z0-9+/]{22}==)`)
		if wsKeyPattern.MatchString(dataStr) {
			// 检查是否有 VShell 特有的 User-Agent 缺失或特定模式
			if !strings.Contains(dataStr, "User-Agent:") {
				return true
			}
		}
	}

	return false
}

// isSuspiciousPath 检查可疑路径
func (v *VShellDefense) isSuspiciousPath(path string) bool {
	suspiciousPaths := []string{
		"/ws",
		"/websocket",
		"/socket",
		"/connect",
		"/beacon",
		"/c2",
		"/shell",
		"/cmd",
		"/exec",
	}

	// 添加用户配置的路径
	suspiciousPaths = append(suspiciousPaths, v.config.CustomBlockPaths...)

	pathLower := strings.ToLower(path)
	for _, sp := range suspiciousPaths {
		if strings.HasPrefix(pathLower, strings.ToLower(sp)) {
			return true
		}
	}

	return false
}

// isVShellVersionHandshake 检查 VShell 版本握手
func (v *VShellDefense) isVShellVersionHandshake(data []byte) bool {
	// VShell 握手格式: [1字节长度=5][5字节版本"4.9.3"]
	// 或完整9字节: [5][4][.][9][.][3] 前面有长度标识

	if len(data) < 6 {
		return false
	}

	// 检查版本号特征
	versionPatterns := [][]byte{
		[]byte{0x05, '4', '.', '9', '.', '3'},                   // 带长度前缀
		[]byte("4.9.3"),                                         // 纯版本号
		[]byte{0x05, 0x00, 0x00, 0x00, '4', '.', '9', '.', '3'}, // 9字节格式
	}

	for _, pattern := range versionPatterns {
		if bytes.Contains(data, pattern) {
			return true
		}
	}

	// 检查已知的 VShell 版本号
	knownVersions := []string{
		"4.9.3", "4.9.2", "4.9.1", "4.9.0",
		"4.8.", "4.7.", "4.6.", "4.5.",
	}

	dataStr := string(data)
	for _, ver := range knownVersions {
		if strings.Contains(dataStr, ver) && len(data) < 100 {
			// 小数据包中包含版本号，很可能是握手
			return true
		}
	}

	return false
}

// isVShellCommand 检查 VShell 命令
func (v *VShellDefense) isVShellCommand(data []byte) bool {
	if len(data) < 4 {
		return false
	}

	// 检查开头4字节命令标识
	commands := [][]byte{
		[]byte("conf"),
		[]byte("file"),
		[]byte("sucs"),
		[]byte("fail"),
		[]byte("ping"),
		[]byte("pong"),
		[]byte("exit"),
		[]byte("kill"),
	}

	prefix := data[:4]
	for _, cmd := range commands {
		if bytes.Equal(prefix, cmd) {
			return true
		}
	}

	// 检查数据包内部是否包含命令
	for _, cmd := range commands {
		if bytes.Contains(data, cmd) {
			// 检查是否在合理的位置 (通常在数据包开头或特定偏移)
			idx := bytes.Index(data, cmd)
			// VShell 协议中命令通常在前16字节内
			if idx >= 0 && idx < 16 {
				return true
			}
		}
	}

	return false
}

// isVShellEncryptedPayload 检查 VShell 加密载荷特征
func (v *VShellDefense) isVShellEncryptedPayload(data []byte) bool {
	// VShell 消息格式: [4字节长度][12字节nonce][加密数据]
	// 最小长度: 4 + 12 + 16(GCM tag) = 32 字节

	if len(data) < 32 {
		return false
	}

	// 解析长度字段
	length := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24

	// 检查长度是否合理且匹配
	if length > 0 && length < 65536 {
		expectedTotal := int(length) + 4
		// 允许一定误差
		if expectedTotal >= len(data)-16 && expectedTotal <= len(data)+16 {
			// Nonce 第一个字节应该是清除了最高位的 (VShell特征: *v17 &= ~0x80u)
			nonce := data[4:16]
			if nonce[0]&0x80 == 0 {
				return true
			}
		}
	}

	// 检查二进制数据模式 (高熵值数据)
	if v.isHighEntropyData(data[4:]) && len(data) > 20 {
		return true
	}

	return false
}

// isVKeyHash 检查 Vkey 哈希特征
func (v *VShellDefense) isVKeyHash(data []byte) bool {
	// Vkey 哈希格式: 32字节十六进制字符串 (MD5)
	if len(data) == 32 {
		// 检查是否全部是十六进制字符
		for _, b := range data {
			if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
				return false
			}
		}
		return true
	}

	// 检查数据中是否包含32字节的十六进制字符串
	if len(data) >= 32 {
		hexPattern := regexp.MustCompile(`^[0-9a-fA-F]{32}$`)
		if hexPattern.Match(data[:32]) {
			return true
		}
	}

	return false
}

// isHighEntropyData 检查是否为高熵值数据 (可能是加密数据)
func (v *VShellDefense) isHighEntropyData(data []byte) bool {
	if len(data) < 16 {
		return false
	}

	// 计算字节分布
	counts := make(map[byte]int)
	for _, b := range data {
		counts[b]++
	}

	// 高熵值数据特征: 字节分布均匀
	uniqueBytes := len(counts)
	expectedUnique := len(data) / 4 // 加密数据通常有很高的唯一字节比例

	return uniqueBytes >= expectedUnique
}

// CheckKnownVkey 检查是否为已知的 Vkey
func (v *VShellDefense) CheckKnownVkey(vkey string) bool {
	hash := md5.Sum([]byte(vkey))
	hashStr := hex.EncodeToString(hash[:])

	// 检查是否在黑名单中
	for _, blocked := range v.config.BlockedVkeys {
		if blocked == vkey || blocked == hashStr {
			return true
		}
	}

	return false
}

// updateConnectionTracker 更新连接追踪
func (v *VShellDefense) updateConnectionTracker(clientIP string, data []byte) {
	v.mu.Lock()
	defer v.mu.Unlock()

	tracker, exists := v.connections[clientIP]
	if !exists {
		tracker = &VShellConnectionTracker{
			FirstSeen: time.Now(),
		}
		v.connections[clientIP] = tracker
	}

	tracker.LastSeen = time.Now()

	// 检测可疑行为并增加分数
	dataStr := string(data)

	if strings.Contains(dataStr, "Upgrade: websocket") {
		tracker.WebSocketUpgrade = true
		tracker.SuspiciousScore += 10
	}

	if v.isVShellVersionHandshake(data) {
		tracker.HandshakeDetected = true
		tracker.SuspiciousScore += 50
	}

	if v.isVShellCommand(data) {
		tracker.SuspiciousScore += 100
	}
}

// cleanupRoutine 定期清理过期的连接追踪
func (v *VShellDefense) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		v.mu.Lock()
		cutoff := time.Now().Add(-30 * time.Minute)
		for ip, tracker := range v.connections {
			if tracker.LastSeen.Before(cutoff) {
				delete(v.connections, ip)
			}
		}
		v.mu.Unlock()
	}
}

// GetStats 获取统计信息
func (v *VShellDefense) GetStats() VShellDefenseStats {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.stats
}

// IsConnectionSuspicious 检查连接是否可疑
func (v *VShellDefense) IsConnectionSuspicious(clientIP string) (bool, int) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	tracker, exists := v.connections[clientIP]
	if !exists {
		return false, 0
	}

	// 阈值: 50分以上认为可疑
	return tracker.SuspiciousScore >= 50, tracker.SuspiciousScore
}

// GenerateVShellSignature 生成 VShell 流量特征签名 (用于IDS/IPS)
func GenerateVShellSignatures() []string {
	return []string{
		// Snort/Suricata 规则格式
		`alert tcp any any -> any any (msg:"VShell WebSocket Upgrade"; content:"GET /ws "; content:"Upgrade: websocket"; content:"Sec-WebSocket-Version: 13"; sid:1000001; rev:1;)`,
		`alert tcp any any -> any any (msg:"VShell Version Handshake"; content:"|05|4.9.3"; sid:1000002; rev:1;)`,
		`alert tcp any any -> any any (msg:"VShell Command conf"; content:"conf"; offset:0; depth:4; sid:1000003; rev:1;)`,
		`alert tcp any any -> any any (msg:"VShell Command file"; content:"file"; offset:0; depth:4; sid:1000004; rev:1;)`,
		`alert tcp any any -> any any (msg:"VShell Beacon Pattern"; content:"{\"Id\":"; content:"\"HostName\":"; sid:1000005; rev:1;)`,
	}
}

// LogVShellAttempt 记录 VShell 攻击尝试
func LogVShellAttempt(clientIP, reason, threatLevel string, details map[string]interface{}) {
	log.Printf("[VSHELL-DEFENSE] BLOCKED | IP: %s | Reason: %s | Threat: %s | Details: %v",
		clientIP, reason, threatLevel, details)
}
