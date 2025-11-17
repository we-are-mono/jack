# Jack TODO List

## Current Status

### ✅ Implemented
- Network interface management (physical, bridges, VLANs)
- DHCP client (for WAN)
- Static routes (full implementation with metrics, tables, interface/gateway routing)
- Firewall with nftables (zones, rules, NAT, port forwarding/DNAT)
- DHCP server with dnsmasq
- DNS server (via dnsmasq)
- Transactional configuration (set/commit/revert/diff)
- Basic CLI commands
- Daemon with Unix socket API
- Provider architecture
- Configuration templates
- WireGuard VPN (server and client support, peer management)
- Port forwarding (DNAT with automatic forward rules)
- Status/monitoring commands (system, interfaces, services, DHCP leases)
- Logs command (journalctl wrapper with non-systemd fallback)
- WireGuard key generation utility (jack wgkey)
- Multi-config registry pattern (interfaces, firewall, dhcp, routes)
- Config type routing in path parsing
- Multi-config diff and commit (all config types)
- Full get/set support with nested field access (all config types)
  - Interfaces: all fields
  - Firewall: defaults + nested zones
  - DHCP: dnsmasq settings + named pools
  - Routes: by route name

### ⚠️ Partially Implemented
- Configuration validation (basic only)
- Path parsing (name-based access works, numeric array indexing not yet supported)

### ❌ Not Implemented
- Array indexing support (e.g., firewall.rules[0], dhcp.static_leases[1])
- CRUD operations (add/delete commands for array/map items)
- Configuration backup/restore commands
- Safe remote access features (rollback timer, SSH protection)
- Show command with pretty printing
- Shell completion

---

## Priority 1: Core Functionality Gaps

### 1.1 Multi-Config Support in get/set/diff

**Status**: ✅ **COMPLETED** - Full get/set support for all config types with nested field access

**What works now**:
```bash
# Interfaces - Full support
jack get interfaces wan enabled
jack set interfaces wan enabled false
jack set interfaces wan protocol static
jack set interfaces wan ipaddr 10.0.0.100
jack set interfaces wan mtu 1500

# Firewall - Full support (defaults + nested zones)
jack get firewall defaults input
jack set firewall defaults input drop
jack set firewall defaults syn_flood true

jack get firewall zones wan masquerade
jack set firewall zones wan masquerade true
jack set firewall zones wan input drop
jack set firewall zones lan comment "LAN zone"

# DHCP - Full support (dnsmasq + pools)
jack get dhcp dnsmasq cache_size
jack set dhcp dnsmasq enabled true
jack set dhcp dnsmasq cache_size 1000
jack set dhcp dnsmasq log_queries false

jack get dhcp dhcp_pools lan start
jack set dhcp dhcp_pools lan start 100
jack set dhcp dhcp_pools lan limit 150
jack set dhcp dhcp_pools lan leasetime "12h"

# Routes - Full support (by route name)
jack get routes routes default enabled
jack set routes routes default enabled false
jack set routes routes default metric 100
jack set routes routes vpn gateway 10.0.0.1

# Multi-config diff and commit
jack set interfaces wan protocol static
jack set firewall zones wan masquerade true
jack set dhcp dnsmasq cache_size 1000
jack set routes routes default metric 50
jack diff  # Shows changes from ALL config types
jack commit  # Saves ALL modified configs
```

**What was implemented**:
- ✅ Registry pattern in `daemon/state.go` (map[string]*ConfigEntry)
- ✅ Config type routing in `daemon/path.go` (ParseConfigType)
- ✅ Multi-config diff and commit (all 4 config types)
- ✅ Deep copy functions for all config types
- ✅ Full get/set for interfaces (all fields)
- ✅ Full get/set for firewall (defaults + nested zones with all fields)
- ✅ Full get/set for DHCP (dnsmasq global settings + named pools)
- ✅ Full get/set for routes (by route name)
- ✅ Type-safe field validation (string, bool, int)
- ✅ Documentation in [Configuration Reference](../configuration.md)

**Note**: Array indexing by numeric index (e.g., `routes.routes[0]`) is not yet implemented. Current implementation uses name-based access (e.g., `routes.routes.default`), which works for all current use cases

---

### 1.2 CRUD Operations (Add/Delete Commands)

**Status**: Not implemented - **High priority** for production use

**Problem**: Can't add or delete items from arrays/maps via CLI.

