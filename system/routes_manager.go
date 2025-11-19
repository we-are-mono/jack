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

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/types"
)

// ApplyRoutesConfig applies static route configuration using the RouteManager.
func (rm *RouteManager) ApplyRoutesConfig(config *types.RoutesConfig) error {
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

		if err := rm.applyRoute(routeName, route); err != nil {
			return fmt.Errorf("failed to apply route %s: %w", routeName, err)
		}
	}

	logger.Info("Successfully configured routes",
		logger.Field{Key: "count", Value: len(config.Routes)})
	return nil
}

func (rm *RouteManager) applyRoute(routeName string, route types.Route) error {
	logger.Info("Adding route",
		logger.Field{Key: "name", Value: routeName},
		logger.Field{Key: "destination", Value: route.Destination},
		logger.Field{Key: "gateway", Value: route.Gateway})

	// Validate route configuration
	if err := route.Validate(); err != nil {
		return fmt.Errorf("invalid route configuration: %w", err)
	}

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
		if route.Interface != "" {
			link, err := rm.netlink.LinkByName(route.Interface)
			if err != nil {
				return fmt.Errorf("interface %s not found: %w", route.Interface, err)
			}
			nlRoute.LinkIndex = link.Attrs().Index
		} else {
			// Find interface by looking for a directly connected network
			linkIndex, err := rm.findInterfaceForGateway(gw)
			if err != nil {
				return fmt.Errorf("failed to find interface for gateway %s: %w", route.Gateway, err)
			}
			nlRoute.LinkIndex = linkIndex
		}
	} else if route.Interface != "" {
		// Direct route (no gateway, just interface)
		link, err := rm.netlink.LinkByName(route.Interface)
		if err != nil {
			return fmt.Errorf("interface %s not found: %w", route.Interface, err)
		}
		nlRoute.LinkIndex = link.Attrs().Index
		nlRoute.Scope = netlink.SCOPE_LINK
	} else {
		return fmt.Errorf("route must specify either gateway or interface")
	}

	// Remove existing identical route first
	existingRoutes, err := rm.netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		logger.Warn("Failed to list existing routes",
			logger.Field{Key: "error", Value: err.Error()})
	} else {
		for _, existing := range existingRoutes {
			if routesMatch(existing, *nlRoute) {
				logger.Info("Removing existing route",
					logger.Field{Key: "destination", Value: route.Destination})
				if err := rm.netlink.RouteDel(&existing); err != nil {
					logger.Warn("Failed to remove existing route",
						logger.Field{Key: "error", Value: err.Error()})
				}
			}
		}
	}

	// Add the route
	if err := rm.netlink.RouteAdd(nlRoute); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	logger.Info("Successfully added route",
		logger.Field{Key: "name", Value: routeName},
		logger.Field{Key: "gateway", Value: nlRoute.Gw},
		logger.Field{Key: "link_index", Value: nlRoute.LinkIndex},
		logger.Field{Key: "destination", Value: nlRoute.Dst})
	return nil
}

