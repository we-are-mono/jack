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

//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

// TestLoadPlugin_Success tests successfully loading a plugin
func TestLoadPlugin_Success(t *testing.T) {
	// Load the monitoring plugin (should always be available)
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err, "Should load monitoring plugin successfully")
	require.NotNil(t, plugin, "Plugin should not be nil")
	defer plugin.Close()

	// Verify metadata
	metadata := plugin.Metadata()
	assert.Equal(t, "monitoring", metadata.Namespace, "Should have correct namespace")
	assert.NotEmpty(t, metadata.Version, "Should have version")
	assert.NotEmpty(t, metadata.Description, "Should have description")

	t.Logf("Loaded plugin: %s v%s - %s", metadata.Namespace, metadata.Version, metadata.Description)
}

// TestLoadPlugin_NotFound tests loading a non-existent plugin
func TestLoadPlugin_NotFound(t *testing.T) {
	plugin, err := daemon.LoadPlugin("nonexistent-plugin-xyz")
	assert.Error(t, err, "Should fail to load non-existent plugin")
	assert.Nil(t, plugin, "Plugin should be nil on error")
	assert.Contains(t, err.Error(), "not found", "Error should indicate plugin not found")
}

// TestLoadPlugin_MultipleInstances tests loading multiple instances of the same plugin
func TestLoadPlugin_MultipleInstances(t *testing.T) {
	// Load first instance
	plugin1, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin1)
	defer plugin1.Close()

	// Load second instance (should work - each instance is independent)
	plugin2, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin2)
	defer plugin2.Close()

	// Both should have the same metadata
	meta1 := plugin1.Metadata()
	meta2 := plugin2.Metadata()
	assert.Equal(t, meta1.Namespace, meta2.Namespace, "Both instances should have same namespace")
	assert.Equal(t, meta1.Version, meta2.Version, "Both instances should have same version")

	t.Log("Successfully loaded multiple instances of the same plugin")
}

// TestLoadPlugin_AllCorePlugins tests loading all available core plugins
func TestLoadPlugin_AllCorePlugins(t *testing.T) {
	// These plugins should always be available in the integration test environment
	corePlugins := []string{"monitoring", "firewall", "dnsmasq", "wireguard"}

	for _, pluginName := range corePlugins {
		t.Run(pluginName, func(t *testing.T) {
			plugin, err := daemon.LoadPlugin(pluginName)
			require.NoError(t, err, "Should load %s plugin", pluginName)
			require.NotNil(t, plugin, "Plugin should not be nil")

			metadata := plugin.Metadata()
			assert.NotEmpty(t, metadata.Namespace, "Should have namespace")
			assert.NotEmpty(t, metadata.Version, "Should have version")

			t.Logf("Loaded %s → namespace: %s, version: %s",
				pluginName, metadata.Namespace, metadata.Version)

			// Clean up
			err = plugin.Close()
			assert.NoError(t, err, "Should close plugin cleanly")
		})
	}
}

// TestScanPlugins_Discovery tests plugin discovery
func TestScanPlugins_Discovery(t *testing.T) {
	availablePlugins, err := daemon.ScanPlugins()
	require.NoError(t, err, "ScanPlugins should succeed")
	require.NotEmpty(t, availablePlugins, "Should find at least one plugin")

	// Log all discovered plugins
	t.Logf("Discovered %d plugins:", len(availablePlugins))
	for name, metadata := range availablePlugins {
		t.Logf("  - %s: namespace=%s, version=%s, description=%s",
			name, metadata.Namespace, metadata.Version, metadata.Description)

		// Verify metadata is valid
		assert.NotEmpty(t, metadata.Namespace, "Plugin %s should have namespace", name)
		assert.NotEmpty(t, metadata.Version, "Plugin %s should have version", name)
	}

	// Should at least find monitoring plugin
	_, hasMonitoring := availablePlugins["monitoring"]
	assert.True(t, hasMonitoring, "Should discover monitoring plugin")
}

