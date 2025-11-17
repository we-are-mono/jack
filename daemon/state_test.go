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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

func TestNewState(t *testing.T) {
	state := NewState()

	assert.NotNil(t, state)
	assert.False(t, state.HasPending())
}

func TestState_SetAndGetPending(t *testing.T) {
	state := NewState()

	// Create test config
	testConfig := map[string]interface{}{
		"enabled": true,
		"port":    8080,
	}

	// Set pending config
	state.SetPending("test", testConfig)

	// Verify pending flag
	assert.True(t, state.HasPending())
	assert.True(t, state.HasPendingFor("test"))

	// Get pending config back
	retrieved, err := state.GetCurrent("test")
	require.NoError(t, err)
	assert.Equal(t, testConfig, retrieved)
}

func TestState_CommitPending(t *testing.T) {
	state := NewState()

	// Set pending config
	testConfig := map[string]interface{}{"key": "value"}
	state.SetPending("test", testConfig)

	// Verify pending exists
	assert.True(t, state.HasPending())

	// Commit pending
	err := state.CommitPending()
	require.NoError(t, err)

	// Verify no longer pending
	assert.False(t, state.HasPending())

	// Verify committed
	committed, err := state.GetCommitted("test")
	require.NoError(t, err)
	assert.Equal(t, testConfig, committed)
}

func TestState_RevertPending(t *testing.T) {
	state := NewState()

	// Set committed config
	committedConfig := map[string]interface{}{"version": "1.0"}
	state.LoadCommitted("test", committedConfig)

	// Set different pending config
	pendingConfig := map[string]interface{}{"version": "2.0"}
	state.SetPending("test", pendingConfig)

	// Verify pending exists
	assert.True(t, state.HasPending())

	// Revert pending
	err := state.RevertPending()
	require.NoError(t, err)

	// Verify no longer pending
	assert.False(t, state.HasPending())

	// Verify current returns committed
	current, err := state.GetCurrent("test")
	require.NoError(t, err)
	assert.Equal(t, committedConfig, current)
}

func TestState_GetPendingTypes(t *testing.T) {
	state := NewState()

	// Initially empty
	pendingTypes := state.GetPendingTypes()
	assert.Empty(t, pendingTypes)

	// Add pending for multiple types
	state.SetPending("firewall", map[string]interface{}{"enabled": true})
	state.SetPending("dhcp", map[string]interface{}{"enabled": true})
	state.SetPending("vpn", map[string]interface{}{"enabled": true})

	// Get pending types
	pendingTypes = state.GetPendingTypes()
	assert.Len(t, pendingTypes, 3)
	assert.Contains(t, pendingTypes, "firewall")
	assert.Contains(t, pendingTypes, "dhcp")
	assert.Contains(t, pendingTypes, "vpn")
}

func TestState_LoadCommittedInterfaces(t *testing.T) {
	state := NewState()

	// Create test interfaces config
	config := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {
				Type:     "physical",
				Protocol: "static",
				IPAddr:   "192.168.1.1",
				Netmask:  "255.255.255.0",
			},
		},
	}

	// Load interfaces
	state.LoadCommittedInterfaces(config)

	// Verify loaded
	retrieved := state.GetCommittedInterfaces()
	assert.NotNil(t, retrieved)
	assert.Len(t, retrieved.Interfaces, 1)
	assert.Contains(t, retrieved.Interfaces, "eth0")
	assert.Equal(t, "physical", retrieved.Interfaces["eth0"].Type)
}

func TestState_LoadCommittedRoutes(t *testing.T) {
	state := NewState()

	// Create test routes config
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"route1": {
				Destination: "10.0.0.0/8",
				Gateway:     "192.168.1.254",
				Interface:   "eth0",
			},
		},
	}

	// Load routes
	state.LoadCommittedRoutes(config)

	// Verify loaded using generic GetCommitted
	retrievedInterface, err := state.GetCommitted("routes")
	require.NoError(t, err)
	retrieved, ok := retrievedInterface.(*types.RoutesConfig)
	require.True(t, ok, "GetCommitted should return *types.RoutesConfig")
	assert.NotNil(t, retrieved)
	assert.Len(t, retrieved.Routes, 1)
	route1, exists := retrieved.Routes["route1"]
	assert.True(t, exists)
	assert.Equal(t, "10.0.0.0/8", route1.Destination)
}

