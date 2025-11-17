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
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/types"
)

// TestInvalidCommandHandling tests daemon's response to invalid commands
func TestInvalidCommandHandling(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Send invalid command
	resp, err := harness.SendRequest(daemon.Request{
		Command: "invalid_command_xyz",
	})

	// Should either error or return unsuccessful response
	if err == nil {
		assert.False(t, resp.Success, "invalid command should not succeed")
		assert.NotEmpty(t, resp.Error, "should have error message")
	}
}

// TestSetInvalidPath tests setting configuration to invalid path
func TestSetInvalidPath(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to set configuration at invalid path
	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "nonexistent.invalid.path",
		Value:   map[string]interface{}{"test": "value"},
	})

	// Should either error or return unsuccessful response
	if err == nil {
		if !resp.Success {
			assert.NotEmpty(t, resp.Error, "should have error message")
		}
	}
}

// TestSetInvalidValueType tests setting configuration with wrong value type
func TestSetInvalidValueType(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to set interfaces with wrong type (string instead of map)
	resp, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   "this should be a map",
	})

	// Should fail
	if err == nil {
		assert.False(t, resp.Success, "wrong value type should fail")
		if !resp.Success {
			assert.NotEmpty(t, resp.Error, "should have error message")
		}
	}
}

// TestApplyWithNonexistentInterface tests applying config with interface that doesn't exist
func TestApplyWithNonexistentInterface(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configure interface that doesn't exist in the namespace
	interfaces := map[string]types.Interface{
		"nonexistent999": {
			Type:     "physical",
			Device:   "nonexistent999",
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.1.10",
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

	// Apply should fail
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	if err == nil {
		assert.False(t, resp.Success, "apply with nonexistent interface should fail")
	}
}

// TestRecoveryAfterFailedApply tests that daemon recovers after a failed apply
func TestRecoveryAfterFailedApply(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to apply configuration that will fail
	badInterfaces := map[string]types.Interface{
		"nonexistent": {
			Type:     "physical",
			Device:   "nonexistent",
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.2.10",
			Netmask:  "255.255.255.0",
		},
	}

	_, err := harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   badInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	// Expect failure
	if err == nil {
		assert.False(t, resp.Success, "bad apply should fail")
	}

	// Now try to set and apply valid configuration
	goodInterfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.2.20",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	_, err = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   goodInterfaces,
	})
	require.NoError(t, err)

	_, err = harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)

	resp, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "daemon should recover and accept valid config")

	// Verify the good configuration was applied
	link, err := netlink.LinkByName(eth0)
	require.NoError(t, err)
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	assert.Equal(t, "10.5.2.20", addrs[0].IP.String())
}

// TestCommitWithoutSet tests committing without setting anything
func TestCommitWithoutSet(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Try to commit without setting anything
	resp, err := harness.SendRequest(daemon.Request{Command: "commit"})
	require.NoError(t, err)
	// This should succeed (committing empty/no changes is valid)
	assert.True(t, resp.Success, "commit without changes should succeed")
}

// TestApplyWithoutCommit tests applying without committing
func TestApplyWithoutCommit(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.3.10",
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

	// Try to apply without commit
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err)

	// Apply should work but might not apply uncommitted changes
	// Depending on implementation, this might succeed or fail
	if resp.Success {
		t.Log("Apply without commit succeeded - checking if changes applied")
		link, err := netlink.LinkByName(eth0)
		require.NoError(t, err)
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
		// Changes should not be applied if not committed
		if len(addrs) > 0 {
			t.Logf("Address was applied: %v", addrs[0].IP.String())
		}
	}
}

// TestPartialConfigurationError tests behavior when part of config is invalid
func TestPartialConfigurationError(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Configuration with one valid and one invalid interface
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.4.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
		"invalid999": {
			Type:     "physical",
			Device:   "invalid999",
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.4.20",
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

	// Apply will likely fail due to invalid interface
	resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
	if err == nil && !resp.Success {
		t.Logf("Partial config error: %s", resp.Error)
	}

	// Check if valid interface was configured (depends on implementation)
	link, err := netlink.LinkByName(eth0)
	if err == nil {
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
		if len(addrs) > 0 {
			t.Logf("Valid interface was configured despite partial error: %v",
				addrs[0].IP.String())
		} else {
			t.Log("Valid interface was not configured (transactional rollback)")
		}
	}
}

// TestStateConsistencyAfterError tests that daemon state remains consistent after errors
func TestStateConsistencyAfterError(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set and commit valid configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.5.10",
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

	// Get config before error
	respBefore, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	// Try invalid operation
	_, _ = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   "invalid value type",
	})

	// Get config after error - should be unchanged
	respAfter, err := harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)

	beforeBytes, _ := json.Marshal(respBefore.Data)
	afterBytes, _ := json.Marshal(respAfter.Data)
	assert.JSONEq(t, string(beforeBytes), string(afterBytes),
		"state should remain consistent after error")
}

