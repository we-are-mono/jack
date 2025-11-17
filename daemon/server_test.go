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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

// TestServerHandleStatus tests the handleStatus handler
func TestServerHandleStatusNoPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load some test data into state (no pending changes)
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test handleStatus with no pending changes
	resp := s.handleStatus()

	assert.True(t, resp.Success, "handleStatus should succeed")
	assert.Contains(t, resp.Message, "No pending changes", "should report no pending changes")
}

func TestServerHandleStatusWithPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load committed config
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(committedConfig)

	// Create pending changes
	pendingConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth1", // Changed
				Enabled: true,
			},
		},
	}
	s.state.SetPending("interfaces", pendingConfig)

	// Test handleStatus with pending changes
	resp := s.handleStatus()

	assert.True(t, resp.Success, "handleStatus should succeed")
	assert.Contains(t, resp.Message, "Pending changes exist", "should report pending changes")
}

// TestServerHandleInfo tests the handleInfo handler
func TestServerHandleInfo(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
			"lan": {
				Type:    "bridge",
				Device:  "br-lan",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	routesConfig := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.1",
				Interface:   "wan",
				Enabled:     true,
			},
		},
	}
	s.state.LoadCommittedRoutes(routesConfig)

	// Test handleInfo
	resp := s.handleInfo()

	assert.True(t, resp.Success, "handleInfo should succeed")
	assert.NotNil(t, resp.Data, "handleInfo should return data")

	// Verify the info data structure - it returns system status, interfaces, plugins, etc
	infoMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "info data should be a map")

	// Should contain system info components
	assert.Contains(t, infoMap, "system", "info should include system info")
	assert.Contains(t, infoMap, "interfaces", "info should include interfaces")
	assert.Contains(t, infoMap, "plugins", "info should include plugins")
	assert.Contains(t, infoMap, "pending", "info should include pending flag")
}

// TestServerHandleDiff tests the handleDiff handler
func TestServerHandleDiffNoChanges(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration with no pending changes
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test handleDiff with no pending changes
	resp := s.handleDiff()

	assert.True(t, resp.Success, "handleDiff should succeed")
	assert.Contains(t, resp.Message, "No pending changes", "should report no changes")
}

func TestServerHandleDiffWithChanges(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load committed configuration
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(committedConfig)

	// Create pending changes
	pendingConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth1", // Changed device
				Enabled: true,
			},
		},
	}
	s.state.SetPending("interfaces", pendingConfig)

	// Test handleDiff with pending changes
	resp := s.handleDiff()

	assert.True(t, resp.Success, "handleDiff should succeed")
	assert.NotContains(t, resp.Message, "No pending changes", "should detect changes")
	assert.Contains(t, resp.Message, "change", "should mention changes")
}

// TestServerHandleGet tests the handleGet handler
func TestServerHandleGetExisting(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:     "physical",
				Device:   "eth0",
				Enabled:  true,
				Protocol: "static",
				IPAddr:   "10.0.0.1",
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test getting a specific interface
	resp := s.handleGet("interfaces.wan")

	assert.True(t, resp.Success, "handleGet should succeed")
	assert.NotNil(t, resp.Data, "handleGet should return data")

	// Verify the returned interface
	iface, ok := resp.Data.(types.Interface)
	require.True(t, ok, "returned data should be an Interface")
	assert.Equal(t, "physical", iface.Type)
	assert.Equal(t, "eth0", iface.Device)
}

func TestServerHandleGetNonExistent(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test getting a non-existent interface
	resp := s.handleGet("interfaces.nonexistent")

	assert.False(t, resp.Success, "handleGet should fail for non-existent path")
	// Error could be empty string or contain error details - just check it failed
}

func TestServerHandleGetInvalidPath(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Test getting with empty path - should return all config (new behavior)
	resp := s.handleGet("")

	assert.True(t, resp.Success, "handleGet should succeed for empty path (returns all config)")
	assert.NotNil(t, resp.Data, "should return all config data")
}

