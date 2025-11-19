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

// Package main implements a self-contained SQLite3 database plugin for Jack.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/we-are-mono/jack/plugins"
)

// SQLite3RPCProvider implements the Provider interface with CLI command support
type SQLite3RPCProvider struct {
	provider *DatabaseProvider
	lastPath string // Track database path to detect when it changes
}

// NewSQLite3RPCProvider creates a new RPC provider for sqlite3
func NewSQLite3RPCProvider() *SQLite3RPCProvider {
	return &SQLite3RPCProvider{
		provider: nil, // Provider is created when config is applied
	}
}

// Metadata returns plugin information including CLI commands
func (p *SQLite3RPCProvider) Metadata(ctx context.Context) (plugins.MetadataResponse, error) {
	return plugins.MetadataResponse{
		Namespace:   "sqlite3",
		Version:     "1.0.0",
		Description: "SQLite3 database for logging and data storage",
		Category:    "database",
		ConfigPath:  "/etc/jack/sqlite3.json",
		DefaultConfig: map[string]interface{}{
			"enabled":             true,
			"database_path":       "/var/lib/jack/data.db",
			"log_storage_enabled": true,
			"max_log_entries":     100000,
		},
		ProvidedServices: []plugins.ServiceDescriptor{
			{
				Name:        "database",
				Description: "SQLite3 database operations for logging and storage",
				Methods:     []string{"InsertLog", "QueryLogs", "Stats", "Vacuum", "Exec", "Query", "QueryRow"},
			},
		},
		CLICommands: []plugins.CLICommand{
			{
				Name:        "sqlite3",
				Short:       "Query database logs and statistics",
				Long:        "Query logs stored in SQLite database and view database statistics.",
				Subcommands: []string{"logs", "stats", "vacuum"},
				Continuous:  false,
			},
		},
	}, nil
}

// ApplyConfig applies sqlite3 configuration
func (p *SQLite3RPCProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var config DatabaseConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}

	// If database is not open yet, open it
	if p.provider == nil {
		provider, err := NewDatabaseProvider(&config)
		if err != nil {
			return fmt.Errorf("failed to create database provider: %w", err)
		}
		p.provider = provider
		p.lastPath = config.DatabasePath
		return nil
	}

	// If database path changed, we need to close old and open new
	if config.DatabasePath != p.lastPath {
		p.provider.Close()
		provider, err := NewDatabaseProvider(&config)
		if err != nil {
			return fmt.Errorf("failed to create database provider: %w", err)
		}
		p.provider = provider
		p.lastPath = config.DatabasePath
		return nil
	}

	// Database is already open with same path - keep it open
	// Just update any runtime config (like max_log_entries) if needed
	// For now, we don't have any runtime-updatable config, so just return
	return nil
}

// ValidateConfig validates sqlite3 configuration
func (p *SQLite3RPCProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var config DatabaseConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return err
	}

	// Basic validation
	if config.DatabasePath == "" {
		return fmt.Errorf("database_path is required")
	}

	return nil
}

// Flush removes all configuration
func (p *SQLite3RPCProvider) Flush(ctx context.Context) error {
	// Close database connection
	if p.provider != nil {
		return p.provider.Close()
	}
	return nil
}

// Status returns current status as JSON
func (p *SQLite3RPCProvider) Status(ctx context.Context) ([]byte, error) {
	if p.provider == nil {
		status := map[string]interface{}{
			"enabled": false,
			"message": "Database not initialized",
		}
		return json.Marshal(status)
	}

	stats, err := p.provider.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	stats["enabled"] = true
	return json.Marshal(stats)
}

