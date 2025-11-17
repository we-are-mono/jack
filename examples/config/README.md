# Example Configuration Files

> **Note**: Configuration guides have moved to [`/docs/guides/`](../../docs/guides/). See the [full documentation](../../docs/README.md) for tutorials on port forwarding, VLANs, VPN, routing, and more.

This directory contains example configuration files demonstrating various Jack gateway features.

## Basic Examples

### Network Interfaces

- **interfaces.json** - Basic network setup
  - WAN: `fm1-mac9` with static IP 10.0.0.199/24
  - LAN: Bridge `br-lan` with 3 ports at 192.168.1.1/24

- **interfaces-vlans.json** - VLAN segmentation example
  - Main LAN bridge (untagged)
  - Guest WiFi (VLAN 10)
  - IoT devices (VLAN 20)
  - Management network (VLAN 99)

- **interfaces-wireguard-server.json** - WireGuard VPN server embedded in interfaces
  - WAN/LAN interfaces
  - WireGuard interface with road warrior clients

### Firewall (nftables plugin)

- **nftables.json** - Basic firewall configuration
  - Default-deny policy
  - WAN zone with NAT masquerading
  - LAN zone with full access
  - LAN→WAN forwarding enabled

- **nftables-vlans.json** - Multi-zone VLAN firewall
  - Isolated guest and IoT zones
  - Management zone accessible from LAN
  - Per-zone forwarding rules

- **nftables-portforward.json** - Port forwarding examples
  - HTTP/HTTPS to web server
  - SSH on custom port
  - Game servers and remote desktop

### DHCP/DNS (dnsmasq plugin)

- **dnsmasq.json** - Basic DHCP/DNS server
  - Single LAN DHCP pool (192.168.1.100-250)
  - Local domain: `.lan`
  - Static lease example

- **dnsmasq-vlans.json** - Multi-VLAN DHCP/DNS
  - Separate DHCP pools for each VLAN
  - Per-VLAN DNS settings
  - Different lease times per network

- **dnsmasq-dns-only.json** - DNS-only server (no DHCP)
  - Custom DNS records (A, CNAME, TXT)
  - Upstream DNS forwarding

### VPN (wireguard plugin)

- **wireguard.json** - WireGuard VPN configuration
  - Road warrior setup with multiple clients
  - Separate from interfaces.json (plugin-based)

### Routing

- **routes.json** - Static route examples
  - Default route via WAN gateway
  - VPN network routing
  - Multi-WAN scenarios

## How Jack Works

When you run `jack apply`, the daemon:

1. Loads configuration files from `/etc/jack/`
2. Calls appropriate plugins for each subsystem:
   - Core network interfaces (built-in)
   - Plugins: nftables (firewall), dnsmasq (DHCP/DNS), wireguard (VPN)
3. Each plugin applies its configuration idempotently
4. Only affected services are restarted

## Deployment

To deploy these configs to your device:

```bash
# Copy config files
scp examples/config/*.json root@debian:/etc/jack/

# Copy template
scp -r examples/config/templates root@debian:/etc/jack/

# On the device, apply the configuration
ssh root@debian
jack apply
```

## Testing

To verify DHCP/DNS is working:

```bash
# Check dnsmasq status
systemctl status dnsmasq

# View generated config
cat /etc/dnsmasq.d/jack.conf

# View DHCP leases
cat /var/lib/misc/dnsmasq.leases

# Test DNS resolution
dig @192.168.1.1 router.lan
```
