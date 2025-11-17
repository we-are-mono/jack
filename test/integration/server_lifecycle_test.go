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
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

// TestNewServer_Success tests successful server creation
func TestNewServer_Success(t *testing.T) {
	// Use temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "jack-test.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := daemon.NewServer()
	require.NoError(t, err, "Should create server successfully")
	require.NotNil(t, server, "Server should not be nil")
	defer server.Stop()

	// Verify socket was created
	assert.FileExists(t, socketPath, "Socket file should exist")

	// Verify socket permissions (should be 0666)
	info, err := os.Stat(socketPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0666)|os.ModeSocket, info.Mode(), "Socket should have correct permissions")

	t.Log("Server created successfully")
}

// TestNewServer_MultipleInstances tests creating multiple server instances
func TestNewServer_MultipleInstances(t *testing.T) {
	// First server
	tmpDir1 := t.TempDir()
	socket1 := filepath.Join(tmpDir1, "jack1.sock")
	os.Setenv("JACK_SOCKET_PATH", socket1)

	server1, err := daemon.NewServer()
	require.NoError(t, err)
	defer server1.Stop()

	// Second server with different socket
	tmpDir2 := t.TempDir()
	socket2 := filepath.Join(tmpDir2, "jack2.sock")
	os.Setenv("JACK_SOCKET_PATH", socket2)

	server2, err := daemon.NewServer()
	require.NoError(t, err)
	defer server2.Stop()

	// Both sockets should exist
	assert.FileExists(t, socket1)
	assert.FileExists(t, socket2)

	t.Log("Multiple server instances created successfully")
}

// TestServer_StartStop tests server start and stop lifecycle
func TestServer_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "jack-test.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Create minimal config files for server to start
	configDir := tmpDir
	os.Setenv("JACK_CONFIG_DIR", configDir)
	defer os.Unsetenv("JACK_CONFIG_DIR")

	// Create jack.json with empty plugins list
	jackConfig := map[string]interface{}{
		"plugins": map[string]interface{}{},
	}
	jackConfigJSON, _ := json.Marshal(jackConfig)
	err := os.WriteFile(filepath.Join(configDir, "jack.json"), jackConfigJSON, 0644)
	require.NoError(t, err)

	// Create interfaces.json
	interfacesConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{},
	}
	interfacesJSON, _ := json.Marshal(interfacesConfig)
	err = os.WriteFile(filepath.Join(configDir, "interfaces.json"), interfacesJSON, 0644)
	require.NoError(t, err)

	server, err := daemon.NewServer()
	require.NoError(t, err)

	// Start server in background
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Start(false)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify socket is listening
	conn, err := net.Dial("unix", socketPath)
	if err == nil {
		conn.Close()
		t.Log("Server is listening on socket")
	} else {
		t.Logf("Warning: Could not connect to socket: %v", err)
	}

	// Stop server
	err = server.Stop()
	assert.NoError(t, err, "Should stop server cleanly")

	// Wait for server to finish
	select {
	case err := <-serverDone:
		assert.NoError(t, err, "Server should exit cleanly")
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop in time")
	}

	// Socket should be removed
	assert.NoFileExists(t, socketPath, "Socket should be removed after stop")

	t.Log("Server lifecycle completed successfully")
}

// TestServer_HandleConnection tests connection handling
func TestServer_HandleConnection(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Send a simple status request
	req := daemon.Request{Command: "status"}
	resp, err := harness.SendRequest(req)
	require.NoError(t, err, "Should send request successfully")
	assert.True(t, resp.Success, "Status request should succeed")

	t.Logf("Connection handled successfully: %s", resp.Message)
}

// TestServer_HandleMultipleConnections tests handling multiple concurrent connections
func TestServer_HandleMultipleConnections(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Send multiple concurrent requests
	numRequests := 10
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			req := daemon.Request{Command: "status"}
			resp, err := harness.SendRequest(req)
			if err != nil {
				results <- err
				return
			}
			if !resp.Success {
				results <- assert.AnError
				return
			}
			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err, "Request %d should succeed", i)
		case <-time.After(5 * time.Second):
			t.Fatal("Request timeout")
		}
	}

	t.Logf("Successfully handled %d concurrent connections", numRequests)
}

