# Jack Architecture

## Design Principles

1. **Transactional by Default** - All changes are staged, reviewed, and committed
2. **Source of Truth** - Jack configuration files are authoritative
3. **Idempotent Operations** - Applying the same config multiple times is safe
4. **Provider Abstraction** - Multiple implementations (nftables/iptables, dnsmasq/kea)
5. **Modern Linux First** - Built for standard Linux, not resource-constrained embedded

## System Components

### 1. Jack Daemon

The daemon is the core of Jack. It:
- Runs as a systemd service
- Listens on Unix socket (`/var/run/jack.sock`)
- Maintains in-memory state (pending vs committed config)
- Applies configuration to the system
- Monitors for changes

**Technology**: Go, netlink, nftables

### 2. Jack CLI

The CLI is a thin client that:
- Connects to daemon via Unix socket
- Sends commands in JSON format
- Displays results to user
- No direct system access

**Technology**: Go, Cobra

### 3. State Management

Jack maintains two configuration states:
```
Committed (Disk)         Pending (Memory)
/etc/jack/*.json    →    Daemon RAM
      ↓                        ↓
   jack apply            jack commit
      ↓                        ↓
   System                 Committed
```

**Flow**:
1. User runs `jack set` → Changes stored in daemon memory (pending)
2. User runs `jack commit` → Pending written to `/etc/jack/*.json` (committed)
3. User runs `jack apply` → Committed config applied to system

### 4. Provider System

Providers abstract different implementations:
```go
type FirewallProvider interface {
    ApplyConfig(config *FirewallConfig) error
    Validate(config *FirewallConfig) error
    Flush() error
    Status() (*Status, error)
}
```

**Current Providers**:
- Firewall: nftables
- DHCP: dnsmasq
- Network: Linux netlink

**Future Providers**:
- Firewall: iptables
- DHCP: kea, systemd-networkd
- DNS: bind, unbound

### 5. Configuration Files

All config is JSON for:
- Easy parsing (Go, Rails, Python, etc.)
- Human-readable
- Schema validation
- API-friendly

**Location**: `/etc/jack/`

**Files**:
- `interfaces.json` - Network interfaces
- `firewall.json` - Firewall rules
- `dhcp.json` - DHCP server
- `routes.json` - Static routes
- `templates/` - Provider templates

### 6. Templates

Templates are used by providers to generate native config files:
- Stored in `/etc/jack/templates/`
- Use Go's `text/template` syntax
- Separate from code (no recompilation needed)
- Example: `dnsmasq.conf.tmpl` → `/etc/dnsmasq.d/jack.conf`

## Data Flow

### Configuration Change Flow
```
User
  ↓ jack set interfaces wan ipaddr 10.0.0.1
Daemon (receives via socket)
  ↓ Updates pending state in memory
User
  ↓ jack diff
Daemon
  ↓ Compares pending vs committed
  ↓ Returns differences
User
  ↓ jack commit
Daemon
  ↓ Writes pending → /etc/jack/interfaces.json
  ↓ Clears pending state
User
  ↓ jack apply
Daemon
  ↓ Reads /etc/jack/interfaces.json
  ↓ Applies via providers
     ↓
   [netlink] → Linux kernel (interfaces, routes)
   [nftables] → Linux kernel (firewall)
   [dnsmasq] → Systemd service (DHCP/DNS)
```

### Apply Process

When `jack apply` runs:

1. **Enable IP Forwarding** (sysctl)
2. **Apply Interfaces** (in order):
   - Physical interfaces (bring up, set IP, set gateway)
   - Bridges (create bridge, add ports, set IP)
   - VLANs (create VLAN interface, set IP)
3. **Apply Firewall**:
   - Flush existing rules
   - Create zones and chains
   - Apply custom rules
   - Apply forwarding rules
   - Apply NAT/masquerade
   - Set default policies
4. **Apply DHCP**:
   - Generate dnsmasq config from template
   - Validate config
   - Reload dnsmasq service

## Security Considerations

### Privilege Separation

- **Daemon**: Runs as root (needs netlink, iptables access)
- **CLI**: Runs as user, communicates via socket
- **Socket**: Permissions set to 0666 (world-writable, but root owns daemon)

### Safe Remote Access

To prevent SSH lockout:
- Firewall rules allow established connections by default
- SSH access preserved unless explicitly blocked
- Future: "Safe mode" that auto-reverts on connection loss

### Configuration Validation

- JSON schema validation before commit
- Provider validation before apply
- Dry-run mode (future feature)

## Performance

### Startup Time
- Daemon starts in <1 second
- Config load: ~10ms per file
- Apply interfaces: ~100ms per interface
- Apply firewall: ~500ms (flush + rebuild)

### Memory Usage
- Daemon: ~10MB idle
- Config in memory: ~1MB per 100 interfaces

### Scalability
- Designed for: 1-100 interfaces
- Tested with: 10 interfaces, 50 firewall rules
- Not designed for: Data center scale (1000s of interfaces)

## Future Architecture Considerations

### 1. High Availability
- Config sync between multiple nodes
- Failover for gateway redundancy
- VRRP integration

### 2. Observability
- Metrics export (Prometheus)
- Structured logging
- Event streaming

### 3. Automation
- Webhook support
- Integration with Ansible/Terraform
- GitOps workflow

### 4. Web UI
- Netrunner (Rails app)
- REST API in addition to Unix socket
- WebSocket for real-time updates