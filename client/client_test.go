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

package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

// TestGetSocketPath tests socket path resolution
func TestGetSocketPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "default path when env not set",
			envValue: "",
			expected: "/var/run/jack.sock",
		},
		{
			name:     "custom path from env",
			envValue: "/tmp/custom-jack.sock",
			expected: "/tmp/custom-jack.sock",
		},
		{
			name:     "relative path from env",
			envValue: "./jack.sock",
			expected: "./jack.sock",
		},
		{
			name:     "absolute path with spaces",
			envValue: "/tmp/path with spaces/jack.sock",
			expected: "/tmp/path with spaces/jack.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env
			originalEnv := os.Getenv("JACK_SOCKET_PATH")
			defer os.Setenv("JACK_SOCKET_PATH", originalEnv)

			// Set test env
			if tt.envValue == "" {
				os.Unsetenv("JACK_SOCKET_PATH")
			} else {
				os.Setenv("JACK_SOCKET_PATH", tt.envValue)
			}

			result := GetSocketPath()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSend_Success tests successful request/response
func TestSend_Success(t *testing.T) {
	// Create temporary socket
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Start mock server
	stopServer := startMockServer(t, sockPath, func(req daemon.Request) daemon.Response {
		return daemon.Response{
			Success: true,
			Message: "OK",
			Data:    map[string]interface{}{"test": "value"},
		}
	})
	defer stopServer()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Send request
	req := daemon.Request{
		Command: "status",
	}

	resp, err := Send(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Equal(t, "OK", resp.Message)

	// Check data
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "Data should be a map")
	assert.Equal(t, "value", dataMap["test"])
}

// TestSend_ConnectionFailure tests connection failures
func TestSend_ConnectionFailure(t *testing.T) {
	// Point to non-existent socket
	sockPath := "/tmp/nonexistent-jack-socket-" + fmt.Sprint(time.Now().UnixNano()) + ".sock"
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	req := daemon.Request{
		Command: "status",
	}

	resp, err := Send(req)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to connect to daemon")
}

// TestSend_ReadFailure tests read failures
func TestSend_ReadFailure(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Start server that closes connection without sending response
	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				// Read request but don't send response - just close
				conn.Close()
			}
		}
	}()

	defer func() {
		close(stopChan)
		listener.Close()
		os.Remove(sockPath)
	}()

	time.Sleep(50 * time.Millisecond)

	req := daemon.Request{
		Command: "status",
	}

	resp, err := Send(req)
	require.Error(t, err)
	assert.Nil(t, resp)
	// Error can be either read or write failure due to race conditions
	errStr := err.Error()
	assert.True(t,
		strings.Contains(errStr, "failed to read response") ||
		strings.Contains(errStr, "failed to send request"),
		"error should be about read or write failure, got: %s", errStr)
}

// TestSend_InvalidJSONResponse tests handling of invalid JSON responses
func TestSend_InvalidJSONResponse(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Start server that sends invalid JSON
	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				// Read request
				reader := bufio.NewReader(conn)
				_, _ = reader.ReadBytes('\n')

				// Send invalid JSON
				conn.Write([]byte("invalid json\n"))
				conn.Close()
			}
		}
	}()

	defer func() {
		close(stopChan)
		listener.Close()
		os.Remove(sockPath)
	}()

	time.Sleep(50 * time.Millisecond)

	req := daemon.Request{
		Command: "status",
	}

	resp, err := Send(req)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to parse response")
}

// TestSend_DifferentActions tests different request actions
func TestSend_DifferentActions(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	// Start mock server that echoes command in response
	stopServer := startMockServer(t, sockPath, func(req daemon.Request) daemon.Response {
		return daemon.Response{
			Success: true,
			Message: "Command: " + req.Command,
		}
	})
	defer stopServer()

	time.Sleep(50 * time.Millisecond)

	actions := []string{"status", "apply", "commit", "diff", "revert"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			req := daemon.Request{
				Command: action,
			}

			resp, err := Send(req)
			require.NoError(t, err)
			assert.True(t, resp.Success)
			assert.Contains(t, resp.Message, action)
		})
	}
}

