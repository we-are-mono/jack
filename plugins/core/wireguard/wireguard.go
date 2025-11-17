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

// Package main implements the WireGuard VPN plugin for Jack.
package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/vishvananda/netlink"
)

// WireGuardProvider implements the VPN provider interface for WireGuard
type WireGuardProvider struct {
	interfaces map[string]VPNInterface
}

// New creates a new WireGuard provider
func New() (*WireGuardProvider, error) {
	// Check if wg command is available
	if _, err := exec.LookPath("wg"); err != nil {
		return nil, fmt.Errorf("wireguard-tools not found: %w (install with: apt install wireguard-tools)", err)
	}

	return &WireGuardProvider{
		interfaces: make(map[string]VPNInterface),
	}, nil
}

// ApplyConfig applies the WireGuard VPN configuration
func (p *WireGuardProvider) ApplyConfig(ctx context.Context, config *VPNConfig) error {
	fmt.Println("Applying WireGuard VPN configuration...")

	// Store interfaces for later reference
	p.interfaces = config.Interfaces

	// Apply each interface
	for name, iface := range config.Interfaces {
		if !iface.Enabled {
			// Remove interface if it exists
			if link, err := netlink.LinkByName(iface.DeviceName); err == nil {
				fmt.Printf("  Removing disabled interface: %s\n", iface.DeviceName)
				if err := netlink.LinkDel(link); err != nil {
					return fmt.Errorf("failed to remove interface %s: %w", iface.DeviceName, err)
				}
				fmt.Printf("    [OK] Interface %s removed\n", iface.DeviceName)
			}
			continue
		}

		if err := p.applyInterface(name, iface); err != nil {
			return fmt.Errorf("failed to apply interface %s: %w", name, err)
		}
	}

	fmt.Println("✓ WireGuard configuration applied successfully")
	return nil
}

// Validate validates the WireGuard configuration
func (p *WireGuardProvider) Validate(ctx context.Context, config *VPNConfig) error {
	// Use comprehensive validation from pure function
	return ValidateVPNConfig(config)
}

// Flush removes all WireGuard interfaces
func (p *WireGuardProvider) Flush(ctx context.Context) error {
	fmt.Println("Flushing all WireGuard interfaces...")

	// List all WireGuard interfaces
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list links: %w", err)
	}

	for _, link := range links {
		// Check if it's a WireGuard interface
		if link.Type() == "wireguard" {
			fmt.Printf("  Removing WireGuard interface: %s\n", link.Attrs().Name)
			if err := netlink.LinkDel(link); err != nil {
				fmt.Printf("  Warning: failed to remove %s: %v\n", link.Attrs().Name, err)
			}
		}
	}

	fmt.Println("✓ WireGuard interfaces flushed")
	return nil
}

// Enable activates all configured WireGuard interfaces
func (p *WireGuardProvider) Enable(ctx context.Context) error {
	fmt.Println("Enabling WireGuard interfaces...")

	for name, iface := range p.interfaces {
		if !iface.Enabled {
			continue
		}

		link, err := netlink.LinkByName(iface.DeviceName)
		if err != nil {
			return fmt.Errorf("interface %s not found: %w", name, err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to bring up %s: %w", name, err)
		}

		fmt.Printf("  ✓ Interface %s is up\n", iface.DeviceName)
	}

	fmt.Println("✓ WireGuard interfaces enabled")
	return nil
}

// Disable deactivates all WireGuard interfaces
func (p *WireGuardProvider) Disable(ctx context.Context) error {
	fmt.Println("Disabling WireGuard interfaces...")

	for name, iface := range p.interfaces {
		link, err := netlink.LinkByName(iface.DeviceName)
		if err != nil {
			// Interface might not exist, skip
			continue
		}

		if err := netlink.LinkSetDown(link); err != nil {
			fmt.Printf("  Warning: failed to bring down %s: %v\n", name, err)
		} else {
			fmt.Printf("  ✓ Interface %s is down\n", iface.DeviceName)
		}
	}

	fmt.Println("✓ WireGuard interfaces disabled")
	return nil
}

// Status returns the status of WireGuard VPN
func (p *WireGuardProvider) Status(ctx context.Context) (bool, string, int, error) {
	// Count active WireGuard interfaces
	links, err := netlink.LinkList()
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to list links: %w", err)
	}

	tunnelCount := 0
	for _, link := range links {
		if link.Type() == "wireguard" && link.Attrs().Flags&net.FlagUp != 0 {
			tunnelCount++
		}
	}

	enabled := tunnelCount > 0
	return enabled, "wireguard", tunnelCount, nil
}

