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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/we-are-mono/jack/plugins"
)

// FirewallLoggingRPCProvider implements the Provider interface for firewall logging
type FirewallLoggingRPCProvider struct {
	config         *FirewallLoggingConfig
	capture        *LogCapture
	db             *FirewallDatabase
	captureStarted bool // Track if log capture has been started
}

// NewFirewallLoggingRPCProvider creates a new RPC provider for firewall logging
func NewFirewallLoggingRPCProvider() *FirewallLoggingRPCProvider {
	return &FirewallLoggingRPCProvider{}
}

// Metadata returns plugin information including CLI commands
func (p *FirewallLoggingRPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:        "firewall-logging",
		Version:          "1.0.0",
		Description:      "Capture and store firewall logs in database",
		Category:         "firewall",
		ConfigPath:       "/etc/jack/firewall-logging.json",
		Dependencies:     []string{"sqlite3"},
		RequiredServices: []string{"database"},
		DefaultConfig: map[string]interface{}{
			"enabled":             true,
			"log_accepts":         false,
			"log_drops":           true,
			"sampling_rate":       1,
			"rate_limit_per_sec":  100,
			"max_log_entries":     50000,
			"retention_days":      7,
		},
		CLICommands: []plugins.CLICommand{
			{
				Name:        "firewall",
				Short:       "Firewall log management",
				Long:        "View firewall logs and statistics",
				Subcommands: []string{"stats", "logs"},
			},
		},
	}, nil
}

// ApplyConfig applies firewall logging configuration
func (p *FirewallLoggingRPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	log.Printf("[FIREWALL-LOGGING] ApplyConfig called\n")
	var config FirewallLoggingConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Validate configuration
	if err := p.validateConfig(&config); err != nil {
		return err
	}

	// Stop existing capture if running
	if p.capture != nil {
		p.capture.Stop()
		p.capture = nil
	}

	// Store config
	p.config = &config

	// If disabled, we're done
	if !config.Enabled {
		log.Printf("[FIREWALL-LOGGING] Plugin disabled, skipping initialization\n")
		return nil
	}

	// NOTE: We defer schema initialization AND log capture start until after ApplyConfig returns.
	// Starting them during ApplyConfig would cause a reentrant RPC call deadlock.
	// Instead, we trigger initialization in a goroutine that runs AFTER this RPC call completes.

	// Create log capture instance (but don't start the goroutine yet)
	p.capture = NewLogCapture(p, &config)

	// Trigger schema initialization in a background goroutine after ApplyConfig returns
	// This small delay ensures the ApplyConfig RPC call has completed and the daemon
	// can process the schema initialization RPC calls without deadlocking
	go func() {
		// Wait for ApplyConfig to return and services to be marked ready
		time.Sleep(100 * time.Millisecond)

		log.Printf("[FIREWALL-LOGGING] Auto-initializing schema after ApplyConfig...\n")
		if err := p.ensureSchemaInitialized(context.Background()); err != nil {
			log.Printf("[FIREWALL-LOGGING] Warning: auto schema initialization failed: %v\n", err)
		}
	}()

	log.Printf("[FIREWALL-LOGGING] ApplyConfig completed successfully\n")
	return nil
}

// ValidateConfig validates firewall logging configuration
func (p *FirewallLoggingRPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var config FirewallLoggingConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return p.validateConfig(&config)
}

// validateConfig performs configuration validation
func (p *FirewallLoggingRPCProvider) validateConfig(config *FirewallLoggingConfig) error {
	if config.SamplingRate < 1 {
		return fmt.Errorf("sampling_rate must be >= 1")
	}
	if config.RateLimitPerSec < 0 {
		return fmt.Errorf("rate_limit_per_sec cannot be negative")
	}
	return nil
}

// Flush removes all configuration
func (p *FirewallLoggingRPCProvider) Flush(ctx context.Context) error {
	if p.capture != nil {
		p.capture.Stop()
		p.capture = nil
	}

	// Reset database initialization state
	if p.db != nil {
		p.db.ResetInitialization()
	}

	p.config = nil
	p.captureStarted = false
	return nil
}

// Status returns current status as JSON
func (p *FirewallLoggingRPCProvider) Status(ctx context.Context) ([]byte, error) {
	status := map[string]interface{}{
		"enabled": false,
		"config":  p.config,
	}

	if p.config != nil && p.config.Enabled {
		status["enabled"] = true
	}

	return json.Marshal(status)
}

// OnLogEvent is not implemented for the firewall-logging plugin
func (p *FirewallLoggingRPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	return fmt.Errorf("plugin does not implement log event handling")
}

// ExecuteCLICommand executes firewall CLI commands
func (p *FirewallLoggingRPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	// Check if plugin is configured
	if p.config == nil || !p.config.Enabled {
		return nil, fmt.Errorf("firewall logging is not enabled")
	}

	// Check if database is available
	if p.db == nil {
		return nil, fmt.Errorf("database service not available")
	}

	// Parse command - expecting "firewall <subcommand>"
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Split command into parts
	var subcommand string
	if len(command) > 9 && command[:9] == "firewall " {
		subcommand = command[9:]
	} else if command == "firewall" {
		return nil, fmt.Errorf("missing subcommand (use: stats or logs)")
	} else {
		return nil, fmt.Errorf("unknown command: %s", command)
	}

	// Handle subcommands
	switch subcommand {
	case "stats":
		return p.executeStats(ctx)
	case "logs":
		return p.executeLogs(ctx, args)
	default:
		return nil, fmt.Errorf("unknown subcommand: %s (use: stats or logs)", subcommand)
	}
}

