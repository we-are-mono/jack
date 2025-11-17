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
)

// TestPluginCLIMonitorStats tests executing "monitor stats" CLI command
func TestPluginCLIMonitorStats(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute "monitor stats" command
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "monitor stats",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.True(t, resp.Success, "monitor stats should succeed")
	assert.NotNil(t, resp.Data, "should return stats data")

	// Output should contain system metrics
	output, ok := resp.Data.(string)
	require.True(t, ok, "output should be string")
	assert.Contains(t, output, "System Metrics")
	t.Logf("Monitor stats output: %s", output)
}

// TestPluginCLIMonitorBandwidth tests executing "monitor bandwidth" CLI command
func TestPluginCLIMonitorBandwidth(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute "monitor bandwidth eth0" command
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "monitor bandwidth",
		CLIArgs:    []string{eth0},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// In test environment, dummy interfaces may not have network stats
	// So we accept either success OR "interface not found" error
	if !resp.Success {
		t.Logf("Monitor bandwidth result: %s (expected in test environment)", resp.Error)
		// Should fail with interface not found, not other errors
		assert.Contains(t, resp.Error, "not found", "should fail with 'not found' error for dummy interface")
	} else {
		// If it succeeds, validate output
		assert.NotNil(t, resp.Data, "should return bandwidth data")
		output, ok := resp.Data.(string)
		require.True(t, ok, "output should be string")
		assert.Contains(t, output, eth0)
		t.Logf("Monitor bandwidth output: %s", output)
	}
}

// TestPluginCLIInvalidPlugin tests executing CLI command on non-existent plugin
func TestPluginCLIInvalidPlugin(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Try to execute command on non-existent plugin
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "nonexistent-plugin",
		CLICommand: "some command",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.Success, "should fail for non-existent plugin")
	assert.Contains(t, resp.Error, "not found")
}

// TestPluginCLIInvalidCommand tests executing invalid CLI command
func TestPluginCLIInvalidCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute invalid command
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "invalid command",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.Success, "should fail for invalid command")
	assert.NotEmpty(t, resp.Error)
}

// TestPluginCLIEmptyCommand tests executing empty CLI command
func TestPluginCLIEmptyCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute empty command
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.Success, "should fail for empty command")
	assert.NotEmpty(t, resp.Error)
}

// TestPluginCLIMonitorInvalidSubcommand tests invalid monitor subcommand
func TestPluginCLIMonitorInvalidSubcommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute monitor with invalid subcommand
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "monitor invalid",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.Success, "should fail for invalid subcommand")
	assert.Contains(t, resp.Error, "unknown")
}

// TestPluginCLIMonitorMissingSubcommand tests monitor without subcommand
func TestPluginCLIMonitorMissingSubcommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Execute monitor without subcommand
	req := daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "monitoring",
		CLICommand: "monitor",
		CLIArgs:    []string{},
	}

	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.False(t, resp.Success, "should fail without subcommand")
	assert.Contains(t, resp.Error, "subcommand")
}

// TestPluginCLIWithDifferentPluginNames tests that plugin-cli works with different plugin name formats
func TestPluginCLIWithDifferentPluginNames(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Test with both "monitoring" (namespace) and potential plugin name variations
	testCases := []struct {
		name       string
		pluginName string
		shouldWork bool
	}{
		{"namespace", "monitoring", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := daemon.Request{
				Command:    "plugin-cli",
				Plugin:     tc.pluginName,
				CLICommand: "monitor stats",
				CLIArgs:    []string{},
			}

			resp, err := harness.SendRequest(req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			if tc.shouldWork {
				assert.True(t, resp.Success, "should work with %s", tc.name)
			} else {
				assert.False(t, resp.Success, "should not work with %s", tc.name)
			}
		})
	}
}
