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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"
	"github.com/we-are-mono/jack/plugins"
)

// MonitoringRPCProvider implements the Provider interface with CLI command support
type MonitoringRPCProvider struct {
	provider *MonitoringProvider
}

// NewMonitoringRPCProvider creates a new RPC provider for monitoring
func NewMonitoringRPCProvider() *MonitoringRPCProvider {
	provider := NewMonitoringProvider()

	return &MonitoringRPCProvider{
		provider: provider,
	}
}

// Metadata returns plugin information including CLI commands
func (p *MonitoringRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "monitoring",
		Version:     "1.0.0",
		Description: "System and network metrics collection",
		Category:    "monitoring",
		ConfigPath:  "/etc/jack/monitoring.json",
		DefaultConfig: map[string]interface{}{
			"enabled":             true,
			"collection_interval": 5,
		},
		CLICommands: []plugins.CLICommand{
			{
				Name:         "monitor",
				Short:        "Monitor system resources and network bandwidth",
				Long:         "Display real-time system metrics including CPU, memory, load, and network bandwidth.",
				Subcommands:  []string{"stats", "bandwidth"},
				Continuous:   true, // Mark all monitor commands as continuous
				PollInterval: 2,    // Poll every 2 seconds
			},
		},
	}, nil
}

// ApplyConfig applies monitoring configuration
func (p *MonitoringRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var config MonitoringConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}
	return p.provider.ApplyConfig(&config)
}

// ValidateConfig validates monitoring configuration
func (p *MonitoringRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var config MonitoringConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}
	return p.provider.Validate(&config)
}

// Flush removes all configuration
func (p *MonitoringRPCProvider) Flush(ctx context.Context) error {
	return p.provider.Stop()
}

// Status returns current status as JSON
func (p *MonitoringRPCProvider) Status(ctx context.Context) ([]byte, error) {
	status, err := p.provider.Status()
	if err != nil {
		return nil, err
	}
	return json.Marshal(status)
}

// OnLogEvent is not implemented for the monitoring plugin
func (p *MonitoringRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *MonitoringRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Command should be "monitor stats" or "monitor bandwidth"
	if parts[0] != "monitor" {
		return nil, fmt.Errorf("unknown command: %s", parts[0])
	}

	if len(parts) < 2 {
		return nil, fmt.Errorf("monitor command requires subcommand: stats or bandwidth")
	}

	subcommand := parts[1]

	switch subcommand {
	case "stats":
		return p.executeStats()
	case "bandwidth":
		return p.executeBandwidth(args)
	default:
		return nil, fmt.Errorf("unknown monitor subcommand: %s", subcommand)
	}
}

// executeStats returns formatted system and interface statistics
func (p *MonitoringRPCProvider) executeStats() ([]byte, error) {
	// Get status from provider
	statusData, err := p.provider.Status()
	if err != nil {
		return nil, err
	}

	data, ok := statusData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid status format")
	}

	var buf bytes.Buffer

	// Print system metrics
	buf.WriteString("System Metrics\n")
	buf.WriteString("==============\n")

	if sysMetrics, ok := data["system_metrics"].(*SystemMetrics); ok && sysMetrics != nil {
		buf.WriteString(fmt.Sprintf("CPU Usage:       %5.1f%%\n", sysMetrics.CPUPercent))
		buf.WriteString(fmt.Sprintf("Memory Usage:    %5.1f%% (%d MB / %d MB)\n",
			sysMetrics.MemoryPercent, sysMetrics.MemoryUsedMB, sysMetrics.MemoryTotalMB))
		buf.WriteString(fmt.Sprintf("Load Average:    %.2f, %.2f, %.2f\n",
			sysMetrics.LoadAvg1, sysMetrics.LoadAvg5, sysMetrics.LoadAvg15))
		buf.WriteString(fmt.Sprintf("Last Updated:    %s\n",
			sysMetrics.Timestamp.Format("2006-01-02 15:04:05")))
	} else {
		buf.WriteString("No system metrics available\n")
	}

	buf.WriteString("\n")

	// Print interface metrics
	buf.WriteString("Network Interfaces\n")
	buf.WriteString("==================\n")

	if ifMetrics, ok := data["interface_metrics"].([]InterfaceMetrics); ok && len(ifMetrics) > 0 {
		buf.WriteString(fmt.Sprintf("%-15s %-8s %15s %15s %10s %10s\n",
			"Interface", "State", "RX Bytes", "TX Bytes", "RX Rate", "TX Rate"))
		buf.WriteString("------------------------------------------------------------------------------\n")

		for _, iface := range ifMetrics {
			buf.WriteString(fmt.Sprintf("%-15s %-8s %15s %15s %10s %10s\n",
				iface.Name,
				iface.State,
				formatBytes(iface.RXBytes),
				formatBytes(iface.TXBytes),
				formatBandwidth(iface.RXBytesRate),
				formatBandwidth(iface.TXBytesRate),
			))
		}
	} else {
		buf.WriteString("No interface metrics available\n")
	}

	return buf.Bytes(), nil
}

