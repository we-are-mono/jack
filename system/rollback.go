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
	"log"
	"net"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
)

// RestoreSnapshot restores the system to the state captured in the snapshot.
// The scope parameter determines what subsystems to restore: "all", "ipforward", "interfaces", "routes".
func RestoreSnapshot(snapshot *SystemSnapshot, scope []string) error {
	var errors []error

	// Rollback in REVERSE order of apply
	// Order: routes → plugins → interfaces → ipforward

	if containsScope(scope, "routes") || containsScope(scope, "all") {
		if err := rollbackRoutes(snapshot.Routes); err != nil {
			log.Printf("[ERROR] Failed to rollback routes: %v", err)
			errors = append(errors, fmt.Errorf("routes: %w", err))
		} else {
			log.Printf("[INFO] Successfully rolled back routes")
		}
	}

	// Note: Plugin rollback is handled separately by the daemon
	// since it requires access to the plugin registry

	if containsScope(scope, "interfaces") || containsScope(scope, "all") {
		if err := rollbackInterfaces(snapshot.Interfaces); err != nil {
			log.Printf("[ERROR] Failed to rollback interfaces: %v", err)
			errors = append(errors, fmt.Errorf("interfaces: %w", err))
		} else {
			log.Printf("[INFO] Successfully rolled back interfaces")
		}
	}

	if containsScope(scope, "ipforward") || containsScope(scope, "all") {
		if err := rollbackIPForwarding(snapshot.IPForwarding); err != nil {
			log.Printf("[ERROR] Failed to rollback IP forwarding: %v", err)
			errors = append(errors, fmt.Errorf("ipforward: %w", err))
		} else {
			log.Printf("[INFO] Successfully rolled back IP forwarding")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with %d error(s): %v", len(errors), errors)
	}

	return nil
}

// rollbackIPForwarding restores the IP forwarding setting.
func rollbackIPForwarding(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}

	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(value), 0644); err != nil {
		return fmt.Errorf("failed to write to /proc/sys/net/ipv4/ip_forward: %w", err)
	}

	return nil
}

// rollbackInterfaces restores all interfaces to their snapshot state.
func rollbackInterfaces(snapshots map[string]InterfaceSnapshot) error {
	// Get current interfaces
	currentLinks, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list current interfaces: %w", err)
	}

	currentNames := make(map[string]netlink.Link)
	for _, link := range currentLinks {
		currentNames[link.Attrs().Name] = link
	}

	// 1. Remove interfaces that were created during apply (didn't exist in snapshot)
	for name, link := range currentNames {
		if name == "lo" {
			continue // Skip loopback
		}

		snapshot, existedInSnapshot := snapshots[name]
		if !existedInSnapshot || !snapshot.Existed {
			// Interface didn't exist in snapshot, remove it
			log.Printf("[INFO] Removing interface %s (created during apply)", name)
			if err := netlink.LinkDel(link); err != nil {
				log.Printf("[WARN] Failed to remove interface %s: %v", name, err)
			}
		}
	}

	// 2. Restore interfaces that existed in snapshot
	for name, snapshot := range snapshots {
		if !snapshot.Existed {
			continue // Don't try to restore interfaces that didn't exist
		}

		if err := restoreInterfaceState(snapshot, currentNames); err != nil {
			log.Printf("[WARN] Failed to restore interface %s: %v", name, err)
			// Continue with other interfaces
		}
	}

	return nil
}