**Needed**:
```bash
# Add firewall rule
jack add firewall.rules '{"name":"Allow-HTTP","proto":"tcp","dest_port":"80","target":"ACCEPT"}'

# Delete firewall rule by name
jack delete firewall.rules.Allow-HTTP

# Add DHCP static lease
jack add dhcp.static_leases '{"name":"printer","mac":"aa:bb:cc:dd:ee:ff","ip":"192.168.1.50"}'

# Delete static lease
jack delete dhcp.static_leases.printer

# Add DNS A record
jack add dhcp.dns_records.a_records '{"name":"server.lan","ip":"192.168.1.10"}'

# Add interface
jack add interfaces.guest '{"type":"physical","device":"eth2","protocol":"static","ipaddr":"192.168.2.1","netmask":"255.255.255.0"}'

# Delete interface
jack delete interfaces.guest
```

**Implementation approach**:
- Create `cmd/add.go` with add command handler
- Create `cmd/delete.go` with delete command handler
- Add `handleAdd` and `handleDelete` to `daemon/server.go`
- Support JSON input for complex objects
- Validate added items before accepting

---

### 1.3 Configuration Validation

**Status**: Not implemented - **High priority** for safety

**Problem**: Minimal validation before applying changes - easy to break network config.

**Needed**:
- IP address validation (CIDR, ranges, valid format)
- Port number validation (1-65535)
- Interface name validation (exists, not in use)
- VLAN ID validation (1-4094)
- Gateway reachability checks
- Firewall rule validation (valid protocols, port ranges)
- DHCP pool overlap detection
- Route validation (valid destinations, no conflicts)

**Implementation**:
```go
// In types/ or validators/
func ValidateInterfaceConfig(iface *Interface) error {
    if iface.Protocol == "static" {
        if !isValidIP(iface.IPAddr) {
            return fmt.Errorf("invalid IP address: %s", iface.IPAddr)
        }
        if !isValidNetmask(iface.Netmask) {
            return fmt.Errorf("invalid netmask: %s", iface.Netmask)
        }
    }
    if iface.MTU < 68 || iface.MTU > 9000 {
        return fmt.Errorf("MTU must be between 68 and 9000")
    }
    return nil
}

// Call before commit
func (s *State) ValidatePending() error {
    // Validate all pending configs
}
```

---

### 1.4 Array Indexing Support (Optional)

**Status**: Not implemented - **Low priority** (name-based access works for all current use cases)

**Current behavior**:
```bash
# Name-based access (works now)
jack get routes routes default metric       # ✅ By route name
jack set dhcp dhcp_pools lan start 100      # ✅ By pool name
jack get firewall zones wan input           # ✅ By zone name
```

**Optional enhancement** (numeric indexing):
```bash
# Would allow accessing by index
jack get firewall rules[0] dest_port        # Not yet supported
jack set routes routes[1] enabled false     # Not yet supported
jack get dhcp static_leases[2] ip           # Not yet supported
```

**Decision**: Name-based access is more user-friendly and sufficient for current use cases. Numeric indexing adds complexity with minimal benefit

---

### 1.3 CRUD Operations

**Problem**: Can't add or delete items from arrays/maps.

**Needed**:
```bash
# Add firewall rule
jack add firewall.rules --name "Allow-HTTP" --proto tcp --dest-port 80

# Delete firewall rule
jack delete firewall.rules.0
jack delete firewall.rules --name "Allow-HTTP"

# Add DHCP static lease
jack add dhcp.static_leases --mac "aa:bb:cc:dd:ee:ff" --ip "192.168.1.50" --name "printer"

# Add bridge port
jack add interfaces.lan.bridge_ports eth4

# Remove bridge port
jack delete interfaces.lan.bridge_ports eth2
# or by index
jack delete interfaces.lan.bridge_ports.1
```

**Implementation**:

1. Create `cmd/add.go`:
```go
var addCmd = &cobra.Command{
    Use:   "add <path> [flags]",
    Short: "Add an item to an array or map",
    Long:  `Adds a new item to an array or creates a new map entry.`,
    Args:  cobra.MinimumNArgs(1),
    Run:   runAdd,
}

func runAdd(cmd *cobra.Command, args []string) {
    path := args[0]
    
    // For arrays: just add to end
    // For maps: need a key (derived from flags or provided)
    // For simple arrays (like bridge_ports): value is the second arg
    
    // Examples:
    // jack add interfaces.lan.bridge_ports eth4
    //   -> path="interfaces.lan.bridge_ports", value="eth4"
    
    // jack add firewall.rules --name "Allow-HTTP" --proto tcp --port 80
    //   -> path="firewall.rules", value={name: "Allow-HTTP", proto: "tcp", ...}
    
    // Parse flags into structured data
    // Send to daemon
}
```

