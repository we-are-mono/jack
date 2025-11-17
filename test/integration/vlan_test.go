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

// TestVLANCreation tests creating a VLAN interface
func TestVLANCreation(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create parent interface
	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure VLAN
	interfaces := map[string]types.Interface{
		eth0 + ".100": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".100",
			VLANId:     100,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.100.1",
			Netmask:    "255.255.255.0",
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
	require.True(t, resp.Success, "apply should succeed: %v", resp.Error)

	// Verify VLAN interface was created
	vlan, err := netlink.LinkByName(eth0 + ".100")
	require.NoError(t, err, "VLAN interface should exist")
	assert.Equal(t, "vlan", vlan.Type(), "should be VLAN type")

	// Verify VLAN is up
	assert.True(t, vlan.Attrs().Flags&net.FlagUp != 0, "VLAN should be up")

	// Verify IP address
	addrs, err := netlink.AddrList(vlan, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1, "VLAN should have exactly one IP")
	assert.Equal(t, "192.168.100.1", addrs[0].IP.String(), "IP should match")

	// Verify VLAN ID
	if vlanLink, ok := vlan.(*netlink.Vlan); ok {
		assert.Equal(t, 100, vlanLink.VlanId, "VLAN ID should match")
	}
}

// TestMultipleVLANsOnSameParent tests creating multiple VLANs on same parent interface
func TestMultipleVLANsOnSameParent(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure multiple VLANs
	interfaces := map[string]types.Interface{
		eth0 + ".10": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".10",
			VLANId:     10,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.10.1",
			Netmask:    "255.255.255.0",
		},
		eth0 + ".20": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".20",
			VLANId:     20,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.20.1",
			Netmask:    "255.255.255.0",
		},
		eth0 + ".30": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".30",
			VLANId:     30,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.30.1",
			Netmask:    "255.255.255.0",
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

	// Verify all VLANs were created
	for vlanName, expectedIP := range map[string]string{
		eth0 + ".10": "192.168.10.1",
		eth0 + ".20": "192.168.20.1",
		eth0 + ".30": "192.168.30.1",
	} {
		vlan, err := netlink.LinkByName(vlanName)
		require.NoError(t, err, "%s should exist", vlanName)
		assert.Equal(t, "vlan", vlan.Type(), "%s should be VLAN type", vlanName)

		addrs, err := netlink.AddrList(vlan, netlink.FAMILY_V4)
		require.NoError(t, err)
		require.Len(t, addrs, 1, "%s should have one IP", vlanName)
		assert.Equal(t, expectedIP, addrs[0].IP.String(), "%s IP should match", vlanName)
	}
}

// TestVLANOnBridge tests creating a VLAN on top of a bridge
func TestVLANOnBridge(t *testing.T) {
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

	// Configure bridge and VLAN on bridge
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
		"br-lan.100": {
			Type:       "vlan",
			Device:     "br-lan",
			DeviceName: "br-lan.100",
			VLANId:     100,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.100.1",
			Netmask:    "255.255.255.0",
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

	// Verify bridge exists
	bridge, err := netlink.LinkByName("br-lan")
	require.NoError(t, err)
	assert.Equal(t, "bridge", bridge.Type())

	// Verify VLAN on bridge exists
	vlan, err := netlink.LinkByName("br-lan.100")
	require.NoError(t, err)
	assert.Equal(t, "vlan", vlan.Type())

	// Verify VLAN parent is the bridge
	if vlanLink, ok := vlan.(*netlink.Vlan); ok {
		assert.Equal(t, bridge.Attrs().Index, vlanLink.ParentIndex)
	}

	// Verify IPs
	bridgeAddrs, err := netlink.AddrList(bridge, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, bridgeAddrs, 1)
	assert.Equal(t, "192.168.1.1", bridgeAddrs[0].IP.String())

	vlanAddrs, err := netlink.AddrList(vlan, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, vlanAddrs, 1)
	assert.Equal(t, "192.168.100.1", vlanAddrs[0].IP.String())
}

// TestVLANDisable tests disabling a VLAN interface
func TestVLANDisable(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure and enable VLAN
	interfaces := map[string]types.Interface{
		eth0 + ".100": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".100",
			VLANId:     100,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.100.1",
			Netmask:    "255.255.255.0",
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

	// Verify VLAN is up
	vlan, err := netlink.LinkByName(eth0 + ".100")
	require.NoError(t, err)
	assert.True(t, vlan.Attrs().Flags&net.FlagUp != 0, "VLAN should be up")

	// Disable VLAN
	interfaces[eth0+".100"] = types.Interface{
		Type:       "vlan",
		Device:     eth0,
		DeviceName: eth0 + ".100",
		VLANId:     100,
		Enabled:    false,
		Protocol:   "static",
		IPAddr:     "192.168.100.1",
		Netmask:    "255.255.255.0",
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

	// Verify VLAN is down
	vlan, err = netlink.LinkByName(eth0 + ".100")
	require.NoError(t, err)
	assert.False(t, vlan.Attrs().Flags&net.FlagUp != 0, "VLAN should be down")
}

// TestVLANIdempotency tests that reapplying VLAN config is idempotent
func TestVLANIdempotency(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure VLAN
	interfaces := map[string]types.Interface{
		eth0 + ".100": {
			Type:       "vlan",
			Device:     eth0,
			DeviceName: eth0 + ".100",
			VLANId:     100,
			Enabled:    true,
			Protocol:   "static",
			IPAddr:     "192.168.100.1",
			Netmask:    "255.255.255.0",
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

	// Verify VLAN is still configured correctly
	vlan, err := netlink.LinkByName(eth0 + ".100")
	require.NoError(t, err)

	addrs, err := netlink.AddrList(vlan, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "192.168.100.1", addrs[0].IP.String())

	if vlanLink, ok := vlan.(*netlink.Vlan); ok {
		assert.Equal(t, 100, vlanLink.VlanId)
	}
}
