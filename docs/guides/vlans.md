# VLAN Configuration Guide

This guide explains how to configure VLANs in Jack to create multiple isolated networks on a single physical interface, each with its own DHCP pool.

## What are VLANs?

VLANs (Virtual LANs) allow you to create multiple logical networks on a single physical interface by tagging traffic with VLAN IDs. This is commonly used for:

- **Network Segmentation**: Separate guest WiFi, IoT devices, and trusted devices
- **Security**: Isolate untrusted devices from your main network
- **Multiple DHCP Pools**: Different IP ranges on the same physical port

## Architecture

```
┌─────────────┐
│   WAN Port  │ fm1-mac9 (10.0.0.199/24)
└─────────────┘

┌─────────────────────────────────────┐
│   LAN Bridge  (192.168.1.0/24)      │
│   br-lan                             │
│   ├─ fm1-mac2 (untagged)            │
│   └─ fm1-mac5 (untagged)            │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│   fm1-mac6 (trunk port)             │
│   ├─ VLAN 10: Guest   (192.168.10.0/24)
│   ├─ VLAN 20: IoT     (192.168.20.0/24)
│   └─ VLAN 99: Mgmt    (192.168.99.0/24)
└─────────────────────────────────────┘
```

## Configuration Files

### 1. interfaces-vlans.json

Defines the network interfaces including VLAN subinterfaces:

```json
{
  "guest": {
    "type": "vlan",
    "device": "fm1-mac6",           // Parent physical interface
    "device_name": "fm1-mac6.10",   // VLAN subinterface name
    "vlan_id": 10,                  // VLAN tag
    "enabled": true,
    "protocol": "static",
    "ipaddr": "192.168.10.1",       // Gateway IP for this VLAN
    "netmask": "255.255.255.0"
  }
}
```

**Key fields for VLANs:**
- `type`: Must be `"vlan"`
- `device`: Parent physical interface
- `device_name`: Name for the VLAN subinterface (convention: `parent.vlanid`)
- `vlan_id`: VLAN tag ID (1-4094)
- `protocol`: `"static"` for gateway, `"dhcp"` to get IP from network, `"none"` for L2 only

### 2. dhcp-vlans.json

Defines DHCP pools for each network:

```json
{
  "dhcp_pools": {
    "guest": {
      "interface": "guest",         // Must match interface name
      "start": 100,                 // Start of range: .100
      "limit": 50,                  // Pool size: .100-.150
      "leasetime": "2h",            // Short leases for guests
      "dns": ["192.168.10.1"],      // DNS server (the gateway)
      "domain": "guest"
    }
  }
}
```

Each VLAN interface gets its own:
- IP range (start + limit)
- Lease time
- DNS servers
- Domain name

### 3. firewall-vlans.json

Defines security zones and forwarding rules:

```json
{
  "zones": {
    "guest": {
      "name": "guest",
      "interfaces": ["fm1-mac6.10"],    // VLAN subinterface
      "input": "DROP",                   // Drop incoming to router
      "forward": "DROP",                 // Drop forwarding by default
      "masquerade": false
    }
  },
  "forwardings": [
    {
      "src": "guest",
      "dest": "wan",
      "comment": "Guest to internet only - isolated from LAN"
    }
  ]
}
```

**Security best practices:**
- Guest and IoT zones: Default DROP, only allow WAN access
- LAN zone: Full access to all zones
- Management zone: Only accessible from LAN

## Common VLAN Scenarios

### Scenario 1: Guest WiFi Network

**Goal**: Provide internet access to guests without access to your main network

```json
// interfaces-vlans.json
"guest": {
  "type": "vlan",
  "device": "fm1-mac6",
  "device_name": "fm1-mac6.10",
  "vlan_id": 10,
  "protocol": "static",
  "ipaddr": "192.168.10.1",
  "netmask": "255.255.255.0"
}

// dhcp-vlans.json - Short leases for guests
"guest": {
  "interface": "guest",
  "start": 100,
  "limit": 50,
  "leasetime": "2h"
}

// firewall-vlans.json - Internet only
"forwardings": [
  {
    "src": "guest",
    "dest": "wan"
  }
]
```

### Scenario 2: IoT Device Isolation

**Goal**: IoT devices can access internet but not your computers/NAS

```json
"iot": {
  "type": "vlan",
  "device": "fm1-mac6",
  "vlan_id": 20,
  "ipaddr": "192.168.20.1",
  "netmask": "255.255.255.0"
}
```

