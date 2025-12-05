# vshell-firewall

A lightweight TCP proxy with HTTP filtering capabilities for blocking specific paths.

## Features

- 🚀 High-performance TCP proxy
- 🔒 HTTP path filtering (blocks `/slt` by default)
- 🔄 Supports both HTTP and raw TCP connections
- ⚡ Long-lived TCP connection support
- 🛡️ Protection against idle connections with timeout
- 📊 Connection logging

## Architecture

```
Client --> vshell-firewall (Port 8880) --> Backend Service (Port 9991)
```

The proxy listens on port 8880 and forwards traffic to backend service on port 9991, with the ability to inspect and block specific HTTP requests.

## Requirements

- Go 1.16 or higher
- Linux (for systemd service)

## Quick Start

### Build

```bash
# Build the binary
make build

# Build with version information
make build-with-version

# Build for all platforms
make build-all
```

### Run Directly

```bash
# Build and run
make run

# Or run the binary directly
./build/slt-proxy
```

### Install as System Service

```bash
# Install binary and systemd service
make install-service

# Start the service
make start

# Enable on boot
make enable

# Check status
make status

# View logs
make logs
```

## Configuration

Edit the constants in `main.go`:

```go
const (
    LISTEN_PORT  = ":8880"           // Port to listen on
    BACKEND_ADDR = "127.0.0.1:9991"  // Backend service address
    BUFFER_SIZE  = 32768             // Buffer size (32KB)
)
```

To block different paths, modify the path check in `handleConnection()` function.

## Makefile Targets

```bash
make help          # Show all available targets
make build         # Build the binary
make run           # Build and run
make install       # Install binary to /usr/local/bin
make install-service  # Install systemd service
make start         # Start the service
make stop          # Stop the service
make restart       # Restart the service
make status        # Show service status
make logs          # Follow service logs
make enable        # Enable on boot
make disable       # Disable on boot
make uninstall     # Remove binary and service
make clean         # Clean build directory
```

## Service Management

After installing as a service:

```bash
# Start
sudo systemctl start slt-proxy

# Stop
sudo systemctl stop slt-proxy

# Restart
sudo systemctl restart slt-proxy

# Status
sudo systemctl status slt-proxy

# View logs
sudo journalctl -u slt-proxy -f

# Enable on boot
sudo systemctl enable slt-proxy

# Disable on boot
sudo systemctl disable slt-proxy
```

## How It Works

1. **Connection Handling**: Accepts incoming connections with a 30-second initial timeout
2. **Protocol Detection**: Reads the first 4KB to detect if it's HTTP or raw TCP
3. **Path Filtering**: For HTTP requests, checks if the path matches blocked patterns
4. **Proxying**: Forwards allowed traffic to backend with bi-directional streaming
5. **Long Connection Support**: After initial data exchange, removes timeouts for long-lived connections

## Development

```bash
# Format code
make fmt

# Run linter
make vet

# Tidy dependencies
make tidy

# Run tests
make test
```

## Cross Compilation

```bash
# Linux AMD64
make build-linux

# Linux ARM64
make build-linux-arm64

# All platforms
make build-all
```

## Logging

The proxy logs:
- Server startup information
- Blocked requests (with client IP)
- Forwarding details for both HTTP and raw TCP
- Connection errors

Example logs:
```
2025/12/05 10:00:00 Proxy server listening on :8880, forwarding to 127.0.0.1:9991
2025/12/05 10:00:10 Blocked /slt request from 192.168.1.100:45678
2025/12/05 10:00:15 Forwarding HTTP request: GET /api/data HTTP/1.1 from 192.168.1.101:45679
2025/12/05 10:00:20 Forwarding raw TCP connection from 192.168.1.102:45680
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
