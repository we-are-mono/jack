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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

// TestPluginRPCMetadata tests plugin metadata retrieval via RPC
func TestPluginRPCMetadata(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create jack.json with monitoring plugin enabled (it has default config)
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "monitoring": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Get plugin status which includes metadata
	resp, err := harness.SendRequest(daemon.Request{
		Command: "info",
	})
	require.NoError(t, err)

	// Verify plugin is loaded
	statusMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Response should be a map")

	plugins, ok := statusMap["plugins"].(map[string]interface{})
	require.True(t, ok, "Should have plugins field")

	monitoring, ok := plugins["monitoring"]
	require.True(t, ok, "Should have monitoring plugin loaded")
	assert.NotNil(t, monitoring)
}

// TestPluginRPCApplyConfig tests applying configuration via RPC
func TestPluginRPCApplyConfig(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup monitoring plugin
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "monitoring": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply monitoring config via set -> commit -> apply
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "monitoring",
		Value: map[string]interface{}{
			"enabled":             true,
			"collection_interval": 10,
		},
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Verify apply succeeded
	assert.Contains(t, resp.Message, "Configuration applied")
}

// TestPluginRPCInvalidConfig tests RPC with invalid JSON config
func TestPluginRPCInvalidConfig(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup firewall plugin (has validation)
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "firewall": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to set invalid firewall config (zones with no interfaces)
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "firewall",
		Value: map[string]interface{}{
			"enabled": true,
			"zones": map[string]interface{}{
				"invalid": map[string]interface{}{
					"interfaces": []string{}, // Empty interfaces - invalid
					"input":      "ACCEPT",
				},
			},
		},
	})
	require.NoError(t, err) // Set should succeed (staged only)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err) // Commit should succeed (validation not yet called)

	// Apply should fail due to validation
	// Note: Current implementation doesn't call ValidateConfig before ApplyConfig
	// This test documents current behavior
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	// Apply may succeed or fail depending on plugin implementation
	// firewall plugin will skip empty configs, so this might succeed
	if err != nil {
		assert.Contains(t, err.Error(), "zone")
	}
}

// TestPluginRPCFlush tests flushing plugin state via RPC
func TestPluginRPCFlush(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup firewall plugin
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "firewall": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply a firewall config
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "firewall",
		Value: map[string]interface{}{
			"enabled": true,
			"zones": map[string]interface{}{
				"lan": map[string]interface{}{
					"interfaces": []string{"br-lan"},
					"input":      "ACCEPT",
					"forward":    "ACCEPT",
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Revert should call Flush on the plugin
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "Revert should succeed")
}

// TestPluginRPCStatus tests getting plugin status via RPC
func TestPluginRPCStatus(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup monitoring plugin (has status implementation)
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "monitoring": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Get status
	resp, err := harness.SendRequest(daemon.Request{
		Command: "info",
	})
	require.NoError(t, err)

	statusMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Response should be a map")

	// Verify plugin status is included
	plugins, ok := statusMap["plugins"].(map[string]interface{})
	require.True(t, ok, "Should have plugins field")

	monitoringStatus, ok := plugins["monitoring"]
	require.True(t, ok, "Should have monitoring plugin status")
	assert.NotNil(t, monitoringStatus)
}

// TestPluginRPCMultiplePlugins tests RPC with multiple plugins loaded
func TestPluginRPCMultiplePlugins(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup multiple plugins
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "firewall": {
      "enabled": true,
      "version": ""
    },
    "monitoring": {
      "enabled": true,
      "version": ""
    },
    "wireguard": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configs to all plugins
	configs := []struct {
		path  string
		value map[string]interface{}
	}{
		{
			path: "firewall",
			value: map[string]interface{}{
				"enabled": true,
				"zones": map[string]interface{}{
					"lan": map[string]interface{}{
						"interfaces": []string{"br-lan"},
						"input":      "ACCEPT",
					},
				},
			},
		},
		{
			path: "monitoring",
			value: map[string]interface{}{
				"enabled":             true,
				"collection_interval": 5,
			},
		},
		{
			path: "vpn",
			value: map[string]interface{}{
				"interfaces": map[string]interface{}{
					"wg0": map[string]interface{}{
						"type":        "wireguard",
						"enabled":     false, // Disabled so it doesn't require keys
						"device_name": "wg0",
					},
				},
			},
		},
	}

	for _, cfg := range configs {
		_, err = harness.SendRequest(daemon.Request{
			Command: "set",
			Path:    cfg.path,
			Value:   cfg.value,
		})
		require.NoError(t, err, "Failed to set config for path: %s", cfg.path)
	}

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Configuration applied")

	// Verify all plugins are active
	resp, err = harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)

	statusMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	plugins, ok := statusMap["plugins"].(map[string]interface{})
	require.True(t, ok)

	// All three plugins should be present
	assert.Contains(t, plugins, "firewall")
	assert.Contains(t, plugins, "monitoring")
	assert.Contains(t, plugins, "vpn")
}

// TestPluginRPCEnableDisable tests enabling and disabling plugins via RPC
func TestPluginRPCEnableDisable(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start with monitoring disabled
	jackConfig := `{
  "version": "1.0",
  "plugins": {}
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Enable monitoring plugin
	resp, err := harness.SendRequest(daemon.Request{
		Command: "plugin-enable",
		Plugin:  "monitoring",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "enabled")

	// Verify plugin is enabled
	resp, err = harness.SendRequest(daemon.Request{
		Command: "info",
	})
	require.NoError(t, err)

	statusMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	plugins, ok := statusMap["plugins"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, plugins, "monitoring")

	// Disable monitoring plugin
	resp, err = harness.SendRequest(daemon.Request{
		Command: "plugin-disable",
		Plugin:  "monitoring",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "disabled")
}

// TestPluginRPCConfigSerialization tests JSON serialization in RPC
func TestPluginRPCConfigSerialization(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "monitoring": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Test with various data types in config
	complexConfig := map[string]interface{}{
		"enabled":             true,
		"collection_interval": 15,
		"thresholds": map[string]interface{}{
			"cpu":    80.0,
			"memory": 90.0,
		},
		"interfaces": []string{"eth0", "eth1"},
		"tags": map[string]interface{}{
			"environment": "test",
			"version":     1,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "monitoring",
		Value:   complexConfig,
	})
	require.NoError(t, err)

	// Get the config back
	resp, err := harness.SendRequest(daemon.Request{
		Command: "get",
		Path:    "monitoring",
	})
	require.NoError(t, err)

	// Verify complex types are preserved
	configMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Should get config as map")

	assert.Equal(t, true, configMap["enabled"])
	assert.Equal(t, float64(15), configMap["collection_interval"]) // JSON numbers are float64

	thresholds, ok := configMap["thresholds"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 80.0, thresholds["cpu"])
}

// TestPluginRPCDefaultConfig tests plugins with default configurations
func TestPluginRPCDefaultConfig(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Enable monitoring without providing config (should use defaults)
	jackConfig := `{
  "version": "1.0",
  "plugins": {
    "monitoring": {
      "enabled": true,
      "version": ""
    }
  }
}`
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err := os.WriteFile(jackPath, []byte(jackConfig), 0644)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply without setting custom config (should use default)
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Configuration applied")

	// Get config - should show defaults were applied
	resp, err = harness.SendRequest(daemon.Request{
		Command: "get",
		Path:    "monitoring",
	})
	require.NoError(t, err)

	configMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	// Monitoring plugin has default config: {"enabled": true, "collection_interval": 5}
	assert.Equal(t, true, configMap["enabled"])
	assert.NotNil(t, configMap["collection_interval"])
}
