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

package system

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// EnableIPForwarding enables kernel IP forwarding using the NetworkManager.
func (nm *NetworkManager) EnableIPForwarding() error {
	fmt.Println("Enabling IP forwarding...")

	// Enable IPv4 forwarding
	if err := nm.sysctl.Set("net.ipv4.ip_forward", "1"); err != nil {
		return fmt.Errorf("failed to set net.ipv4.ip_forward: %w", err)
	}

	fmt.Println("  [OK] IP forwarding enabled")
	return nil
}

// ApplyInterfaceConfig applies configuration to a network interface.
func (nm *NetworkManager) ApplyInterfaceConfig(name string, iface types.Interface) error {
	if !iface.Enabled {
		return nm.disableInterface(name, iface)
	}

	switch iface.Type {
	case "physical":
		return nm.applyPhysicalInterface(name, iface)
	case "bridge":
		return nm.applyBridgeInterface(name, iface)
	case "vlan":
		return nm.applyVLANInterface(name, iface)
	default:
		return fmt.Errorf("unsupported interface type: %s", iface.Type)
	}
}

func (nm *NetworkManager) disableInterface(name string, iface types.Interface) error {
	fmt.Printf("  Disabling %s\n", name)

	// Determine device name
	deviceName := iface.Device
	if deviceName == "" {
		deviceName = iface.DeviceName // For VLANs
	}
	if deviceName == "" {
		deviceName = name // Fallback to interface name
	}

	link, err := nm.netlink.LinkByName(deviceName)
	if err != nil {
		// Interface doesn't exist, nothing to disable
		return nil
	}

	if err := nm.netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to bring down %s: %w", deviceName, err)
	}
	fmt.Printf("    [OK] Interface %s brought down\n", deviceName)
	return nil
}

