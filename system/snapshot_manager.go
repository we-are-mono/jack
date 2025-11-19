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
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon/logger"
)

const (
	// FAMILY_V4 is the IPv4 address family
	FAMILY_V4 = netlink.FAMILY_V4
	// SCOPE_LINK is the link-local scope
	SCOPE_LINK = netlink.SCOPE_LINK
)

// CaptureSystemSnapshot captures the current state of the system.
// This includes IP forwarding, all network interfaces, routes, and firewall rules.
func (sm *SnapshotManager) CaptureSystemSnapshot() (*SystemSnapshot, error) {
	snapshot := &SystemSnapshot{
		Timestamp:    time.Now(),
		CheckpointID: fmt.Sprintf("auto-%d", time.Now().Unix()),
		Interfaces:   make(map[string]InterfaceSnapshot),
	}

	// Capture IP forwarding setting
	if data, err := sm.fs.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		snapshot.IPForwarding = strings.TrimSpace(string(data)) == "1"
	}

	// Capture all network interfaces
	links, err := sm.netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, link := range links {
		// Skip loopback interface
		if link.Attrs().Name == "lo" {
			continue
		}

		ifaceSnapshot, err := sm.captureInterfaceState(link)
		if err != nil {
			// Log warning but continue with other interfaces
			fmt.Fprintf(os.Stderr, "[WARN] Failed to capture state for %s: %v\n", link.Attrs().Name, err)
			continue
		}
		snapshot.Interfaces[link.Attrs().Name] = ifaceSnapshot
	}

	// Capture routing table
	routes, err := sm.netlink.RouteList(nil, FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	for _, route := range routes {
		routeSnapshot := RouteSnapshot{
			Metric: route.Priority,
			Table:  route.Table,
			Scope:  int(route.Scope),
		}

		// Destination
		if route.Dst != nil {
			routeSnapshot.Destination = route.Dst.String()
		} else {
			routeSnapshot.Destination = "default"
		}

		// Gateway
		if route.Gw != nil {
			routeSnapshot.Gateway = route.Gw.String()
		}

		// Device
		if route.LinkIndex > 0 {
			if link, err := sm.netlink.LinkByIndex(route.LinkIndex); err == nil {
				routeSnapshot.Device = link.Attrs().Name
			}
		}

		snapshot.Routes = append(snapshot.Routes, routeSnapshot)
	}

	// Capture nftables ruleset (if jack table exists)
	// This is best-effort; if nft command fails, we continue
	if output, err := sm.cmd.Run("nft", "-j", "list", "table", "inet", "jack"); err == nil {
		snapshot.NftablesRules = string(output)
	}

	return snapshot, nil
}

// captureInterfaceState captures the state of a single interface.
func (sm *SnapshotManager) captureInterfaceState(link netlink.Link) (InterfaceSnapshot, error) {
	snapshot := InterfaceSnapshot{
		Name:    link.Attrs().Name,
		Existed: true,
		MTU:     link.Attrs().MTU,
	}

	// Capture state (up/down)
	if link.Attrs().Flags&net.FlagUp != 0 {
		snapshot.State = "up"
	} else {
		snapshot.State = "down"
	}

	// Capture IP addresses
	addrs, err := sm.netlink.AddrList(link, FAMILY_V4)
	if err != nil {
		return snapshot, fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		snapshot.Addresses = append(snapshot.Addresses, addr.IPNet.String())
	}

	// Capture default gateway (if any)
	routes, err := sm.netlink.RouteList(link, FAMILY_V4)
	if err == nil {
		for _, route := range routes {
			if route.Dst == nil && route.Gw != nil {
				// This is a default route
				snapshot.Gateway = route.Gw.String()
				snapshot.Metric = route.Priority
				break
			}
		}
	}

	// Capture type-specific details
	switch link.Type() {
	case "bridge":
		snapshot.Type = "bridge"
		ports, err := sm.getBridgePorts(link.Attrs().Name)
		if err == nil {
			snapshot.Ports = ports
		}

	case "vlan":
		snapshot.Type = "vlan"
		if vlan, ok := link.(*netlink.Vlan); ok {
			snapshot.VLANId = vlan.VlanId
			// Get parent name
			if parent, err := sm.netlink.LinkByIndex(vlan.ParentIndex); err == nil {
				snapshot.ParentDevice = parent.Attrs().Name
			}
		}

	case "wireguard":
		snapshot.Type = "wireguard"

	default:
		snapshot.Type = "physical"
	}

	return snapshot, nil
}

