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
	"context"
	"encoding/json"
	"fmt"

	"github.com/we-are-mono/jack/plugins"
)

// NftablesRPCProvider implements the Provider interface with direct RPC support
type NftablesRPCProvider struct {
	provider *NftablesProvider
}

// NewNftablesRPCProvider creates a new RPC provider for nftables
func NewNftablesRPCProvider() (*NftablesRPCProvider, error) {
	provider, err := NewNftables()
	if err != nil {
		return nil, err
	}

	return &NftablesRPCProvider{
		provider: provider,
	}, nil
}

// Metadata returns plugin information
func (p *NftablesRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "firewall",
		Version:     "1.0.0",
		Description: "nftables-based firewall management",
		Category:    "firewall",
		ConfigPath:  "/etc/jack/firewall.json",
	}, nil
}

// ApplyConfig applies firewall configuration
func (p *NftablesRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
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
func (p *NftablesRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var firewallConfig FirewallConfig
	if err := json.Unmarshal(configJSON, &firewallConfig); err != nil {
		return err
	}

	return p.provider.Validate(&firewallConfig)
}

// Flush removes all firewall rules
func (p *NftablesRPCProvider) Flush(ctx context.Context) error {
	return p.provider.Flush()
}

// Status returns current status as JSON
func (p *NftablesRPCProvider) Status(ctx context.Context) ([]byte, error) {
	status, err := p.provider.Status()
	if err != nil {
		return nil, err
	}

	// Return as map for consistent JSON marshaling
	statusMap := map[string]interface{}{
		"enabled":    status.Enabled,
		"backend":    status.Backend,
		"rule_count": status.RuleCount,
	}

	return json.Marshal(statusMap)
}

// OnLogEvent is not implemented for the nftables plugin
func (p *NftablesRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *NftablesRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	// Future: could add commands like "jack firewall list", "jack firewall reload"
	return nil, fmt.Errorf("plugin does not implement CLI commands")
}

// GetProvidedServices returns the list of services this plugin provides (none)
func (p *NftablesRPCProvider) GetProvidedServices(ctx context.Context) ([]plugins.ServiceDescriptor, error) {
	return nil, nil
}

// CallService is not implemented as this plugin doesn't provide services
func (p *NftablesRPCProvider) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	return nil, fmt.Errorf("plugin does not provide any services")
}

// SetDaemonService stores daemon service reference (not used by this plugin)
func (p *NftablesRPCProvider) SetDaemonService(daemon plugins.DaemonService) {
	// Not used by this plugin
}
