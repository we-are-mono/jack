# Claude Development Guide for Jack

## Project Overview

Jack is a modern, transactional network configuration daemon for Linux systems. It provides a clean abstraction layer over Linux networking, firewalls, and services, making it easy to configure routers, gateways, and network appliances.

### Key Features
- **Transactional Configuration**: Stage changes, review them, then commit atomically
- **Plugin-Based Architecture**: All services implemented as RPC-based plugins (nftables, dnsmasq, wireguard, monitoring)
- **Firewall Management**: Zone-based firewall with NAT and port forwarding
- **DHCP/DNS Server**: Built-in services via dnsmasq plugin
- **VPN Support**: WireGuard client and server configurations
- **System Monitoring**: Real-time status of daemon, interfaces, and services
- **Unix Socket API**: Integration with web UIs or automation tools

### Architecture Components
1. **Jack Daemon** - Background service managing system state and plugins
2. **Jack CLI** - Command-line interface for configuration
3. **Plugins** - RPC-based providers (nftables, dnsmasq, wireguard, monitoring)

```
┌─────────────┐
│  Jack CLI   │
└──────┬──────┘
       │ Unix Socket (/var/run/jack.sock)
┌──────▼──────┐
│ Jack Daemon │
└──────┬──────┘
       │ RPC (stdin/stdout)
   ┌───┴────┬─────────┬──────────┐
   ▼        ▼         ▼          ▼
Network  Firewall   DHCP       VPN
 (ip)   (nftables) (dnsmasq) (wireguard)
```

### Use Cases
- Home routers and network gateways
- Lab environments requiring frequent reconfiguration
- Backend for web UIs (like Netrunner)
- Automation via Ansible, Terraform, or custom tools

**Target Deployment**: Debian-based systems (arm64 architecture)

## Plugin Architecture

**CRITICAL**: All functionality outside of core routing and interface management MUST be implemented as plugins. The daemon should remain lean and focused on:
- Network interface configuration (bridges, VLANs, physical interfaces)
- Static route management
- Plugin lifecycle management
- Configuration loading

### How Plugins Work

Plugins use **HashiCorp go-plugin with RPC communication**:

1. **Plugin Discovery**: Daemon scans `/usr/lib/jack/plugins/` for executables matching `jack-plugin-*`
2. **RPC Protocol**: Each plugin runs as a separate process and communicates with the daemon via RPC over stdin/stdout
3. **Namespace Registration**: Plugins register capabilities (e.g., "firewall", "dhcp", "vpn") with version info
4. **Configuration**: Plugins receive JSON configuration from `/etc/jack/<namespace>.json`
5. **Lifecycle**: Daemon manages plugin start/stop/reload via RPC calls

### Core Plugins

- **nftables** (`plugins/core/nftables/`) - Firewall management with zones, NAT, port forwarding
- **dnsmasq** (`plugins/core/dnsmasq/`) - DHCP and DNS server configuration
- **wireguard** (`plugins/core/wireguard/`) - VPN tunnel management (client and server)
- **monitoring** (`plugins/core/monitoring/`) - System health checks and metrics

### Plugin Implementation Pattern (Modern Direct RPC)

Each plugin must:
1. Implement the `Provider` interface directly (from `plugins/rpc.go`)
2. Provide all RPC methods: `Metadata`, `ApplyConfig`, `ValidateConfig`, `Flush`, `Status`, `ExecuteCLICommand`, `OnLogEvent`
3. Use `context.Context` for all methods to support cancellation
4. Handle JSON marshaling/unmarshaling at the RPC boundary
5. Use `plugins.ServePlugin()` helper to expose RPC interface
6. Define configuration structs in `types.go` with JSON struct tags
7. Implement core functionality in provider-specific files

Example structure:
```
plugins/core/example/
├── main.go          # Plugin entry point - calls ServePlugin(provider)
├── rpc_provider.go  # Implements Provider interface with all 7 RPC methods
├── types.go         # Configuration structs
└── example.go       # Core functionality (business logic)
```

**DEPRECATED**: The legacy `Plugin` interface and `PluginAdapter` wrapper are deprecated. They lack support for CLI commands, log events, and context-based cancellation. All core plugins use the modern Direct RPC pattern.

**When adding new functionality, create a new plugin instead of modifying the daemon core.**

## Plugin Internals

### Plugin Lifecycle

Plugins go through the following lifecycle managed by the daemon:

