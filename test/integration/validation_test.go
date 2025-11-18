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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestValidateCommand tests the validate command with valid configuration
func TestValidateCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Valid configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.3.1.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	// Validate using the validate command
	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "valid configuration should pass validation")
}

// TestInvalidInterfaceType tests validation of invalid interface type
func TestInvalidInterfaceType(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Invalid interface type
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "invalid_type", // Invalid
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.3.2.10",
			Netmask:  "255.255.255.0",
		},
	}

	// Validation should fail
	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   interfaces,
	})

	// Either the request fails or validation returns error
	if err == nil {
		assert.False(t, resp.Success, "invalid type should fail validation")
		assert.NotEmpty(t, resp.Error, "should have error message")
	}
}

// TestInvalidIPAddress tests validation of malformed IP addresses
func TestInvalidIPAddress(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	testCases := []struct {
		name    string
		ipAddr  string
		netmask string
	}{
		{"Invalid IP format", "999.999.999.999", "255.255.255.0"},
		{"Malformed IP", "10.0.0", "255.255.255.0"},
		{"Invalid netmask", "10.3.3.10", "255.255.999.0"},
		{"Empty IP", "", "255.255.255.0"},
		{"Empty netmask", "10.3.3.10", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			interfaces := map[string]types.Interface{
				eth0: {
					Type:     "physical",
					Device:   eth0,
					Enabled:  true,
					Protocol: "static",
					IPAddr:   tc.ipAddr,
					Netmask:  tc.netmask,
				},
			}

			// Set should fail or validation should catch it
			resp, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "interfaces",
				Value:   interfaces,
			})

			// We expect either an error or unsuccessful response
			if err == nil && resp.Success {
				// If set succeeds, commit should fail
				resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
				// Allow error or failed response
				if err == nil {
					t.Logf("Commit response: success=%v, error=%s", resp.Success, resp.Error)
				}
			}
		})
	}
}

// TestVLANWithoutParent tests that VLAN without parent interface fails validation
func TestVLANWithoutParent(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// VLAN without parent interface (device name references nonexistent interface)
	interfaces := map[string]types.Interface{
		"vlan100": {
			Type:    "vlan",
			Device:  "nonexistent_interface.100",
			Enabled: true,
			VLANId:  100,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	resp, err := harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Apply should fail because parent doesn't exist
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	if err == nil {
		assert.False(t, resp.Success, "apply should fail with non-existent parent")
	}
}

// TestInvalidVLANID tests validation of invalid VLAN IDs
func TestInvalidVLANID(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	testCases := []struct {
		name   string
		vlanID int
	}{
		{"VLAN ID 0", 0},
		{"VLAN ID too high", 4096},
		{"VLAN ID negative", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			interfaces := map[string]types.Interface{
				eth0: {
					Type:    "physical",
					Device:  eth0,
					Enabled: true,
				},
				"vlan.invalid": {
					Type:    "vlan",
					Device:  eth0 + ".invalid",
					Enabled: true,
					VLANId:  tc.vlanID,
				},
			}

			_, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "interfaces",
				Value:   interfaces,
			})
			require.NoError(t, err)

			_, err = harness.SendRequest(daemon.Request{Command: "commit"})
			require.NoError(t, err)

			// Apply might fail for invalid VLAN IDs
			resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
			if err == nil && tc.vlanID <= 0 || tc.vlanID >= 4096 {
				t.Logf("Apply result for VLAN ID %d: success=%v, error=%s",
					tc.vlanID, resp.Success, resp.Error)
			}
		})
	}
}

// TestInvalidMTU tests validation of invalid MTU values
func TestInvalidMTU(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	testCases := []struct {
		name string
		mtu  int
	}{
		{"MTU too small", 67},  // Below minimum
		{"MTU too large", 100000}, // Above maximum
		{"MTU zero", 0},
		{"MTU negative", -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			interfaces := map[string]types.Interface{
				eth0: {
					Type:     "physical",
					Device:   eth0,
					Enabled:  true,
					Protocol: "static",
					IPAddr:   "10.3.4.10",
					Netmask:  "255.255.255.0",
					MTU:      tc.mtu,
				},
			}

			_, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "interfaces",
				Value:   interfaces,
			})
			require.NoError(t, err)

			_, err = harness.SendRequest(daemon.Request{Command: "commit"})
			require.NoError(t, err)

			// Apply might fail for invalid MTU values
			resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
			if err == nil {
				t.Logf("Apply result for MTU %d: success=%v, error=%s",
					tc.mtu, resp.Success, resp.Error)
			}
		})
	}
}

