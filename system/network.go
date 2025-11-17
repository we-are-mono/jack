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

// Package system provides low-level system integration for network, firewall, DHCP, and routing.
package system

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// EnableIPForwarding enables kernel IP forwarding
func EnableIPForwarding() error {
	fmt.Println("Enabling IP forwarding...")

	// Enable IPv4 forwarding
	if err := setSysctl("net.ipv4.ip_forward", "1"); err != nil {
		return err
	}

	fmt.Println("  [OK] IP forwarding enabled")
	return nil
}

func setSysctl(key, value string) error {
	// Write to /proc/sys
	path := "/proc/sys/" + strings.Replace(key, ".", "/", -1)
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		return fmt.Errorf("failed to set %s: %w", key, err)
	}
	return nil
}

func ApplyInterfaceConfig(name string, iface types.Interface) error {
	if !iface.Enabled {
		fmt.Printf("  Disabling %s\n", name)
		// Bring down the interface
		deviceName := iface.Device
		if deviceName == "" {
			deviceName = iface.DeviceName // For VLANs
		}
		if deviceName == "" {
			deviceName = name // Fallback to interface name
		}

		link, err := netlink.LinkByName(deviceName)
		if err != nil {
			// Interface doesn't exist, nothing to disable
			return nil
		}

		if err := netlink.LinkSetDown(link); err != nil {
			return fmt.Errorf("failed to bring down %s: %w", deviceName, err)
		}
		fmt.Printf("    [OK] Interface %s brought down\n", deviceName)
		return nil
	}

	switch iface.Type {
	case "physical":
		return applyPhysicalInterface(name, iface)
	case "bridge":
		return applyBridgeInterface(name, iface)
	case "vlan":
		return applyVLANInterface(name, iface)
	default:
		return fmt.Errorf("unsupported interface type: %s", iface.Type)
	}
}

// applyPhysicalInterface configures a physical interface
func applyPhysicalInterface(name string, iface types.Interface) error {
	fmt.Printf("  Configuring physical interface %s -> %s\n", name, iface.Device)

	// Set default MTU if not specified
	if iface.MTU == 0 {
		iface.MTU = 1500
	}

	// Get the link by device name
	link, err := netlink.LinkByName(iface.Device)
	if err != nil {
		return fmt.Errorf("failed to find interface %s: %w", iface.Device, err)
	}

	// Set MTU if different from current
	if link.Attrs().MTU != iface.MTU {
		if err := netlink.LinkSetMTU(link, iface.MTU); err != nil {
			return fmt.Errorf("failed to set MTU: %w", err)
		}
		fmt.Printf("    [OK] Set MTU to %d\n", iface.MTU)
	}

	// Bring interface up first
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	fmt.Printf("    [OK] Interface %s is up\n", iface.Device)

	// Flush all existing IPs (Jack is source of truth)
	if err := flushIPs(link); err != nil {
		return fmt.Errorf("failed to flush IPs: %w", err)
	}

	// Apply configuration based on protocol
	switch iface.Protocol {
	case "static":
		if err := setStaticIP(link, iface.IPAddr, iface.Netmask); err != nil {
			return err
		}
		// Set gateway for static interfaces
		if iface.Gateway != "" {
			metric := iface.Metric
			if metric == 0 {
				metric = 100 // Default metric
			}
			if err := setDefaultGateway(link, iface.Gateway, metric); err != nil {
				return err
			}
		}
	case "dhcp":
		if err := startDHCPClient(iface.Device); err != nil {
			return err
		}
		// DHCP client will set gateway automatically
	case "none", "":
		// No IP configuration
		if iface.Protocol == "" {
			fmt.Printf("    No IP configuration (protocol not specified)\n")
		} else {
			fmt.Printf("    No IP configuration (protocol: none)\n")
		}
	default:
		return fmt.Errorf("unknown protocol: %s", iface.Protocol)
	}

	return nil
}

// getBridgePorts returns the list of interfaces attached to a bridge
func getBridgePorts(bridgeName string) ([]string, error) {
	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge: %w", err)
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}

	var ports []string
	bridgeIndex := bridge.Attrs().Index
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			ports = append(ports, link.Attrs().Name)
		}
	}

	return ports, nil
}

