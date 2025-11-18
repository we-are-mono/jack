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

// Package plugins defines the plugin system for Jack using Hashicorp's go-plugin framework.
package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrInvalidConfig is returned when a plugin receives an invalid configuration type
var ErrInvalidConfig = errors.New("invalid configuration type")

// Plugin is the unified interface that all Jack plugins must implement.
// Plugins are self-describing and tell the core what they can do.
//
// DEPRECATED: This interface is deprecated. Plugins should implement the Provider
// interface directly instead of using the Plugin interface with PluginAdapter.
// The Plugin interface lacks support for CLI commands, log events, and context-based
// cancellation. All core plugins have been migrated to the modern Provider pattern.
type Plugin interface {
	// Metadata returns information about what this plugin provides
	Metadata() PluginMetadata

	// ApplyConfig applies the given configuration
	ApplyConfig(config interface{}) error

	// ValidateConfig validates the given configuration without applying it
	ValidateConfig(config interface{}) error

	// Flush removes all configuration managed by this plugin
	Flush() error

	// Status returns the current status of the plugin
	Status() (interface{}, error)

	// Close terminates the plugin (for RPC-based plugins)
	Close() error
}

// PluginMetadata describes what a plugin provides and how to interact with it.
// Plugins declare this information to the core, which uses it for routing and discovery.
type PluginMetadata struct {
	// Namespace is the plugin's domain (e.g., "firewall", "dhcp", "vpn")
	Namespace string `json:"namespace"`

	// Version is the plugin version
	Version string `json:"version"`

	// Description is a human-readable description
	Description string `json:"description"`

	// Category groups related plugins together (e.g., "firewall", "vpn", "dhcp", "hardware", "monitoring")
	Category string `json:"category,omitempty"`

	// ConfigPath is the path to the plugin's configuration file
	ConfigPath string `json:"config_path"`

	// DefaultConfig provides the default configuration for this plugin
	// This will be used to create the initial config file when the plugin is added
	DefaultConfig map[string]interface{} `json:"default_config,omitempty"`

	// Dependencies lists other plugin namespaces this plugin depends on
	Dependencies []string `json:"dependencies,omitempty"`

	// PathPrefix is automatically prepended to paths when accessing this plugin's config
	// Example: If PathPrefix="leds", then "led.status:green" becomes "led.leds.status:green"
	PathPrefix string `json:"path_prefix,omitempty"`
}

// PluginManager manages plugin discovery and loading.
// It searches for plugins in multiple directories and provides methods to load them.
type PluginManager struct {
	pluginDirs []string
}

// NewPluginManager creates a new plugin manager with default search directories.
// Search order: ./bin (dev), /usr/lib/jack/plugins (system), /opt/jack/plugins (alt).
func NewPluginManager() *PluginManager {
	return &PluginManager{
		pluginDirs: []string{
			"./bin",                 // Local development
			"/usr/lib/jack/plugins", // System installation
			"/opt/jack/plugins",     // Alternative installation
		},
	}
}

// FindPlugin searches for a plugin binary by name.
// The name is converted to the full plugin binary name (e.g., "nftables" -> "jack-plugin-nftables").
// Returns the full path to the plugin binary or an error if not found.
func (pm *PluginManager) FindPlugin(name string) (string, error) {
	pluginName := fmt.Sprintf("jack-plugin-%s", name)

	for _, dir := range pm.pluginDirs {
		pluginPath := filepath.Join(dir, pluginName)

		// Check if plugin exists and is executable
		if info, err := os.Stat(pluginPath); err == nil {
			if info.Mode().IsRegular() && (info.Mode().Perm()&0111) != 0 {
				return pluginPath, nil
			}
		}
	}

	return "", fmt.Errorf("plugin not found: %s", name)
}

// ListPlugins returns a list of all available plugin names (without the "jack-plugin-" prefix).
// It searches all configured plugin directories and returns unique plugin names.
func (pm *PluginManager) ListPlugins() ([]string, error) {
	plugins := make(map[string]bool)

	for _, dir := range pm.pluginDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Directory might not exist
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, "jack-plugin-") {
				// Check if executable
				fullPath := filepath.Join(dir, name)
				if info, err := os.Stat(fullPath); err == nil {
					if info.Mode().IsRegular() && (info.Mode().Perm()&0111) != 0 {
						// Extract plugin name (remove "jack-plugin-" prefix)
						pluginName := strings.TrimPrefix(name, "jack-plugin-")
						plugins[pluginName] = true
					}
				}
			}
		}
	}

	// Convert map to slice
	var result []string
	for name := range plugins {
		result = append(result, name)
	}

	return result, nil
}