func (nm *NetworkManager) applyPhysicalInterface(name string, iface types.Interface) error {
	fmt.Printf("  Configuring physical interface %s -> %s\n", name, iface.Device)

	// Set default MTU if not specified
	if iface.MTU == 0 {
		iface.MTU = 1500
	}

	// Get the link by device name
	link, err := nm.netlink.LinkByName(iface.Device)
	if err != nil {
		return fmt.Errorf("failed to find interface %s: %w", iface.Device, err)
	}

	// Set MTU if different from current
	if link.Attrs().MTU != iface.MTU {
		if err := nm.netlink.LinkSetMTU(link, iface.MTU); err != nil {
			return fmt.Errorf("failed to set MTU: %w", err)
		}
		fmt.Printf("    [OK] Set MTU to %d\n", iface.MTU)
	}

	// Bring interface up first
	if err := nm.netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	fmt.Printf("    [OK] Interface %s is up\n", iface.Device)

	// Flush all existing IPs (Jack is source of truth)
	if err := nm.flushIPs(link); err != nil {
		return fmt.Errorf("failed to flush IPs: %w", err)
	}

	// Apply configuration based on protocol
	switch iface.Protocol {
	case "static":
		if err := nm.setStaticIP(link, iface.IPAddr, iface.Netmask); err != nil {
			return err
		}
		// Set gateway for static interfaces
		if iface.Gateway != "" {
			metric := iface.Metric
			if metric == 0 {
				metric = 100 // Default metric
			}
			if err := nm.setDefaultGateway(link, iface.Gateway, metric); err != nil {
				return err
			}
		}
	case "dhcp":
		if err := startDHCPClient(iface.Device); err != nil {
			return err
		}
	case "none", "":
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

func (nm *NetworkManager) applyBridgeInterface(name string, iface types.Interface) error {
	// Use Device field for bridge name
	bridgeDevice := iface.Device
	if bridgeDevice == "" {
		bridgeDevice = name
	}

	fmt.Printf("  Configuring bridge %s -> %s\n", name, bridgeDevice)

	// Set default MTU if not specified
	if iface.MTU == 0 {
		iface.MTU = 1500
	}

	// Check if bridge already exists
	existingBridge, err := nm.netlink.LinkByName(bridgeDevice)
	bridgeExists := (err == nil)

	var bridge netlink.Link

	if bridgeExists {
		// Bridge exists - check if configuration matches
		configMatches, configErr := nm.bridgeConfigMatches(existingBridge, iface)
		if configErr != nil {
			return fmt.Errorf("failed to check bridge config: %w", configErr)
		}

		if configMatches {
			// Bridge and ports match - check if IP also matches
			if iface.Protocol == "static" {
				ipMatches, ipErr := nm.checkIPMatches(existingBridge, iface.IPAddr, iface.Netmask)
				if ipErr != nil {
					return fmt.Errorf("failed to check IP matches: %w", ipErr)
				}
				if ipMatches {
					fmt.Printf("    [OK] Bridge %s already configured correctly\n", bridgeDevice)
					return nil
				}
			} else {
				fmt.Printf("    [OK] Bridge %s already configured correctly\n", bridgeDevice)
				return nil
			}
			bridge = existingBridge
		} else {
			// Check if MTU changed
			mtuChanged := existingBridge.Attrs().MTU != iface.MTU

			if mtuChanged {
				fmt.Printf("    [INFO] Bridge MTU changed (%d -> %d), recreating bridge\n",
					existingBridge.Attrs().MTU, iface.MTU)
				if err := nm.netlink.LinkDel(existingBridge); err != nil {
					return fmt.Errorf("failed to delete bridge for MTU change: %w", err)
				}
				bridgeExists = false
			} else {
				// Only port changes - update in-place
				if err := nm.updateBridgePorts(existingBridge, iface); err != nil {
					return err
				}
				bridge = existingBridge
			}
		}
	}

	if !bridgeExists {
		// Create new bridge
		bridge, err = nm.createBridge(bridgeDevice, iface)
		if err != nil {
			return err
		}
	}

	// Configure STP if requested
	if iface.BridgeSTP {
		fmt.Printf("    [INFO] STP configuration not yet implemented\n")
	}

	// Apply IP configuration
	return nm.applyIPConfiguration(bridge, bridgeDevice, iface)
}

func (nm *NetworkManager) createBridge(bridgeDevice string, iface types.Interface) (netlink.Link, error) {
	newBridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: bridgeDevice,
			MTU:  iface.MTU,
		},
	}

	if err := nm.netlink.LinkAdd(newBridge); err != nil {
		return nil, fmt.Errorf("failed to create bridge: %w", err)
	}

	fmt.Printf("    [OK] Bridge %s created\n", bridgeDevice)

	// Get the bridge we just created
	bridge, err := nm.netlink.LinkByName(bridgeDevice)
	if err != nil {
		return nil, fmt.Errorf("failed to get newly created bridge: %w", err)
	}

	// Add ports to bridge
	for _, portName := range iface.BridgePorts {
		if err := nm.addPortToBridge(portName, bridge); err != nil {
			return nil, err
		}
	}

	// Bring bridge up
	if err := nm.netlink.LinkSetUp(bridge); err != nil {
		return nil, fmt.Errorf("failed to bring up bridge: %w", err)
	}

	fmt.Printf("    [OK] Bridge %s is up\n", bridgeDevice)
	return bridge, nil
}

func (nm *NetworkManager) updateBridgePorts(bridge netlink.Link, iface types.Interface) error {
	fmt.Printf("    Updating bridge %s ports\n", bridge.Attrs().Name)

	// Get current ports
	currentPorts, err := nm.getBridgePorts(bridge.Attrs().Name)
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
			if err := nm.removePortFromBridge(portName); err != nil {
				return err
			}
		}
	}

	// Add ports that should be there
	for _, portName := range iface.BridgePorts {
		if !currentPortsMap[portName] {
			if err := nm.addPortToBridge(portName, bridge); err != nil {
				return err
			}
		}
	}

	return nil
}