// executeStats queries and formats firewall statistics
func (p *FirewallLoggingRPCProvider) executeStats(ctx context.Context) ([]byte, error) {
	// Ensure schema is initialized before querying
	if err := p.ensureSchemaInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Query stats from database
	stats, err := p.db.QueryStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}

	// Format output
	output := fmt.Sprintf("Firewall Log Statistics\n")
	output += fmt.Sprintf("=======================\n")
	output += fmt.Sprintf("Total Logs: %d\n", stats.Total)
	output += fmt.Sprintf("Accepts: %d\n", stats.Accepts)
	output += fmt.Sprintf("Drops: %d\n", stats.Drops)

	return []byte(output), nil
}

// executeLogs queries and formats firewall log entries
func (p *FirewallLoggingRPCProvider) executeLogs(ctx context.Context, args []string) ([]byte, error) {
	// Ensure schema is initialized before querying
	if err := p.ensureSchemaInitialized(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Query logs from database (limit to 100)
	logs, err := p.db.QueryLogs(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	// Format output
	output := "Firewall Logs\n"
	output += "=============\n"

	if len(logs) == 0 {
		output += "No logs found\n"
	} else {
		for _, entry := range logs {
			output += fmt.Sprintf("[%s] %s: %s:%d -> %s:%d (%s)\n",
				entry.Timestamp, entry.Action, entry.SrcIP, entry.SrcPort,
				entry.DstIP, entry.DstPort, entry.Protocol)
		}
	}

	return []byte(output), nil
}

// ensureCaptureStarted lazily starts the log capture goroutine on first use
// This avoids starting it during ApplyConfig which could cause reentrant RPC deadlocks
func (p *FirewallLoggingRPCProvider) ensureCaptureStarted() error {
	if p.capture == nil {
		return fmt.Errorf("capture not initialized (plugin not configured)")
	}

	// Check if already started
	if p.captureStarted {
		return nil // Already started
	}

	if err := p.capture.Start(); err != nil {
		return fmt.Errorf("failed to start log capture: %w", err)
	}

	p.captureStarted = true
	log.Printf("[FIREWALL-LOGGING] Log capture started successfully\n")
	return nil
}

// ensureSchemaInitialized lazily initializes the database schema on first use
// This avoids reentrant RPC calls during ApplyConfig
func (p *FirewallLoggingRPCProvider) ensureSchemaInitialized(ctx context.Context) error {
	if p.db != nil && p.db.IsInitialized() {
		return nil // Already initialized
	}

	if p.db == nil {
		return fmt.Errorf("database not available")
	}

	log.Printf("[FIREWALL-LOGGING] Lazy initializing database schema...\n")
	if err := p.db.InitSchema(ctx); err != nil {
		return fmt.Errorf("failed to initialize database schema: %w", err)
	}

	log.Printf("[FIREWALL-LOGGING] Database schema initialized successfully\n")

	// Now that schema is ready, start log capture
	if err := p.ensureCaptureStarted(); err != nil {
		log.Printf("[FIREWALL-LOGGING] Warning: failed to start log capture: %v\n", err)
		// Don't return error - schema init succeeded
	}

	return nil
}

// InsertLog inserts a firewall log entry into the database via the daemon service
func (p *FirewallLoggingRPCProvider) InsertLog(entry *FirewallLogEntry) error {
	// If database is not set, silently drop the log
	if p.db == nil {
		return nil
	}

	// If schema is not initialized yet, silently drop the log
	// This avoids reentrant RPC calls during ApplyConfig
	if !p.db.IsInitialized() {
		return nil
	}

	return p.db.InsertLog(context.Background(), entry)
}

// CleanupOldLogs is not implemented - maintenance handled by sqlite3 plugin
func (p *FirewallLoggingRPCProvider) CleanupOldLogs(retentionDays int) (int64, error) {
	return 0, nil
}

// EnforceMaxEntries is not implemented - maintenance handled by sqlite3 plugin
func (p *FirewallLoggingRPCProvider) EnforceMaxEntries(maxEntries int) (int64, error) {
	return 0, nil
}

// Vacuum is not implemented - database maintenance is handled by sqlite3 plugin
func (p *FirewallLoggingRPCProvider) Vacuum() error {
	return nil
}

// SetDaemonService stores the daemon service reference for plugin-to-plugin calls
func (p *FirewallLoggingRPCProvider) SetDaemonService(daemon plugins.DaemonService) {
	log.Printf("[FIREWALL-LOGGING] SetDaemonService called\n")
	p.db = NewFirewallDatabase(daemon)
	log.Printf("[FIREWALL-LOGGING] Database handler created\n")
}

// GetProvidedServices returns the list of services this plugin provides (none)
func (p *FirewallLoggingRPCProvider) GetProvidedServices(ctx context.Context) ([]plugins.ServiceDescriptor, error) {
	return nil, nil
}

// CallService is not implemented as this plugin doesn't provide services
func (p *FirewallLoggingRPCProvider) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	return nil, fmt.Errorf("plugin does not provide any services")
}
