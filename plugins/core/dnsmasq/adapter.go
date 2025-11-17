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

// Package main implements a self-contained dnsmasq DHCP plugin for Jack.
package main

import (
	"encoding/json"

	"github.com/we-are-mono/jack/plugins"
)

// DHCPAdapter adapts the dnsmasq provider to the unified Plugin interface.
type DHCPAdapter struct {
	provider *DnsmasqProvider
}

// NewDHCPAdapter creates a new DHCP plugin adapter
func NewDHCPAdapter() (*DHCPAdapter, error) {
	provider, err := NewDnsmasq()
	if err != nil {
		return nil, err
	}

	return &DHCPAdapter{
		provider: provider,
	}, nil
}

// Metadata returns information about what this plugin provides
func (p *DHCPAdapter) Metadata() plugins.PluginMetadata {
	return plugins.PluginMetadata{
		Namespace:   "dhcp",
		Version:     "1.0.0",
		Description: "dnsmasq-based DHCP server",
		Category:    "dhcp",
		ConfigPath:  "/etc/jack/dhcp.json",
	}
}

// ApplyConfig applies DHCP configuration
func (p *DHCPAdapter) ApplyConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	// Check if config is empty (no meaningful configuration)
	var configMap map[string]interface{}
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		return err
	}
	if len(configMap) == 0 {
		// Empty config - nothing to apply, skip silently
		return nil
	}

	var dhcpConfig DHCPConfig
	if err := json.Unmarshal(configJSON, &dhcpConfig); err != nil {
		return err
	}

	return p.provider.ApplyDHCPConfig(nil, &dhcpConfig)
}

// ValidateConfig validates DHCP configuration
func (p *DHCPAdapter) ValidateConfig(config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	var dhcpConfig DHCPConfig
	if err := json.Unmarshal(configJSON, &dhcpConfig); err != nil {
		return err
	}

	return p.provider.ValidateDHCP(nil, &dhcpConfig)
}

// Flush stops DHCP service
func (p *DHCPAdapter) Flush() error {
	return p.provider.FlushDHCP(nil)
}

// Status returns the current DHCP status
func (p *DHCPAdapter) Status() (interface{}, error) {
	enabled, provider, leaseCount, err := p.provider.StatusDHCP(nil)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"enabled":     enabled,
		"provider":    provider,
		"lease_count": leaseCount,
	}, nil
}

// Close terminates the plugin
func (p *DHCPAdapter) Close() error {
	return nil
}
