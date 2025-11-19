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
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// Default route manager for backwards compatibility.
var defaultRouteManager = NewDefaultRouteManager()

// ApplyRoutesConfig applies static route configuration.
// For testing, create a RouteManager with injected dependencies.
func ApplyRoutesConfig(config *types.RoutesConfig) error {
	return defaultRouteManager.ApplyRoutesConfig(config)
}

// routesMatch is a public helper for comparing routes
func routesMatch(a, b netlink.Route) bool {
	if (a.Dst == nil) != (b.Dst == nil) {
		return false
	}
	if a.Dst != nil && b.Dst != nil {
		if a.Dst.String() != b.Dst.String() {
			return false
		}
	}

	if !a.Gw.Equal(b.Gw) {
		return false
	}

	if a.LinkIndex != b.LinkIndex && a.LinkIndex != 0 && b.LinkIndex != 0 {
		return false
	}

	if a.Table != b.Table {
		return false
	}

	return true
}