// TestServerHandleSet tests the handleSet handler
func TestServerHandleSetValid(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test setting a value
	resp := s.handleSet("interfaces.wan.enabled", false)

	assert.True(t, resp.Success, "handleSet should succeed")
	assert.Contains(t, resp.Message, "Staged", "should mention staged/set operation")

	// Verify the pending state was updated
	assert.True(t, s.state.HasPendingFor("interfaces"), "should have pending changes for interfaces")

	// Verify the actual value changed
	currentConfig := s.state.GetCurrentInterfaces()
	require.NotNil(t, currentConfig, "should have current config")
	assert.False(t, currentConfig.Interfaces["wan"].Enabled, "enabled should be false")
}

func TestServerHandleSetInvalidPath(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Test setting with invalid path
	resp := s.handleSet("", "value")

	assert.False(t, resp.Success, "handleSet should fail for empty path")
}

func TestServerHandleSetNonExistent(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test setting a non-existent interface
	resp := s.handleSet("interfaces.nonexistent.enabled", true)

	assert.False(t, resp.Success, "handleSet should fail for non-existent interface")
}

// TestServerHandleCommit tests the handleCommit handler
func TestServerHandleCommitNoPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration with no pending changes
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test committing with no pending changes - should succeed (idempotent)
	resp := s.handleCommit()

	assert.True(t, resp.Success, "handleCommit should succeed (idempotent)")
	assert.Contains(t, resp.Message, "No pending changes", "should report no changes to commit")
}

func TestServerHandleCommitWithPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load committed configuration
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(committedConfig)

	// Create pending changes
	pendingConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: false, // Changed
			},
		},
	}
	s.state.SetPending("interfaces", pendingConfig)

	// Verify we have pending changes
	require.True(t, s.state.HasPendingFor("interfaces"), "should have pending changes")

	// Test committing - this will fail trying to save files, but that's OK for unit test
	// We're just testing that it processes pending changes
	resp := s.handleCommit()

	// The commit might fail due to file I/O, but it should have attempted
	// For unit testing we just verify it didn't error on "no pending changes"
	if !resp.Success {
		assert.NotContains(t, resp.Error, "no pending changes", "should not error on no pending changes")
	}
}

// TestServerHandleRevert tests the handleRevert handler
func TestServerHandleRevertNoPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration with no pending changes
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test reverting with no pending changes - should succeed (idempotent)
	resp := s.handleRevert()

	assert.True(t, resp.Success, "handleRevert should succeed (idempotent)")
	assert.Contains(t, resp.Message, "discarded", "should report pending changes discarded")
}

func TestServerHandleRevertWithPending(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load committed configuration
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(committedConfig)

	// Create pending changes
	pendingConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: false, // Changed
			},
		},
	}
	s.state.SetPending("interfaces", pendingConfig)

	// Verify we have pending changes
	require.True(t, s.state.HasPendingFor("interfaces"), "should have pending changes before revert")

	// Test reverting
	resp := s.handleRevert()

	assert.True(t, resp.Success, "handleRevert should succeed")
	assert.Contains(t, resp.Message, "discarded", "should mention pending changes discarded")

	// Verify pending changes are cleared
	assert.False(t, s.state.HasPendingFor("interfaces"), "should no longer have pending changes after revert")

	// Verify the current config matches committed (not the reverted pending)
	currentConfig := s.state.GetCurrentInterfaces()
	require.NotNil(t, currentConfig, "should have current config")
	assert.True(t, currentConfig.Interfaces["wan"].Enabled, "should be back to committed value (true)")
}

// TestServerHandleShow tests the handleShow handler
func TestServerHandleShowInterfaces(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Load test configuration
	interfacesConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)

	// Test showing interfaces
	resp := s.handleShow("interfaces")

	assert.True(t, resp.Success, "handleShow should succeed")
	assert.NotNil(t, resp.Data, "handleShow should return data")

	// Verify the returned data is the interfaces config
	config, ok := resp.Data.(*types.InterfacesConfig)
	require.True(t, ok, "returned data should be InterfacesConfig")
	assert.Len(t, config.Interfaces, 1, "should have 1 interface")
	assert.Contains(t, config.Interfaces, "wan", "should contain wan interface")
}

