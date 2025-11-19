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
	"context"
	"encoding/json"
	"fmt"

	"github.com/we-are-mono/jack/plugins"
)

// WireGuardRPCProvider implements the Provider interface with direct RPC support
type WireGuardRPCProvider struct {
	provider *WireGuardProvider
}

// NewWireGuardRPCProvider creates a new RPC provider for WireGuard
func NewWireGuardRPCProvider() (*WireGuardRPCProvider, error) {
	provider, err := New()
	if err != nil {
		return nil, err
	}

	return &WireGuardRPCProvider{
		provider: provider,
	}, nil
}

// Metadata returns plugin information
func (p *WireGuardRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "vpn",
		Version:     "1.0.0",
		Description: "WireGuard-based VPN management",
		Category:    "vpn",
		ConfigPath:  "/etc/jack/vpn.json",
	}, nil
}

// ApplyConfig applies VPN configuration
func (p *WireGuardRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var vpnConfig VPNConfig
	if err := json.Unmarshal(configJSON, &vpnConfig); err != nil {
		return err
	}

	return p.provider.ApplyConfig(nil, &vpnConfig)
}

// ValidateConfig validates VPN configuration
func (p *WireGuardRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var vpnConfig VPNConfig
	if err := json.Unmarshal(configJSON, &vpnConfig); err != nil {
		return err
	}

	return p.provider.Validate(nil, &vpnConfig)
}

// Flush removes all VPN tunnels
func (p *WireGuardRPCProvider) Flush(ctx context.Context) error {
	return p.provider.Flush(nil)
}

// Status returns current status as JSON
func (p *WireGuardRPCProvider) Status(ctx context.Context) ([]byte, error) {
	enabled, provider, tunnels, err := p.provider.Status(nil)
	if err != nil {
		return nil, err
	}

	statusMap := map[string]interface{}{
		"enabled":  enabled,
		"provider": provider,
		"tunnels":  tunnels,
	}

	return json.Marshal(statusMap)
}

// OnLogEvent is not implemented for the wireguard plugin
func (p *WireGuardRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *WireGuardRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	// Future: could add commands like "jack vpn status", "jack vpn restart"
	return nil, fmt.Errorf("plugin does not implement CLI commands")
}

// GetProvidedServices returns the list of services this plugin provides (none)
func (p *WireGuardRPCProvider) GetProvidedServices(ctx context.Context) ([]plugins.ServiceDescriptor, error) {
	return nil, nil
}

// CallService is not implemented as this plugin doesn't provide services
func (p *WireGuardRPCProvider) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	return nil, fmt.Errorf("plugin does not provide any services")
}

// SetDaemonService stores daemon service reference (not used by this plugin)
func (p *WireGuardRPCProvider) SetDaemonService(daemon plugins.DaemonService) {
	// Not used by this plugin
}
