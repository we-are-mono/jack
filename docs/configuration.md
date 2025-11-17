# Jack Configuration Reference

All Jack configuration files use JSON format and are stored in `/etc/jack/`.

## Configuration Files

- `interfaces.json` - Network interface configuration
- `firewall.json` - Firewall zones, rules, and NAT
- `dhcp.json` - DHCP server and DNS configuration
- `routes.json` - Static routes and policy routing

## Transactional Configuration Management

Jack uses a transactional workflow for configuration changes:

```bash
# 1. Make changes (staged in memory)
jack set interfaces wan enabled false
jack set firewall zones wan masquerade true
jack set dhcp dnsmasq cache_size 500

# 2. Review changes
jack diff

# 3. Commit changes (save to disk)
jack commit

# 4. Apply changes (apply to system)
jack apply
```

### Path Syntax

Jack uses space-separated arguments in CLI commands, which are internally joined with dots:

```
<config_type> <field> <subfield>...
```

**Path Components:**
- Config type: `interfaces`, `firewall`, `dhcp`, `routes`
- Map keys: Interface names, zone names, pool names, route names
- Field names: Match JSON field names (use underscores)

**Note:** Commands use space-separated arguments (e.g., `jack get interfaces wan ipaddr`), which are internally converted to dot-notation paths (`interfaces.wan.ipaddr`)

### Interfaces Paths

```bash
# Get/set interface fields
jack get interfaces wan enabled          # → true
jack get interfaces wan protocol         # → "dhcp"
jack get interfaces lan ipaddr           # → "192.168.1.1"

jack set interfaces wan enabled false
jack set interfaces wan protocol static
jack set interfaces wan ipaddr 10.0.0.199
jack set interfaces wan netmask 255.255.255.0
jack set interfaces wan gateway 10.0.0.1
jack set interfaces wan mtu 1500
jack set interfaces wan comment "Static WAN config"

# Get entire interface
jack get interfaces wan                  # → shows all fields
```

**Supported fields:** type, device, device_name, enabled, protocol, ipaddr, netmask, gateway, mtu, comment

### Firewall Paths

```bash
# Firewall defaults
jack get firewall defaults input         # → "DROP"
jack set firewall defaults input accept
jack set firewall defaults forward drop
jack set firewall defaults output accept
jack set firewall defaults syn_flood true
jack set firewall defaults drop_invalid true

# Firewall zones (nested access)
jack get firewall zones wan masquerade   # → true
jack get firewall zones lan input        # → "ACCEPT"

jack set firewall zones wan input drop
jack set firewall zones wan forward drop
jack set firewall zones wan output accept
jack set firewall zones wan masquerade true
jack set firewall zones wan mtu_fix true
jack set firewall zones wan family ipv4
jack set firewall zones lan comment "LAN zone with full access"

# Get entire zone
jack get firewall zones wan              # → shows all zone fields

# Get all zones
jack get firewall zones                  # → shows all zones

# Other firewall fields (read-only for now)
jack get firewall forwardings
jack get firewall rules
jack get firewall port_forwards
```

**Zone fields (settable):** input, forward, output, masquerade, mtu_fix, family, comment
**Zone fields (read-only):** name, interfaces

### DHCP Paths

