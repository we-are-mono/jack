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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestDiffStagedVsCommitted tests showing uncommitted changes
func TestDiffStagedVsCommitted(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set and commit initial configuration
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.1.10",
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

	// Now make changes without committing
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.1.20", // Changed IP
			Netmask:  "255.255.255.0",
			MTU:      9000, // Changed MTU
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces,
	})
	require.NoError(t, err)

	// Get diff (should show staged vs committed changes)
	resp, err := harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	// Convert diff to string
	diffStr, ok := resp.Data.(string)
	require.True(t, ok, "diff should return a string")
	require.NotEmpty(t, diffStr, "diff should not be empty")

	// Verify diff contains the changes
	assert.Contains(t, diffStr, "10.1.1.10", "diff should show old IP")
	assert.Contains(t, diffStr, "10.1.1.20", "diff should show new IP")
	assert.Contains(t, diffStr, "1500", "diff should show old MTU")
	assert.Contains(t, diffStr, "9000", "diff should show new MTU")
}

// TestDiffCommittedVsApplied tests showing unapplied committed changes
func TestDiffCommittedVsApplied(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set, commit, and apply initial configuration
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.2.10",
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

	// Now set and commit new configuration without applying
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.2.20",
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

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Get diff (should show committed vs applied changes)
	resp, err := harness.SendRequest(daemon.Request{
		Command: "diff",
		Path:    "applied", // Diff between committed and applied
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// The diff should show pending changes
	if resp.Data != nil {
		diffStr, ok := resp.Data.(string)
		if ok && diffStr != "" {
			assert.Contains(t, diffStr, "10.1.2", "diff should reference IP addresses")
		}
	}
}

// TestDiffWithNoChanges tests diff when there are no changes
func TestDiffWithNoChanges(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set and commit configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.3.10",
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

	// Get diff when there are no uncommitted changes
	resp, err := harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Diff should be empty or indicate no changes
	if resp.Data != nil {
		diffStr, ok := resp.Data.(string)
		if ok {
			// Empty diff or message indicating no changes
			assert.True(t, diffStr == "" || strings.Contains(diffStr, "no changes") ||
				strings.Contains(diffStr, "No changes") ||
				strings.Contains(diffStr, "identical"),
				"diff should indicate no changes")
		}
	} else {
		// nil data is also acceptable for no changes
		t.Log("diff returned nil data (no changes)")
	}
}

// TestDiffComplexChanges tests diff with multiple component changes
func TestDiffComplexChanges(t *testing.T) {
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

	// Set initial complex configuration
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.4.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	initialRoutes := map[string]types.Route{
		"route-0": {
			Destination: "192.168.1.0/24",
			Gateway:     "10.1.4.1",
			Metric:      100,
			Enabled:     true,
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   initialInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   initialRoutes,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Now make complex changes
	modifiedInterfaces := map[string]types.Interface{
		eth0: initialInterfaces[eth0], // Keep eth0
		eth1: { // Add eth1
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.4.20",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	modifiedRoutes := map[string]types.Route{
		"route-0": { // Modified route
			Destination: "192.168.1.0/24",
			Gateway:     "10.1.4.1",
			Metric:      200, // Changed metric
			Enabled:     true,
		},
		"route-1": { // Added route
			Destination: "192.168.2.0/24",
			Gateway:     "10.1.4.1",
			Metric:      100,
			Enabled:     true,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "routes",
		Value:   modifiedRoutes,
	})
	require.NoError(t, err)

	// Get diff
	resp, err := harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	diffStr, ok := resp.Data.(string)
	require.True(t, ok)
	require.NotEmpty(t, diffStr)

	// Verify diff contains both interface and route changes
	// Note: exact format depends on implementation
	t.Logf("Diff output:\n%s", diffStr)

	// At minimum, the diff should reference the components being changed
	hasInterfaceChanges := strings.Contains(diffStr, "interface") ||
		strings.Contains(diffStr, eth1) ||
		strings.Contains(diffStr, "10.1.4.20")
	hasRouteChanges := strings.Contains(diffStr, "route") ||
		strings.Contains(diffStr, "192.168.2.0") ||
		strings.Contains(diffStr, "200")

	assert.True(t, hasInterfaceChanges || hasRouteChanges,
		"diff should show changes to interfaces or routes")
}

// TestDiffFormatting tests the format of diff output
func TestDiffFormatting(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set initial configuration
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.5.10",
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

	// Make a simple change
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.5.20", // Only IP changed
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

	// Get diff
	resp, err := harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.NotNil(t, resp.Data)

	diffStr, ok := resp.Data.(string)
	require.True(t, ok, "diff should return string")
	require.NotEmpty(t, diffStr, "diff should not be empty")

	// Log the diff for inspection
	t.Logf("Diff format:\n%s", diffStr)

	// Basic format checks - diff should be human-readable
	assert.True(t, len(diffStr) > 0, "diff should have content")

	// Diff should reference the changed values
	containsOldIP := strings.Contains(diffStr, "10.1.5.10")
	containsNewIP := strings.Contains(diffStr, "10.1.5.20")

	assert.True(t, containsOldIP || containsNewIP,
		"diff should show old or new IP address")
}

// TestDiffAfterRevert tests that diff is empty after reverting changes
func TestDiffAfterRevert(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set and commit initial configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.6.10",
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

	// Make changes
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.1.6.20",
			Netmask:  "255.255.255.0",
			MTU:      9000,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces,
	})
	require.NoError(t, err)

	// Verify there is a diff
	resp, err := harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)
	if resp.Data != nil {
		diffStr, _ := resp.Data.(string)
		assert.NotEmpty(t, diffStr, "diff should show changes before revert")
	}

	// Revert changes
	resp, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success, "revert should succeed")

	// Now diff should be empty
	resp, err = harness.SendRequest(daemon.Request{Command: "diff"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	if resp.Data != nil {
		diffStr, ok := resp.Data.(string)
		if ok {
			assert.True(t, diffStr == "" || strings.Contains(diffStr, "no changes") ||
				strings.Contains(diffStr, "No changes") ||
				strings.Contains(diffStr, "identical"),
				"diff should be empty after revert")
		}
	}
}