// getBridgePorts returns the list of ports attached to a bridge.
func (sm *SnapshotManager) getBridgePorts(bridgeName string) ([]string, error) {
	// Get bridge interface
	bridge, err := sm.netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to find bridge %s: %w", bridgeName, err)
	}

	bridgeIndex := bridge.Attrs().Index

	// List all interfaces and find those with this bridge as master
	links, err := sm.netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var ports []string
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			ports = append(ports, link.Attrs().Name)
		}
	}

	return ports, nil
}

// RestoreSnapshot restores the system to the state captured in the snapshot.
// The scope parameter determines what subsystems to restore: "all", "ipforward", "interfaces", "routes".
func (sm *SnapshotManager) RestoreSnapshot(snapshot *SystemSnapshot, scope []string) error {
	var errors []error

	// Rollback in REVERSE order of apply
	// Order: routes → plugins → interfaces → ipforward

	if containsScope(scope, "routes") || containsScope(scope, "all") {
		if err := sm.rollbackRoutes(snapshot.Routes); err != nil {
			logger.Error("Failed to rollback routes",
				logger.Field{Key: "error", Value: err.Error()})
			errors = append(errors, fmt.Errorf("routes: %w", err))
		} else {
			logger.Info("Successfully rolled back routes")
		}
	}

	// Note: Plugin rollback is handled separately by the daemon
	// since it requires access to the plugin registry

	if containsScope(scope, "interfaces") || containsScope(scope, "all") {
		if err := sm.rollbackInterfaces(snapshot.Interfaces); err != nil {
			logger.Error("Failed to rollback interfaces",
				logger.Field{Key: "error", Value: err.Error()})
			errors = append(errors, fmt.Errorf("interfaces: %w", err))
		} else {
			logger.Info("Successfully rolled back interfaces")
		}
	}

	if containsScope(scope, "ipforward") || containsScope(scope, "all") {
		if err := sm.rollbackIPForwarding(snapshot.IPForwarding); err != nil {
			logger.Error("Failed to rollback IP forwarding",
				logger.Field{Key: "error", Value: err.Error()})
			errors = append(errors, fmt.Errorf("ipforward: %w", err))
		} else {
			logger.Info("Successfully rolled back IP forwarding")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("rollback completed with %d error(s): %v", len(errors), errors)
	}

	return nil
}

// rollbackIPForwarding restores the IP forwarding setting.
func (sm *SnapshotManager) rollbackIPForwarding(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}

	if err := sm.fs.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(value), 0644); err != nil {
		return fmt.Errorf("failed to write to /proc/sys/net/ipv4/ip_forward: %w", err)
	}

	return nil
}

