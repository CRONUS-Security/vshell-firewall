package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	configFile = flag.String("config", "config.toml", "配置文件路径")
	version    = flag.Bool("version", false, "显示版本信息")
	
	// 版本信息 (通过 -ldflags 注入)
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	flag.Parse()

	// 显示版本信息
	if *version {
		fmt.Printf("vshell-firewall version %s\n", Version)
		fmt.Printf("Build time: %s\n", BuildTime)
		fmt.Printf("Git commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// 加载配置
	config, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded config with %d listener(s)", len(config.Listeners))

	// 启动所有监听器
	var wg sync.WaitGroup
	for _, listenerConfig := range config.Listeners {
		wg.Add(1)
		go func(cfg ListenerConfig) {
			defer wg.Done()
			startListener(cfg, config.Global)
		}(listenerConfig)
	}

	log.Println("All listeners started")
	wg.Wait()
}

// startListener 启动单个监听器
func startListener(cfg ListenerConfig, global GlobalConfig) {
	listener, err := net.Listen("tcp", cfg.ListenPort)
	if err != nil {
		log.Fatalf("[%s] Failed to listen on %s: %v", cfg.Name, cfg.ListenPort, err)
	}
	defer listener.Close()

	log.Printf("[%s] Listening on %s, forwarding to %s (timeout: %v)",
		cfg.Name, cfg.ListenPort, cfg.BackendAddr, cfg.Timeout.Enabled)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("[%s] Failed to accept connection: %v", cfg.Name, err)
			continue
		}

		go handleConnection(clientConn, cfg, global)
	}
}

// handleConnection 处理单个连接
func handleConnection(clientConn net.Conn, cfg ListenerConfig, global GlobalConfig) {
	defer clientConn.Close()

	// 设置初始读取超时
	if cfg.Timeout.Enabled && cfg.Timeout.InitialRead > 0 {
		clientConn.SetReadDeadline(time.Now().Add(time.Duration(cfg.Timeout.InitialRead) * time.Second))
	}

	// 读取初始数据块（最多4KB用于检测）
	reader := bufio.NewReader(clientConn)
	buf := make([]byte, 4096)
	n, err := reader.Read(buf)
	if err != nil {
		if global.LogLevel == "debug" {
			log.Printf("[%s] Error reading initial data from %s: %v",
				cfg.Name, clientConn.RemoteAddr(), err)
		}
		return
	}
	initialData := buf[:n]

	// 读取到数据后，根据配置决定是否移除超时限制
	if cfg.Timeout.Enabled && cfg.Timeout.InitialRead > 0 {
		clientConn.SetReadDeadline(time.Time{}) // 移除超时，支持长连接
	}

	// 检测是否为 HTTP 请求
	isHTTP := isHTTPRequest(initialData)

	// HTTP 协议处理
	if isHTTP {
		// 查找第一行（请求行）
		firstLineEnd := findFirstLine(initialData)
		var path string
		var requestLine string

		if firstLineEnd > 0 {
			requestLine = string(initialData[:firstLineEnd])
			path = extractHTTPPath(requestLine)
		}

		// 匹配 HTTP 处理器
		processor := cfg.MatchHTTPProcessor(path)
		if processor == nil {
			// 没有匹配的处理器，默认拒绝
			log.Printf("[%s] No HTTP processor matched for path '%s' from %s, dropping",
				cfg.Name, path, clientConn.RemoteAddr())
			sendErrorResponse(clientConn, "404")
			return
		}

		// 执行处理器动作
		if global.LogLevel == "debug" || global.LogLevel == "info" {
			log.Printf("[%s] HTTP request: %s from %s, action: %s",
				cfg.Name, strings.TrimSpace(requestLine), clientConn.RemoteAddr(), processor.Action)
		}

		switch processor.Action {
		case "drop":
			response := processor.Response
			if response == "" {
				response = "404"
			}
			log.Printf("[%s] Blocked request to '%s' from %s (response: %s)",
				cfg.Name, path, clientConn.RemoteAddr(), response)
			if response != "close" {
				sendErrorResponse(clientConn, response)
			}
			return

		case "file":
			// 返回文件内容
			serveFile(clientConn, processor.File, cfg.Name)
			return

		case "allow", "rewrite":
			// 允许通过或重写后转发
			forwardConnection(clientConn, reader, initialData, cfg, global, "HTTP", processor)
		}
	} else {
		// TCP 协议处理
		processor := cfg.MatchTCPProcessor()
		if processor == nil {
			log.Printf("[%s] No TCP processor configured, dropping connection from %s",
				cfg.Name, clientConn.RemoteAddr())
			return
		}

		if global.LogLevel == "debug" || global.LogLevel == "info" {
			log.Printf("[%s] TCP connection from %s, action: %s",
				cfg.Name, clientConn.RemoteAddr(), processor.Action)
		}

		switch processor.Action {
		case "drop":
			log.Printf("[%s] TCP connection from %s blocked by processor",
				cfg.Name, clientConn.RemoteAddr())
			return

		case "allow":
			forwardConnection(clientConn, reader, initialData, cfg, global, "TCP", processor)
		}
	}
}

