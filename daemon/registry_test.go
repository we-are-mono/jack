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
	"github.com/we-are-mono/jack/plugins"
)

// mockPlugin implements the Plugin interface for testing
type mockPlugin struct {
	namespace string
}

func (m *mockPlugin) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:   m.namespace,
		Version:     "1.0.0",
		Description: "Mock plugin for testing",
		ConfigPath:  "/etc/jack/mock.json",
	}
}

func (m *mockPlugin) ApplyConfig(config interface{}) error {
	return nil
}

func (m *mockPlugin) ValidateConfig(config interface{}) error {
	return nil
}

func (m *mockPlugin) Flush() error {
	return nil
}

func (m *mockPlugin) Status() (interface{}, error) {
	return map[string]string{"status": "ok"}, nil
}

func (m *mockPlugin) Close() error {
	return nil
}

func TestPluginRegistry_Register(t *testing.T) {
	registry := NewPluginRegistry()

	// Register a mock plugin
	plugin := &mockPlugin{namespace: "test"}
	err := registry.Register(plugin, "test-plugin")
	require.NoError(t, err)

	// Verify plugin is registered
	retrieved, exists := registry.Get("test")
	assert.True(t, exists, "plugin should be registered")
	assert.NotNil(t, retrieved)
}

func TestPluginRegistry_DuplicateNamespace(t *testing.T) {
	registry := NewPluginRegistry()

	// Register first plugin
	plugin1 := &mockPlugin{namespace: "duplicate"}
	err := registry.Register(plugin1, "plugin1")
	require.NoError(t, err)

	// Try to register second plugin with same namespace
	plugin2 := &mockPlugin{namespace: "duplicate"}
	err = registry.Register(plugin2, "plugin2")
	assert.Error(t, err, "should error on duplicate namespace")
}

func TestPluginRegistry_GetNonExistent(t *testing.T) {
	registry := NewPluginRegistry()

	// Try to get plugin that doesn't exist
	plugin, exists := registry.Get("nonexistent")
	assert.False(t, exists, "should return false for non-existent plugin")
	assert.Nil(t, plugin)
}

func TestPluginRegistry_List(t *testing.T) {
	registry := NewPluginRegistry()

	// Initially empty
	list := registry.List()
	assert.Empty(t, list)

	// Register multiple plugins
	registry.Register(&mockPlugin{namespace: "plugin1"}, "plugin1")
	registry.Register(&mockPlugin{namespace: "plugin2"}, "plugin2")
	registry.Register(&mockPlugin{namespace: "plugin3"}, "plugin3")

	// List should contain all namespaces
	list = registry.List()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "plugin1")
	assert.Contains(t, list, "plugin2")
	assert.Contains(t, list, "plugin3")
}

func TestPluginRegistry_CloseAll(t *testing.T) {
	registry := NewPluginRegistry()

	// Register some plugins
	registry.Register(&mockPlugin{namespace: "plugin1"}, "plugin1")
	registry.Register(&mockPlugin{namespace: "plugin2"}, "plugin2")

	// Close all should not panic
	assert.NotPanics(t, func() {
		registry.CloseAll()
	})

	// Registry should still work after close
	list := registry.List()
	assert.Len(t, list, 2)
}

func TestPluginRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewPluginRegistry()

	// Register initial plugin
	registry.Register(&mockPlugin{namespace: "initial"}, "initial")

	// Concurrent reads should be safe
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, exists := registry.Get("initial")
			assert.True(t, exists)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPluginRegistry_Unregister(t *testing.T) {
	registry := NewPluginRegistry()

	// Register a plugin
	plugin := &mockPlugin{namespace: "test"}
	err := registry.Register(plugin, "test-plugin")
	require.NoError(t, err)

	// Verify it exists
	_, exists := registry.Get("test")
	assert.True(t, exists)

	// Unregister it
	registry.Unregister("test")

	// Verify it's gone
	_, exists = registry.Get("test")
	assert.False(t, exists)

	// Unregister non-existent plugin should not panic
	assert.NotPanics(t, func() {
		registry.Unregister("nonexistent")
	})
}

func TestPluginRegistry_IsRegistered(t *testing.T) {
	registry := NewPluginRegistry()

	// Check non-registered plugin
	assert.False(t, registry.IsRegistered("test"))

	// Register a plugin
	plugin := &mockPlugin{namespace: "test"}
	err := registry.Register(plugin, "test-plugin")
	require.NoError(t, err)

	// Check registered plugin
	assert.True(t, registry.IsRegistered("test"))

	// Unregister and check again
	registry.Unregister("test")
	assert.False(t, registry.IsRegistered("test"))
}

func TestPluginRegistry_GetNamespaceForPlugin(t *testing.T) {
	registry := NewPluginRegistry()

	// Register a plugin
	plugin := &mockPlugin{namespace: "firewall"}
	err := registry.Register(plugin, "jack-plugin-firewall")
	require.NoError(t, err)

	// Get namespace for plugin name
	namespace, exists := registry.GetNamespaceForPlugin("jack-plugin-firewall")
	assert.True(t, exists)
	assert.Equal(t, "firewall", namespace)

	// Get namespace for non-existent plugin
	namespace, exists = registry.GetNamespaceForPlugin("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, namespace)
}

func TestPluginRegistry_GetPluginNameForNamespace(t *testing.T) {
	registry := NewPluginRegistry()

	// Register a plugin
	plugin := &mockPlugin{namespace: "firewall"}
	err := registry.Register(plugin, "jack-plugin-firewall")
	require.NoError(t, err)

	// Get plugin name for namespace
	pluginName, exists := registry.GetPluginNameForNamespace("firewall")
	assert.True(t, exists)
	assert.Equal(t, "jack-plugin-firewall", pluginName)

	// Get plugin name for non-existent namespace
	pluginName, exists = registry.GetPluginNameForNamespace("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, pluginName)
}

func TestPluginRegistry_GetAll(t *testing.T) {
	registry := NewPluginRegistry()

	// Initially empty
	all := registry.GetAll()
	assert.Empty(t, all)

	// Register multiple plugins
	registry.Register(&mockPlugin{namespace: "firewall"}, "firewall")
	registry.Register(&mockPlugin{namespace: "dhcp"}, "dnsmasq")
	registry.Register(&mockPlugin{namespace: "vpn"}, "wireguard")

	// GetAll should return all plugins
	all = registry.GetAll()
	assert.Len(t, all, 3)

	// Verify each plugin is present
	_, hasFirewall := all["firewall"]
	_, hasDHCP := all["dhcp"]
	_, hasVPN := all["vpn"]
	assert.True(t, hasFirewall)
	assert.True(t, hasDHCP)
	assert.True(t, hasVPN)
}
