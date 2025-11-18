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
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

func TestNetworkObserver_MarkChange(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	// Initially, no recent change
	if observer.isRecentChange() {
		t.Error("Expected no recent change on new observer")
	}

	// Mark a change
	observer.MarkChange()

	// Should now have a recent change
	if !observer.isRecentChange() {
		t.Error("Expected recent change after MarkChange()")
	}

	// Wait for debounce window to expire
	time.Sleep(1100 * time.Millisecond)

	// Should no longer have a recent change
	if observer.isRecentChange() {
		t.Error("Expected no recent change after debounce window expired")
	}
}

func TestNetworkObserver_NewObserver(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	if observer.server != server {
		t.Error("Observer server reference incorrect")
	}

	if observer.linkCh == nil {
		t.Error("linkCh not initialized")
	}

	if observer.addrCh == nil {
		t.Error("addrCh not initialized")
	}

	if observer.routeCh == nil {
		t.Error("routeCh not initialized")
	}
}

func TestNetworkObserver_isRecentChange_ThreadSafety(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	// Run concurrent reads and writes to test mutex safety
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			observer.MarkChange()
			time.Sleep(time.Millisecond)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = observer.isRecentChange()
		time.Sleep(time.Millisecond)
	}

	<-done // Wait for writes to complete
}

// TestCheckLinkDrift tests interface state drift detection
func TestCheckLinkDrift(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.InterfacesConfig
		linkName    string
		isUp        bool
		actualMTU   int
		expectDrift string
	}{
		{
			name: "interface should be up but is down",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    true,
					},
				},
			},
			linkName:    "eth0",
			isUp:        false,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (wan) is down but should be up",
		},
		{
			name: "interface should be down but is up",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    false,
					},
				},
			},
			linkName:    "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (wan) is up but should be down",
		},
		{
			name: "MTU mismatch",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    true,
						MTU:        9000,
					},
				},
			},
			linkName:    "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "Interface eth0 (wan) has MTU 1500 but should have 9000",
		},
		{
			name: "no drift - interface up and MTU matches",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    true,
						MTU:        1500,
					},
				},
			},
			linkName:    "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "",
		},
		{
			name: "no drift - MTU not specified in config",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    true,
						MTU:        0, // Not specified
					},
				},
			},
			linkName:    "eth0",
			isUp:        true,
			actualMTU:   1500,
			expectDrift: "",
		},
		{
			name: "interface not managed by Jack",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						Enabled:    true,
					},
				},
			},
			linkName:    "eth1", // Different interface
			isUp:        false,
			actualMTU:   1500,
			expectDrift: "", // Should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState()
			state.LoadCommittedInterfaces(tt.config)

			server := &Server{
				state: state,
			}

			observer := &NetworkObserver{
				server: server,
			}

			drift := observer.checkLinkDrift(tt.linkName, tt.isUp, tt.actualMTU)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestCheckAddressDrift tests IP address drift detection
func TestCheckAddressDrift(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.InterfacesConfig
		linkName    string
		ipAddr      string
		isNew       bool
		expectDrift string
	}{
		{
			name: "unexpected IP address added",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						IPAddr:     "10.0.0.1",
					},
				},
			},
			linkName:    "eth0",
			ipAddr:      "10.0.0.2/24",
			isNew:       true,
			expectDrift: "Interface eth0 (wan) has unexpected IP 10.0.0.2 (expected 10.0.0.1)",
		},
		{
			name: "correct IP address added",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						IPAddr:     "10.0.0.1",
					},
				},
			},
			linkName:    "eth0",
			ipAddr:      "10.0.0.1/24",
			isNew:       true,
			expectDrift: "",
		},
		{
			name: "IP address with CIDR matches",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						IPAddr:     "10.0.0.1/24", // Config has CIDR
					},
				},
			},
			linkName:    "eth0",
			ipAddr:      "10.0.0.1/24",
			isNew:       true,
			expectDrift: "",
		},
		{
			name: "interface not managed by Jack",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						IPAddr:     "10.0.0.1",
					},
				},
			},
			linkName:    "eth1", // Different interface
			ipAddr:      "10.0.0.2/24",
			isNew:       true,
			expectDrift: "", // Should be ignored
		},
		{
			name: "no IP configured in Jack",
			config: &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"wan": {
						Device:     "eth0",
						DeviceName: "eth0",
						IPAddr:     "", // No IP configured
					},
				},
			},
			linkName:    "eth0",
			ipAddr:      "10.0.0.1/24",
			isNew:       true,
			expectDrift: "", // Should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState()
			state.LoadCommittedInterfaces(tt.config)

			server := &Server{
				state: state,
			}

			observer := &NetworkObserver{
				server: server,
			}

			drift := observer.checkAddressDrift(tt.linkName, tt.ipAddr, tt.isNew)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestCheckRouteDrift tests route drift detection
