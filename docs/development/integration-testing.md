# Integration Testing Guide

This document describes the integration testing strategy for Jack and provides instructions for running system-level tests that require privileged operations.

## Overview

Jack's architecture presents unique testing challenges:

1. **System Dependencies**: Network operations require privileged access (CAP_NET_ADMIN, CAP_NET_RAW)
2. **Plugin Architecture**: Plugins run as separate RPC processes over stdin/stdout
3. **State Management**: Configuration persisted to disk with atomic file operations
4. **Unix Socket Communication**: Daemon listens on `/var/run/jack.sock` (hardcoded)

## Recent Achievements - November 2025 ✅

**Status**: All 76 integration tests now passing (100%)!

Jack has achieved comprehensive integration testing coverage through a three-phase implementation:

- **Phase 1**: Foundation & daemon lifecycle (3 tests)
- **Phase 2**: Network operations - interfaces, bridges, VLANs, routes (23 tests)
- **Phase 3**: Configuration workflow & validation (50 tests)

**Total**: 76 tests across 13 test files (~7,600 lines of test infrastructure)

### Test Execution Success
- All tests passing in isolated network namespaces
- Docker-ready for CI/CD pipelines
- Average execution time: ~0.25 seconds per test
- Zero host environment impact (complete isolation)

### Coverage Highlights
- **Network operations**: Bridges, VLANs, physical interfaces, static routes
- **Configuration workflow**: Set, commit, apply, diff, revert, get
- **Validation**: 15+ error cases with boundary testing
- **Error handling**: Daemon resilience under concurrent failures
- **State management**: Persistence across daemon restarts

## Current Test Coverage

As of November 2025:

| Package | Coverage | Test Files | Status |
|---------|----------|------------|--------|
| **test/integration** | **76 tests** | **13 files** | **✅ 100% pass** |
| daemon | 52.5% | 7 test files (5 unit + 2 integration) | ✅ Enhanced |
| state | 78-100% | 4 test files (2 unit + 2 integration) | ✅ Enhanced |
| plugins | RPC tested | rpc_test.go + integration | ✅ Core complete |
| types | [no statements] | types_test.go | ✅ Complete |
| client | 0% | (none) | Covered via integration |
| cmd | 0% | (none) | Covered via integration |
| system | 0% | (none) | Covered via integration |

**Overall Project Coverage**: ~18% (realistic for system-level tool with privileged operations)

## Test Organization

### Unit Tests (Current Focus)

**Location**: `*_test.go` files alongside source code

**What's tested**:
- Protocol message marshaling/unmarshaling
- Configuration diffing logic
- Plugin loader wrapper methods
- Plugin registry operations
- File I/O and state management
- Type JSON serialization
- RPC provider interfaces

**Run with**:
```bash
go test ./...
```

**Advantages**:
- Fast execution
- No system dependencies
- Run in CI/CD pipelines
- Catch regressions early

**Limitations**:
- Cannot test actual network changes
- Cannot test real plugin RPC communication
- Cannot test daemon startup/shutdown
- Cannot test CLI end-to-end workflows

### Integration Tests (Future Work)

**Location**: `test/integration/` (to be created)

**What should be tested**:
1. **Full daemon lifecycle**
   - Daemon starts and creates socket
   - Client connects via Unix socket
   - Configuration loaded from disk
   - Plugins discovered and loaded
   - Daemon shuts down gracefully

2. **Configuration workflows**
   - `jack get` retrieves current config
   - `jack set` stages changes
   - `jack diff` shows pending changes
   - `jack commit` persists to disk
   - `jack apply` activates changes
   - `jack revert` discards pending changes

3. **Plugin integration**
   - Plugin processes spawn correctly
   - RPC communication works (stdin/stdout)
   - Metadata queries succeed
   - Config validation and application
   - Plugin status reporting
   - CLI command execution
   - Plugin enable/disable

4. **System operations** (privileged)
   - Interface creation (bridges, VLANs)
   - IP address assignment
   - Static route management
   - Firewall rule application (nftables)
   - WireGuard tunnel setup
   - DHCP/DNS server configuration (dnsmasq)

## Testing Environments

### Option 1: QEMU VM (Recommended)

**Advantages**:
- Full system isolation
- Privileged operations allowed
- Matches production environment (arm64 Debian)
- Network namespace testing possible
- Can test daemon as systemd service

**Setup**:

1. **Install QEMU and dependencies**:
```bash
sudo apt-get install qemu-system-aarch64 qemu-efi-aarch64
```