// rollbackInterfaces restores all interfaces to their snapshot state.
func (sm *SnapshotManager) rollbackInterfaces(snapshots map[string]InterfaceSnapshot) error {
	// Get current interfaces
	currentLinks, err := sm.netlink.LinkList()
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
			logger.Info("Removing interface (created during apply)",
				logger.Field{Key: "interface", Value: name})
			if err := sm.netlink.LinkDel(link); err != nil {
				logger.Warn("Failed to remove interface",
					logger.Field{Key: "interface", Value: name},
					logger.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// 2. Restore interfaces that existed in snapshot
	for name, snapshot := range snapshots {
		if !snapshot.Existed {
			continue // Don't try to restore interfaces that didn't exist
		}

		if err := sm.restoreInterfaceState(snapshot, currentNames); err != nil {
			logger.Warn("Failed to restore interface",
				logger.Field{Key: "interface", Value: name},
				logger.Field{Key: "error", Value: err.Error()})
			// Continue with other interfaces
		}
	}

	return nil
}

// restoreInterfaceState restores a single interface to its snapshot state.
func (sm *SnapshotManager) restoreInterfaceState(snapshot InterfaceSnapshot, currentLinks map[string]netlink.Link) error {
	link, exists := currentLinks[snapshot.Name]
	if !exists {
		// Interface was deleted during apply
		logger.Info("Interface was deleted, attempting to recreate",
			logger.Field{Key: "interface", Value: snapshot.Name})
		return sm.recreateInterface(snapshot)
	}

	// Restore MTU
	if link.Attrs().MTU != snapshot.MTU {
		logger.Info("Restoring MTU",
			logger.Field{Key: "interface", Value: snapshot.Name},
			logger.Field{Key: "current_mtu", Value: link.Attrs().MTU},
			logger.Field{Key: "target_mtu", Value: snapshot.MTU})
		if err := sm.netlink.LinkSetMTU(link, snapshot.MTU); err != nil {
			return fmt.Errorf("failed to set MTU: %w", err)
		}
	}

	// Restore state (up/down)
	currentUp := link.Attrs().Flags&net.FlagUp != 0
	snapshotUp := snapshot.State == "up"

	if currentUp != snapshotUp {
		logger.Info("Restoring interface state",
			logger.Field{Key: "interface", Value: snapshot.Name},
			logger.Field{Key: "state", Value: snapshot.State})
		if snapshotUp {
			if err := sm.netlink.LinkSetUp(link); err != nil {
				return fmt.Errorf("failed to set link up: %w", err)
			}
		} else {
			if err := sm.netlink.LinkSetDown(link); err != nil {
				return fmt.Errorf("failed to set link down: %w", err)
			}
		}
	}

	// Restore IP addresses
	if err := sm.restoreIPAddresses(link, snapshot.Addresses); err != nil {
		return fmt.Errorf("failed to restore IP addresses: %w", err)
	}

	// Restore bridge ports if this is a bridge
	if snapshot.Type == "bridge" && len(snapshot.Ports) > 0 {
		if err := sm.restoreBridgePorts(link, snapshot.Ports); err != nil {
			return fmt.Errorf("failed to restore bridge ports: %w", err)
		}
	}

	return nil
}

// restoreIPAddresses restores the IP addresses on an interface.
func (sm *SnapshotManager) restoreIPAddresses(link netlink.Link, snapshotAddrs []string) error {
	// Get current addresses
	currentAddrs, err := sm.netlink.AddrList(link, FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list current addresses: %w", err)
	}

	// Remove addresses not in snapshot
	for _, current := range currentAddrs {
		addrStr := current.IPNet.String()
		if !containsString(snapshotAddrs, addrStr) {
			logger.Info("Removing address",
				logger.Field{Key: "address", Value: addrStr},
				logger.Field{Key: "interface", Value: link.Attrs().Name})
			if err := sm.netlink.AddrDel(link, &current); err != nil {
				logger.Warn("Failed to remove address",
					logger.Field{Key: "address", Value: addrStr},
					logger.Field{Key: "error", Value: err.Error()})
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
			logger.Info("Adding address",
				logger.Field{Key: "address", Value: addrStr},
				logger.Field{Key: "interface", Value: link.Attrs().Name})
			addr, err := netlink.ParseAddr(addrStr)
			if err != nil {
				logger.Warn("Failed to parse address",
					logger.Field{Key: "address", Value: addrStr},
					logger.Field{Key: "error", Value: err.Error()})
				continue
			}

			if err := sm.netlink.AddrAdd(link, addr); err != nil {
				logger.Warn("Failed to add address",
					logger.Field{Key: "address", Value: addrStr},
					logger.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	return nil
}

// restoreBridgePorts restores the port membership of a bridge.
func (sm *SnapshotManager) restoreBridgePorts(bridge netlink.Link, snapshotPorts []string) error {
	// Get current ports
	currentPorts, err := sm.getBridgePorts(bridge.Attrs().Name)
	if err != nil {
		return fmt.Errorf("failed to get current bridge ports: %w", err)
	}

	// Remove ports not in snapshot
	for _, port := range currentPorts {
		if !containsString(snapshotPorts, port) {
			logger.Info("Removing port from bridge",
				logger.Field{Key: "port", Value: port},
				logger.Field{Key: "bridge", Value: bridge.Attrs().Name})
			if portLink, err := sm.netlink.LinkByName(port); err == nil {
				if err := sm.netlink.LinkSetNoMaster(portLink); err != nil {
					logger.Warn("Failed to remove port",
						logger.Field{Key: "port", Value: port},
						logger.Field{Key: "error", Value: err.Error()})
				}
			}
		}
	}

	// Add ports from snapshot
	for _, port := range snapshotPorts {
		if !containsString(currentPorts, port) {
			logger.Info("Adding port to bridge",
				logger.Field{Key: "port", Value: port},
				logger.Field{Key: "bridge", Value: bridge.Attrs().Name})
			if portLink, err := sm.netlink.LinkByName(port); err == nil {
				if err := sm.netlink.LinkSetMaster(portLink, bridge); err != nil {
					logger.Warn("Failed to add port",
						logger.Field{Key: "port", Value: port},
						logger.Field{Key: "error", Value: err.Error()})
				}
			}
		}
	}

	return nil
}

// recreateInterface attempts to recreate an interface that was deleted.
func (sm *SnapshotManager) recreateInterface(snapshot InterfaceSnapshot) error {
	// This is complex and depends on interface type
	// For now, we log a warning and skip recreation
	logger.Warn("Cannot recreate deleted interface",
		logger.Field{Key: "interface", Value: snapshot.Name},
		logger.Field{Key: "type", Value: snapshot.Type})
	logger.Warn("Manual intervention may be required to restore interface",
		logger.Field{Key: "interface", Value: snapshot.Name})
	return nil
}

// rollbackRoutes restores the routing table to the snapshot state.
func (sm *SnapshotManager) rollbackRoutes(snapshotRoutes []RouteSnapshot) error {
	// Get current routes
	currentRoutes, err := sm.netlink.RouteList(nil, FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list current routes: %w", err)
	}

	// Remove routes not in snapshot
	for _, current := range currentRoutes {
		// Skip routes we shouldn't touch (kernel routes, etc.)
		if current.Protocol == 2 { // RTPROT_KERNEL
			continue
		}
		if current.Scope == SCOPE_LINK {
			continue // Skip link-local routes
		}

		if !sm.snapshotRouteExists(current, snapshotRoutes) {
			logger.Info("Removing route",
				logger.Field{Key: "destination", Value: current.Dst},
				logger.Field{Key: "gateway", Value: current.Gw})
			if err := sm.netlink.RouteDel(&current); err != nil {
				logger.Warn("Failed to remove route",
					logger.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	// Add routes from snapshot
	for _, snapshot := range snapshotRoutes {
		if !sm.snapshotRouteInCurrent(snapshot, currentRoutes) {
			logger.Info("Adding route",
				logger.Field{Key: "destination", Value: snapshot.Destination},
				logger.Field{Key: "gateway", Value: snapshot.Gateway})

			route := &netlink.Route{
				Priority: snapshot.Metric,
				Table:    snapshot.Table,
				Scope:    netlink.Scope(snapshot.Scope),
			}

			// Parse destination
			if snapshot.Destination != "default" {
				_, dst, err := net.ParseCIDR(snapshot.Destination)
				if err != nil {
					logger.Warn("Failed to parse destination",
						logger.Field{Key: "destination", Value: snapshot.Destination},
						logger.Field{Key: "error", Value: err.Error()})
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
				if link, err := sm.netlink.LinkByName(snapshot.Device); err == nil {
					route.LinkIndex = link.Attrs().Index
				}
			}

			if err := sm.netlink.RouteAdd(route); err != nil {
				logger.Warn("Failed to add route",
					logger.Field{Key: "error", Value: err.Error()})
			}
		}
	}

	return nil
}

// snapshotRouteExists checks if a netlink.Route exists in the snapshot.
func (sm *SnapshotManager) snapshotRouteExists(route netlink.Route, snapshots []RouteSnapshot) bool {
	for _, snapshot := range snapshots {
		if sm.snapshotRoutesMatch(route, snapshot) {
			return true
		}
	}
	return false
}

// snapshotRouteInCurrent checks if a RouteSnapshot exists in the current routes.
func (sm *SnapshotManager) snapshotRouteInCurrent(snapshot RouteSnapshot, routes []netlink.Route) bool {
	for _, route := range routes {
		if sm.snapshotRoutesMatch(route, snapshot) {
			return true
		}
	}
	return false
}

// snapshotRoutesMatch checks if a netlink.Route matches a RouteSnapshot.
func (sm *SnapshotManager) snapshotRoutesMatch(route netlink.Route, snapshot RouteSnapshot) bool {
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
		if link, err := sm.netlink.LinkByIndex(route.LinkIndex); err == nil {
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

// RestoreNftablesRules restores the nftables ruleset from a JSON dump.
func (sm *SnapshotManager) RestoreNftablesRules(rulesJSON string) error {
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

	if _, err := io.WriteString(tmpFile, rulesJSON); err != nil {
		return fmt.Errorf("failed to write rules to temp file: %w", err)
	}
	tmpFile.Close()

	// Restore rules using nft
	if output, err := sm.cmd.Run("nft", "-j", "-f", tmpFile.Name()); err != nil {
		return fmt.Errorf("failed to restore nftables rules: %w (output: %s)", err, string(output))
	}

	logger.Info("Successfully restored nftables rules")
	return nil
}