// ExecuteCLICommand executes CLI commands provided by this plugin
func (p *SQLite3RPCProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
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
	case "vacuum":
		return p.executeVacuum(ctx)
	default:
		return nil, fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

// executeStats returns database statistics
func (p *SQLite3RPCProvider) executeStats(ctx context.Context) ([]byte, error) {
	if p.provider == nil {
		return []byte("Database not initialized\n"), nil
	}

	stats, err := p.provider.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Format stats for display
	output := fmt.Sprintf("Database Statistics:\n")
	output += fmt.Sprintf("  Path: %s\n", stats["database_path"])
	output += fmt.Sprintf("  Size: %d bytes\n", stats["database_size_bytes"])
	output += fmt.Sprintf("  Log entries: %d\n", stats["log_count"])

	return []byte(output), nil
}

// executeLogs queries and displays logs from the database
func (p *SQLite3RPCProvider) executeLogs(ctx context.Context, args []string) ([]byte, error) {
	if p.provider == nil {
		return []byte("Database not initialized\n"), nil
	}

	// Parse command-line arguments
	var level, component string
	limit := 50 // Default limit

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--level":
			if i+1 < len(args) {
				level = args[i+1]
				i++
			}
		case "--component":
			if i+1 < len(args) {
				component = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &limit)
				i++
			}
		}
	}

	// Query logs
	entries, err := p.provider.QueryLogs(level, component, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	if len(entries) == 0 {
		return []byte("No logs found matching criteria\n"), nil
	}

	// Format output
	var output strings.Builder
	fmt.Fprintf(&output, "Found %d log entries:\n\n", len(entries))

	for _, entry := range entries {
		fmt.Fprintf(&output, "[%s] %s - %s: %s\n",
			entry.Timestamp,
			strings.ToUpper(entry.Level),
			entry.Component,
			entry.Message)
		if entry.Fields != "" && entry.Fields != "null" {
			fmt.Fprintf(&output, "  Fields: %s\n", entry.Fields)
		}
	}

	return []byte(output.String()), nil
}

// executeVacuum compacts the database
func (p *SQLite3RPCProvider) executeVacuum(ctx context.Context) ([]byte, error) {
	if p.provider == nil {
		return []byte("Database not initialized\n"), nil
	}

	// Get database size before vacuum
	statsBefore, err := p.provider.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats: %w", err)
	}
	sizeBefore := statsBefore["database_size_bytes"].(int64)

	// Execute vacuum
	if err := p.provider.Vacuum(); err != nil {
		return nil, fmt.Errorf("failed to vacuum database: %w", err)
	}

	// Get database size after vacuum
	statsAfter, err := p.provider.Stats()
	if err != nil {
		return nil, fmt.Errorf("failed to get database stats after vacuum: %w", err)
	}
	sizeAfter := statsAfter["database_size_bytes"].(int64)

	// Calculate space saved
	spaceSaved := sizeBefore - sizeAfter
	var output strings.Builder
	fmt.Fprintf(&output, "Database vacuum completed successfully\n")
	fmt.Fprintf(&output, "  Size before: %d bytes\n", sizeBefore)
	fmt.Fprintf(&output, "  Size after:  %d bytes\n", sizeAfter)
	if spaceSaved > 0 {
		fmt.Fprintf(&output, "  Space saved: %d bytes\n", spaceSaved)
	} else {
		fmt.Fprintf(&output, "  No space reclaimed (database was already compact)\n")
	}

	return []byte(output.String()), nil
}

// OnLogEvent receives a log event and stores it in the database
// This is called by the daemon when log events occur
func (p *SQLite3RPCProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	if p.provider == nil {
		return fmt.Errorf("database not initialized")
	}

	// Deserialize log event from JSON
	var logEvent map[string]interface{}
	if err := json.Unmarshal(logEventJSON, &logEvent); err != nil {
		return fmt.Errorf("failed to unmarshal log event: %w", err)
	}

	// Extract fields from log event
	timestamp, _ := logEvent["timestamp"].(string)
	level, _ := logEvent["level"].(string)
	component, _ := logEvent["component"].(string)
	message, _ := logEvent["message"].(string)

	// Marshal fields back to JSON
	fields, _ := json.Marshal(logEvent["fields"])

	// Store in database
	return p.provider.InsertLog(timestamp, level, component, message, string(fields))
}

