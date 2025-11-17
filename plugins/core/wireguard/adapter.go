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

// Package main implements a self-contained WireGuard VPN plugin for Jack.
package main

import (
	"encoding/json"

	"github.com/we-are-mono/jack/plugins"
)

// VPNAdapter adapts the WireGuard provider to the unified Plugin interface.
type VPNAdapter struct {
	provider *WireGuardProvider
}

// NewVPNAdapter creates a new VPN plugin adapter
func NewVPNAdapter() (*VPNAdapter, error) {
	provider, err := New()
	if err != nil {
		return nil, err
	}

	return &VPNAdapter{
		provider: provider,
	}, nil
}

// Metadata returns information about what this plugin provides
func (p *VPNAdapter) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:   "vpn",
		Version:     "1.0.0",
		Description: "WireGuard-based VPN management",
		Category:    "vpn",
		ConfigPath:  "/etc/jack/vpn.json",
	}
}

// ApplyConfig applies VPN configuration
func (p *VPNAdapter) ApplyConfig(config interface{}) error {
	// Config comes in as a generic map from JSON unmarshaling
	// We need to re-marshal and unmarshal to get the right type
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	// Debug: log the raw config
	println("[WIREGUARD-ADAPTER] Received config JSON:", string(configJSON))

	var vpnConfig VPNConfig
	if err := json.Unmarshal(configJSON, &vpnConfig); err != nil {
		println("[WIREGUARD-ADAPTER] Failed to unmarshal config:", err.Error())
		return err
	}

	println("[WIREGUARD-ADAPTER] Unmarshaled config - interfaces count:", len(vpnConfig.Interfaces))
	return p.provider.ApplyConfig(nil, &vpnConfig)
}

// ValidateConfig validates VPN configuration
func (p *VPNAdapter) ValidateConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var vpnConfig VPNConfig
	if err := json.Unmarshal(configJSON, &vpnConfig); err != nil {
		return err
	}

	return p.provider.Validate(nil, &vpnConfig)
}

// Flush removes all VPN tunnels
func (p *VPNAdapter) Flush() error {
	return p.provider.Flush(nil)
}

// Status returns the current VPN status
func (p *VPNAdapter) Status() (interface{}, error) {
	enabled, provider, tunnels, err := p.provider.Status(nil)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled":  enabled,
		"provider": provider,
		"tunnels":  tunnels,
	}, nil
}

// Close terminates the plugin
func (p *VPNAdapter) Close() error {
	return nil
}
