# Port Forwarding Guide

Port forwarding (DNAT - Destination NAT) allows you to expose services running on your internal network to the internet by forwarding incoming traffic from your WAN to specific internal IP addresses.

## How Port Forwarding Works

```
Internet                Gateway (Jack)              Internal Network
   |                         |                             |
   |  WAN: 10.0.0.199        |      LAN: 192.168.1.0/24   |
   |                         |                             |
   | HTTP request            |                             |
   | dst: 10.0.0.199:80  --->| DNAT rule translates to --> | 192.168.1.10:80
   |                         | dst: 192.168.1.10:80        |
   |                         |                             |
   |                         | Forward chain allows   ---> | Web Server
   |                         | traffic through             | 192.168.1.10
   | <------------------------| Response comes back    <--- |
   | src: 10.0.0.199:80      | (SNAT back to WAN IP)       |
```

## Configuration Structure

Port forwards are defined in the `port_forwards` section of [nftables.json](nftables.json):

```json
{
  "port_forwards": [
    {
      "name": "HTTP-Server",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "80",
      "dest_ip": "192.168.1.10",
      "dest_port": "80",
      "target": "DNAT",
      "enabled": true,
      "comment": "Forward HTTP to internal web server"
    }
  ]
}
```

### Fields Explained

- **name**: Unique identifier for this port forward
- **src**: Source zone (typically "wan")
- **dest**: Destination zone (typically "lan")
- **proto**: Protocol (`tcp`, `udp`, or `tcp,udp`)
- **src_dport**: External port (what port clients connect to on your WAN IP)
- **dest_ip**: Internal IP address to forward to
- **dest_port**: Internal port (can be different from src_dport for port translation)
- **target**: Always "DNAT" for port forwarding
- **enabled**: Set to `false` to disable without deleting the rule
- **comment**: Description of what this forward does

## Common Use Cases

### 1. Web Server (HTTP/HTTPS)

Expose a web server running on an internal machine:

```json
{
  "port_forwards": [
    {
      "name": "HTTP-Server",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "80",
      "dest_ip": "192.168.1.10",
      "dest_port": "80",
      "target": "DNAT",
      "enabled": true,
      "comment": "Web server HTTP"
    },
    {
      "name": "HTTPS-Server",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "443",
      "dest_ip": "192.168.1.10",
      "dest_port": "443",
      "target": "DNAT",
      "enabled": true,
      "comment": "Web server HTTPS"
    }
  ]
}
```

### 2. SSH with Port Translation

Forward external port 2222 to internal SSH on port 22 (more secure than exposing port 22):

```json
{
  "name": "SSH-Server",
  "src": "wan",
  "dest": "lan",
  "proto": "tcp",
  "src_dport": "2222",
  "dest_ip": "192.168.1.10",
  "dest_port": "22",
  "target": "DNAT",
  "enabled": true,
  "comment": "SSH with non-standard external port"
}
```

Access from outside: `ssh -p 2222 user@your-wan-ip`

### 3. Game Server

Forward game server ports:

```json
{
  "name": "Minecraft-Server",
  "src": "wan",
  "dest": "lan",
  "proto": "tcp",
  "src_dport": "25565",
  "dest_ip": "192.168.1.20",
  "dest_port": "25565",
  "target": "DNAT",
  "enabled": true,
  "comment": "Minecraft server"
}
```

### 4. Multiple Servers Same Port

If you have multiple internal servers on different IPs but want to expose them on different external ports:

```json
{
  "port_forwards": [
    {
      "name": "Web-Server-1",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "8001",
      "dest_ip": "192.168.1.10",
      "dest_port": "80",
      "target": "DNAT",
      "enabled": true,
      "comment": "First web server on port 8001"
    },
    {
      "name": "Web-Server-2",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "8002",
      "dest_ip": "192.168.1.11",
      "dest_port": "80",
      "target": "DNAT",
      "enabled": true,
      "comment": "Second web server on port 8002"
    }
  ]
}
```

### 5. Security Camera / NVR

```json
{
  "name": "RTSP-Camera",
  "src": "wan",
  "dest": "lan",
  "proto": "tcp",
  "src_dport": "554",
  "dest_ip": "192.168.1.50",
  "dest_port": "554",
  "target": "DNAT",
  "enabled": true,
  "comment": "RTSP camera stream"
}
```

## Port Forwarding with VLANs

You can forward ports to devices on specific VLANs:

```json
{
  "name": "IoT-Camera",
  "src": "wan",
  "dest": "iot",
  "proto": "tcp",
  "src_dport": "8554",
  "dest_ip": "192.168.20.10",
  "dest_port": "554",
  "target": "DNAT",
  "enabled": true,
  "comment": "Camera on IoT VLAN"
}
```

Make sure you have appropriate firewall forwarding rules to allow traffic from WAN to the destination zone.

## How Jack Implements Port Forwarding