1. **Discovery** - Daemon scans `/usr/lib/jack/plugins/` for binaries matching `jack-plugin-*`
2. **Loading** - Daemon spawns plugin process and establishes RPC connection via stdin/stdout
3. **Metadata Query** - Daemon calls `Metadata()` to get plugin namespace, version, description, and default config
4. **Registration** - Plugin registered in daemon's plugin registry by namespace
5. **Configuration** - Daemon loads config from `/etc/jack/<plugin-name>.json` or uses plugin's default config
6. **Activation** - Daemon calls `ApplyConfig()` to activate the plugin with its configuration
7. **Running** - Plugin remains active, responding to RPC calls (Status, Validate, Flush)
8. **Shutdown** - Daemon calls `Flush()` then `Close()` to gracefully terminate plugin

### Configuration Loading

The daemon loads plugin configurations in this priority order:

1. **Configuration File** (`/etc/jack/<plugin-name>.json`)
   - If the file exists, daemon loads and unmarshals it
   - Passes the configuration to the plugin via `ApplyConfig(config)`

2. **Default Configuration** (Plugin Metadata)
   - If no config file exists, daemon checks plugin metadata for `DefaultConfig`
   - If `DefaultConfig` is present, uses it instead of requiring a config file
   - This allows plugins to work "out of the box" with sensible defaults

3. **No Configuration**
   - If neither file nor default config exists, plugin is skipped during apply operations
   - Plugin can still be queried for status but won't perform any system changes

**Example**: The monitoring plugin defines `DefaultConfig: {"enabled": true}` in its metadata. When enabled, it works immediately without requiring `/etc/jack/monitoring.json`.

### Configuration Loading Points

The daemon loads plugin configurations at three key points:

1. **Daemon Startup** (`daemon/server.go:149-178`)
   - Loads all enabled plugins from `jack.json`
   - For each plugin, tries to load `<plugin-name>.json`
   - Falls back to `DefaultConfig` if file doesn't exist
   - Stores config in daemon state for later apply operations

2. **Plugin Enable** (`daemon/server.go:813-833`)
   - When user runs `jack plugin enable <name>`
   - Immediately loads and applies config (file or default)
   - Allows plugin to start working right away

3. **Apply Configuration** (`daemon/server.go:497-509`)
   - When user runs `jack apply`
   - Applies all plugin configs to the system
   - Uses config file if available, otherwise uses `DefaultConfig`

### Plugin Metadata Structure

Each plugin returns metadata via the `Metadata()` method:

```go
type PluginMetadata struct {
    Namespace     string                 // Plugin namespace (e.g., "firewall", "vpn")
    Version       string                 // Plugin version (e.g., "1.0.0")
    Description   string                 // Human-readable description
    ConfigPath    string                 // Expected config file path
    DefaultConfig map[string]interface{} // Optional default configuration
    Dependencies  []string               // Other plugins this depends on
}
```

**DefaultConfig Example**:
```go
func (p *MonitoringRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:   "monitoring",
        Version:     "1.0.0",
        Description: "System and network metrics collection",
        ConfigPath:  "/etc/jack/monitoring.json",
        DefaultConfig: map[string]interface{}{
            "enabled": true,
            "collection_interval": 5,
        },
    }, nil
}
```

### RPC Interface

Plugins implement the `Provider` interface with the following RPC methods (via `plugins/rpc.go`):

- **`Metadata(ctx context.Context) (MetadataResponse, error)`** - Returns plugin metadata (namespace, version, defaults, CLI commands)
- **`ApplyConfig(ctx context.Context, configJSON []byte) error`** - Apply configuration to the system
- **`ValidateConfig(ctx context.Context, configJSON []byte) error`** - Validate configuration without applying
- **`Flush(ctx context.Context) error`** - Remove all plugin-managed system state (cleanup)
- **`Status(ctx context.Context) ([]byte, error)`** - Return current plugin status as JSON
- **`ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error)`** - Execute plugin-provided CLI commands
- **`OnLogEvent(ctx context.Context, logEventJSON []byte) error`** - Receive log events from daemon (optional, return error if not implemented)

### Plugin State Management

The daemon maintains plugin state in two places:

1. **Registry** (`daemon/registry.go`)
   - Maps namespace → plugin instance (RPC client)
   - Maps plugin name → namespace (for config file lookups)
   - Tracks which plugins are loaded and active

2. **Configuration State** (`daemon/state.go`)
   - Committed config (loaded from disk at startup)
   - Staged config (pending changes not yet committed)
   - Last applied config (for change detection)

### Creating a New Plugin

To create a new plugin using the modern Direct RPC pattern:

1. Create directory: `plugins/core/<name>/`
2. Implement files:
   - `main.go` - Entry point that creates RPC provider and calls `plugins.ServePlugin(provider)`
   - `rpc_provider.go` - Implements `plugins.Provider` interface with all 7 RPC methods
   - `types.go` - Configuration structs with JSON tags
   - `<name>.go` - Core functionality (business logic) implementation

