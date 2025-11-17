# Jack Commands Reference

This document provides a complete inventory of all available Jack commands and their variations.

## Core Configuration Commands

### apply
```
jack apply
```

### commit
```
jack commit
```

### diff
```
jack diff
```

### get
```
jack get <path...>
jack get interfaces
jack get interfaces eth0
jack get interfaces eth0 ipaddr
jack get interfaces eth0 netmask
jack get interfaces eth0 gateway
jack get interfaces eth0 mtu
jack get interfaces eth0 enabled
jack get interfaces eth0 protocol
jack get interfaces eth0 type
jack get interfaces eth0 device
jack get interfaces eth0 device_name
jack get interfaces eth0 comment
jack get routes
jack get routes default-via-vpn
jack get routes default-via-vpn destination
jack get routes default-via-vpn gateway
jack get routes default-via-vpn interface
jack get routes default-via-vpn metric
jack get routes default-via-vpn table
jack get routes default-via-vpn enabled
jack get routes default-via-vpn comment
jack get firewall
jack get firewall enabled
jack get firewall defaults
jack get firewall defaults input
jack get firewall defaults forward
jack get firewall defaults output
jack get firewall zones
jack get firewall zones wan
jack get firewall zones wan interfaces
jack get firewall zones wan input
jack get firewall zones wan forward
jack get firewall zones wan masquerade
jack get firewall forwardings
jack get firewall rules
jack get firewall port_forwards
jack get dhcp
jack get dhcp enabled
jack get dhcp interfaces
jack get vpn
jack get vpn enabled
jack get vpn interfaces
jack get monitoring
jack get monitoring enabled
jack get monitoring collection_interval
jack get led
jack get led status:green
jack get led status:green brightness
jack get led status:green trigger
jack get led status:blue
jack get led status:blue brightness
jack get led status:red
jack get led status:white
```

### revert
```
jack revert
```

### set
```
jack set <path...> <value>
jack set interfaces eth0 ipaddr 10.0.0.1
jack set interfaces eth0 netmask 255.255.255.0
jack set interfaces eth0 gateway 10.0.0.254
jack set interfaces eth0 mtu 1500
jack set interfaces eth0 enabled true
jack set interfaces eth0 protocol static
jack set interfaces eth0 comment "Primary interface"
jack set routes default-via-vpn destination 0.0.0.0/0
jack set routes default-via-vpn gateway 10.8.0.1
jack set routes default-via-vpn interface wg-proton
jack set routes default-via-vpn metric 100
jack set routes default-via-vpn table 254
jack set routes default-via-vpn enabled true
jack set routes default-via-vpn comment "Default route via VPN"
jack set firewall enabled true
jack set firewall defaults input DROP
jack set firewall defaults forward DROP
jack set firewall defaults output ACCEPT
jack set firewall zones wan interfaces '["eth0"]'
jack set firewall zones wan masquerade true
jack set dhcp enabled true
jack set vpn enabled true
jack set monitoring enabled true
jack set monitoring collection_interval 5
jack set led status:green brightness 255
jack set led status:green trigger none
jack set led status:green trigger timer
jack set led status:green trigger heartbeat
jack set led status:green trigger netdev
```

### status
```
jack status
```

### validate
```
jack validate
```

## Checkpoint and Rollback Commands

### checkpoint
```
jack checkpoint list
jack checkpoint create
```

### rollback
```
jack rollback
jack rollback <checkpoint-id>
jack rollback auto-1234567890
jack rollback manual-1234567890
```

## Plugin Management Commands

### plugin
```
jack plugin list
jack plugin status
jack plugin enable <plugin>
jack plugin enable nftables
jack plugin enable dnsmasq
jack plugin enable wireguard
jack plugin enable monitoring
jack plugin enable leds
jack plugin disable <plugin>
jack plugin disable nftables
jack plugin disable dnsmasq
jack plugin disable wireguard
jack plugin disable monitoring
jack plugin disable leds
jack plugin rescan
jack plugin info <plugin>
jack plugin info nftables
jack plugin info dnsmasq
jack plugin info wireguard
jack plugin info monitoring
jack plugin info leds
```

## Daemon Commands

### daemon
```
jack daemon
```

### logs
```
jack logs
```

## Plugin-Provided Commands

### led (leds plugin)
```
jack led status
jack led list
```

### monitor (monitoring plugin)
```
jack monitor stats
jack monitor bandwidth
jack monitor bandwidth <interface>
jack monitor bandwidth br-lan
jack monitor bandwidth wg-proton
jack monitor bandwidth eth0
```

## Global Options

```
jack --version
jack -v
jack --help
jack -h
```

## Notes

- **Paths use space-separated arguments**: `jack get interfaces eth0 ipaddr`
- Path components are joined internally with dots but specified as separate arguments on the command line
- Interface paths: `jack get interfaces <interface-name> <field>`
- Route paths: `jack get routes <route-name> <field>`
- Plugin namespaces:
  - `firewall` (nftables plugin)
  - `dhcp` (dnsmasq plugin)
  - `vpn` (wireguard plugin)
  - `monitoring` (monitoring plugin)
  - `led` (leds plugin)
- **LED names contain colons**: Linux LED names like `status:green` include the colon as part of the name itself (from `/sys/class/leds/status:green`)
- Values can be strings, numbers, booleans, or JSON arrays/objects (use quotes for strings with spaces)
- Continuous commands (`monitor stats`, `monitor bandwidth`) refresh automatically until Ctrl+C
- Legacy dot notation (`jack get interfaces.eth0.ipaddr`) is still supported for backward compatibility