// restoreInterfaceState restores a single interface to its snapshot state.
func restoreInterfaceState(snapshot InterfaceSnapshot, currentLinks map[string]netlink.Link) error {
	link, exists := currentLinks[snapshot.Name]
	if !exists {
		// Interface was deleted during apply
		log.Printf("[INFO] Interface %s was deleted, attempting to recreate", snapshot.Name)
		return recreateInterface(snapshot)
	}

	// Restore MTU
	if link.Attrs().MTU != snapshot.MTU {
		log.Printf("[INFO] Restoring MTU for %s: %d -> %d", snapshot.Name, link.Attrs().MTU, snapshot.MTU)
		if err := netlink.LinkSetMTU(link, snapshot.MTU); err != nil {
			return fmt.Errorf("failed to set MTU: %w", err)
		}
	}

	// Restore state (up/down)
	currentUp := link.Attrs().Flags&net.FlagUp != 0
	snapshotUp := snapshot.State == "up"

	if currentUp != snapshotUp {
		log.Printf("[INFO] Restoring state for %s: %s", snapshot.Name, snapshot.State)
		if snapshotUp {
			if err := netlink.LinkSetUp(link); err != nil {
				return fmt.Errorf("failed to set link up: %w", err)
			}
		} else {
			if err := netlink.LinkSetDown(link); err != nil {
				return fmt.Errorf("failed to set link down: %w", err)
			}
		}
	}

	// Restore IP addresses
	if err := restoreIPAddresses(link, snapshot.Addresses); err != nil {
		return fmt.Errorf("failed to restore IP addresses: %w", err)
	}

	// Restore bridge ports if this is a bridge
	if snapshot.Type == "bridge" && len(snapshot.Ports) > 0 {
		if err := restoreBridgePorts(link, snapshot.Ports); err != nil {
			return fmt.Errorf("failed to restore bridge ports: %w", err)
		}
	}

	return nil
}

// restoreIPAddresses restores the IP addresses on an interface.
func restoreIPAddresses(link netlink.Link, snapshotAddrs []string) error {
	// Get current addresses
	currentAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list current addresses: %w", err)
	}

	// Remove addresses not in snapshot
	for _, current := range currentAddrs {
		addrStr := current.IPNet.String()
		if !containsString(snapshotAddrs, addrStr) {
			log.Printf("[INFO] Removing address %s from %s", addrStr, link.Attrs().Name)
			if err := netlink.AddrDel(link, &current); err != nil {
				log.Printf("[WARN] Failed to remove address %s: %v", addrStr, err)
			}
		}
	}

	// Add addresses from snapshot
	for _, addrStr := range snapshotAddrs {
		// Check if address already exists
		found := false
		for _, current := range currentAddrs {
			if current.IPNet.String() == addrStr {
				found = true
				break
			}
		}

		if !found {
			log.Printf("[INFO] Adding address %s to %s", addrStr, link.Attrs().Name)
			addr, err := netlink.ParseAddr(addrStr)
			if err != nil {
				log.Printf("[WARN] Failed to parse address %s: %v", addrStr, err)
				continue
			}

			if err := netlink.AddrAdd(link, addr); err != nil {
				log.Printf("[WARN] Failed to add address %s: %v", addrStr, err)
			}
		}
	}

	return nil
}

// restoreBridgePorts restores the port membership of a bridge.
func restoreBridgePorts(bridge netlink.Link, snapshotPorts []string) error {
	// Get current ports
	currentPorts, err := getBridgePorts(bridge.Attrs().Name)
	if err != nil {
		return fmt.Errorf("failed to get current bridge ports: %w", err)
	}

	// Remove ports not in snapshot
	for _, port := range currentPorts {
		if !containsString(snapshotPorts, port) {
			log.Printf("[INFO] Removing port %s from bridge %s", port, bridge.Attrs().Name)
			if portLink, err := netlink.LinkByName(port); err == nil {
				if err := netlink.LinkSetNoMaster(portLink); err != nil {
					log.Printf("[WARN] Failed to remove port %s: %v", port, err)
				}
			}
		}
	}

	// Add ports from snapshot
	for _, port := range snapshotPorts {
		if !containsString(currentPorts, port) {
			log.Printf("[INFO] Adding port %s to bridge %s", port, bridge.Attrs().Name)
			if portLink, err := netlink.LinkByName(port); err == nil {
				if err := netlink.LinkSetMaster(portLink, bridge); err != nil {
					log.Printf("[WARN] Failed to add port %s: %v", port, err)
				}
			}
		}
	}

	return nil
}

// recreateInterface attempts to recreate an interface that was deleted.
func recreateInterface(snapshot InterfaceSnapshot) error {
	// This is complex and depends on interface type
	// For now, we log a warning and skip recreation
	log.Printf("[WARN] Cannot recreate deleted interface %s (type: %s)", snapshot.Name, snapshot.Type)
	log.Printf("[WARN] Manual intervention may be required to restore %s", snapshot.Name)
	return nil
}