3. Implement all Provider methods:
   - `Metadata()` - Return plugin metadata with optional `DefaultConfig` and `CLICommands`
   - `ApplyConfig()` - Unmarshal JSON config and apply to system
   - `ValidateConfig()` - Unmarshal JSON config and validate without applying
   - `Flush()` - Remove all plugin-managed state
   - `Status()` - Return status as marshaled JSON
   - `ExecuteCLICommand()` - Handle plugin CLI commands (or return error if not supported)
   - `OnLogEvent()` - Handle log events (or return error if not supported)

4. Build with correct architecture: `GOOS=linux GOARCH=arm64 go build -o jack-plugin-<name>`
5. Deploy to `/usr/lib/jack/plugins/`
6. Enable via `jack plugin enable <name>`

**Reference Implementation**: See `plugins/core/monitoring/rpc_provider.go` for a complete example of the modern pattern.

### Plugin CLI Commands System

Plugins can provide custom CLI commands that are automatically registered with the Jack CLI. This allows plugins to expose monitoring, diagnostics, or management functionality directly to users.

#### Command Types

**One-Off Commands** (default)
- Execute once and return output
- Example: `jack status`, `jack plugin info monitoring`
- Standard request-response pattern

**Continuous Commands** (metadata-driven)
- Poll continuously with screen refresh
- Used for live monitoring, real-time stats, streaming output
- Controlled entirely by plugin metadata - no hardcoded logic in core
- Example: `jack monitor stats`, `jack monitor bandwidth wg-proton`

#### Implementing CLI Commands in Plugins

Plugins declare CLI commands in their metadata response:

```go
func (p *MonitoringRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
    return plugins.MetadataResponse{
        Namespace:   "monitoring",
        Version:     "1.0.0",
        Description: "System monitoring plugin",
        ConfigPath:  "/etc/jack/monitoring.json",
        CLICommands: []plugins.CLICommand{
            {
                Name:         "monitor",
                Short:        "Monitor system resources and network bandwidth",
                Long:         "Display real-time system metrics including CPU, memory, load, and network bandwidth.",
                Subcommands:  []string{"stats", "bandwidth"},
                Continuous:   true,         // Mark as continuous command
                PollInterval: 2,            // Poll every 2 seconds (default: 2)
            },
        },
    }, nil
}
```

The plugin must also implement the `ExecuteCLICommand()` RPC method:

```go
func (p *MonitoringRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
    // Parse command string (e.g., "monitor stats" or "monitor bandwidth")
    parts := strings.Fields(command)

    switch parts[1] {
    case "stats":
        return p.executeStats()
    case "bandwidth":
        return p.executeBandwidth(args)
    default:
        return nil, fmt.Errorf("unknown subcommand: %s", parts[1])
    }
}
```

#### Continuous Commands Architecture

The continuous command system is **completely metadata-driven**:

1. **Plugin Declaration**: Plugin sets `Continuous: true` in CLI command metadata
2. **Core Detection**: CLI automatically detects continuous commands via metadata
3. **Polling Loop**: CLI implements generic polling loop with screen refresh
4. **Custom Interval**: Plugin can specify poll interval (default: 2 seconds)

