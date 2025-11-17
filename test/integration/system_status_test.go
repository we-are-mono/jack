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
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestSystemStatusCollection tests comprehensive system status gathering via `jack info`
func TestSystemStatusCollection(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create multiple test interfaces for comprehensive status check
	eth0 := harness.CreateDummyInterface("eth0")
	eth1 := harness.CreateDummyInterface("eth1")

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interfaces with IPs to test interface status collection
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.1.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.2.10",
			Netmask:  "255.255.255.0",
			MTU:      9000,
		},
	}

	// Set and apply configuration
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

	// Get system status via info command
	resp, err := harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)
	require.True(t, resp.Success, "info command should succeed")
	require.NotNil(t, resp.Data, "info should return data")

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "info data should be a map")

	// Test daemon status
	t.Run("daemon status", func(t *testing.T) {
		daemonData, ok := data["daemon"].(map[string]interface{})
		require.True(t, ok, "daemon status should be present")

		// Daemon should be running (we're communicating with it)
		running, ok := daemonData["running"].(bool)
		require.True(t, ok, "running field should be present")
		assert.True(t, running, "daemon should be running")

		// Config path should be set
		configPath, ok := daemonData["config_path"].(string)
		require.True(t, ok, "config_path should be present")
		assert.NotEmpty(t, configPath, "config path should not be empty")

		// Uptime field should exist (may be empty if systemd not available)
		_, hasUptime := daemonData["uptime"]
		assert.True(t, hasUptime, "uptime field should exist")
	})

	// Test interfaces status
	t.Run("interfaces status", func(t *testing.T) {
		interfacesData, ok := data["interfaces"].([]interface{})
		require.True(t, ok, "interfaces should be a list")
		require.NotEmpty(t, interfacesData, "should have at least one interface")

		// Find our configured interfaces
		foundEth0 := false
		foundEth1 := false

		for _, ifaceRaw := range interfacesData {
			iface, ok := ifaceRaw.(map[string]interface{})
			require.True(t, ok, "interface should be a map")

			name, ok := iface["name"].(string)
			require.True(t, ok, "interface name should be present")

			// Skip loopback and other system interfaces
			if name == "lo" || strings.HasPrefix(name, "docker") {
				continue
			}

			if name == eth0 {
				foundEth0 = true
				// Verify interface details
				assert.Equal(t, "up", iface["state"], "eth0 should be up")
				// Dummy interfaces report as "dummy" type, which is fine
				assert.NotEmpty(t, iface["type"], "eth0 should have a type")

				// Verify IP address
				ipAddrs, ok := iface["ipaddr"].([]interface{})
				require.True(t, ok, "ipaddr should be a list")
				require.NotEmpty(t, ipAddrs, "should have at least one IP")
				assert.Contains(t, ipAddrs[0].(string), "10.0.1.10", "should have configured IP")

				// Verify MTU
				mtu, ok := iface["mtu"].(float64) // JSON numbers are float64
				require.True(t, ok, "mtu should be present")
				assert.Equal(t, float64(1500), mtu, "MTU should match configured value")

				// Verify statistics fields exist
				assert.Contains(t, iface, "tx_packets", "should have TX packets")
				assert.Contains(t, iface, "rx_packets", "should have RX packets")
				assert.Contains(t, iface, "tx_bytes", "should have TX bytes")
				assert.Contains(t, iface, "rx_bytes", "should have RX bytes")
			}

			if name == eth1 {
				foundEth1 = true
				assert.Equal(t, "up", iface["state"], "eth1 should be up")

				// Verify custom MTU
				mtu, ok := iface["mtu"].(float64)
				require.True(t, ok, "mtu should be present")
				assert.Equal(t, float64(9000), mtu, "MTU should match configured jumbo frame value")
			}
		}

		assert.True(t, foundEth0, "should find eth0 in interface list")
		assert.True(t, foundEth1, "should find eth1 in interface list")
	})

	// Test system info
	t.Run("system info", func(t *testing.T) {
		systemData, ok := data["system"].(map[string]interface{})
		require.True(t, ok, "system info should be present")

		// Hostname should be present and non-empty
		hostname, ok := systemData["hostname"].(string)
		require.True(t, ok, "hostname should be present")
		assert.NotEmpty(t, hostname, "hostname should not be empty")

		// Kernel version should be present
		kernel, ok := systemData["kernel_version"].(string)
		require.True(t, ok, "kernel version should be present")
		assert.NotEmpty(t, kernel, "kernel version should not be empty")
		assert.Contains(t, kernel, ".", "kernel version should contain version number")

		// Uptime should be present
		uptime, ok := systemData["uptime"].(string)
		require.True(t, ok, "uptime should be present")
		assert.NotEmpty(t, uptime, "uptime should not be empty")
		assert.True(t, strings.Contains(uptime, "m") || strings.Contains(uptime, "h") || strings.Contains(uptime, "d"),
			"uptime should contain time unit (m/h/d)")
	})

	// Test IP forwarding status
	t.Run("IP forwarding", func(t *testing.T) {
		ipForwarding, ok := data["ip_forwarding"].(bool)
		require.True(t, ok, "ip_forwarding should be present")
		// Should be true after apply (daemon enables it)
		assert.True(t, ipForwarding, "IP forwarding should be enabled after apply")
	})

	// Test plugin status collection
	t.Run("plugin status", func(t *testing.T) {
		plugins, ok := data["plugins"].(map[string]interface{})
		require.True(t, ok, "plugins should be present")
		// May be empty or contain plugin statuses depending on configuration
		// Just verify the field exists and is a map
		assert.NotNil(t, plugins, "plugins map should not be nil")
	})

	// Test pending changes field
	t.Run("pending changes", func(t *testing.T) {
		pending, ok := data["pending"].(bool)
		require.True(t, ok, "pending field should be present")
		assert.False(t, pending, "should have no pending changes after apply")
	})
}

