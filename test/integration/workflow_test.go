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
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestSetCommitApplyWorkflow tests the complete transactional workflow
func TestSetCommitApplyWorkflow(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create test interface
	eth0 := harness.CreateDummyInterface("eth0")

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Step 1: Set configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.1.100",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "set should succeed")

	// Verify configuration not applied yet (interface should not have IP)
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	assert.Len(t, addrs, 0, "IP should not be applied before commit")

	// Step 2: Commit configuration
	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	require.True(t, resp.Success, "commit should succeed")

	// Configuration still not applied to system
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	assert.Len(t, addrs, 0, "IP should not be applied before apply command")

	// Step 3: Apply configuration
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Now configuration should be applied
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1, "should have one IP address after apply")
	assert.Equal(t, "10.0.1.100", addrs[0].IP.String())
}

// TestStagingArea tests that changes are staged but not applied
func TestStagingArea(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set initial configuration and apply it
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.2.10",
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

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify initial IP is applied
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.2.10", addrs[0].IP.String())

	// Now set new configuration (stage it)
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.2.20", // Changed IP
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

	// Check that system still has old IP (changes only staged)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.2.10", addrs[0].IP.String(), "system should still have old IP")

	// Commit the changes
	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// System should still have old IP (not yet applied)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.2.10", addrs[0].IP.String(), "system should still have old IP before apply")

	// Now apply
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Now system should have new IP
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.2.20", addrs[0].IP.String(), "system should have new IP after apply")
}

// TestMultiStepTransaction tests multiple set operations before commit
func TestMultiStepTransaction(t *testing.T) {
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

	// Set first interface
	interfaces1 := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.3.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces1,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Set second interface (without committing first)
	interfaces2 := map[string]types.Interface{
		eth0: interfaces1[eth0], // Keep eth0
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.3.20",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces2,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Add routes in same transaction
	routes := []types.Route{
		{
			Destination: "192.168.1.0/24",
			Gateway:     "10.0.3.1",
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

	// Now commit all changes at once
	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Apply everything
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Verify both interfaces are configured
	link0, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs0, err := netlink.AddrList(link0, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs0, 1)
	assert.Equal(t, "10.0.3.10", addrs0[0].IP.String())

	link1, err := netlink.LinkByName(eth1)
	require.NoError(t, err)
	addrs1, err := netlink.AddrList(link1, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs1, 1)
	assert.Equal(t, "10.0.3.20", addrs1[0].IP.String())

	// Verify route is configured
	nlRoutes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	require.NoError(t, err)
	found := false
	for _, route := range nlRoutes {
		if route.Dst != nil && route.Dst.String() == "192.168.1.0/24" {
			found = true
			assert.Equal(t, "10.0.3.1", route.Gw.String())
			break
		}
	}
	assert.True(t, found, "route should be configured")
}

// TestWorkflowIdempotency tests that repeated workflow operations are idempotent
func TestWorkflowIdempotency(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.4.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	// Apply configuration first time
	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify configuration
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	initialIP := addrs[0].IP.String()

	// Apply same configuration again (idempotency test)
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "second apply should succeed")

	// Verify configuration unchanged
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, initialIP, addrs[0].IP.String(), "IP should remain the same")

	// Apply a third time
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "third apply should succeed")
}

// TestConfigurationPersistence tests that configuration survives daemon restart
func TestConfigurationPersistence(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	// Start daemon first time
	ctx1, cancel1 := context.WithCancel(context.Background())

	go func() {
		_ = harness.StartDaemon(ctx1)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure and commit
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.5.10",
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

	resp, err := harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Stop daemon
	cancel1()
	time.Sleep(500 * time.Millisecond)

	// Clear the interface to simulate clean state
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	for _, addr := range addrs {
		_ = netlink.AddrDel(link, &addr)
	}

	// Start daemon second time
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go func() {
		_ = harness.StartDaemon(ctx2)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Get configuration to verify it was loaded from disk
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Marshal and unmarshal to get clean structure
	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	// Verify interface configuration was loaded
	require.Contains(t, config.Interfaces, eth0, "configuration should be loaded from disk")
	loadedInterface := config.Interfaces[eth0]
	assert.Equal(t, "10.0.5.10", loadedInterface.IPAddr)
	assert.Equal(t, "255.255.255.0", loadedInterface.Netmask)
	assert.Equal(t, 1500, loadedInterface.MTU)

	// Apply the loaded configuration
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply after reload should succeed")

	// Verify interface is configured correctly
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.5.10", addrs[0].IP.String(), "configuration should persist across restarts")
}
