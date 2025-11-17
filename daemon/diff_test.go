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

package daemon

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

// TestDiffInterfacesNoChanges tests that identical interfaces produce no diff
func TestDiffInterfacesNoChanges(t *testing.T) {
	config := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth0",
			},
		},
	}

	diffs := DiffInterfaces(config, config)
	assert.Empty(t, diffs)
}

// TestDiffInterfacesAdded tests detecting added interfaces
func TestDiffInterfacesAdded(t *testing.T) {
	old := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}

	new := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth0",
			},
		},
	}

	diffs := DiffInterfaces(old, new)
	require.Len(t, diffs, 1)
	assert.Equal(t, "added", diffs[0].ChangeType)
	assert.Equal(t, "interfaces.wan", diffs[0].Path)
}

// TestDiffInterfacesRemoved tests detecting removed interfaces
func TestDiffInterfacesRemoved(t *testing.T) {
	old := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth0",
			},
		},
	}

	new := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}

	diffs := DiffInterfaces(old, new)
	require.Len(t, diffs, 1)
	assert.Equal(t, "removed", diffs[0].ChangeType)
	assert.Equal(t, "interfaces.wan", diffs[0].Path)
}

// TestDiffInterfacesModified tests detecting modified interface fields
func TestDiffInterfacesModified(t *testing.T) {
	old := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth0",
			},
		},
	}

	new := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth1", // Modified
			},
		},
	}

	diffs := DiffInterfaces(old, new)
	require.NotEmpty(t, diffs)
	assert.Equal(t, "modified", diffs[0].ChangeType)
	assert.Contains(t, diffs[0].Path, "wan")
}

// TestDiffInterfacesMultipleChanges tests multiple changes at once
func TestDiffInterfacesMultipleChanges(t *testing.T) {
	old := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth0",
			},
			"lan": {
				Type:   "bridge",
				Device: "br-lan",
			},
		},
	}

	new := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:   "physical",
				Device: "eth1", // Modified
			},
			// lan removed
			"vlan": { // Added
				Type:   "vlan",
				Device: "eth0.100",
			},
		},
	}

	diffs := DiffInterfaces(old, new)
	assert.NotEmpty(t, diffs)

	// Should have changes for modified, removed, and added
	changeTypes := make(map[string]int)
	for _, diff := range diffs {
		changeTypes[diff.ChangeType]++
	}

	assert.Greater(t, changeTypes["modified"], 0, "Should have modified changes")
	assert.Greater(t, changeTypes["removed"], 0, "Should have removed interfaces")
	assert.Greater(t, changeTypes["added"], 0, "Should have added interfaces")
}

// TestFormatDiffNoChanges tests formatting when there are no changes
func TestFormatDiffNoChanges(t *testing.T) {
	formatted := FormatDiff([]DiffResult{})
	assert.Equal(t, "No changes", formatted)
}

// TestFormatDiffAdded tests formatting added changes
func TestFormatDiffAdded(t *testing.T) {
	diffs := []DiffResult{
		{
			Path:       "interfaces.wan",
			ChangeType: "added",
			New:        "value",
		},
	}

	formatted := FormatDiff(diffs)
	assert.Contains(t, formatted, "+")
	assert.Contains(t, formatted, "interfaces.wan")
	assert.Contains(t, formatted, "added")
}

// TestFormatDiffRemoved tests formatting removed changes
func TestFormatDiffRemoved(t *testing.T) {
	diffs := []DiffResult{
		{
			Path:       "interfaces.lan",
			ChangeType: "removed",
			Old:        "value",
		},
	}

	formatted := FormatDiff(diffs)
	assert.Contains(t, formatted, "-")
	assert.Contains(t, formatted, "interfaces.lan")
	assert.Contains(t, formatted, "removed")
}

// TestFormatDiffModified tests formatting modified changes
func TestFormatDiffModified(t *testing.T) {
	diffs := []DiffResult{
		{
			Path:       "interfaces.wan.device",
			ChangeType: "modified",
			Old:        "eth0",
			New:        "eth1",
		},
	}

	formatted := FormatDiff(diffs)
	assert.Contains(t, formatted, "~")
	assert.Contains(t, formatted, "interfaces.wan.device")
	assert.Contains(t, formatted, "eth0")
	assert.Contains(t, formatted, "eth1")
	assert.Contains(t, formatted, "→")
}

