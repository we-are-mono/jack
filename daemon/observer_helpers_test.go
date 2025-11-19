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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

// TestFindInterfaceByDevice tests finding interfaces by device name
func TestFindInterfaceByDevice(t *testing.T) {
	tests := []struct {
		name          string
		config        *types.InterfacesConfig
		deviceName    string
		expectFound   bool
		expectName    string
		expectDevice  string
	}{
		{
			name: "interface found by Device field",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"lan": {
						Type:    "physical",
						Device:  "eth0",
						Enabled: true,
					},
				},
			},
			deviceName:   "eth0",
			expectFound:  true,
			expectName:   "lan",
			expectDevice: "eth0",
		},
		{
			name: "interface found by DeviceName field",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Type:       "physical",
						DeviceName: "eth1",
						Enabled:    true,
					},
				},
			},
			deviceName:   "eth1",
			expectFound:  true,
			expectName:   "wan",
			expectDevice: "eth1",
		},
		{
			name: "interface not found",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"lan": {
						Type:    "physical",
						Device:  "eth0",
						Enabled: true,
					},
				},
			},
			deviceName:  "eth2",
			expectFound: false,
		},
		{
			name:        "nil config",
			config:      nil,
			deviceName:  "eth0",
			expectFound: false,
		},
		{
			name: "nil interfaces map",
			config: &types.InterfacesConfig{
				Interfaces: nil,
			},
			deviceName:  "eth0",
			expectFound: false,
		},
		{
			name: "empty interfaces map",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{},
			},
			deviceName:  "eth0",
			expectFound: false,
		},
		{
			name: "multiple interfaces, find second",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"lan": {
						Type:    "physical",
						Device:  "eth0",
						Enabled: true,
					},
					"wan": {
						Type:    "physical",
						Device:  "eth1",
						Enabled: true,
					},
				},
			},
			deviceName:   "eth1",
			expectFound:  true,
			expectName:   "wan",
			expectDevice: "eth1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface, name, found := findInterfaceByDevice(tt.config, tt.deviceName)

			assert.Equal(t, tt.expectFound, found)

			if found {
				assert.Equal(t, tt.expectName, name)
				require.NotNil(t, iface, "Expected non-nil interface")
				actualDevice := iface.Device
				if actualDevice == "" {
					actualDevice = iface.DeviceName
				}
				assert.Equal(t, tt.expectDevice, actualDevice)
			} else {
				assert.Nil(t, iface, "Expected nil interface when not found")
				assert.Empty(t, name, "Expected empty name when not found")
			}
		})
	}
}

