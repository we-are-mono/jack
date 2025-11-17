# Jack VPN Configuration Guide

Jack supports VPN functionality through plugins. The WireGuard plugin is included as a core plugin and provides secure VPN tunnels.

## Quick Start

1. **Install WireGuard Tools**
   ```bash
   sudo apt install wireguard-tools
   ```

2. **Generate Keys**
   ```bash
   wg genkey | tee privatekey | wg pubkey > publickey
   ```

3. **Configure Provider**
   Edit `/etc/jack/jack.json`:
   ```json
   {
     "version": "1.0",
     "providers": {
       "vpn": "wireguard"
     }
   }
   ```

4. **Configure VPN**
   Create `/etc/jack/wireguard.json` (see examples below)

5. **Apply Configuration**
   ```bash
   jack vpn apply
   ```

## WireGuard VPN Server Example

Create `/etc/jack/wireguard.json`:

```json
{
  "version": "1.0",
  "interfaces": {
    "wg0": {
      "type": "wireguard",
      "enabled": true,
      "device_name": "wg0",
      "private_key": "SERVER_PRIVATE_KEY_HERE",
      "listen_port": 51820,
      "address": "10.0.0.1",
      "netmask": "255.255.255.0",
      "mtu": 1420,
      "firewall_zone": "vpn",
      "peers": [
        {
          "public_key": "CLIENT1_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.2/32"],
          "persistent_keepalive": 25,
          "comment": "Laptop"
        },
        {
          "public_key": "CLIENT2_PUBLIC_KEY",
          "allowed_ips": ["10.0.0.3/32"],
          "persistent_keepalive": 25,
          "comment": "Phone"
        }
      ],
      "comment": "Main VPN server"
    }
  }
}
```

## WireGuard Site-to-Site Example

Connect two networks through a VPN tunnel:

```json
{
  "version": "1.0",
  "interfaces": {
    "wg-site2": {
      "type": "wireguard",
      "enabled": true,
      "device_name": "wg-site2",
      "private_key": "SITE1_PRIVATE_KEY",
      "listen_port": 51821,
      "address": "10.1.0.1",
      "netmask": "255.255.255.252",
      "peers": [
        {
          "public_key": "SITE2_PUBLIC_KEY",
          "endpoint": "site2.example.com:51821",
          "allowed_ips": ["10.1.0.2/32", "192.168.2.0/24"],
          "persistent_keepalive": 25,
          "comment": "Remote Office"
        }
      ],
      "comment": "Site-to-site VPN"
    }
  }
}
```

## Configuration Fields

### VPNInterface

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Must be "wireguard" |
| `enabled` | bool | Yes | Enable/disable this interface |
| `device_name` | string | Yes | Interface name (e.g., "wg0") |
| `private_key` | string | Yes | WireGuard private key |
| `listen_port` | int | No | UDP port for incoming connections |
| `address` | string | Yes | VPN interface IP address |
| `netmask` | string | Yes | Interface netmask |
| `mtu` | int | No | Interface MTU (default: 1420) |
| `firewall_zone` | string | No | Firewall zone for this interface |
| `peers` | array | No | List of WireGuard peers |
| `comment` | string | No | Description |

### WireGuardPeer

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `public_key` | string | Yes | Peer's WireGuard public key |
| `preshared_key` | string | No | Optional preshared key for extra security |
| `endpoint` | string | No | Peer's address:port (for clients, leave empty on server) |
| `allowed_ips` | array | Yes | CIDR ranges allowed from this peer |
| `persistent_keepalive` | int | No | Keepalive interval in seconds (25 recommended for NAT) |
| `comment` | string | No | Description of this peer |

## Key Generation

Generate a new WireGuard keypair using standard WireGuard tools:

```bash
# Generate private key and derive public key
$ wg genkey | tee privatekey | wg pubkey > publickey

# View the keys
$ cat privatekey
uJt6KQfKxxx...

$ cat publickey
g8VZw5xxx...
```

**IMPORTANT**: Keep your private key secret! Add it to your interface config (`private_key`) and share the public key with your peers.

For enhanced security with preshared keys:
```bash
$ wg genpsk > presharedkey
```

## Firewall Integration

To allow VPN traffic through the firewall:

```json
{
  "zones": {
    "vpn": {
      "name": "vpn",
      "interfaces": ["wg0"],
      "input": "accept",
      "forward": "accept",
      "output": "accept",
      "masquerade": false,
      "comment": "VPN zone"
    }
  },
  "forwardings": [
    {
      "src": "vpn",
      "dest": "lan",
      "comment": "Allow VPN to LAN"
    },
    {
      "src": "lan",
      "dest": "vpn",
      "comment": "Allow LAN to VPN"
    }
  ],
  "rules": [
    {
      "name": "allow-wireguard",
      "src": "wan",
      "dest_port": "51820",
      "proto": "udp",
      "target": "ACCEPT",
      "comment": "Allow WireGuard VPN"
    }
  ]
}
```

## Client Configuration

For WireGuard clients connecting to your Jack VPN server:

```ini
[Interface]
PrivateKey = CLIENT_PRIVATE_KEY
Address = 10.0.0.2/32
DNS = 10.0.0.1

[Peer]
PublicKey = SERVER_PUBLIC_KEY
Endpoint = your-server.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
```

Save as `client.conf` and import to WireGuard client app.

## Common Commands

### Jack VPN Management
```bash
# Validate VPN configuration
jack vpn validate

# Apply VPN configuration
jack vpn apply

# Show VPN status
jack vpn status

# Enable VPN tunnels
jack vpn enable

# Disable VPN tunnels
jack vpn disable

# Remove all VPN configuration
jack vpn flush

# Check which plugin provides VPN
jack plugin status | grep vpn
```

### WireGuard Tools
```bash
# Generate keypair
wg genkey | tee privatekey | wg pubkey > publickey

# Generate preshared key
wg genpsk > presharedkey

# Show current WireGuard status
sudo wg show

# Show specific interface
sudo wg show wg0
```

## Troubleshooting

### Check if WireGuard tools are installed
```bash
which wg
wg version
```

### View active WireGuard interfaces
```bash
sudo wg show
```

### Check interface status
```bash
ip link show wg0
ip addr show wg0
```

### Test connectivity
```bash
# From VPN client
ping 10.0.0.1

# Check routing
ip route
```

### Common Issues

**Issue**: Plugin fails to start
- **Solution**: Install wireguard-tools: `sudo apt install wireguard-tools`

**Issue**: Interface fails to create
- **Solution**: Ensure WireGuard kernel module is loaded: `sudo modprobe wireguard`

**Issue**: No connectivity
- **Solution**: Check firewall rules allow UDP port 51820 (or your custom port)

**Issue**: Peers can't connect
- **Solution**:
  - Verify `allowed_ips` includes the peer's VPN IP
  - Check `endpoint` is reachable from peer
  - Ensure NAT/firewall forwards UDP port to server

## Security Best Practices

1. **Use Strong Keys**: Always generate keys with `wg genkey` (or use `wg genpsk` for preshared keys)
2. **Preshared Keys**: Add preshared keys with `wg genpsk` for post-quantum security
3. **Firewall Zones**: Isolate VPN traffic in dedicated firewall zone
4. **Allowed IPs**: Restrict `allowed_ips` to only necessary ranges
5. **Port**: Consider using a non-standard port to reduce noise
6. **Monitoring**: Enable logging and monitor VPN connections
7. **Key Rotation**: Regularly rotate keys for long-term deployments

## Performance Tuning

### Optimize MTU
```json
{
  "mtu": 1420
}
```
Standard WireGuard MTU. Adjust based on your network:
- PPPoE: 1412
- Standard Ethernet: 1420
- Jumbo frames: 1500+

### Multiple Interfaces
Run multiple WireGuard interfaces for different purposes:
```json
{
  "interfaces": {
    "wg-clients": {
      "device_name": "wg0",
      "listen_port": 51820,
      "comment": "Client VPN"
    },
    "wg-sites": {
      "device_name": "wg1",
      "listen_port": 51821,
      "comment": "Site-to-site"
    }
  }
}
```

## Related Documentation

- [Port Forwarding Guide](port-forwarding.md) - Configure firewall for VPN
- [Static Routes Guide](static-routes.md) - Add routes for VPN networks
- [Plugin Providers Guide](plugin-providers.md) - Plugin provider system
- [Configuration Reference](../configuration.md) - Complete configuration guide