2. **Create Debian arm64 VM**:
```bash
# Download Debian arm64 installer
wget https://cdimage.debian.org/debian-cd/current/arm64/iso-cd/debian-12.4.0-arm64-netinst.iso

# Create disk image
qemu-img create -f qcow2 debian-arm64.qcow2 20G

# Install Debian
qemu-system-aarch64 \
  -M virt \
  -cpu cortex-a72 \
  -m 2048 \
  -bios /usr/share/qemu-efi-aarch64/QEMU_EFI.fd \
  -drive file=debian-arm64.qcow2,if=virtio \
  -cdrom debian-12.4.0-arm64-netinst.iso \
  -net nic -net user,hostfwd=tcp::2222-:22 \
  -nographic
```

3. **Install Go and build tools in VM**:
```bash
ssh -p 2222 root@localhost

# Inside VM
apt-get update
apt-get install -y git build-essential wget

# Install Go 1.21+
wget https://go.dev/dl/go1.21.0.linux-arm64.tar.gz
tar -C /usr/local -xzf go1.21.0.linux-arm64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

4. **Clone and build Jack**:
```bash
git clone <repo-url> /opt/jack
cd /opt/jack
go build -o jack
go build -o bin/jack-plugin-nftables plugins/core/nftables/*.go
go build -o bin/jack-plugin-dnsmasq plugins/core/dnsmasq/*.go
go build -o bin/jack-plugin-wireguard plugins/core/wireguard/*.go
go build -o bin/jack-plugin-monitoring plugins/core/monitoring/*.go
go build -o bin/jack-plugin-leds plugins/core/leds/*.go
```

5. **Run integration tests**:
```bash
# Run tests with privileges
sudo go test ./test/integration -v

# Or as root
su -
cd /opt/jack
go test ./test/integration -v
```

**Network isolation** (optional):
```bash
# Create network namespace for testing
ip netns add jack-test
ip netns exec jack-test bash

# Inside namespace - run tests
cd /opt/jack
go test ./test/integration -v
```

### Option 2: Docker Container with Privileges

**Advantages**:
- Faster than VM
- Easier CI/CD integration
- Reproducible builds

**Limitations**:
- May have restrictions on network operations
- Systemd testing harder
- Cross-architecture emulation slower

**Setup**:

1. **Create Dockerfile**:
```dockerfile
FROM debian:bookworm

# Install dependencies
RUN apt-get update && apt-get install -y \
    git \
    build-essential \
    wget \
    nftables \
    dnsmasq \
    wireguard-tools \
    iproute2 \
    iptables

# Install Go
RUN wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz && \
    rm go1.21.0.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin

# Copy source code
WORKDIR /opt/jack
COPY . .

# Build Jack
RUN go build -o jack && \
    go build -o bin/jack-plugin-nftables plugins/core/nftables/*.go && \
    go build -o bin/jack-plugin-dnsmasq plugins/core/dnsmasq/*.go && \
    go build -o bin/jack-plugin-wireguard plugins/core/wireguard/*.go && \
    go build -o bin/jack-plugin-monitoring plugins/core/monitoring/*.go

CMD ["/bin/bash"]
```

2. **Build and run with privileges**:
```bash
docker build -t jack-test .
docker run --rm --privileged --cap-add=ALL --network=host -it jack-test

# Inside container
go test ./test/integration -v
```

### Option 3: Local System (Development Only)

**Warning**: Running integration tests on your development machine can:
- Modify network configuration
- Create/delete interfaces
- Change firewall rules
- Interfere with existing network services

**Use network namespaces for isolation**:
```bash
# Create isolated namespace
sudo ip netns add jack-test

# Enter namespace
sudo ip netns exec jack-test bash

# Run tests (network changes isolated)
cd /path/to/jack
go test ./test/integration -v

# Exit and cleanup
exit
sudo ip netns delete jack-test
```

## Writing Integration Tests

### Basic Structure

```go
// test/integration/daemon_test.go
package integration

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
)

func TestDaemonStartup(t *testing.T) {
	// Require root for integration tests
	if os.Geteuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	// Use temp directory for configs
	tmpDir := t.TempDir()
	os.Setenv("JACK_CONFIG_DIR", tmpDir)
	defer os.Setenv("JACK_CONFIG_DIR", "")

	// Start daemon in background
	srv, err := daemon.NewServer()
	require.NoError(t, err)

	go func() {
		err := srv.Run()
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()
	defer srv.Shutdown()

	// Wait for daemon to start
	time.Sleep(100 * time.Millisecond)

	// Test client connection
	req := daemon.Request{Command: "status"}
	resp, err := client.Send(req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}
```

### Testing Plugins

```go
func TestPluginRPCCommunication(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	// Load plugin
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	// Check metadata
	metadata := plugin.Metadata()
	assert.Equal(t, "monitoring", metadata.Namespace)

	// Test config application
	config := map[string]interface{}{
		"enabled": true,
		"interval": 5,
	}
	err = plugin.ApplyConfig(config)
	assert.NoError(t, err)

	// Get status
	status, err := plugin.Status()
	require.NoError(t, err)
	assert.NotNil(t, status)
}
```

### Testing Network Operations

```go
func TestInterfaceCreation(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	// Test bridge creation
	err := system.CreateBridge("br-test", []string{"dummy0"})
	require.NoError(t, err)
	defer system.DeleteInterface("br-test")

	// Verify interface exists
	exists, err := system.InterfaceExists("br-test")
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify interface type
	ifaceType, err := system.GetInterfaceType("br-test")
	require.NoError(t, err)
	assert.Equal(t, "bridge", ifaceType)
}
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Run integration tests in privileged container
      run: |
        docker build -t jack-test -f Dockerfile.test .
        docker run --rm --privileged --cap-add=ALL jack-test \
          go test ./test/integration -v
```

### Self-Hosted Runner (QEMU)

For arm64 testing, use a self-hosted runner:

```yaml
jobs:
  integration-arm64:
    runs-on: self-hosted

    steps:
    - uses: actions/checkout@v3

    - name: Run integration tests
      run: |
        ssh qemu-vm "cd /opt/jack && git pull && go test ./test/integration -v"
```

## Best Practices

### Test Isolation

1. **Use temp directories** for all config files:
```go
tmpDir := t.TempDir()
os.Setenv("JACK_CONFIG_DIR", tmpDir)
defer os.Setenv("JACK_CONFIG_DIR", "")
```

2. **Clean up resources** in defer statements:
```go
plugin, err := LoadPlugin("test")
require.NoError(t, err)
defer plugin.Close()
```

3. **Use unique interface names** to avoid conflicts:
```go
ifaceName := fmt.Sprintf("test-%d", time.Now().UnixNano())
```

### Privileged Tests

1. **Skip if not root**:
```go
if os.Geteuid() != 0 {
	t.Skip("This test requires root privileges")
}
```

2. **Use build tags** for integration tests:
```go
//go:build integration
// +build integration

package integration
```

Run with:
```bash
go test -tags=integration ./test/integration
```

### Error Handling

1. **Use require for critical failures**:
```go
require.NoError(t, err, "Failed to start daemon")
```

2. **Use assert for non-critical checks**:
```go
assert.Equal(t, expected, actual, "Unexpected value")
```

3. **Provide context in error messages**:
```go
assert.True(t, exists, "Interface %s should exist after creation", ifaceName)
```

## Roadmap

### ✅ Completed (November 2025)

- [x] Refactor daemon to accept configurable socket path
- [x] Create `test/integration/` directory structure
- [x] Write basic daemon lifecycle tests (Phase 1: 3 tests)
- [x] Document integration testing strategy
- [x] Full end-to-end workflow tests (set → commit → apply)
- [x] Network operation tests (bridges, VLANs, routes - Phase 2: 23 tests)
- [x] Configuration workflow & validation (Phase 3: 50 tests)
- [x] CI/CD pipeline with Docker
- [x] Test coverage achievement: 76 integration tests, ~18% overall project coverage
- [x] Error handling and daemon resilience tests
- [x] State persistence testing

### Future Phases (Next Quarter)

- [ ] Plugin functionality testing (Phase 4: nftables, dnsmasq, wireguard)
- [ ] Plugin RPC advanced integration tests (Phase 5)
- [ ] QEMU VM automation (Terraform/Packer)
- [ ] Self-hosted arm64 runner for CI
- [ ] Performance benchmarks (Phase 6)
- [ ] Stress testing (concurrent operations, high load)
- [ ] Security testing (fuzzing, privilege escalation scenarios)
- [ ] Test coverage target: 30%+ overall (realistic for system-level tool)

## Troubleshooting

### Tests Fail with "permission denied"

**Cause**: Insufficient privileges for network operations

**Solution**: Run tests as root or with CAP_NET_ADMIN:
```bash
sudo go test ./test/integration -v
```

### "Address already in use" errors

**Cause**: Previous test run didn't clean up socket

**Solution**: Remove stale socket:
```bash
sudo rm -f /var/run/jack.sock
```

### Plugin not found errors

**Cause**: Plugin binaries not in search path

**Solution**: Build plugins and set JACK_PLUGIN_DIR:
```bash
./build.sh
export JACK_PLUGIN_DIR=./bin
go test ./test/integration -v
```

### RPC timeout errors

**Cause**: Plugin process hung or crashed

**Solution**: Check plugin logs and increase timeout:
```bash
export JACK_RPC_TIMEOUT=30
go test ./test/integration -v
```

## References

- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Network Namespaces](https://man7.org/linux/man-pages/man8/ip-netns.8.html)
- [QEMU Documentation](https://www.qemu.org/docs/master/)
- [Docker Privileged Mode](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities)