```bash
# Dnsmasq global settings
jack get dhcp dnsmasq enabled            # → true
jack get dhcp dnsmasq cache_size         # → 150

jack set dhcp dnsmasq enabled true
jack set dhcp dnsmasq port 53
jack set dhcp dnsmasq domain lan
jack set dhcp dnsmasq local "/lan/"
jack set dhcp dnsmasq expand_hosts true
jack set dhcp dnsmasq read_ethers false
jack set dhcp dnsmasq leasefile "/tmp/dhcp.leases"
jack set dhcp dnsmasq resolvfile "/tmp/resolv.conf.auto"
jack set dhcp dnsmasq no_resolv false
jack set dhcp dnsmasq localise_queries true
jack set dhcp dnsmasq rebind_protection true
jack set dhcp dnsmasq rebind_localhost false
jack set dhcp dnsmasq authoritative true
jack set dhcp dnsmasq domain_needed true
jack set dhcp dnsmasq bogus_priv true
jack set dhcp dnsmasq filterwin2k false
jack set dhcp dnsmasq log_queries false
jack set dhcp dnsmasq log_dhcp false
jack set dhcp dnsmasq dns_forward_max 150
jack set dhcp dnsmasq cache_size 500

# DHCP pools (by pool name)
jack get dhcp dhcp_pools lan start       # → 100
jack get dhcp dhcp_pools lan limit       # → 150

jack set dhcp dhcp_pools lan interface lan
jack set dhcp dhcp_pools lan start 100
jack set dhcp dhcp_pools lan limit 150
jack set dhcp dhcp_pools lan leasetime "12h"
jack set dhcp dhcp_pools lan dhcpv4 server
jack set dhcp dhcp_pools lan dhcpv6 disabled
jack set dhcp dhcp_pools lan ra server
jack set dhcp dhcp_pools lan ra_management 1
jack set dhcp dhcp_pools lan ra_default 1
jack set dhcp dhcp_pools lan domain lan
jack set dhcp dhcp_pools lan comment "Main LAN pool"

# Get entire pool
jack get dhcp dhcp_pools lan             # → shows all pool fields

# Other DHCP fields (read-only for now)
jack get dhcp static_leases
jack get dhcp dns_records
```

**Dnsmasq fields (settable):** enabled, port, domain, local, expand_hosts, read_ethers, leasefile, resolvfile, no_resolv, localise_queries, rebind_protection, rebind_localhost, authoritative, domain_needed, bogus_priv, filterwin2k, log_queries, log_dhcp, dns_forward_max, cache_size
**Dnsmasq fields (read-only):** servers

**DHCP pool fields (settable):** interface, start, limit, leasetime, dhcpv4, dhcpv6, ra, ra_management, ra_default, domain, comment
**DHCP pool fields (read-only):** dns

### Routes Paths

```bash
# Routes (by route name)
jack get routes routes default enabled   # → true
jack get routes routes vpn gateway       # → "10.0.0.1"

jack set routes routes default destination "0.0.0.0/0"
jack set routes routes default gateway "192.168.1.254"
jack set routes routes default interface lan
jack set routes routes default metric 100
jack set routes routes default table 0
jack set routes routes default enabled true
jack set routes routes default comment "Default route via LAN"

# Get entire route
jack get routes routes default           # → shows all route fields

# Get all routes
jack get routes routes                   # → shows all routes
```

**Route fields (settable):** destination, gateway, interface, metric, table, enabled, comment
**Route fields (read-only):** name

### Field Types

Jack enforces type checking when setting values:

**Boolean fields** (true/false):
- `interfaces.*.enabled`
- `firewall.defaults.syn_flood`, `firewall.defaults.drop_invalid`
- `firewall.zones.*.masquerade`, `firewall.zones.*.mtu_fix`
- `dhcp.dnsmasq.enabled`, `dhcp.dnsmasq.expand_hosts`, etc.
- `routes.routes.*.enabled`

**String fields**:
- `interfaces.*.protocol`, `interfaces.*.ipaddr`, `interfaces.*.netmask`, `interfaces.*.comment`
- `firewall.defaults.input`, `firewall.defaults.forward`, `firewall.defaults.output`
- `firewall.zones.*.input`, `firewall.zones.*.forward`, `firewall.zones.*.output`, `firewall.zones.*.family`
- `dhcp.dnsmasq.domain`, `dhcp.dnsmasq.local`, etc.
- `routes.routes.*.destination`, `routes.routes.*.gateway`, `routes.routes.*.interface`

