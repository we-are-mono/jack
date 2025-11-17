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

// Package daemon implements the Jack daemon server and protocol.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/we-are-mono/jack/plugins"
)

// PluginLoader wraps an RPC plugin and implements the Plugin interface.
// It queries the plugin for its metadata and proxies all calls.
type PluginLoader struct {
	client   *plugins.PluginClient
	provider plugins.Provider
	metadata plugins.PluginMetadata
}

// LoadPlugin discovers, loads, and wraps a plugin by name.
// The plugin tells us what namespace it handles via its metadata.
func LoadPlugin(name string) (plugins.Plugin, error) {
	// Find the plugin binary
	pm := plugins.NewPluginManager()
	pluginPath, err := pm.FindPlugin(name)
	if err != nil {
		return nil, fmt.Errorf("plugin '%s' not found: %w", name, err)
	}

	log.Printf("[LOADER] Found plugin '%s' at: %s", name, pluginPath)

	// Start the plugin
	client, err := plugins.NewPluginClient(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start plugin '%s': %w", name, err)
	}

	// Dispense the provider
	provider, err := client.Dispense()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to dispense plugin '%s': %w", name, err)
	}

	// Query the plugin for its metadata
	metaResp, err := provider.Metadata(context.TODO())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to get metadata from plugin '%s': %w", name, err)
	}

	log.Printf("[LOADER] Plugin '%s' provides namespace: %s (v%s)",
		name, metaResp.Namespace, metaResp.Version)

	// Convert metadata response to PluginMetadata
	metadata := plugins.PluginMetadata{
		Namespace:     metaResp.Namespace,
		Version:       metaResp.Version,
		Description:   metaResp.Description,
		Category:      metaResp.Category,
		ConfigPath:    metaResp.ConfigPath,
		DefaultConfig: metaResp.DefaultConfig,
		Dependencies:  metaResp.Dependencies,
		PathPrefix:    metaResp.PathPrefix,
	}

	return &PluginLoader{
		client:   client,
		provider: provider,
		metadata: metadata,
	}, nil
}

// Metadata returns the plugin's self-declared metadata
func (l *PluginLoader) Metadata() plugins.PluginMetadata {
	return l.metadata
}

// ApplyConfig proxies the config application to the underlying provider
func (l *PluginLoader) ApplyConfig(config interface{}) error {
	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Debug logging
	log.Printf("[LOADER] Sending config to plugin %s: %s", l.metadata.Namespace, string(configJSON))

	// Send to plugin via RPC
	return l.provider.ApplyConfig(context.TODO(), configJSON)
}

// ValidateConfig proxies config validation to the underlying provider
func (l *PluginLoader) ValidateConfig(config interface{}) error {
	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Send to plugin via RPC
	return l.provider.ValidateConfig(context.TODO(), configJSON)
}

// Flush proxies flush operation to the underlying provider
func (l *PluginLoader) Flush() error {
	return l.provider.Flush(context.TODO())
}

// Status proxies status query to the underlying provider
func (l *PluginLoader) Status() (interface{}, error) {
	// Get status as JSON from plugin
	statusJSON, err := l.provider.Status(context.TODO())
	if err != nil {
		return nil, err
	}

	// Deserialize to generic map
	var status map[string]interface{}
	if err := json.Unmarshal(statusJSON, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status: %w", err)
	}

	return status, nil
}

// Close terminates the plugin RPC connection
func (l *PluginLoader) Close() error {
	if l.client != nil {
		return l.client.Close()
	}
	return nil
}

// ExecuteCLICommand proxies CLI command execution to the underlying provider
func (l *PluginLoader) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	return l.provider.ExecuteCLICommand(ctx, command, args)
}

// ScanPlugins discovers all available plugins in plugin directories
// Returns a map of plugin name → metadata (without loading into registry)
func ScanPlugins() (map[string]plugins.PluginMetadata, error) {
	pm := plugins.NewPluginManager()
	availablePlugins, err := pm.ListPlugins()
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}

	result := make(map[string]plugins.PluginMetadata)

	for _, pluginName := range availablePlugins {
		// Temporarily load plugin to get metadata
		plugin, err := LoadPlugin(pluginName)
		if err != nil {
			log.Printf("[WARN] Failed to load plugin '%s' for scanning: %v", pluginName, err)
			continue
		}

		metadata := plugin.Metadata()
		result[pluginName] = metadata

		// Close the temp instance
		if err := plugin.Close(); err != nil {
			log.Printf("[WARN] Failed to close temp plugin '%s': %v", pluginName, err)
		}
	}

	return result, nil
}

// UnloadPlugin flushes and closes a plugin
// This should be called before removing it from the registry
func UnloadPlugin(plugin plugins.Plugin, namespace string) error {
	log.Printf("[LOADER] Unloading plugin: %s", namespace)

	// Flush system state
	if err := plugin.Flush(); err != nil {
		return fmt.Errorf("failed to flush plugin '%s': %w", namespace, err)
	}

	// Close RPC connection
	if err := plugin.Close(); err != nil {
		return fmt.Errorf("failed to close plugin '%s': %w", namespace, err)
	}

	log.Printf("[LOADER] Successfully unloaded plugin: %s", namespace)
	return nil
}

// CheckDependencies checks if any enabled plugins depend on the given plugin
// Returns an error if dependencies exist
func CheckDependencies(pluginToDisable string, registry *PluginRegistry) error {
	allPlugins := registry.GetAll()

	var dependents []string
	for name, plugin := range allPlugins {
		metadata := plugin.Metadata()
		for _, dep := range metadata.Dependencies {
			if dep == pluginToDisable {
				dependents = append(dependents, name)
			}
		}
	}

	if len(dependents) > 0 {
		return fmt.Errorf("cannot disable plugin '%s': it is required by: %v", pluginToDisable, dependents)
	}

	return nil
}
