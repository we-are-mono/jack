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

// TestDaemonStartStop tests basic daemon lifecycle
func TestDaemonStartStop(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := harness.StartDaemon(ctx)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	// Wait for daemon to be ready
	harness.WaitForDaemon(5 * time.Second)

	// Test that daemon is responding
	resp, err := harness.SendRequest(daemon.Request{Command: "status"})
	require.NoError(t, err, "status request should succeed")
	assert.True(t, resp.Success, "status should return success")

	// Cancel context to stop daemon
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Daemon should no longer respond
	_, err = harness.SendRequest(daemon.Request{Command: "status"})
	assert.Error(t, err, "daemon should not respond after shutdown")
}

// TestDaemonInfo tests the info command
func TestDaemonInfo(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Test info command
	resp, err := harness.SendRequest(daemon.Request{Command: "info"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data, "info should return data")

	// Verify data structure
	infoMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "info data should be a map")
	assert.Contains(t, infoMap, "system", "info should include system info")
	assert.Contains(t, infoMap, "pending", "info should include pending flag")
}

// TestDaemonMultipleClients tests concurrent client connections
func TestDaemonMultipleClients(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Send multiple concurrent requests
	const numClients = 10
	results := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func() {
			resp, err := harness.SendRequest(daemon.Request{Command: "status"})
			if err != nil {
				results <- err
				return
			}
			if !resp.Success {
				results <- assert.AnError
				return
			}
			results <- nil
		}()
	}

	// Collect results
	for i := 0; i < numClients; i++ {
		err := <-results
		assert.NoError(t, err, "concurrent request should succeed")
	}
}
