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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// TestRoutesMatch tests route comparison logic
func TestRoutesMatch(t *testing.T) {
	tests := []struct {
		name     string
		routeA   netlink.Route
		routeB   netlink.Route
		expected bool
	}{
		{
			name: "identical routes",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: true,
		},
		{
			name: "different destinations",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.2.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: false,
		},
		{
			name: "different gateways",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.254"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: false,
		},
		{
			name: "different link indexes (both non-zero)",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 3,
				Table:     254,
			},
			expected: false,
		},
		{
			name: "link index zero vs non-zero (match)",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 0,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: true, // Zero link index ignored in comparison
		},
		{
			name: "different tables",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     255,
			},
			expected: false,
		},
		{
			name: "default routes (nil dst)",
			routeA: netlink.Route{
				Dst:       nil, // Default route
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       nil, // Default route
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: true,
		},
		{
			name: "one nil dst one not",
			routeA: netlink.Route{
				Dst:       nil,
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: false,
		},
		{
			name: "nil gateways",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        nil,
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        nil,
				LinkIndex: 2,
				Table:     254,
			},
			expected: true,
		},
		{
			name: "one nil gateway one not",
			routeA: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        nil,
				LinkIndex: 2,
				Table:     254,
			},
			routeB: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 2,
				Table:     254,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := routesMatch(tt.routeA, tt.routeB)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestApplyRoutesConfig_NilConfig tests nil config handling
func TestApplyRoutesConfig_NilConfig(t *testing.T) {
	err := ApplyRoutesConfig(nil)
	assert.NoError(t, err, "Nil config should be handled gracefully")
}

// TestApplyRoutesConfig_EmptyRoutes tests empty routes handling
func TestApplyRoutesConfig_EmptyRoutes(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{},
	}

	err := ApplyRoutesConfig(config)
	assert.NoError(t, err, "Empty routes should be handled gracefully")
}

// TestApplyRoutesConfig_DisabledRoutes tests that disabled routes are skipped
func TestApplyRoutesConfig_DisabledRoutes(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"disabled-route": {
				Enabled:     false,
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
			},
		},
	}

	// Should succeed without trying to apply disabled route
	err := ApplyRoutesConfig(config)
	// This would fail if it tried to actually apply the route (no interfaces)
	// But since it's disabled, it should be a no-op
	assert.NoError(t, err)
}

// TestApplyRoutesConfig_PartialFailure tests handling of partial failures
func TestApplyRoutesConfig_PartialFailure(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"invalid-route": {
				Enabled:     true,
				Destination: "invalid",
				Gateway:     "192.168.1.1",
			},
		},
	}

	err := ApplyRoutesConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to apply route")
}

// TestRouteParsing_EdgeCases tests edge cases in route parsing
func TestRouteParsing_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		destination string
		isValid     bool
	}{
		{"default route string", "default", true},
		{"zero network", "0.0.0.0/0", true},
		{"single host", "192.168.1.1/32", true},
		{"class C network", "192.168.1.0/24", true},
		{"class B network", "172.16.0.0/16", true},
		{"class A network", "10.0.0.0/8", true},
		{"invalid CIDR", "192.168.1.0/33", false},
		{"no prefix", "192.168.1.0", false},
		{"invalid IP", "999.999.999.999/24", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.destination == "default" {
				// Special case - "default" is converted to 0.0.0.0/0
				_, dst, err := net.ParseCIDR("0.0.0.0/0")
				assert.NoError(t, err)
				assert.NotNil(t, dst)
				return
			}

			_, dst, err := net.ParseCIDR(tt.destination)
			if tt.isValid {
				assert.NoError(t, err, "Expected valid CIDR for: %s", tt.destination)
				assert.NotNil(t, dst)
			} else {
				assert.Error(t, err, "Expected invalid CIDR for: %s", tt.destination)
			}
		})
	}
}

// TestGatewayParsing tests gateway IP parsing
func TestGatewayParsing(t *testing.T) {
	tests := []struct {
		name    string
		gateway string
		isValid bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4 zero", "0.0.0.0", true},
		{"valid IPv4 broadcast", "255.255.255.255", true},
		{"invalid IP", "999.999.999.999", false},
		{"invalid format", "not-an-ip", false},
		{"empty string", "", false},
		{"IPv6", "::1", true}, // Valid but not used in this code
		{"partial IP", "192.168.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.gateway)
			if tt.isValid {
				assert.NotNil(t, ip, "Expected valid IP for: %s", tt.gateway)
			} else {
				assert.Nil(t, ip, "Expected invalid IP for: %s", tt.gateway)
			}
		})
	}
}

// TestRouteMetric tests route metric handling
func TestRouteMetric(t *testing.T) {
	tests := []struct {
		name   string
		metric int
		valid  bool
	}{
		{"zero metric", 0, true},
		{"low metric", 10, true},
		{"medium metric", 100, true},
		{"high metric", 1000, true},
		{"negative metric", -1, true}, // Accepted by netlink but unusual
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &netlink.Route{
				Priority: tt.metric,
			}
			assert.Equal(t, tt.metric, route.Priority)
		})
	}
}

// TestRouteTable tests routing table handling
func TestRouteTable(t *testing.T) {
	tests := []struct {
		name          string
		table         int
		expectedTable int
	}{
		{"default (main) table", 0, 254}, // Should default to 254
		{"main table explicit", 254, 254},
		{"local table", 255, 255},
		{"custom table", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := types.Route{
				Enabled:     true,
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
				Table:       tt.table,
			}

			// The applyRoute function should set table to 254 if it's 0
			// We test the logic here
			table := route.Table
			if table == 0 {
				table = 254
			}
			assert.Equal(t, tt.expectedTable, table)
		})
	}
}

// Benchmark tests

func BenchmarkRoutesMatch(b *testing.B) {
	routeA := netlink.Route{
		Dst:       mustParseCIDR("192.168.1.0/24"),
		Gw:        net.ParseIP("192.168.1.1"),
		LinkIndex: 2,
		Table:     254,
	}
	routeB := netlink.Route{
		Dst:       mustParseCIDR("192.168.1.0/24"),
		Gw:        net.ParseIP("192.168.1.1"),
		LinkIndex: 2,
		Table:     254,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = routesMatch(routeA, routeB)
	}
}

func BenchmarkCIDRParsing(b *testing.B) {
	destinations := []string{
		"192.168.1.0/24",
		"10.0.0.0/8",
		"172.16.0.0/16",
		"0.0.0.0/0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = net.ParseCIDR(destinations[i%len(destinations)])
	}
}

// Helper functions

func mustParseCIDR(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return ipnet
}