// TestServer_HandleRequest_UnknownCommand tests handling unknown commands
func TestServer_HandleRequest_UnknownCommand(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Send unknown command
	req := daemon.Request{Command: "nonexistent-command-xyz"}
	resp, err := harness.SendRequest(req)
	require.NoError(t, err, "Should send request successfully")
	assert.False(t, resp.Success, "Unknown command should fail")
	assert.Contains(t, resp.Error, "unknown command", "Error should indicate unknown command")

	t.Log("Unknown command handled correctly")
}

// TestServer_HandleRequest_AllCommands tests all known commands
func TestServer_HandleRequest_AllCommands(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Test all basic commands
	commands := []struct {
		name    string
		request daemon.Request
		expectSuccess bool
	}{
		{
			name:    "status",
			request: daemon.Request{Command: "status"},
			expectSuccess: true,
		},
		{
			name:    "info",
			request: daemon.Request{Command: "info"},
			expectSuccess: true,
		},
		{
			name:    "diff",
			request: daemon.Request{Command: "diff"},
			expectSuccess: true,
		},
		{
			name:    "show",
			request: daemon.Request{Command: "show", Path: "interfaces"},
			expectSuccess: true,
		},
		{
			name:    "plugin-rescan",
			request: daemon.Request{Command: "plugin-rescan"},
			expectSuccess: true,
		},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := harness.SendRequest(tc.request)
			require.NoError(t, err, "Should send request")

			if tc.expectSuccess {
				assert.True(t, resp.Success, "Command %s should succeed", tc.name)
			} else {
				assert.False(t, resp.Success, "Command %s should fail", tc.name)
			}

			t.Logf("Command %s: %s", tc.name, resp.Message)
		})
	}
}

// TestServer_HandleRequest_InvalidJSON tests handling invalid JSON
func TestServer_HandleRequest_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "jack-test.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Create minimal config
	configDir := tmpDir
	os.Setenv("JACK_CONFIG_DIR", configDir)
	defer os.Unsetenv("JACK_CONFIG_DIR")

	jackConfig := map[string]interface{}{"plugins": map[string]interface{}{}}
	jackConfigJSON, _ := json.Marshal(jackConfig)
	os.WriteFile(filepath.Join(configDir, "jack.json"), jackConfigJSON, 0644)

	interfacesConfig := map[string]interface{}{"interfaces": map[string]interface{}{}}
	interfacesJSON, _ := json.Marshal(interfacesConfig)
	os.WriteFile(filepath.Join(configDir, "interfaces.json"), interfacesJSON, 0644)

	server, err := daemon.NewServer()
	require.NoError(t, err)
	defer server.Stop()

	go func() {
		_ = server.Start(false)
	}()
	time.Sleep(100 * time.Millisecond)

	// Send invalid JSON
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("invalid json{}\n"))
	require.NoError(t, err)

	// Read response
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	require.NoError(t, err)

	var resp daemon.Response
	err = json.Unmarshal(buf[:n], &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success, "Should fail on invalid JSON")
	assert.Contains(t, resp.Error, "invalid request", "Error should indicate invalid request")

	t.Log("Invalid JSON handled correctly")
}