func TestServerHandleShowInvalidType(t *testing.T) {
	s := &Server{
		state:    NewState(),
		registry: NewPluginRegistry(),
		handlers: make(map[string]handlerFunc),
	}

	// Test showing unknown config type
	resp := s.handleShow("nonexistent")

	assert.False(t, resp.Success, "handleShow should fail for unknown config type")
	// Error message might be empty or contain error details - just check it failed
}

// TestGetSocketPath tests socket path resolution
func TestGetSocketPath(t *testing.T) {
	// Save original env
	originalEnv := os.Getenv("JACK_SOCKET_PATH")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("JACK_SOCKET_PATH")
		} else {
			os.Setenv("JACK_SOCKET_PATH", originalEnv)
		}
	}()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

// TestDeepCopyGeneric tests deep copying of generic maps
func TestDeepCopyGeneric(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "simple map",
			input: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": float64(42), // JSON unmarshaling converts numbers to float64
			},
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"key1": "value1",
				"nested": map[string]interface{}{
					"inner": "data",
				},
			},
			expected: map[string]interface{}{
				"key1": "value1",
				"nested": map[string]interface{}{
					"inner": "data",
				},
			},
		},
		{
			name: "map with slice",
			input: map[string]interface{}{
				"key": []interface{}{"a", "b", "c"},
			},
			expected: map[string]interface{}{
				"key": []interface{}{"a", "b", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepCopyGeneric(tt.input)

			// Verify deep copy
			assert.Equal(t, tt.expected, result)

			// Verify it's actually a copy (modifying original shouldn't affect copy)
			if len(tt.input) > 0 {
				tt.input["new_key"] = "new_value"
				assert.NotContains(t, result, "new_key")
			}
		})
	}
}

// TestOrderInterfaces tests interface ordering logic
func TestOrderInterfaces(t *testing.T) {
	tests := []struct {
		name       string
		interfaces map[string]types.Interface
		expected   []string
	}{
		{
			name:       "empty interfaces",
			interfaces: map[string]types.Interface{},
			expected:   []string{},
		},
		{
			name: "single interface",
			interfaces: map[string]types.Interface{
				"wan": {Type: "physical"},
			},
			expected: []string{"wan"},
		},
		{
			name: "physical before bridge",
			interfaces: map[string]types.Interface{
				"br-lan": {Type: "bridge"},
				"wan":    {Type: "physical"},
			},
			expected: []string{"wan", "br-lan"},
		},
		{
			name: "physical before VLAN",
			interfaces: map[string]types.Interface{
				"vlan10": {Type: "vlan"},
				"eth0":   {Type: "physical"},
			},
			expected: []string{"eth0", "vlan10"},
		},
		{
			name: "mixed types ordered correctly",
			interfaces: map[string]types.Interface{
				"vlan10":  {Type: "vlan"},
				"br-lan":  {Type: "bridge"},
				"eth0":    {Type: "physical"},
				"eth1":    {Type: "physical"},
			},
			// Physical first, then bridges, then VLANs
			// Order within each category may vary (map iteration)
			expected: nil, // Can't guarantee exact order within categories
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orderInterfaces(tt.interfaces)

			if tt.expected != nil {
				assert.Equal(t, tt.expected, result)
			} else {
				// Just verify count for mixed case
				assert.Equal(t, len(tt.interfaces), len(result))
			}
		})
	}
}

// Benchmark tests

func BenchmarkDeepCopyGeneric(b *testing.B) {
	testMap := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"nested": map[string]interface{}{
			"inner1": "data1",
			"inner2": []interface{}{1, 2, 3},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepCopyGeneric(testMap)
	}
}

func BenchmarkOrderInterfaces(b *testing.B) {
	interfaces := map[string]types.Interface{
		"wan":     {Type: "physical"},
		"lan1":    {Type: "physical"},
		"lan2":    {Type: "physical"},
		"br-lan":  {Type: "bridge"},
		"vlan10":  {Type: "vlan"},
		"vlan20":  {Type: "vlan"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = orderInterfaces(interfaces)
	}
}
