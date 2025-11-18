// Copyright (C) 2025 Mono Technologies Inc.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// Package state provides configuration management for Jack.
package state

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/types"
)

// LoadInterfacesConfig loads the interfaces configuration from disk.
// If the file doesn't exist, it auto-detects the network interfaces and
// creates a sensible default configuration with WAN and LAN bridge.
func LoadInterfacesConfig() (*types.InterfacesConfig, error) {
	var config types.InterfacesConfig
	err := LoadConfig("interfaces", &config)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// First boot - generate default config
			logger.Info("No interfaces config found, auto-detecting network interfaces")
			defaultConfig, genErr := generateDefaultInterfacesConfig()
			if genErr != nil {
				return nil, fmt.Errorf("failed to generate default interfaces config: %w", genErr)
			}

			// Save the generated config to disk
			if saveErr := SaveConfig("interfaces", defaultConfig); saveErr != nil {
				logger.Warn("Failed to save auto-generated interfaces config",
					logger.Field{Key: "error", Value: saveErr.Error()})
			} else {
				logger.Info("Saved auto-generated interfaces configuration",
					logger.Field{Key: "path", Value: GetConfigDir() + "/interfaces.json"})
			}

			return defaultConfig, nil
		}
		return nil, err
	}

	// Initialize empty map if nil (defensive programming for malformed JSON)
	if config.Interfaces == nil {
		config.Interfaces = make(map[string]types.Interface)
	}

	return &config, nil
}

// generateDefaultInterfacesConfig auto-detects network interfaces and creates
// a sensible default configuration: one WAN interface and a LAN bridge.
func generateDefaultInterfacesConfig() (*types.InterfacesConfig, error) {
	// Detect WAN interface
	wanIface, err := detectWANInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to detect WAN interface: %w", err)
	}

	// Detect LAN interfaces
	lanIfaces, err := detectLANInterfaces(wanIface)
	if err != nil {
		return nil, fmt.Errorf("failed to detect LAN interfaces: %w", err)
	}

	config := &types.InterfacesConfig{
		Interfaces: make(map[string]types.Interface),
		Version:    "1.0",
	}

	// Add WAN interface
	if wanIface != nil {
		wanName := wanIface.Attrs().Name
		logger.Info("Detected WAN interface",
			logger.Field{Key: "interface", Value: wanName})

		// Try to preserve existing IP configuration
		wanInterface := types.Interface{
			Type:       "physical",
			Device:     wanName,
			DeviceName: wanName,
			Protocol:   "dhcp",
			Enabled:    true,
		}

		// Get existing IP addresses
		addrs, err := netlink.AddrList(wanIface, netlink.FAMILY_V4)
		if err == nil && len(addrs) > 0 {
			// Use static configuration with existing IP
			addr := addrs[0]
			ip, ipNet, parseErr := net.ParseCIDR(addr.IPNet.String())
			if parseErr == nil {
				wanInterface.Protocol = "static"
				wanInterface.IPAddr = ip.String()
				wanInterface.Netmask = net.IP(ipNet.Mask).String()
				logger.Info("Preserving existing WAN IP",
					logger.Field{Key: "ip", Value: ip.String()},
					logger.Field{Key: "netmask", Value: net.IP(ipNet.Mask).String()})
			}
		} else {
			logger.Info("No existing IP on WAN, defaulting to DHCP")
		}

		// Get existing default gateway
		routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
		if err == nil {
			for _, route := range routes {
				// Check for default route (0.0.0.0/0 or nil destination) that uses this interface
				isDefaultRoute := route.Dst == nil || (route.Dst != nil && route.Dst.String() == "0.0.0.0/0")
				if isDefaultRoute && route.Gw != nil && route.LinkIndex == wanIface.Attrs().Index {
					wanInterface.Gateway = route.Gw.String()
					logger.Info("Preserving existing WAN gateway",
						logger.Field{Key: "gateway", Value: route.Gw.String()})
					break
				}
			}
		} else {
			logger.Warn("Failed to list routes",
				logger.Field{Key: "error", Value: err.Error()})
		}

		config.Interfaces["wan"] = wanInterface
	}

	// Add LAN bridge if we have LAN interfaces
	if len(lanIfaces) > 0 {
		bridgePorts := make([]string, len(lanIfaces))
		for i, iface := range lanIfaces {
			bridgePorts[i] = iface.Attrs().Name
			logger.Info("Adding interface to LAN bridge",
				logger.Field{Key: "interface", Value: iface.Attrs().Name})
		}

		config.Interfaces["lan"] = types.Interface{
			Type:        "bridge",
			Device:      "br-lan",
			DeviceName:  "br-lan",
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
			BridgePorts: bridgePorts,
			Enabled:     true,
		}

		logger.Info("Created LAN bridge",
			logger.Field{Key: "bridge", Value: "br-lan"},
			logger.Field{Key: "port_count", Value: len(bridgePorts)})
	}

	if len(config.Interfaces) == 0 {
		return nil, fmt.Errorf("no network interfaces detected")
	}

	return config, nil
}

