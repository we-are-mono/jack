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

// Package main implements a self-contained firewall plugin for Jack using nftables.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/we-are-mono/jack/plugins"
)

// FirewallRPCProvider implements the Provider interface with direct RPC support
type FirewallRPCProvider struct {
	provider       *FirewallProvider
	config         *FirewallConfig
	capture        *LogCapture
	db             *FirewallDatabase
	daemonService  plugins.DaemonService
	captureStarted bool
	initOnce       sync.Once
	initReady      chan struct{}
}

// NewFirewallRPCProvider creates a new RPC provider for firewall
func NewFirewallRPCProvider() (*FirewallRPCProvider, error) {
	provider, err := NewFirewall()
	if err != nil {
		return nil, err
	}

	return &FirewallRPCProvider{
		provider:  provider,
		initReady: make(chan struct{}),
	}, nil
}

// Metadata returns plugin information
func (p *FirewallRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "firewall",
		Version:     "1.0.0",
		Description: "nftables firewall management with optional logging",
		Category:    "firewall",
		ConfigPath:  "/etc/jack/firewall.json",
		CLICommands: []plugins.CLICommand{
			{
				Name:        "firewall",
				Short:       "Query firewall logs and statistics",
				Long:        "Display firewall logging statistics and recent log entries.",
				Subcommands: []string{"stats", "logs"},
			},
			{
				Name:         "firewall watch",
				Short:        "Monitor firewall logs in real-time",
				Long:         "Continuously display recent firewall log entries. Optional filters: action=ACCEPT|DROP, src_ip=IP, dst_ip=IP, proto=TCP|UDP|ICMP.",
				Continuous:   true,
				PollInterval: 2,
			},
		},
	}, nil
}

// ApplyConfig applies firewall configuration
func (p *FirewallRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var firewallConfig FirewallConfig
	if err := json.Unmarshal(configJSON, &firewallConfig); err != nil {
		return err
	}

	// Store config for later reference
	p.config = &firewallConfig

	// Skip if firewall is disabled
	if !firewallConfig.Enabled {
		return nil
	}

	// Apply firewall rules first
	if err := p.provider.ApplyConfig(&firewallConfig); err != nil {
		return err
	}

	// Initialize logging if enabled
	if firewallConfig.Logging.Enabled {
		if p.daemonService != nil && p.daemonService.IsServiceReady("database") {
			log.Printf("[FIREWALL] Logging enabled, initializing capture...\n")

			// Create database handler if not already created
			if p.db == nil {
				p.db = NewFirewallDatabase(p.daemonService)
			}

			// Create log capture if not already created
			if p.capture == nil {
				p.capture = NewLogCapture(p.db, &firewallConfig.Logging)
			}

			// Initialize database schema and start capture in background
			// This avoids reentrant RPC calls during ApplyConfig
			p.initOnce.Do(func() {
				go func() {
					// Wait for ApplyConfig to return
					<-p.initReady

					// Initialize database schema
					if err := p.ensureSchemaInitialized(context.Background()); err != nil {
						log.Printf("[FIREWALL] Failed to initialize database schema: %v\n", err)
						return
					}

					// Start packet capture
					if err := p.capture.Start(); err != nil {
						log.Printf("[FIREWALL] Failed to start packet capture: %v\n", err)
						return
					}

					p.captureStarted = true
					log.Printf("[FIREWALL] Packet capture started successfully\n")
				}()

				// Signal that ApplyConfig has returned
				defer close(p.initReady)
			})
		} else {
			log.Printf("[FIREWALL] Logging enabled but database service not available\n")
		}
	} else {
		// Logging disabled, stop capture if running
		if p.capture != nil && p.captureStarted {
			log.Printf("[FIREWALL] Stopping packet capture (logging disabled)\n")
			p.capture.Stop()
			p.captureStarted = false
		}
	}

	return nil
}

// ValidateConfig validates firewall configuration
func (p *FirewallRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var firewallConfig FirewallConfig
	if err := json.Unmarshal(configJSON, &firewallConfig); err != nil {
		return err
	}

	return p.provider.Validate(&firewallConfig)
}

