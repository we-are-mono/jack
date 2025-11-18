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

// TestShowAllConfiguration tests the "show" command without path
func TestShowAllConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			IPAddr:   "10.5.1.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	routes := map[string]types.Route{
		"test-route-show": {
			Destination: "192.168.5.0/24",
			Gateway:     "10.5.1.1",
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

	// Show all configuration (no path)
	resp, err := harness.SendRequest(daemon.Request{Command: "show"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Verify data structure
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var allConfigs map[string]interface{}
	err = json.Unmarshal(dataBytes, &allConfigs)
	require.NoError(t, err)

	// Should contain interfaces and routes
	assert.Contains(t, allConfigs, "interfaces")
	assert.Contains(t, allConfigs, "routes")
}

// TestShowSpecificPath tests the "show" command with a specific path
func TestShowSpecificPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			IPAddr:   "10.5.2.10",
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

	// Show specific path
	resp, err := harness.SendRequest(daemon.Request{
		Command: "show",
		Path:    "interfaces",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Verify we got interfaces config
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var interfacesConfig types.InterfacesConfig
	err = json.Unmarshal(dataBytes, &interfacesConfig)
	require.NoError(t, err)

	require.Contains(t, interfacesConfig.Interfaces, eth0)
	assert.Equal(t, "10.5.2.10", interfacesConfig.Interfaces[eth0].IPAddr)
}

// TestShowInvalidPath tests the "show" command with an invalid path
func TestShowInvalidPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Try to show invalid path
	resp, err := harness.SendRequest(daemon.Request{
		Command: "show",
		Path:    "nonexistent.config.path",
	})
	require.NoError(t, err)

	// Should return error
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

// TestShowEmptyConfiguration tests showing empty config
func TestShowEmptyConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Show before setting any configuration
	resp, err := harness.SendRequest(daemon.Request{Command: "show"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Data should be non-nil but may be empty
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	t.Logf("Empty show output: %s", string(dataBytes))
}

// TestShowVsGet tests the difference between "show" and "get" commands
func TestShowVsGet(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			IPAddr:   "10.5.3.10",
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

	// Get "show" response
	showResp, err := harness.SendRequest(daemon.Request{Command: "show"})
	require.NoError(t, err)
	require.True(t, showResp.Success)

	// Get "get" response
	getResp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, getResp.Success)

	// Both should succeed
	assert.NotNil(t, showResp.Data)
	assert.NotNil(t, getResp.Data)

	// Log both for comparison
	showBytes, _ := json.Marshal(showResp.Data)
	getBytes, _ := json.Marshal(getResp.Data)
	t.Logf("Show response: %s", string(showBytes))
	t.Logf("Get response: %s", string(getBytes))
}

// TestShowAfterRevert tests that show returns correct config after revert
func TestShowAfterRevert(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			IPAddr:   "10.5.4.10",
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

	// Make uncommitted changes
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.4.20", // Changed
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

	// Revert changes
	_, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)

	// Show should return original committed config
	resp, err := harness.SendRequest(daemon.Request{
		Command: "show",
		Path:    "interfaces",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var interfacesConfig types.InterfacesConfig
	err = json.Unmarshal(dataBytes, &interfacesConfig)
	require.NoError(t, err)

	require.Contains(t, interfacesConfig.Interfaces, eth0)
	assert.Equal(t, "10.5.4.10", interfacesConfig.Interfaces[eth0].IPAddr,
		"show should return committed config after revert")
}