// TestFormatDiffMultiple tests formatting multiple changes
func TestFormatDiffMultiple(t *testing.T) {
	diffs := []DiffResult{
		{
			Path:       "interfaces.wan",
			ChangeType: "added",
			New:        "config",
		},
		{
			Path:       "interfaces.lan",
			ChangeType: "removed",
			Old:        "config",
		},
		{
			Path:       "interfaces.vlan.device",
			ChangeType: "modified",
			Old:        "eth0.10",
			New:        "eth0.20",
		},
	}

	formatted := FormatDiff(diffs)
	assert.Contains(t, formatted, "Found 3 change(s)")
	assert.Contains(t, formatted, "+")
	assert.Contains(t, formatted, "-")
	assert.Contains(t, formatted, "~")
}

// TestDiffConfigsInvalidType tests error handling for invalid types
func TestDiffConfigsInvalidType(t *testing.T) {
	// Try to diff interfaces with wrong type
	_, err := DiffConfigs("interfaces", "not-a-config", "also-not-a-config")
	assert.Error(t, err)
}

// TestDiffResultStructure tests DiffResult structure
func TestDiffResultStructure(t *testing.T) {
	diff := DiffResult{
		Path:       "test.path",
		Old:        "old_value",
		New:        "new_value",
		ChangeType: "modified",
	}

	assert.Equal(t, "test.path", diff.Path)
	assert.Equal(t, "old_value", diff.Old)
	assert.Equal(t, "new_value", diff.New)
	assert.Equal(t, "modified", diff.ChangeType)
}

// TestFormatValueEdgeCases tests formatValue helper with edge cases
func TestFormatValueEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		contains string
	}{
		{
			name:     "nil value",
			input:    nil,
			contains: "none",
		},
		{
			name:     "empty string",
			input:    "",
			contains: "empty",
		},
		{
			name:     "string value",
			input:    "test",
			contains: "test",
		},
		{
			name:     "numeric value",
			input:    42,
			contains: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			assert.Contains(t, strings.ToLower(result), strings.ToLower(tt.contains))
		})
	}
}

// TestDiffConfigsRoutes tests diffing route configurations
func TestDiffConfigsRoutes(t *testing.T) {
	old := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.1",
				Interface:   "wan",
				Enabled:     true,
			},
		},
	}

	new := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.254", // Changed
				Interface:   "wan",
				Enabled:     true,
			},
		},
	}

	diffs, err := DiffConfigs("routes", old, new)
	require.NoError(t, err)
	assert.NotEmpty(t, diffs)
}

// TestDiffConfigsGeneric tests diffing generic plugin configurations
func TestDiffConfigsGeneric(t *testing.T) {
	type PluginConfig struct {
		Enabled bool   `json:"enabled"`
		Port    int    `json:"port"`
		Host    string `json:"host"`
	}

	old := &PluginConfig{
		Enabled: true,
		Port:    8080,
		Host:    "localhost",
	}

	new := &PluginConfig{
		Enabled: true,
		Port:    9090, // Changed
		Host:    "localhost",
	}

	diffs, err := DiffConfigs("plugin", old, new)
	require.NoError(t, err)
	assert.NotEmpty(t, diffs)
}