// Flush removes all firewall rules and stops logging
func (p *FirewallRPCProvider) Flush(ctx context.Context) error {
	// Stop packet capture if running
	if p.capture != nil && p.captureStarted {
		log.Printf("[FIREWALL] Stopping packet capture\n")
		p.capture.Stop()
		p.captureStarted = false
	}

	return p.provider.Flush()
}

// Status returns current status as JSON
func (p *FirewallRPCProvider) Status(ctx context.Context) ([]byte, error) {
	status, err := p.provider.Status()
	if err != nil {
		return nil, err
	}

	loggingEnabled := p.config != nil && p.config.Logging.Enabled
	databaseReady := p.daemonService != nil && p.daemonService.IsServiceReady("database")

	// Collect warnings
	var warnings []string
	if loggingEnabled && !databaseReady {
		warnings = append(warnings, "Logging configured but database service unavailable (sqlite3 plugin may be disabled)")
	}

	// Return as map for consistent JSON marshaling
	statusMap := map[string]interface{}{
		"enabled":          status.Enabled,
		"backend":          status.Backend,
		"rule_count":       status.RuleCount,
		"logging_enabled":  loggingEnabled,
		"logging_active":   p.captureStarted,
		"database_ready":   databaseReady,
		"schema_init":      p.db != nil && p.db.IsInitialized(),
		"warnings":         warnings,
	}

	return json.Marshal(statusMap)
}

// OnLogEvent is not implemented for the firewall plugin
func (p *FirewallRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *FirewallRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	// Check if database is available
	if p.db == nil || !p.db.IsInitialized() {
		// Provide more specific error message based on what's missing
		loggingEnabled := p.config != nil && p.config.Logging.Enabled
		databaseReady := p.daemonService != nil && p.daemonService.IsServiceReady("database")

		var errorMsg string
		if !loggingEnabled {
			errorMsg = `Error: Firewall logging not enabled.

Action: Set "logging.enabled": true in /etc/jack/firewall.json
        Then run 'jack apply'
`
		} else if !databaseReady {
			errorMsg = `Error: Firewall logging unavailable.

Reason: Database service not ready (sqlite3 plugin may be disabled)
Action: Run 'jack plugin enable sqlite3' and 'jack apply'

Note: If you don't need logging, set "logging.enabled": false in /etc/jack/firewall.json
`
		} else {
			errorMsg = `Error: Firewall logging database not initialized.

This may indicate a database initialization error.
Check daemon logs: journalctl -u jack -n 50
`
		}

		return []byte(errorMsg), nil
	}

	// Parse command
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid command format")
	}

	subcommand := parts[1]
	switch subcommand {
	case "stats":
		return p.executeStats(ctx)
	case "logs":
		return p.executeLogs(ctx, args)
	case "watch":
		return p.executeWatch(ctx, args)
	default:
		return nil, fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// executeStats displays firewall statistics
func (p *FirewallRPCProvider) executeStats(ctx context.Context) ([]byte, error) {
	stats, err := p.db.QueryStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}

	output := fmt.Sprintf(`Firewall Statistics:
  Total Logs:   %d
  Accepts:      %d
  Drops:        %d
`, stats.Total, stats.Accepts, stats.Drops)

	return []byte(output), nil
}

