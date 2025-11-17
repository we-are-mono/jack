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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestRevertUncommittedChanges tests discarding uncommitted staged changes
func TestRevertUncommittedChanges(t *testing.T) {
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
			IPAddr:   "10.2.1.10",
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

	// Get committed config to verify later
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	committedData := resp.Data

	// Make changes without committing
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.1.20", // Changed
			Netmask:  "255.255.255.0",
			MTU:      9000, // Changed
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces,
	})
	require.NoError(t, err)

	// Revert uncommitted changes
	resp, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success, "revert should succeed")

	// Get config after revert
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	// Verify config matches committed version
	committedJSON, _ := json.Marshal(committedData)
	revertedJSON, _ := json.Marshal(resp.Data)
	assert.JSONEq(t, string(committedJSON), string(revertedJSON),
		"config after revert should match committed config")
}

// TestRevertWithNoChanges tests revert when there are no pending changes
func TestRevertWithNoChanges(t *testing.T) {
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
			IPAddr:   "10.2.2.10",
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

	// Revert when there are no uncommitted changes
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	// Revert should succeed even with no changes
	assert.True(t, resp.Success, "revert should succeed even with no changes")

	// Verify config is unchanged
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	require.NotNil(t, resp.Data)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	assert.Equal(t, "10.2.2.10", config.Interfaces[eth0].IPAddr)
}

// TestRevertPartialChanges tests reverting some components while keeping others
func TestRevertPartialChanges(t *testing.T) {
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

	// Set and commit initial configuration for both interfaces
	initialInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.3.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.3.20",
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

	// Modify both interfaces
	modifiedInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.3.11", // Changed
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
		eth1: {
			Type:     "physical",
			Device:   eth1,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.3.21", // Changed
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

	// Revert all changes (jack revert reverts all pending changes)
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify both interfaces reverted to original values
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	require.Contains(t, config.Interfaces, eth1)
	assert.Equal(t, "10.2.3.10", config.Interfaces[eth0].IPAddr, "eth0 should be reverted")
	assert.Equal(t, "10.2.3.20", config.Interfaces[eth1].IPAddr, "eth1 should be reverted")
}

// TestRevertAfterCommit tests that revert doesn't affect committed config
func TestRevertAfterCommit(t *testing.T) {
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
			IPAddr:   "10.2.4.10",
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

	// Get committed config
	resp, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	committedData := resp.Data

	// Revert when already committed (should have no effect)
	resp, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Verify config unchanged
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	committedJSON, _ := json.Marshal(committedData)
	afterRevertJSON, _ := json.Marshal(resp.Data)
	assert.JSONEq(t, string(committedJSON), string(afterRevertJSON),
		"revert should not change committed config")
}

// TestRevertMultipleTimes tests that revert can be called multiple times safely
func TestRevertMultipleTimes(t *testing.T) {
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
			IPAddr:   "10.2.5.10",
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
			IPAddr:   "10.2.5.20",
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

	// Revert first time
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success, "first revert should succeed")

	// Revert second time (no changes to revert)
	resp, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success, "second revert should succeed")

	// Revert third time
	resp, err = harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success, "third revert should succeed")

	// Verify config still correct
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	assert.Equal(t, "10.2.5.10", config.Interfaces[eth0].IPAddr)
	assert.Equal(t, 1500, config.Interfaces[eth0].MTU)
}

// TestRevertThenModify tests setting new config after a revert
func TestRevertThenModify(t *testing.T) {
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
			IPAddr:   "10.2.6.10",
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

	// Make changes
	modifiedInterfaces1 := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.6.20",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces1,
	})
	require.NoError(t, err)

	// Revert
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	require.True(t, resp.Success)

	// Now make different changes
	modifiedInterfaces2 := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.2.6.30", // Different from first modification
			Netmask:  "255.255.255.0",
			MTU:      9000,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   modifiedInterfaces2,
	})
	require.NoError(t, err)

	// Commit the new changes
	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	// Verify new config is committed
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	dataBytes, err := json.Marshal(resp.Data)
	require.NoError(t, err)

	var config struct {
		Interfaces map[string]types.Interface `json:"interfaces"`
	}
	err = json.Unmarshal(dataBytes, &config)
	require.NoError(t, err)

	require.Contains(t, config.Interfaces, eth0)
	assert.Equal(t, "10.2.6.30", config.Interfaces[eth0].IPAddr,
		"should have new IP after revert and new modification")
	assert.Equal(t, 9000, config.Interfaces[eth0].MTU,
		"should have new MTU after revert and new modification")
}
