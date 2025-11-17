# Static Routes Guide

Static routes allow you to control how traffic flows through your network by manually defining paths to specific networks or hosts. This guide explains how to configure static routes using Jack.

## What are Static Routes?

Static routes tell your gateway how to reach networks that aren't directly connected. Instead of relying solely on dynamic routing protocols, you explicitly define:

- **Where traffic should go** (destination network)
- **How to get there** (via which gateway or interface)
- **Priority** (which route to prefer if multiple exist)

## Use Cases

- **Multiple WAN connections**: Define which traffic uses which internet connection
- **VPN routing**: Route specific networks through VPN tunnels
- **Site-to-Site networks**: Connect to remote office networks
- **Failover routes**: Define backup routes with higher metrics
- **Policy routing**: Direct traffic based on source or destination
- **Internal segmentation**: Route between different network segments

## Configuration Structure

Routes are configured in [/etc/jack/routes.json](routes.json):

```json
{
  "version": "1",
  "routes": [
    {
      "name": "route-name",
      "destination": "10.0.0.0/8",
      "gateway": "192.168.1.254",
      "interface": "wan",
      "metric": 10,
      "table": 0,
      "enabled": true,
      "comment": "Description of this route"
    }
  ]
}
```

### Field Descriptions

**Required Fields:**
- **name**: Unique identifier for this route
- **destination**: Target network in CIDR notation (e.g., "10.0.0.0/8") or "default" for 0.0.0.0/0
- **enabled**: Whether this route is active (true/false)

**Routing Fields (at least one required):**
- **gateway**: IP address of next-hop router
- **interface**: Network interface to use for this route

**Optional Fields:**
- **metric**: Route priority (lower values are preferred, default: 0)
- **table**: Routing table ID (default: main table)
- **comment**: Human-readable description

### Gateway vs Interface

You must specify at least one of these:

1. **Gateway only**: Route via a specific IP address
   ```json
   {
     "destination": "10.0.0.0/8",
     "gateway": "192.168.1.254"
   }
   ```

2. **Interface only**: Route directly through an interface (point-to-point links)
   ```json
   {
     "destination": "172.16.0.0/12",
     "interface": "wg0"
   }
   ```

3. **Both**: Specify gateway AND interface (recommended for clarity)
   ```json
   {
     "destination": "10.0.0.0/8",
     "gateway": "192.168.1.254",
     "interface": "wan"
   }
   ```

## Scenario 1: Default Route

Define which gateway provides internet access:

```json
{
  "version": "1",
  "routes": [
    {
      "name": "default-internet",
      "destination": "default",
      "gateway": "192.168.1.1",
      "interface": "wan",
      "metric": 100,
      "enabled": true,
      "comment": "Primary internet gateway"
    }
  ]
}
```

**Note**: Default routes are also configured via interface settings (`gateway` field). Only use routes.json if you need advanced control.

## Scenario 2: Multi-WAN with Failover

Primary and backup internet connections:

```json
{
  "version": "1",
  "routes": [
    {
      "name": "primary-wan",
      "destination": "default",
      "gateway": "192.168.1.1",
      "interface": "wan",
      "metric": 100,
      "enabled": true,
      "comment": "Primary ISP connection"
    },
    {
      "name": "backup-wan",
      "destination": "default",
      "gateway": "192.168.2.1",
      "interface": "wan2",
      "metric": 200,
      "enabled": true,
      "comment": "Backup ISP (higher metric = lower priority)"
    }
  ]
}
```

Linux will automatically use the backup route if the primary fails.

## Scenario 3: VPN Split Tunneling

Route specific networks through VPN, rest through normal internet:

```json
{
  "version": "1",
  "routes": [
    {
      "name": "corporate-network",
      "destination": "10.0.0.0/8",
      "interface": "wg0",
      "metric": 10,
      "enabled": true,
      "comment": "Corporate networks via WireGuard VPN"
    },
    {
      "name": "partner-network",
      "destination": "172.16.0.0/12",
      "interface": "wg0",
      "metric": 10,
      "enabled": true,
      "comment": "Partner company network via VPN"
    },
    {
      "name": "default-direct",
      "destination": "default",
      "gateway": "192.168.1.1",
      "interface": "wan",
      "metric": 100,
      "enabled": true,
      "comment": "Everything else goes direct to internet"
    }
  ]
}
```

## Scenario 4: Site-to-Site Routing

Connect multiple office locations:

