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

// TestBridgeCreation tests creating a bridge with member ports
func TestBridgeCreation(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create dummy interfaces for bridge ports
	eth0 := harness.CreateDummyInterface("eth0")
	eth1 := harness.CreateDummyInterface("eth1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure bridge
	interfaces := map[string]types.Interface{
		"br-lan": {
			Type:        "bridge",
			Device:      "br-lan",
			BridgePorts: []string{eth0, eth1},
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
			MTU:         1500,
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "set should succeed: %v", resp.Error)

	resp, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Verify bridge was created
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err, "bridge should exist")
	assert.Equal(t, "bridge", bridge.Type(), "should be bridge type")

	// Verify MTU
	assert.Equal(t, 1500, bridge.Attrs().MTU, "MTU should match")

	// Verify bridge is up
	assert.True(t, bridge.Attrs().Flags&net.FlagUp != 0, "bridge should be up")

	// Verify IP address on bridge
	addrs, err := netlink.AddrList(bridge, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1, "bridge should have exactly one IP")
	assert.Equal(t, "192.168.1.1", addrs[0].IP.String(), "IP should match")

	// Verify ports are attached to bridge
	links, err := netlink.LinkList()
	require.NoError(t, err)

	bridgeIndex := bridge.Attrs().Index
	attachedPorts := []string{}
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attachedPorts = append(attachedPorts, link.Attrs().Name)
		}
	}

	assert.ElementsMatch(t, []string{eth0, eth1}, attachedPorts,
		"bridge should have correct ports attached")
}

// TestBridgePortAddition tests adding a port to an existing bridge
func TestBridgePortAddition(t *testing.T) {
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

	// Create bridge with one port
	interfaces := map[string]types.Interface{
		"br-lan": {
			Type:        "bridge",
			Device:      "br-lan",
			BridgePorts: []string{eth0},
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
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

	// Verify initial port
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err)

	links, err := netlink.LinkList()
	require.NoError(t, err)

	bridgeIndex := bridge.Attrs().Index
	attachedPorts := []string{}
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attachedPorts = append(attachedPorts, link.Attrs().Name)
		}
	}
	assert.ElementsMatch(t, []string{eth0}, attachedPorts)

	// Add two more ports
	interfaces["br-lan"] = types.Interface{
		Type:        "bridge",
		Device:      "br-lan",
		BridgePorts: []string{eth0, eth1, eth2},
		Enabled:     true,
		Protocol:    "static",
		IPAddr:      "192.168.1.1",
		Netmask:     "255.255.255.0",
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

	// Verify all ports are now attached
	links, err = netlink.LinkList()
	require.NoError(t, err)

	attachedPorts = []string{}
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attachedPorts = append(attachedPorts, link.Attrs().Name)
		}
	}
	assert.ElementsMatch(t, []string{eth0, eth1, eth2}, attachedPorts,
		"all ports should be attached")
}

// TestBridgePortRemoval tests removing a port from a bridge
func TestBridgePortRemoval(t *testing.T) {
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

	// Create bridge with three ports
	interfaces := map[string]types.Interface{
		"br-lan": {
			Type:        "bridge",
			Device:      "br-lan",
			BridgePorts: []string{eth0, eth1, eth2},
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
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

	// Remove one port
	interfaces["br-lan"] = types.Interface{
		Type:        "bridge",
		Device:      "br-lan",
		BridgePorts: []string{eth0, eth1},
		Enabled:     true,
		Protocol:    "static",
		IPAddr:      "192.168.1.1",
		Netmask:     "255.255.255.0",
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

	// Verify port was removed
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err)

	links, err := netlink.LinkList()
	require.NoError(t, err)

	bridgeIndex := bridge.Attrs().Index
	attachedPorts := []string{}
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attachedPorts = append(attachedPorts, link.Attrs().Name)
		}
	}
	assert.ElementsMatch(t, []string{eth0, eth1}, attachedPorts,
		"only two ports should remain")
}

// TestBridgeIdempotency tests that reapplying bridge config is idempotent
func TestBridgeIdempotency(t *testing.T) {
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

	// Configure bridge
	interfaces := map[string]types.Interface{
		"br-lan": {
			Type:        "bridge",
			Device:      "br-lan",
			BridgePorts: []string{eth0, eth1},
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
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
	require.True(t, resp.Success, "first apply should succeed")

	// Apply again (idempotency test)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "second apply should succeed (idempotent)")

	// Verify bridge is still configured correctly
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err)

	addrs, err := netlink.AddrList(bridge, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "192.168.1.1", addrs[0].IP.String())

	// Verify ports are still attached
	links, err := netlink.LinkList()
	require.NoError(t, err)

	bridgeIndex := bridge.Attrs().Index
	attachedPorts := []string{}
	for _, link := range links {
		if link.Attrs().MasterIndex == bridgeIndex {
			attachedPorts = append(attachedPorts, link.Attrs().Name)
		}
	}
	assert.ElementsMatch(t, []string{eth0, eth1}, attachedPorts)
}

// TestBridgeWithPhysicalInterfaces tests bridge coexisting with physical interfaces
func TestBridgeWithPhysicalInterfaces(t *testing.T) {
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

	// Configure bridge and standalone physical interface
	interfaces := map[string]types.Interface{
		"br-lan": {
			Type:        "bridge",
			Device:      "br-lan",
			BridgePorts: []string{eth0, eth1},
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
		},
		eth2: {
			Type:     "physical",
			Device:   eth2,
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

	// Verify bridge
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err)

	addrs, err := netlink.AddrList(bridge, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "192.168.1.1", addrs[0].IP.String())

	// Verify physical interface
	eth2Link, err := netlink.LinkByName(eth2)
	require.NoError(t, err)

	addrs, err = netlink.AddrList(eth2Link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.0.0.1", addrs[0].IP.String())
}
