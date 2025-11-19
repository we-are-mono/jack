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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContainsScope tests the containsScope helper function.
func TestContainsScope(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		scope    string
		expected bool
	}{
		{
			name:     "scope found in list",
			scopes:   []string{"all", "interfaces", "routes"},
			scope:    "interfaces",
			expected: true,
		},
		{
			name:     "scope not found in list",
			scopes:   []string{"interfaces", "routes"},
			scope:    "firewall",
			expected: false,
		},
		{
			name:     "empty list",
			scopes:   []string{},
			scope:    "interfaces",
			expected: false,
		},
		{
			name:     "nil list",
			scopes:   nil,
			scope:    "interfaces",
			expected: false,
		},
		{
			name:     "scope at beginning",
			scopes:   []string{"ipforward", "interfaces", "routes"},
			scope:    "ipforward",
			expected: true,
		},
		{
			name:     "scope at end",
			scopes:   []string{"interfaces", "routes", "all"},
			scope:    "all",
			expected: true,
		},
		{
			name:     "case sensitive match",
			scopes:   []string{"all", "Interfaces"},
			scope:    "interfaces",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsScope(tt.scopes, tt.scope)
			assert.Equal(t, tt.expected, result, "containsScope(%v, %q)", tt.scopes, tt.scope)
		})
	}
}

// TestContainsString tests the containsString helper function.
func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		str      string
		expected bool
	}{
		{
			name:     "string found in slice",
			slice:    []string{"192.168.1.1/24", "10.0.0.1/8"},
			str:      "192.168.1.1/24",
			expected: true,
		},
		{
			name:     "string not found in slice",
			slice:    []string{"192.168.1.1/24", "10.0.0.1/8"},
			str:      "172.16.0.1/16",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			str:      "192.168.1.1/24",
			expected: false,
		},
		{
			name:     "nil slice",
			slice:    nil,
			str:      "192.168.1.1/24",
			expected: false,
		},
		{
			name:     "empty string search",
			slice:    []string{"a", "b", "c"},
			str:      "",
			expected: false,
		},
		{
			name:     "empty string in slice",
			slice:    []string{"a", "", "c"},
			str:      "",
			expected: true,
		},
		{
			name:     "string at beginning",
			slice:    []string{"eth0", "eth1", "eth2"},
			str:      "eth0",
			expected: true,
		},
		{
			name:     "string at end",
			slice:    []string{"eth0", "eth1", "eth2"},
			str:      "eth2",
			expected: true,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Eth0", "eth1"},
			str:      "eth0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.slice, tt.str)
			assert.Equal(t, tt.expected, result, "containsString(%v, %q)", tt.slice, tt.str)
		})
	}
}

// NOTE: snapshotRoutesMatch is not tested here because it calls netlink.LinkByIndex()
// directly, which requires actual network interfaces. This function will need to be
// refactored to accept a NetlinkClient interface before it can be unit tested.
// For now, it is covered by integration tests.