// executeLogs displays recent log entries
func (p *FirewallRPCProvider) executeLogs(ctx context.Context, args []string) ([]byte, error) {
	// Query recent logs (limit 100)
	logs, err := p.db.QueryLogs(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	if len(logs) == 0 {
		return []byte("No firewall logs found.\n"), nil
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Recent Firewall Logs (%d entries):\n\n", len(logs)))
	output.WriteString(fmt.Sprintf("%-20s %-8s %-15s -> %-15s %-8s %s\n",
		"TIMESTAMP", "ACTION", "SRC_IP", "DST_IP", "PROTO", "PORTS"))
	output.WriteString(strings.Repeat("-", 100) + "\n")

	for _, log := range logs {
		ports := ""
		if log.SrcPort > 0 || log.DstPort > 0 {
			ports = fmt.Sprintf("%d:%d", log.SrcPort, log.DstPort)
		}

		output.WriteString(fmt.Sprintf("%-20s %-8s %-15s -> %-15s %-8s %s\n",
			log.Timestamp, log.Action, log.SrcIP, log.DstIP, log.Protocol, ports))
	}

	return []byte(output.String()), nil
}

// executeWatch displays recent logs with optional filtering
func (p *FirewallRPCProvider) executeWatch(ctx context.Context, args []string) ([]byte, error) {
	// Parse filter arguments
	filter := &FirewallLogQuery{
		Limit: 20, // Default limit for watch
	}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "action":
			filter.Action = value
		case "src_ip":
			filter.SrcIP = value
		case "dst_ip":
			filter.DstIP = value
		case "proto":
			filter.Protocol = value
		}
	}

	// Query logs with filter
	logs, err := p.db.QueryLogsFiltered(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Firewall Logs (Last %d entries) - %s\n\n",
		filter.Limit, time.Now().Format("15:04:05")))

	if len(logs) == 0 {
		output.WriteString("No logs matching filters.\n")
		return []byte(output.String()), nil
	}

	output.WriteString(fmt.Sprintf("%-20s %-8s %-15s -> %-15s %-8s %s\n",
		"TIMESTAMP", "ACTION", "SRC_IP", "DST_IP", "PROTO", "PORTS"))
	output.WriteString(strings.Repeat("-", 100) + "\n")

	for _, log := range logs {
		ports := ""
		if log.SrcPort > 0 || log.DstPort > 0 {
			ports = fmt.Sprintf("%d:%d", log.SrcPort, log.DstPort)
		}

		output.WriteString(fmt.Sprintf("%-20s %-8s %-15s -> %-15s %-8s %s\n",
			log.Timestamp, log.Action, log.SrcIP, log.DstIP, log.Protocol, ports))
	}

	return []byte(output.String()), nil
}

// GetProvidedServices returns the list of services this plugin provides (none)
func (p *FirewallRPCProvider) GetProvidedServices(ctx context.Context) ([]plugins.ServiceDescriptor, error) {
	return nil, nil
}

// CallService is not implemented as this plugin doesn't provide services
func (p *FirewallRPCProvider) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	return nil, fmt.Errorf("plugin does not provide any services")
}

// SetDaemonService stores daemon service reference
func (p *FirewallRPCProvider) SetDaemonService(daemon plugins.DaemonService) {
	p.daemonService = daemon

	// Create database handler if needed
	if daemon != nil {
		p.db = NewFirewallDatabase(daemon)
	}
}

// ensureSchemaInitialized initializes the database schema if not already done
func (p *FirewallRPCProvider) ensureSchemaInitialized(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if p.db.IsInitialized() {
		return nil // Already initialized
	}

	// Wait for database service to be ready
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := p.daemonService.WaitForService(waitCtx, "database"); err != nil {
		return fmt.Errorf("database service not ready: %w", err)
	}

	// Initialize schema
	return p.db.InitSchema(ctx)
}

// DatabaseOperations interface implementation (pass-through to FirewallDatabase)

// InsertLog inserts a firewall log entry into the database
func (p *FirewallRPCProvider) InsertLog(ctx context.Context, entry *FirewallLogEntry) error {
	if p.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return p.db.InsertLog(ctx, entry)
}

// CleanupOldLogs removes logs older than the retention period
func (p *FirewallRPCProvider) CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	return p.db.CleanupOldLogs(ctx, retentionDays)
}

// EnforceMaxEntries ensures the log table doesn't exceed max entries
func (p *FirewallRPCProvider) EnforceMaxEntries(ctx context.Context, maxEntries int) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}
	return p.db.EnforceMaxEntries(ctx, maxEntries)
}

// Vacuum performs database vacuum operation
func (p *FirewallRPCProvider) Vacuum(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database not initialized")
	}
	return p.db.Vacuum(ctx)
}
