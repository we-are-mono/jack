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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/daemon/logger"
)

// TestHarness provides isolated test environment for integration tests
type TestHarness struct {
	t             *testing.T
	testName      string
	configDir     string
	socketPath    string
	createdIfaces []string
	daemonCancel  context.CancelFunc
	originalEnv   map[string]string
}

// NewTestHarness creates a new isolated test environment
// When running in Docker, uses the container's network namespace directly
// When running locally with root, still provides isolation via unique socket/config paths
func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()

	// Check for root privileges (needed for network interface manipulation)
	if os.Geteuid() != 0 {
		t.Skip("Integration tests require root privileges")
	}

	// Create unique test identifier
	testName := fmt.Sprintf("jack-test-%d", os.Getpid())

	// Create test config directory
	configDir := t.TempDir()

	// Create unique socket path
	socketPath := filepath.Join(configDir, "jack.sock")

	h := &TestHarness{
		t:             t,
		testName:      testName,
		configDir:     configDir,
		socketPath:    socketPath,
		createdIfaces: []string{},
		originalEnv:   make(map[string]string),
	}

	// Save original environment variables
	h.originalEnv["JACK_CONFIG_DIR"] = os.Getenv("JACK_CONFIG_DIR")
	h.originalEnv["JACK_SOCKET_PATH"] = os.Getenv("JACK_SOCKET_PATH")

	// Set environment variables for test
	os.Setenv("JACK_CONFIG_DIR", configDir)
	os.Setenv("JACK_SOCKET_PATH", socketPath)

	t.Logf("Created test harness: test=%s, socket=%s", testName, socketPath)

	return h
}

// CreateDummyInterface creates a dummy interface directly in the current namespace
// In Docker, this creates it in the container's namespace
// In local testing, this creates it in the host namespace (tests should clean up)
// The interface name is prefixed with "test-" to avoid conflicts with real interfaces
func (h *TestHarness) CreateDummyInterface(name string) string {
	h.t.Helper()

	// Prefix with "test-" to avoid conflicts with real interfaces (e.g., eth0 in Docker)
	actualName := "test-" + name

	// Create dummy interface directly (no namespace wrapper)
	cmd := exec.Command("ip", "link", "add", actualName, "type", "dummy")
	if output, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("Failed to create dummy interface %s: %v\nOutput: %s", actualName, err, output)
	}

	// Bring interface up
	cmd = exec.Command("ip", "link", "set", actualName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		h.t.Fatalf("Failed to bring up interface %s: %v\nOutput: %s", actualName, err, output)
	}

	h.createdIfaces = append(h.createdIfaces, actualName)
	h.t.Logf("Created dummy interface: %s (actual name: %s)", name, actualName)
	return actualName
}

// DeleteInterface removes an interface
func (h *TestHarness) DeleteInterface(name string) {
	cmd := exec.Command("ip", "link", "del", name)
	_ = cmd.Run() // Ignore errors on cleanup
}

// StartDaemon starts the Jack daemon
// The daemon runs in the same namespace as the test, so it can see all created interfaces
func (h *TestHarness) StartDaemon(ctx context.Context) error {
	return h.startDaemonInternal(ctx, nil)
}

// StartDaemonWithOutput starts the Jack daemon and captures log output to the provided writer
func (h *TestHarness) StartDaemonWithOutput(ctx context.Context, logWriter *bytes.Buffer) error {
	return h.startDaemonInternal(ctx, logWriter)
}

// startDaemonInternal is the internal implementation of daemon startup
func (h *TestHarness) startDaemonInternal(ctx context.Context, logWriter *bytes.Buffer) error {
	// Create empty interfaces.json to prevent auto-detection (only if it doesn't exist)
	// Tests will use "set" command to configure interfaces explicitly
	// Preserve existing interfaces.json if it exists (for persistence tests)
	interfacesPath := filepath.Join(h.configDir, "interfaces.json")
	if _, err := os.Stat(interfacesPath); os.IsNotExist(err) {
		emptyInterfaces := `{"interfaces": {}}`
		if err := os.WriteFile(interfacesPath, []byte(emptyInterfaces), 0644); err != nil {
			return fmt.Errorf("failed to create empty interfaces config: %w", err)
		}
	}

	// Initialize logger infrastructure (required for structured logging to work)
	loggerConfig := logger.Config{
		Level:     "debug",
		Format:    "json",
		Component: "daemon",
	}
	emitter := logger.NewEmitter()

	// Set up log backends
	var backends []logger.Backend
	if logWriter != nil {
		// For tests: capture logs to buffer
		bufferBackend := logger.NewBufferBackend(logWriter, "json")
		backends = []logger.Backend{bufferBackend}
	}

	logger.Init(loggerConfig, backends, emitter)

	// Create daemon server
	srv, err := daemon.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create daemon server: %w", err)
	}

	// Store cancel function from parent context
	_, cancel := context.WithCancel(ctx)
	h.daemonCancel = cancel

	// Run daemon in goroutine
	errCh := make(chan error, 1)
	go func() {
		// Start daemon with applyOnStartup=false for testing
		if err := srv.Start(false); err != nil {
			errCh <- err
		}
	}()

	// Monitor context cancellation
	go func() {
		<-ctx.Done()
		srv.Stop()
	}()

	// Wait briefly for startup
	time.Sleep(100 * time.Millisecond)

	// Check if daemon started successfully
	select {
	case err := <-errCh:
		return fmt.Errorf("daemon failed to start: %w", err)
	default:
		h.t.Logf("Daemon started successfully")
		return nil
	}
}

// WriteConfig writes a configuration file to the test config directory
func (h *TestHarness) WriteConfig(filename string, content []byte) error {
	configPath := filepath.Join(h.configDir, filename)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}
	h.t.Logf("Wrote config file: %s", filename)
	return nil
}

// WaitForDaemon waits for daemon to be ready to accept connections
func (h *TestHarness) WaitForDaemon(timeout time.Duration) {
	h.t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Try to connect via client
		req := daemon.Request{Command: "status"}
		_, err := client.Send(req)
		if err == nil {
			h.t.Logf("Daemon is ready")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	h.t.Fatal("Daemon did not become ready within timeout")
}

// SendRequest sends a request to the daemon and returns the response
func (h *TestHarness) SendRequest(req daemon.Request) (*daemon.Response, error) {
	return client.Send(req)
}

// Cleanup tears down the test environment
func (h *TestHarness) Cleanup() {
	h.t.Helper()

	// Cancel daemon context
	if h.daemonCancel != nil {
		h.daemonCancel()
	}

	// Give daemon time to shut down
	time.Sleep(100 * time.Millisecond)

	// Flush all routes added by tests (delete all non-default routes)
	h.cleanupRoutes()

	// Cleanup interfaces
	for _, iface := range h.createdIfaces {
		h.DeleteInterface(iface)
	}

	// Restore original environment
	for key, val := range h.originalEnv {
		if val == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, val)
		}
	}

	// Cleanup socket
	os.Remove(h.socketPath)

	h.t.Logf("Test harness cleanup complete")
}

// cleanupRoutes removes all routes added during testing
func (h *TestHarness) cleanupRoutes() {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return // Ignore cleanup errors
	}

	for _, route := range routes {
		// Skip default route and loopback routes
		if route.Dst == nil || route.Dst.String() == "127.0.0.0/8" {
			continue
		}
		// Delete test routes (non-directly-connected routes)
		_ = netlink.RouteDel(&route)
	}
}