// TestInterfaceStatisticsGathering tests that interface statistics are collected correctly
func TestInterfaceStatisticsGathering(t *testing.T) {
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

	// Configure and apply interface
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.1.10",
			Netmask:  "255.255.255.0",
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

	// Send some traffic through the interface using ping
	_, err = netlink.LinkByName(eth0)
	require.NoError(t, err)

	// Send packets to ourself
	cmd := exec.Command("ping", "-c", "5", "-I", eth0, "10.0.1.10")
	_ = cmd.Run() // Ignore error if ping fails

	// Get statistics
	resp, err := harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	interfacesData, ok := data["interfaces"].([]interface{})
	require.True(t, ok)

	// Find our interface
	for _, ifaceRaw := range interfacesData {
		iface, ok := ifaceRaw.(map[string]interface{})
		require.True(t, ok)

		if iface["name"] == eth0 {
			// Verify statistics fields exist
			txPackets, hasTX := iface["tx_packets"]
			rxPackets, hasRX := iface["rx_packets"]
			txBytes, hasTXBytes := iface["tx_bytes"]
			rxBytes, hasRXBytes := iface["rx_bytes"]
			txErrors, hasTXErrors := iface["tx_errors"]
			rxErrors, hasRXErrors := iface["rx_errors"]

			assert.True(t, hasTX, "should have TX packets field")
			assert.True(t, hasRX, "should have RX packets field")
			assert.True(t, hasTXBytes, "should have TX bytes field")
			assert.True(t, hasRXBytes, "should have RX bytes field")
			assert.True(t, hasTXErrors, "should have TX errors field")
			assert.True(t, hasRXErrors, "should have RX errors field")

			// After sending ping packets, counters might have increased
			// Just verify they're non-negative numbers
			assert.GreaterOrEqual(t, txPackets.(float64), float64(0), "TX packets should be non-negative")
			assert.GreaterOrEqual(t, rxPackets.(float64), float64(0), "RX packets should be non-negative")
			assert.GreaterOrEqual(t, txBytes.(float64), float64(0), "TX bytes should be non-negative")
			assert.GreaterOrEqual(t, rxBytes.(float64), float64(0), "RX bytes should be non-negative")
			assert.GreaterOrEqual(t, txErrors.(float64), float64(0), "TX errors should be non-negative")
			assert.GreaterOrEqual(t, rxErrors.(float64), float64(0), "RX errors should be non-negative")

			return
		}
	}

	t.Fatal("test interface not found in status")
}

// TestSystemStatus_WithDownInterface tests status of disabled interfaces
func TestSystemStatus_WithDownInterface(t *testing.T) {
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

	// Configure interface as disabled
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  false, // Explicitly disabled
			Protocol: "static",
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

	// Get status
	resp, err := harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	interfacesData, ok := data["interfaces"].([]interface{})
	require.True(t, ok)

	// Find our interface
	for _, ifaceRaw := range interfacesData {
		iface, ok := ifaceRaw.(map[string]interface{})
		require.True(t, ok)

		if iface["name"] == eth0 {
			// Interface should be down
			state, ok := iface["state"].(string)
			require.True(t, ok, "state should be present")
			assert.Equal(t, "down", state, "disabled interface should be down")
			return
		}
	}

	t.Fatal("test interface not found in status")
}

// TestSystemStatus_WithMultipleInterfaceTypes tests status with different interface types
func TestSystemStatus_WithMultipleInterfaceTypes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create test interfaces
	eth0 := harness.CreateDummyInterface("eth0")
	br0 := "br-test"

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure multiple interface types
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.0.1.10",
			Netmask:  "255.255.255.0",
		},
		br0: {
			Type:        "bridge",
			Device:      br0,
			Enabled:     true,
			Protocol:    "static",
			IPAddr:      "192.168.1.1",
			Netmask:     "255.255.255.0",
			BridgePorts: []string{},
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

	// Get status
	resp, err := harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	interfacesData, ok := data["interfaces"].([]interface{})
	require.True(t, ok)

	foundPhysical := false
	foundBridge := false

	for _, ifaceRaw := range interfacesData {
		iface, ok := ifaceRaw.(map[string]interface{})
		require.True(t, ok)

		name, ok := iface["name"].(string)
		require.True(t, ok)

		ifaceType, ok := iface["type"].(string)
		require.True(t, ok)

		if name == eth0 {
			// Dummy test interfaces report as "dummy" type
			assert.NotEmpty(t, ifaceType, "eth0 should have a type")
			foundPhysical = true
		}

		if name == br0 {
			assert.Equal(t, "bridge", ifaceType, "br-test should be bridge type")
			foundBridge = true
		}
	}

	assert.True(t, foundPhysical, "should find physical interface")
	assert.True(t, foundBridge, "should find bridge interface")
}
