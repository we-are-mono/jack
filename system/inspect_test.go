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

package system

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

			result := getSocketPath()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatDuration tests duration formatting
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0m",
		},
		{
			name:     "only minutes",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 30*time.Minute,
			expected: "2h 30m",
		},
		{
			name:     "days hours and minutes",
			duration: 3*24*time.Hour + 5*time.Hour + 15*time.Minute,
			expected: "3d 5h 15m",
		},
		{
			name:     "exact hours",
			duration: 5 * time.Hour,
			expected: "5h 0m",
		},
		{
			name:     "exact days",
			duration: 2 * 24 * time.Hour,
			expected: "2d 0h 0m",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "59 minutes",
			duration: 59 * time.Minute,
			expected: "59m",
		},
		{
			name:     "1 hour 0 minutes",
			duration: 1 * time.Hour,
			expected: "1h 0m",
		},
		{
			name:     "1 hour 1 minute",
			duration: 1*time.Hour + 1*time.Minute,
			expected: "1h 1m",
		},
		{
			name:     "23 hours 59 minutes",
			duration: 23*time.Hour + 59*time.Minute,
			expected: "23h 59m",
		},
		{
			name:     "24 hours (1 day)",
			duration: 24 * time.Hour,
			expected: "1d 0h 0m",
		},
		{
			name:     "large duration",
			duration: 365*24*time.Hour + 12*time.Hour + 45*time.Minute,
			expected: "365d 12h 45m",
		},
		{
			name:     "seconds are truncated",
			duration: 1*time.Minute + 30*time.Second,
			expected: "1m",
		},
		{
			name:     "59 seconds becomes 0 minutes",
			duration: 59 * time.Second,
			expected: "0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDaemonStatusFields tests DaemonStatus struct fields
func TestDaemonStatusFields(t *testing.T) {
	status := DaemonStatus{
		Uptime:     "1h 30m",
		ConfigPath: "/etc/jack",
		PID:        1234,
		Running:    true,
	}

	assert.Equal(t, "1h 30m", status.Uptime)
	assert.Equal(t, "/etc/jack", status.ConfigPath)
	assert.Equal(t, 1234, status.PID)
	assert.True(t, status.Running)
}

// TestInterfaceStatusFields tests InterfaceStatus struct fields
func TestInterfaceStatusFields(t *testing.T) {
	status := InterfaceStatus{
		Name:      "eth0",
		Type:      "physical",
		Device:    "eth0",
		State:     "up",
		IPAddr:    []string{"192.168.1.1/24"},
		MTU:       1500,
		TXPackets: 1000,
		RXPackets: 2000,
		TXBytes:   500000,
		RXBytes:   1000000,
		TXErrors:  0,
		RXErrors:  0,
	}

	assert.Equal(t, "eth0", status.Name)
	assert.Equal(t, "physical", status.Type)
	assert.Equal(t, "eth0", status.Device)
	assert.Equal(t, "up", status.State)
	assert.Equal(t, []string{"192.168.1.1/24"}, status.IPAddr)
	assert.Equal(t, 1500, status.MTU)
	assert.Equal(t, uint64(1000), status.TXPackets)
	assert.Equal(t, uint64(2000), status.RXPackets)
	assert.Equal(t, uint64(500000), status.TXBytes)
	assert.Equal(t, uint64(1000000), status.RXBytes)
	assert.Equal(t, uint64(0), status.TXErrors)
	assert.Equal(t, uint64(0), status.RXErrors)
}

// TestSystemInfoFields tests SystemInfo struct fields
func TestSystemInfoFields(t *testing.T) {
	info := SystemInfo{
		Hostname:      "router",
		KernelVersion: "5.10.0",
		Uptime:        "2d 5h 30m",
	}

	assert.Equal(t, "router", info.Hostname)
	assert.Equal(t, "5.10.0", info.KernelVersion)
	assert.Equal(t, "2d 5h 30m", info.Uptime)
}

// TestSystemStatusFields tests SystemStatus struct fields
func TestSystemStatusFields(t *testing.T) {
	status := SystemStatus{
		System: SystemInfo{
			Hostname:      "router",
			KernelVersion: "5.10.0",
			Uptime:        "1d 2h 30m",
		},
		Interfaces: []InterfaceStatus{
			{
				Name:   "eth0",
				Type:   "physical",
				Device: "eth0",
				State:  "up",
				IPAddr: []string{"192.168.1.1/24"},
				MTU:    1500,
			},
		},
		Daemon: DaemonStatus{
			Uptime:     "5h 15m",
			ConfigPath: "/etc/jack",
			PID:        1234,
			Running:    true,
		},
		IPForwarding: true,
	}

	assert.Equal(t, "router", status.System.Hostname)
	assert.Equal(t, 1, len(status.Interfaces))
	assert.Equal(t, "eth0", status.Interfaces[0].Name)
	assert.Equal(t, 1234, status.Daemon.PID)
	assert.True(t, status.Daemon.Running)
	assert.True(t, status.IPForwarding)
}

// TestFormatDuration_EdgeCases tests edge cases in duration formatting
func TestFormatDuration_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		check    func(*testing.T, string)
	}{
		{
			name:     "negative duration",
			duration: -1 * time.Hour,
			check: func(t *testing.T, result string) {
				// Negative durations might produce unexpected output
				// Document the behavior
				t.Logf("Negative duration result: %s", result)
			},
		},
		{
			name:     "very large duration",
			duration: 10000 * 24 * time.Hour,
			check: func(t *testing.T, result string) {
				assert.Contains(t, result, "10000d")
			},
		},
		{
			name:     "microseconds (truncated)",
			duration: 500 * time.Microsecond,
			check: func(t *testing.T, result string) {
				assert.Equal(t, "0m", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			tt.check(t, result)
		})
	}
}

// TestGetSocketPath_ConcurrentAccess tests concurrent socket path resolution
func TestGetSocketPath_ConcurrentAccess(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("JACK_SOCKET_PATH")
	defer os.Setenv("JACK_SOCKET_PATH", originalEnv)

	os.Setenv("JACK_SOCKET_PATH", "/tmp/test.sock")

	// Run concurrent goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			path := getSocketPath()
			assert.Equal(t, "/tmp/test.sock", path)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests

func BenchmarkFormatDuration(b *testing.B) {
	durations := []time.Duration{
		45 * time.Minute,
		2*time.Hour + 30*time.Minute,
		3*24*time.Hour + 5*time.Hour + 15*time.Minute,
		365 * 24 * time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatDuration(durations[i%len(durations)])
	}
}

func BenchmarkGetSocketPath(b *testing.B) {
	os.Setenv("JACK_SOCKET_PATH", "/tmp/test.sock")
	defer os.Unsetenv("JACK_SOCKET_PATH")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getSocketPath()
	}
}