func TestCheckRouteDrift(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.RoutesConfig
		route       *netlink.Route
		action      string
		expectDrift string
	}{
		{
			name: "Jack-managed route deleted externally",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": {
						Destination: "default",
						Gateway:     "10.0.0.1",
						Enabled:     true,
					},
				},
			},
			route: &netlink.Route{
				Dst: nil, // Default route
				Gw:  net.ParseIP("10.0.0.1"),
			},
			action:      "deleted",
			expectDrift: "Route default (0.0.0.0/0) was deleted externally",
		},
		{
			name: "gateway changed",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": {
						Destination: "default",
						Gateway:     "10.0.0.1",
						Enabled:     true,
					},
				},
			},
			route: &netlink.Route{
				Dst: nil,
				Gw:  net.ParseIP("10.0.0.2"), // Different gateway
			},
			action:      "added",
			expectDrift: "Route default (0.0.0.0/0) has gateway 10.0.0.2 but should have 10.0.0.1",
		},
		{
			name: "route table changed",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"custom": {
						Destination: "192.168.1.0/24",
						Gateway:     "10.0.0.1",
						Table:       100,
						Enabled:     true,
					},
				},
			},
			route: &netlink.Route{
				Dst:   &net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)},
				Gw:    net.ParseIP("10.0.0.1"),
				Table: 200, // Different table
			},
			action:      "added",
			expectDrift: "Route custom (192.168.1.0/24) is in table 200 but should be in table 100",
		},
		{
			name: "correct route added",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": {
						Destination: "default",
						Gateway:     "10.0.0.1",
						Enabled:     true,
					},
				},
			},
			route: &netlink.Route{
				Dst: nil,
				Gw:  net.ParseIP("10.0.0.1"),
			},
			action:      "added",
			expectDrift: "",
		},
		{
			name: "route not managed by Jack",
			config: &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": {
						Destination: "default",
						Gateway:     "10.0.0.1",
						Enabled:     true,
					},
				},
			},
			route: &netlink.Route{
				Dst: &net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)},
				Gw:  net.ParseIP("192.168.1.1"),
			},
			action:      "deleted",
			expectDrift: "", // Not managed by Jack, should be ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := NewState()
			state.LoadCommittedRoutes(tt.config)

			server := &Server{
				state: state,
			}

			observer := &NetworkObserver{
				server: server,
			}

			drift := observer.checkRouteDrift(tt.route, tt.action)
			assert.Equal(t, tt.expectDrift, drift)
		})
	}
}

// TestMaybeReconcile tests rate limiting logic
func TestMaybeReconcile(t *testing.T) {
	tests := []struct {
		name                 string
		autoReconcile        bool
		reconcileInterval    time.Duration
		timeSinceLastApply   time.Duration
		expectReconciliation bool
	}{
		{
			name:                 "auto-reconcile disabled",
			autoReconcile:        false,
			reconcileInterval:    60 * time.Second,
			timeSinceLastApply:   120 * time.Second,
			expectReconciliation: false,
		},
		{
			name:                 "rate limited - too soon",
			autoReconcile:        true,
			reconcileInterval:    60 * time.Second,
			timeSinceLastApply:   30 * time.Second,
			expectReconciliation: false,
		},
		{
			name:                 "allowed - enough time passed",
			autoReconcile:        true,
			reconcileInterval:    60 * time.Second,
			timeSinceLastApply:   120 * time.Second,
			expectReconciliation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observer := &NetworkObserver{
				autoReconcile:     tt.autoReconcile,
				reconcileInterval: tt.reconcileInterval,
				lastReconcile:     time.Now().Add(-tt.timeSinceLastApply),
				server:            &Server{state: NewState()},
			}

			lastReconcileBefore := observer.lastReconcile
			observer.maybeReconcile()

			// If reconciliation should happen, lastReconcile should be updated
			if tt.expectReconciliation {
				assert.True(t, observer.lastReconcile.After(lastReconcileBefore),
					"lastReconcile should be updated when reconciliation is triggered")
			} else {
				assert.Equal(t, lastReconcileBefore, observer.lastReconcile,
					"lastReconcile should not change when reconciliation is blocked")
			}
		})
	}
}

// TestRouteDestinationsMatch tests route destination comparison logic
func TestRouteDestinationsMatch(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		desired  string
		expected bool
	}{
		{
			name:     "both default",
			actual:   "default",
			desired:  "default",
			expected: true,
		},
		{
			name:     "actual default, desired 0.0.0.0/0",
			actual:   "default",
			desired:  "0.0.0.0/0",
			expected: true,
		},
		{
			name:     "actual 0.0.0.0/0, desired default",
			actual:   "0.0.0.0/0",
			desired:  "default",
			expected: true,
		},
		{
			name:     "same CIDR",
			actual:   "192.168.1.0/24",
			desired:  "192.168.1.0/24",
			expected: true,
		},
		{
			name:     "different networks",
			actual:   "192.168.1.0/24",
			desired:  "192.168.2.0/24",
			expected: false,
		},
		{
			name:     "same network, different notation",
			actual:   "192.168.1.0/24",
			desired:  "192.168.1.128/24", // Will normalize to 192.168.1.0/24
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := routeDestinationsMatch(tt.actual, tt.desired)
			assert.Equal(t, tt.expected, result)
		})
	}
}