func TestState_GetCurrentInterfaces_WithPending(t *testing.T) {
	state := NewState()

	// Load committed
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {Type: "physical"},
		},
	}
	state.LoadCommittedInterfaces(committedConfig)

	// Set pending
	pendingConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {Type: "physical"},
			"eth1": {Type: "physical"},
		},
	}
	state.SetPending("interfaces", pendingConfig)

	// GetCurrent should return pending
	current := state.GetCurrentInterfaces()
	assert.Len(t, current.Interfaces, 2)
	assert.Contains(t, current.Interfaces, "eth0")
	assert.Contains(t, current.Interfaces, "eth1")
}

func TestState_GetCurrentInterfaces_WithoutPending(t *testing.T) {
	state := NewState()

	// Load committed only
	committedConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {Type: "physical"},
		},
	}
	state.LoadCommittedInterfaces(committedConfig)

	// GetCurrent should return committed
	current := state.GetCurrentInterfaces()
	assert.Len(t, current.Interfaces, 1)
	assert.Contains(t, current.Interfaces, "eth0")
}

func TestState_LoadCommitted_Generic(t *testing.T) {
	state := NewState()

	// Load generic plugin config
	config := map[string]interface{}{
		"enabled": true,
		"port":    9090,
	}

	state.LoadCommitted("monitoring", config)

	// Retrieve committed
	retrieved, err := state.GetCommitted("monitoring")
	require.NoError(t, err)
	assert.Equal(t, config, retrieved)
}

// TestState_GetCurrentRoutes tests the GetCurrentRoutes method
func TestState_GetCurrentRoutes(t *testing.T) {
	state := NewState()

	// Load committed routes
	committedConfig := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.1",
				Interface:   "wan",
				Enabled:     true,
			},
		},
	}
	state.LoadCommittedRoutes(committedConfig)

	// Test GetCurrentRoutes without pending
	current := state.GetCurrentRoutes()
	assert.NotNil(t, current)
	assert.Len(t, current.Routes, 1)
	assert.Contains(t, current.Routes, "default")

	// Set pending routes
	pendingConfig := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.254", // Changed
				Interface:   "wan",
				Enabled:     true,
			},
		},
	}
	state.SetPending("routes", pendingConfig)

	// Test GetCurrentRoutes with pending - should return pending
	current = state.GetCurrentRoutes()
	assert.NotNil(t, current)
	assert.Equal(t, "10.0.0.254", current.Routes["default"].Gateway)
}

func TestState_GetCurrentRoutesNil(t *testing.T) {
	state := NewState()

	// GetCurrentRoutes with no routes loaded should return nil
	current := state.GetCurrentRoutes()
	assert.Nil(t, current)
}

// TestState_SetLastApplied tests the SetLastApplied method
func TestState_SetLastApplied(t *testing.T) {
	state := NewState()

	// Set last applied config
	appliedConfig := map[string]interface{}{
		"enabled": true,
		"version": "1.0",
	}
	state.SetLastApplied("firewall", appliedConfig)

	// Verify it was set
	retrieved, err := state.GetLastApplied("firewall")
	require.NoError(t, err)
	assert.Equal(t, appliedConfig, retrieved)
}

