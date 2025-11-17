# Jack - Network Configuration Daemon

Jack is a modern, transactional network configuration daemon for Linux systems. It provides a clean abstraction layer over Linux networking, firewalls, and services, making it easy to configure routers, gateways, and network appliances.

## Features

- **Transactional Configuration**: Stage changes, review them, then commit atomically
- **Multiple Config Sources**: Physical interfaces, bridges, VLANs, WireGuard, VXLAN
- **Firewall Management**: nftables-based firewall with zones, NAT, and port forwarding
- **DHCP Server**: Built-in DHCP server (dnsmasq) with DNS integration
- **System Monitoring**: Real-time status of daemon, interfaces, services, and DHCP leases
- **Provider Architecture**: Pluggable providers for different implementations (nftables/iptables, dnsmasq/kea, etc.)
- **CLI Interface**: Simple, intuitive command-line interface
- **API Access**: Unix socket API for integration with web UIs or automation tools

## Quick Start

### Installation
```bash
# Download latest release
wget https://github.com/we-are-mono/jack/releases/latest/download/jack-linux-arm64
sudo mv jack-linux-arm64 /usr/local/bin/jack
sudo chmod +x /usr/local/bin/jack

# Install dependencies
sudo apt install nftables dnsmasq isc-dhcp-client

# Create config directory
sudo mkdir -p /etc/jack/{templates,providers}

# Install systemd service
sudo curl -o /etc/systemd/system/jack.service \
  https://raw.githubusercontent.com/we-are-mono/jack/main/contrib/jack.service
sudo systemctl daemon-reload
sudo systemctl enable jack
```

### Basic Configuration

Create `/etc/jack/interfaces.json`:
```json
{
  "version": "1",
  "interfaces": {
    "wan": {
      "type": "physical",
      "device": "eth0",
      "enabled": true,
      "protocol": "dhcp",
      "comment": "WAN port"
    },
    "lan": {
      "type": "bridge",
      "device_name": "br-lan",
      "bridge_ports": ["eth1", "eth2", "eth3"],
      "enabled": true,
      "protocol": "static",
      "ipaddr": "192.168.1.1",
      "netmask": "255.255.255.0",
      "comment": "LAN bridge"
    }
  }
}
```

Start Jack:
```bash
sudo systemctl start jack
jack apply
```

## Architecture

Jack consists of three main components:

1. **Jack Daemon** - Background service that manages system state
2. **Jack CLI** - Command-line interface for configuration
3. **Providers** - Pluggable backends for different services
```
┌─────────────┐
│  Jack CLI   │
└──────┬──────┘
       │ Unix Socket
┌──────▼──────┐
│ Jack Daemon │
└──────┬──────┘
       │
   ┌───┴────┬─────────┬──────────┐
   ▼        ▼         ▼          ▼
Network  Firewall   DHCP       DNS
 (ip)   (nftables) (dnsmasq) (dnsmasq)
```

## Configuration Files

Jack uses JSON configuration files stored in `/etc/jack/`:

- `interfaces.json` - Network interface configuration
- `nftables.json` - Firewall rules and zones
- `dnsmasq.json` - DHCP and DNS server configuration
- `wireguard.json` - WireGuard VPN configuration
- `routes.json` - Static routes
- `templates/` - Configuration templates for plugins

See [Configuration Reference](docs/configuration.md) for detailed documentation.

## Commands

### Daemon Management
```bash
jack daemon [--apply]       # Run daemon (with optional auto-apply on start)
```

### Configuration Management
```bash
jack get                   # Get configuration value
jack set                   # Stage configuration change
jack show [path]           # Show configuration
jack diff                  # Show pending changes
jack status [-v]           # Show system status (use -v for verbose)
jack commit                # Commit staged changes
jack revert                # Discard staged changes
jack apply                 # Apply committed configuration to system
```

### Utilities
```bash
jack wgkey                 # Generate WireGuard keypair
```

### Examples
```bash
# Check system status
jack status              # Quick overview
jack status -v           # Detailed status with interface stats

# Configure static IP
jack set interfaces wan protocol static
jack set interfaces wan ipaddr 10.0.0.199
jack set interfaces wan netmask 255.255.255.0
jack set interfaces wan gateway 10.0.0.1

# Review and apply
jack diff
jack commit
jack apply

# Switch to DHCP
jack set interfaces wan protocol dhcp
jack commit
jack apply
```

## Documentation

- **[Getting Started Guide](docs/getting-started.md)** - Installation and first steps
- **[Configuration Reference](docs/configuration.md)** - Complete configuration guide
- **[Architecture](docs/architecture.md)** - System design and components
- **[User Guides](docs/README.md)** - Feature tutorials (VLANs, VPN, routing, etc.)
- **[CLI Commands](docs/reference/cli-commands.md)** - Complete command reference
- **[Contributing](docs/development/contributing.md)** - Build instructions and development guide
- **[Project Status](docs/development/project-status.md)** - Feature roadmap

## Use Cases

- **Home Router**: Turn a mini PC into a full-featured router
- **Network Gateway**: Manage network edge devices
- **Lab Environment**: Quickly reconfigure test networks
- **Automation**: Integrate with Ansible, Terraform, or custom tools
- **Web UI Backend**: Use as backend for router web interfaces (like Netrunner)

## Comparison

| Feature | Jack | OpenWRT UCI | systemd-networkd | NetworkManager |
|---------|------|-------------|------------------|----------------|
| Transactional | ✅ | ✅ | ❌ | ❌ |
| Firewall Integration | ✅ | ✅ | ❌ | ❌ |
| DHCP Server | ✅ | ✅ | ❌ | ✅ |
| Modern Linux | ✅ | ❌ | ✅ | ✅ |
| Embedded Focus | ✅ | ✅ | ❌ | ❌ |
| API Access | ✅ | ❌ | ✅ | ✅ |

## License

GPL 2.0 - see LICENSE file for details

## Project Status

Jack is under active development. Current status:

- ✅ Network interfaces (physical, bridges)
- ✅ VLANs (802.1Q tagging)
- ✅ WireGuard VPN
- ✅ DHCP client
- ✅ Static routes
- ✅ Firewall (nftables)
- ✅ NAT/Masquerading
- ✅ Port forwarding (DNAT)
- ✅ DHCP server (dnsmasq)
- ✅ System status monitoring

## Community

- GitHub: https://github.com/we-are-mono/jack
- Issues: https://github.com/we-are-mono/jack/issues
- Discussions: https://github.com/we-are-mono/jack/discussions

## Related Projects

- **Netrunner** - Web UI for Jack (Rails-based)
- **OpenWRT** - Embedded Linux distribution (inspiration for Jack)