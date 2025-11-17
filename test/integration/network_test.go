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

// TestPhysicalInterfaceStaticIP tests configuring a physical interface with static IP
func TestPhysicalInterfaceStaticIP(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create dummy interface to configure
	eth0 := harness.CreateDummyInterface("eth0")

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface with static IP
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "192.168.1.100",
			Netmask:  "255.255.255.0",
			Gateway:  "192.168.1.1",
			MTU:      1500,
		},
	}

	// Set configuration
	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "set should succeed")

	// Commit configuration
	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	require.True(t, resp.Success, "commit should succeed")

	// Apply configuration
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Verify interface configuration using netlink
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err, "interface should exist")

	// Check MTU
	assert.Equal(t, 1500, link.Attrs().MTU, "MTU should be set")

	// Check interface is up
	assert.True(t, link.Attrs().Flags&net.FlagUp != 0, "interface should be up")

	// Check IP address
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1, "should have exactly one IP address")
	assert.Equal(t, "192.168.1.100", addrs[0].IP.String(), "IP should match")
	assert.Equal(t, "ffffff00", addrs[0].Mask.String(), "netmask should match")
}

// TestPhysicalInterfaceIdempotency tests that reapplying same config doesn't cause errors
func TestPhysicalInterfaceIdempotency(t *testing.T) {
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

	// Apply first time
	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "first apply should succeed")

	// Apply second time (idempotency test)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "second apply should succeed (idempotent)")

	// Verify configuration is still correct
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.0.1", addrs[0].IP.String())
}

// TestPhysicalInterfaceMTUChange tests changing MTU on existing interface
func TestPhysicalInterfaceMTUChange(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure with MTU 1500
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.0.1",
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
	require.True(t, resp.Success)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify initial MTU
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	assert.Equal(t, 1500, link.Attrs().MTU)

	// Change MTU to 1400
	interfaces[eth0] = types.Interface{
		Type:     "physical",
		Device:   eth0,
		Enabled:  true,
		Protocol: "static",
		IPAddr:   "10.0.0.1",
		Netmask:  "255.255.255.0",
		MTU:      1400,
	}

	resp, err = harness.SendRequest(daemon.Request{
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

	// Verify MTU changed
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	assert.Equal(t, 1400, link.Attrs().MTU, "MTU should be updated")
}

// TestPhysicalInterfaceDisable tests disabling an interface
func TestPhysicalInterfaceDisable(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure and enable interface
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

	// Verify interface is up
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	assert.True(t, link.Attrs().Flags&net.FlagUp != 0, "interface should be up")

	// Disable interface
	interfaces[eth0] = types.Interface{
		Type:     "physical",
		Device:   eth0,
		Enabled:  false,
		Protocol: "static",
		IPAddr:   "10.0.0.1",
		Netmask:  "255.255.255.0",
	}

	resp, err = harness.SendRequest(daemon.Request{
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

	// Verify interface is down
	link, err = netlink.LinkByName(eth0)
	require.NoError(t, err)
	assert.False(t, link.Attrs().Flags&net.FlagUp != 0, "interface should be down")
}

// TestMultiplePhysicalInterfaces tests configuring multiple interfaces
func TestMultiplePhysicalInterfaces(t *testing.T) {
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

	// Configure multiple interfaces
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "192.168.1.1",
			Netmask:  "255.255.255.0",
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "192.168.2.1",
			Netmask:  "255.255.255.0",
		},
		eth2: {
			Type:     "physical",
			Device:   eth2,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "192.168.3.1",
			Netmask:  "255.255.255.0",
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify all interfaces are configured
	for iface, expectedIP := range map[string]string{
		eth0: "192.168.1.1",
		eth1: "192.168.2.1",
		eth2: "192.168.3.1",
	} {
		link, err := netlink.LinkByName(iface)
		require.NoError(t, err, "interface %s should exist", iface)

		assert.True(t, link.Attrs().Flags&net.FlagUp != 0, "%s should be up", iface)

		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		require.NoError(t, err)
		require.Len(t, addrs, 1, "%s should have one IP", iface)
		assert.Equal(t, expectedIP, addrs[0].IP.String(), "%s IP should match", iface)
	}
}
