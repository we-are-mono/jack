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

// Package main implements a self-contained nftables firewall plugin for Jack.
package main

import (
	"encoding/json"

	"github.com/we-are-mono/jack/plugins"
)

// PluginAdapter adapts the nftables provider to the unified Plugin interface.
// This makes the plugin self-describing and self-contained.
type PluginAdapter struct {
	provider *NftablesProvider
}

// NewPluginAdapter creates a new self-contained plugin
func NewPluginAdapter() (*PluginAdapter, error) {
	provider, err := NewNftables()
	if err != nil {
		return nil, err
	}

	return &PluginAdapter{
		provider: provider,
	}, nil
}

// Metadata returns information about what this plugin provides
func (p *PluginAdapter) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:   "firewall",
		Version:     "1.0.0",
		Description: "nftables-based firewall management",
		Category:    "firewall",
		ConfigPath:  "/etc/jack/firewall.json",
	}
}

// ApplyConfig applies firewall configuration
func (p *PluginAdapter) ApplyConfig(config interface{}) error {
	// Config comes in as a generic map from JSON unmarshaling
	// We need to re-marshal and unmarshal to get the right type
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var firewallConfig FirewallConfig
	if err := json.Unmarshal(configJSON, &firewallConfig); err != nil {
		return err
	}

	// Skip if firewall is disabled
	if !firewallConfig.Enabled {
		return nil
	}

	return p.provider.ApplyConfig(&firewallConfig)
}

// ValidateConfig validates firewall configuration
func (p *PluginAdapter) ValidateConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var firewallConfig FirewallConfig
	if err := json.Unmarshal(configJSON, &firewallConfig); err != nil {
		return err
	}

	return p.provider.Validate(&firewallConfig)
}

// Flush removes all firewall rules
func (p *PluginAdapter) Flush() error {
	return p.provider.Flush()
}

// Status returns the current firewall status
func (p *PluginAdapter) Status() (interface{}, error) {
	status, err := p.provider.Status()
	if err != nil {
		return nil, err
	}

	// Return as map for consistent JSON marshaling
	return map[string]interface{}{
		"enabled":    status.Enabled,
		"backend":    status.Backend,
		"rule_count": status.RuleCount,
	}, nil
}

// Close terminates the plugin (no-op for direct plugins)
func (p *PluginAdapter) Close() error {
	return nil
}
