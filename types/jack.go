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

// Package types defines the core data structures for Jack's configuration.
package types

// PluginState represents the state of a plugin
type PluginState struct {
	Version string `json:"version"` // Plugin version from metadata
	Enabled bool   `json:"enabled"` // Whether the plugin is enabled
}

// JackConfig represents the main Jack configuration (/etc/jack/jack.json)
type JackConfig struct {
	Plugins map[string]PluginState `json:"plugins"` // Map of plugin name to state
	Version string                 `json:"version"`
}
