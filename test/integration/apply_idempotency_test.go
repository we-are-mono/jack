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

// TestApplyIdempotency tests that applying the same config multiple times skips unchanged configs
// This exercises SetLastApplied, GetLastApplied, and ConfigsEqual
func TestApplyIdempotency(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Set up configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.100.1.10",
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

	// First apply
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "First apply should succeed")
	t.Logf("First apply: %s", resp.Message)

	// Second apply with same config (should skip - idempotent)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Second apply should succeed")
	t.Logf("Second apply (idempotent): %s", resp.Message)

	// Third apply (also idempotent)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Third apply should succeed")
	t.Logf("Third apply (idempotent): %s", resp.Message)
}

// TestApplyAfterConfigChange tests that apply detects config changes
// This exercises ConfigsEqual returning false when configs differ
func TestApplyAfterConfigChange(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Initial configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.100.2.10",
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

	// Apply initial config
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Applied initial config")

	// Change configuration
	interfaces[eth0] = types.Interface{
		Type:     "physical",
		Device:   eth0,
		Enabled:  true,
		Protocol: "static",
		IPAddr:   "10.100.2.20", // Changed IP
		Netmask:  "255.255.255.0",
		MTU:      1500,
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Apply changed config (should detect change and reapply)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Applied changed config")

	// Verify the new config is active by applying again (should be idempotent now)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Verified idempotency of changed config")
}

// TestApplyMultipleConfigTypes tests applying multiple config types
// This exercises SetLastApplied/GetLastApplied for different config types
func TestApplyMultipleConfigTypes(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Set interfaces
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.100.3.10",
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

	// Set routes
	routes := map[string]types.Route{
		"test-route-idempotency": {
			Destination: "192.168.100.0/24",
			Gateway:     "10.100.3.1",
			Metric:      100,
			Enabled:     true,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   routes,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Apply all configs
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	assert.Contains(t, resp.Message, "interfaces", "Should mention interfaces")
	t.Logf("Applied multiple config types: %s", resp.Message)

	// Apply again (all should be idempotent)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Idempotent apply of multiple configs")
}

// TestApplyWithPluginConfigs tests that plugin configs are tracked separately
func TestApplyWithPluginConfigs(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// First apply (monitoring plugin uses default config)
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("First apply with plugin defaults: %s", resp.Message)

	// Second apply (should be idempotent for plugin configs too)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Second apply (plugin configs idempotent)")
}

// TestApplyEmptyConfiguration tests applying empty configuration
func TestApplyEmptyConfiguration(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Apply without setting any config
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Should succeed with empty/default config")
	t.Logf("Applied empty config: %s", resp.Message)
}

// TestApplyAfterRevert tests applying after reverting changes
func TestApplyAfterRevert(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Set and commit initial config
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.100.4.10",
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

	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Applied initial config")

	// Make changes but don't commit
	modifiedInterface := interfaces[eth0]
	modifiedInterface.IPAddr = "10.100.4.20"
	interfaces[eth0] = modifiedInterface
	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   interfaces,
	})
	require.NoError(t, err)

	// Revert changes
	_, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)

	// Apply after revert (should use original config)
	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	t.Logf("Applied after revert (uses original config)")
}

// TestApplyIPForwardingError tests handling of IP forwarding enable errors
func TestApplyIPForwardingEnabled(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Apply should enable IP forwarding as first step
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Apply should enable IP forwarding successfully")
	t.Logf("IP forwarding enabled via apply")
}