// stringSlicesEqual checks if two string slices contain the same elements (order-independent)
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create map for quick lookup
	aMap := make(map[string]bool)
	for _, item := range a {
		aMap[item] = true
	}

	for _, item := range b {
		if !aMap[item] {
			return false
		}
	}

	return true
}

// checkIPMatches checks if the interface has the correct IP configured
func checkIPMatches(link netlink.Link, ipAddr, netmask string) (bool, error) {
	if ipAddr == "" {
		return true, nil // No IP desired, so it matches
	}

	// Parse desired IP and mask
	desiredIP := net.ParseIP(ipAddr)
	if desiredIP == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	desiredMask := net.IPMask(net.ParseIP(netmask).To4())

	// Get current addresses
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return false, fmt.Errorf("failed to list addresses: %w", err)
	}

	// Check if the desired IP exists (allow multiple IPs, just check if ours is there)
	for _, addr := range addrs {
		if addr.IP.Equal(desiredIP) && addr.Mask.String() == desiredMask.String() {
			return true, nil // Found our IP with correct mask
		}
	}

	return false, nil // Desired IP not found
}

// bridgeConfigMatches checks if the current bridge matches the desired configuration
func bridgeConfigMatches(bridge netlink.Link, desired types.Interface) (bool, error) {
	// Check MTU
	if bridge.Attrs().MTU != desired.MTU {
		return false, nil
	}

	// Check current ports vs desired ports
	currentPorts, err := getBridgePorts(bridge.Attrs().Name)
	if err != nil {
		return false, err
	}

	if !stringSlicesEqual(currentPorts, desired.BridgePorts) {
		return false, nil
	}

	return true, nil
}

