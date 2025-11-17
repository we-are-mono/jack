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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
)

// setupWireGuardTest creates a test harness with WireGuard plugin enabled
func setupWireGuardTest(t *testing.T) (*TestHarness, context.CancelFunc) {
	harness := NewTestHarness(t)

	// Create jack.json with wireguard plugin enabled
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "wireguard": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	// Verify file was created
	data, err := os.ReadFile(jackPath)
	require.NoError(t, err)
	t.Logf("Created jack.json at %s: %s", jackPath, string(data))
	t.Logf("JACK_CONFIG_DIR=%s", os.Getenv("JACK_CONFIG_DIR"))

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	return harness, cancel
}

// TestWireGuardBasicTunnel tests basic WireGuard tunnel creation
func TestWireGuardBasicTunnel(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()

	// Configure WireGuard interface
	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-test": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-test",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=", // test key
				"listen_port": 51820,
				"address":     "10.100.0.1",
				"netmask":     "255.255.255.0",
				"mtu":         1420,
				"peers":       []interface{}{},
			},
		},
	}

	// Set and apply configuration
	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface created
	link, err := netlink.LinkByName("wg-test")
	require.NoError(t, err, "WireGuard interface should be created")
	assert.Equal(t, "wg-test", link.Attrs().Name)
	assert.Equal(t, 1420, link.Attrs().MTU, "MTU should be set correctly")

	// Verify interface has IP address
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1, "Should have one IP address")
	assert.Equal(t, "10.100.0.1/24", addrs[0].IPNet.String())

	// Verify WireGuard configuration with wg command
	cmd := exec.Command("wg", "show", "wg-test")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "wg show should succeed")

	outputStr := string(output)
	assert.Contains(t, outputStr, "listening port: 51820", "Listen port should be configured")
	assert.Contains(t, outputStr, "private key:", "Private key should be set")

	// Cleanup
	harness.DeleteInterface("wg-test")
}

// TestWireGuardWithPeer tests WireGuard tunnel with peer configuration
func TestWireGuardWithPeer(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()

	// Start daemon



	// Configure WireGuard interface with peer
	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-peer": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-peer",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=",
				"listen_port": 51821,
				"address":     "10.100.1.1",
				"netmask":     "255.255.255.0",
				"peers": []interface{}{
					map[string]interface{}{
						"public_key":            "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
						"endpoint":              "192.168.1.100:51820",
						"allowed_ips":           []string{"10.200.0.0/24"},
						"persistent_keepalive":  25,
						"comment":               "Test peer",
					},
				},
			},
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface created
	link, err := netlink.LinkByName("wg-peer")
	require.NoError(t, err)

	// Verify peer configuration with wg command
	cmd := exec.Command("wg", "show", "wg-peer")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "peer: xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=", "Peer should be configured")
	assert.Contains(t, outputStr, "endpoint: 192.168.1.100:51820", "Peer endpoint should be set")
	assert.Contains(t, outputStr, "allowed ips: 10.200.0.0/24", "Allowed IPs should be set")
	assert.Contains(t, outputStr, "persistent keepalive: every 25 seconds", "Keepalive should be set")

	// Verify route created for allowed-ips
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	require.NoError(t, err)

	foundRoute := false
	for _, route := range routes {
		if route.Dst != nil && route.Dst.String() == "10.200.0.0/24" {
			foundRoute = true
			break
		}
	}
	assert.True(t, foundRoute, "Route should be created for peer allowed-ips")

	// Cleanup
	harness.DeleteInterface("wg-peer")
}

// TestWireGuardMultiplePeers tests WireGuard with multiple peers
func TestWireGuardMultiplePeers(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()




	// Configure with multiple peers
	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-multi": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-multi",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=",
				"listen_port": 51822,
				"address":     "10.100.2.1",
				"netmask":     "255.255.255.0",
				"peers": []interface{}{
					map[string]interface{}{
						"public_key":  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
						"endpoint":    "192.168.1.100:51820",
						"allowed_ips": []string{"10.200.0.0/24"},
						"comment":     "Peer 1",
					},
					map[string]interface{}{
						"public_key":  "Wplla46JbD53Am3eVu1S7yrLnCOKFIZBF3W9xfvNP0Y=",
						"endpoint":    "192.168.1.101:51820",
						"allowed_ips": []string{"10.201.0.0/24", "10.202.0.0/24"},
						"comment":     "Peer 2",
					},
				},
			},
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify both peers configured
	cmd := exec.Command("wg", "show", "wg-multi", "peers")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	outputStr := string(output)
	peers := strings.Split(strings.TrimSpace(outputStr), "\n")
	assert.Len(t, peers, 2, "Should have 2 peers configured")

	// Verify routes for all allowed-ips
	link, err := netlink.LinkByName("wg-multi")
	require.NoError(t, err)

	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	require.NoError(t, err)

	expectedRoutes := map[string]bool{
		"10.200.0.0/24": false,
		"10.201.0.0/24": false,
		"10.202.0.0/24": false,
	}

	for _, route := range routes {
		if route.Dst != nil {
			dst := route.Dst.String()
			if _, ok := expectedRoutes[dst]; ok {
				expectedRoutes[dst] = true
			}
		}
	}

	for dst, found := range expectedRoutes {
		assert.True(t, found, "Route for %s should be created", dst)
	}

	// Cleanup
	harness.DeleteInterface("wg-multi")
}

