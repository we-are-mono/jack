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

// Package state manages configuration state and persistence for Jack.
package state

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/we-are-mono/jack/types"
)

// LoadJackConfig loads the jack.json configuration from disk.
// If the file doesn't exist, it returns a default configuration.
func LoadJackConfig() (*types.JackConfig, error) {
	// Get the config path (respects JACK_CONFIG_DIR env var)
	jackConfigPath := filepath.Join(GetConfigDir(), "jack.json")

	// Check if file exists
	if _, err := os.Stat(jackConfigPath); os.IsNotExist(err) {
		return getDefaultJackConfig(), nil
	}

	// Load configuration
	var config types.JackConfig
	err := LoadConfig("jack", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to load jack config: %w", err)
	}

	// If plugins is nil, initialize with defaults
	if config.Plugins == nil {
		return getDefaultJackConfig(), nil
	}

	return &config, nil
}

// SaveJackConfig saves the jack.json configuration to disk.
func SaveJackConfig(config *types.JackConfig) error {
	return SaveConfig("jack", config)
}

// getDefaultJackConfig returns the default jack configuration with common plugins enabled.
func getDefaultJackConfig() *types.JackConfig {
	return &types.JackConfig{
		Version: "1.0",
		Plugins: map[string]types.PluginState{
			"nftables": {
				Enabled: true,
				Version: "", // Will be filled in on load
			},
			"dnsmasq": {
				Enabled: true,
				Version: "",
			},
			"monitoring": {
				Enabled: true,
				Version: "",
			},
		},
	}
}