// applyInterface configures a single WireGuard interface
func (p *WireGuardProvider) applyInterface(name string, iface VPNInterface) error {
	fmt.Printf("  Configuring WireGuard interface %s -> %s\n", name, iface.DeviceName)

	// Check if interface already exists
	existingLink, err := netlink.LinkByName(iface.DeviceName)
	interfaceExists := (err == nil)

	if interfaceExists {
		// Interface exists - check if config matches
		configMatches, err := p.interfaceConfigMatches(iface)
		if err != nil {
			fmt.Printf("    [WARN] Failed to check interface config: %v, recreating...\n", err)
		} else if configMatches {
			fmt.Printf("    [OK] WireGuard interface %s already configured correctly\n", iface.DeviceName)
			return nil
		} else {
			fmt.Printf("    [INFO] WireGuard configuration changed, recreating interface\n")
		}

		// Delete existing interface to recreate with new config
		fmt.Printf("    Removing existing interface %s\n", iface.DeviceName)
		if err := netlink.LinkDel(existingLink); err != nil {
			return fmt.Errorf("failed to delete existing interface: %w", err)
		}
	}

	// Create WireGuard interface
	cmd := exec.Command("ip", "link", "add", "dev", iface.DeviceName, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create interface: %s: %w", string(output), err)
	}

	fmt.Printf("    [OK] Interface %s created\n", iface.DeviceName)

	// Set private key
	cmd = exec.Command("wg", "set", iface.DeviceName, "private-key", "/dev/stdin")
	cmd.Stdin = strings.NewReader(iface.PrivateKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set private key: %s: %w", string(output), err)
	}

	// Set listen port if specified
	if iface.ListenPort > 0 {
		cmd = exec.Command("wg", "set", iface.DeviceName, "listen-port", fmt.Sprintf("%d", iface.ListenPort))
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to set listen port: %s: %w", string(output), err)
		}
		fmt.Printf("    [OK] Listen port set to %d\n", iface.ListenPort)
	}

	// Configure peers
	for i, peer := range iface.Peers {
		if err := p.configurePeer(iface.DeviceName, peer, i); err != nil {
			return fmt.Errorf("failed to configure peer %d: %w", i, err)
		}
	}

	// Get the interface link
	wgLink, err := netlink.LinkByName(iface.DeviceName)
	if err != nil {
		return fmt.Errorf("failed to get interface: %w", err)
	}

	// Set MTU if specified
	if iface.MTU > 0 {
		if err := netlink.LinkSetMTU(wgLink, iface.MTU); err != nil {
			return fmt.Errorf("failed to set MTU: %w", err)
		}
	}

	// Bring interface up
	if err := netlink.LinkSetUp(wgLink); err != nil {
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	fmt.Printf("    [OK] Interface %s is up\n", iface.DeviceName)

	// Flush any existing IPs
	if err := p.flushIPs(wgLink); err != nil {
		return fmt.Errorf("failed to flush IPs: %w", err)
	}

	// Set IP address
	if err := p.setStaticIP(wgLink, iface.Address, iface.Netmask); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}

	// Add routes for peer allowed IPs
	if err := p.addPeerRoutes(wgLink, iface); err != nil {
		return fmt.Errorf("failed to add peer routes: %w", err)
	}

	return nil
}

// configurePeer configures a single WireGuard peer
func (p *WireGuardProvider) configurePeer(deviceName string, peer WireGuardPeer, index int) error {
	// Build wg set command using pure function
	args, needsStdin := BuildPeerArgs(deviceName, peer)

	cmd := exec.Command("wg", args...)

	// If preshared key is provided, pass it via stdin
	if needsStdin {
		cmd.Stdin = strings.NewReader(peer.PresharedKey)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to configure peer: %s: %w", string(output), err)
	}

	peerDesc := peer.PublicKey[:16] + "..."
	if peer.Comment != "" {
		peerDesc = peer.Comment
	}
	fmt.Printf("    [OK] Peer %d configured: %s\n", index+1, peerDesc)

	return nil
}

// flushIPs removes all IP addresses from an interface
func (p *WireGuardProvider) flushIPs(link netlink.Link) error {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		if err := netlink.AddrDel(link, &addr); err != nil {
			return fmt.Errorf("failed to delete address %s: %w", addr.IPNet, err)
		}
	}

	return nil
}

// setStaticIP sets a static IP address on an interface
func (p *WireGuardProvider) setStaticIP(link netlink.Link, ipAddr, netmask string) error {
	// Parse CIDR or construct from IP + netmask using pure function
	cidr := FormatCIDR(ipAddr, netmask)

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("invalid IP address %s: %w", cidr, err)
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add address: %w", err)
	}

	fmt.Printf("    [OK] IP address set: %s\n", cidr)
	return nil
}

