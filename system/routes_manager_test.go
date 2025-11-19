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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// TestApplyRoutesConfig demonstrates testing route configuration without actual routes.
func TestApplyRoutesConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.RoutesConfig
		setupMock   func(*MockNetlinkClient)
		expectError bool
		errorMsg    string
		verifyMock  func(*testing.T, *MockNetlinkClient)
	}{
		{
			name:   "nil config",
			config: nil,
			setupMock: func(m *MockNetlinkClient) {
				// No setup needed
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 0 {
					t.Errorf("Expected no route adds, got %d", m.RouteAddCalls)
				}
			},
		},
		{
			name: "empty routes",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{},
			},
			setupMock: func(m *MockNetlinkClient) {},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 0 {
					t.Errorf("Expected no route adds, got %d", m.RouteAddCalls)
				}
			},
		},
		{
			name: "disabled route",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Enabled:     false,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 0 {
					t.Errorf("Expected no route adds for disabled route, got %d", m.RouteAddCalls)
				}
			},
		},
		{
			name: "successful route with explicit interface",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Metric:      100,
						Table:       254,
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 1 {
					t.Errorf("Expected 1 route add, got %d", m.RouteAddCalls)
				}
				if len(m.Routes) != 1 {
					t.Errorf("Expected 1 route in mock, got %d", len(m.Routes))
				}
			},
		},
		{
			name: "route with gateway but interface not found",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "nonexistent",
						Enabled:     true,
					},
				},
			},
			setupMock:   func(m *MockNetlinkClient) {},
			expectError: true,
			errorMsg:    "interface nonexistent not found",
		},
		{
			name: "default route",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": {
						Destination: "default",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 1 {
					t.Errorf("Expected 1 route add, got %d", m.RouteAddCalls)
				}
				// Verify it's a default route (Dst should be 0.0.0.0/0)
				if len(m.Routes) == 1 {
					route := m.Routes[0]
					if route.Dst != nil && route.Dst.String() != "0.0.0.0/0" {
						t.Errorf("Expected default route 0.0.0.0/0, got %s", route.Dst.String())
					}
				}
			},
		},
		{
			name: "route with auto-detected interface",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						// No interface specified - should auto-detect
						Enabled: true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				link := &netlink.Dummy{LinkAttrs: attrs}
				m.Links["eth0"] = link

				// Add an IP address on eth0 that contains the gateway
				_, ipnet, _ := net.ParseCIDR("10.0.0.10/24")
				m.Addresses["eth0"] = []netlink.Addr{
					{IPNet: ipnet},
				}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 1 {
					t.Errorf("Expected 1 route add, got %d", m.RouteAddCalls)
				}
			},
		},
		{
			name: "route without gateway or interface",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						// No gateway or interface
						Enabled: true,
					},
				},
			},
			setupMock:   func(m *MockNetlinkClient) {},
			expectError: true,
			errorMsg:    "route must specify either gateway or interface",
		},
		{
			name: "interface-only route (direct route)",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Interface:   "eth0",
						// No gateway - direct route
						Enabled: true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 1 {
					t.Errorf("Expected 1 route add, got %d", m.RouteAddCalls)
				}
				// Verify it's a link-scoped route
				if len(m.Routes) == 1 {
					route := m.Routes[0]
					if route.Scope != netlink.SCOPE_LINK {
						t.Errorf("Expected SCOPE_LINK for direct route, got %d", route.Scope)
					}
				}
			},
		},
		{
			name: "negative metric value",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Metric:      -100,
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: true,
			errorMsg:    "metric -100 cannot be negative",
		},
		{
			name: "invalid routing table negative",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Table:       -1,
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: true,
			errorMsg:    "table ID -1 out of valid range",
		},
		{
			name: "routing table exceeds maximum",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Table:       4294967296, // > max uint32
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: true,
			errorMsg:    "table ID 4294967296 out of valid range",
		},
		{
			name: "netlink RouteAdd failure",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
				// Simulate RouteAdd failure
				m.RouteAddError = fmt.Errorf("mock RouteAdd error")
			},
			expectError: true,
			errorMsg:    "failed to add route",
		},
		{
			name: "gateway lookup failure no matching interface",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test-route": {
						Destination: "192.168.1.0/24",
						Gateway:     "172.16.0.1",
						// No interface specified - gateway lookup should fail
						Enabled: true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				// Create interface with non-matching network
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				link := &netlink.Dummy{LinkAttrs: attrs}
				m.Links["eth0"] = link
				_, ipnet, _ := net.ParseCIDR("10.0.0.10/24")
				m.Addresses["eth0"] = []netlink.Addr{{IPNet: ipnet}}
			},
			expectError: true,
			errorMsg:    "no interface found",
		},
		{
			name: "conflicting destination with different parameters",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"route1": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Interface:   "eth0",
						Metric:      100,
						Enabled:     true,
					},
					"route2": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.2",
						Interface:   "eth0",
						Metric:      200,
						Enabled:     true,
					},
				},
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false, // Both routes can coexist with different metrics
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				if m.RouteAddCalls != 2 {
					t.Errorf("Expected 2 route adds, got %d", m.RouteAddCalls)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mockNetlink := NewMockNetlinkClient()
			tt.setupMock(mockNetlink)

			// Create RouteManager
			rm := NewRouteManager(mockNetlink)

			// Execute
			err := rm.ApplyRoutesConfig(tt.config)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mock state if provided
			if tt.verifyMock != nil {
				tt.verifyMock(t, mockNetlink)
			}
		})
	}
}

