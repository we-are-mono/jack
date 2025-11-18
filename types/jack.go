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

// ObserverConfig represents configuration for the network observer
type ObserverConfig struct {
	Enabled             bool `json:"enabled"`               // Enable observer
	AutoReconcile       bool `json:"auto_reconcile"`        // Automatically fix drift
	ReconcileIntervalMS int  `json:"reconcile_interval_ms"` // Minimum time between reconciliations (default: 60000ms = 1 minute)
}

// LoggingConfig represents configuration for the logging system
type LoggingConfig struct {
	Level   string   `json:"level"`   // debug, info, warn, error (default: info)
	Format  string   `json:"format"`  // text, json (default: json)
	Outputs []string `json:"outputs"` // ["file", "journald"] (default: auto-detect)
	File    string   `json:"file"`    // Log file path (default: /var/log/jack/jack.log)
}

// JackConfig represents the main Jack configuration (/etc/jack/jack.json)
type JackConfig struct {
	Plugins  map[string]PluginState `json:"plugins"`  // Map of plugin name to state
	Observer *ObserverConfig        `json:"observer"` // Observer configuration (optional)
	Logging  *LoggingConfig         `json:"logging"`  // Logging configuration (optional)
	Version  string                 `json:"version"`
}
