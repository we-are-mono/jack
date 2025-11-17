# Jack Development Guide

## Prerequisites

- Go 1.21 or later
- Linux system (for development/testing)
- Git

## Building from Source

### Clone Repository
```bash
git clone https://github.com/we-are-mono/jack.git
cd jack
```

### Build for Your Platform
```bash
go build -o jack main.go
./jack --version
```

### Cross-Compile for ARM64 (Gateway devices)
```bash
GOOS=linux GOARCH=arm64 go build -o jack-arm64 main.go
```

### Build Script

The project includes a `build.sh` script:
```bash
#!/bin/bash
set -e

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

GOOS=linux
GOARCH=arm64

echo "Building Jack for ${GOOS}/${GOARCH}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"

CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" \
    -o jack \
    .

echo "✓ Build complete: jack"
echo "  Size: $(ls -lh jack | awk '{print $5}')"
```

Usage:
```bash
chmod +x build.sh
./build.sh
```

## Project Structure
```
jack/
├── main.go                      # Entry point
├── cmd/                         # CLI commands
│   ├── root.go                 # Root command and version
│   ├── daemon.go               # Daemon mode
│   ├── apply.go                # Apply configuration
│   ├── get.go                  # Get config values
│   ├── set.go                  # Set config values
│   ├── commit.go               # Commit changes
│   ├── revert.go               # Revert changes
│   ├── diff.go                 # Show differences
│   └── status.go               # Show status
├── daemon/                      # Daemon implementation
│   ├── server.go               # Unix socket server
│   ├── state.go                # In-memory state management
│   ├── protocol.go             # Request/response protocol
│   └── diff.go                 # Configuration diffing
├── client/                      # CLI client
│   └── client.go               # Unix socket client
├── state/                       # Config file management
│   └── manager.go              # Load/save config files
├── system/                      # System interaction
│   ├── network.go              # Network interface management
│   ├── firewall.go             # Firewall management
│   └── dhcp.go                 # DHCP management
├── providers/                   # Provider implementations
│   ├── firewall/
│   │   ├── provider.go         # Firewall provider interface
│   │   ├── factory.go          # Provider factory
│   │   └── nftables.go         # nftables implementation
│   └── dhcp/
│       ├── provider.go         # DHCP provider interface
│       ├── factory.go          # Provider factory
│       └── dnsmasq.go          # dnsmasq implementation
├── types/                       # Data structures
│   ├── types.go                # Interface types
│   ├── firewall.go             # Firewall types
│   └── dhcp.go                 # DHCP types
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
└── build.sh                     # Build script
```

## Key Packages

### External Dependencies
```go
require (
    github.com/spf13/cobra v1.10.1          // CLI framework
    github.com/vishvananda/netlink v1.3.0   // Linux netlink library
)
```

### Internal Packages

- `cmd` - Cobra command definitions
- `daemon` - Background service implementation
- `client` - Unix socket client
- `state` - Configuration file I/O
- `system` - System-level operations (netlink, exec)
- `providers` - Abstraction for different implementations
- `types` - Shared data structures

## Development Workflow

### 1. Make Changes

Edit code in appropriate package.

### 2. Build
```bash
go build -o jack .
```

### 3. Test on Development Machine
```bash
# Create test config
sudo mkdir -p /etc/jack
sudo cp examples/interfaces.json /etc/jack/

# Run daemon (foreground for debugging)
sudo ./jack daemon

# In another terminal, test commands
./jack status
./jack get interfaces wan
```

### 4. Cross-Compile for Gateway
```bash
./build.sh
```

### 5. Deploy to Gateway
```bash
scp jack root@gateway:/usr/local/bin/
ssh root@gateway systemctl restart jack
```

## Testing

### Unit Tests
```bash
go test ./...
```

### Integration Tests
```bash
# Requires root and will modify network config!
sudo go test ./system -tags=integration
```

### Manual Testing

Create a test environment:
```bash
# Create namespace for isolated testing
sudo ip netns add jack-test
sudo ip netns exec jack-test bash

# Inside namespace, test Jack
```

## Code Style

### Go Formatting
```bash
# Format all code
go fmt ./...

# Run linter
go vet ./...
```

### Naming Conventions

- Packages: lowercase, single word (`daemon`, `system`)
- Files: lowercase with underscores (`unix_socket.go`)
- Interfaces: end with "Provider" or "Manager"
- Structs: CamelCase
- Functions: CamelCase (exported), camelCase (internal)

