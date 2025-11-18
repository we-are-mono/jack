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
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/system"
	"github.com/we-are-mono/jack/types"
)

func TestSnapshotCapture(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("test0")
	link, err := netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	// Add IP address
	addr, _ := netlink.ParseAddr("192.168.100.1/24")
	require.NoError(t, netlink.AddrAdd(link, addr))

	// Bring interface up
	require.NoError(t, netlink.LinkSetUp(link))

	// Capture snapshot
	snapshot, err := system.CaptureSystemSnapshot()
	require.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.NotEmpty(t, snapshot.CheckpointID)
	assert.NotZero(t, snapshot.Timestamp)

	// Verify interface is captured
	ifaceSnapshot, exists := snapshot.Interfaces[ifaceName]
	require.True(t, exists)
	assert.Equal(t, "physical", ifaceSnapshot.Type)
	assert.True(t, ifaceSnapshot.Existed)
	assert.Equal(t, "up", ifaceSnapshot.State)
	assert.Contains(t, ifaceSnapshot.Addresses, "192.168.100.1/24")
}

func TestSnapshotRestore(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create initial state
	ifaceName := harness.CreateDummyInterface("test0")
	link, err := netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addr1, _ := netlink.ParseAddr("192.168.100.1/24")
	require.NoError(t, netlink.AddrAdd(link, addr1))
	require.NoError(t, netlink.LinkSetUp(link))

	// Capture snapshot
	snapshot, err := system.CaptureSystemSnapshot()
	require.NoError(t, err)

	// Modify state (add another IP, bring down)
	addr2, _ := netlink.ParseAddr("192.168.100.2/24")
	require.NoError(t, netlink.AddrAdd(link, addr2))
	require.NoError(t, netlink.LinkSetDown(link))

	// Verify modified state
	link, _ = netlink.LinkByName(ifaceName) // Refetch to get updated flags
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	assert.Len(t, addrs, 2)
	assert.Equal(t, net.Flags(0), link.Attrs().Flags&net.FlagUp) // Link down

	// Restore snapshot
	err = system.RestoreSnapshot(snapshot, []string{"interfaces"})
	require.NoError(t, err)

	// Verify restored state
	link, err = netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	assert.Len(t, addrs, 1)
	assert.Equal(t, "192.168.100.1/24", addrs[0].IPNet.String())

	// Check link is up
	assert.NotEqual(t, net.Flags(0), link.Attrs().Flags&net.FlagUp)
}

func TestApplyWithAutomaticRollback(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		harness.StartDaemon(ctx)
	}()
	defer func() {
		cancel()
		<-serverDone
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Create initial interface state
	ifaceName := harness.CreateDummyInterface("eth0")
	link, err := netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addr, _ := netlink.ParseAddr("10.0.0.1/24")
	require.NoError(t, netlink.AddrAdd(link, addr))
	require.NoError(t, netlink.LinkSetUp(link))

	// Create invalid config with nonexistent interface
	invalidInterfaces := map[string]types.Interface{
		ifaceName: {
			Device:   ifaceName,
			Type:     "physical",
			IPAddr:   "10.0.0.2",
			Netmask:  "255.255.255.0",
			Enabled:  true,
			Protocol: "static",
		},
		"invalid-iface": {
			Device:   "nonexistent",
			Type:     "physical",
			IPAddr:   "192.168.1.1",
			Netmask:  "255.255.255.0",
			Enabled:  true,
			Protocol: "static",
		},
	}

	// Set and commit the invalid config
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   invalidInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Try to apply (should fail and rollback)
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	if resp.Success {
		t.Logf("Apply unexpectedly succeeded. Expected failure for nonexistent interface")
	} else {
		t.Logf("Apply failed as expected with error: %s", resp.Error)
	}
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "rolled back")

	// Verify original state is preserved
	link, err = netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)

	// Original IP should still be there (rollback succeeded)
	found := false
	for _, a := range addrs {
		if a.IPNet.String() == "10.0.0.1/24" {
			found = true
			break
		}
	}
	assert.True(t, found, "Original IP should be preserved after rollback")
}

func TestManualCheckpointAndRollback(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		harness.StartDaemon(ctx)
	}()
	defer func() {
		cancel()
		<-serverDone
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Create initial state
	ifaceName := harness.CreateDummyInterface("eth0")
	link, err := netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addr1, _ := netlink.ParseAddr("10.0.0.1/24")
	require.NoError(t, netlink.AddrAdd(link, addr1))
	require.NoError(t, netlink.LinkSetUp(link))

	// Create manual checkpoint
	resp, err := client.Send(daemon.Request{Command: "checkpoint-create"})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	checkpointID := resp.Data.(string)
	assert.Contains(t, checkpointID, "manual-")

	// Modify state
	addr2, _ := netlink.ParseAddr("10.0.0.2/24")
	require.NoError(t, netlink.AddrAdd(link, addr2))
	require.NoError(t, netlink.LinkSetDown(link))

	// List checkpoints
	resp, err = client.Send(daemon.Request{Command: "checkpoint-list"})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Rollback to manual checkpoint
	resp, err = client.Send(daemon.Request{
		Command:      "rollback",
		CheckpointID: checkpointID,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify state is restored
	link, err = netlink.LinkByName(ifaceName)
	require.NoError(t, err)

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	assert.Len(t, addrs, 1)
	assert.Equal(t, "10.0.0.1/24", addrs[0].IPNet.String())

	// Check link is up
	assert.NotEqual(t, net.Flags(0), link.Attrs().Flags&net.FlagUp)
}

func TestCheckpointListAndCreate(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start daemon
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		harness.StartDaemon(ctx)
	}()
	defer func() {
		cancel()
		<-serverDone
	}()

	harness.WaitForDaemon(5 * time.Second)

	// List checkpoints (should be empty or have auto checkpoints)
	resp, err := client.Send(daemon.Request{Command: "checkpoint-list"})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Create manual checkpoint
	resp, err = client.Send(daemon.Request{Command: "checkpoint-create"})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	checkpointID, ok := resp.Data.(string)
	require.True(t, ok)
	assert.Contains(t, checkpointID, "manual-")

	// List checkpoints again (should include our manual checkpoint)
	resp, err = client.Send(daemon.Request{Command: "checkpoint-list"})
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Try to use the checkpoint
	resp, err = client.Send(daemon.Request{
		Command:      "rollback",
		CheckpointID: checkpointID,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}
