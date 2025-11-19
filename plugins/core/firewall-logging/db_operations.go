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

	"github.com/we-are-mono/jack/plugins"
)

// DatabaseService defines the interface for database operations
type DatabaseService interface {
	CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error)
}

// FirewallDatabase handles all database operations for firewall logging
type FirewallDatabase struct {
	daemonService DatabaseService
	schemaInit    bool
}

// NewFirewallDatabase creates a new database handler
func NewFirewallDatabase(daemonService plugins.DaemonService) *FirewallDatabase {
	return &FirewallDatabase{
		daemonService: daemonService,
		schemaInit:    false,
	}
}

// IsInitialized returns whether the schema has been initialized
func (db *FirewallDatabase) IsInitialized() bool {
	return db.schemaInit
}

// InitSchema creates the firewall_logs table and indexes if they don't exist
func (db *FirewallDatabase) InitSchema(ctx context.Context) error {
	if db.schemaInit {
		return nil // Already initialized
	}

	if db.daemonService == nil {
		return fmt.Errorf("daemon service not available")
	}

	log.Printf("[FIREWALL-DB] Initializing database schema...\n")

	// Execute schema creation statements
	statements := []string{
		`CREATE TABLE IF NOT EXISTS firewall_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp TEXT NOT NULL,
			action TEXT NOT NULL,
			src_ip TEXT NOT NULL,
			dst_ip TEXT NOT NULL,
			protocol TEXT,
			src_port INTEGER DEFAULT 0,
			dst_port INTEGER DEFAULT 0,
			interface_in TEXT,
			interface_out TEXT,
			packet_length INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_logs_timestamp ON firewall_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_logs_action ON firewall_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_logs_src_ip ON firewall_logs(src_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_logs_dst_ip ON firewall_logs(dst_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_firewall_logs_created_at ON firewall_logs(created_at)`,
	}

	for i, stmt := range statements {
		log.Printf("[FIREWALL-DB] Executing statement %d/%d\n", i+1, len(statements))
		execArgs := map[string]interface{}{
			"query": stmt,
			"args":  []interface{}{},
		}

		argsJSON, err := json.Marshal(execArgs)
		if err != nil {
			return fmt.Errorf("failed to marshal schema args: %w", err)
		}

		_, err = db.daemonService.CallService(ctx, "database", "Exec", argsJSON)
		if err != nil {
			return fmt.Errorf("failed to create schema (statement %d): %w", i+1, err)
		}
	}

	db.schemaInit = true
	log.Printf("[FIREWALL-DB] Schema initialized successfully\n")
	return nil
}

// ResetInitialization marks the schema as uninitialized (for cleanup)
func (db *FirewallDatabase) ResetInitialization() {
	db.schemaInit = false
}

// StatsResult contains firewall statistics
type StatsResult struct {
	Total   int64
	Accepts int64
	Drops   int64
}

// QueryStats retrieves firewall log statistics
func (db *FirewallDatabase) QueryStats(ctx context.Context) (*StatsResult, error) {
	// Query total logs
	totalQuery := "SELECT COUNT(*) FROM firewall_logs"
	totalArgs := map[string]interface{}{
		"query": totalQuery,
		"args":  []interface{}{},
	}
	totalJSON, err := json.Marshal(totalArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	totalResult, err := db.daemonService.CallService(ctx, "database", "QueryRow", totalJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query total logs: %w", err)
	}

	var totalCount struct {
		Columns []string      `json:"columns"`
		Values  []interface{} `json:"values"`
	}
	if err := json.Unmarshal(totalResult, &totalCount); err != nil {
		return nil, fmt.Errorf("failed to unmarshal total count: %w", err)
	}

	total := int64(0)
	if len(totalCount.Values) > 0 {
		if v, ok := totalCount.Values[0].(float64); ok {
			total = int64(v)
		}
	}

	// Query drops
	dropsQuery := "SELECT COUNT(*) FROM firewall_logs WHERE action = ?"
	dropsArgs := map[string]interface{}{
		"query": dropsQuery,
		"args":  []interface{}{"DROP"},
	}
	dropsJSON, err := json.Marshal(dropsArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	dropsResult, err := db.daemonService.CallService(ctx, "database", "QueryRow", dropsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query drops: %w", err)
	}

	var dropsCount struct {
		Columns []string      `json:"columns"`
		Values  []interface{} `json:"values"`
	}
	if err := json.Unmarshal(dropsResult, &dropsCount); err != nil {
		return nil, fmt.Errorf("failed to unmarshal drops count: %w", err)
	}

	drops := int64(0)
	if len(dropsCount.Values) > 0 {
		if v, ok := dropsCount.Values[0].(float64); ok {
			drops = int64(v)
		}
	}

	return &StatsResult{
		Total:   total,
		Accepts: total - drops,
		Drops:   drops,
	}, nil
}

// LogEntry represents a single firewall log entry
type LogEntry struct {
	Timestamp string
	Action    string
	SrcIP     string
	DstIP     string
	Protocol  string
	SrcPort   int64
	DstPort   int64
}

// QueryLogs retrieves recent firewall log entries
func (db *FirewallDatabase) QueryLogs(ctx context.Context, limit int) ([]LogEntry, error) {
	query := fmt.Sprintf("SELECT timestamp, action, src_ip, dst_ip, protocol, src_port, dst_port FROM firewall_logs ORDER BY id DESC LIMIT %d", limit)
	queryArgs := map[string]interface{}{
		"query": query,
		"args":  []interface{}{},
	}
	queryJSON, err := json.Marshal(queryArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	result, err := db.daemonService.CallService(ctx, "database", "Query", queryJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	var rows struct {
		Columns []string        `json:"columns"`
		Rows    [][]interface{} `json:"rows"`
	}
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	// Convert to LogEntry structs
	entries := make([]LogEntry, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		if len(row) < 7 {
			continue
		}

		entry := LogEntry{}
		if v, ok := row[0].(string); ok {
			entry.Timestamp = v
		}
		if v, ok := row[1].(string); ok {
			entry.Action = v
		}
		if v, ok := row[2].(string); ok {
			entry.SrcIP = v
		}
		if v, ok := row[3].(string); ok {
			entry.DstIP = v
		}
		if v, ok := row[4].(string); ok {
			entry.Protocol = v
		}
		if v, ok := row[5].(float64); ok {
			entry.SrcPort = int64(v)
		}
		if v, ok := row[6].(float64); ok {
			entry.DstPort = int64(v)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// InsertLog inserts a firewall log entry into the database
func (db *FirewallDatabase) InsertLog(ctx context.Context, entry *FirewallLogEntry) error {
	query := `INSERT INTO firewall_logs (timestamp, action, src_ip, dst_ip, protocol, src_port, dst_port, interface_in, interface_out, packet_length)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	execArgs := map[string]interface{}{
		"query": query,
		"args": []interface{}{
			entry.Timestamp,
			entry.Action,
			entry.SrcIP,
			entry.DstIP,
			entry.Protocol,
			entry.SrcPort,
			entry.DstPort,
			entry.InterfaceIn,
			entry.InterfaceOut,
			entry.PacketLength,
		},
	}

	argsJSON, err := json.Marshal(execArgs)
	if err != nil {
		return fmt.Errorf("failed to marshal insert args: %w", err)
	}

	_, err = db.daemonService.CallService(ctx, "database", "Exec", argsJSON)
	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}