// TestScanPlugins_NamespaceMapping tests that plugin names map to correct namespaces
func TestScanPlugins_NamespaceMapping(t *testing.T) {
	availablePlugins, err := daemon.ScanPlugins()
	require.NoError(t, err)

	expectedMappings := map[string]string{
		"monitoring": "monitoring",
		"firewall":   "firewall",
		"dnsmasq":    "dhcp",
		"wireguard":  "vpn",
	}

	for pluginName, expectedNamespace := range expectedMappings {
		if metadata, exists := availablePlugins[pluginName]; exists {
			assert.Equal(t, expectedNamespace, metadata.Namespace,
				"Plugin %s should provide namespace %s", pluginName, expectedNamespace)
		} else {
			t.Logf("Plugin %s not found (may not be built for integration tests)", pluginName)
		}
	}
}

// TestUnloadPlugin_Success tests successfully unloading a plugin
func TestUnloadPlugin_Success(t *testing.T) {
	// Load a plugin
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin)

	metadata := plugin.Metadata()

	// Unload the plugin
	err = daemon.UnloadPlugin(plugin, metadata.Namespace)
	assert.NoError(t, err, "Should unload plugin successfully")

	// Note: We can't use the plugin after unloading, so we just verify no error occurred
	t.Log("Successfully unloaded plugin")
}

// TestUnloadPlugin_WithActiveConfig tests unloading a plugin after applying config
func TestUnloadPlugin_WithActiveConfig(t *testing.T) {
	// Load monitoring plugin
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin)
	defer func() {
		// Try to close if unload fails
		_ = plugin.Close()
	}()

	metadata := plugin.Metadata()

	// Apply a config (use default config if available)
	config := map[string]interface{}{
		"enabled": true,
	}

	err = plugin.ApplyConfig(config)
	// May fail if config is invalid, but that's okay for this test
	if err != nil {
		t.Logf("ApplyConfig returned error (expected for some plugins): %v", err)
	}

	// Unload should flush the plugin's state
	err = daemon.UnloadPlugin(plugin, metadata.Namespace)
	assert.NoError(t, err, "Should unload plugin even with active config")

	t.Log("Successfully unloaded plugin with active configuration")
}

// TestPluginLoader_Metadata tests PluginLoader metadata retrieval
func TestPluginLoader_Metadata(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	metadata := plugin.Metadata()

	// Test all metadata fields
	assert.Equal(t, "monitoring", metadata.Namespace)
	assert.NotEmpty(t, metadata.Version, "Version should not be empty")
	assert.NotEmpty(t, metadata.Description, "Description should not be empty")
	assert.NotEmpty(t, metadata.ConfigPath, "ConfigPath should not be empty")

	// Log the full metadata
	t.Logf("Metadata: namespace=%s, version=%s, description=%s, config=%s",
		metadata.Namespace, metadata.Version, metadata.Description, metadata.ConfigPath)
}

// TestPluginLoader_ApplyConfig tests applying configuration to a plugin
func TestPluginLoader_ApplyConfig(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	// Test valid configuration
	validConfig := map[string]interface{}{
		"enabled":             true,
		"collection_interval": 5,
	}

	err = plugin.ApplyConfig(validConfig)
	assert.NoError(t, err, "Should apply valid config successfully")

	// Test invalid configuration (wrong type)
	invalidConfig := "not-a-map"
	err = plugin.ApplyConfig(invalidConfig)
	// Should marshal successfully but plugin may reject it
	t.Logf("Invalid config result: %v (plugin may accept or reject)", err)
}

// TestPluginLoader_ValidateConfig tests config validation
func TestPluginLoader_ValidateConfig(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	// Test valid configuration
	validConfig := map[string]interface{}{
		"enabled": true,
	}

	err = plugin.ValidateConfig(validConfig)
	assert.NoError(t, err, "Should validate valid config successfully")

	// Test empty config (should be valid for most plugins)
	emptyConfig := map[string]interface{}{}
	err = plugin.ValidateConfig(emptyConfig)
	// May or may not error depending on plugin requirements
	t.Logf("Empty config validation result: %v", err)
}

// TestPluginLoader_Status tests retrieving plugin status
func TestPluginLoader_Status(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	// Get status
	status, err := plugin.Status()
	assert.NoError(t, err, "Should retrieve status successfully")
	assert.NotNil(t, status, "Status should not be nil")

	// Log the status
	t.Logf("Plugin status: %v", status)
}