2. Create `cmd/delete.go`:
```go
var deleteCmd = &cobra.Command{
    Use:   "delete <path>",
    Short: "Delete an item from an array or map",
    Args:  cobra.ExactArgs(1),
    Run:   runDelete,
}

func runDelete(cmd *cobra.Command, args []string) {
    path := args[0]
    
    // Examples:
    // jack delete firewall.rules.0
    //   -> Remove first rule
    
    // jack delete dhcp.static_leases --mac "aa:bb:cc:dd:ee:ff"
    //   -> Remove lease with matching MAC
    
    // Send delete request to daemon
}
```

3. Add handlers in `daemon/server.go`:
```go
case "add":
    return s.handleAdd(req.Path, req.Value)
case "delete":
    return s.handleDelete(req.Path)
```

**Files to create**:
- `cmd/add.go`
- `cmd/delete.go`
- Update `daemon/server.go` with add/delete handlers
- Update `daemon/path.go` with add/delete operations

---

## Priority 2: Status & Monitoring

### 2.1 Status Commands

**Status**: ✅ COMPLETED - Basic status command implemented with verbose mode

**Original requirements**:
```bash
# Overall system status
jack status
# Output:
# System Status:
#   Interfaces: 3 configured, 3 up
#   Firewall: Active (nftables)
#   DHCP: Active, 5 leases
#   Pending changes: Yes (2 changes)

# Interface status
jack status interfaces
# Output:
# Interfaces:
#   wan (fm1-mac9): UP, 10.0.0.199/24, gateway 10.0.0.1
#   lan (br-lan): UP, 192.168.1.1/24, 3 ports
#   guest (br-guest): DOWN (disabled)

# DHCP leases
jack status dhcp
# or
jack status dhcp leases
# Output:
# Active DHCP Leases:
#   192.168.1.100  aa:bb:cc:dd:ee:ff  laptop.lan      expires in 11h
#   192.168.1.101  11:22:33:44:55:66  phone.lan       expires in 10h

# Firewall status
jack status firewall
# Output:
# Firewall Status:
#   Backend: nftables
#   Zones: 2 (wan, lan)
#   Rules: 5 custom rules
#   NAT: Enabled on wan
#   Packets processed: 12,453 (5,234 dropped)
```

**Implementation**:

1. Create `cmd/status.go` with subcommands:
```go
var statusCmd = &cobra.Command{
    Use:   "status [subsystem]",
    Short: "Show system status",
    Args:  cobra.MaximumNArgs(1),
    Run:   runStatus,
}

var statusInterfacesCmd = &cobra.Command{
    Use:   "interfaces",
    Short: "Show interface status",
    Run:   runStatusInterfaces,
}

var statusDHCPCmd = &cobra.Command{
    Use:   "dhcp",
    Short: "Show DHCP status",
    Run:   runStatusDHCP,
}

var statusFirewallCmd = &cobra.Command{
    Use:   "firewall",
    Short: "Show firewall status",
    Run:   runStatusFirewall,
}

func init() {
    rootCmd.AddCommand(statusCmd)
    statusCmd.AddCommand(statusInterfacesCmd)
    statusCmd.AddCommand(statusDHCPCmd)
    statusCmd.AddCommand(statusFirewallCmd)
}
```

2. Add status collection in `daemon/server.go`:
```go
func (s *Server) handleStatus(subsystem string) Response {
    switch subsystem {
    case "", "all":
        return s.getOverallStatus()
    case "interfaces":
        return s.getInterfaceStatus()
    case "dhcp":
        return s.getDHCPStatus()
    case "firewall":
        return s.getFirewallStatus()
    default:
        return Response{
            Success: false,
            Error:   fmt.Sprintf("unknown subsystem: %s", subsystem),
        }
    }
}
```

3. Implement status collectors:
```go
// system/status.go
func GetInterfaceStatus() (*InterfaceStatus, error) {
    // Use netlink to get real interface state
    links, err := netlink.LinkList()
    // ...
    
    return &InterfaceStatus{
        Interfaces: interfaceList,
        TotalUp:    upCount,
        TotalDown:  downCount,
    }, nil
}

func GetDHCPStatus() (*DHCPStatus, error) {
    // Parse dnsmasq lease file
    leases, err := parseDnsmasqLeases("/var/lib/misc/dnsmasq.leases")
    // ...
    
    return &DHCPStatus{
        Active: true,
        Leases: leases,
    }, nil
}

func GetFirewallStatus() (*FirewallStatus, error) {
    // Query nftables
    cmd := exec.Command("nft", "list", "ruleset", "-j")
    output, err := cmd.Output()
    // Parse JSON output
    
    return &FirewallStatus{
        Backend:    "nftables",
        ZoneCount:  zoneCount,
        RuleCount:  ruleCount,
        PacketStats: stats,
    }, nil
}
```

