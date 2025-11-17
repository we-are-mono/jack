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
	"fmt"
	"log"
	"sync"

	"github.com/we-are-mono/jack/plugins"
)

// PluginRegistry manages plugin discovery, registration, and lifecycle.
// It provides namespace-based routing to registered plugins.
type PluginRegistry struct {
	plugins      map[string]plugins.Plugin // namespace -> plugin
	nameToNS     map[string]string         // plugin name -> namespace
	pathPrefixes map[string]string         // namespace -> path prefix
	mu           sync.RWMutex
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins:      make(map[string]plugins.Plugin),
		nameToNS:     make(map[string]string),
		pathPrefixes: make(map[string]string),
	}
}

// Register adds a plugin to the registry.
// The plugin's namespace is obtained from its metadata.
// The pluginName parameter is the plugin's binary name (e.g., "wireguard" for jack-plugin-wireguard).
func (r *PluginRegistry) Register(plugin plugins.Plugin, pluginName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata := plugin.Metadata()
	if _, exists := r.plugins[metadata.Namespace]; exists {
		return fmt.Errorf("plugin namespace '%s' already registered", metadata.Namespace)
	}

	r.plugins[metadata.Namespace] = plugin
	r.nameToNS[pluginName] = metadata.Namespace
	if metadata.PathPrefix != "" {
		r.pathPrefixes[metadata.Namespace] = metadata.PathPrefix
	}
	log.Printf("[REGISTRY] Registered plugin: %s (v%s) - %s",
		metadata.Namespace, metadata.Version, metadata.Description)
	return nil
}

// Get retrieves a plugin by namespace
func (r *PluginRegistry) Get(namespace string) (plugins.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[namespace]
	return plugin, exists
}

// List returns all registered plugin namespaces
func (r *PluginRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// ListByCategory returns all registered plugin namespaces grouped by category
func (r *PluginRegistry) ListByCategory() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categories := make(map[string][]string)
	for namespace, plugin := range r.plugins {
		metadata := plugin.Metadata()
		category := metadata.Category
		if category == "" {
			category = "other" // Default category for plugins without one
		}
		categories[category] = append(categories[category], namespace)
	}
	return categories
}

// Unregister removes a plugin from the registry
func (r *PluginRegistry) Unregister(namespace string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from plugins map
	delete(r.plugins, namespace)

	// Remove from nameToNS map (find and delete the reverse mapping)
	for name, ns := range r.nameToNS {
		if ns == namespace {
			delete(r.nameToNS, name)
			break
		}
	}

	log.Printf("[REGISTRY] Unregistered plugin: %s", namespace)
}

// GetPathPrefix returns the path prefix for a given namespace
func (r *PluginRegistry) GetPathPrefix(namespace string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.pathPrefixes[namespace]
}

// GetNamespaceForPlugin returns the namespace for a given plugin name
func (r *PluginRegistry) GetNamespaceForPlugin(pluginName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	namespace, exists := r.nameToNS[pluginName]
	return namespace, exists
}

// GetPluginNameForNamespace returns the plugin name for a given namespace
func (r *PluginRegistry) GetPluginNameForNamespace(namespace string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Reverse lookup: find plugin name by namespace
	for name, ns := range r.nameToNS {
		if ns == namespace {
			return name, true
		}
	}
	return "", false
}

// GetAll returns a map of all registered plugins
func (r *PluginRegistry) GetAll() map[string]plugins.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]plugins.Plugin)
	for k, v := range r.plugins {
		result[k] = v
	}
	return result
}

// IsRegistered checks if a plugin is registered
func (r *PluginRegistry) IsRegistered(namespace string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.plugins[namespace]
	return exists
}

// CloseAll terminates all registered plugins
func (r *PluginRegistry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, plugin := range r.plugins {
		if err := plugin.Close(); err != nil {
			log.Printf("[REGISTRY] Failed to close plugin '%s': %v", name, err)
		}
	}
}