// forwardConnection 转发连接到后端
func forwardConnection(clientConn net.Conn, reader *bufio.Reader, initialData []byte,
	cfg ListenerConfig, global GlobalConfig, protocol string, processor *Processor) {
	
	// 连接后端
	var backendConn net.Conn
	var err error
	
	if cfg.Timeout.Enabled && cfg.Timeout.ConnectBackend > 0 {
		backendConn, err = net.DialTimeout("tcp", cfg.BackendAddr,
			time.Duration(cfg.Timeout.ConnectBackend)*time.Second)
	} else {
		backendConn, err = net.Dial("tcp", cfg.BackendAddr)
	}
	
	if err != nil {
		log.Printf("[%s] Failed to connect to backend %s: %v",
			cfg.Name, cfg.BackendAddr, err)
		if protocol == "HTTP" {
			sendErrorResponse(clientConn, "502")
		}
		return
	}
	defer backendConn.Close()

	// 对于 HTTP 协议，如果配置了路径重写，则重写请求
	dataToSend := initialData
	if protocol == "HTTP" && processor != nil && processor.Action == "rewrite" && processor.RewriteTo != "" {
		paths := processor.GetPaths()
		if len(paths) > 0 {
			dataToSend = rewriteHTTPPath(initialData, paths[0], processor.RewriteTo)
			if global.LogLevel == "debug" {
				log.Printf("[%s] Rewriting path from %s to %s", cfg.Name, paths[0], processor.RewriteTo)
			}
		}
	}

	// 发送初始数据
	if _, err := backendConn.Write(dataToSend); err != nil {
		if global.LogLevel == "debug" {
			log.Printf("[%s] Error writing to backend: %v", cfg.Name, err)
		}
		return
	}

	// 启动双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	// 客户端 -> 后端
	go func() {
		defer wg.Done()
		buf := make([]byte, global.BufferSize)
		
		// 先读取 reader 缓冲中的剩余数据
		for reader.Buffered() > 0 {
			n, err := reader.Read(buf)
			if n > 0 {
				if _, err := backendConn.Write(buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
		
		// 然后直接从连接读取
		io.CopyBuffer(backendConn, clientConn, buf)
		if tcpConn, ok := backendConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	// 后端 -> 客户端
	go func() {
		defer wg.Done()
		buf := make([]byte, global.BufferSize)
		io.CopyBuffer(clientConn, backendConn, buf)
		if tcpConn, ok := clientConn.(*net.TCPConn); ok {
			tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
}

// isHTTPRequest 判断是否是 HTTP 请求
func isHTTPRequest(data []byte) bool {
	line := string(data)
	methods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH ", "CONNECT ", "TRACE "}
	for _, method := range methods {
		if strings.HasPrefix(line, method) {
			return true
		}
	}
	return false
}

// findFirstLine 查找第一行的结束位置
func findFirstLine(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			return i
		}
	}
	return -1
}

// extractHTTPPath 提取 HTTP 路径
func extractHTTPPath(requestLine string) string {
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// rewriteHTTPPath 重写 HTTP 请求路径
func rewriteHTTPPath(data []byte, fromPath, toPath string) []byte {
	// 查找请求行的结束位置
	firstLineEnd := -1
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			firstLineEnd = i
			break
		}
	}
	
	if firstLineEnd < 0 {
		return data
	}
	
	requestLine := string(data[:firstLineEnd])
	parts := strings.Split(requestLine, " ")
	
	// 检查是否需要重写路径
	if len(parts) >= 3 {
		path := parts[1]
		
		// 如果路径匹配 fromPath，则重写为 toPath
		if strings.HasPrefix(path, fromPath) {
			newPath := toPath + strings.TrimPrefix(path, fromPath)
			parts[1] = newPath
			
			// 重新构造请求行
			newRequestLine := strings.Join(parts, " ")
			
			// 构造新的请求数据
			result := make([]byte, 0, len(data))
			result = append(result, []byte(newRequestLine)...)
			result = append(result, data[firstLineEnd:]...)
			
			return result
		}
	}
	
	return data
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(conn net.Conn, responseType string) {
	var response string
	
	switch responseType {
	case "404":
		response = "HTTP/1.1 404 Not Found\r\n" +
			"Content-Type: text/plain\r\n" +
			"Content-Length: 9\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			"Not Found"
	case "403":
		response = "HTTP/1.1 403 Forbidden\r\n" +
			"Content-Type: text/plain\r\n" +
			"Content-Length: 9\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			"Forbidden"
	case "502":
		response = "HTTP/1.1 502 Bad Gateway\r\n" +
			"Content-Type: text/plain\r\n" +
			"Content-Length: 11\r\n" +
			"Connection: close\r\n" +
			"\r\n" +
			"Bad Gateway"
	default:
		return
	}
	
	conn.Write([]byte(response))
}

// serveFile 返回文件内容作为 HTTP 响应
func serveFile(conn net.Conn, filePath string, listenerName string) {
	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[%s] Error reading file %s: %v", listenerName, filePath, err)
		sendErrorResponse(conn, "404")
		return
	}

	// 检测 Content-Type
	contentType := "text/html; charset=utf-8"
	if strings.HasSuffix(filePath, ".json") {
		contentType = "application/json"
	} else if strings.HasSuffix(filePath, ".txt") {
		contentType = "text/plain; charset=utf-8"
	} else if strings.HasSuffix(filePath, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(filePath, ".js") {
		contentType = "application/javascript"
	}

	// 构造 HTTP 响应
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"Content-Type: %s\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"\r\n", contentType, len(data))

	conn.Write([]byte(response))
	conn.Write(data)
}