// addPeerRoutes adds routes for all allowed IPs from all peers
func (p *WireGuardProvider) addPeerRoutes(link netlink.Link, iface VPNInterface) error {
	deviceName := link.Attrs().Name

	// Calculate metric based on interface config or use default
	metric := 50 // Default metric for VPN routes

	for _, peer := range iface.Peers {
		for _, allowedIP := range peer.AllowedIPs {
			// Parse the allowed IP as CIDR
			_, ipNet, err := net.ParseCIDR(allowedIP)
			if err != nil {
				fmt.Printf("    [WARN] Failed to parse allowed IP %s: %v\n", allowedIP, err)
				continue
			}

			// Check if this is a default route (0.0.0.0/0 or ::/0)
			isDefaultRoute := (allowedIP == "0.0.0.0/0" || allowedIP == "::/0")

			// Create the route
			route := &netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       ipNet,
				Priority:  metric,
			}

			// For default routes, we don't set Dst (it means default)
			if isDefaultRoute {
				route.Dst = nil
			}

			// Try to add the route (ignore error if already exists)
			if err := netlink.RouteAdd(route); err != nil {
				// Check if route already exists
				if !strings.Contains(err.Error(), "file exists") {
					fmt.Printf("    [WARN] Failed to add route for %s: %v\n", allowedIP, err)
				}
			} else {
				if isDefaultRoute {
					fmt.Printf("    [OK] Added default route via %s (metric %d)\n", deviceName, metric)
				} else {
					fmt.Printf("    [OK] Added route %s via %s\n", allowedIP, deviceName)
				}
			}
		}
	}

	return nil
}

// interfaceConfigMatches checks if the current WireGuard interface matches the desired config
func (p *WireGuardProvider) interfaceConfigMatches(desired VPNInterface) (bool, error) {
	// Get current WireGuard config using 'wg show'
	cmd := exec.Command("wg", "show", desired.DeviceName, "dump")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to run 'wg show': %w", err)
	}

	// Parse wg dump output format:
	// Line 1: interface private-key public-key listen-port fwmark
	// Line 2+: peer public-key preshared-key endpoint allowed-ips latest-handshake transfer-rx transfer-tx persistent-keepalive
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 1 {
		return false, fmt.Errorf("empty wg dump output")
	}

	// Parse interface line
	interfaceFields := strings.Fields(lines[0])
	if len(interfaceFields) < 4 {
		return false, fmt.Errorf("invalid wg dump interface line")
	}

	// Check listen port (field index 3)
	if desired.ListenPort > 0 {
		currentListenPort := interfaceFields[3]
		desiredListenPort := fmt.Sprintf("%d", desired.ListenPort)
		if currentListenPort != desiredListenPort && currentListenPort != "0" {
			return false, nil // Listen port mismatch
		}
	}

	// Check peer count
	currentPeerCount := len(lines) - 1
	if currentPeerCount != len(desired.Peers) {
		return false, nil // Different number of peers
	}

	// Build map of desired peers by public key for easy lookup
	desiredPeers := make(map[string]WireGuardPeer)
	for _, peer := range desired.Peers {
		desiredPeers[peer.PublicKey] = peer
	}

	// Check each current peer
	for i := 1; i < len(lines); i++ {
		peerFields := strings.Fields(lines[i])
		if len(peerFields) < 9 {
			return false, fmt.Errorf("invalid wg dump peer line")
		}

		currentPublicKey := peerFields[1]
		currentEndpoint := peerFields[3]
		currentAllowedIPs := peerFields[4]
		currentKeepalive := peerFields[8]

		// Find matching desired peer
		desiredPeer, found := desiredPeers[currentPublicKey]
		if !found {
			return false, nil // Peer not in desired config
		}

		// Check endpoint
		if desiredPeer.Endpoint != "" && currentEndpoint != desiredPeer.Endpoint {
			return false, nil
		}

		// Check allowed IPs (split and compare as sets)
		currentIPs := strings.Split(currentAllowedIPs, ",")
		desiredIPs := desiredPeer.AllowedIPs
		if !StringSlicesEqual(currentIPs, desiredIPs) {
			return false, nil
		}

		// Check persistent keepalive
		if desiredPeer.PersistentKeepalive > 0 {
			desiredKeepalive := fmt.Sprintf("%d", desiredPeer.PersistentKeepalive)
			if currentKeepalive != desiredKeepalive {
				return false, nil
			}
		}
	}

	// Check interface IP and MTU
	link, err := netlink.LinkByName(desired.DeviceName)
	if err != nil {
		return false, fmt.Errorf("failed to get interface: %w", err)
	}

	// Check MTU
	if desired.MTU > 0 && link.Attrs().MTU != desired.MTU {
		return false, nil
	}

	// Check IP address
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return false, fmt.Errorf("failed to list addresses: %w", err)
	}

	if len(addrs) == 0 && desired.Address != "" {
		return false, nil // No IP but one is desired
	}

	if len(addrs) > 0 && desired.Address != "" {
		currentIP := addrs[0].IP.String()
		if currentIP != desired.Address {
			return false, nil
		}
	}

	// All checks passed - config matches
	return true, nil
}