// GetProvidedServices returns the list of services this plugin provides
func (p *SQLite3RPCProvider) GetProvidedServices(ctx context.Context) ([]plugins.ServiceDescriptor, error) {
	return []plugins.ServiceDescriptor{
		{
			Name:        "database",
			Description: "SQLite3 database operations for logging and storage",
			Methods:     []string{"InsertLog", "QueryLogs", "Stats", "Vacuum", "Exec", "Query", "QueryRow"},
		},
	}, nil
}

// CallService handles service method calls from other plugins
func (p *SQLite3RPCProvider) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	if serviceName != "database" {
		return nil, fmt.Errorf("unknown service: %s", serviceName)
	}

	if p.provider == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	switch method {
	case "InsertLog":
		return p.handleInsertLog(ctx, argsJSON)
	case "QueryLogs":
		return p.handleQueryLogs(ctx, argsJSON)
	case "Stats":
		return p.handleStats(ctx)
	case "Vacuum":
		return p.handleVacuum(ctx)
	case "Exec":
		return p.handleExec(ctx, argsJSON)
	case "Query":
		return p.handleQuery(ctx, argsJSON)
	case "QueryRow":
		return p.handleQueryRow(ctx, argsJSON)
	default:
		return nil, fmt.Errorf("unknown method: %s", method)
	}
}

// handleInsertLog handles InsertLog service calls
func (p *SQLite3RPCProvider) handleInsertLog(ctx context.Context, argsJSON []byte) ([]byte, error) {
	var args struct {
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Component string `json:"component"`
		Message   string `json:"message"`
		Fields    string `json:"fields"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	if err := p.provider.InsertLog(args.Timestamp, args.Level, args.Component, args.Message, args.Fields); err != nil {
		return nil, err
	}

	// Return success response
	return json.Marshal(map[string]interface{}{"success": true})
}

// handleQueryLogs handles QueryLogs service calls
func (p *SQLite3RPCProvider) handleQueryLogs(ctx context.Context, argsJSON []byte) ([]byte, error) {
	var args struct {
		Level     string `json:"level"`
		Component string `json:"component"`
		Limit     int    `json:"limit"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	entries, err := p.provider.QueryLogs(args.Level, args.Component, args.Limit)
	if err != nil {
		return nil, err
	}

	return json.Marshal(entries)
}

// handleStats handles Stats service calls
func (p *SQLite3RPCProvider) handleStats(ctx context.Context) ([]byte, error) {
	stats, err := p.provider.Stats()
	if err != nil {
		return nil, err
	}

	return json.Marshal(stats)
}

// handleVacuum handles Vacuum service calls
func (p *SQLite3RPCProvider) handleVacuum(ctx context.Context) ([]byte, error) {
	if err := p.provider.Vacuum(); err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{"success": true})
}

// handleExec handles Exec service calls for arbitrary SQL execution
func (p *SQLite3RPCProvider) handleExec(ctx context.Context, argsJSON []byte) ([]byte, error) {
	var args struct {
		Query string        `json:"query"`
		Args  []interface{} `json:"args"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	rowsAffected, err := p.provider.Exec(args.Query, args.Args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"rows_affected": rowsAffected,
	})
}

// handleQuery handles Query service calls for SELECT queries returning multiple rows
func (p *SQLite3RPCProvider) handleQuery(ctx context.Context, argsJSON []byte) ([]byte, error) {
	var args struct {
		Query string        `json:"query"`
		Args  []interface{} `json:"args"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	columns, rows, err := p.provider.Query(args.Query, args.Args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"columns": columns,
		"rows":    rows,
	})
}

// handleQueryRow handles QueryRow service calls for SELECT queries returning a single row
func (p *SQLite3RPCProvider) handleQueryRow(ctx context.Context, argsJSON []byte) ([]byte, error) {
	var args struct {
		Query string        `json:"query"`
		Args  []interface{} `json:"args"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	columns, values, err := p.provider.QueryRow(args.Query, args.Args...)
	if err != nil {
		return nil, err
	}

	return json.Marshal(map[string]interface{}{
		"columns": columns,
		"values":  values,
	})
}

// SetDaemonService stores daemon service reference (not used by this plugin)
func (p *SQLite3RPCProvider) SetDaemonService(daemon plugins.DaemonService) {
	// Not used by this plugin
}