func applyBridgeInterface(name string, iface types.Interface) error {
	// Use Device field for bridge name (DeviceName is for VLANs)
	bridgeDevice := iface.Device
	if bridgeDevice == "" {
		bridgeDevice = name // Fallback to interface name if Device not set
	}

	fmt.Printf("  Configuring bridge %s -> %s\n", name, bridgeDevice)

	// Set default MTU if not specified (0 means not set)
	if iface.MTU == 0 {
		iface.MTU = 1500 // Standard Ethernet MTU
	}

	// Check if bridge already exists
	existingBridge, err := netlink.LinkByName(bridgeDevice)
	bridgeExists := (err == nil)

	var bridge netlink.Link

	if bridgeExists {
		// Bridge exists - check if configuration matches
		configMatches, configErr := bridgeConfigMatches(existingBridge, iface)
		if configErr != nil {
			return fmt.Errorf("failed to check bridge config: %w", configErr)
		}

		if configMatches {
			// Bridge and ports match - check if IP also matches before declaring success
			if iface.Protocol == "static" {
				ipMatches, ipErr := checkIPMatches(existingBridge, iface.IPAddr, iface.Netmask)
				if ipErr != nil {
					return fmt.Errorf("failed to check IP matches: %w", ipErr)
				}
				if ipMatches {
					fmt.Printf("    [OK] Bridge %s already configured correctly\n", bridgeDevice)
					return nil // Early return - nothing to do
				}
			} else {
				// Non-static protocol - bridge match is enough
				fmt.Printf("    [OK] Bridge %s already configured correctly\n", bridgeDevice)
				return nil
			}

			// IP needs updating but bridge is fine
			bridge = existingBridge
		} else {
			// Check if MTU changed - if so, we need to recreate the bridge
			// (can't change MTU with ports attached)
			mtuChanged := existingBridge.Attrs().MTU != iface.MTU

			if mtuChanged {
				fmt.Printf("    [INFO] Bridge MTU changed (%d -> %d), recreating bridge\n",
					existingBridge.Attrs().MTU, iface.MTU)
				// Delete bridge and fall through to creation
				if err := netlink.LinkDel(existingBridge); err != nil {
					return fmt.Errorf("failed to delete bridge for MTU change: %w", err)
				}
				bridgeExists = false
			} else {
				// Only port changes - can update in-place
				fmt.Printf("    Updating bridge %s ports\n", bridgeDevice)

				// Get current ports
				currentPorts, err := getBridgePorts(bridgeDevice)
				if err != nil {
					return fmt.Errorf("failed to get current bridge ports: %w", err)
				}

				// Calculate ports to add and remove
				desiredPortsMap := make(map[string]bool)
				for _, portName := range iface.BridgePorts {
					desiredPortsMap[portName] = true
				}

				currentPortsMap := make(map[string]bool)
				for _, portName := range currentPorts {
					currentPortsMap[portName] = true
				}

				// Remove ports that shouldn't be there
				for _, portName := range currentPorts {
					if !desiredPortsMap[portName] {
						port, err := netlink.LinkByName(portName)
						if err != nil {
							return fmt.Errorf("failed to find port %s: %w", portName, err)
						}
						if err := netlink.LinkSetNoMaster(port); err != nil {
							return fmt.Errorf("failed to remove port %s from bridge: %w", portName, err)
						}
						fmt.Printf("    [OK] Removed port %s from bridge\n", portName)
					}
				}

				// Add ports that should be there
				for _, portName := range iface.BridgePorts {
					if !currentPortsMap[portName] {
						port, err := netlink.LinkByName(portName)
						if err != nil {
							return fmt.Errorf("failed to find port %s: %w", portName, err)
						}

						// Bring port up first
						if err := netlink.LinkSetUp(port); err != nil {
							return fmt.Errorf("failed to bring up port %s: %w", portName, err)
						}

						// Add port to bridge
						if err := netlink.LinkSetMaster(port, existingBridge); err != nil {
							return fmt.Errorf("failed to add port %s to bridge: %w", portName, err)
						}
						fmt.Printf("    [OK] Added port %s to bridge\n", portName)
					}
				}

				bridge = existingBridge
			}
		}
	}

	if !bridgeExists {
		// Bridge doesn't exist - create it
		newBridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: bridgeDevice,
				MTU:  iface.MTU,
			},
		}

		if err := netlink.LinkAdd(newBridge); err != nil {
			return fmt.Errorf("failed to create bridge: %w", err)
		}

		fmt.Printf("    [OK] Bridge %s created\n", bridgeDevice)

		// Get the bridge we just created
		var linkErr error
		bridge, linkErr = netlink.LinkByName(bridgeDevice)
		if linkErr != nil {
			return fmt.Errorf("failed to get newly created bridge: %w", linkErr)
		}

		// Add ports to bridge
		for _, portName := range iface.BridgePorts {
			port, err := netlink.LinkByName(portName)
			if err != nil {
				return fmt.Errorf("failed to find port %s: %w", portName, err)
			}

			// Bring port up first
			if err := netlink.LinkSetUp(port); err != nil {
				return fmt.Errorf("failed to bring up port %s: %w", portName, err)
			}

			// Add port to bridge
			if err := netlink.LinkSetMaster(port, bridge); err != nil {
				return fmt.Errorf("failed to add port %s to bridge: %w", portName, err)
			}

			fmt.Printf("    [OK] Added port %s to bridge\n", portName)
		}

		// Bring bridge up
		if err := netlink.LinkSetUp(bridge); err != nil {
			return fmt.Errorf("failed to bring up bridge: %w", err)
		}

		fmt.Printf("    [OK] Bridge %s is up\n", bridgeDevice)
	}

	// Configure STP if requested
	if iface.BridgeSTP {
		// Enable STP on bridge
		fmt.Printf("    [INFO] STP configuration not yet implemented\n")
	}

	// Apply IP configuration based on protocol (idempotent)
	switch iface.Protocol {
	case "static":
		if err := ensureStaticIP(bridge, iface.IPAddr, iface.Netmask); err != nil {
			return err
		}
		// Set gateway for static bridges
		if iface.Gateway != "" {
			metric := iface.Metric
			if metric == 0 {
				metric = 100
			}
			if err := setDefaultGateway(bridge, iface.Gateway, metric); err != nil {
				return err
			}
		}
	case "dhcp":
		if err := startDHCPClient(bridgeDevice); err != nil {
			return err
		}
	case "none", "":
		// No IP configuration
		if iface.Protocol == "" {
			fmt.Printf("    No IP configuration (protocol not specified)\n")
		} else {
			fmt.Printf("    No IP configuration (protocol: none)\n")
		}
	default:
		return fmt.Errorf("unknown protocol: %s", iface.Protocol)
	}

	return nil
}

