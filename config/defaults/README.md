# Jack Default Configurations

This directory contains the minimal, secure default configurations that are deployed to fresh Jack installations. These configs are designed to provide a safe starting point for new users.

## Purpose

- **Defaults** (`config/defaults/`): Minimal configs used for fresh installations
- **Examples** (`examples/config/`): Comprehensive configs showing all available features and options

## Security Considerations

### Firewall (firewall.json)
- **Enabled by default** to protect the system immediately
- SSH access allowed **only from LAN**, not WAN
- ICMP allowed from WAN for connectivity testing
- Established/related connections allowed
- All other WAN input dropped by default
- NAT/masquerading enabled on WAN zone
- LAN fully trusted with all traffic accepted

### Services (dhcp.json, dns.json, vpn.json)
- **Disabled by default** - users must explicitly enable them
- Prevents unintended service exposure on fresh installations
- Minimal configuration with safe defaults for when enabled

### Routes (routes.json)
- Empty by default
- Jack auto-detects and preserves existing network configuration
- Users can add custom static routes as needed

## Default Configuration Values

| Service | Enabled | Key Settings |
|---------|---------|-------------|
| Firewall | ✅ Yes | SSH from LAN only, WAN locked down |
| DHCP | ❌ No | Port 53, domain: lan |
| DNS | ❌ No | Port 53, upstreams: 1.1.1.1, 8.8.8.8 |
| VPN | ❌ No | No interfaces configured |
| Routes | ❌ No | No static routes |

## Deployment

These configs are copied to `/etc/jack/` during:
- Manual deployment via `./deploy.sh <device-ip> manual`
- Package installation via `./deploy.sh <device-ip> deb`

The deployment script (`deploy.sh`) uses these defaults to ensure consistent, secure initial state across all fresh installations.

## Modifying Defaults

When modifying these files:
1. Keep configs minimal - only essential fields
2. Maintain secure-by-default posture (services disabled, firewall restrictive)
3. Never expose management interfaces (SSH) to WAN by default
4. Document any changes to security posture in this README
5. Test on a fresh installation to verify safety

## See Also

- `examples/config/` - Comprehensive example configs with all features
- `examples/jack.json` - Main Jack configuration example