Firewall: Only allow IoT → WAN, but LAN → IoT (for management)

### Scenario 3: Management Network

**Goal**: Dedicated network for switches, APs, servers

```json
"management": {
  "type": "vlan",
  "device": "fm1-mac6",
  "vlan_id": 99,
  "ipaddr": "192.168.99.1",
  "netmask": "255.255.255.0"
}
```

Use static DHCP leases for known devices:

```json
"static_leases": [
  {
    "mac": "aa:bb:cc:dd:ee:02",
    "ip": "192.168.99.10",
    "hostname": "switch"
  }
]
```

## Switch Configuration

Your network switch must support 802.1Q VLAN tagging. Configure the port connected to Jack's `fm1-mac6` as a **trunk port** that passes:
- VLAN 10 (tagged)
- VLAN 20 (tagged)
- VLAN 99 (tagged)

Example (managed switch CLI):
```
interface ethernet 1/1
  switchport mode trunk
  switchport trunk allowed vlan 10,20,99
  switchport trunk native vlan none
```

For WiFi access points, configure SSIDs to use specific VLANs:
- Main WiFi → Untagged (LAN)
- Guest WiFi → VLAN 10
- IoT WiFi → VLAN 20

## Deployment

1. **Copy configuration files:**
   ```bash
   scp examples/config/interfaces-vlans.json root@gateway:/etc/jack/interfaces.json
   scp examples/config/dhcp-vlans.json root@gateway:/etc/jack/dnsmasq.json
   scp examples/config/firewall-vlans.json root@gateway:/etc/jack/nftables.json
   ```

2. **Restart daemon and apply:**
   ```bash
   ssh root@gateway
   systemctl restart jack
   jack apply
   ```

3. **Verify VLAN interfaces:**
   ```bash
   ip link show | grep fm1-mac6
   # Should show: fm1-mac6, fm1-mac6.10, fm1-mac6.20, fm1-mac6.99

   ip addr show fm1-mac6.10
   # Should show: 192.168.10.1/24
   ```

4. **Check DHCP is serving all networks:**
   ```bash
   cat /etc/dnsmasq.d/jack.conf
   # Should contain dhcp-range entries for each VLAN

   systemctl status dnsmasq
   ```

5. **Test connectivity:**
   ```bash
   # From a guest device (connected to VLAN 10):
   # - Should get IP in 192.168.10.100-150 range
   # - Should be able to ping 8.8.8.8 (internet)
   # - Should NOT be able to ping 192.168.1.1 (main LAN)
   ```

## Troubleshooting

### VLAN interface not created
```bash
# Check parent interface exists and is up
ip link show fm1-mac6

# Try creating VLAN manually to test
ip link add link fm1-mac6 name fm1-mac6.10 type vlan id 10
ip link set fm1-mac6.10 up
```

### DHCP not working on VLAN
```bash
# Check dnsmasq is listening on VLAN interface
tcpdump -i fm1-mac6.10 port 67 or port 68

# Check generated dnsmasq config
grep "fm1-mac6.10" /etc/dnsmasq.d/jack.conf
```

### Can't route between VLANs
- This is intentional! VLANs are isolated by firewall rules
- To allow routing, add forwarding rules in `firewall-vlans.json`
- Check that IP forwarding is enabled: `cat /proc/sys/net/ipv4/ip_forward` (should be 1)

### Switch not passing VLAN traffic
- Verify switch port is configured as trunk
- Check VLAN IDs match between Jack config and switch config
- Test with `tcpdump -i fm1-mac6 -e` and look for `vlan` tags in output

## Advanced: VLANs on Bridges

For more complex scenarios, you can create bridges on top of VLANs:

```json
{
  "guest_vlan": {
    "type": "vlan",
    "device": "fm1-mac6",
    "vlan_id": 10,
    "protocol": "none"              // No IP on VLAN itself
  },
  "guest_bridge": {
    "type": "bridge",
    "device_name": "br-guest",
    "bridge_ports": ["fm1-mac6.10", "fm1-mac3"],  // Bridge VLAN + physical port
    "protocol": "static",
    "ipaddr": "192.168.10.1",
    "netmask": "255.255.255.0"
  }
}
```

This allows you to have both VLAN-tagged traffic and untagged traffic on the same logical network.

## Interface Ordering

Jack automatically applies interfaces in the correct order:
1. **Physical** interfaces first
2. **VLAN** interfaces second (require physical interfaces)
3. **Bridge** interfaces last (may use physical or VLAN interfaces)

You don't need to worry about ordering in your config file.
