package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	LISTEN_PORT  = ":8880"
	BACKEND_ADDR = "127.0.0.1:9991"
	BUFFER_SIZE  = 32768 // 32KB buffer
)

func main() {
	listener, err := net.Listen("tcp", LISTEN_PORT)
	if err != nil {
		log. Fatalf("Failed to listen on %s: %v", LISTEN_PORT, err)
	}
	defer listener.Close()

	log.Printf("Proxy server listening on %s, forwarding to %s", LISTEN_PORT, BACKEND_ADDR)

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		go handleConnection(clientConn)
	}
}

func handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// 设置初始读取超时（30秒），防止恶意空连接占用资源
	clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// 读取初始数据块（最多4KB用于检测）
	reader := bufio.NewReader(clientConn)
	buf := make([]byte, 4096)
	n, err := reader.Read(buf)
	if err != nil {
		log.Printf("Error reading initial data: %v", err)
		return
	}
	initialData := buf[:n]

	// 读取到数据后，立即移除超时限制，支持长连接
	clientConn.SetReadDeadline(time.Time{})

	// 尝试解析为 HTTP 请求
	if isHTTPRequest(initialData) {
		// 查找第一行（请求行）
		firstLineEnd := findFirstLine(initialData)
		if firstLineEnd > 0 {
			requestLine := string(initialData[:firstLineEnd])
			path := extractHTTPPath(requestLine)

			// 检查是否是 /slt 路径
			if path == "/slt" {
				log.Printf("Blocked /slt request from %s", clientConn.RemoteAddr())
				send404Response(clientConn)
				return
			}

			// 检查是否是 /swt 路径
			if path == "/swt" {
				log.Printf("Blocked /swt request from %s", clientConn.RemoteAddr())
				send404Response(clientConn)
				return
			}

			log.Printf("Forwarding HTTP request: %s from %s", strings.TrimSpace(requestLine), clientConn.RemoteAddr())
		} else {
			log.Printf("Forwarding HTTP request from %s", clientConn.RemoteAddr())
		}
		forwardHTTPRequest(clientConn, reader, initialData)
	} else {
		// 作为 raw TCP 转发
		log.Printf("Forwarding raw TCP connection from %s", clientConn.RemoteAddr())
		forwardRawTCP(clientConn, reader, initialData)
	}
}

// 查找第一行的结束位置
func findFirstLine(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			return i
		}
	}
	return -1
}

// 判断是否是 HTTP 请求
func isHTTPRequest(data []byte) bool {
	line := string(data)
	// 检查是否以 HTTP 方法开头
	methods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH ", "CONNECT ", "TRACE "}
	for _, method := range methods {
		if strings.HasPrefix(line, method) {
			return true
		}
	}
	return false
}

// 提取 HTTP 路径
func extractHTTPPath(requestLine string) string {
	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// 发送 404 响应
func send404Response(conn net.Conn) {
	response := "HTTP/1.1 404 Not Found\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 9\r\n" +
		"Connection: close\r\n" +
		"\r\n" +
		"Not Found"
	conn.Write([]byte(response))
}

// 转发 HTTP 请求
func forwardHTTPRequest(clientConn net.Conn, reader *bufio.Reader, initialData []byte) {
	// 连接后端
	backendConn, err := net.Dial("tcp", BACKEND_ADDR)
	if err != nil {
		log.Printf("Failed to connect to backend: %v", err)
		send502Response(clientConn)
		return
	}
	defer backendConn.Close()

	// 发送初始数据
	if _, err := backendConn.Write(initialData); err != nil {
		log.Printf("Error writing to backend: %v", err)
		return
	}

	// 启动双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	// 客户端 -> 后端（从 reader 读取剩余数据）
	go func() {
		defer wg.Done()
		io.Copy(backendConn, reader)
		backendConn.(*net.TCPConn).CloseWrite()
	}()

	// 后端 -> 客户端
	go func() {
		defer wg.Done()
		io.Copy(clientConn, backendConn)
		clientConn.(*net.TCPConn).CloseWrite()
	}()

	wg.Wait()
}

// 转发原始 TCP 连接
func forwardRawTCP(clientConn net.Conn, reader *bufio.Reader, initialData []byte) {
	// 连接后端
	backendConn, err := net.Dial("tcp", BACKEND_ADDR)
	if err != nil {
		log.Printf("Failed to connect to backend: %v", err)
		return
	}
	defer backendConn.Close()

	// 发送初始数据
	if _, err := backendConn.Write(initialData); err != nil {
		log.Printf("Error writing to backend: %v", err)
		return
	}

	// 启动双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	// 客户端 -> 后端
	go func() {
		defer wg.Done()
		buf := make([]byte, BUFFER_SIZE)
		for {
			// 先读取 reader 中缓冲的数据
			if reader.Buffered() > 0 {
				n, err := reader.Read(buf)
				if n > 0 {
					if _, err := backendConn. Write(buf[:n]); err != nil {
						return
					}
				}
				if err != nil {
					return
				}
			} else {
				// reader 缓冲已空，直接从 conn 读取
				n, err := clientConn.Read(buf)
				if n > 0 {
					if _, err := backendConn.Write(buf[:n]); err != nil {
						return
					}
				}
				if err != nil {
					return
				}
			}
		}
	}()

	// 后端 -> 客户端
	go func() {
		defer wg.Done()
		io. Copy(clientConn, backendConn)
	}()

	wg.Wait()
}

// 发送 502 响应
func send502Response(conn net.Conn) {
	response := "HTTP/1.1 502 Bad Gateway\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: 15\r\n" +
		"Connection: close\r\n" +
		"\r\n" +
		"Bad Gateway"
	conn.Write([]byte(response))
}