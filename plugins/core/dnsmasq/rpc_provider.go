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
	"context"
	"encoding/json"
	"fmt"

	"github.com/we-are-mono/jack/plugins"
)

// DnsmasqRPCProvider implements the Provider interface with direct RPC support
type DnsmasqRPCProvider struct {
	provider *DnsmasqProvider
}

// NewDnsmasqRPCProvider creates a new RPC provider for dnsmasq
func NewDnsmasqRPCProvider() (*DnsmasqRPCProvider, error) {
	provider, err := NewDnsmasq()
	if err != nil {
		return nil, err
	}

	return &DnsmasqRPCProvider{
		provider: provider,
	}, nil
}

// Metadata returns plugin information
func (p *DnsmasqRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "dhcp",
		Version:     "1.0.0",
		Description: "dnsmasq-based DHCP server",
		Category:    "dhcp",
		ConfigPath:  "/etc/jack/dhcp.json",
	}, nil
}

// ApplyConfig applies DHCP configuration
func (p *DnsmasqRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
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
func (p *DnsmasqRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var dhcpConfig DHCPConfig
	if err := json.Unmarshal(configJSON, &dhcpConfig); err != nil {
		return err
	}

	return p.provider.ValidateDHCP(nil, &dhcpConfig)
}

// Flush stops DHCP service
func (p *DnsmasqRPCProvider) Flush(ctx context.Context) error {
	return p.provider.FlushDHCP(nil)
}

// Status returns current status as JSON
func (p *DnsmasqRPCProvider) Status(ctx context.Context) ([]byte, error) {
	enabled, provider, leaseCount, err := p.provider.StatusDHCP(nil)
	if err != nil {
		return nil, err
	}

	statusMap := map[string]interface{}{
		"enabled":     enabled,
		"provider":    provider,
		"lease_count": leaseCount,
	}

	return json.Marshal(statusMap)
}

// OnLogEvent is not implemented for the dnsmasq plugin
func (p *DnsmasqRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *DnsmasqRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	// Future: could add commands like "jack dhcp leases", "jack dhcp restart"
	return nil, fmt.Errorf("plugin does not implement CLI commands")
}
