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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateRates tests bandwidth rate calculations
func TestCalculateRates(t *testing.T) {
	baseTime := time.Now()

	tests := []struct {
		name     string
		prev     []InterfaceMetrics
		current  []InterfaceMetrics
		expected map[string]struct{ rx, tx uint64 }
	}{
		{
			name: "single interface with traffic",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   2000,
					TXBytes:   4000,
					Timestamp: baseTime.Add(1 * time.Second),
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0": {rx: 1000, tx: 2000},
			},
		},
		{
			name: "multiple interfaces",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
				{
					Name:      "eth1",
					RXBytes:   5000,
					TXBytes:   6000,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   3000,
					TXBytes:   6000,
					Timestamp: baseTime.Add(2 * time.Second),
				},
				{
					Name:      "eth1",
					RXBytes:   7000,
					TXBytes:   8000,
					Timestamp: baseTime.Add(2 * time.Second),
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0": {rx: 1000, tx: 2000},
				"eth1": {rx: 1000, tx: 1000},
			},
		},
		{
			name: "new interface appears",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   2000,
					TXBytes:   4000,
					Timestamp: baseTime.Add(1 * time.Second),
				},
				{
					Name:      "wlan0",
					RXBytes:   500,
					TXBytes:   1000,
					Timestamp: baseTime.Add(1 * time.Second),
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0":  {rx: 1000, tx: 2000},
				"wlan0": {rx: 0, tx: 0}, // No previous data
			},
		},
		{
			name: "interface disappears",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
				{
					Name:      "wlan0",
					RXBytes:   500,
					TXBytes:   1000,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   2000,
					TXBytes:   4000,
					Timestamp: baseTime.Add(1 * time.Second),
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0": {rx: 1000, tx: 2000},
			},
		},
		{
			name:    "empty previous metrics",
			prev:    []InterfaceMetrics{},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0": {rx: 0, tx: 0},
			},
		},
		{
			name: "zero time delta",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   1000,
					TXBytes:   2000,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   2000,
					TXBytes:   4000,
					Timestamp: baseTime, // Same timestamp
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				"eth0": {rx: 0, tx: 0}, // No time passed
			},
		},
		{
			name: "counter overflow handling",
			prev: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   ^uint64(0) - 100, // Near max value
					TXBytes:   ^uint64(0) - 200,
					Timestamp: baseTime,
				},
			},
			current: []InterfaceMetrics{
				{
					Name:      "eth0",
					RXBytes:   50,  // Wrapped around
					TXBytes:   100, // Wrapped around
					Timestamp: baseTime.Add(1 * time.Second),
				},
			},
			expected: map[string]struct{ rx, tx uint64 }{
				// Will underflow, but that's expected behavior
				"eth0": {rx: 151, tx: 301},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRates(tt.prev, tt.current)

			require.Len(t, result, len(tt.expected), "result length mismatch")

			for _, metric := range result {
				expected, ok := tt.expected[metric.Name]
				require.True(t, ok, "unexpected interface: %s", metric.Name)

				assert.Equal(t, expected.rx, metric.RXBytesRate,
					"interface %s RX rate mismatch", metric.Name)
				assert.Equal(t, expected.tx, metric.TXBytesRate,
					"interface %s TX rate mismatch", metric.Name)
			}
		})
	}
}

// TestCalculateRates_Precision tests rate calculation precision
func TestCalculateRates_Precision(t *testing.T) {
	baseTime := time.Now()

	prev := []InterfaceMetrics{
		{
			Name:      "eth0",
			RXBytes:   0,
			TXBytes:   0,
			Timestamp: baseTime,
		},
	}

	current := []InterfaceMetrics{
		{
			Name:      "eth0",
			RXBytes:   1500, // 1500 bytes in 500ms = 3000 bytes/sec
			TXBytes:   3000, // 3000 bytes in 500ms = 6000 bytes/sec
			Timestamp: baseTime.Add(500 * time.Millisecond),
		},
	}

	result := CalculateRates(prev, current)

	require.Len(t, result, 1)
	assert.Equal(t, uint64(3000), result[0].RXBytesRate, "RX rate should be 3000 B/s")
	assert.Equal(t, uint64(6000), result[0].TXBytesRate, "TX rate should be 6000 B/s")
}

// TestGetSystemMetrics tests system metrics collection structure
func TestGetSystemMetrics(t *testing.T) {
	// This test verifies the function returns a valid structure
	// Actual values depend on the system and /proc filesystem
	metrics := GetSystemMetrics()

	require.NotNil(t, metrics)
	assert.False(t, metrics.Timestamp.IsZero(), "timestamp should be set")

	// Metrics should be reasonable (or zero if reading failed)
	assert.True(t, metrics.CPUPercent >= 0 && metrics.CPUPercent <= 100,
		"CPU percent should be 0-100, got %f", metrics.CPUPercent)
	assert.True(t, metrics.MemoryPercent >= 0 && metrics.MemoryPercent <= 100,
		"Memory percent should be 0-100, got %f", metrics.MemoryPercent)

	// Load averages can be any positive number
	assert.True(t, metrics.LoadAvg1 >= 0, "load avg should be non-negative")
	assert.True(t, metrics.LoadAvg5 >= 0, "load avg should be non-negative")
	assert.True(t, metrics.LoadAvg15 >= 0, "load avg should be non-negative")
}

// TestGetInterfaceMetrics tests interface metrics collection structure
func TestGetInterfaceMetrics(t *testing.T) {
	// This test verifies the function returns valid structures
	// Actual interfaces depend on the system
	metrics, err := GetInterfaceMetrics()

	// Function should not error, but may return empty list
	require.NoError(t, err)

	// Verify structure if any interfaces exist
	for _, m := range metrics {
		assert.NotEmpty(t, m.Name, "interface should have a name")
		assert.False(t, m.Timestamp.IsZero(), "timestamp should be set")
		assert.Contains(t, []string{"UP", "DOWN"}, m.State, "state should be UP or DOWN")

		// Loopback should be skipped
		assert.NotEqual(t, "lo", m.Name, "loopback should be skipped")
	}
}

// BenchmarkCalculateRates benchmarks rate calculation performance
func BenchmarkCalculateRates(b *testing.B) {
	baseTime := time.Now()

	prev := make([]InterfaceMetrics, 10)
	current := make([]InterfaceMetrics, 10)

	for i := 0; i < 10; i++ {
		prev[i] = InterfaceMetrics{
			Name:      string(rune('a' + i)),
			RXBytes:   uint64(i * 1000),
			TXBytes:   uint64(i * 2000),
			Timestamp: baseTime,
		}
		current[i] = InterfaceMetrics{
			Name:      string(rune('a' + i)),
			RXBytes:   uint64(i*1000 + 5000),
			TXBytes:   uint64(i*2000 + 10000),
			Timestamp: baseTime.Add(1 * time.Second),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateRates(prev, current)
	}
}

// BenchmarkGetSystemMetrics benchmarks system metrics collection
func BenchmarkGetSystemMetrics(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetSystemMetrics()
	}
}