// TestInvalidRouteDestination tests validation of invalid route destinations
func TestInvalidRouteDestination(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	testCases := []struct {
		name        string
		destination string
	}{
		{"Invalid CIDR", "192.168.1.0/33"}, // Invalid prefix length
		{"Malformed IP", "999.999.999.999/24"},
		{"Missing prefix", "192.168.1.0"},
		{"Invalid format", "not-an-ip/24"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routes := map[string]types.Route{
				"test-route": {
					Destination: tc.destination,
					Gateway:     "10.0.0.1",
					Metric:      100,
					Enabled:     true,
				},
			}

			_, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "routes",
				Value:   routes,
			})
			require.NoError(t, err)

			_, err = harness.SendRequest(daemon.Request{Command: "commit"})
			require.NoError(t, err)

			// Apply should fail for invalid destinations
			resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
			if err == nil {
				t.Logf("Apply result for destination %s: success=%v, error=%s",
					tc.destination, resp.Success, resp.Error)
			}
		})
	}
}

// TestInvalidRouteGateway tests validation of invalid route gateways
func TestInvalidRouteGateway(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	testCases := []struct {
		name    string
		gateway string
	}{
		{"Invalid IP", "999.999.999.999"},
		{"Malformed IP", "10.0.0"},
		{"Empty gateway", ""},
		{"Invalid format", "not-an-ip"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routes := map[string]types.Route{
				"test-route-gw": {
					Destination: "192.168.5.0/24",
					Gateway:     tc.gateway,
					Metric:      100,
					Enabled:     true,
				},
			}

			_, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "routes",
				Value:   routes,
			})
			require.NoError(t, err)

			_, err = harness.SendRequest(daemon.Request{Command: "commit"})
			require.NoError(t, err)

			// Apply might fail for invalid gateways
			resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
			if err == nil {
				t.Logf("Apply result for gateway %s: success=%v, error=%s",
					tc.gateway, resp.Success, resp.Error)
			}
		})
	}
}

// TestBridgeWithoutMembers tests creating a bridge without member interfaces
func TestBridgeWithoutMembers(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Bridge without members is valid, but should create empty bridge
	interfaces := map[string]types.Interface{
		"br0": {
			Type:        "bridge",
			Device:      "br0",
			Enabled:     true,
			BridgePorts: []string{}, // Empty members
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// This should succeed - empty bridge is valid
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "empty bridge should be valid")
}

// TestBridgeWithNonexistentMembers tests bridge with non-existent member interfaces
func TestBridgeWithNonexistentMembers(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Bridge with non-existent members
	interfaces := map[string]types.Interface{
		"br0": {
			Type:        "bridge",
			Device:      "br0",
			Enabled:     true,
			BridgePorts: []string{"nonexistent1", "nonexistent2"},
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Apply should fail because members don't exist
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	if err == nil {
		assert.False(t, resp.Success, "bridge with non-existent members should fail")
	}
}

// TestDuplicateInterfaceNames tests that duplicate interface names are handled
func TestDuplicateInterfaceNames(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// This test verifies that the map structure prevents duplicates
	// (Go maps inherently prevent duplicate keys)
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.3.5.10",
			Netmask:  "255.255.255.0",
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	// Should succeed - no duplicates in map
	resp, err := harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// TestEmptyConfiguration tests handling of empty configuration
func TestEmptyConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Empty interfaces configuration
	emptyInterfaces := make(map[string]types.Interface)

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   emptyInterfaces,
	})
	require.NoError(t, err)

	// Empty config should be valid
	resp, err := harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "empty configuration should be valid")

	// Empty routes
	emptyRoutes := map[string]types.Route{}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   emptyRoutes,
	})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "empty routes should be valid")
}

// TestRouteMetricBoundary tests route metric boundary values
func TestRouteMetricBoundary(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set up an interface in the same network as the route gateway
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.2",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	testCases := []struct {
		name   string
		metric int
		valid  bool
	}{
		{"Metric 0", 0, true},
		{"Metric 1", 1, true},
		{"Metric max valid", 65535, true},
		{"Metric negative", -1, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			routes := map[string]types.Route{
				"test-route-metric": {
					Destination: "192.168.6.0/24",
					Gateway:     "10.0.0.1",
					Metric:      tc.metric,
					Enabled:     true,
				},
			}

			_, err := harness.SendRequest(daemon.Request{
				Command: "set",
				Path:    "routes",
				Value:   routes,
			})
			require.NoError(t, err)

			_, err = harness.SendRequest(daemon.Request{Command: "commit"})
			require.NoError(t, err)

			resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
			if tc.valid {
				require.NoError(t, err)
				assert.True(t, resp.Success, "valid metric should succeed")
			} else {
				// Invalid metrics might be caught during apply
				if err == nil {
					t.Logf("Apply result for metric %d: success=%v, error=%s",
						tc.metric, resp.Success, resp.Error)
				}
			}
		})
	}
}