// TestSend_WithPayload tests sending requests with payloads
func TestSend_WithPayload(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	receivedValue := make(map[string]interface{})
	var mu sync.Mutex

	stopServer := startMockServer(t, sockPath, func(req daemon.Request) daemon.Response {
		mu.Lock()
		if val, ok := req.Value.(map[string]interface{}); ok {
			receivedValue = val
		}
		mu.Unlock()

		return daemon.Response{
			Success: true,
			Message: "Received value",
		}
	})
	defer stopServer()

	time.Sleep(50 * time.Millisecond)

	req := daemon.Request{
		Command: "set",
		Value: map[string]interface{}{
			"key": "value",
			"nested": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	resp, err := Send(req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	mu.Lock()
	assert.Equal(t, "value", receivedValue["key"])
	assert.Equal(t, map[string]interface{}{"foo": "bar"}, receivedValue["nested"])
	mu.Unlock()
}

// TestSend_ConcurrentRequests tests concurrent client requests
func TestSend_ConcurrentRequests(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	var requestCount int
	var mu sync.Mutex

	stopServer := startMockServer(t, sockPath, func(req daemon.Request) daemon.Response {
		mu.Lock()
		requestCount++
		mu.Unlock()

		// Small delay to simulate processing
		time.Sleep(10 * time.Millisecond)

		return daemon.Response{
			Success: true,
			Message: "OK",
		}
	})
	defer stopServer()

	time.Sleep(50 * time.Millisecond)

	// Send 10 concurrent requests
	const numRequests = 10
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := daemon.Request{
				Command: fmt.Sprintf("action-%d", id),
			}

			_, err := Send(req)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}

	mu.Lock()
	assert.Equal(t, numRequests, requestCount)
	mu.Unlock()
}

// TestSend_LargePayload tests handling of large payloads
func TestSend_LargePayload(t *testing.T) {
	sockPath := createTempSocket(t)
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	stopServer := startMockServer(t, sockPath, func(req daemon.Request) daemon.Response {
		return daemon.Response{
			Success: true,
			Data:    req.Value,
		}
	})
	defer stopServer()

	time.Sleep(50 * time.Millisecond)

	// Create large payload
	largePayload := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largePayload[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	req := daemon.Request{
		Command: "set",
		Value:   largePayload,
	}

	resp, err := Send(req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Check that response data is a map with 1000 elements
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 1000, len(dataMap))
}

// TestGetSocketPath_ConcurrentAccess tests concurrent socket path resolution
func TestGetSocketPath_ConcurrentAccess(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("JACK_SOCKET_PATH")
	defer os.Setenv("JACK_SOCKET_PATH", originalEnv)

	os.Setenv("JACK_SOCKET_PATH", "/tmp/test.sock")

	// Run concurrent goroutines
	const numGoroutines = 100
	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- GetSocketPath()
		}()
	}

	wg.Wait()
	close(results)

	// All results should be the same
	for path := range results {
		assert.Equal(t, "/tmp/test.sock", path)
	}
}

// Helper functions

func createTempSocket(t *testing.T) string {
	t.Helper()
	sockPath := fmt.Sprintf("/tmp/jack-test-%d.sock", time.Now().UnixNano())
	t.Cleanup(func() {
		os.Remove(sockPath)
	})
	return sockPath
}

func startMockServer(t *testing.T, sockPath string, handler func(daemon.Request) daemon.Response) func() {
	t.Helper()

	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)

	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopChan:
				return
			default:
				// Set accept deadline to allow checking stopChan
				listener.(*net.UnixListener).SetDeadline(time.Now().Add(100 * time.Millisecond))

				conn, err := listener.Accept()
				if err != nil {
					// Check if it's a timeout (expected)
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					// Other error, check if we're stopping
					select {
					case <-stopChan:
						return
					default:
						continue
					}
				}

				// Handle connection in separate goroutine
				wg.Add(1)
				go func(c net.Conn) {
					defer wg.Done()
					defer c.Close()

					reader := bufio.NewReader(c)
					reqData, err := reader.ReadBytes('\n')
					if err != nil {
						return
					}

					var req daemon.Request
					if err := json.Unmarshal(reqData, &req); err != nil {
						return
					}

					resp := handler(req)
					respData, _ := json.Marshal(resp)
					respData = append(respData, '\n')
					c.Write(respData)
				}(conn)
			}
		}
	}()

	return func() {
		close(stopChan)
		listener.Close()
		wg.Wait()
	}
}

// Benchmark tests

func BenchmarkGetSocketPath(b *testing.B) {
	os.Setenv("JACK_SOCKET_PATH", "/tmp/test.sock")
	defer os.Unsetenv("JACK_SOCKET_PATH")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetSocketPath()
	}
}

func BenchmarkSend(b *testing.B) {
	sockPath := fmt.Sprintf("/tmp/jack-bench-%d.sock", time.Now().UnixNano())
	os.Setenv("JACK_SOCKET_PATH", sockPath)
	defer func() {
		os.Unsetenv("JACK_SOCKET_PATH")
		os.Remove(sockPath)
	}()

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()

	// Start simple echo server
	stopChan := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopChan:
				return
			default:
				listener.(*net.UnixListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
				conn, err := listener.Accept()
				if err != nil {
					continue
				}

				go func(c net.Conn) {
					defer c.Close()
					reader := bufio.NewReader(c)
					reqData, _ := reader.ReadBytes('\n')

					var req daemon.Request
					json.Unmarshal(reqData, &req)

					resp := daemon.Response{Success: true}
					respData, _ := json.Marshal(resp)
					respData = append(respData, '\n')
					c.Write(respData)
				}(conn)
			}
		}
	}()
	defer close(stopChan)

	time.Sleep(50 * time.Millisecond)

	req := daemon.Request{Command: "status"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Send(req)
	}
}
