# Jack Plugin Development Guide

This guide explains how to develop plugins for Jack using the modern **Direct RPC pattern**.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Provider Interface](#provider-interface)
4. [File Structure](#file-structure)
5. [Metadata Configuration](#metadata-configuration)
6. [Configuration Handling](#configuration-handling)
7. [CLI Commands](#cli-commands)
8. [Plugin Lifecycle](#plugin-lifecycle)
9. [Building & Deployment](#building--deployment)
10. [Testing](#testing)
11. [Common Pitfalls](#common-pitfalls)
12. [Complete Example](#complete-example)
13. [Migration Guide](#migration-guide)

## Overview

Jack uses a plugin-based architecture where all functionality beyond core routing and interface management is implemented as RPC-based plugins. Plugins communicate with the daemon using HashiCorp's go-plugin framework over stdin/stdout.

### Architecture

```
User CLI Command
    ↓ Unix Socket (/var/run/jack.sock)
Jack Daemon
    ↓ RPC (stdin/stdout)
Plugin Process
    ↓ System calls (ip, nft, systemctl, sysfs, etc.)
Operating System
```

### Core Plugins

- **nftables** - Firewall management with zones, NAT, port forwarding
- **wireguard** - VPN tunnel management (client and server)
- **dnsmasq** - DHCP and DNS server configuration
- **monitoring** - System health checks and real-time metrics
- **leds** - Hardware LED control via sysfs
- **sqlite3** - Database for logging and data storage

All plugins follow the **Direct RPC pattern** where they implement the `Provider` interface directly.

## Quick Start

The simplest possible plugin that works out of the box:

```go
// main.go
package main

import (
    "log"
    "os"
    jplugin "github.com/we-are-mono/jack/plugins"
)

func main() {
    log.SetOutput(os.Stderr)
    log.SetPrefix("[jack-plugin-example] ")
    jplugin.ServePlugin(NewExampleRPCProvider())
}

// rpc_provider.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/we-are-mono/jack/plugins"
)

type ExampleRPCProvider struct{}

func NewExampleRPCProvider() *ExampleRPCProvider {
    return &ExampleRPCProvider{}
}

func (p *ExampleRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:   "example",
        Version:     "1.0.0",
        Description: "Example plugin",
        ConfigPath:  "/etc/jack/example.json",
        DefaultConfig: map[string]interface{}{"enabled": true},
    }, nil
}

func (p *ExampleRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    return nil
}

func (p *ExampleRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
    return nil
}

func (p *ExampleRPCProvider) Flush(ctx context.Context) error {
    return nil
}

func (p *ExampleRPCProvider) Status(ctx context.Context) ([]byte, error) {
    return json.Marshal(map[string]interface{}{"enabled": true})
}

func (p *ExampleRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    return fmt.Errorf("plugin does not implement log event handling")
}

func (p *ExampleRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
    return nil, fmt.Errorf("plugin does not implement CLI commands")
}
```

Build and test:

```bash
cd plugins/core/example
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o jack-plugin-example .
sudo cp jack-plugin-example /usr/lib/jack/plugins/
jack plugin list  # Should show example plugin
```

## Provider Interface

All plugins **must** implement the `Provider` interface defined in `plugins/rpc.go`:

```go
type Provider interface {
    // Metadata returns plugin information
    Metadata(ctx context.Context) (MetadataResponse, error)

    // ApplyConfig applies configuration (config is JSON-encoded)
    ApplyConfig(ctx context.Context, configJSON []byte) error

    // ValidateConfig validates configuration (config is JSON-encoded)
    ValidateConfig(ctx context.Context, configJSON []byte) error

    // Flush removes all configuration
    Flush(ctx context.Context) error

    // Status returns current status (response is JSON-encoded)
    Status(ctx context.Context) ([]byte, error)

    // ExecuteCLICommand executes a CLI command provided by the plugin
    ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error)

    // OnLogEvent receives a log event from the daemon (optional)
    OnLogEvent(ctx context.Context, logEventJSON []byte) error
}
```

### Method Responsibilities

#### Metadata

Returns plugin information used by the daemon for discovery and routing.

```go
func (p *MyRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:     "myplugin",
        Version:       "1.0.0",
        Description:   "My plugin description",
        Category:      "mycategory",
        ConfigPath:    "/etc/jack/myplugin.json",
        DefaultConfig: nil,  // or map[string]interface{}{"setting": "value"}
        CLICommands:   []plugins.CLICommand{},  // Optional
    }, nil
}
```

#### ApplyConfig

Applies configuration to the system. Configuration is provided as JSON bytes - unmarshal to your own config struct.

```go
func (p *MyRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    var config MyConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to unmarshal config: %w", err)
    }

    // Apply configuration to system
    return p.provider.ApplyConfig(&config)
}
```

#### ValidateConfig

Validates configuration without applying it. Should check syntax and semantic correctness.

```go
func (p *MyRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
    var config MyConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    // Semantic validation
    if config.Port < 1 || config.Port > 65535 {
        return fmt.Errorf("port must be 1-65535, got %d", config.Port)
    }

    return nil
}
```

#### Flush

Removes all plugin-managed configuration from the system. Should be idempotent.

```go
func (p *MyRPCProvider) Flush(ctx context.Context) error {
    // Remove all configuration
    // Stop services, delete files, clear system state
    return p.provider.Flush()
}
```

#### Status

Returns current plugin status as JSON. Marshal your own status struct.

```go
func (p *MyRPCProvider) Status(ctx context.Context) ([]byte, error) {
    status, err := p.provider.Status()
    if err != nil {
        return nil, err
    }
    return json.Marshal(status)
}
```

#### ExecuteCLICommand

Executes custom CLI commands. Return formatted output as bytes.

```go
func (p *MyRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
    // Parse command and return formatted output
    // See "CLI Commands" section for details
    return nil, fmt.Errorf("plugin does not implement CLI commands")
}
```

#### OnLogEvent

Receives log events from the daemon. Optional - return error if not implemented.

```go
func (p *MyRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    return fmt.Errorf("plugin does not implement log event handling")
}
```

For plugins that do implement it (like sqlite3):

```go
func (p *MyRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    var logEvent map[string]interface{}
    if err := json.Unmarshal(logEventJSON, &logEvent); err != nil {
        return fmt.Errorf("failed to unmarshal log event: %w", err)
    }

    // Process log event
    return p.provider.InsertLog(logEvent)
}
```

## File Structure

Standard plugin layout (followed by all core plugins):

```
plugins/core/<plugin-name>/
├── main.go                      # Entry point
├── rpc_provider.go              # Provider interface implementation
├── provider.go                  # Core business logic (optional)
├── types.go                     # Configuration and status structs
├── <plugin>_config.go           # Config generation (if applicable)
├── <plugin>_config_test.go      # Config tests
└── <plugin>.go                  # System interaction code
```

### File Responsibilities

**main.go** - Plugin entry point:

```go
package main

import (
    "log"
    "os"
    jplugin "github.com/we-are-mono/jack/plugins"
)

func main() {
    // IMPORTANT: Log to stderr, stdout is used for RPC
    log.SetOutput(os.Stderr)
    log.SetPrefix("[jack-plugin-myplugin] ")

    log.Println("Starting myplugin...")

    // Create and serve RPC provider
    provider := NewMyRPCProvider()
    jplugin.ServePlugin(provider)
}
```

**rpc_provider.go** - RPC layer, handles JSON marshaling:

```go
package main

import (
    "context"
    "encoding/json"
    "github.com/we-are-mono/jack/plugins"
)

type MyRPCProvider struct {
    provider *MyProvider  // Optional: separate business logic
}

func NewMyRPCProvider() *MyRPCProvider {
    return &MyRPCProvider{
        provider: NewMyProvider(),
    }
}

// Implement all Provider interface methods...
```

**types.go** - Configuration and status structs:

```go
package main

type MyConfig struct {
    Enabled bool   `json:"enabled"`
    Setting string `json:"setting"`
}

type MyStatus struct {
    Enabled bool   `json:"enabled"`
    Active  bool   `json:"active"`
    Message string `json:"message,omitempty"`
}
```

**provider.go** - Core business logic (optional for complex plugins):

```go
package main

type MyProvider struct {
    config *MyConfig
}

func NewMyProvider() *MyProvider {
    return &MyProvider{}
}

func (p *MyProvider) ApplyConfig(config *MyConfig) error {
    p.config = config
    // Apply configuration to system
    return nil
}

func (p *MyProvider) Status() (*MyStatus, error) {
    return &MyStatus{
        Enabled: p.config != nil && p.config.Enabled,
        Active:  true,
    }, nil
}
```

## Metadata Configuration

The `MetadataResponse` structure tells the daemon how to interact with your plugin:

```go
type MetadataResponse struct {
    Namespace     string                 `json:"namespace"`              // REQUIRED
    Version       string                 `json:"version"`                // REQUIRED
    Description   string                 `json:"description"`            // REQUIRED
    Category      string                 `json:"category,omitempty"`     // OPTIONAL
    ConfigPath    string                 `json:"config_path"`            // REQUIRED
    DefaultConfig map[string]interface{} `json:"default_config,omitempty"` // OPTIONAL
    Dependencies  []string               `json:"dependencies,omitempty"` // OPTIONAL
    PathPrefix    string                 `json:"path_prefix,omitempty"`  // OPTIONAL
    CLICommands   []CLICommand           `json:"cli_commands,omitempty"` // OPTIONAL
}
```

### Field Details

**Namespace** (required) - Unique identifier for your plugin:
- Used for routing configuration to the correct plugin
- Convention: lowercase, single word
- Examples: `"firewall"`, `"vpn"`, `"monitoring"`, `"led"`

**Version** (required) - Plugin version:
- Semantic versioning recommended: `"1.0.0"`, `"1.2.3"`
- Displayed in `jack plugin list`

**Description** (required) - Human-readable description:
- One line, concise
- Example: `"System LED control via sysfs"`

**Category** (optional) - Groups related plugins:
- Standard categories: `"firewall"`, `"vpn"`, `"dhcp"`, `"hardware"`, `"monitoring"`, `"database"`
- Displayed in plugin listings

**ConfigPath** (required) - Expected configuration file path:
- Convention: `/etc/jack/<namespace>.json`
- Example: `"/etc/jack/monitoring.json"`

**DefaultConfig** (optional) - Default configuration:
- If provided, plugin works without config file
- Used when `/etc/jack/<namespace>.json` doesn't exist
- Example:
  ```go
  DefaultConfig: map[string]interface{}{
      "enabled": true,
      "interval": 5,
  }
  ```

**Dependencies** (optional) - Other plugins this depends on:
- List of namespace strings
- Example: `[]string{"firewall", "vpn"}`
- Currently informational only

**PathPrefix** (optional) - Auto-prepended to path queries:
- Simplifies configuration access
- Example: LED plugin uses `"leds"` so `led.status:green` becomes `led.leds.status:green`

**CLICommands** (optional) - Custom CLI commands:
- See "CLI Commands" section for details

## Configuration Handling

### Loading Priority

The daemon loads plugin configurations in this order:

1. **Configuration file** (`/etc/jack/<namespace>.json`)
2. **DefaultConfig** from metadata (if config file doesn't exist)
3. **Skip plugin** if neither exists

### Configuration Patterns

#### Pattern 1: No Default Config (Explicit Configuration Required)

Used by plugins that require specific user configuration (nftables, wireguard, dnsmasq):

```go
func (p *MyRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:     "myplugin",
        ConfigPath:    "/etc/jack/myplugin.json",
        DefaultConfig: nil,  // No defaults - user must configure
    }, nil
}
```

Plugin won't activate until user creates `/etc/jack/myplugin.json`.

#### Pattern 2: Default Config (Works Out of Box)

Used by plugins that can work with sensible defaults (monitoring, sqlite3):

```go
func (p *MyRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:   "monitoring",
        ConfigPath:  "/etc/jack/monitoring.json",
        DefaultConfig: map[string]interface{}{
            "enabled":             true,
            "collection_interval": 5,
        },
    }, nil
}
```

Plugin activates immediately with defaults. User can override by creating config file.

#### Pattern 3: Empty Config Allowed

Used by plugins that query hardware/system state (leds):

```go
func (p *MyRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:     "led",
        ConfigPath:    "/etc/jack/led.json",
        DefaultConfig: nil,  // No defaults
    }, nil
}

func (p *MyRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    var config LEDConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return err
    }

    // Empty config is OK - we'll just query current state
    return p.provider.ApplyConfig(&config)
}
```

### Validation Best Practices

Always validate both syntax (JSON) and semantics:

```go
func (p *MyRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
    // Syntax validation
    var config MyConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("invalid JSON syntax: %w", err)
    }

    // Semantic validation
    if config.RequiredField == "" {
        return fmt.Errorf("required_field must be set")
    }

    if config.Port < 1 || config.Port > 65535 {
        return fmt.Errorf("port must be 1-65535, got %d", config.Port)
    }

    if config.Interval < 0 {
        return fmt.Errorf("interval cannot be negative")
    }

    return nil
}
```

## CLI Commands

Plugins can provide custom CLI commands that appear automatically in `jack --help`.

### Command Declaration

Declare commands in your metadata:

```go
func (p *MyRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        // ... other fields ...
        CLICommands: []plugins.CLICommand{
            {
                Name:         "monitor",                           // Command name
                Short:        "Monitor system resources",          // Short description
                Long:         "Display real-time system metrics including CPU, memory, and network.", // Long description
                Subcommands:  []string{"stats", "bandwidth"},      // Available subcommands
                Continuous:   true,                                // Enable live polling
                PollInterval: 2,                                   // Poll every 2 seconds
            },
        },
    }, nil
}
```

### Command Types

**One-off Commands** (`Continuous: false`):
- Execute once, print output, exit
- Examples: `jack led status`, `jack plugin info monitoring`
- Default behavior

**Continuous Commands** (`Continuous: true`):
- Poll continuously with screen refresh
- Examples: `jack monitor stats`, `jack monitor bandwidth wg-proton`
- Daemon calls `ExecuteCLICommand()` repeatedly at `PollInterval` seconds
- User presses Ctrl+C to exit

### Command Implementation

Implement `ExecuteCLICommand()` to handle your commands:

```go
func (p *MyRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
    // Parse command - comes as "monitor stats" or "monitor bandwidth"
    parts := strings.Fields(command)
    if len(parts) < 2 {
        return nil, fmt.Errorf("invalid command format")
    }

    subcommand := parts[1]

    switch subcommand {
    case "stats":
        return p.executeStats()
    case "bandwidth":
        return p.executeBandwidth(args)  // args from command line
    default:
        return nil, fmt.Errorf("unknown subcommand: %s", subcommand)
    }
}

func (p *MyRPCProvider) executeStats() ([]byte, error) {
    // Get current status
    statusData, err := p.provider.Status()
    if err != nil {
        return nil, err
    }

    // Format for display
    var buf bytes.Buffer
    buf.WriteString("System Statistics\n")
    buf.WriteString("=================\n\n")
    buf.WriteString(fmt.Sprintf("CPU: %.1f%%\n", statusData.CPUPercent))
    buf.WriteString(fmt.Sprintf("Memory: %.1f%%\n", statusData.MemoryPercent))

    return buf.Bytes(), nil
}

func (p *MyRPCProvider) executeBandwidth(args []string) ([]byte, error) {
    // args contains additional command-line arguments
    // e.g., "jack monitor bandwidth wg-proton" -> args = ["wg-proton"]

    iface := "default"
    if len(args) > 0 {
        iface = args[0]
    }

    // Format bandwidth display
    var buf bytes.Buffer
    buf.WriteString(fmt.Sprintf("Bandwidth Monitor - %s\n", iface))
    // ... add data ...

    return buf.Bytes(), nil
}
```

### Continuous Command System

The continuous command system is **completely metadata-driven**:

1. Plugin sets `Continuous: true` in CLI command metadata
2. Jack CLI automatically detects continuous commands
3. CLI implements generic polling loop with screen refresh
4. Plugin specifies poll interval (default: 2 seconds)

**No hardcoded logic in core** - any plugin can implement continuous commands.

## Plugin Lifecycle

Understanding the lifecycle helps debug issues:

```
1. Discovery
   ↓
   Daemon scans /usr/lib/jack/plugins/ for jack-plugin-* binaries

2. Loading
   ↓
   Daemon spawns plugin process, establishes RPC connection via stdin/stdout

3. Metadata Query
   ↓
   Daemon calls Metadata() to get namespace, version, config path, etc.

4. Registration
   ↓
   Plugin registered in daemon's registry by namespace

5. Configuration
   ↓
   Daemon loads config from file or uses DefaultConfig

6. Activation
   ↓
   Daemon calls ApplyConfig() to activate plugin

7. Running
   ↓
   Plugin responds to RPC calls (Status, Validate, Flush, CLI commands)

8. Shutdown
   ↓
   Daemon calls Flush() then closes RPC connection
```

### Plugin Discovery Locations

The daemon searches these directories in order:

1. `./bin/` - Local development
2. `/usr/lib/jack/plugins/` - System installation
3. `/opt/jack/plugins/` - Alternative installation

Plugins must be:
- Named `jack-plugin-<name>`
- Executable (`chmod +x`)
- For target architecture (arm64 for routers)

## Building & Deployment

### Building Plugins

Standard build command for cross-compilation:

```bash
cd plugins/core/myplugin

CGO_ENABLED=0 \
GOOS=linux \
GOARCH=arm64 \
go build \
  -ldflags "-s -w" \
  -o jack-plugin-myplugin \
  .
```

**Flags explained:**
- `CGO_ENABLED=0` - Static binary, no C dependencies (use `CGO_ENABLED=1` only if absolutely necessary)
- `GOOS=linux GOARCH=arm64` - Cross-compile for target architecture
- `-ldflags "-s -w"` - Strip debug info, reduce binary size
- `-o jack-plugin-myplugin` - Output binary name

**CGO Warning:** If your plugin requires CGO (like sqlite3), you need a cross-compiler toolchain. Consider alternatives or build on target architecture.

### Build Script Integration

Add your plugin to `build.sh`:

```bash
echo "  Building jack-plugin-myplugin..."
(cd plugins/core/myplugin && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags "-s -w" \
    -o ../../../bin/jack-plugin-myplugin \
    .)
```

### Deployment

Copy built binary to plugin directory:

```bash
# Local development
sudo cp bin/jack-plugin-myplugin /usr/lib/jack/plugins/
sudo chmod +x /usr/lib/jack/plugins/jack-plugin-myplugin

# Remote deployment
scp bin/jack-plugin-myplugin root@gateway:/usr/lib/jack/plugins/
```

Restart daemon to load plugin:

```bash
sudo systemctl restart jack
# or
jack daemon restart
```

Verify plugin is loaded:

```bash
jack plugin list
```

## Testing

### Unit Tests

Test pure logic without system dependencies:

```go
// types_test.go
package main

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name        string
        config      MyConfig
        expectError bool
    }{
        {
            name:        "valid config",
            config:      MyConfig{Enabled: true, Port: 8080},
            expectError: false,
        },
        {
            name:        "invalid port",
            config:      MyConfig{Enabled: true, Port: -1},
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateConfig(&tt.config)
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestRateCalculation(t *testing.T) {
    prev := Metrics{Bytes: 1000, Timestamp: time.Now()}
    curr := Metrics{Bytes: 2000, Timestamp: prev.Timestamp.Add(1 * time.Second)}

    rate := calculateRate(prev, curr)
    assert.Equal(t, uint64(1000), rate)
}
```

Run unit tests:

```bash
go test ./plugins/core/myplugin/...
```

### Integration Tests

Test full plugin lifecycle with system interactions:

```go
// test/integration/myplugin_test.go
//go:build integration

package integration

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestMyPluginIntegration(t *testing.T) {
    harness := NewTestHarness(t)
    defer harness.Cleanup()

    // Create jack.json with plugin enabled
    harness.CreateJackConfig(map[string]interface{}{
        "version": "1.0",
        "plugins": map[string]interface{}{
            "myplugin": map[string]interface{}{
                "enabled": true,
            },
        },
    })

    // Start daemon
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    go harness.StartDaemon(ctx)
    harness.WaitForDaemon(5 * time.Second)

    // Test plugin functionality
    // ...
}
```

Run integration tests:

```bash
sg docker -c "make test-integration"
```

**Important:** Integration tests require privileged operations and **must** run in Docker.

## Common Pitfalls

### 1. Logging to stdout

```go
// ❌ BAD: Breaks RPC protocol
log.SetOutput(os.Stdout)

// ✅ GOOD: Log to stderr
log.SetOutput(os.Stderr)
log.SetPrefix("[jack-plugin-myplugin] ")
```

RPC communication uses stdout - any output to stdout breaks the protocol.

### 2. Wrong Package Name

```go
// ❌ BAD: Won't compile as executable
package myplugin

// ✅ GOOD: All plugins use package main
package main
```

### 3. Not Handling Empty Config

```go
// ❌ BAD: Crashes on empty config
func (p *MyProvider) ApplyConfig(config *MyConfig) error {
    return doSomething(config.Field)  // nil pointer if config is empty
}

// ✅ GOOD: Handle gracefully
func (p *MyProvider) ApplyConfig(config *MyConfig) error {
    if config == nil || !config.Enabled {
        return nil
    }
    return doSomething(config.Field)
}
```

### 4. Blocking Operations Without Context

```go
// ❌ BAD: Can't be canceled
func (p *MyRPCProvider) Status(ctx context.Context) ([]byte, error) {
    data := collectDataForever()  // Hangs indefinitely
    return json.Marshal(data)
}

// ✅ GOOD: Respect context
func (p *MyRPCProvider) Status(ctx context.Context) ([]byte, error) {
    select {
    case data := <-collectData():
        return json.Marshal(data)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 5. Wrong OnLogEvent Error

```go
// ❌ BAD: Daemon thinks plugin supports log events
func (p *MyRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    return nil
}

// ✅ GOOD: Explicit error message
func (p *MyRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    return fmt.Errorf("plugin does not implement log event handling")
}
```

### 6. Not Validating Config JSON

```go
// ❌ BAD: Panics on invalid JSON
func (p *MyRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    var config MyConfig
    json.Unmarshal(configJSON, &config)  // Ignores error
    return p.provider.ApplyConfig(&config)
}

// ✅ GOOD: Check errors
func (p *MyRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    var config MyConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to unmarshal config: %w", err)
    }
    return p.provider.ApplyConfig(&config)
}
```

### 7. Forgetting to Export Structs

```go
// ❌ BAD: JSON marshaling won't work
type myConfig struct {
    enabled bool
    setting string
}

// ✅ GOOD: Export struct and fields
type MyConfig struct {
    Enabled bool   `json:"enabled"`
    Setting string `json:"setting"`
}
```

## Complete Example

A full working plugin with all recommended patterns:

```go
// main.go
package main

import (
    "log"
    "os"
    jplugin "github.com/we-are-mono/jack/plugins"
)

func main() {
    log.SetOutput(os.Stderr)
    log.SetPrefix("[jack-plugin-example] ")
    log.Println("Starting example plugin...")
    jplugin.ServePlugin(NewExampleRPCProvider())
}

// rpc_provider.go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "github.com/we-are-mono/jack/plugins"
)

type ExampleRPCProvider struct {
    provider *ExampleProvider
}

func NewExampleRPCProvider() *ExampleRPCProvider {
    return &ExampleRPCProvider{
        provider: NewExampleProvider(),
    }
}

func (p *ExampleRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:   "example",
        Version:     "1.0.0",
        Description: "Example plugin demonstrating best practices",
        Category:    "example",
        ConfigPath:  "/etc/jack/example.json",
        DefaultConfig: map[string]interface{}{
            "enabled": true,
            "message": "Hello from example plugin",
        },
        CLICommands: []plugins.CLICommand{
            {
                Name:        "example",
                Short:       "Example plugin commands",
                Long:        "Demonstrate CLI command implementation",
                Subcommands: []string{"status", "message"},
                Continuous:  false,
            },
        },
    }, nil
}

func (p *ExampleRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
    var config ExampleConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to unmarshal config: %w", err)
    }
    return p.provider.ApplyConfig(&config)
}

func (p *ExampleRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
    var config ExampleConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }

    if config.Message == "" {
        return fmt.Errorf("message cannot be empty")
    }

    return nil
}

func (p *ExampleRPCProvider) Flush(ctx context.Context) error {
    return p.provider.Flush()
}

func (p *ExampleRPCProvider) Status(ctx context.Context) ([]byte, error) {
    status := p.provider.Status()
    return json.Marshal(status)
}

func (p *ExampleRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
    return fmt.Errorf("plugin does not implement log event handling")
}

func (p *ExampleRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
    parts := strings.Fields(command)
    if len(parts) < 2 {
        return nil, fmt.Errorf("invalid command format")
    }

    subcommand := parts[1]
    switch subcommand {
    case "status":
        return p.executeStatus()
    case "message":
        return p.executeMessage()
    default:
        return nil, fmt.Errorf("unknown subcommand: %s", subcommand)
    }
}

func (p *ExampleRPCProvider) executeStatus() ([]byte, error) {
    status := p.provider.Status()

    var buf bytes.Buffer
    buf.WriteString("Example Plugin Status\n")
    buf.WriteString("====================\n\n")
    buf.WriteString(fmt.Sprintf("Enabled: %v\n", status.Enabled))
    buf.WriteString(fmt.Sprintf("Message: %s\n", status.Message))

    return buf.Bytes(), nil
}

func (p *ExampleRPCProvider) executeMessage() ([]byte, error) {
    status := p.provider.Status()
    return []byte(status.Message + "\n"), nil
}

// provider.go
package main

type ExampleProvider struct {
    config *ExampleConfig
}

func NewExampleProvider() *ExampleProvider {
    return &ExampleProvider{}
}

func (p *ExampleProvider) ApplyConfig(config *ExampleConfig) error {
    if config == nil || !config.Enabled {
        p.config = nil
        return nil
    }

    p.config = config
    return nil
}

func (p *ExampleProvider) Flush() error {
    p.config = nil
    return nil
}

func (p *ExampleProvider) Status() *ExampleStatus {
    if p.config == nil {
        return &ExampleStatus{
            Enabled: false,
            Message: "Not configured",
        }
    }

    return &ExampleStatus{
        Enabled: p.config.Enabled,
        Message: p.config.Message,
    }
}

// types.go
package main

type ExampleConfig struct {
    Enabled bool   `json:"enabled"`
    Message string `json:"message"`
}

type ExampleStatus struct {
    Enabled bool   `json:"enabled"`
    Message string `json:"message"`
}
```

Build and test:

```bash
cd plugins/core/example
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o jack-plugin-example .
sudo cp jack-plugin-example /usr/lib/jack/plugins/
sudo systemctl restart jack

# Test
jack plugin list           # Should show example plugin
jack example status        # Should display status
jack example message       # Should display configured message
```

## Additional Resources

- **Plugin Examples**: See `plugins/core/` for production plugins
  - Simple: `leds/` - Hardware queries, CLI commands
  - Moderate: `monitoring/` - Continuous commands, metrics collection
  - Complex: `nftables/` - Configuration generation, system interaction

- **Core Code**:
  - `plugins/rpc.go` - Provider interface definition
  - `plugins/plugin.go` - Plugin manager and discovery
  - `daemon/registry.go` - Plugin registry

- **Testing**:
  - `test/integration/` - Integration test examples
  - `plugins/core/*/` - Unit test examples

- **Build System**:
  - `build.sh` - Build script with all plugins
  - `Makefile` - Test targets

## Questions?

For questions or issues with plugin development, please:

1. Check existing plugins in `plugins/core/` for examples
2. Review this documentation thoroughly
3. Run integration tests to verify your implementation
4. Open an issue on GitHub with details

Happy plugin development!