func (nm *NetworkManager) addPortToBridge(portName string, bridge netlink.Link) error {
	port, err := nm.netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("failed to find port %s: %w", portName, err)
	}

	// Bring port up first
	if err := nm.netlink.LinkSetUp(port); err != nil {
		return fmt.Errorf("failed to bring up port %s: %w", portName, err)
	}

	// Add port to bridge
	if err := nm.netlink.LinkSetMaster(port, bridge); err != nil {
		return fmt.Errorf("failed to add port %s to bridge: %w", portName, err)
	}

	fmt.Printf("    [OK] Added port %s to bridge\n", portName)
	return nil
}

func (nm *NetworkManager) removePortFromBridge(portName string) error {
	port, err := nm.netlink.LinkByName(portName)
	if err != nil {
		return fmt.Errorf("failed to find port %s: %w", portName, err)
	}

	if err := nm.netlink.LinkSetNoMaster(port); err != nil {
		return fmt.Errorf("failed to remove port %s from bridge: %w", portName, err)
	}

	fmt.Printf("    [OK] Removed port %s from bridge\n", portName)
	return nil
}

func (nm *NetworkManager) applyVLANInterface(name string, iface types.Interface) error {
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
	parent, err := nm.netlink.LinkByName(iface.Device)
	if err != nil {
		return fmt.Errorf("failed to find parent interface %s: %w", iface.Device, err)
	}

	// Ensure parent is up
	if err = nm.netlink.LinkSetUp(parent); err != nil {
		return fmt.Errorf("failed to bring up parent interface: %w", err)
	}

	// Check if VLAN interface already exists
	existingLink, err := nm.netlink.LinkByName(iface.DeviceName)
	if err == nil {
		// VLAN exists, delete it first to ensure clean state
		fmt.Printf("    Removing existing VLAN interface %s\n", iface.DeviceName)
		if err = nm.netlink.LinkDel(existingLink); err != nil {
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

	if err = nm.netlink.LinkAdd(vlan); err != nil {
		return fmt.Errorf("failed to create VLAN interface: %w", err)
	}

	fmt.Printf("    [OK] VLAN interface %s created\n", iface.DeviceName)

	// Get the VLAN link we just created
	vlanLink, err := nm.netlink.LinkByName(iface.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to get VLAN interface: %w", err)
	}

	// Bring VLAN interface up
	if err := nm.netlink.LinkSetUp(vlanLink); err != nil {
		return fmt.Errorf("failed to bring up VLAN interface: %w", err)
	}

	fmt.Printf("    [OK] VLAN interface %s is up\n", iface.DeviceName)

	// Flush any existing IPs from VLAN interface
	if err := nm.flushIPs(vlanLink); err != nil {
		return fmt.Errorf("failed to flush IPs: %w", err)
	}

	// Apply IP configuration
	return nm.applyIPConfiguration(vlanLink, iface.DeviceName, iface)
}

func (nm *NetworkManager) applyIPConfiguration(link netlink.Link, deviceName string, iface types.Interface) error {
	switch iface.Protocol {
	case "static":
		if err := nm.ensureStaticIP(link, iface.IPAddr, iface.Netmask); err != nil {
			return err
		}
		if iface.Gateway != "" {
			metric := iface.Metric
			if metric == 0 {
				metric = 100
			}
			if err := nm.setDefaultGateway(link, iface.Gateway, metric); err != nil {
				return err
			}
		}
	case "dhcp":
		if err := startDHCPClient(deviceName); err != nil {
			return err
		}
	case "none", "":
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

// Helper functions

func (nm *NetworkManager) getBridgePorts(bridgeName string) ([]string, error) {
	bridge, err := nm.netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bridge: %w", err)
	}

	links, err := nm.netlink.LinkList()
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

func (nm *NetworkManager) checkIPMatches(link netlink.Link, ipAddr, netmask string) (bool, error) {
	if ipAddr == "" {
		return true, nil
	}

	desiredIP := net.ParseIP(ipAddr)
	if desiredIP == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	desiredMask := net.IPMask(net.ParseIP(netmask).To4())

	addrs, err := nm.netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return false, fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(desiredIP) && addr.Mask.String() == desiredMask.String() {
			return true, nil
		}
	}

	return false, nil
}

func (nm *NetworkManager) bridgeConfigMatches(bridge netlink.Link, desired types.Interface) (bool, error) {
	// Check MTU
	if bridge.Attrs().MTU != desired.MTU {
		return false, nil
	}

	// Check current ports vs desired ports
	currentPorts, err := nm.getBridgePorts(bridge.Attrs().Name)
	if err != nil {
		return false, err
	}

	if !stringSlicesEqual(currentPorts, desired.BridgePorts) {
		return false, nil
	}

	return true, nil
}

func (nm *NetworkManager) flushIPs(link netlink.Link) error {
	addrs, err := nm.netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		fmt.Printf("    Removing existing IP: %s\n", addr.IPNet.String())
		if err := nm.netlink.AddrDel(link, &addr); err != nil {
			return fmt.Errorf("failed to remove IP: %w", err)
		}
	}

	return nil
}

func (nm *NetworkManager) setStaticIP(link netlink.Link, ipAddr, netmask string) error {
	if ipAddr == "" {
		return nil
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

	if err := nm.netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %w", err)
	}

	fmt.Printf("    [OK] Assigned IP: %s/%d\n", ipAddr, prefixLen)
	return nil
}

func (nm *NetworkManager) ensureStaticIP(link netlink.Link, ipAddr, netmask string) error {
	if ipAddr == "" {
		return nil
	}

	desiredIP := net.ParseIP(ipAddr)
	if desiredIP == nil {
		return fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	desiredMask := net.IPMask(net.ParseIP(netmask).To4())
	prefixLen, _ := desiredMask.Size()

	addrs, err := nm.netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

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
		fmt.Printf("    Removing existing IP: %s\n", addr.IPNet.String())
		if err := nm.netlink.AddrDel(link, &addr); err != nil {
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

		if err := nm.netlink.AddrAdd(link, desiredAddr); err != nil {
			return fmt.Errorf("failed to add IP address: %w", err)
		}

		fmt.Printf("    [OK] Assigned IP: %s/%d\n", ipAddr, prefixLen)
	} else if len(unwantedAddrs) == 0 {
		fmt.Printf("    [OK] IP %s/%d already configured\n", ipAddr, prefixLen)
	}

	return nil
}

func (nm *NetworkManager) setDefaultGateway(link netlink.Link, gateway string, metric int) error {
	if gateway == "" {
		return nil
	}

	fmt.Printf("    Setting default gateway: %s\n", gateway)

	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("invalid gateway address: %s", gateway)
	}

	// Remove existing default routes first
	routes, err := nm.netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			fmt.Printf("    Removing old default route via %s\n", route.Gw)
			_ = nm.netlink.RouteDel(&route) // Ignore errors
		}
	}

	// Add new default route
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       nil,
		Gw:        gw,
		Priority:  metric,
		Table:     254,
	}

	if err := nm.netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("failed to add default route: %w", err)
	}

	fmt.Printf("    [OK] Default gateway set\n")
	return nil
}

// Helper functions (not methods)

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

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

func startDHCPClient(deviceName string) error {
	fmt.Printf("    Starting DHCP client on %s\n", deviceName)

	if _, err := exec.LookPath("dhclient"); err != nil {
		return fmt.Errorf("dhclient not found: %w", err)
	}

	// Kill any existing dhclient for this interface
	_ = exec.Command("pkill", "-f", fmt.Sprintf("dhclient.*%s", deviceName)).Run() //nolint:errcheck,gosec

	cmd := exec.Command("dhclient", "-v", deviceName)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dhclient: %w", err)
	}

	fmt.Printf("    [OK] DHCP client started\n")
	return nil
}