// detectWANInterface identifies the WAN interface by checking:
// 1. Interface with default route (0.0.0.0/0)
// 2. First physical interface that is UP and has an IP address
// 3. First available physical interface
func detectWANInterface() (netlink.Link, error) {
	// Strategy 1: Find interface with default route
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err == nil {
		for _, route := range routes {
			// Check for default route (0.0.0.0/0)
			if route.Dst == nil && route.LinkIndex > 0 {
				link, linkErr := netlink.LinkByIndex(route.LinkIndex)
				if linkErr == nil && !isLoopback(link) {
					logger.Info("Found WAN interface via default route",
						logger.Field{Key: "interface", Value: link.Attrs().Name})
					return link, nil
				}
			}
		}
	}

	// Strategy 2: First physical interface UP with IP
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var firstPhysical netlink.Link
	for _, link := range links {
		attrs := link.Attrs()

		// Skip loopback, bridge, and virtual interfaces
		if isLoopback(link) || isBridge(link) || isVirtual(link) {
			continue
		}

		// Remember first physical interface
		if firstPhysical == nil {
			firstPhysical = link
		}

		// Check if UP and has IP
		if attrs.Flags&net.FlagUp != 0 {
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err == nil && len(addrs) > 0 {
				logger.Info("Found WAN interface (UP with IP)",
					logger.Field{Key: "interface", Value: attrs.Name})
				return link, nil
			}
		}
	}

	// Strategy 3: Use first physical interface as fallback
	if firstPhysical != nil {
		logger.Info("Using first physical interface as WAN",
			logger.Field{Key: "interface", Value: firstPhysical.Attrs().Name})
		return firstPhysical, nil
	}

	return nil, fmt.Errorf("no suitable WAN interface found")
}

// detectLANInterfaces finds all physical interfaces that should be part of the LAN bridge.
// It excludes the WAN interface and any virtual/loopback interfaces.
func detectLANInterfaces(wanIface netlink.Link) ([]netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}

	var lanIfaces []netlink.Link
	wanName := ""
	if wanIface != nil {
		wanName = wanIface.Attrs().Name
	}

	for _, link := range links {
		attrs := link.Attrs()

		// Skip WAN interface
		if attrs.Name == wanName {
			continue
		}

		// Skip loopback, bridge, and virtual interfaces
		if isLoopback(link) || isBridge(link) || isVirtual(link) {
			continue
		}

		lanIfaces = append(lanIfaces, link)
	}

	return lanIfaces, nil
}

// isLoopback checks if an interface is a loopback interface
func isLoopback(link netlink.Link) bool {
	return link.Attrs().Name == "lo" || link.Attrs().Flags&net.FlagLoopback != 0
}

// isBridge checks if an interface is a bridge
func isBridge(link netlink.Link) bool {
	_, ok := link.(*netlink.Bridge)
	return ok
}

// isVirtual checks if an interface is virtual (veth, tun, tap, docker, wireguard, etc.)
func isVirtual(link netlink.Link) bool {
	name := link.Attrs().Name
	_, isVeth := link.(*netlink.Veth)
	_, isTuntap := link.(*netlink.Tuntap)
	_, isWireguard := link.(*netlink.Wireguard)

	// Check for virtual interface types
	if isVeth || isTuntap || isWireguard {
		return true
	}

	// Check for virtual interface name patterns
	virtualPrefixes := []string{
		"veth", "docker", "tun", "tap", // Container and VPN tunnels
		"wg", "wg-", // WireGuard interfaces
		"sit",     // IPv6-in-IPv4 tunnels
		"teql",    // Traffic equalizer
		"ip6tnl",  // IPv6 tunnels
		"gre",     // GRE tunnels
		"vlan",    // VLAN interfaces (should be configured separately)
		"macvlan", // MAC VLAN interfaces
		"vxlan",   // VXLAN interfaces
	}

	for _, prefix := range virtualPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