// TestState_GetLastApplied tests the GetLastApplied method
func TestState_GetLastApplied(t *testing.T) {
	state := NewState()

	// Test getting last applied for non-existent config type
	_, err := state.GetLastApplied("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config type")

	// Load committed config first
	committedConfig := map[string]interface{}{"key": "value"}
	state.LoadCommitted("test", committedConfig)

	// Get last applied when it hasn't been set - should return nil
	retrieved, err := state.GetLastApplied("test")
	require.NoError(t, err)
	assert.Nil(t, retrieved)

	// Set last applied
	appliedConfig := map[string]interface{}{"key": "new_value"}
	state.SetLastApplied("test", appliedConfig)

	// Get last applied - should return the applied config
	retrieved, err = state.GetLastApplied("test")
	require.NoError(t, err)
	assert.Equal(t, appliedConfig, retrieved)
}

// TestState_ConfigsEqual tests the ConfigsEqual function
func TestState_ConfigsEqualIdentical(t *testing.T) {
	config1 := map[string]interface{}{
		"enabled": true,
		"port":    8080,
		"host":    "localhost",
	}

	config2 := map[string]interface{}{
		"enabled": true,
		"port":    8080,
		"host":    "localhost",
	}

	assert.True(t, ConfigsEqual(config1, config2), "identical configs should be equal")
}

func TestState_ConfigsEqualDifferent(t *testing.T) {
	config1 := map[string]interface{}{
		"enabled": true,
		"port":    8080,
	}

	config2 := map[string]interface{}{
		"enabled": false, // Different
		"port":    8080,
	}

	assert.False(t, ConfigsEqual(config1, config2), "different configs should not be equal")
}

func TestState_ConfigsEqualNil(t *testing.T) {
	assert.True(t, ConfigsEqual(nil, nil), "both nil should be equal")
	assert.False(t, ConfigsEqual(nil, map[string]interface{}{}), "nil and non-nil should not be equal")
	assert.False(t, ConfigsEqual(map[string]interface{}{}, nil), "non-nil and nil should not be equal")
}

func TestState_ConfigsEqualStructs(t *testing.T) {
	config1 := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}

	config2 := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"wan": {
				Type:    "physical",
				Device:  "eth0",
				Enabled: true,
			},
		},
	}

	assert.True(t, ConfigsEqual(config1, config2), "identical struct configs should be equal")

	// Modify config2
	config2.Interfaces["wan"] = types.Interface{
		Type:    "physical",
		Device:  "eth1", // Different
		Enabled: true,
	}

	assert.False(t, ConfigsEqual(config1, config2), "different struct configs should not be equal")
}

// TestState_GetCommittedInterfacesNil tests edge case where interfaces are not loaded
func TestState_GetCommittedInterfacesNil(t *testing.T) {
	state := NewState()

	// GetCommittedInterfaces with no interfaces loaded should return nil
	retrieved := state.GetCommittedInterfaces()
	assert.Nil(t, retrieved)
}

// TestState_GetCurrentInterfacesNil tests edge case where interfaces are not loaded
func TestState_GetCurrentInterfacesNil(t *testing.T) {
	state := NewState()

	// GetCurrentInterfaces with no interfaces loaded should return nil
	retrieved := state.GetCurrentInterfaces()
	assert.Nil(t, retrieved)
}

// TestState_GetCommittedNonExistent tests getting committed config for unknown type
func TestState_GetCommittedNonExistent(t *testing.T) {
	state := NewState()

	_, err := state.GetCommitted("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config type")
}

// TestState_HasPendingForNonExistent tests checking pending for unknown type
func TestState_HasPendingForNonExistent(t *testing.T) {
	state := NewState()

	// HasPendingFor should return false for unknown type (not error)
	assert.False(t, state.HasPendingFor("nonexistent"))
}

// TestState_CommitPendingNoPending tests committing when nothing is pending
func TestState_CommitPendingNoPending(t *testing.T) {
	state := NewState()

	err := state.CommitPending()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending changes")
}

// TestState_RevertPendingNoPending tests reverting when nothing is pending
func TestState_RevertPendingNoPending(t *testing.T) {
	state := NewState()

	// Reverting with no pending changes should succeed (idempotent)
	err := state.RevertPending()
	assert.NoError(t, err)
}