**Implementation Files**:
- [plugins/rpc.go:52-60](plugins/rpc.go#L52-L60) - `CLICommand` struct with `Continuous` and `PollInterval` fields
- [cmd/plugin_commands.go:137-146](cmd/plugin_commands.go#L137-L146) - Metadata-driven detection logic
- [cmd/plugin_commands.go:167-192](cmd/plugin_commands.go#L167-L192) - Generic `executeContinuousCommand()` function

**Benefits**:
- Zero hardcoded command checks in core
- Any plugin can implement continuous commands
- Flexible per-command poll intervals
- Clean separation: core handles polling, plugin handles output formatting

#### Adding CLI Commands to a Plugin

To add CLI commands to your plugin:

1. **Extend Metadata**: Add `CLICommands` array to your `MetadataResponse`
   ```go
   CLICommands: []plugins.CLICommand{
       {
           Name:         "mycmd",
           Short:        "Short description",
           Long:         "Long description",
           Subcommands:  []string{"sub1", "sub2"}, // Optional
           Continuous:   false,  // true for live polling commands
           PollInterval: 2,      // Only used if Continuous is true
       },
   }
   ```

2. **Implement ExecuteCLICommand**: Handle command execution in your RPC provider
   ```go
   func (p *MyRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
       // Parse command and return formatted output
       return []byte("command output"), nil
   }
   ```

3. **Format Output**: Return plain text or formatted output
   - One-off commands: Output printed once, then CLI exits
   - Continuous commands: Output refreshed every poll interval with screen clear

4. **Test**: Commands appear automatically in `jack --help` after plugin loads

**Example Plugins with CLI Commands**:
- **monitoring** - `jack monitor stats` (continuous), `jack monitor bandwidth <iface>` (continuous)

### Plugin Communication Flow

```
User CLI Command
    ↓
Unix Socket → Daemon
    ↓
Daemon loads/validates config
    ↓
RPC Call → Plugin Process
    ↓
Plugin applies config to system
    ↓
Response ← Plugin
    ↓
Result ← Daemon
    ↓
User sees output
```

### Default Config Design Pattern

When designing a plugin with default config:

1. **Keep defaults minimal** - Only include essential settings
2. **Make them safe** - Defaults should not disrupt the system
3. **Document behavior** - Clearly indicate what the defaults do
4. **Allow overrides** - Users can always provide their own config file

**Good Example**: Monitoring plugin defaults to `{"enabled": true}` - passive observation, no system changes

**Bad Example**: Firewall plugin with default DROP policy - could lock users out

## Development Guidelines

### Backwards Compatibility

**IGNORE BACKWARDS COMPATIBILITY.** This is an internal tool under active development. Breaking changes are acceptable and preferred over maintaining legacy code. Always prioritize:
- Clean, maintainable code
- Modern Go idioms
- Simplicity over compatibility

### Task Estimation

**NEVER provide time estimates for completing tasks.** Do not say things like:
- "This will take about 5 minutes"
- "Should be quick"
- "This is a 2-hour task"
- "Estimated completion time: 30 minutes"

**Rationale:**
- Time estimates are frequently inaccurate and set false expectations
- Software development has unpredictable complexity
- Focus should be on quality and correctness, not speed
- User can observe progress through actual work completed

**What to do instead:**
- Start working on the task immediately
- Provide progress updates as you complete steps
- Use the TodoWrite tool to track multi-step tasks
- Let completed work speak for itself

### Issue Resolution Priority

**CRITICAL: Any discovered issues must be resolved immediately, regardless of whether they are related to the current activity.**

When you discover bugs, inconsistencies, or problems during development:

1. **Stop and fix immediately** - Don't defer or ignore issues
2. **Fix the root cause** - Address the underlying problem, not just symptoms
3. **Test the fix** - Verify the issue is actually resolved
4. **Document if needed** - Update comments or documentation if the issue revealed confusion

**Examples of issues to fix immediately:**
- Misleading status messages (e.g., "[DOWN] Daemon: Not running" when daemon is actually running)
- Database locks preventing operations
- Incorrect error handling
- Race conditions or timing issues
- Configuration inconsistencies
- Security vulnerabilities

**Rationale:**
- Technical debt compounds quickly if ignored
- Fresh context makes fixes faster and more accurate
- Issues discovered are often symptoms of larger problems
- Deferring fixes leads to forgotten bugs
- User experience suffers from accumulated minor issues

**Exception:** Only defer a fix if it requires major architectural changes that would derail critical work. In this case, document the issue clearly and notify the user.

### Concurrency and Timing

**CRITICAL: Never use time.Sleep() or any kind of waiting/polling mechanisms in production code.**

Go provides proper concurrency primitives for synchronization and coordination. Always use these instead of sleep-based timing:

**What NOT to do:**
- `time.Sleep()` to wait for operations to complete
- Polling loops checking for state changes
- Fixed delays to "give things time to happen"
- Arbitrary timeouts to work around race conditions

**What to do instead:**
- **Channels** - Use channels to signal completion, readiness, or state changes
- **sync.WaitGroup** - Wait for multiple goroutines to complete
- **context.Context** - Handle cancellation and timeouts properly
- **Blocking calls** - Use blocking operations that wait for actual completion (e.g., `Dial()`, `Accept()`)
- **Defer** - Use defer for cleanup that must happen after a function completes
- **Callbacks/Observers** - Use event-driven patterns for notifications
- **Mutexes** - Protect shared state with proper locking

**Example patterns:**

```go
// ❌ BAD: Sleep-based waiting
go startServer()
time.Sleep(100 * time.Millisecond)
connectToServer()

// ✅ GOOD: Channel-based signaling
ready := make(chan struct{})
go func() {
    startServer()
    close(ready)
}()
<-ready
connectToServer()

// ❌ BAD: Polling for state
for !isReady() {
    time.Sleep(10 * time.Millisecond)
}

// ✅ GOOD: Blocking until ready
<-readyChannel

// ❌ BAD: Sleep between goroutine start and connection
go Accept(conn)
time.Sleep(50 * time.Millisecond)
Dial(conn)

// ✅ GOOD: Blocking Dial waits for Accept
go Accept(conn)
Dial(conn)  // Blocks until Accept is ready
```

**Rationale:**
- Sleep-based timing is inherently unreliable and non-deterministic
- Creates race conditions that may pass tests but fail in production
- Makes code slower (always waits full duration even if operation completes quickly)
- Hides the real synchronization requirements
- Proper Go concurrency patterns are faster, safer, and more maintainable

#### Service Waiting for Plugin Coordination

When plugins need to wait for other services (e.g., database service) to become ready, use the `DaemonService` interface methods instead of time.Sleep:

```go
// ✅ GOOD: Wait for service using channel-based infrastructure
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := p.daemonService.WaitForService(ctx, "database"); err != nil {
    log.Printf("Warning: database service not ready: %v\n", err)
    return
}
// Service is ready, proceed with operations
```

**Available methods:**
- `WaitForService(ctx, serviceName)` - Blocks until service ready or timeout
- `WaitForServices(ctx, serviceNames)` - Waits for multiple services
- `IsServiceReady(serviceName)` - Non-blocking ready check

**Architecture:**
- ServiceRegistry maintains `readyChannels` (closed when service ready)
- `MarkServiceReady()` called by daemon after successful `ApplyConfig`
- Channels provide immediate wake-up when service becomes available
- Context-aware for proper timeout and cancellation handling

**Example use case:** Firewall plugin waiting for database service before initializing logging schema (see [plugins/core/firewall/rpc_provider.go:357-377](plugins/core/firewall/rpc_provider.go#L357-L377))

### Git Workflow

**IMPORTANT: Claude should NEVER create git commits.** All git commits must be done manually by the user.

**Rationale:**
- User maintains full control over commit history
- User can review all changes before committing
- User can write appropriate commit messages
- Prevents automatic commits that may bundle unrelated changes

**What Claude CAN do:**
- Run read-only git commands (`git status`, `git log`, `git diff`)
- Suggest commit messages for user to use manually
- Review changes before user commits
- Build and test code without committing

**What Claude CANNOT do:**
- Run `git add`, `git commit`, `git push`
- Modify `.git/` directory
- Create branches or tags
- Any command that modifies git repository state

When code changes are complete and ready for version control, inform the user and let them handle commits manually.

### Licensing

**All Go source files must include the GPL 2.0 header:**

```go
// Copyright (C) 2025 Mono Technologies Inc.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
```

Place this header at the top of every `.go` file before the package documentation. When creating new Go files, always include this header first.

### Code Quality Checks

**Run these checks periodically** (at minimum before major commits):

```bash
# Linter (golangci-lint)
make lint
# or directly:
~/go/bin/golangci-lint run

# Deadcode detection
make deadcode
# or directly:
~/go/bin/deadcode ./...

# Unused code detection
make lint-unused
```

Fix all issues reported by these tools. Remove dead code immediately rather than commenting it out.

### Build Process

Cross-compilation for arm64 target:
```bash
# Main daemon
GOOS=linux GOARCH=arm64 go build -o jack

# Plugins (must match target architecture)
GOOS=linux GOARCH=arm64 go build -o jack-plugin-firewall plugins/core/nftables/*.go
GOOS=linux GOARCH=arm64 go build -o jack-plugin-dnsmasq plugins/core/dnsmasq/*.go
GOOS=linux GOARCH=arm64 go build -o jack-plugin-wireguard plugins/core/wireguard/*.go
```

Or use the build script:
```bash
./build.sh
```

### Testing

```bash
# Run unit tests
go test ./...

# Run integration tests (ALWAYS use docker with sg command)
sg docker -c "make test-integration"

# With coverage
make coverage
```

**IMPORTANT**: Integration tests require privileged operations (netlink, sysctl, root access) and MUST be run in Docker. Always use `sg docker -c "make test-integration"` to run integration tests.

**CRITICAL Docker Configuration**:
- **NEVER use `--network host` when running integration tests in Docker**
- The Makefile uses the correct configuration: `--privileged --cap-add=ALL` with default Docker networking
- Jack uses Unix domain sockets (filesystem-based), not network sockets, so `--network host` is unnecessary and breaks test isolation
- Using `--network host` bypasses Docker's network isolation and causes test failures
- Always follow the Makefile's Docker configuration exactly

**CRITICAL**: Always run integration tests before reporting success on any code changes or test additions. Integration tests verify the entire system works correctly and must pass before considering work complete.

```bash
# Before reporting success, ALWAYS run:
sg docker -c "make coverage-integration"
```

When testing new functionality, prefer using CLI commands when possible:
```bash
# Use Jack CLI for direct testing (if applicable)
jack set <component> <config>

# Example: Testing network configuration
jack set interfaces '{"br-lan": {"type": "bridge", "members": ["lan1", "lan2"]}}'

# Example: Testing firewall changes
jack set firewall '{"zones": {"wan": {"interfaces": ["eth0"]}}}'
```

Using `jack set` provides immediate feedback and validates the entire configuration pipeline (parsing, validation, application) in one step. Only use direct system commands (e.g., `ip`, `nft`, `echo > /sys/`) when testing low-level functionality or debugging specific issues.

### Testing Best Practices

**CRITICAL: Always prefer testify/assert for unit tests.**

All unit tests should use the [testify/assert](https://github.com/stretchr/testify) library instead of manual error checking. This provides:
- Cleaner, more readable test code
- Consistent assertion patterns across the codebase
- Better error messages when tests fail
- Less boilerplate code

**Import structure:**
```go
import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**When to use `require` vs `assert`:**
- **`require`** (fatal) - Test should stop immediately if this fails (e.g., setup operations, critical preconditions)
- **`assert`** (non-fatal) - Test can continue even if this fails (e.g., multiple field validations)

**Standard assertion patterns:**

```go
// Error checking - fatal if error occurs
require.NoError(t, err)
require.NoError(t, err, "Optional custom message")

// Expected errors - fatal if no error
require.Error(t, err, "Function() expected error")
if errContain != "" {
    assert.Contains(t, err.Error(), errContain)
}

// Value comparisons - non-fatal
assert.Equal(t, expected, actual)
assert.Equal(t, expected, actual, "Optional custom message")

// Nil checks
require.NotNil(t, value, "Expected non-nil value")
assert.Nil(t, value, "Expected nil value")

// Boolean checks
assert.True(t, condition)
assert.False(t, condition)

// String/slice containment
assert.Contains(t, haystack, needle)
assert.NotContains(t, haystack, needle)

// Length checks
require.Len(t, slice, expectedLength)
assert.Empty(t, value)
assert.NotEmpty(t, value)
```

**Common table-driven test pattern:**

```go
func TestValidateConfig(t *testing.T) {
    tests := []struct {
        name       string
        config     *Config
        wantError  bool
        errContain string
    }{
        {
            name:      "valid config",
            config:    &Config{Field: "value"},
            wantError: false,
        },
        {
            name:       "invalid config",
            config:     &Config{Field: ""},
            wantError:  true,
            errContain: "field cannot be empty",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateConfig(tt.config)

            if tt.wantError {
                require.Error(t, err, "ValidateConfig() expected error")
                if tt.errContain != "" {
                    assert.Contains(t, err.Error(), tt.errContain)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

**Migration from manual assertions:**

When converting existing tests to testify:

```go
// ❌ OLD STYLE: Manual error checking
if err != nil {
    t.Fatalf("Function() unexpected error: %v", err)
}
if got != want {
    t.Errorf("Function() = %v, want %v", got, want)
}

// ✅ NEW STYLE: testify/assert
require.NoError(t, err)
assert.Equal(t, want, got)
```

**Important migration lessons:**

1. **Avoid over-reliance on automated replacements** - Large files (500+ lines) with complex assertion patterns require careful manual migration. Automated regex replacements can create syntax errors (e.g., broken else-clauses).

2. **Always verify with gofmt and test execution** - After migration:
   ```bash
   gofmt -w path/to/test_file.go
   go test ./path/to/package -v
   ```

3. **Clean up unused imports** - After migrating `strings.Contains()` to `assert.Contains()`, remove unused `strings` imports.

4. **Use consistent patterns** - Follow the table-driven test pattern above for error checking across all tests.

5. **Batch similar files** - Group migrations by package (e.g., all `cmd/` tests, all `system/` tests) for efficiency.

**When NOT to use testify:**

- Integration tests can use testify, but focus on system-level validation
- Benchmark tests (`testing.B`) may skip testify for performance-critical assertions
- Tests that specifically validate error message formatting may need direct string comparison

### Test Quality Standards

**CRITICAL RULE: ALL TESTS MUST PASS.**

Having failing tests defeats the entire purpose of testing. Tests exist to verify that code works correctly. If tests are failing:
1. **Fix the code** - The test is revealing a bug that must be fixed
2. **Fix the test** - The test itself may be incorrect or outdated
3. **Delete the test** - If it's testing something no longer relevant

**NEVER commit or leave failing tests in the codebase.** A failing test is worse than no test because:
- It creates noise that hides real failures
- It trains developers to ignore test failures
- It provides false confidence ("we have tests")
- It makes the test suite unreliable

**Before completing ANY work:**
```bash
# All tests must pass
sg docker -c "make test-integration"
```

If integration tests fail, the work is NOT complete. Period.

**CRITICAL RULE: NEVER ignore failing tests, even if unrelated to current work.**

If tests are failing during development:
1. **Fix the test immediately** - Even if it's "unrelated" to your current task
2. **Never report success with failing tests** - Work is not complete until ALL tests pass
3. **Investigate the root cause** - Failing tests indicate real problems that compound over time
4. **Do not defer fixes** - "I'll fix it later" leads to accumulated technical debt

**Examples:**
- ❌ BAD: "The plugin works, but there's an unrelated daemon test failing - I'll report success anyway"
- ✅ GOOD: "The plugin works, but I found a daemon test failing. Let me fix that first before reporting completion"

The only exception is if the fix requires major architectural changes that would derail critical work. In this case, document the issue clearly and notify the user immediately.

**CRITICAL RULE: Never use `t.Skip()` in tests.**

Tests should either run properly or not exist at all. A test that always skips provides a false sense of coverage and misleads developers about the true state of testing.

**If you encounter dependencies that prevent testing:**
1. **Mock the dependency** - Create interfaces and inject mocks (preferred approach)
2. **Move to integration tests** - If the test truly requires system resources
3. **Delete the test** - If it cannot be made to run and provides no value

**Examples of proper handling:**

```go
// ❌ BAD: Test that always skips
func TestApplyInterfaceConfig(t *testing.T) {
    t.Skip("Requires netlink mock or integration test environment")
    // Test code that never runs
}

// ✅ GOOD: Extract testable logic with interface
type NetworkManager interface {
    LinkByName(name string) (netlink.Link, error)
}

func TestApplyInterfaceConfig(t *testing.T) {
    mockNet := &MockNetworkManager{
        links: map[string]netlink.Link{
            "eth0": &mockLink{name: "eth0"},
        },
    }

    err := ApplyInterfaceConfig(mockNet, "eth0", config)
    assert.NoError(t, err)
}
```

**Test coverage goals:**
- Pure business logic: 100% coverage via unit tests
- I/O-heavy code: Tested via integration tests or mocked interfaces
- Every test must actually execute - no skipped tests allowed

**CRITICAL RULE: Never split success and failure cases between unit and integration tests.**

A test that only validates failure cases with success cases "covered elsewhere" is incomplete and misleading. Each test should be self-contained and test both success and failure paths, or not exist at all.

**Anti-pattern to avoid:**

```go
// ❌ BAD: Incomplete test that only checks failures
func TestApplyVLANInterface_Validation(t *testing.T) {
    tests := []struct {
        name        string
        expectError bool
        errorMsg    string
    }{
        {"missing parent", true, "parent device not specified"},
        {"valid config", false, ""}, // This case never actually runs
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if !tt.expectError {
                return // Success cases covered by integration tests
            }
            err := applyVLANInterface(tt.iface)
            require.Error(t, err)
            assert.Contains(t, err.Error(), tt.errorMsg)
        })
    }
}
```

**Why this is wrong:**
- Creates false sense of coverage (appears to test success, but doesn't)
- Splits testing responsibility across files (unit for failures, integration for success)
- Makes refactoring harder (need to check multiple test files)
- Violates single responsibility (test should be complete)

**Correct approaches:**

1. **Delete the test** if it only validates trivial input parsing and integration tests cover the complete behavior:
```go
// Just delete the incomplete unit test entirely
// Integration tests in test/integration/vlan_test.go already cover this
```

2. **Mock dependencies** to test both success AND failure in the same unit test:
```go
// ✅ GOOD: Complete unit test with mocked dependencies
func TestApplyVLANInterface(t *testing.T) {
    tests := []struct {
        name        string
        mockNet     *MockNetlink
        expectError bool
        errorMsg    string
    }{
        {
            name: "missing parent device",
            mockNet: &MockNetlink{},
            expectError: true,
            errorMsg: "parent device not specified",
        },
        {
            name: "successful VLAN creation",
            mockNet: &MockNetlink{
                links: map[string]netlink.Link{
                    "eth0": &mockLink{name: "eth0"},
                },
            },
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := applyVLANInterface(tt.mockNet, tt.iface)
            if tt.expectError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

3. **Integration test only** for I/O-heavy functions where mocking provides minimal value:
```go
// No unit test - just comprehensive integration tests
// See test/integration/vlan_test.go for complete coverage
```

**Decision criteria:**
- **Delete unit test**: Function is I/O-heavy, integration tests are comprehensive, only trivial validation logic
- **Mock dependencies**: Function has business logic worth testing, abstraction layer exists or is worth creating
- **Integration only**: Function is thin wrapper around system calls, mocking effort > value gained

### Integration Test Best Practices

**CRITICAL: Environment Variable Race Conditions**

When testing daemon components that use environment variables (like socket paths), always use the test harness's stored values instead of calling getter functions:

```go
// ❌ BAD: Environment variable may have changed
socketPath := daemon.GetSocketPath()
assert.NoFileExists(t, socketPath)

// ✅ GOOD: Use the harness's stored socket path
socketPath := harness.socketPath
assert.NoFileExists(t, socketPath)
```

**Why**: The `GetSocketPath()` function reads `JACK_SOCKET_PATH` at call time, which may have been modified by cleanup routines or other tests. The test harness stores the actual path configured for each test instance.

**CRITICAL: Async Goroutine Cleanup**

When testing graceful shutdown or cleanup operations that run in separate goroutines, allow time for async operations to complete:

```go
// ❌ BAD: Checking immediately after shutdown signal
cancel()
select {
case <-serverDone:
    t.Log("Server shut down")
}
assert.NoFileExists(t, socketPath) // May fail due to race condition

// ✅ GOOD: Allow time for Stop() to complete
cancel()
select {
case <-serverDone:
    t.Log("Server shut down")
}
time.Sleep(100 * time.Millisecond) // Let Stop() finish cleanup
assert.NoFileExists(t, socketPath)
```

**Why**: When context is canceled, the `Stop()` method runs in a monitoring goroutine (e.g., `harness.go:151-154`). Even though `Start()` returns, `Stop()` may still be executing cleanup operations like socket removal.

**CRITICAL: Test Harness Usage**

Integration tests should use the `TestHarness` pattern for proper test isolation:

```go
func TestSomeFeature(t *testing.T) {
    harness := NewTestHarness(t)
    defer harness.Cleanup()

    // Use harness methods for setup
    harness.CreateDummyInterface("eth0")

    // Start daemon through harness
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    go harness.StartDaemon(ctx)
    harness.WaitForDaemon(5 * time.Second)

    // Use harness.socketPath for daemon communication
    // Use harness.configDir for config files
}
```

**Benefits**:
- Isolated config directories per test
- Automatic cleanup of interfaces and sockets
- Consistent environment variable management
- Proper daemon lifecycle handling

## Common Pitfalls

### Plugin Architecture Issues

1. **Wrong architecture**: Always build plugins with `GOOS=linux GOARCH=arm64` for the target device
2. **Missing routes**: WireGuard allowed-ips don't automatically create routes - plugins must add them via netlink
3. **Command splitting**: When using `exec.Command()` with special characters (braces, quotes), pass arguments separately, not via `strings.Split()`

### Firewall Configuration

- Base chains (INPUT, OUTPUT, FORWARD) need netfilter hooks to actually process packets
- Zone chains dispatch from base chains and handle zone-specific policies
- Masquerading on VPN client interfaces breaks source IP visibility - only masquerade on outbound zones

## Tips

### Go Map Iteration is Non-Deterministic

**Issue**: Go's map iteration order is intentionally randomized. This can cause flaky tests and race conditions when map iteration order matters.

**Example Problem**: When applying multiple network interfaces from a `map[string]Interface`, the order of processing is non-deterministic. If interface A fails during apply, the system state depends on which interfaces were already processed before the failure.

**Solution**: Always use explicit ordering when processing maps where order matters:

```go
// ❌ BAD: Non-deterministic order
for name, iface := range interfaces {
    applyInterface(name, iface)
}

// ✅ GOOD: Explicit, deterministic ordering
orderedNames := orderInterfaces(interfaces)  // Sort by type: physical, bridge, vlan
for _, name := range orderedNames {
    iface := interfaces[name]
    applyInterface(name, iface)
}
```

**Testing Implications**: Tests that depend on map iteration order may pass sometimes and fail other times. Always assume maps are unordered and use explicit ordering when needed.

**Related Code**: See `daemon/server.go:1214` (`orderInterfaces` function) and the rollback logic fix in `daemon/server.go:707-708`.

## Important Files

- `/etc/jack/*.json` - Plugin configuration files
- `/var/run/jack.sock` - Unix socket for daemon communication
- `/var/run/jack.pid` - Daemon PID file
- `/usr/lib/jack/plugins/` - Plugin binaries location
- `/var/log/jack/jack.log` - Daemon log file

## DO NOT EDIT THIS FILE

**Unless explicitly instructed by the user, DO NOT modify CLAUDE.md.** This file serves as the persistent development guide and should only be updated when project architecture or development practices fundamentally change.