### Error Handling

Always wrap errors with context:
```go
if err != nil {
    return fmt.Errorf("failed to create bridge: %w", err)
}
```

### Logging

Use structured logging:
```go
log.Printf("[ERROR] Failed to apply config: %v", err)
log.Printf("[WARN] No firewall config found")
log.Printf("[INFO] Interface br-lan created")
```

## Adding New Features

### Adding a New Interface Type

1. Update `types/types.go` with new fields
2. Add handling in `system/network.go`
3. Update `daemon/diff.go` for diffing
4. Add tests

Example: Adding VXLAN support
```go
// types/types.go
type Interface struct {
    // ... existing fields
    
    // VXLAN-specific
    VXLANId    int    `json:"vxlan_id,omitempty"`
    VXLANLocal string `json:"vxlan_local,omitempty"`
    VXLANRemote string `json:"vxlan_remote,omitempty"`
}

// system/network.go
func applyVXLANInterface(name string, iface types.Interface) error {
    // Implementation
}

// Update ApplyInterfaceConfig switch
case "vxlan":
    return applyVXLANInterface(name, iface)
```

### Adding a New Provider

1. Define interface in `providers/<type>/provider.go`
2. Implement in `providers/<type>/<impl>.go`
3. Update factory in `providers/<type>/factory.go`
4. Add template if needed

Example: Adding ISC Kea DHCP provider
```bash
# Create files
touch providers/dhcp/kea.go

# Edit providers/dhcp/kea.go
package dhcp

type KeaProvider struct {
    configPath string
}

func NewKea() (*KeaProvider, error) {
    // Implementation
}

func (k *KeaProvider) ApplyConfig(config *types.DHCPConfig) error {
    // Implementation
}

// Edit providers/dhcp/factory.go
case "kea":
    return NewKea()
```

### Adding a New CLI Command

1. Create `cmd/<command>.go`
2. Register with root command
3. Implement handler

Example: Adding a `jack logs` command
```go
// cmd/logs.go
package cmd

import (
    "fmt"
    
    "github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "Show Jack daemon logs",
    Run:   runLogs,
}

func init() {
    rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) {
    // Implementation: exec journalctl -u jack
}
```

## Debugging

### Enable Verbose Logging
```bash
# Run daemon with debug output
sudo ./jack daemon --verbose
```

### Trace netlink Calls
```bash
# Use strace to see netlink system calls
sudo strace -e trace=network,socket ./jack apply
```

### Inspect nftables Rules
```bash
# View raw nftables output
sudo nft list ruleset

# Watch rule changes in real-time
sudo watch -n 1 'nft list ruleset | grep -A 5 "chain forward"'
```

### Check Unix Socket
```bash
# Verify socket exists
ls -l /var/run/jack.sock

# Test socket communication
sudo nc -U /var/run/jack.sock
{"command":"status"}
```

## Performance Profiling

### CPU Profiling
```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Then:
```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Memory Profiling
```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

## Release Process

### 1. Version Bump
```bash
# Tag new version
git tag v0.2.0
git push origin v0.2.0
```

### 2. Build Releases
```bash
# Build for multiple architectures
GOOS=linux GOARCH=amd64 go build -o jack-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o jack-linux-arm64 .
GOOS=linux GOARCH=arm GOARM=7 go build -o jack-linux-armv7 .
```

### 3. Create GitHub Release
```bash
gh release create v0.2.0 \
    jack-linux-amd64 \
    jack-linux-arm64 \
    jack-linux-armv7 \
    --title "Jack v0.2.0" \
    --notes "Release notes here"
```

## Contributing

### Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`go test ./...`)
5. Format code (`go fmt ./...`)
6. Commit (`git commit -m 'Add amazing feature'`)
7. Push (`git push origin feature/amazing-feature`)
8. Open Pull Request

### Commit Messages

Follow conventional commits:
```
feat: add VLAN support
fix: resolve bridge creation race condition
docs: update configuration reference
refactor: simplify netlink error handling
test: add unit tests for firewall provider
```

### Code Review Guidelines

- Keep PRs focused (one feature/fix per PR)
- Add tests for new features
- Update documentation
- Maintain backward compatibility

## Getting Help

- GitHub Issues: https://github.com/we-are-mono/jack/issues
- GitHub Discussions: https://github.com/we-are-mono/jack/discussions
- Documentation: https://github.com/we-are-mono/jack/tree/main/docs