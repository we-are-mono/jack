# Jack - Network Configuration Daemon

[![Tests](https://github.com/we-are-mono/jack/actions/workflows/test.yml/badge.svg)](https://github.com/we-are-mono/jack/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/we-are-mono/jack/branch/master/graph/badge.svg)](https://codecov.io/gh/we-are-mono/jack)

Jack is a modern, transactional network configuration daemon for Linux systems. It provides a clean abstraction layer over Linux networking, firewalls, and services, making it easy to configure routers, gateways, and network appliances.

## ⚠️ Experimental Software Warning

**Jack is highly experimental and under active development. DO NOT use on production systems.**

This software is in early stages and may contain bugs, security vulnerabilities, or cause system instability. Use only in test environments, development systems, or lab setups. The API and configuration formats are subject to breaking changes without notice.

**Use at your own risk.**

## Features

- **Plugin Architecture**: 6 core plugins (nftables, dnsmasq, wireguard, monitoring, leds, sqlite3)
- **Transactional Configuration**: Stage changes, review them, then commit atomically
- **Auto-Configuration**: Daemon automatically applies configuration on startup
- **Multiple Config Sources**: Physical interfaces, bridges, VLANs, WireGuard, VXLAN
- **Firewall Management**: Zone-based nftables firewall with NAT and port forwarding
- **VPN Support**: WireGuard client and server configurations
- **DHCP/DNS Server**: Built-in services via dnsmasq plugin
- **System Monitoring**: Real-time metrics collection and bandwidth monitoring
- **SQLite3 Logging**: Persistent log storage with WAL mode for concurrent access
- **LED Control**: Hardware indicator management via sysfs
- **CLI Interface**: Simple, intuitive command-line interface with plugin commands
- **API Access**: Unix socket API for integration with web UIs or automation tools

## Quick Start

### Installation

Jack supports multiple architectures and package formats. Choose the installation method that works best for your system.

#### Supported Platforms

- **linux/arm64** - ARM 64-bit (Raspberry Pi 3/4/5, ARM servers)
- **linux/amd64** - x86_64 (Intel/AMD 64-bit)

#### Option 1: Debian/Ubuntu Package (.deb)

**Recommended for Debian, Ubuntu, and derivatives**

```bash
# Determine your architecture
ARCH=$(dpkg --print-architecture)

# Download latest release (replace VERSION with actual version, e.g., 0.1.0)
VERSION="0.1.0"
wget https://github.com/we-are-mono/jack/releases/download/v${VERSION}/jack_${VERSION}_${ARCH}.deb

# Install the package
sudo apt install ./jack_${VERSION}_${ARCH}.deb

# Start and enable the service
sudo systemctl enable --now jack
```

#### Option 2: Portable Archive (.tar.gz)

**For other Linux distributions or manual installation**

```bash
# Determine your architecture
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

# Download latest release (replace VERSION with actual version, e.g., 0.1.0)
VERSION="0.1.0"
wget https://github.com/we-are-mono/jack/releases/download/v${VERSION}/jack_${VERSION}_linux_${ARCH}.tar.gz

# Extract
tar xzf jack_${VERSION}_linux_${ARCH}.tar.gz
cd jack-${VERSION}-linux-${ARCH}

# Install (requires root)
sudo ./install.sh

# Start and enable the service
sudo systemctl enable --now jack
```

#### Verifying Downloads (Optional but Recommended)

```bash
# Download checksums file
wget https://github.com/we-are-mono/jack/releases/download/v${VERSION}/SHA256SUMS

# Verify package integrity
sha256sum -c SHA256SUMS --ignore-missing
```

#### Verifying Your Installation

After installation, verify Jack is working:

```bash
# Check version
jack --version

# Check service status
sudo systemctl status jack

# View available commands
jack --help
```

#### Dependencies

Jack requires these system packages (automatically installed with .deb):

- **nftables** (>= 0.9) - For firewall management
- **dnsmasq** (>= 2.80) - For DHCP/DNS services
- **wireguard-tools** - For VPN functionality
- **systemd** - For service management

On non-Debian systems, install these manually:

```bash
# Example for other distributions
sudo dnf install nftables dnsmasq wireguard-tools  # Fedora/RHEL
sudo pacman -S nftables dnsmasq wireguard-tools    # Arch Linux
sudo apk add nftables dnsmasq wireguard-tools      # Alpine Linux
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

1. **Jack Daemon** - Background service that manages system state and plugins
2. **Jack CLI** - Command-line interface for configuration
3. **Plugins** - RPC-based providers for different services (nftables, dnsmasq, wireguard, monitoring, leds, sqlite3)

```
┌─────────────┐
│  Jack CLI   │
└──────┬──────┘
       │ Unix Socket (/var/run/jack.sock)
┌──────▼──────┐
│ Jack Daemon │
└──────┬──────┘
       │ RPC (stdin/stdout)
   ┌───┴────┬─────────┬──────────┬───────────┬──────┐
   ▼        ▼         ▼          ▼           ▼      ▼
Network  Firewall   DHCP       VPN      Monitoring LEDs
 (ip)   (nftables) (dnsmasq) (wireguard) (metrics) (sysfs)
```

## Configuration Files

Jack uses JSON configuration files stored in `/etc/jack/`:

### Main Configuration
- `jack.json` - Main daemon configuration (enabled plugins, logging settings)
- `interfaces.json` - Network interface configuration

### Plugin Configurations
- `nftables.json` - Firewall rules and zones
- `dnsmasq.json` - DHCP and DNS server configuration
- `wireguard.json` - WireGuard VPN configuration
- `routes.json` - Static routes
- `leds.json` - LED hardware control settings
- `sqlite3.json` - Database and logging configuration
- `monitoring.json` - Metrics collection settings

### Templates
- `templates/` - Default configuration templates for all plugins

Each plugin has its own configuration file and can be enabled/disabled in `jack.json`. See [Configuration Reference](docs/configuration.md) for detailed documentation.

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

### Plugin Management
```bash
jack plugin list           # List all available plugins
jack plugin info <name>    # Show plugin information
jack plugin status <name>  # Show plugin status
jack plugin enable <name>  # Enable a plugin
jack plugin disable <name> # Disable a plugin
jack plugin set <name>     # Configure plugin settings
```

### Plugin Commands
```bash
# Monitoring plugin
jack monitor stats         # Show system resource usage (continuous)
jack monitor bandwidth <interface>  # Show bandwidth usage (continuous)

# SQLite3 plugin
jack sqlite3 stats         # Show database statistics
jack sqlite3 logs          # Query stored logs
jack sqlite3 vacuum        # Compact database

# LED plugin
jack led status            # Show LED states
jack led set <name> <state>  # Control LED

# Logs
jack logs watch            # Watch live logs (continuous)
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

Jack v0.1.0 is an experimental release with the following features:

### Core Functionality
- ✅ Network interfaces (physical, bridges)
- ✅ VLANs (802.1Q tagging)
- ✅ WireGuard VPN (client and server)
- ✅ DHCP client
- ✅ Static routes with metrics
- ✅ Firewall (zone-based nftables)
- ✅ NAT/Masquerading
- ✅ Port forwarding (DNAT)
- ✅ DHCP/DNS server (dnsmasq)
- ✅ System status monitoring
- ✅ Auto-apply configuration on startup

### Plugins
- ✅ **nftables** - Firewall management (23 rules on test deployment)
- ✅ **dnsmasq** - DHCP and DNS services
- ✅ **wireguard** - VPN tunnel management
- ✅ **monitoring** - Real-time system metrics and bandwidth monitoring
- ✅ **leds** - Hardware LED control
- ✅ **sqlite3** - Persistent logging with WAL mode

### Multi-Architecture Support
- ✅ AMD64 (x86_64) - Debian packages and tarballs
- ✅ ARM64 (aarch64) - Debian packages and tarballs
- ✅ Pure Go (CGO_ENABLED=0) - No C dependencies

## Community

- GitHub: https://github.com/we-are-mono/jack
- Issues: https://github.com/we-are-mono/jack/issues
- Discussions: https://github.com/we-are-mono/jack/discussions

## Related Projects

- **Netrunner** - Web UI for Jack (Rails-based)
- **OpenWRT** - Embedded Linux distribution (inspiration for Jack)