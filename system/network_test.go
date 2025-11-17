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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

// TestStringSlicesEqual tests the helper function for comparing string slices
func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "equal slices same order",
			a:        []string{"eth0", "eth1", "eth2"},
			b:        []string{"eth0", "eth1", "eth2"},
			expected: true,
		},
		{
			name:     "equal slices different order",
			a:        []string{"eth0", "eth1", "eth2"},
			b:        []string{"eth2", "eth0", "eth1"},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []string{"eth0", "eth1"},
			b:        []string{"eth0", "eth1", "eth2"},
			expected: false,
		},
		{
			name:     "different elements",
			a:        []string{"eth0", "eth1", "eth2"},
			b:        []string{"eth0", "eth1", "eth3"},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "one empty one not",
			a:        []string{},
			b:        []string{"eth0"},
			expected: false,
		},
		{
			name:     "nil vs empty",
			a:        nil,
			b:        []string{},
			expected: true,
		},
		{
			name:     "duplicate elements in a",
			a:        []string{"eth0", "eth0", "eth1"},
			b:        []string{"eth0", "eth1"},
			expected: false,
		},
		{
			name:     "single element match",
			a:        []string{"eth0"},
			b:        []string{"eth0"},
			expected: true,
		},
		{
			name:     "single element no match",
			a:        []string{"eth0"},
			b:        []string{"eth1"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSlicesEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestApplyInterfaceConfig_UnsupportedType tests unsupported interface types
func TestApplyInterfaceConfig_UnsupportedType(t *testing.T) {
	iface := types.Interface{
		Enabled: true,
		Type:    "unsupported",
	}

	err := ApplyInterfaceConfig("test", iface)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported interface type")
}

// TestNetlinkIPParsing tests IP parsing edge cases
func TestNetlinkIPParsing(t *testing.T) {
	tests := []struct {
		name    string
		ipStr   string
		isValid bool
	}{
		{"valid IPv4", "192.168.1.1", true},
		{"valid IPv4 zero", "0.0.0.0", true},
		{"valid IPv4 broadcast", "255.255.255.255", true},
		{"invalid IP", "999.999.999.999", false},
		{"invalid format", "not-an-ip", false},
		{"empty string", "", false},
		{"partial IP", "192.168", false},
		{"IPv6", "::1", true}, // IPv6 is valid but not used in this code
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ipStr)
			if tt.isValid {
				assert.NotNil(t, ip, "Expected valid IP for: %s", tt.ipStr)
			} else {
				assert.Nil(t, ip, "Expected invalid IP for: %s", tt.ipStr)
			}
		})
	}
}

// TestNetmaskParsing tests netmask parsing
func TestNetmaskParsing(t *testing.T) {
	tests := []struct {
		name       string
		netmask    string
		expectSize int // Expected prefix length
	}{
		{"class C", "255.255.255.0", 24},
		{"class B", "255.255.0.0", 16},
		{"class A", "255.0.0.0", 8},
		{"single host", "255.255.255.255", 32},
		{"invalid netmask", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maskIP := net.ParseIP(tt.netmask)
			if maskIP == nil {
				// Invalid netmask
				assert.Equal(t, 0, tt.expectSize)
				return
			}

			mask := net.IPMask(maskIP.To4())
			if mask == nil {
				// Not IPv4
				assert.Equal(t, 0, tt.expectSize)
				return
			}

			size, _ := mask.Size()
			assert.Equal(t, tt.expectSize, size)
		})
	}
}

// Benchmark tests for performance regression detection

func BenchmarkStringSlicesEqual(b *testing.B) {
	a := []string{"eth0", "eth1", "eth2", "eth3", "eth4"}
	c := []string{"eth4", "eth3", "eth2", "eth1", "eth0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stringSlicesEqual(a, c)
	}
}

func BenchmarkNetlinkIPParsing(b *testing.B) {
	ips := []string{
		"192.168.1.1",
		"10.0.0.1",
		"172.16.0.1",
		"255.255.255.255",
		"0.0.0.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = net.ParseIP(ips[i%len(ips)])
	}
}

// TestCheckIPMatches tests IP address matching