```json
{
  "version": "1",
  "routes": [
    {
      "name": "office-hq",
      "destination": "192.168.1.0/24",
      "gateway": "10.0.9.1",
      "interface": "wg-site-hq",
      "metric": 10,
      "enabled": true,
      "comment": "Headquarters LAN"
    },
    {
      "name": "office-branch1",
      "destination": "192.168.2.0/24",
      "gateway": "10.0.9.2",
      "interface": "wg-site-branch1",
      "metric": 10,
      "enabled": true,
      "comment": "Branch office 1 LAN"
    },
    {
      "name": "office-branch2",
      "destination": "192.168.3.0/24",
      "gateway": "10.0.9.3",
      "interface": "wg-site-branch2",
      "metric": 10,
      "enabled": true,
      "comment": "Branch office 2 LAN"
    }
  ]
}
```

## Scenario 5: Load Balancing by Destination

Route different networks through different connections:

```json
{
  "version": "1",
  "routes": [
    {
      "name": "cdn-traffic",
      "destination": "8.8.8.0/24",
      "gateway": "192.168.1.1",
      "interface": "wan",
      "metric": 10,
      "enabled": true,
      "comment": "Route Google services through WAN"
    },
    {
      "name": "streaming-traffic",
      "destination": "23.0.0.0/8",
      "gateway": "192.168.2.1",
      "interface": "wan2",
      "metric": 10,
      "enabled": true,
      "comment": "Route streaming services through WAN2"
    },
    {
      "name": "default",
      "destination": "default",
      "gateway": "192.168.1.1",
      "interface": "wan",
      "metric": 100,
      "enabled": true,
      "comment": "Default route"
    }
  ]
}
```

## Understanding Metrics

The metric determines route priority when multiple routes to the same destination exist:

- **Lower metric = higher priority**
- **Metric 0**: Highest priority
- **Metric 100**: Standard priority
- **Metric 200+**: Backup routes

Example:
```json
[
  {
    "name": "fast-route",
    "destination": "10.0.0.0/8",
    "gateway": "192.168.1.1",
    "metric": 10,
    "enabled": true
  },
  {
    "name": "slow-backup",
    "destination": "10.0.0.0/8",
    "gateway": "192.168.2.1",
    "metric": 100,
    "enabled": true
  }
]
```

Traffic will use `fast-route` (metric 10) unless it's unavailable, then fall back to `slow-backup` (metric 100).

## Advanced: Routing Tables

Linux supports multiple routing tables for policy-based routing:

```json
{
  "name": "special-table",
  "destination": "10.0.0.0/8",
  "gateway": "192.168.1.254",
  "table": 100,
  "enabled": true,
  "comment": "Route in custom table 100"
}
```

**Common table IDs:**
- **254** (main): Default routing table (table: 0 uses this)
- **255** (local): System-generated local routes
- **1-253**: Custom tables for policy routing

**Note**: Using custom tables requires additional `ip rule` configuration (not yet supported via Jack config).

## Deployment

1. Create or edit `/etc/jack/routes.json`:
   ```bash
   sudo nano /etc/jack/routes.json
   ```

2. Apply the configuration:
   ```bash
   jack apply
   ```

3. Verify routes were added:
   ```bash
   ip route show
   ```

## Verification Commands

### View all routes
```bash
ip route show
```

Output example:
```
default via 192.168.1.1 dev wan metric 100
10.0.0.0/8 via 192.168.1.254 dev wan metric 10
172.16.0.0/12 dev wg0 scope link metric 20
192.168.1.0/24 dev lan scope link
```

### View specific table
```bash
ip route show table main
ip route show table 100
```

### Test route selection
```bash
ip route get 10.0.5.100
```

Shows which route would be used for that destination.

### Trace route path
```bash
traceroute 10.0.5.100
```

Shows the actual path packets take.

## Troubleshooting

### Route not appearing

1. **Check configuration syntax**:
   ```bash
   cat /etc/jack/routes.json | jq
   ```

2. **Verify interface exists**:
   ```bash
   ip link show
   ```

3. **Check jack logs**:
   ```bash
   journalctl -u jack -f
   ```

4. **Manually test route**:
   ```bash
   sudo ip route add 10.0.0.0/8 via 192.168.1.254
   ```

### Traffic not using route

1. **Check route priority**:
   - Lower metric routes take precedence
   - More specific routes (longer prefix) take precedence over less specific

2. **Verify gateway is reachable**:
   ```bash
   ping 192.168.1.254
   ```

