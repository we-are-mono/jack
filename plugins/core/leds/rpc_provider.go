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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/we-are-mono/jack/plugins"
)

// LEDRPCProvider implements the Provider interface with CLI command support
type LEDRPCProvider struct {
	adapter *LEDAdapter
}

// NewLEDRPCProvider creates a new RPC provider for LED control
func NewLEDRPCProvider() *LEDRPCProvider {
	adapter := NewLEDAdapter()

	return &LEDRPCProvider{
		adapter: adapter,
	}
}

// Metadata returns plugin information including CLI commands
func (p *LEDRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	meta := p.adapter.Metadata()

	return plugins.MetadataResponse{
		Namespace:     meta.Namespace,
		Version:       meta.Version,
		Description:   meta.Description,
		Category:      meta.Category,
		ConfigPath:    meta.ConfigPath,
		DefaultConfig: meta.DefaultConfig,
		Dependencies:  meta.Dependencies,
		PathPrefix:    meta.PathPrefix, // Include path prefix for automatic path rewriting
		CLICommands: []plugins.CLICommand{
			{
				Name:         "led",
				Short:        "LED control and status",
				Long:         "Control system LEDs and display their current state.",
				Subcommands:  []string{"status", "list"},
				Continuous:   false, // Default: status and list are one-off commands
				PollInterval: 0,
			},
		},
	}, nil
}

// ApplyConfig applies LED configuration
func (p *LEDRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var config interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}
	return p.adapter.ApplyConfig(config)
}

// ValidateConfig validates LED configuration
func (p *LEDRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var config interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}
	return p.adapter.ValidateConfig(config)
}

// Flush removes all configuration
func (p *LEDRPCProvider) Flush(ctx context.Context) error {
	return p.adapter.Flush()
}

// Status returns current status as JSON
func (p *LEDRPCProvider) Status(ctx context.Context) ([]byte, error) {
	status, err := p.adapter.Status()
	if err != nil {
		return nil, err
	}
	return json.Marshal(status)
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *LEDRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Command should be "led status" or "led list"
	if parts[0] != "led" {
		return nil, fmt.Errorf("unknown command: %s", parts[0])
	}

	if len(parts) < 2 {
		return nil, fmt.Errorf("led command requires subcommand: status or list")
	}

	subcommand := parts[1]

	switch subcommand {
	case "status":
		return p.executeStatus()
	case "list":
		return p.executeList()
	default:
		return nil, fmt.Errorf("unknown led subcommand: %s", subcommand)
	}
}

// executeStatus returns formatted LED status
func (p *LEDRPCProvider) executeStatus() ([]byte, error) {
	// Get status from provider
	statusData, err := p.adapter.Status()
	if err != nil {
		return nil, err
	}

	ledStatus, ok := statusData.(*LEDStatus)
	if !ok {
		return nil, fmt.Errorf("invalid status format")
	}

	var buf bytes.Buffer

	buf.WriteString("LED Status\n")
	buf.WriteString("==========\n\n")

	if len(ledStatus.LEDs) == 0 {
		buf.WriteString("No LEDs found\n")
		return buf.Bytes(), nil
	}

	// Print header
	buf.WriteString(fmt.Sprintf("%-20s %-12s %-15s %-20s\n",
		"LED", "Brightness", "Max Brightness", "Trigger"))
	buf.WriteString(strings.Repeat("-", 70) + "\n")

	// Print each LED
	for _, led := range ledStatus.LEDs {
		buf.WriteString(fmt.Sprintf("%-20s %-12d %-15d %-20s\n",
			led.Name,
			led.Brightness,
			led.MaxBrightness,
			led.CurrentTrigger))
	}

	return buf.Bytes(), nil
}

// executeList returns a list of available LEDs with their capabilities
func (p *LEDRPCProvider) executeList() ([]byte, error) {
	// Get status from provider (which includes all LEDs)
	statusData, err := p.adapter.Status()
	if err != nil {
		return nil, err
	}

	ledStatus, ok := statusData.(*LEDStatus)
	if !ok {
		return nil, fmt.Errorf("invalid status format")
	}

	var buf bytes.Buffer

	buf.WriteString("Available LEDs\n")
	buf.WriteString("==============\n\n")

	if len(ledStatus.LEDs) == 0 {
		buf.WriteString("No LEDs found\n")
		return buf.Bytes(), nil
	}

	for _, led := range ledStatus.LEDs {
		buf.WriteString(fmt.Sprintf("%s\n", led.Name))
		buf.WriteString(fmt.Sprintf("  Max Brightness: %d\n", led.MaxBrightness))
		buf.WriteString(fmt.Sprintf("  Current Trigger: %s\n", led.CurrentTrigger))
		if len(led.AvailTriggers) > 0 {
			buf.WriteString(fmt.Sprintf("  Available Triggers: %s\n", strings.Join(led.AvailTriggers, ", ")))
		}
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}
