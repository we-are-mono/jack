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

package daemon

import (
	"fmt"
	"strings"

	"github.com/we-are-mono/jack/types"
)

// findInterfaceByDevice finds an interface in config by device name.
// Returns the interface config, config name, and found boolean.
// This eliminates code duplication between checkLinkDrift and checkAddressDrift.
func findInterfaceByDevice(config *types.InterfacesConfig, deviceName string) (*types.Interface, string, bool) {
	if config == nil || config.Interfaces == nil {
		return nil, "", false
	}

	for name, iface := range config.Interfaces {
		if iface.DeviceName == deviceName || iface.Device == deviceName {
			return &iface, name, true
		}
	}

	return nil, "", false
}

// compareLinkState compares desired vs actual link state.
// Returns drift message if mismatch detected, empty string otherwise.
func compareLinkState(desired *types.Interface, configName string, actualName string, isUp bool, actualMTU int) string {
	// Check if interface should be enabled
	if desired.Enabled && !isUp {
		return fmt.Sprintf("Interface %s (%s) is down but should be up", actualName, configName)
	}
	if !desired.Enabled && isUp {
		return fmt.Sprintf("Interface %s (%s) is up but should be down", actualName, configName)
	}

	// Check MTU if specified
	if desired.MTU > 0 && actualMTU != desired.MTU {
		return fmt.Sprintf("Interface %s (%s) has MTU %d but should have %d", actualName, configName, actualMTU, desired.MTU)
	}

	return "" // No drift detected
}

// compareIPAddress compares desired vs actual IP address.
// Returns drift message if mismatch detected, empty string otherwise.
func compareIPAddress(desired *types.Interface, configName string, linkName string, ipAddr string, isNew bool) string {
	// Check if the IP address matches Jack's configuration
	if desired.IPAddr != "" {
		// Strip CIDR suffix from both for comparison (e.g., "10.0.0.1/24" -> "10.0.0.1")
		desiredIP := strings.Split(desired.IPAddr, "/")[0]
		actualIP := strings.Split(ipAddr, "/")[0]

		if isNew && actualIP != desiredIP {
			return fmt.Sprintf("Interface %s (%s) has unexpected IP %s (expected %s)", linkName, configName, actualIP, desiredIP)
		}
	}

	return ""
}

// RouteData is a pure data structure extracted from netlink.Route.
// This allows route comparison logic to be tested without netlink dependencies.
type RouteData struct {
	Dst   string // "default" or CIDR notation
	Gw    string // Gateway IP (may be empty)
	Table int    // Routing table number
}

// compareRoute compares desired route vs actual route.
// Returns drift message if mismatch detected, empty string otherwise.
func compareRoute(desiredRoute types.Route, routeName string, actualRoute RouteData, action string) string {
	// Normalize destination (support "default" keyword)
	desiredDst := desiredRoute.Destination
	if desiredDst == "default" {
		desiredDst = "0.0.0.0/0"
	}

	// Compare destination
	if !routeDestinationsMatch(actualRoute.Dst, desiredDst) {
		return "" // Not the same route, no drift for this one
	}

	// This is a Jack-managed route - check for drift
	if action == "deleted" {
		return fmt.Sprintf("Route %s (%s) was deleted externally", routeName, desiredDst)
	}

	// Compare gateway if specified
	if desiredRoute.Gateway != "" && desiredRoute.Gateway != actualRoute.Gw {
		return fmt.Sprintf("Route %s (%s) has gateway %s but should have %s", routeName, desiredDst, actualRoute.Gw, desiredRoute.Gateway)
	}

	// Compare table if specified
	if desiredRoute.Table > 0 && actualRoute.Table != desiredRoute.Table {
		return fmt.Sprintf("Route %s (%s) is in table %d but should be in table %d", routeName, desiredDst, actualRoute.Table, desiredRoute.Table)
	}

	// Route matches desired config
	return ""
}

// Note: routeDestinationsMatch already exists in observer.go and is used by these helpers
