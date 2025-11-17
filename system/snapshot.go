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
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
)

// SystemSnapshot captures the complete state of the system before apply operations.
// This allows rollback to a known good state if apply fails.
type SystemSnapshot struct {
	Timestamp    time.Time                  `json:"timestamp"`
	CheckpointID string                     `json:"checkpoint_id"`
	IPForwarding bool                       `json:"ip_forwarding"`
	Interfaces   map[string]InterfaceSnapshot `json:"interfaces"`
	Routes       []RouteSnapshot            `json:"routes"`
	NftablesRules string                    `json:"nftables_rules,omitempty"`
}

// InterfaceSnapshot captures the state of a single network interface.
type InterfaceSnapshot struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "physical", "bridge", "vlan", "wireguard"
	Existed      bool     `json:"existed"`
	MTU          int      `json:"mtu"`
	State        string   `json:"state"` // "up" or "down"
	Addresses    []string `json:"addresses"`
	Gateway      string   `json:"gateway,omitempty"`
	Metric       int      `json:"metric,omitempty"`

	// Bridge-specific fields
	Ports        []string `json:"ports,omitempty"`

	// VLAN-specific fields
	VLANId       int      `json:"vlan_id,omitempty"`
	ParentDevice string   `json:"parent_device,omitempty"`
}

// RouteSnapshot captures the state of a single route.
type RouteSnapshot struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Device      string `json:"device,omitempty"`
	Metric      int    `json:"metric"`
	Table       int    `json:"table"`
	Scope       int    `json:"scope"`
}

// CaptureSystemSnapshot captures the current state of the system.
// This includes IP forwarding, all network interfaces, routes, and firewall rules.
func CaptureSystemSnapshot() (*SystemSnapshot, error) {
	snapshot := &SystemSnapshot{
		Timestamp:    time.Now(),
		CheckpointID: fmt.Sprintf("auto-%d", time.Now().Unix()),
		Interfaces:   make(map[string]InterfaceSnapshot),
	}

	// Capture IP forwarding setting
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		snapshot.IPForwarding = strings.TrimSpace(string(data)) == "1"
	}

	// Capture all network interfaces
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, link := range links {
		// Skip loopback interface
		if link.Attrs().Name == "lo" {
			continue
		}

		ifaceSnapshot, err := captureInterfaceState(link)
		if err != nil {
			// Log warning but continue with other interfaces
			fmt.Fprintf(os.Stderr, "[WARN] Failed to capture state for %s: %v\n", link.Attrs().Name, err)
			continue
		}
		snapshot.Interfaces[link.Attrs().Name] = ifaceSnapshot
	}

	// Capture routing table
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
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
			if link, err := netlink.LinkByIndex(route.LinkIndex); err == nil {
				routeSnapshot.Device = link.Attrs().Name
			}
		}

		snapshot.Routes = append(snapshot.Routes, routeSnapshot)
	}

	// Capture nftables ruleset (if jack table exists)
	// This is best-effort; if nft command fails, we continue
	cmd := exec.Command("nft", "-j", "list", "table", "inet", "jack")
	if output, err := cmd.Output(); err == nil {
		snapshot.NftablesRules = string(output)
	}

	return snapshot, nil
}

// captureInterfaceState captures the state of a single interface.
func captureInterfaceState(link netlink.Link) (InterfaceSnapshot, error) {
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
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return snapshot, fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		snapshot.Addresses = append(snapshot.Addresses, addr.IPNet.String())
	}

	// Capture default gateway (if any)
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
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
		ports, err := getBridgePorts(link.Attrs().Name)
		if err == nil {
			snapshot.Ports = ports
		}

	case "vlan":
		snapshot.Type = "vlan"
		if vlan, ok := link.(*netlink.Vlan); ok {
			snapshot.VLANId = vlan.VlanId
			// Get parent name
			if parent, err := netlink.LinkByIndex(vlan.ParentIndex); err == nil {
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
