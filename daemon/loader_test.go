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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/plugins"
)

// mockProvider implements plugins.Provider for testing
type mockProvider struct {
	metadataFunc       func(ctx context.Context) (plugins.MetadataResponse, error)
	applyConfigFunc    func(ctx context.Context, configJSON []byte) error
	validateConfigFunc func(ctx context.Context, configJSON []byte) error
	flushFunc          func(ctx context.Context) error
	statusFunc         func(ctx context.Context) ([]byte, error)
	executeCLIFunc     func(ctx context.Context, command string, args []string) ([]byte, error)
}

func (m *mockProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	if m.metadataFunc != nil {
		return m.metadataFunc(ctx)
	}
	return plugins.MetadataResponse{
		Namespace:   "test",
		Version:     "1.0.0",
		Description: "Test plugin",
	}, nil
}

func (m *mockProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	if m.applyConfigFunc != nil {
		return m.applyConfigFunc(ctx, configJSON)
	}
	return nil
}

func (m *mockProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	if m.validateConfigFunc != nil {
		return m.validateConfigFunc(ctx, configJSON)
	}
	return nil
}

func (m *mockProvider) Flush(ctx context.Context) error {
	if m.flushFunc != nil {
		return m.flushFunc(ctx)
	}
	return nil
}

func (m *mockProvider) Status(ctx context.Context) ([]byte, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return json.Marshal(map[string]interface{}{"status": "running"})
}

func (m *mockProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	if m.executeCLIFunc != nil {
		return m.executeCLIFunc(ctx, command, args)
	}
	return []byte("command output"), nil
}

func (m *mockProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	// Mock implementation - just return nil
	return nil
}

// mockPluginWithMetadata implements plugins.Plugin for testing with full metadata
type mockPluginWithMetadata struct {
	metadata plugins.PluginMetadata
}

func (m *mockPluginWithMetadata) Metadata() plugins.PluginMetadata {
	return m.metadata
}

func (m *mockPluginWithMetadata) ApplyConfig(config interface{}) error {
	return nil
}

func (m *mockPluginWithMetadata) ValidateConfig(config interface{}) error {
	return nil
}

func (m *mockPluginWithMetadata) Flush() error {
	return nil
}

func (m *mockPluginWithMetadata) Status() (interface{}, error) {
	return map[string]interface{}{"status": "ok"}, nil
}

func (m *mockPluginWithMetadata) Close() error {
	return nil
}

// TestPluginLoader_Metadata tests that PluginLoader returns correct metadata
func TestPluginLoader_Metadata(t *testing.T) {
	metadata := plugins.PluginMetadata{
		Namespace:   "firewall",
		Version:     "2.0.0",
		Description: "Firewall management",
		ConfigPath:  "/etc/jack/firewall.json",
	}

	loader := &PluginLoader{
		metadata: metadata,
	}

	result := loader.Metadata()
	assert.Equal(t, "firewall", result.Namespace)
	assert.Equal(t, "2.0.0", result.Version)
	assert.Equal(t, "Firewall management", result.Description)
	assert.Equal(t, "/etc/jack/firewall.json", result.ConfigPath)
}

// TestPluginLoader_ApplyConfig tests config marshaling and application
func TestPluginLoader_ApplyConfig(t *testing.T) {
	var receivedConfig []byte

	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			receivedConfig = configJSON
			return nil
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	config := map[string]interface{}{
		"enabled": true,
		"setting": "value",
	}

	err := loader.ApplyConfig(config)
	require.NoError(t, err)
	assert.NotEmpty(t, receivedConfig)

	// Verify JSON was properly marshaled
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(receivedConfig, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, true, unmarshaled["enabled"])
	assert.Equal(t, "value", unmarshaled["setting"])
}

