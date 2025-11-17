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
	"bytes"
	"context"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon"
)

// TestObserver_DetectsLinkChanges tests that the observer detects external interface changes
func TestObserver_DetectsLinkChanges(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs0")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture daemon logs
	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	// Wait for daemon to be ready
	harness.WaitForDaemon(5 * time.Second)

	// Give observer time to start
	time.Sleep(500 * time.Millisecond)

	// Make external change: bring interface down
	cmd := exec.Command("ip", "link", "set", ifaceName, "down")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface down: %s", output)

	// Wait for observer to detect the change
	time.Sleep(500 * time.Millisecond)

	// Check that observer logged the change
	logs := logBuf.String()
	assert.Contains(t, logs, "[OBSERVER]", "Observer should be logging")
	assert.Contains(t, logs, "Network observer started", "Observer should have started")
	assert.Contains(t, logs, ifaceName, "Observer should have detected interface change")

	// Make another change: bring interface back up
	cmd = exec.Command("ip", "link", "set", ifaceName, "up")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface up: %s", output)

	// Wait for observer to detect the change
	time.Sleep(500 * time.Millisecond)

	// Should have logged both changes
	logs = logBuf.String()
	linkChanges := bytes.Count([]byte(logs), []byte("Link change detected"))
	assert.GreaterOrEqual(t, linkChanges, 2, "Observer should have detected at least 2 link changes")
}

// TestObserver_DetectsAddressChanges tests that the observer detects IP address modifications
func TestObserver_DetectsAddressChanges(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs1")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)
	time.Sleep(500 * time.Millisecond)

	// Make external change: add IP address
	cmd := exec.Command("ip", "addr", "add", "10.99.1.1/24", "dev", ifaceName)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to add IP address: %s", output)

	// Wait for observer to detect
	time.Sleep(500 * time.Millisecond)

	// Check logs
	logs := logBuf.String()
	assert.Contains(t, logs, "[OBSERVER]", "Observer should be logging")
	assert.Contains(t, logs, "Address", "Observer should have detected address change")

	// Remove the IP address
	cmd = exec.Command("ip", "addr", "del", "10.99.1.1/24", "dev", ifaceName)
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to delete IP address: %s", output)

	// Wait for observer to detect
	time.Sleep(500 * time.Millisecond)

	// Should have logged address changes
	logs = logBuf.String()
	addrChanges := bytes.Count([]byte(logs), []byte("Address"))
	assert.GreaterOrEqual(t, addrChanges, 2, "Observer should have detected address changes")
}

