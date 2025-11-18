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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestGetEntireConfiguration tests retrieving the complete configuration
func TestGetEntireConfiguration(t *testing.T) {
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

	// Set complex configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.1.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.1.20",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	routes := map[string]types.Route{
		"test-route": {
			Destination: "192.168.1.0/24",
			Gateway:     "10.4.1.1",
			Metric:      100,
			Enabled:     true,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Get entire configuration
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Verify configuration structure
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
		Routes     map[string]types.Route      `json:"routes"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	// Verify interfaces
	require.Contains(t, config.Interfaces, eth0)
	require.Contains(t, config.Interfaces, eth1)
	assert.Equal(t, "10.4.1.10", config.Interfaces[eth0].IPAddr)
	assert.Equal(t, "10.4.1.20", config.Interfaces[eth1].IPAddr)

	// Verify routes
	require.Len(t, config.Routes, 1)
	require.Contains(t, config.Routes, "test-route")
	assert.Equal(t, "192.168.1.0/24", config.Routes["test-route"].Destination)
}

// TestGetSpecificPath tests path-based configuration retrieval
func TestGetSpecificPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.2.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	routes := map[string]types.Route{
		"test-route-2": {
			Destination: "192.168.2.0/24",
			Gateway:     "10.4.2.1",
			Metric:      100,
			Enabled:     true,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Get specific path - interfaces only
	resp, err := harness.SendRequest(daemon.Request{
		Command: "get",
		Path:    "interfaces",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Verify only interfaces are returned
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var retrievedInterfaces map[string]types.Interface
	err = json.Unmarshal(dataBytes, &retrievedInterfaces)
	require.NoError(t, err)

	require.Contains(t, retrievedInterfaces, eth0)
	assert.Equal(t, "10.4.2.10", retrievedInterfaces[eth0].IPAddr)

	// Get specific path - routes only
	resp, err = harness.SendRequest(daemon.Request{
		Command: "get",
		Path:    "routes",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	dataBytes, err = json.Marshal(resp.Data)
	require.NoError(t, err)

	var retrievedRoutes map[string]types.Route
	err = json.Unmarshal(dataBytes, &retrievedRoutes)
	require.NoError(t, err)

	require.Len(t, retrievedRoutes, 1)
	require.Contains(t, retrievedRoutes, "test-route-2")
	assert.Equal(t, "192.168.2.0/24", retrievedRoutes["test-route-2"].Destination)
}

// TestGetInvalidPath tests getting configuration from non-existent path
func TestGetInvalidPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to get invalid path
	resp, err := harness.SendRequest(daemon.Request{
		Command: "get",
		Path:    "nonexistent.path.to.config",
	})

	// Should either error or return unsuccessful response
	if err == nil {
		// If no error, response should indicate failure or return nil/empty data
		if resp.Success {
			// Data might be nil or empty
			t.Logf("Get invalid path returned: data=%v", resp.Data)
		} else {
			assert.NotEmpty(t, resp.Error, "should have error message")
		}
	}
}

// TestGetEmptyConfiguration tests getting configuration when nothing is set
func TestGetEmptyConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Get config before setting anything
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Data should be non-nil but empty or have default structure
	if resp.Data != nil {
		dataBytes, err := json.Marshal(resp.Data)
		require.NoError(t, err)
		t.Logf("Empty config structure: %s", string(dataBytes))
	}
}

// TestGetAfterSet tests getting uncommitted configuration
func TestGetAfterSet(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set configuration without committing
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.3.10",
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

	// Get should return the staged configuration
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	assert.Equal(t, "10.4.3.10", config.Interfaces[eth0].IPAddr,
		"get should return staged configuration")
}

// TestGetAfterRevert tests getting configuration after revert
func TestGetAfterRevert(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set and commit initial configuration
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.4.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   initialInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Make changes
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.4.20", // Changed
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces,
	})
	require.NoError(t, err)

	// Revert
	_, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)

	// Get should return original committed configuration
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	assert.Equal(t, "10.4.4.10", config.Interfaces[eth0].IPAddr,
		"get should return committed config after revert")
}

// TestGetComplexNestedConfiguration tests getting complex nested structures
func TestGetComplexNestedConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")
	eth1 := harness.CreateDummyInterface("eth1")
	eth2 := harness.CreateDummyInterface("eth2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Create complex configuration with bridge and VLANs
	interfaces := map[string]types.Interface{
		eth0: {
			Type:    "physical",
			Device:  eth0,
			Enabled: true,
		},
		eth1: {
			Type:    "physical",
			Device:  eth1,
			Enabled: true,
		},
		eth2: {
			Type:    "physical",
			Device:  eth2,
			Enabled: true,
		},
		"br0": {
			Type:        "bridge",
			Device:      "br0",
			Enabled:     true,
			BridgePorts: []string{eth1, eth2},
		},
		"vlan100": {
			Type:    "vlan",
			Device:  eth0 + ".100",
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

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Get entire configuration
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	// Verify all interfaces present
	require.Contains(t, config.Interfaces, eth0)
	require.Contains(t, config.Interfaces, eth1)
	require.Contains(t, config.Interfaces, eth2)
	require.Contains(t, config.Interfaces, "br0")
	require.Contains(t, config.Interfaces, "vlan100")

	// Verify bridge members
	assert.ElementsMatch(t, []string{eth1, eth2}, config.Interfaces["br0"].BridgePorts)

	// Verify VLAN properties
	assert.Equal(t, 100, config.Interfaces["vlan100"].VLANId)
}

// TestGetMultipleTimes tests that get is idempotent
func TestGetMultipleTimes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.5.10",
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

	// Get multiple times and verify consistency
	var configs []string
	for i := 0; i < 5; i++ {
		resp, err := harness.SendRequest(daemon.Request{Command: "get"})
		require.NoError(t, err)
		require.True(t, resp.Success)

		dataBytes, err := json.Marshal(resp.Data)
		require.NoError(t, err)
		configs = append(configs, string(dataBytes))
	}

	// All gets should return identical configuration
	for i := 1; i < len(configs); i++ {
		assert.JSONEq(t, configs[0], configs[i],
			"get should return consistent results")
	}
}

// TestGetAfterApply tests that get returns correct config after apply
func TestGetAfterApply(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set, commit, and apply
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.4.6.10",
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

	// Get before apply
	respBefore, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Get after apply
	respAfter, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	// Configuration should be same before and after apply
	beforeBytes, _ := json.Marshal(respBefore.Data)
	afterBytes, _ := json.Marshal(respAfter.Data)
	assert.JSONEq(t, string(beforeBytes), string(afterBytes),
		"get should return same config before and after apply")
}