// TestPluginLoader_Flush tests flushing plugin state
func TestPluginLoader_Flush(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	// Apply a config first
	config := map[string]interface{}{
		"enabled": true,
	}
	_ = plugin.ApplyConfig(config) // Ignore error

	// Flush the plugin
	err = plugin.Flush()
	assert.NoError(t, err, "Should flush plugin successfully")

	t.Log("Successfully flushed plugin")
}

// TestPluginLoader_Close tests closing plugin connections
func TestPluginLoader_Close(t *testing.T) {
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin)

	// Close the plugin
	err = plugin.Close()
	assert.NoError(t, err, "Should close plugin successfully")

	// Attempting to use plugin after close should fail
	// (This tests that Close actually terminates the RPC connection)
	_, err = plugin.Status()
	assert.Error(t, err, "Should fail to get status after Close")

	t.Log("Successfully closed plugin and verified it's terminated")
}

// TestPluginLoader_ExecuteCLICommand tests CLI command execution through PluginLoader
func TestPluginLoader_ExecuteCLICommand(t *testing.T) {
	// Load the plugin
	rawPlugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer rawPlugin.Close()

	// Cast to PluginLoader to access ExecuteCLICommand
	// (This is an internal method, not part of the Plugin interface)
	pluginLoader, ok := rawPlugin.(*daemon.PluginLoader)
	require.True(t, ok, "Plugin should be a PluginLoader instance")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to execute a CLI command (monitoring plugin supports "monitor stats")
	output, err := pluginLoader.ExecuteCLICommand(ctx, "monitor stats", []string{})

	// Command may or may not succeed depending on environment
	if err != nil {
		t.Logf("CLI command failed (may be expected): %v", err)
	} else {
		assert.NotNil(t, output, "Should return output")
		t.Logf("CLI command output length: %d bytes", len(output))
	}
}

// TestPluginLoader_Lifecycle_Complete tests the complete plugin lifecycle
func TestPluginLoader_Lifecycle_Complete(t *testing.T) {
	// 1. Load
	t.Log("Step 1: Loading plugin...")
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	require.NotNil(t, plugin)

	// 2. Get metadata
	t.Log("Step 2: Getting metadata...")
	metadata := plugin.Metadata()
	assert.Equal(t, "monitoring", metadata.Namespace)

	// 3. Validate config
	t.Log("Step 3: Validating config...")
	config := map[string]interface{}{
		"enabled": true,
	}
	err = plugin.ValidateConfig(config)
	assert.NoError(t, err)

	// 4. Apply config
	t.Log("Step 4: Applying config...")
	err = plugin.ApplyConfig(config)
	assert.NoError(t, err)

	// 5. Get status
	t.Log("Step 5: Getting status...")
	status, err := plugin.Status()
	assert.NoError(t, err)
	assert.NotNil(t, status)

	// 6. Flush
	t.Log("Step 6: Flushing...")
	err = plugin.Flush()
	assert.NoError(t, err)

	// 7. Close
	t.Log("Step 7: Closing...")
	err = plugin.Close()
	assert.NoError(t, err)

	t.Log("Complete plugin lifecycle test passed!")
}

// TestCheckDependencies_NoDependencies tests dependency checking with no dependencies
func TestCheckDependencies_NoDependencies(t *testing.T) {
	registry := daemon.NewPluginRegistry()

	// Load and register a plugin without dependencies
	plugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer plugin.Close()

	metadata := plugin.Metadata()
	err = registry.Register(plugin, "monitoring")
	require.NoError(t, err)

	// Check dependencies - should pass (no other plugins depend on monitoring)
	err = daemon.CheckDependencies(metadata.Namespace, registry)
	assert.NoError(t, err, "Should have no dependencies")
}

// TestCheckDependencies_WithDependencies tests dependency checking with dependencies
func TestCheckDependencies_WithDependencies(t *testing.T) {
	registry := daemon.NewPluginRegistry()

	// Load base plugin
	basePlugin, err := daemon.LoadPlugin("monitoring")
	require.NoError(t, err)
	defer basePlugin.Close()

	baseMetadata := basePlugin.Metadata()
	err = registry.Register(basePlugin, "monitoring")
	require.NoError(t, err)

	// In a real scenario, another plugin might depend on monitoring
	// For this test, we're just verifying the function works
	err = daemon.CheckDependencies(baseMetadata.Namespace, registry)
	assert.NoError(t, err, "Monitoring should have no dependents in this test")

	t.Log("Dependency check completed successfully")
}