// applyVLANInterface configures a VLAN interface
func applyVLANInterface(name string, iface types.Interface) error {
	fmt.Printf("  Configuring VLAN interface %s -> %s (VLAN ID: %d)\n", name, iface.DeviceName, iface.VLANId)

	// Set default MTU if not specified
	if iface.MTU == 0 {
		iface.MTU = 1500
	}

	if iface.Device == "" {
		return fmt.Errorf("parent device not specified for VLAN")
	}

	if iface.VLANId == 0 {
		return fmt.Errorf("VLAN ID not specified")
	}

	if iface.DeviceName == "" {
		return fmt.Errorf("device_name not specified for VLAN interface")
	}

	// Get parent interface
	parent, err := netlink.LinkByName(iface.Device)
	if err != nil {
		return fmt.Errorf("failed to find parent interface %s: %w", iface.Device, err)
	}

	// Ensure parent is up
	if err = netlink.LinkSetUp(parent); err != nil {
		return fmt.Errorf("failed to bring up parent interface: %w", err)
	}

	// Check if VLAN interface already exists
	var existingLink netlink.Link
	existingLink, err = netlink.LinkByName(iface.DeviceName)
	if err == nil {
		// VLAN exists, delete it first to ensure clean state
		fmt.Printf("    Removing existing VLAN interface %s\n", iface.DeviceName)
		if err = netlink.LinkDel(existingLink); err != nil {
			return fmt.Errorf("failed to delete existing VLAN interface: %w", err)
		}
	}

	// Create VLAN interface
	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        iface.DeviceName,
			ParentIndex: parent.Attrs().Index,
			MTU:         iface.MTU,
		},
		VlanId: iface.VLANId,
	}

	if err = netlink.LinkAdd(vlan); err != nil {
		return fmt.Errorf("failed to create VLAN interface: %w", err)
	}

	fmt.Printf("    [OK] VLAN interface %s created\n", iface.DeviceName)

	// Get the VLAN link we just created
	vlanLink, err := netlink.LinkByName(iface.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to get VLAN interface: %w", err)
	}

	// Bring VLAN interface up
	if err := netlink.LinkSetUp(vlanLink); err != nil {
		return fmt.Errorf("failed to bring up VLAN interface: %w", err)
	}

	fmt.Printf("    [OK] VLAN interface %s is up\n", iface.DeviceName)

	// Flush any existing IPs from VLAN interface
	if err := flushIPs(vlanLink); err != nil {
		return fmt.Errorf("failed to flush IPs: %w", err)
	}

	// Apply IP configuration based on protocol
	switch iface.Protocol {
	case "static":
		if err := setStaticIP(vlanLink, iface.IPAddr, iface.Netmask); err != nil {
			return err
		}
		// Set gateway for static VLAN interfaces if needed
		if iface.Gateway != "" {
			metric := iface.Metric
			if metric == 0 {
				metric = 100
			}
			if err := setDefaultGateway(vlanLink, iface.Gateway, metric); err != nil {
				return err
			}
		}
	case "dhcp":
		if err := startDHCPClient(iface.DeviceName); err != nil {
			return err
		}
	case "none", "":
		// No IP configuration
		if iface.Protocol == "" {
			fmt.Printf("    No IP configuration (protocol not specified)\n")
		} else {
			fmt.Printf("    No IP configuration (protocol: none)\n")
		}
	default:
		return fmt.Errorf("unknown protocol: %s", iface.Protocol)
	}

	return nil
}