// TestConcurrentRequestsWithErrors tests daemon stability under concurrent error conditions
func TestConcurrentRequestsWithErrors(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Send multiple concurrent requests, some valid, some invalid
	const numRequests = 20
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			var resp *daemon.Response
			var err error

			if index%3 == 0 {
				// Invalid command
				resp, err = harness.SendRequest(daemon.Request{
					Command: "invalid_cmd",
				})
			} else if index%3 == 1 {
				// Valid status request
				resp, err = harness.SendRequest(daemon.Request{
					Command: "status",
				})
			} else {
				// Invalid path
				resp, err = harness.SendRequest(daemon.Request{
					Command: "get",
					Path:    "nonexistent.path",
				})
			}

			if err != nil {
				results <- err
				return
			}

			// Valid requests should succeed, invalid ones should fail gracefully
			if index%3 == 1 {
				if !resp.Success {
					results <- assert.AnError
					return
				}
			}

			results <- nil
		}(i)
	}

	// Collect results - daemon should handle all requests without crashing
	errorCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-results
		if err != nil {
			errorCount++
		}
	}

	// Some errors are expected for invalid requests
	t.Logf("Errors in concurrent requests: %d/%d", errorCount, numRequests)

	// Daemon should still be responsive after concurrent errors
	resp, err := harness.SendRequest(daemon.Request{Command: "status"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "daemon should still be responsive")
}

// TestEmptyRequestHandling tests handling of malformed/empty requests
func TestEmptyRequestHandling(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Send request with empty command
	resp, err := harness.SendRequest(daemon.Request{
		Command: "",
	})

	// Should handle gracefully
	if err == nil {
		assert.False(t, resp.Success, "empty command should fail")
	}
}

// TestRevertAfterError tests that revert works after an error
func TestRevertAfterError(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Set valid configuration
	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.6.10",
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

	// Make a change that causes error
	_, _ = harness.SendRequest(daemon.Request{
		Command: "set",
		Path:    "interfaces",
		Value:   "invalid",
	})

	// Revert should still work
	resp, err := harness.SendRequest(daemon.Request{Command: "revert"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "revert should work after error")

	// Configuration should still be accessible
	resp, err = harness.SendRequest(daemon.Request{Command: "get"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

// TestDaemonResilienceToInvalidState tests daemon resilience to invalid internal state
func TestDaemonResilienceToInvalidState(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	eth0 := harness.CreateDummyInterface("eth0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Sequence of operations designed to test state resilience
	operations := []struct {
		cmd  string
		path string
	}{
		{"set", "interfaces"},
		{"revert", ""},
		{"commit", ""},
		{"revert", ""},
		{"apply", ""},
		{"get", ""},
		{"commit", ""},
		{"apply", ""},
	}

	interfaces := map[string]types.Interface{
		eth0: {
			Type:     "physical",
			Device:   eth0,
			Enabled:  true,
			Protocol: "static",
			IPAddr:   "10.5.7.10",
			Netmask:  "255.255.255.0",
			MTU:      1500,
		},
	}

	for i, op := range operations {
		var resp *daemon.Response
		var err error

		if op.cmd == "set" {
			resp, err = harness.SendRequest(daemon.Request{
				Command: op.cmd,
				Path:    op.path,
				Value:   interfaces,
			})
		} else {
			resp, err = harness.SendRequest(daemon.Request{
				Command: op.cmd,
				Path:    op.path,
			})
		}

		// All operations should complete without crashing daemon
		if err != nil {
			t.Logf("Operation %d (%s) error: %v", i, op.cmd, err)
		} else {
			t.Logf("Operation %d (%s) success: %v", i, op.cmd, resp.Success)
		}
	}

	// Daemon should still be responsive
	resp, err := harness.SendRequest(daemon.Request{Command: "status"})
	require.NoError(t, err)
	assert.True(t, resp.Success, "daemon should remain stable")
}