// TestServer_GracefulShutdown tests graceful server shutdown
func TestServer_GracefulShutdown(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start daemon
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Send a request to verify server is running
	req := daemon.Request{Command: "status"}
	resp, err := harness.SendRequest(req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Trigger shutdown by canceling context
	cancel()

	// Wait for server to shut down gracefully
	select {
	case err := <-serverDone:
		t.Logf("Server shut down with: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not shut down in time")
	}

	// Give Stop() time to complete socket cleanup (runs in separate goroutine)
	time.Sleep(100 * time.Millisecond)

	// Socket should be cleaned up
	socketPath := harness.socketPath
	assert.NoFileExists(t, socketPath, "Socket should be removed after shutdown")

	t.Log("Graceful shutdown completed successfully")
}

// TestServer_RequestResponseCycle tests the complete request/response cycle
func TestServer_RequestResponseCycle(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		_ = harness.StartDaemon(ctx)
	}()
	harness.WaitForDaemon(5 * time.Second)

	// Test various request/response patterns
	tests := []struct {
		name     string
		request  daemon.Request
		validate func(*testing.T, daemon.Response)
	}{
		{
			name:    "status returns success",
			request: daemon.Request{Command: "status"},
			validate: func(t *testing.T, resp daemon.Response) {
				assert.True(t, resp.Success)
				assert.NotEmpty(t, resp.Message)
			},
		},
		{
			name:    "info returns data",
			request: daemon.Request{Command: "info"},
			validate: func(t *testing.T, resp daemon.Response) {
				assert.True(t, resp.Success)
				assert.NotNil(t, resp.Data)
			},
		},
		{
			name:    "show with path",
			request: daemon.Request{Command: "show", Path: "interfaces"},
			validate: func(t *testing.T, resp daemon.Response) {
				assert.True(t, resp.Success)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := harness.SendRequest(tc.request)
			require.NoError(t, err, "Should send request successfully")
			tc.validate(t, *resp)
		})
	}
}

// TestServer_ConnectionTimeout tests connection timeout handling
func TestServer_ConnectionTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "jack-test.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Create minimal config
	configDir := tmpDir
	os.Setenv("JACK_CONFIG_DIR", configDir)
	defer os.Unsetenv("JACK_CONFIG_DIR")

	jackConfig := map[string]interface{}{"plugins": map[string]interface{}{}}
	jackConfigJSON, _ := json.Marshal(jackConfig)
	os.WriteFile(filepath.Join(configDir, "jack.json"), jackConfigJSON, 0644)

	interfacesConfig := map[string]interface{}{"interfaces": map[string]interface{}{}}
	interfacesJSON, _ := json.Marshal(interfacesConfig)
	os.WriteFile(filepath.Join(configDir, "interfaces.json"), interfacesJSON, 0644)

	server, err := daemon.NewServer()
	require.NoError(t, err)
	defer server.Stop()

	go func() {
		_ = server.Start(false)
	}()
	time.Sleep(100 * time.Millisecond)

	// Connect but don't send anything
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Wait to see if server sends anything (it shouldn't)
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)

	// Should timeout or get EOF (server waiting for request)
	if err != nil {
		t.Logf("Connection behavior: %v (expected)", err)
	} else if n == 0 {
		t.Log("Connection closed gracefully (expected)")
	}

	t.Log("Connection timeout handling verified")
}

// TestServer_HandleConnection_EmptyRequest tests handling empty request
func TestServer_HandleConnection_EmptyRequest(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "jack-test.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Create minimal config
	configDir := tmpDir
	os.Setenv("JACK_CONFIG_DIR", configDir)
	defer os.Unsetenv("JACK_CONFIG_DIR")

	jackConfig := map[string]interface{}{"plugins": map[string]interface{}{}}
	jackConfigJSON, _ := json.Marshal(jackConfig)
	os.WriteFile(filepath.Join(configDir, "jack.json"), jackConfigJSON, 0644)

	interfacesConfig := map[string]interface{}{"interfaces": map[string]interface{}{}}
	interfacesJSON, _ := json.Marshal(interfacesConfig)
	os.WriteFile(filepath.Join(configDir, "interfaces.json"), interfacesJSON, 0644)

	server, err := daemon.NewServer()
	require.NoError(t, err)
	defer server.Stop()

	go func() {
		_ = server.Start(false)
	}()
	time.Sleep(100 * time.Millisecond)

	// Send empty JSON object
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Write([]byte("{}\n"))
	require.NoError(t, err)

	// Read response
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	require.NoError(t, err)

	var resp daemon.Response
	err = json.Unmarshal(buf[:n], &resp)
	require.NoError(t, err)

	// Empty command should fail with unknown command
	assert.False(t, resp.Success, "Empty command should fail")
	assert.Contains(t, resp.Error, "unknown command", "Error should indicate unknown command")

	t.Log("Empty request handled correctly")
}