func startDHCPClient(deviceName string) error {
	fmt.Printf("    Starting DHCP client on %s\n", deviceName)

	if _, err := exec.LookPath("dhclient"); err != nil {
		return fmt.Errorf("dhclient not found: %w", err)
	}

	// Kill any existing dhclient for this interface (ignore errors if no process exists)
	_ = exec.Command("pkill", "-f", fmt.Sprintf("dhclient.*%s", deviceName)).Run() //nolint:errcheck,gosec // deviceName from config, admin-controlled

	cmd := exec.Command("dhclient", "-v", deviceName)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dhclient: %w", err)
	}

	fmt.Printf("    [OK] DHCP client started\n")
	return nil
}

func flushIPs(link netlink.Link) error {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		fmt.Printf("    Removing existing IP: %s\n", addr.IPNet.String())
		if err := netlink.AddrDel(link, &addr); err != nil {
			return fmt.Errorf("failed to remove IP: %w", err)
		}
	}

	return nil
}

func setStaticIP(link netlink.Link, ipAddr, netmask string) error {
	if ipAddr == "" {
		return nil // No IP to set
	}

	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	mask := net.IPMask(net.ParseIP(netmask).To4())
	prefixLen, _ := mask.Size()

	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: mask,
		},
	}

	// Add address to interface
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %w", err)
	}

	fmt.Printf("    [OK] Assigned IP: %s/%d\n", ipAddr, prefixLen)
	return nil
}

// ensureStaticIP checks if the interface has the correct IP, only updates if needed (idempotent)
func ensureStaticIP(link netlink.Link, ipAddr, netmask string) error {
	if ipAddr == "" {
		return nil // No IP to set
	}

	// Parse desired IP and mask
	desiredIP := net.ParseIP(ipAddr)
	if desiredIP == nil {
		return fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	desiredMask := net.IPMask(net.ParseIP(netmask).To4())
	prefixLen, _ := desiredMask.Size()

	// Get current addresses
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

	// Check if desired IP already exists
	hasDesiredIP := false
	var unwantedAddrs []netlink.Addr

	for _, addr := range addrs {
		if addr.IP.Equal(desiredIP) && addr.Mask.String() == desiredMask.String() {
			hasDesiredIP = true
		} else {
			unwantedAddrs = append(unwantedAddrs, addr)
		}
	}

	// Remove unwanted IPs
	for _, addr := range unwantedAddrs {
		fmt.Printf("    Removing unwanted IP: %s\n", addr.IPNet.String())
		if err := netlink.AddrDel(link, &addr); err != nil {
			return fmt.Errorf("failed to remove IP: %w", err)
		}
	}

	// Add desired IP if not present
	if !hasDesiredIP {
		desiredAddr := &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   desiredIP,
				Mask: desiredMask,
			},
		}

		if err := netlink.AddrAdd(link, desiredAddr); err != nil {
			return fmt.Errorf("failed to add IP address: %w", err)
		}

		fmt.Printf("    [OK] Assigned IP: %s/%d\n", ipAddr, prefixLen)
	} else if len(unwantedAddrs) == 0 {
		fmt.Printf("    [OK] IP %s/%d already configured\n", ipAddr, prefixLen)
	}

	return nil
}

func setDefaultGateway(link netlink.Link, gateway string, metric int) error {
	if gateway == "" {
		return nil // No gateway to set
	}

	fmt.Printf("    Setting default gateway: %s\n", gateway)

	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("invalid gateway address: %s", gateway)
	}

	// Remove existing default routes first
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	for _, route := range routes {
		// Check if it's a default route (0.0.0.0/0)
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			fmt.Printf("    Removing old default route via %s\n", route.Gw)
			if err := netlink.RouteDel(&route); err != nil {
				// Ignore errors, route might already be gone
				continue
			}
		}
	}

	// Add new default route
	// Dst nil means default route (0.0.0.0/0)
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       nil, // nil = default route
		Gw:        gw,
		Priority:  metric,
		Table:     254, // Main routing table
	}

	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to add default route: %w", err)
	}

	fmt.Printf("    [OK] Default gateway set\n")
	return nil
}
