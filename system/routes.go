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

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/types"
)

// ApplyRoutesConfig applies static route configuration
func ApplyRoutesConfig(config *types.RoutesConfig) error {
	logger.Info("Applying static routes configuration")

	if config == nil || len(config.Routes) == 0 {
		logger.Info("No routes to configure")
		return nil
	}

	for routeName, route := range config.Routes {
		if !route.Enabled {
			logger.Info("Route is disabled, skipping",
				logger.Field{Key: "route", Value: routeName})
			continue
		}

		if err := applyRoute(routeName, route); err != nil {
			return fmt.Errorf("failed to apply route %s: %w", routeName, err)
		}
	}

	logger.Info("Successfully configured routes",
		logger.Field{Key: "count", Value: len(config.Routes)})
	return nil
}

func applyRoute(routeName string, route types.Route) error {
	logger.Info("Adding route",
		logger.Field{Key: "name", Value: routeName},
		logger.Field{Key: "destination", Value: route.Destination},
		logger.Field{Key: "gateway", Value: route.Gateway})

	// Parse destination
	var dst *net.IPNet
	if route.Destination == "default" {
		// Default route specified as "default" - convert to 0.0.0.0/0
		_, parsedDst, _ := net.ParseCIDR("0.0.0.0/0")
		dst = parsedDst
	} else {
		_, parsedDst, err := net.ParseCIDR(route.Destination)
		if err != nil {
			return fmt.Errorf("invalid destination %s: %w", route.Destination, err)
		}
		dst = parsedDst
	}

	// Create netlink route
	nlRoute := &netlink.Route{
		Dst:      dst,
		Priority: route.Metric,
		Table:    route.Table,
	}

	// Set default table if not specified (254 = main routing table)
	if nlRoute.Table == 0 {
		nlRoute.Table = 254
	}

	// Parse gateway and interface
	if route.Gateway != "" {
		gw := net.ParseIP(route.Gateway)
		if gw == nil {
			return fmt.Errorf("invalid gateway address: %s", route.Gateway)
		}
		nlRoute.Gw = gw

		// Find the interface that can reach this gateway
		// If interface is explicitly specified, use it; otherwise find it automatically
		if route.Interface != "" {
			link, err := netlink.LinkByName(route.Interface)
			if err != nil {
				return fmt.Errorf("interface %s not found: %w", route.Interface, err)
			}
			nlRoute.LinkIndex = link.Attrs().Index
		} else {
			// Find interface by looking for a directly connected network that contains the gateway
			linkIndex, err := findInterfaceForGateway(gw)
			if err != nil {
				return fmt.Errorf("failed to find interface for gateway %s: %w", route.Gateway, err)
			}
			nlRoute.LinkIndex = linkIndex
		}
	} else if route.Interface != "" {
		// Direct route (no gateway, just interface)
		link, err := netlink.LinkByName(route.Interface)
		if err != nil {
			return fmt.Errorf("interface %s not found: %w", route.Interface, err)
		}
		nlRoute.LinkIndex = link.Attrs().Index
		// For routes without a gateway (interface-only), set scope to LINK
		nlRoute.Scope = netlink.SCOPE_LINK
	} else {
		// Validate: must have either gateway or interface
		return fmt.Errorf("route must specify either gateway or interface")
	}

	// Remove existing identical route first (if it exists)
	existingRoutes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		logger.Warn("Failed to list existing routes",
			logger.Field{Key: "error", Value: err.Error()})
	} else {
		for _, existing := range existingRoutes {
			if routesMatch(existing, *nlRoute) {
				logger.Info("Removing existing route",
					logger.Field{Key: "destination", Value: route.Destination})
				if err := netlink.RouteDel(&existing); err != nil {
					logger.Warn("Failed to remove existing route",
						logger.Field{Key: "error", Value: err.Error()})
				}
			}
		}
	}

	// Add the route
	if err := netlink.RouteAdd(nlRoute); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	logger.Info("Successfully added route",
		logger.Field{Key: "name", Value: routeName},
		logger.Field{Key: "gateway", Value: nlRoute.Gw},
		logger.Field{Key: "link_index", Value: nlRoute.LinkIndex},
		logger.Field{Key: "destination", Value: nlRoute.Dst})
	return nil
}

// findInterfaceForGateway finds the interface that can reach the given gateway
// by checking which interface has an IP address on the same subnet as the gateway
func findInterfaceForGateway(gateway net.IP) (int, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return 0, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, link := range links {
		// Get addresses for this interface
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		// Check if any address on this interface is on the same subnet as the gateway
		for _, addr := range addrs {
			if addr.IPNet.Contains(gateway) {
				return link.Attrs().Index, nil
			}
		}
	}

	return 0, fmt.Errorf("no interface found with network containing gateway %s", gateway.String())
}

// routesMatch checks if two routes are equivalent
func routesMatch(a, b netlink.Route) bool {
	// Compare destination
	if (a.Dst == nil) != (b.Dst == nil) {
		return false
	}
	if a.Dst != nil && b.Dst != nil {
		if a.Dst.String() != b.Dst.String() {
			return false
		}
	}

	// Compare gateway
	if !a.Gw.Equal(b.Gw) {
		return false
	}

	// Compare link index
	if a.LinkIndex != b.LinkIndex && a.LinkIndex != 0 && b.LinkIndex != 0 {
		return false
	}

	// Compare table
	if a.Table != b.Table {
		return false
	}

	return true
}