---

### 2.2 Logs Command

**Status**: ✅ COMPLETED - Implemented in cmd/logs.go

**Original requirements**:
```bash
# Show Jack daemon logs
jack logs

# Follow logs in real-time
jack logs -f
jack logs --follow

# Show last N lines
jack logs -n 50

# Show logs since time
jack logs --since "1 hour ago"
```

**Implementation**:

Simple wrapper around journalctl:

```go
// cmd/logs.go
var logsCmd = &cobra.Command{
    Use:   "logs",
    Short: "Show Jack daemon logs",
    Run:   runLogs,
}

var (
    logsFollow bool
    logsLines  int
    logsSince  string
)

func init() {
    rootCmd.AddCommand(logsCmd)
    logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
    logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of lines to show")
    logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since time")
}

func runLogs(cmd *cobra.Command, args []string) {
    jcmd := []string{"journalctl", "-u", "jack"}
    
    if logsFollow {
        jcmd = append(jcmd, "-f")
    }
    
    if logsLines > 0 && !logsFollow {
        jcmd = append(jcmd, "-n", fmt.Sprintf("%d", logsLines))
    }
    
    if logsSince != "" {
        jcmd = append(jcmd, "--since", logsSince)
    }
    
    execCmd := exec.Command(jcmd[0], jcmd[1:]...)
    execCmd.Stdout = os.Stdout
    execCmd.Stderr = os.Stderr
    execCmd.Run()
}
```

---

## Priority 3: Safety & Robustness

### 3.1 Configuration Validation

**Problem**: Minimal validation before applying changes.

**Needed**:
- Validate IP addresses, netmasks, ports
- Check for conflicting configurations
- Validate interface references exist
- Check firewall rule syntax
- Dry-run mode

**Implementation**:

```go
// Add to each config type
func (c *InterfacesConfig) Validate() error {
    for name, iface := range c.Interfaces {
        // Validate IP addresses
        if iface.Protocol == "static" {
            if net.ParseIP(iface.IPAddr) == nil {
                return fmt.Errorf("invalid IP for %s: %s", name, iface.IPAddr)
            }
        }
        
        // Check for duplicate IPs
        // Check bridge ports exist
        // etc.
    }
    return nil
}

// Call before commit
jack commit
  -> Validate all pending configs
  -> If validation fails, reject commit with clear error message
```

---

### 3.2 Automatic Rollback on Failure

**Needed**:
- If `jack apply` fails, automatically rollback
- Keep backup of last known-good config
- Restore on failure

**Implementation**:

```go
func (s *Server) handleApply() Response {
    // Save current state
    backup := s.captureCurrentState()
    
    // Try to apply
    err := s.applyConfiguration()
    
    if err != nil {
        // Rollback
        s.restoreState(backup)
        return Response{
            Success: false,
            Error:   fmt.Sprintf("Apply failed, rolled back: %v", err),
        }
    }
    
    return Response{Success: true, Message: "Configuration applied"}
}
```

---

### 3.3 Safe Remote Access

**Problem**: Easy to lock yourself out via SSH when changing firewall.

**Needed**:
- Automatic SSH preserve rule
- Rollback timer (apply changes, auto-revert in 5 minutes unless confirmed)
- Warning when changing SSH port or WAN IP

**Implementation**:

```bash
# Apply with rollback timer
jack apply --confirm 5m
# Output: Configuration applied. Will auto-revert in 5 minutes.
#         Run 'jack confirm' to keep changes.

# After testing SSH still works:
jack confirm
# Output: Changes confirmed, rollback cancelled.

# If you don't confirm in 5 minutes:
# Automatic rollback happens
```

---

### 3.4 Backup & Restore

**Needed**:
```bash
# Create backup
jack backup
jack backup --output /backup/jack-20251111.tar.gz

# List backups
jack backup list

# Restore from backup
jack restore /backup/jack-20251111.tar.gz

# Show what's in a backup
jack backup show /backup/jack-20251111.tar.gz
```

**Implementation**:

```go
func CreateBackup(outputPath string) error {
    // Create tarball of /etc/jack/
    // Include metadata (timestamp, jack version, etc.)
}

func RestoreBackup(backupPath string) error {
    // Extract tarball
    // Validate configs
    // Restore to /etc/jack/
    // Offer to apply immediately
}
```