// TestIsZeroValueAllTypes tests isZeroValue with all supported types
func TestIsZeroValueAllTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		wantZero bool
	}{
		// Arrays and slices
		{name: "empty slice", value: []string{}, wantZero: true},
		{name: "non-empty slice", value: []string{"item"}, wantZero: false},
		{name: "empty map", value: map[string]string{}, wantZero: true},
		{name: "non-empty map", value: map[string]string{"key": "val"}, wantZero: false},
		{name: "empty string", value: "", wantZero: true},
		{name: "non-empty string", value: "text", wantZero: false},

		// Booleans
		{name: "false bool", value: false, wantZero: true},
		{name: "true bool", value: true, wantZero: false},

		// Integers
		{name: "zero int", value: int(0), wantZero: true},
		{name: "non-zero int", value: int(42), wantZero: false},
		{name: "zero int8", value: int8(0), wantZero: true},
		{name: "zero int16", value: int16(0), wantZero: true},
		{name: "zero int32", value: int32(0), wantZero: true},
		{name: "zero int64", value: int64(0), wantZero: true},

		// Unsigned integers
		{name: "zero uint", value: uint(0), wantZero: true},
		{name: "non-zero uint", value: uint(42), wantZero: false},
		{name: "zero uint8", value: uint8(0), wantZero: true},
		{name: "zero uint16", value: uint16(0), wantZero: true},
		{name: "zero uint32", value: uint32(0), wantZero: true},
		{name: "zero uint64", value: uint64(0), wantZero: true},

		// Floats
		{name: "zero float32", value: float32(0), wantZero: true},
		{name: "non-zero float32", value: float32(3.14), wantZero: false},
		{name: "zero float64", value: float64(0), wantZero: true},
		{name: "non-zero float64", value: float64(3.14), wantZero: false},

		// Pointers and interfaces
		{name: "nil pointer", value: (*string)(nil), wantZero: true},
		{name: "nil interface", value: (interface{})(nil), wantZero: false}, // nil interface is not a reflect.Value with IsNil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == nil {
				// Special case for actual nil
				return
			}
			v := reflect.ValueOf(tt.value)
			got := isZeroValue(v)
			assert.Equal(t, tt.wantZero, got, "isZeroValue(%v) = %v, want %v", tt.value, got, tt.wantZero)
		})
	}
}

// TestFormatValueAllTypes tests formatValue with various types
func TestFormatValueAllTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{name: "nil", input: nil, expected: "(none)"},
		{name: "empty string", input: "", expected: "(empty)"},
		{name: "string", input: "hello", expected: `"hello"`},
		{name: "string slice", input: []string{"a", "b"}, expected: "[a b]"},
		{name: "integer", input: 42, expected: "42"},
		{name: "boolean", input: true, expected: "true"},
		{name: "empty slice", input: []string{}, expected: "(empty)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDiffConfigsNilValues tests diffing with nil configurations
func TestDiffConfigsNilValues(t *testing.T) {
	old := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}
	new := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}

	diffs, err := DiffConfigs("interfaces", old, new)
	require.NoError(t, err)
	assert.Empty(t, diffs)
}

// TestDiffStructsWithZeroValues tests that zero-to-zero changes are ignored
func TestDiffStructsWithZeroValues(t *testing.T) {
	type TestStruct struct {
		EmptyString  string `json:"empty_string"`
		ZeroInt      int    `json:"zero_int"`
		FalseBool    bool   `json:"false_bool"`
		NonZeroField string `json:"non_zero"`
	}

	old := TestStruct{
		EmptyString:  "",
		ZeroInt:      0,
		FalseBool:    false,
		NonZeroField: "value",
	}

	new := TestStruct{
		EmptyString:  "",
		ZeroInt:      0,
		FalseBool:    false,
		NonZeroField: "changed",
	}

	diffs := diffStructs("test", old, new)
	// Should only report the non-zero field change
	assert.Len(t, diffs, 1)
	assert.Equal(t, "test.non_zero", diffs[0].Path)
}

// TestDiffConfigsPointerHandling tests that pointer dereferencing works
func TestDiffConfigsPointerHandling(t *testing.T) {
	type TestConfig struct {
		Value string `json:"value"`
	}

	old := &TestConfig{Value: "old"}
	new := &TestConfig{Value: "new"}

	diffs, err := DiffConfigs("test", old, new)
	require.NoError(t, err)
	assert.NotEmpty(t, diffs)
}

// TestFormatDiffOutput tests complete diff formatting
func TestFormatDiffOutput(t *testing.T) {
	diffs := []DiffResult{
		{
			Path:       "config.port",
			ChangeType: "modified",
			Old:        8080,
			New:        9090,
		},
	}

	formatted := FormatDiff(diffs)
	assert.Contains(t, formatted, "Found 1 change(s)")
	assert.Contains(t, formatted, "config.port")
	assert.Contains(t, formatted, "8080")
	assert.Contains(t, formatted, "9090")
}