// TestObserver_IgnoresOwnChanges tests that the observer doesn't report Jack's own changes
func TestObserver_IgnoresOwnChanges(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs2")

	// Create config for the interface
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "test-obs2-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.2.1",
      "netmask": "255.255.255.0",
      "enabled": true
    }
  }
}`
	err := harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	// Create minimal jack.json
	jackCfg := `{
  "version": "1.0",
  "plugins": {}
}`
	err = harness.WriteConfig("jack.json", []byte(jackCfg))
	require.NoError(t, err, "Failed to write jack config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)
	time.Sleep(500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Apply configuration through Jack (this should be ignored by observer)
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait a bit to ensure observer would have logged if it detected changes
	time.Sleep(1 * time.Second)

	// Check that observer didn't log Jack's own changes
	logs := logBuf.String()
	observerDetections := bytes.Count([]byte(logs), []byte("Link change detected"))

	// Observer should NOT have logged Jack's changes (debouncing)
	// It may still see some changes before debounce window, so allow up to 1
	assert.LessOrEqual(t, observerDetections, 1,
		"Observer should not log Jack's own changes (debouncing should prevent it)")
}

// TestObserver_DetectsInterfaceCreation tests that observer detects new interfaces
func TestObserver_DetectsInterfaceCreation(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Start daemon with observer BEFORE creating interface
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)
	time.Sleep(500 * time.Millisecond)

	// Now create a new interface while daemon is running
	ifaceName := "test-obs3"
	cmd := exec.Command("ip", "link", "add", ifaceName, "type", "dummy")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to create interface: %s", output)

	// Clean up the interface
	defer func() {
		exec.Command("ip", "link", "del", ifaceName).Run()
	}()

	// Wait for observer to detect
	time.Sleep(500 * time.Millisecond)

	// Check logs for interface creation
	logs := logBuf.String()
	assert.Contains(t, logs, "[OBSERVER]", "Observer should be logging")
	assert.Contains(t, logs, ifaceName, "Observer should have detected new interface")
	assert.Contains(t, logs, "Link change detected", "Observer should have logged link change")
}

// TestObserver_DetectsLinkDrift tests drift detection for interface state
func TestObserver_DetectsLinkDrift(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs-drift")

	// Configure Jack to manage this interface in "up" state
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "obs-drift-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.3.1",
      "netmask": "255.255.255.0",
      "mtu": 1400,
      "enabled": true
    }
  }
}`
	err := harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait for apply to complete and debounce window to expire
	time.Sleep(1500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Externally bring the interface down (violates Jack's config)
	cmd := exec.Command("ip", "link", "set", ifaceName, "down")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface down: %s", output)

	// Wait for observer to detect drift
	time.Sleep(500 * time.Millisecond)

	// Check that drift was detected
	logs := logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Observer should detect drift")
	assert.Contains(t, logs, "is down but should be up", "Observer should identify down interface as drift")

	// Clear logs again
	logBuf.Reset()

	// Bring interface back up first
	cmd = exec.Command("ip", "link", "set", ifaceName, "up")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface up: %s", output)

	// Wait for that change to be processed
	time.Sleep(500 * time.Millisecond)
	logBuf.Reset()

	// Externally change MTU (violates Jack's config which expects 1400)
	cmd = exec.Command("ip", "link", "set", ifaceName, "mtu", "1500")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to change MTU: %s", output)

	time.Sleep(500 * time.Millisecond)

	// Check that MTU drift was detected
	logs = logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Observer should detect MTU drift")
	assert.Contains(t, logs, "MTU", "Observer should identify MTU change as drift")
}

// TestObserver_DetectsAddressDrift tests drift detection for IP addresses
func TestObserver_DetectsAddressDrift(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs-addr")

	// Configure Jack to manage this interface with specific IP
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "obs-addr-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.4.1",
      "netmask": "255.255.255.0",
      "enabled": true
    }
  }
}`
	err := harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait for apply to complete and debounce window to expire
	time.Sleep(1500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Externally add a different IP address (violates Jack's config)
	cmd := exec.Command("ip", "addr", "add", "10.99.4.99/24", "dev", ifaceName)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to add IP address: %s", output)

	// Wait for observer to detect drift
	time.Sleep(500 * time.Millisecond)

	// Check that drift was detected
	logs := logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Observer should detect address drift")
	assert.Contains(t, logs, "unexpected IP", "Observer should identify unexpected IP as drift")
}

// TestObserver_DetectsRouteDrift tests drift detection for routes
func TestObserver_DetectsRouteDrift(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs-route")

	// Bring interface up with IP
	cmd := exec.Command("ip", "addr", "add", "10.99.5.1/24", "dev", ifaceName)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to add IP: %s", output)

	// Configure Jack to manage a specific route
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "obs-route-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.5.1",
      "netmask": "255.255.255.0",
      "enabled": true
    }
  }
}`
	err = harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	routesCfg := `{
  "version": "1.0",
  "routes": {
    "test-route": {
      "name": "test-route",
      "destination": "192.168.100.0/24",
      "gateway": "10.99.5.254",
      "interface": "` + ifaceName + `",
      "enabled": true
    }
  }
}`
	err = harness.WriteConfig("routes.json", []byte(routesCfg))
	require.NoError(t, err, "Failed to write routes config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration (creates the route)
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait for apply to complete and debounce window to expire
	time.Sleep(1500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Externally delete the route (violates Jack's config)
	cmd = exec.Command("ip", "route", "del", "192.168.100.0/24", "via", "10.99.5.254")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to delete route: %s", output)

	// Wait for observer to detect drift
	time.Sleep(500 * time.Millisecond)

	// Check that drift was detected
	logs := logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Observer should detect route drift")
	assert.Contains(t, logs, "deleted externally", "Observer should identify deleted route as drift")
}

// TestObserver_AutoReconcile tests automatic reconciliation when drift is detected
func TestObserver_AutoReconcile(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs-recon")

	// Configure Jack to manage this interface
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "obs-recon-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.6.1",
      "netmask": "255.255.255.0",
      "mtu": 1400,
      "enabled": true
    }
  }
}`
	err := harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	// Enable auto-reconciliation with short interval for testing
	jackCfg := `{
  "version": "1.0",
  "plugins": {},
  "observer": {
    "enabled": true,
    "auto_reconcile": true,
    "reconcile_interval_ms": 5000
  }
}`
	err = harness.WriteConfig("jack.json", []byte(jackCfg))
	require.NoError(t, err, "Failed to write jack config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait for apply to complete and debounce window to expire
	time.Sleep(1500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Externally bring the interface down (violates Jack's config)
	cmd := exec.Command("ip", "link", "set", ifaceName, "down")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface down: %s", output)

	// Wait for observer to detect drift and reconcile (should auto-fix)
	time.Sleep(3 * time.Second)

	// Check logs for drift detection AND automatic reconciliation
	logs := logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Observer should detect drift")
	assert.Contains(t, logs, "Triggering automatic reconciliation", "Observer should trigger reconciliation")
	assert.Contains(t, logs, "Automatic reconciliation completed", "Reconciliation should complete")

	// Verify that the interface is back up (reconciliation fixed it)
	link, err := netlink.LinkByName(ifaceName)
	require.NoError(t, err, "Should be able to query interface")
	assert.True(t, link.Attrs().Flags&net.FlagUp != 0, "Interface should be back up after auto-reconciliation")
}

// TestObserver_ReconcileRateLimit tests that reconciliation respects rate limiting
func TestObserver_ReconcileRateLimit(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a test interface
	ifaceName := harness.CreateDummyInterface("obs-limit")

	// Configure Jack to manage this interface
	interfacesCfg := `{
  "version": "1.0",
  "interfaces": {
    "obs-limit-cfg": {
      "type": "physical",
      "device": "` + ifaceName + `",
      "device_name": "` + ifaceName + `",
      "protocol": "static",
      "ipaddr": "10.99.7.1",
      "netmask": "255.255.255.0",
      "enabled": true
    }
  }
}`
	err := harness.WriteConfig("interfaces.json", []byte(interfacesCfg))
	require.NoError(t, err, "Failed to write interfaces config")

	// Enable auto-reconciliation with long interval (60 seconds)
	jackCfg := `{
  "version": "1.0",
  "plugins": {},
  "observer": {
    "enabled": true,
    "auto_reconcile": true,
    "reconcile_interval_ms": 60000
  }
}`
	err = harness.WriteConfig("jack.json", []byte(jackCfg))
	require.NoError(t, err, "Failed to write jack config")

	// Start daemon with observer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	go func() {
		err := harness.StartDaemonWithOutput(ctx, &logBuf)
		if err != nil {
			t.Logf("Daemon error: %v", err)
		}
	}()

	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration
	_, err = harness.SendRequest(daemon.Request{Command: "apply"})
	require.NoError(t, err, "Apply should succeed")

	// Wait for apply to complete and debounce window to expire
	time.Sleep(1500 * time.Millisecond)

	// Clear logs up to this point
	logBuf.Reset()

	// Externally bring the interface down (first drift)
	cmd := exec.Command("ip", "link", "set", ifaceName, "down")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface down: %s", output)

	// Wait for drift detection
	time.Sleep(1 * time.Second)

	// Check that first reconciliation was triggered
	logs := logBuf.String()
	assert.Contains(t, logs, "Triggering automatic reconciliation", "First reconciliation should trigger")

	// Clear logs
	logBuf.Reset()

	// Immediately cause another drift (bring down again after it's fixed)
	time.Sleep(3 * time.Second) // Wait for first reconciliation to complete
	cmd = exec.Command("ip", "link", "set", ifaceName, "down")
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "Failed to bring interface down again: %s", output)

	// Wait for drift detection
	time.Sleep(1 * time.Second)

	// Second reconciliation should be rate limited
	logs = logBuf.String()
	assert.Contains(t, logs, "Configuration drift detected", "Second drift should be detected")
	assert.Contains(t, logs, "Reconciliation rate limited", "Second reconciliation should be rate limited")
}