// executeBandwidth returns formatted bandwidth information with ASCII graphs
func (p *MonitoringRPCProvider) executeBandwidth(args []string) ([]byte, error) {
	// Determine target interface (default: wg-proton)
	targetInterface := "wg-proton"
	if len(args) > 0 {
		targetInterface = args[0]
	}

	// Access provider directly
	if p.provider == nil {
		return nil, fmt.Errorf("monitoring provider not initialized")
	}

	p.provider.mu.RLock()
	defer p.provider.mu.RUnlock()

	// Find the target interface in current metrics
	var targetMetric *InterfaceMetrics
	for i := range p.provider.interfaceMetrics {
		if p.provider.interfaceMetrics[i].Name == targetInterface {
			targetMetric = &p.provider.interfaceMetrics[i]
			break
		}
	}

	if targetMetric == nil {
		return nil, fmt.Errorf("interface '%s' not found", targetInterface)
	}

	// Get bandwidth history
	history, hasHistory := p.provider.bandwidthHistory[targetInterface]

	var buf bytes.Buffer

	buf.WriteString("Bandwidth Monitor - ")
	buf.WriteString(targetInterface)
	buf.WriteString(" (Press Ctrl+C to exit)\n")
	buf.WriteString("========================================\n\n")

	now := time.Now().Format("15:04:05")
	buf.WriteString(fmt.Sprintf("Updated: %s | State: %s\n\n", now, targetMetric.State))

	// Current rates
	buf.WriteString("Current Rates:\n")
	buf.WriteString(fmt.Sprintf("  RX: %15s (%6.2f Mbps)\n",
		formatBandwidth(targetMetric.RXBytesRate),
		float64(targetMetric.RXBytesRate)*8/1000000))
	buf.WriteString(fmt.Sprintf("  TX: %15s (%6.2f Mbps)\n\n",
		formatBandwidth(targetMetric.TXBytesRate),
		float64(targetMetric.TXBytesRate)*8/1000000))

	// Render graphs if we have history
	if hasHistory && len(history.RXRates) > 1 {
		buf.WriteString("RX Rate (Mbps) - Last 1 minute:\n")
		rxGraph := asciigraph.Plot(history.RXRates,
			asciigraph.Height(8),
			asciigraph.Width(60),
			asciigraph.Caption(""))
		buf.WriteString(rxGraph)
		buf.WriteString("\n\n")

		buf.WriteString("TX Rate (Mbps) - Last 1 minute:\n")
		txGraph := asciigraph.Plot(history.TXRates,
			asciigraph.Height(8),
			asciigraph.Width(60),
			asciigraph.Caption(""))
		buf.WriteString(txGraph)
		buf.WriteString("\n\n")

		buf.WriteString(fmt.Sprintf("Showing %d data points (%.0f seconds of history)\n",
			len(history.RXRates), float64(len(history.RXRates))*5))
	} else {
		buf.WriteString("Collecting data... graphs will appear shortly.\n")
	}

	return buf.Bytes(), nil
}

// formatBytes formats byte counts in human-readable format
func formatBytes(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatBandwidth formats bandwidth in human-readable format
func formatBandwidth(bytesPerSec uint64) string {
	if bytesPerSec == 0 {
		return "0 B/s"
	}

	const unit = 1024
	if bytesPerSec < unit {
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}

	div, exp := uint64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB/s", float64(bytesPerSec)/float64(div), "KMGTPE"[exp])
}
