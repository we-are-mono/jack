# Getting Started with Jack

Jack is a transactional network configuration tool that makes managing Linux gateways and routers safe and predictable.

## Prerequisites

- Linux system (Debian/Ubuntu recommended)
- Root access
- Go 1.21+ (for building from source)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/we-are-mono/jack.git
cd jack

# Build Jack and plugins
./build.sh

# Install binaries
sudo cp jack /usr/local/bin/
sudo mkdir -p /usr/lib/jack/plugins
sudo cp bin/jack-plugin-* /usr/lib/jack/plugins/
```

### Install as Systemd Service

```bash
# Create systemd service file
sudo tee /etc/systemd/system/jack.service > /dev/null << 'EOF'
[Unit]
Description=Jack - Netrunner System Daemon
Documentation=https://github.com/we-are-mono/jack
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/jack daemon --apply
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable --now jack
```

## First Configuration

### 1. Create Configuration Directory

```bash
sudo mkdir -p /etc/jack
```

### 2. Configure Interfaces

Create `/etc/jack/interfaces.json`:

```json
{
  "interfaces": {
    "wan": {
      "type": "physical",
      "device": "eth0",
      "protocol": "dhcp",
      "enabled": true
    },
    "lan": {
      "type": "bridge",
      "device_name": "br-lan",
      "protocol": "static",
      "ipaddr": "192.168.1.1",
      "netmask": "255.255.255.0",
      "bridge_ports": ["eth1"],
      "enabled": true
    }
  },
  "version": "1.0"
}
```

### 3. Configure Jack Plugins

Create `/etc/jack/jack.json`:

```json
{
  "plugins": {
    "nftables": {
      "enabled": true,
      "provider": "nftables"
    },
    "dnsmasq": {
      "enabled": true,
      "provider": "dnsmasq"
    }
  },
  "version": "1.0"
}
```

### 4. Validate Configuration

Before applying, validate your configuration:

```bash
sudo jack validate
```

You should see:
```
✓ interfaces.json: valid
✓ jack.json: valid
✓ All configuration files are valid
```

### 5. Restart Daemon to Apply

```bash
sudo systemctl restart jack
```

Or if running daemon manually:
```bash
sudo jack daemon --apply
```

## Basic Usage

### View Current Configuration

```bash
jack show
```

### Check Status

```bash
jack status
```

### View Logs

```bash
jack logs
# or
tail -f /var/log/jack/jack.log
```

### Making Changes

Jack uses a transactional model:

```bash
# Make changes (stored as pending)
jack set interfaces lan ipaddr 192.168.2.1

# Review pending changes
jack diff

# Commit changes
jack commit

# Apply to system
jack apply
```

Or revert if you change your mind:
```bash
jack revert
```

## Common Tasks

### Add a VLAN

See [VLAN Guide](guides/vlans.md)

### Configure Port Forwarding

See [Port Forwarding Guide](guides/port-forwarding.md)

### Set Up VPN

See [WireGuard VPN Guide](guides/wireguard-vpn.md)

### Configure Static Routes

See [Static Routes Guide](guides/static-routes.md)

## Troubleshooting

### Daemon Won't Start

Check the logs:
```bash
sudo journalctl -u jack -n 50 --no-pager
```

Common issues:
- **Invalid JSON**: Run `jack validate` to find syntax errors
- **Missing dependencies**: Install `nftables`, `dnsmasq`, or `wireguard-tools` as needed
- **Permission denied**: Ensure jack is run as root

### Configuration Not Applied

1. Check daemon is running: `systemctl status jack`
2. View daemon logs: `tail -f /var/log/jack/jack.log`
3. Validate config: `jack validate`
4. Check for errors: `jack apply`

### Network Connectivity Lost

If you lose connectivity after applying config:

```bash
# Check interface status
ip addr show
ip link show

# Check routes
ip route show

# Restart jack daemon to reapply
sudo systemctl restart jack

# Or revert to backup config
sudo cp /etc/jack/interfaces.json.backup /etc/jack/interfaces.json
sudo systemctl restart jack
```

## Next Steps

- **[Configuration Reference](configuration.md)** - Complete configuration options
- **[Architecture](architecture.md)** - Understand how Jack works
- **[User Guides](README.md#user-guides)** - Feature-specific tutorials
- **[CLI Commands](reference/cli-commands.md)** - All available commands

## Getting Help

- Check the [FAQ](../README.md#faq)
- Browse [example configurations](../examples/config/)
- Report issues on [GitHub](https://github.com/we-are-mono/jack/issues)
