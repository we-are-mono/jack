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

// Package main implements a self-contained monitoring plugin for Jack.
package main

import (
	"encoding/json"
	"log"

	"github.com/we-are-mono/jack/plugins"
)

// MonitoringAdapter adapts the monitoring provider to the unified Plugin interface.
type MonitoringAdapter struct {
	provider *MonitoringProvider
}

// NewMonitoringAdapter creates a new monitoring plugin adapter
func NewMonitoringAdapter() *MonitoringAdapter {
	provider := NewMonitoringProvider()

	return &MonitoringAdapter{
		provider: provider,
	}
}

// Metadata returns information about what this plugin provides
func (p *MonitoringAdapter) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:   "monitoring",
		Version:     "1.0.0",
		Description: "System and network metrics collection",
		Category:    "monitoring",
		ConfigPath:  "/etc/jack/monitoring.json",
		DefaultConfig: map[string]interface{}{
			"enabled":             true,
			"collection_interval": 5,
		},
	}
}

// ApplyConfig applies monitoring configuration
func (p *MonitoringAdapter) ApplyConfig(config interface{}) error {
	// Config comes in as a generic map from JSON unmarshaling
	// We need to re-marshal and unmarshal to get the right type
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	log.Printf("[ADAPTER] Received config JSON: %s", string(configJSON))

	var monitoringConfig MonitoringConfig
	if err := json.Unmarshal(configJSON, &monitoringConfig); err != nil {
		return err
	}

	log.Printf("[ADAPTER] Parsed config: %+v", monitoringConfig)

	return p.provider.ApplyConfig(&monitoringConfig)
}

// ValidateConfig validates monitoring configuration
func (p *MonitoringAdapter) ValidateConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var monitoringConfig MonitoringConfig
	if err := json.Unmarshal(configJSON, &monitoringConfig); err != nil {
		return err
	}

	return p.provider.Validate(&monitoringConfig)
}

// Flush stops monitoring collection
func (p *MonitoringAdapter) Flush() error {
	return p.provider.Stop()
}

// Status returns the current monitoring status and metrics
func (p *MonitoringAdapter) Status() (interface{}, error) {
	return p.provider.Status()
}

// Close terminates the plugin
func (p *MonitoringAdapter) Close() error {
	return p.provider.Stop()
}
