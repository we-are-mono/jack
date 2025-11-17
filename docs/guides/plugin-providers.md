# Jack Plugin Provider Configuration

Jack uses a plugin-based architecture to provide flexibility in how different services are implemented. This document explains how to configure which plugins provide which services.

## Provider Configuration (`jack.json`)

The `/etc/jack/jack.json` file (or `jack.json` in your working directory) is used to specify which plugin should provide each service.

### Example Configuration

```json
{
  "version": "1.0",
  "providers": {
    "firewall": "nftables",
    "dhcp": "dnsmasq",
    "dns": "dnsmasq"
  }
}
```

### Available Services

- **firewall**: Firewall management (e.g., nftables, iptables)
- **dhcp**: DHCP server functionality
- **dns**: DNS server functionality
- **vpn**: VPN services (future)
- **ddns**: Dynamic DNS (future)
- **monitor**: Monitoring and metrics (future)

### Provider Selection

Each service can be provided by a different plugin. For example:
- Use `nftables` for firewall
- Use `dnsmasq` for both DHCP and DNS
- Or use `dnsmasq` for DHCP and `unbound` for DNS

## Multi-Capability Plugins

Some plugins can provide multiple services. For example, the `dnsmasq` plugin can provide both DHCP and DNS services:

```json
{
  "providers": {
    "dhcp": "dnsmasq",
    "dns": "dnsmasq"
  }
}
```

Jack will load the plugin once and dispense both the DHCP and DNS capabilities from it.

## Separate Configuration Files

Each service has its own configuration file:
- `/etc/jack/nftables.json` - Firewall rules and zones
- `/etc/jack/dnsmasq.json` - DHCP server settings and pools
- `/etc/jack/dnsmasq-dns-only.json` - DNS server settings and records

This separation allows you to:
1. Mix and match providers (e.g., dnsmasq for DHCP, unbound for DNS)
2. Keep configurations clean and focused
3. Manage services independently

## Example: Using Different Providers

If you want to use `dnsmasq` for DHCP but `unbound` for DNS:

```json
{
  "version": "1.0",
  "providers": {
    "firewall": "nftables",
    "dhcp": "dnsmasq",
    "dns": "unbound"
  }
}
```

Then configure each service in its respective file:
- `dnsmasq.json` - Configure dnsmasq DHCP settings
- `dnsmasq-dns-only.json` - Configure unbound DNS settings

## Plugin Discovery

Jack searches for plugins in the following directories (in order):
1. `./bin/` - Local development
2. `/usr/lib/jack/plugins/` - System installation
3. `/opt/jack/plugins/` - Alternative installation

Plugins follow the naming convention: `jack-plugin-{name}`

For example:
- `jack-plugin-nftables` provides firewall capability
- `jack-plugin-dnsmasq` provides both DHCP and DNS capabilities

## Core Plugins

Jack ships with these core plugins:
- **nftables**: Linux nftables firewall provider
- **dnsmasq**: DHCP and DNS server provider (provides both capabilities)

Additional plugins can be installed separately or developed as needed.

### Multi-Capability: dnsmasq Plugin

The `dnsmasq` plugin is special because it provides both DHCP and DNS services. This means:
- One plugin binary (`jack-plugin-dnsmasq`) handles both services
- Jack loads the plugin once and dispenses both capabilities
- You can use dnsmasq for DHCP only, DNS only, or both
- Configuration is still kept separate in `dnsmasq.json` and `dnsmasq-dns-only.json`

Example using dnsmasq for both:
```json
{
  "providers": {
    "firewall": "nftables",
    "dhcp": "dnsmasq",
    "dns": "dnsmasq"
  }
}
```

The plugin architecture allows this flexibility while keeping configurations clean and service-specific.