// TestPluginLoader_ValidateConfig tests config validation
func TestPluginLoader_ValidateConfig(t *testing.T) {
	var receivedConfig []byte

	provider := &mockProvider{
		validateConfigFunc: func(ctx context.Context, configJSON []byte) error {
			receivedConfig = configJSON
			return nil
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	config := map[string]interface{}{
		"port": 8080,
	}

	err := loader.ValidateConfig(config)
	require.NoError(t, err)
	assert.NotEmpty(t, receivedConfig)
}

// TestPluginLoader_Flush tests flush operation
func TestPluginLoader_Flush(t *testing.T) {
	flushCalled := false

	provider := &mockProvider{
		flushFunc: func(ctx context.Context) error {
			flushCalled = true
			return nil
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	err := loader.Flush()
	require.NoError(t, err)
	assert.True(t, flushCalled)
}

// TestPluginLoader_Status tests status retrieval
func TestPluginLoader_Status(t *testing.T) {
	provider := &mockProvider{
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return json.Marshal(map[string]interface{}{
				"active":  true,
				"uptime":  3600,
				"metrics": map[string]int{"connections": 42},
			})
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	status, err := loader.Status()
	require.NoError(t, err)
	require.NotNil(t, status)

	statusMap, ok := status.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, statusMap["active"])
	assert.Equal(t, float64(3600), statusMap["uptime"]) // JSON unmarshals numbers as float64
}

// TestPluginLoader_ExecuteCLICommand tests CLI command execution
func TestPluginLoader_ExecuteCLICommand(t *testing.T) {
	provider := &mockProvider{
		executeCLIFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
			assert.Equal(t, "monitor stats", command)
			assert.Equal(t, []string{"--verbose"}, args)
			return []byte("CPU: 50%\nMemory: 2GB"), nil
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	output, err := loader.ExecuteCLICommand(context.Background(), "monitor stats", []string{"--verbose"})
	require.NoError(t, err)
	assert.Contains(t, string(output), "CPU: 50%")
	assert.Contains(t, string(output), "Memory: 2GB")
}

// TestCheckDependencies_NoDependents tests when no plugins depend on target
func TestCheckDependencies_NoDependents(t *testing.T) {
	registry := NewPluginRegistry()

	// Add plugin with no dependencies
	plugin1 := &mockPluginWithMetadata{
		metadata: plugins.PluginMetadata{
			Namespace:    "firewall",
			Dependencies: []string{},
		},
	}
	registry.Register(plugin1, "firewall")

	err := CheckDependencies("monitoring", registry)
	assert.NoError(t, err)
}

// TestCheckDependencies_HasDependents tests when plugins depend on target
func TestCheckDependencies_HasDependents(t *testing.T) {
	registry := NewPluginRegistry()

	// Plugin that depends on "monitoring"
	plugin1 := &mockPluginWithMetadata{
		metadata: plugins.PluginMetadata{
			Namespace:    "advanced-firewall",
			Dependencies: []string{"monitoring", "firewall"},
		},
	}
	registry.Register(plugin1, "advanced-firewall")

	// Another plugin that depends on "monitoring"
	plugin2 := &mockPluginWithMetadata{
		metadata: plugins.PluginMetadata{
			Namespace:    "alerts",
			Dependencies: []string{"monitoring"},
		},
	}
	registry.Register(plugin2, "alerts")

	err := CheckDependencies("monitoring", registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot disable plugin 'monitoring'")
	assert.Contains(t, err.Error(), "advanced-firewall")
	assert.Contains(t, err.Error(), "alerts")
}

// TestCheckDependencies_MultipleDependencies tests complex dependency graph
func TestCheckDependencies_MultipleDependencies(t *testing.T) {
	registry := NewPluginRegistry()

	// Plugin with multiple dependencies
	plugin1 := &mockPluginWithMetadata{
		metadata: plugins.PluginMetadata{
			Namespace:    "web-ui",
			Dependencies: []string{"monitoring", "firewall", "vpn"},
		},
	}
	registry.Register(plugin1, "web-ui")

	// Disabling "monitoring" should fail
	err := CheckDependencies("monitoring", registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web-ui")

	// Disabling "firewall" should fail
	err = CheckDependencies("firewall", registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web-ui")

	// Disabling "dhcp" (not a dependency) should succeed
	err = CheckDependencies("dhcp", registry)
	assert.NoError(t, err)
}

// TestCheckDependencies_EmptyRegistry tests with no plugins loaded
func TestCheckDependencies_EmptyRegistry(t *testing.T) {
	registry := NewPluginRegistry()

	err := CheckDependencies("any-plugin", registry)
	assert.NoError(t, err)
}

// TestPluginLoader_Close tests cleanup
func TestPluginLoader_Close(t *testing.T) {
	// Loader with nil client (no RPC connection)
	loader := &PluginLoader{
		client: nil,
	}

	err := loader.Close()
	assert.NoError(t, err)
}

// TestPluginLoader_ApplyConfig_MarshalError tests handling of invalid config
func TestPluginLoader_ApplyConfig_MarshalError(t *testing.T) {
	loader := &PluginLoader{
		provider: &mockProvider{},
	}

	// Channel cannot be marshaled to JSON
	invalidConfig := make(chan int)

	err := loader.ApplyConfig(invalidConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal config")
}

// TestPluginLoader_Status_UnmarshalError tests handling of invalid status JSON
func TestPluginLoader_Status_UnmarshalError(t *testing.T) {
	provider := &mockProvider{
		statusFunc: func(ctx context.Context) ([]byte, error) {
			// Return invalid JSON
			return []byte("not valid json {"), nil
		},
	}

	loader := &PluginLoader{
		provider: provider,
	}

	status, err := loader.Status()
	require.Error(t, err)
	assert.Nil(t, status)
	assert.Contains(t, err.Error(), "failed to unmarshal status")
}