3. **Check if gateway is on same subnet**:
   - Gateway must be directly reachable on the interface

4. **Firewall rules**:
   - Ensure firewall allows forwarding between zones

### Routes conflicting

When multiple routes match:
1. **Most specific prefix wins**: 10.0.5.0/24 beats 10.0.0.0/8
2. **Lower metric wins**: metric 10 beats metric 100
3. **Both used**: Equal-cost multipath (ECMP) may balance traffic

To avoid conflicts:
- Use different metrics for backup routes
- Disable conflicting routes
- Make routes more specific

## Integration with Jack Features

### With VLANs

Route traffic to specific VLAN:

```json
{
  "name": "iot-vlan",
  "destination": "192.168.50.0/24",
  "interface": "lan.50",
  "enabled": true,
  "comment": "Route to IoT VLAN"
}
```

### With WireGuard

Route traffic through VPN tunnel:

```json
{
  "name": "vpn-route",
  "destination": "10.0.8.0/24",
  "interface": "wg0",
  "enabled": true,
  "comment": "VPN peer networks"
}
```

### With Firewall

Routes and firewall zones work together:

```json
// routes.json
{
  "name": "guest-route",
  "destination": "192.168.100.0/24",
  "interface": "guest",
  "enabled": true
}
```

```json
// nftables.json
{
  "forwardings": [
    {
      "src": "guest",
      "dest": "wan",
      "comment": "Guests can access internet only"
    }
  ]
}
```

## Common Pitfalls

1. **Forgetting to specify gateway OR interface**: Both fields are optional, but you need at least one

2. **Gateway not on same subnet**: Gateway must be reachable on the specified interface

3. **Conflicting metrics**: Using same metric for backup routes won't fail over properly

4. **Missing return routes**: Remote end needs routes back to your network

5. **Firewall blocking**: Routes don't bypass firewall - ensure forwarding rules exist

## Best Practices

1. **Use descriptive names**: "backup-wan" is better than "route2"

2. **Always set comments**: Explain why this route exists

3. **Use metrics consistently**:
   - Primary: 10-99
   - Standard: 100
   - Backup: 200+

4. **Test before enabling**: Set enabled: false, test manually first

5. **Document dependencies**: Note which interfaces/gateways must exist

6. **Keep it simple**: Only add routes you actually need

7. **Monitor route health**: Use scripts to check gateway reachability

## Security Considerations

1. **Validate destinations**: Ensure routes don't expose internal networks

2. **Backup route security**: Ensure backup ISP has same firewall rules

3. **VPN routes**: Verify VPN is actually up before routing sensitive traffic

4. **Metric manipulation**: Be aware that misconfigured metrics can route traffic unexpectedly

5. **Gateway trust**: Only route through trusted gateways

## Performance Tips

1. **Minimize routes**: Fewer routes = faster lookups

2. **Use summarization**: Route 10.0.0.0/8 instead of many /24s

3. **Specific routes first**: More specific routes for critical traffic

4. **Monitor route cache**: Check `ip -s route show cache`

## Example Configurations

Complete examples are available in:
- [routes.json](routes.json) - Multi-WAN with VPN routing
- [interfaces-wireguard-server.json](interfaces-wireguard-server.json) - VPN interfaces
- [nftables.json](nftables.json) - Firewall configuration

## References

- [Linux Advanced Routing & Traffic Control](https://lartc.org/)
- [iproute2 documentation](https://wiki.linuxfoundation.org/networking/iproute2)
- [Understanding Linux Routing](https://www.kernel.org/doc/Documentation/networking/ip-sysctl.txt)

## Quick Reference

### Basic route
```json
{
  "name": "example",
  "destination": "10.0.0.0/8",
  "gateway": "192.168.1.254",
  "enabled": true
}
```

### Interface route
```json
{
  "name": "example",
  "destination": "172.16.0.0/12",
  "interface": "wg0",
  "enabled": true
}
```

### Default route with failover
```json
[
  {
    "name": "primary",
    "destination": "default",
    "gateway": "192.168.1.1",
    "metric": 100,
    "enabled": true
  },
  {
    "name": "backup",
    "destination": "default",
    "gateway": "192.168.2.1",
    "metric": 200,
    "enabled": true
  }
]
```

### High priority route
```json
{
  "name": "priority",
  "destination": "10.0.0.0/8",
  "gateway": "192.168.1.254",
  "metric": 10,
  "enabled": true
}
```