// rollbackRoutes restores the routing table to the snapshot state.
func rollbackRoutes(snapshotRoutes []RouteSnapshot) error {
	// Get current routes
	currentRoutes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list current routes: %w", err)
	}

	// Remove routes not in snapshot
	for _, current := range currentRoutes {
		// Skip routes we shouldn't touch (kernel routes, etc.)
		if current.Protocol == 2 { // RTPROT_KERNEL
			continue
		}
		if current.Scope == netlink.SCOPE_LINK {
			continue // Skip link-local routes
		}

		if !snapshotRouteExists(current, snapshotRoutes) {
			log.Printf("[INFO] Removing route: dst=%v gw=%v", current.Dst, current.Gw)
			if err := netlink.RouteDel(&current); err != nil {
				log.Printf("[WARN] Failed to remove route: %v", err)
			}
		}
	}

	// Add routes from snapshot
	for _, snapshot := range snapshotRoutes {
		if !snapshotRouteInCurrent(snapshot, currentRoutes) {
			log.Printf("[INFO] Adding route: dst=%s gw=%s", snapshot.Destination, snapshot.Gateway)

			route := &netlink.Route{
				Priority: snapshot.Metric,
				Table:    snapshot.Table,
				Scope:    netlink.Scope(snapshot.Scope),
			}

			// Parse destination
			if snapshot.Destination != "default" {
				_, dst, err := net.ParseCIDR(snapshot.Destination)
				if err != nil {
					log.Printf("[WARN] Failed to parse destination %s: %v", snapshot.Destination, err)
					continue
				}
				route.Dst = dst
			}

			// Parse gateway
			if snapshot.Gateway != "" {
				route.Gw = net.ParseIP(snapshot.Gateway)
			}

			// Get link index
			if snapshot.Device != "" {
				if link, err := netlink.LinkByName(snapshot.Device); err == nil {
					route.LinkIndex = link.Attrs().Index
				}
			}

			if err := netlink.RouteAdd(route); err != nil {
				log.Printf("[WARN] Failed to add route: %v", err)
			}
		}
	}

	return nil
}

// snapshotRouteExists checks if a netlink.Route exists in the snapshot.
func snapshotRouteExists(route netlink.Route, snapshots []RouteSnapshot) bool {
	for _, snapshot := range snapshots {
		if snapshotRoutesMatch(route, snapshot) {
			return true
		}
	}
	return false
}

// snapshotRouteInCurrent checks if a RouteSnapshot exists in the current routes.
func snapshotRouteInCurrent(snapshot RouteSnapshot, routes []netlink.Route) bool {
	for _, route := range routes {
		if snapshotRoutesMatch(route, snapshot) {
			return true
		}
	}
	return false
}

// snapshotRoutesMatch checks if a netlink.Route matches a RouteSnapshot.
func snapshotRoutesMatch(route netlink.Route, snapshot RouteSnapshot) bool {
	// Compare destination
	routeDst := "default"
	if route.Dst != nil {
		routeDst = route.Dst.String()
	}
	if routeDst != snapshot.Destination {
		return false
	}

	// Compare gateway
	routeGw := ""
	if route.Gw != nil {
		routeGw = route.Gw.String()
	}
	if routeGw != snapshot.Gateway {
		return false
	}

	// Compare device
	routeDev := ""
	if route.LinkIndex > 0 {
		if link, err := netlink.LinkByIndex(route.LinkIndex); err == nil {
			routeDev = link.Attrs().Name
		}
	}
	if routeDev != snapshot.Device {
		return false
	}

	// Compare metric and table
	if route.Priority != snapshot.Metric {
		return false
	}
	if route.Table != snapshot.Table {
		return false
	}

	return true
}

// containsScope checks if a scope list contains the given scope.
func containsScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// containsString checks if a string slice contains the given string.
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RestoreNftablesRules restores the nftables ruleset from a JSON dump.
func RestoreNftablesRules(rulesJSON string) error {
	if rulesJSON == "" {
		// No rules to restore
		return nil
	}

	// Write rules to temp file
	tmpFile, err := os.CreateTemp("", "jack-nft-rollback-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(rulesJSON); err != nil {
		return fmt.Errorf("failed to write rules to temp file: %w", err)
	}
	tmpFile.Close()

	// Restore rules using nft
	cmd := exec.Command("nft", "-j", "-f", tmpFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restore nftables rules: %w (output: %s)", err, string(output))
	}

	log.Printf("[INFO] Successfully restored nftables rules")
	return nil
}