// TestCompareLinkState tests link state comparison
func TestCompareLinkState(t *testing.T) {
	tests := []struct {
		name        string
		desired     types.Interface
		configName  string
		actualName  string
		isUp        bool
		actualMTU   int
		expectDrift string
	}{
		{
			name: "no drift - interface up and correct MTU",
			desired: types.Interface{
				Enabled: true,
				MTU:     1500,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "",
		},
		{
			name: "no drift - interface down as expected",
			desired: types.Interface{
				Enabled: false,
				MTU:     1500,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        false,
			actualMTU:   1500,
			expectDrift: "",
		},
		{
			name: "drift - interface should be up but is down",
			desired: types.Interface{
				Enabled: true,
				MTU:     1500,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        false,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (lan) is down but should be up",
		},
		{
			name: "drift - interface should be down but is up",
			desired: types.Interface{
				Enabled: false,
				MTU:     1500,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (lan) is up but should be down",
		},
		{
			name: "drift - wrong MTU",
			desired: types.Interface{
				Enabled: true,
				MTU:     9000,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (lan) has MTU 1500 but should have 9000",
		},
		{
			name: "no drift - MTU not specified in config",
			desired: types.Interface{
				Enabled: true,
				MTU:     0, // Not specified
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "",
		},
		{
			name: "multiple drifts - enabled takes precedence",
			desired: types.Interface{
				Enabled: true,
				MTU:     9000,
			},
			configName:  "lan",
			actualName:  "eth0",
			isUp:        false,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (lan) is down but should be up", // First check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drift := compareLinkState(&tt.desired, tt.configName, tt.actualName, tt.isUp, tt.actualMTU)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestCompareIPAddress tests IP address comparison
func TestCompareIPAddress(t *testing.T) {
	tests := []struct {
		name        string
		desired     types.Interface
		configName  string
		linkName    string
		ipAddr      string
		isNew       bool
		expectDrift string
	}{
		{
			name: "no drift - IP matches (no CIDR)",
			desired: types.Interface{
				IPAddr: "192.168.1.1",
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.1",
			isNew:       true,
			expectDrift: "",
		},
		{
			name: "no drift - IP matches (with CIDR)",
			desired: types.Interface{
				IPAddr: "192.168.1.1/24",
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.1/24",
			isNew:       true,
			expectDrift: "",
		},
		{
			name: "no drift - IP matches (CIDR mismatch ignored)",
			desired: types.Interface{
				IPAddr: "192.168.1.1/24",
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.1/16", // Different CIDR, but IP matches
			isNew:       true,
			expectDrift: "",
		},
		{
			name: "drift - wrong IP address",
			desired: types.Interface{
				IPAddr: "192.168.1.1",
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.2",
			isNew:       true,
			expectDrift: "Interface eth0 (lan) has unexpected IP 192.168.1.2 (expected 192.168.1.1)",
		},
		{
			name: "drift - wrong IP with CIDR",
			desired: types.Interface{
				IPAddr: "10.0.0.1/8",
			},
			configName:  "wan",
			linkName:    "eth1",
			ipAddr:      "10.0.0.2/8",
			isNew:       true,
			expectDrift: "Interface eth1 (wan) has unexpected IP 10.0.0.2 (expected 10.0.0.1)",
		},
		{
			name: "no drift - not a new address",
			desired: types.Interface{
				IPAddr: "192.168.1.1",
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.2",
			isNew:       false, // Not new, so drift check skipped
			expectDrift: "",
		},
		{
			name: "no drift - no desired IP",
			desired: types.Interface{
				IPAddr: "", // No IP configured
			},
			configName:  "lan",
			linkName:    "eth0",
			ipAddr:      "192.168.1.1",
			isNew:       true,
			expectDrift: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drift := compareIPAddress(&tt.desired, tt.configName, tt.linkName, tt.ipAddr, tt.isNew)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestCompareRoute tests route comparison
func TestCompareRoute(t *testing.T) {
	tests := []struct {
		name        string
		desired     types.Route
		routeName   string
		actual      RouteData
		action      string
		expectDrift string
	}{
		{
			name: "no drift - exact match",
			desired: types.Route{
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
				Table:       254,
				Enabled:     true,
			},
			routeName: "lan-route",
			actual: RouteData{
				Dst:   "192.168.1.0/24",
				Gw:    "192.168.1.1",
				Table: 254,
			},
			action:      "added",
			expectDrift: "",
		},
		{
			name: "no drift - default route",
			desired: types.Route{
				Destination: "default",
				Gateway:     "192.168.1.1",
				Table:       254,
				Enabled:     true,
			},
			routeName: "default-route",
			actual: RouteData{
				Dst:   "0.0.0.0/0",
				Gw:    "192.168.1.1",
				Table: 254,
			},
			action:      "added",
			expectDrift: "",
		},
		{
			name: "drift - route deleted",
			desired: types.Route{
				Destination: "10.0.0.0/8",
				Gateway:     "192.168.1.1",
				Table:       254,
				Enabled:     true,
			},
			routeName: "vpn-route",
			actual: RouteData{
				Dst:   "10.0.0.0/8",
				Gw:    "192.168.1.1",
				Table: 254,
			},
			action:      "deleted",
			expectDrift: "Route vpn-route (10.0.0.0/8) was deleted externally",
		},
		{
			name: "drift - wrong gateway",
			desired: types.Route{
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
				Table:       254,
				Enabled:     true,
			},
			routeName: "lan-route",
			actual: RouteData{
				Dst:   "192.168.1.0/24",
				Gw:    "192.168.1.254",
				Table: 254,
			},
			action:      "added",
			expectDrift: "Route lan-route (192.168.1.0/24) has gateway 192.168.1.254 but should have 192.168.1.1",
		},
		{
			name: "drift - wrong table",
			desired: types.Route{
				Destination: "10.0.0.0/8",
				Gateway:     "192.168.1.1",
				Table:       100,
				Enabled:     true,
			},
			routeName: "custom-route",
			actual: RouteData{
				Dst:   "10.0.0.0/8",
				Gw:    "192.168.1.1",
				Table: 254,
			},
			action:      "added",
			expectDrift: "Route custom-route (10.0.0.0/8) is in table 254 but should be in table 100",
		},
		{
			name: "no drift - different destination (not our route)",
			desired: types.Route{
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
				Table:       254,
				Enabled:     true,
			},
			routeName: "lan-route",
			actual: RouteData{
				Dst:   "10.0.0.0/8", // Different destination
				Gw:    "192.168.1.1",
				Table: 254,
			},
			action:      "added",
			expectDrift: "", // Not our route, no drift reported
		},
		{
			name: "no drift - gateway not specified in config",
			desired: types.Route{
				Destination: "192.168.1.0/24",
				Gateway:     "", // Not specified
				Table:       254,
				Enabled:     true,
			},
			routeName: "direct-route",
			actual: RouteData{
				Dst:   "192.168.1.0/24",
				Gw:    "192.168.1.1", // Has gateway but config doesn't specify
				Table: 254,
			},
			action:      "added",
			expectDrift: "",
		},
		{
			name: "no drift - table not specified in config (default)",
			desired: types.Route{
				Destination: "192.168.1.0/24",
				Gateway:     "192.168.1.1",
				Table:       0, // Not specified
				Enabled:     true,
			},
			routeName: "lan-route",
			actual: RouteData{
				Dst:   "192.168.1.0/24",
				Gw:    "192.168.1.1",
				Table: 254, // Actual table, but config doesn't specify
			},
			action:      "added",
			expectDrift: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drift := compareRoute(tt.desired, tt.routeName, tt.actual, tt.action)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestRouteDestinationsMatch tests route destination matching
func TestRouteDestinationsMatch(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		desired  string
		expected bool
	}{
		{
			name:     "exact match",
			actual:   "192.168.1.0/24",
			desired:  "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "default keyword vs CIDR",
			actual:   "default",
			desired:  "0.0.0.0/0",
			expected: true,
		},
		{
			name:     "CIDR vs default keyword",
			actual:   "0.0.0.0/0",
			desired:  "default",
			expected: true,
		},
		{
			name:     "both default keywords",
			actual:   "default",
			desired:  "default",
			expected: true,
		},
		{
			name:     "different networks",
			actual:   "192.168.1.0/24",
			desired:  "10.0.0.0/8",
			expected: false,
		},
		{
			name:     "same network different prefix length",
			actual:   "192.168.0.0/16",
			desired:  "192.168.1.0/24",
			expected: false,
		},
		{
			name:     "normalized CIDR",
			actual:   "192.168.1.0/24",
			desired:  "192.168.1.1/24", // Host bit set, but normalizes to same network
			expected: true,
		},
		{
			name:     "invalid CIDR fallback to string comparison",
			actual:   "not-a-cidr",
			desired:  "not-a-cidr",
			expected: true,
		},
		{
			name:     "invalid CIDR no match",
			actual:   "not-a-cidr",
			desired:  "different",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := routeDestinationsMatch(tt.actual, tt.desired)
			assert.Equal(t, tt.expected, result, "routeDestinationsMatch(%q, %q)", tt.actual, tt.desired)
		})
	}
}
