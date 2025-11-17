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

// Package main implements a self-contained LED control plugin for Jack.
package main

import (
	"encoding/json"

	"github.com/we-are-mono/jack/plugins"
)

// LEDAdapter adapts the LED provider to the unified Plugin interface.
type LEDAdapter struct {
	provider *LEDProvider
}

// NewLEDAdapter creates a new LED plugin adapter
func NewLEDAdapter() *LEDAdapter {
	provider := NewLEDProvider()

	return &LEDAdapter{
		provider: provider,
	}
}

// Metadata returns information about what this plugin provides
func (p *LEDAdapter) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:     "led",
		Version:       "1.0.0",
		Description:   "System LED control via sysfs",
		Category:      "hardware",
		ConfigPath:    "/etc/jack/led.json",
		DefaultConfig: nil,    // Empty config - LEDs are configured via /etc/jack/led.json
		PathPrefix:    "leds", // Auto-insert "leds" in paths: led.status:green -> led.leds.status:green
	}
}

// ApplyConfig applies LED configuration
func (p *LEDAdapter) ApplyConfig(config interface{}) error {
	// Config comes in as a generic map from JSON unmarshaling
	// We need to re-marshal and unmarshal to get the right type
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var ledConfig LEDConfig
	if err := json.Unmarshal(configJSON, &ledConfig); err != nil {
		return err
	}

	return p.provider.ApplyConfig(&ledConfig)
}

// ValidateConfig validates LED configuration
func (p *LEDAdapter) ValidateConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var ledConfig LEDConfig
	if err := json.Unmarshal(configJSON, &ledConfig); err != nil {
		return err
	}

	return p.provider.ValidateConfig(&ledConfig)
}

// Flush removes all LED configuration
func (p *LEDAdapter) Flush() error {
	return p.provider.Flush()
}

// Status returns the current LED status
func (p *LEDAdapter) Status() (interface{}, error) {
	return p.provider.Status()
}

// Close terminates the plugin
func (p *LEDAdapter) Close() error {
	// No cleanup needed for LED plugin
	return nil
}