When you apply a port forward, Jack creates two nftables rules:

### 1. DNAT Rule (prerouting chain)
Translates the destination address of incoming packets:
```
nft add rule inet jack prerouting meta l4proto tcp th dport 80 counter dnat to 192.168.1.10
```

### 2. Forward Allow Rule (forward chain)
Permits the translated traffic through the firewall:
```
nft add rule inet jack forward ip daddr 192.168.1.10 meta l4proto tcp th dport 80 counter accept
```

## Security Considerations

1. **Least Privilege**: Only forward ports you actually need
2. **Non-Standard Ports**: Use port translation to hide services on non-standard ports (e.g., SSH on 2222 instead of 22)
3. **Disable Unused Forwards**: Set `enabled: false` instead of deleting rules you might need later
4. **Internal Firewall**: Consider running a firewall on the destination server too
5. **VPN Alternative**: For administrative access, consider WireGuard VPN instead of port forwarding
6. **Monitor Logs**: Check firewall logs for suspicious connection attempts

## Troubleshooting

### Port forward not working

1. **Check firewall is applied**:
   ```bash
   jack status -v
   # Look for: [UP] Firewall: Active
   ```

2. **Verify nftables rules**:
   ```bash
   nft list table inet jack | grep -A 5 prerouting
   nft list table inet jack | grep -A 5 forward
   ```

3. **Check destination server is listening**:
   ```bash
   # On the internal server
   netstat -tuln | grep :80
   # Or
   ss -tuln | grep :80
   ```

4. **Test from inside the network first**:
   ```bash
   curl http://192.168.1.10:80
   ```

5. **Test external connectivity**:
   ```bash
   # From an external machine
   curl http://YOUR-WAN-IP:80
   ```

6. **Check NAT masquerading is enabled**:
   Your WAN zone must have `"masquerade": true` for return traffic to work.

### Connection refused

- Destination server is not running/listening
- Destination server firewall is blocking the connection
- Wrong destination IP or port

### Connection timeout

- Firewall forward rule missing or blocking
- Masquerading not enabled on WAN zone
- Routing issue on destination server (check default gateway)

### Works internally but not externally

- ISP might be blocking the port
- Need to test from truly external network (not same ISP)
- Check if WAN interface has correct IP

## Testing Port Forwards

### From External Network

```bash
# Test TCP port
nc -zv YOUR-WAN-IP 80

# Test with curl (for HTTP)
curl -v http://YOUR-WAN-IP:80

# Test with telnet
telnet YOUR-WAN-IP 80
```

### From LAN (hairpin NAT)

Note: Hairpin NAT (accessing your WAN IP from LAN) is not currently implemented in Jack. Test port forwards from an external network or use the internal IP from LAN.

## Deployment

1. **Edit firewall config**:
   ```bash
   vim /etc/jack/nftables.json
   # Add your port forward rules
   ```

2. **Apply configuration**:
   ```bash
   jack apply
   ```

3. **Verify rules were created**:
   ```bash
   jack status -v
   nft list table inet jack
   ```

4. **Test from external network**:
   ```bash
   # From another network
   curl http://YOUR-WAN-IP:PORT
   ```

## Example: Home Server Setup

Complete example for a home server with multiple services:

```json
{
  "port_forwards": [
    {
      "name": "SSH-Admin",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "2222",
      "dest_ip": "192.168.1.10",
      "dest_port": "22",
      "enabled": true,
      "comment": "SSH administration"
    },
    {
      "name": "HTTP",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "80",
      "dest_ip": "192.168.1.10",
      "dest_port": "80",
      "enabled": true,
      "comment": "Web server HTTP"
    },
    {
      "name": "HTTPS",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "443",
      "dest_ip": "192.168.1.10",
      "dest_port": "443",
      "enabled": true,
      "comment": "Web server HTTPS"
    },
    {
      "name": "Plex",
      "src": "wan",
      "dest": "lan",
      "proto": "tcp",
      "src_dport": "32400",
      "dest_ip": "192.168.1.10",
      "dest_port": "32400",
      "enabled": true,
      "comment": "Plex media server"
    }
  ]
}
```

## Best Practices

1. **Use descriptive names**: Make it clear what each forward does
2. **Document everything**: Use comments field
3. **Test before enabling**: Set `enabled: false` while testing
4. **Review regularly**: Disable forwards you no longer need
5. **Use static IPs**: Assign static DHCP leases to servers that have port forwards
6. **Monitor**: Check logs for unusual connection attempts
7. **Consider alternatives**: VPN is more secure than forwarding management ports

## Related Documentation

- [VLAN Guide](vlans.md) - Forwarding to VLANs
- [Configuration Reference](../configuration.md) - Complete firewall reference
- [Static Routes](static-routes.md) - Advanced routing scenarios
- [firewall-portforward.json](firewall-portforward.json) - Example configuration