---

## Priority 4: Additional Network Features

### 4.1 VLAN Support

**Status**: ✅ COMPLETED - Implemented in system/network.go with interface ordering

---

### 4.2 WireGuard Support

**Status**: ✅ COMPLETED - Full implementation with peer management and key generation utility

Implemented features:
- WireGuard interface creation (system/network.go)
- Peer configuration with endpoints, allowed IPs, keepalive
- Key generation utility (jack wgkey command)
- Example configs and comprehensive guide ([WireGuard VPN Guide](../guides/wireguard-vpn.md))

---

### 4.3 Port Forwarding

**Status**: ✅ COMPLETED - Implemented in providers/firewall/nftables.go

Implemented features:
- DNAT rules in prerouting chain
- Automatic forward allow rules
- Port translation support
- Example configs and guide ([Port Forwarding Guide](../guides/port-forwarding.md))

---

### 4.4 Static Routes Implementation

**Status**: ✅ COMPLETED - Full implementation in system/routes.go

Implemented features:
- Gateway-based and interface-based routing
- Route metrics for priority/failover
- Support for routing tables
- Automatic duplicate route removal
- Example configs and comprehensive guide ([Static Routes Guide](../guides/static-routes.md))

---

## Priority 5: Developer Experience

### 5.1 Show Command with Pretty Printing

**Needed**:
```bash
jack show interfaces
jack show interfaces wan
jack show firewall
jack show firewall zones wan
```

With nice formatting, colors, tree view.

---

### 5.2 Shell Completion

**Needed**:
```bash
# Bash
jack completion bash > /etc/bash_completion.d/jack

# Zsh
jack completion zsh > ~/.zsh/completion/_jack

# Then:
jack get interfaces.<TAB>
# Shows: wan, lan, guest
```

Cobra supports this out of the box:

```go
var completionCmd = &cobra.Command{
    Use:   "completion [bash|zsh|fish|powershell]",
    Short: "Generate completion script",
    // Cobra handles the rest
}
```

---

### 5.3 Comprehensive Examples

Create `/etc/jack/examples/` with:
- `home-router.json` - Basic home router setup
- `multi-wan.json` - Dual WAN with failover
- `vlan-segmentation.json` - VLANs for IoT, guest, etc.
- `vpn-gateway.json` - WireGuard VPN server
- `hotspot.json` - WiFi hotspot configuration

Note: Currently have example configs in examples/config/ with comprehensive guides for VLANs, WireGuard, Port Forwarding, and Static Routes.

---

## Priority 6: Testing & Quality

### 6.1 Unit Tests

Add tests for:
- Path parsing
- Configuration diffing
- Provider implementations
- State management

---

### 6.2 Integration Tests

Test actual system changes:
- Create/delete interfaces
- Apply firewall rules
- Start/stop DHCP

---

### 6.3 CI/CD Pipeline

- Automated builds for multiple architectures
- Run tests on PR
- Create releases automatically

---

## Priority 7: Documentation

### 7.1 Man Pages

Create man pages:
- `man jack`
- `man jack-get`
- `man jack-set`
- `man jack.conf`

---

### 7.2 Tutorial/Cookbook

Step-by-step guides for common scenarios:
- "Setting up a home router"
- "Creating a guest network"
- "Setting up a VPN server"
- "Multi-WAN failover"

---

## Quick Wins

1. ✅ Logs command - COMPLETED
2. Shell completion - Cobra supports this out of the box
3. Version command improvements
4. Basic validation (IP addresses, ports, CIDR notation)
5. Show command (basic with pretty printing)
6. ✅ Port forwarding - COMPLETED

---

## Notes

- Priorities may shift based on user feedback
- Some features may be deprioritized or removed
- Integration with Netrunner (Rails UI) is separate project
- Consider community contributions for lower-priority items

---

## How to Use This TODO

When implementing a feature:

1. Create a new branch: `git checkout -b feature/multi-config-support`
2. Reference this file: "Implements: project-status.md section 1.1"
3. Update TODO when complete: Move from "Not Implemented" to "Implemented"
4. Add tests for new functionality
5. Update relevant documentation (README, Configuration Reference, guides, etc.)
6. Create pull request with reference to TODO section

When you (or an AI assistant) returns to this project, read:
1. README.md - What is Jack?
2. [Architecture](../architecture.md) - How does it work?
3. [Project Status](project-status.md) (this file) - What needs to be done?
4. Pick a section and implement it!