// TestFindInterfaceForGateway tests gateway interface resolution.
func TestFindInterfaceForGateway(t *testing.T) {
	tests := []struct {
		name          string
		gateway       string
		setupMock     func(*MockNetlinkClient)
		expectError   bool
		expectedIndex int
	}{
		{
			name:    "gateway on directly connected network",
			gateway: "192.168.1.1",
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				link := &netlink.Dummy{LinkAttrs: attrs}
				m.Links["eth0"] = link

				_, ipnet, _ := net.ParseCIDR("192.168.1.10/24")
				m.Addresses["eth0"] = []netlink.Addr{
					{IPNet: ipnet},
				}
			},
			expectError:   false,
			expectedIndex: 1,
		},
		{
			name:    "gateway not on any connected network",
			gateway: "10.0.0.1",
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				link := &netlink.Dummy{LinkAttrs: attrs}
				m.Links["eth0"] = link

				_, ipnet, _ := net.ParseCIDR("192.168.1.10/24")
				m.Addresses["eth0"] = []netlink.Addr{
					{IPNet: ipnet},
				}
			},
			expectError: true,
		},
		{
			name:    "multiple interfaces, gateway on second",
			gateway: "10.0.0.1",
			setupMock: func(m *MockNetlinkClient) {
				// First interface
				attrs1 := netlink.LinkAttrs{Name: "eth0", Index: 1}
				link1 := &netlink.Dummy{LinkAttrs: attrs1}
				m.Links["eth0"] = link1
				_, ipnet1, _ := net.ParseCIDR("192.168.1.10/24")
				m.Addresses["eth0"] = []netlink.Addr{{IPNet: ipnet1}}

				// Second interface - contains gateway
				attrs2 := netlink.LinkAttrs{Name: "eth1", Index: 2}
				link2 := &netlink.Dummy{LinkAttrs: attrs2}
				m.Links["eth1"] = link2
				_, ipnet2, _ := net.ParseCIDR("10.0.0.10/24")
				m.Addresses["eth1"] = []netlink.Addr{{IPNet: ipnet2}}
			},
			expectError:   false,
			expectedIndex: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNetlink := NewMockNetlinkClient()
			tt.setupMock(mockNetlink)

			rm := NewRouteManager(mockNetlink)

			gw := net.ParseIP(tt.gateway)
			index, err := rm.findInterfaceForGateway(gw)

			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedIndex, index)
			}
		})
	}
}

// TestRouteTableDefaults tests that routes use the main table by default.
func TestRouteTableDefaults(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}

	rm := NewRouteManager(mockNetlink)

	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"test-route": {
				Destination: "192.168.1.0/24",
				Gateway:     "10.0.0.1",
				Interface:   "eth0",
				// Table not specified - should default to 254 (main)
				Enabled: true,
			},
		},
	}

	err := rm.ApplyRoutesConfig(config)
	require.NoError(t, err)

	require.Len(t, mockNetlink.Routes, 1, "Expected 1 route")
	assert.Equal(t, 254, mockNetlink.Routes[0].Table, "Expected table 254 (main)")
}

// TestInvalidDestination tests error handling for invalid destinations.
func TestInvalidDestination(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}

	rm := NewRouteManager(mockNetlink)

	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"test-route": {
				Destination: "invalid-cidr",
				Gateway:     "10.0.0.1",
				Interface:   "eth0",
				Enabled:     true,
			},
		},
	}

	err := rm.ApplyRoutesConfig(config)
	require.Error(t, err, "Expected error for invalid destination")

	// Should not have added any routes
	assert.Empty(t, mockNetlink.Routes, "Expected 0 routes after error")
}

// TestInvalidGateway tests error handling for invalid gateway IPs.
func TestInvalidGateway(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}

	rm := NewRouteManager(mockNetlink)

	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"test-route": {
				Destination: "192.168.1.0/24",
				Gateway:     "invalid-ip",
				Interface:   "eth0",
				Enabled:     true,
			},
		},
	}

	err := rm.ApplyRoutesConfig(config)
	require.Error(t, err, "Expected error for invalid gateway")
}