// TestWireGuardDisableEnable tests disabling and re-enabling tunnel
func TestWireGuardDisableEnable(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()




	// Configure enabled tunnel
	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-toggle": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-toggle",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=",
				"listen_port": 51823,
				"address":     "10.100.3.1",
				"netmask":     "255.255.255.0",
				"peers":       []interface{}{},
			},
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface exists
	_, err = netlink.LinkByName("wg-toggle")
	require.NoError(t, err, "Interface should exist when enabled")

	// Disable tunnel
	vpnConfig["interfaces"].(map[string]interface{})["wg-toggle"].(map[string]interface{})["enabled"] = false

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface removed
	_, err = netlink.LinkByName("wg-toggle")
	assert.Error(t, err, "Interface should be removed when disabled")

	// Re-enable tunnel
	vpnConfig["interfaces"].(map[string]interface{})["wg-toggle"].(map[string]interface{})["enabled"] = true

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface recreated
	link, err := netlink.LinkByName("wg-toggle")
	require.NoError(t, err, "Interface should be recreated when re-enabled")
	assert.Equal(t, "wg-toggle", link.Attrs().Name)

	// Cleanup
	harness.DeleteInterface("wg-toggle")
}

// TestWireGuardConfigurationChange tests modifying tunnel configuration
func TestWireGuardConfigurationChange(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()




	// Initial configuration
	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-change": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-change",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=",
				"listen_port": 51824,
				"address":     "10.100.4.1",
				"netmask":     "255.255.255.0",
				"mtu":         1420,
				"peers":       []interface{}{},
			},
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify initial configuration
	link, err := netlink.LinkByName("wg-change")
	require.NoError(t, err)
	assert.Equal(t, 1420, link.Attrs().MTU)

	// Change MTU
	vpnConfig["interfaces"].(map[string]interface{})["wg-change"].(map[string]interface{})["mtu"] = 1400

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify MTU changed
	link, err = netlink.LinkByName("wg-change")
	require.NoError(t, err)
	assert.Equal(t, 1400, link.Attrs().MTU, "MTU should be updated")

	// Add a peer
	vpnConfig["interfaces"].(map[string]interface{})["wg-change"].(map[string]interface{})["peers"] = []interface{}{
		map[string]interface{}{
			"public_key":  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
			"allowed_ips": []string{"10.210.0.0/24"},
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify peer added
	cmd := exec.Command("wg", "show", "wg-change", "peers")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=", "Peer should be added")

	// Cleanup
	harness.DeleteInterface("wg-change")
}

// TestWireGuardIdempotency tests that reapplying same config is idempotent
func TestWireGuardIdempotency(t *testing.T) {
	harness, cancel := setupWireGuardTest(t)
	defer cancel()
	defer harness.Cleanup()




	vpnConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{
			"wg-idem": map[string]interface{}{
				"type":        "wireguard",
				"enabled":     true,
				"device_name": "wg-idem",
				"private_key": "YFAnNRfU+mHMi80r7ivXd5drGO8g8J9gOFZKT6LfMlE=",
				"listen_port": 51825,
				"address":     "10.100.5.1",
				"netmask":     "255.255.255.0",
				"peers": []interface{}{
					map[string]interface{}{
						"public_key":  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
						"allowed_ips": []string{"10.220.0.0/24"},
					},
				},
			},
		},
	}

	// Apply configuration first time
	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Get initial interface index
	link1, err := netlink.LinkByName("wg-idem")
	require.NoError(t, err)
	index1 := link1.Attrs().Index

	// Apply same configuration again
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "vpn",
		Value:   vpnConfig,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify interface still exists with same index (wasn't recreated)
	link2, err := netlink.LinkByName("wg-idem")
	require.NoError(t, err)
	index2 := link2.Attrs().Index

	assert.Equal(t, index1, index2, "Interface should not be recreated when applying same config")

	// Verify peer still configured
	cmd := exec.Command("wg", "show", "wg-idem", "peers")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(output), "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=")

	// Cleanup
	harness.DeleteInterface("wg-idem")
}
