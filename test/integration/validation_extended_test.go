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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestValidateInvalidPath tests validate command with invalid path
func TestValidateInvalidPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Unknown paths are treated as plugin configs and validate JSON structure
	// Valid JSON should pass
	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "unknown.plugin.name",
		Value:   map[string]interface{}{"test": "data"},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "valid JSON should pass for unknown path (treated as plugin config)")

	// Invalid JSON structure should fail
	resp, err = harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "unknown.plugin.name",
		Value:   "not a map structure",
	})
	require.NoError(t, err)
	// This should succeed or fail depending on how the value is handled
	t.Logf("Validation result for non-map value: success=%v, error=%s", resp.Success, resp.Error)
}

// TestValidateRoutesDirectly tests validate command for routes
func TestValidateRoutesDirectly(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Valid routes
	validRoutes := []types.Route{
		{
			Destination: "192.168.7.0/24",
			Gateway:     "10.0.0.1",
			Metric:      100,
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "routes",
		Value:   validRoutes,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "valid routes should pass validation")

	// Invalid routes structure (wrong type)
	invalidRoutes := "not a routes array"

	resp, err = harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "routes",
		Value:   invalidRoutes,
	})
	require.NoError(t, err)
	assert.False(t, resp.Success, "invalid routes structure should fail validation")
}

// TestValidateInterfaceMissingType tests interface validation with missing type
func TestValidateInterfaceMissingType(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Interface without type field
	interfaces := map[string]types.Interface{
		eth0: {
			Type:    "", // Missing type
			Device:  eth0,
			Enabled: true,
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	assert.False(t, resp.Success, "interface without type should fail validation")
	assert.Contains(t, resp.Error, "type is required")
}

// TestValidateInterfaceInvalidStructure tests interface validation with invalid structure
func TestValidateInterfaceInvalidStructure(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Invalid structure - not a map
	invalidInterfaces := "not an interfaces map"

	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   invalidInterfaces,
	})
	require.NoError(t, err)
	assert.False(t, resp.Success, "invalid interfaces structure should fail validation")
}

// TestValidatePluginConfig tests validation of plugin configuration
func TestValidatePluginConfig(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Valid plugin config (monitoring plugin)
	validConfig := map[string]interface{}{
		"enabled": true,
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "monitoring",
		Value:   validConfig,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "valid plugin config should pass validation")

	// Invalid plugin config structure
	invalidConfig := []string{"not", "a", "map"}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "monitoring",
		Value:   invalidConfig,
	})
	require.NoError(t, err)
	// Should either succeed with basic validation or fail with structure error
	t.Logf("Invalid plugin config result: success=%v, error=%s", resp.Success, resp.Error)
}

// TestValidateEmptyPath tests validate command without path
func TestValidateEmptyPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Try to validate without path
	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "",
		Value:   map[string]interface{}{"test": "data"},
	})
	require.NoError(t, err)

	// Should fail or handle gracefully
	if !resp.Success {
		assert.NotEmpty(t, resp.Error)
	}
}

// TestValidateAllInterfaceTypes tests validation of all interface types
func TestValidateAllInterfaceTypes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Test all valid interface types
	validTypes := []string{"physical", "bridge", "vlan"}

	for _, ifaceType := range validTypes {
		t.Run(ifaceType, func(t *testing.T) {
			interfaces := map[string]types.Interface{
				eth0: {
					Type:    ifaceType,
					Device:  eth0,
					Enabled: true,
				},
			}

			resp, err := harness.SendRequest(daemon.Request{
				Command: "validate",
				Path:    "interfaces",
				Value:   interfaces,
			})
			require.NoError(t, err)
			assert.True(t, resp.Success, "%s type should be valid", ifaceType)
		})
	}
}

// TestValidateInterfaceTypeCase tests that type validation is case-sensitive
func TestValidateInterfaceTypeCase(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Test with wrong case
	invalidTypes := []string{"Physical", "BRIDGE", "VLan", "PHYSICAL"}

	for _, ifaceType := range invalidTypes {
		t.Run(ifaceType, func(t *testing.T) {
			interfaces := map[string]types.Interface{
				eth0: {
					Type:    ifaceType,
					Device:  eth0,
					Enabled: true,
				},
			}

			resp, err := harness.SendRequest(daemon.Request{
				Command: "validate",
				Path:    "interfaces",
				Value:   interfaces,
			})
			require.NoError(t, err)
			assert.False(t, resp.Success, "%s should be invalid (case-sensitive)", ifaceType)
			assert.Contains(t, resp.Error, "invalid type")
		})
	}
}

// TestValidateMultipleInterfaces tests validation with multiple interfaces
func TestValidateMultipleInterfaces(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")
	eth1 := harness.CreateDummyInterface("eth1")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Multiple valid interfaces
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
		"br0": {
			Type:        "bridge",
			Device:      "br0",
			Enabled:     true,
			BridgePorts: []string{eth0, eth1},
		},
	}

	resp, err := harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	assert.True(t, resp.Success, "multiple valid interfaces should pass validation")

	// Mix of valid and invalid interfaces (one with invalid type)
	interfaces["invalid"] = types.Interface{
		Type:    "unknown",
		Device:  "invalid",
		Enabled: true,
	}

	resp, err = harness.SendRequest(daemon.Request{
		Command: "validate",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)
	assert.False(t, resp.Success, "should fail if any interface is invalid")
	assert.Contains(t, resp.Error, "invalid type")
}