**Integer/Number fields**:
- `interfaces.*.mtu`
- `dhcp.dnsmasq.port`, `dhcp.dnsmasq.dns_forward_max`, `dhcp.dnsmasq.cache_size`
- `dhcp.dhcp_pools.*.start`, `dhcp.dhcp_pools.*.limit`
- `routes.routes.*.metric`, `routes.routes.*.table`

### Multi-Config Operations

Jack tracks changes across all config types and commits them together:

```bash
# Make changes to different config types
jack set interfaces wan protocol static
jack set firewall zones wan masquerade true
jack set dhcp dnsmasq cache_size 1000
jack set routes routes default metric 50

# Diff shows changes from ALL config types
jack diff
# Output:
# Found 4 change(s):
#   ~ interfaces.wan.protocol: "dhcp" → "static"
#   ~ firewall.zones.wan.masquerade: false → true
#   ~ dhcp.dnsmasq.cache_size: 150 → 1000
#   ~ routes.routes.default.metric: 100 → 50

# Commit saves ALL modified configs to disk
jack commit
# Output: All pending changes committed successfully

# Apply updates the running system
jack apply
```

### Common Workflows

**Change WAN from DHCP to static:**
```bash
jack set interfaces wan protocol static
jack set interfaces wan ipaddr 10.0.0.100
jack set interfaces wan netmask 255.255.255.0
jack set interfaces wan gateway 10.0.0.1
jack diff
jack commit
jack apply
```

**Enable NAT on WAN zone:**
```bash
jack set firewall zones wan masquerade true
jack commit
jack apply
```

**Increase DNS cache size:**
```bash
jack set dhcp dnsmasq cache_size 1000
jack commit
jack apply
```

**Disable a route:**
```bash
jack set routes routes vpn enabled false
jack commit
jack apply
```

**Revert uncommitted changes:**
```bash
jack set interfaces wan enabled false
jack diff                        # Shows the change
jack revert                      # Discards the change
jack diff                        # No changes
```

## interfaces.json

### Structure
```json
{
  "version": "1",
  "interfaces": {
    "<interface-name>": {
      "type": "physical|bridge|vlan|wireguard|vxlan",
      "enabled": true|false,
      "protocol": "static|dhcp|none",
      ...
    }
  }
}
```

### Physical Interface
```json
{
  "wan": {
    "type": "physical",
    "device": "eth0",
    "enabled": true,
    "protocol": "dhcp",
    "metric": 100,
    "mtu": 1500,
    "mac": null,
    "ipv6": {
      "enabled": true,
      "protocol": "dhcpv6"
    },
    "comment": "WAN port with DHCP"
  }
}
```

**Static IP Example:**
```json
{
  "wan": {
    "type": "physical",
    "device": "eth0",
    "enabled": true,
    "protocol": "static",
    "ipaddr": "10.0.0.199",
    "netmask": "255.255.255.0",
    "gateway": "10.0.0.1",
    "dns": ["8.8.8.8", "1.1.1.1"],
    "metric": 100
  }
}
```

### Bridge Interface
```json
{
  "lan": {
    "type": "bridge",
    "device_name": "br-lan",
    "bridge_ports": ["eth1", "eth2", "eth3"],
    "enabled": true,
    "protocol": "static",
    "ipaddr": "192.168.1.1",
    "netmask": "255.255.255.0",
    "bridge_stp": false,
    "bridge_forward_delay": 2,
    "comment": "LAN bridge with 3 ports"
  }
}
```

### VLAN Interface
```json
{
  "vlan10": {
    "type": "vlan",
    "device": "eth1",
    "vlan_id": 10,
    "device_name": "eth1.10",
    "enabled": true,
    "protocol": "static",
    "ipaddr": "10.0.10.1",
    "netmask": "255.255.255.0",
    "comment": "VLAN 10 for IoT devices"
  }
}
```

