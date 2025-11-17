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

//go:build integration
// +build integration

package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestStaticRouteCreation tests creating a static route
func TestStaticRouteCreation(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create interface for routing
	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface first
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
			Netmask:  "255.255.255.0",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Configure route
	routes := map[string]types.Route{
		"test-route": {
			Destination: "192.168.0.0/24",
			Gateway:     "10.0.0.254",
			Interface:   eth0,
			Metric:      100,
			Enabled:     true,
		},
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Verify route was created
	netlinkRoutes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	// Find our route
	var foundRoute *netlink.Route
	_, destNet, _ := net.ParseCIDR("192.168.0.0/24")
	for i := range netlinkRoutes {
		if netlinkRoutes[i].Dst != nil && netlinkRoutes[i].Dst.String() == destNet.String() {
			foundRoute = &netlinkRoutes[i]
			break
		}
	}

	require.NotNil(t, foundRoute, "route should exist")
	assert.Equal(t, "10.0.0.254", foundRoute.Gw.String(), "gateway should match")
	assert.Equal(t, 100, foundRoute.Priority, "metric should match")
}

// TestDefaultRouteCreation tests creating a default route
func TestDefaultRouteCreation(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
			Netmask:  "255.255.255.0",
			Gateway:  "10.0.0.254",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify default route was created
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	// Find default route
	var foundRoute *netlink.Route
	for i := range routes {
		// Default route can be either nil or 0.0.0.0/0 depending on kernel/netlink version
		if routes[i].Dst == nil || (routes[i].Dst != nil && routes[i].Dst.String() == "0.0.0.0/0") {
			foundRoute = &routes[i]
			break
		}
	}

	require.NotNil(t, foundRoute, "default route should exist")
	assert.Equal(t, "10.0.0.254", foundRoute.Gw.String(), "gateway should match")
}

// TestMultipleRoutes tests creating multiple static routes
func TestMultipleRoutes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")
	eth1 := harness.CreateDummyInterface("eth1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interfaces
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
			Netmask:  "255.255.255.0",
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.0.1",
			Netmask:  "255.255.255.0",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Configure multiple routes
	routes := map[string]types.Route{
		"route1": {
			Destination: "192.168.0.0/24",
			Gateway:     "10.0.0.254",
			Interface:   eth0,
			Metric:      100,
			Enabled:     true,
		},
		"route2": {
			Destination: "192.168.1.0/24",
			Gateway:     "10.0.0.254",
			Interface:   eth0,
			Metric:      110,
			Enabled:     true,
		},
		"route3": {
			Destination: "192.168.2.0/24",
			Gateway:     "10.1.0.254",
			Interface:   eth1,
			Metric:      100,
			Enabled:     true,
		},
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify all routes were created
	netlinkRoutes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	expectedRoutes := map[string]string{
		"192.168.0.0/24": "10.0.0.254",
		"192.168.1.0/24": "10.0.0.254",
		"192.168.2.0/24": "10.1.0.254",
	}

	foundCount := 0
	for destCIDR, expectedGW := range expectedRoutes {
		_, destNet, _ := net.ParseCIDR(destCIDR)
		for i := range netlinkRoutes {
			if netlinkRoutes[i].Dst != nil && netlinkRoutes[i].Dst.String() == destNet.String() {
				assert.Equal(t, expectedGW, netlinkRoutes[i].Gw.String(), "gateway for %s should match", destCIDR)
				foundCount++
				break
			}
		}
	}

	assert.Equal(t, 3, foundCount, "all routes should be found")
}

// TestRouteDisable tests disabling a route
func TestRouteDisable(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
			Netmask:  "255.255.255.0",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Configure route
	routes := map[string]types.Route{
		"test-route": {
			Destination: "192.168.0.0/24",
			Gateway:     "10.0.0.254",
			Interface:   eth0,
			Enabled:     true,
		},
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify route exists
	routeList, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	_, destNet, _ := net.ParseCIDR("192.168.0.0/24")
	routeExists := false
	for i := range routeList {
		if routeList[i].Dst != nil && routeList[i].Dst.String() == destNet.String() {
			routeExists = true
			break
		}
	}
	assert.True(t, routeExists, "route should exist initially")

	// Disable route
	routes["test-route"] = types.Route{
		Destination: "192.168.0.0/24",
		Gateway:     "10.0.0.254",
		Interface:   "eth0",
		Enabled:     false,
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify route was removed
	routeList, err = netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	routeExists = false
	for i := range routeList {
		if routeList[i].Dst != nil && routeList[i].Dst.String() == destNet.String() {
			routeExists = true
			break
		}
	}
	assert.False(t, routeExists, "route should be removed when disabled")
}

// TestRouteIdempotency tests that reapplying route config is idempotent
func TestRouteIdempotency(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
			Netmask:  "255.255.255.0",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Configure route
	routes := map[string]types.Route{
		"test-route": {
			Destination: "192.168.0.0/24",
			Gateway:     "10.0.0.254",
			Interface:   eth0,
			Metric:      100,
			Enabled:     true,
		},
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "first apply should succeed")

	// Apply again (idempotency test)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "second apply should succeed (idempotent)")

	// Verify route still exists and is correct
	routeList, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)

	_, destNet, _ := net.ParseCIDR("192.168.0.0/24")
	var foundRoute *netlink.Route
	for i := range routeList {
		if routeList[i].Dst != nil && routeList[i].Dst.String() == destNet.String() {
			foundRoute = &routeList[i]
			break
		}
	}

	require.NotNil(t, foundRoute, "route should still exist")
	assert.Equal(t, "10.0.0.254", foundRoute.Gw.String())
	assert.Equal(t, 100, foundRoute.Priority)
}