### WireGuard Interface
```json
{
  "wg0": {
    "type": "wireguard",
    "device_name": "wg0",
    "enabled": true,
    "protocol": "static",
    "ipaddr": "10.200.0.1",
    "netmask": "255.255.255.0",
    "private_key": "base64_encoded_private_key",
    "listen_port": 51820,
    "peers": [
      {
        "public_key": "peer_public_key",
        "allowed_ips": ["10.200.0.2/32"],
        "endpoint": "example.com:51820",
        "persistent_keepalive": 25
      }
    ],
    "comment": "WireGuard VPN"
  }
}
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Interface type |
| `device` | string | For physical | Physical device name (e.g., eth0) |
| `device_name` | string | For virtual | Virtual device name to create |
| `enabled` | boolean | Yes | Enable/disable interface |
| `protocol` | string | Yes | IP protocol: static, dhcp, none |
| `ipaddr` | string | For static | IPv4 address |
| `netmask` | string | For static | IPv4 netmask |
| `gateway` | string | Optional | Default gateway |
| `dns` | array | Optional | DNS servers |
| `metric` | integer | Optional | Route metric |
| `mtu` | integer | Optional | MTU size |
| `bridge_ports` | array | For bridge | List of interfaces in bridge |
| `vlan_id` | integer | For VLAN | VLAN ID (1-4094) |

## firewall.json

### Structure
```json
{
  "version": "1",
  "defaults": { ... },
  "zones": { ... },
  "forwardings": [ ... ],
  "rules": [ ... ],
  "port_forwards": [ ... ]
}
```

### Defaults
```json
{
  "defaults": {
    "input": "DROP",
    "forward": "DROP",
    "output": "ACCEPT",
    "syn_flood": true,
    "drop_invalid": true
  }
}
```

### Zones
```json
{
  "zones": {
    "wan": {
      "name": "wan",
      "interfaces": ["eth0"],
      "input": "DROP",
      "forward": "DROP",
      "output": "ACCEPT",
      "masquerade": true,
      "mtu_fix": true,
      "family": "ipv4",
      "comment": "WAN zone with NAT"
    },
    "lan": {
      "name": "lan",
      "interfaces": ["br-lan"],
      "input": "ACCEPT",
      "forward": "ACCEPT",
      "output": "ACCEPT",
      "masquerade": false,
      "comment": "LAN zone"
    }
  }
}
```

### Forwardings
```json
{
  "forwardings": [
    {
      "src": "lan",
      "dest": "wan",
      "family": "ipv4",
      "comment": "LAN to WAN forwarding"
    }
  ]
}
```

### Rules
```json
{
  "rules": [
    {
      "name": "Allow-SSH",
      "src": "wan",
      "proto": "tcp",
      "dest_port": "22",
      "target": "ACCEPT",
      "family": "ipv4"
    },
    {
      "name": "Allow-ICMP",
      "src": "wan",
      "proto": "icmp",
      "target": "ACCEPT",
      "family": "ipv4"
    }
  ]
}
```

### Port Forwards
```json
{
  "port_forwards": [
    {
      "name": "SSH to Internal Server",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "2222",
      "dest_ip": "192.168.1.10",
      "dest_port": "22",
      "target": "DNAT",
      "enabled": true,
      "comment": "Forward SSH to internal server"
    }
  ]
}
```

## dhcp.json

### Structure
```json
{
  "version": "1",
  "dnsmasq": { ... },
  "dhcp_pools": { ... },
  "static_leases": [ ... ],
  "dns_records": { ... }
}
```

### Dnsmasq Global Settings
```json
{
  "dnsmasq": {
    "enabled": true,
    "port": 53,
    "domain": "lan",
    "local": "/lan/",
    "expand_hosts": true,
    "authoritative": true,
    "cache_size": 150,
    "servers": ["8.8.8.8", "1.1.1.1"]
  }
}
```

### DHCP Pools
```json
{
  "dhcp_pools": {
    "lan": {
      "interface": "lan",
      "start": 100,
      "limit": 150,
      "leasetime": "12h",
      "dhcpv4": "server",
      "dns": ["192.168.1.1"],
      "comment": "Main LAN DHCP pool"
    }
  }
}
```

**Fields**:
- `start`: Starting host number (e.g., 100 = .100)
- `limit`: Number of addresses in pool
- Range will be: `<network>.start` to `<network>.(start+limit-1)`

### Static Leases
```json
{
  "static_leases": [
    {
      "name": "server01",
      "mac": "00:11:22:33:44:55",
      "ip": "192.168.1.10",
      "comment": "Main server"
    }
  ]
}
```

### DNS Records
```json
{
  "dns_records": {
    "a_records": [
      {
        "name": "router.lan",
        "ip": "192.168.1.1"
      }
    ],
    "cname_records": [
      {
        "cname": "gateway",
        "target": "router"
      }
    ]
  }
}
```

## routes.json

### Structure
```json
{
  "version": "1",
  "static_routes": [ ... ],
  "ipv6_routes": [ ... ],
  "policy_routes": [ ... ]
}
```

### Static Routes
```json
{
  "static_routes": [
    {
      "name": "route-to-remote-network",
      "enabled": true,
      "target": "10.20.0.0/16",
      "gateway": "192.168.1.254",
      "interface": "lan",
      "metric": 10,
      "comment": "Route to remote office"
    }
  ]
}
```

### Policy Routes
```json
{
  "policy_routes": [
    {
      "name": "guest-via-wan2",
      "enabled": true,
      "from": "192.168.100.0/24",
      "table": 100,
      "priority": 100,
      "comment": "Route guest traffic through secondary WAN"
    }
  ]
}
```

## Configuration Validation

Jack validates configuration before applying:

1. **JSON Syntax** - Must be valid JSON
2. **Schema Validation** - Required fields present
3. **Value Validation** - Valid IP addresses, ports, etc.
4. **Reference Validation** - Referenced interfaces exist
5. **Provider Validation** - Provider-specific checks (e.g., dnsmasq --test)

**Example Error:**
```bash
$ jack commit
[ERROR] Failed to commit: invalid IP address: "192.168.1.999"
```

## Best Practices

### 1. Always Use Comments
```json
{
  "lan": {
    "comment": "Main LAN - 3 ports for office devices"
  }
}
```

### 2. Use Logical Interface Names

Good: `wan`, `lan`, `guest`, `iot`, `dmz`
Bad: `int1`, `iface2`, `br0`

### 3. Set Explicit Metrics

For multi-WAN setups, set explicit metrics:
```json
{
  "wan": { "metric": 100 },
  "wan2": { "metric": 200 }
}
```

### 4. Document Firewall Rules
```json
{
  "name": "Allow-SSH",
  "comment": "SSH access from office network only"
}
```

### 5. Test Before Committing
```bash
jack set interfaces wan ipaddr 10.0.0.100
jack diff                    # Review changes
jack commit                  # Only commit if correct
jack apply                   # Apply to system
```

### 6. Keep Backups

Jack automatically creates backups:
```
/etc/jack/interfaces.json.backup.20251111-140530
```

But also keep your own:
```bash
cp -r /etc/jack /backup/jack-$(date +%Y%m%d)
```

## Migration from Other Systems

### From OpenWRT UCI
```bash
# UCI
uci set network.lan.ipaddr='192.168.1.1'
uci commit network
/etc/init.d/network restart

# Jack
jack set interfaces lan ipaddr 192.168.1.1
jack commit
jack apply
```

### From systemd-networkd
```bash
# systemd-networkd (/etc/systemd/network/10-lan.network)
[Match]
Name=eth1

[Network]
Address=192.168.1.1/24

# Jack (interfaces.json)
{
  "lan": {
    "type": "physical",
    "device": "eth1",
    "protocol": "static",
    "ipaddr": "192.168.1.1",
    "netmask": "255.255.255.0"
  }
}
```