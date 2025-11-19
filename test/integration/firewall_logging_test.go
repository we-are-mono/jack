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

//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// TestFirewallLoggingPlugin verifies the firewall-logging plugin integration
func TestFirewallLoggingPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create jack.json with sqlite3 and firewall-logging plugins enabled
	jackConfig := map[string]interface{}{
		"version": "1.0",
		"plugins": map[string]interface{}{
			"sqlite3": map[string]interface{}{
				"enabled": true,
				"version": "",
			},
			"firewall-logging": map[string]interface{}{
				"enabled": true,
				"version": "",
			},
		},
	}
	jackConfigJSON, err := json.Marshal(jackConfig)
	require.NoError(t, err)
	jackPath := filepath.Join(harness.configDir, "jack.json")
	err = os.WriteFile(jackPath, jackConfigJSON, 0644)
	require.NoError(t, err)

	// Create sqlite3 plugin config
	dbPath := filepath.Join(harness.configDir, "logs.db")
	sqlite3Config := map[string]interface{}{
		"enabled":             true,
		"database_path":       dbPath,
		"log_storage_enabled": true,
		"max_log_entries":     0,
	}
	sqlite3ConfigJSON, err := json.Marshal(sqlite3Config)
	require.NoError(t, err)
	sqlite3Path := filepath.Join(harness.configDir, "sqlite3.json")
	err = os.WriteFile(sqlite3Path, sqlite3ConfigJSON, 0644)
	require.NoError(t, err)

	// Create firewall-logging plugin config
	fwLoggingConfig := map[string]interface{}{
		"enabled":            true,
		"log_accepts":        false,
		"log_drops":          true,
		"sampling_rate":      1,
		"rate_limit_per_sec": 100,
		"max_log_entries":    1000,
		"retention_days":     7,
	}
	fwLoggingJSON, err := json.Marshal(fwLoggingConfig)
	require.NoError(t, err)
	fwLoggingPath := filepath.Join(harness.configDir, "firewall-logging.json")
	err = os.WriteFile(fwLoggingPath, fwLoggingJSON, 0644)
	require.NoError(t, err)

	// Create minimal interfaces and routes configs
	interfacesConfig := map[string]interface{}{
		"interfaces": map[string]interface{}{},
	}
	interfacesJSON, err := json.Marshal(interfacesConfig)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(harness.configDir, "interfaces.json"), interfacesJSON, 0644)
	require.NoError(t, err)

	routesConfig := map[string]interface{}{
		"routes": map[string]interface{}{},
	}
	routesJSON, err := json.Marshal(routesConfig)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(harness.configDir, "routes.json"), routesJSON, 0644)
	require.NoError(t, err)

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = harness.StartDaemon(ctx)
	}()

	// Wait for daemon to be ready
	harness.WaitForDaemon(5 * time.Second)

	// Apply configuration to activate plugins
	applyResp, err := harness.SendRequest(daemon.Request{
		Command: "apply",
	})
	require.NoError(t, err)
	if !applyResp.Success {
		t.Logf("Apply failed with error: %s", applyResp.Error)
	}
	require.True(t, applyResp.Success, "apply command should succeed")

	// Verify database was created
	time.Sleep(500 * time.Millisecond) // Allow plugins to initialize
	assert.FileExists(t, dbPath, "Database file should be created")

	// Open database and verify schema
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify firewall_logs table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='firewall_logs'").Scan(&tableName)
	require.NoError(t, err, "firewall_logs table should exist")
	assert.Equal(t, "firewall_logs", tableName)

	// Verify table schema (check for expected columns)
	rows, err := db.Query("PRAGMA table_info(firewall_logs)")
	require.NoError(t, err)
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	expectedColumns := []string{"id", "timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port", "interface_in", "interface_out", "packet_length", "created_at"}
	for _, col := range expectedColumns {
		assert.True(t, columns[col], "Column %s should exist", col)
	}

	// Insert test log entries directly into database
	insertTestLogs(t, db)

	// Wait a moment for database write
	time.Sleep(200 * time.Millisecond)

	// Verify logs can be queried
	var logCount int
	err = db.QueryRow("SELECT COUNT(*) FROM firewall_logs").Scan(&logCount)
	require.NoError(t, err)
	assert.Equal(t, 3, logCount, "Should have 3 test log entries")

	// Test firewall stats CLI command
	resp, err := harness.SendRequest(daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "firewall-logging",
		CLICommand: "firewall stats",
		CLIArgs:    []string{},
	})
	require.NoError(t, err)
	if !resp.Success {
		t.Logf("Stats command failed with error: %s", resp.Error)
	}
	require.True(t, resp.Success, "Stats command should succeed")
	statsOutput, ok := resp.Data.(string)
	require.True(t, ok, "output should be string")
	assert.Contains(t, statsOutput, "Total Logs:", "Stats should show total logs")
	assert.Contains(t, statsOutput, "Drops:", "Stats should show drop count")

	// Test firewall logs CLI command
	resp, err = harness.SendRequest(daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "firewall-logging",
		CLICommand: "firewall logs",
		CLIArgs:    []string{},
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "Logs command should succeed")
	logsOutput, ok := resp.Data.(string)
	require.True(t, ok, "output should be string")
	assert.Contains(t, logsOutput, "192.168.1.100", "Logs should show test IP")
	assert.Contains(t, logsOutput, "DROP", "Logs should show DROP action")

	// Test filtering with --action flag
	resp, err = harness.SendRequest(daemon.Request{
		Command:    "plugin-cli",
		Plugin:     "firewall-logging",
		CLICommand: "firewall logs",
		CLIArgs:    []string{"--action", "DROP"},
	})
	require.NoError(t, err)
	require.True(t, resp.Success, "Filtered logs command should succeed")

	// Shutdown daemon
	cancel()
	select {
	case <-serverDone:
		t.Log("Daemon shut down gracefully")
	case <-time.After(2 * time.Second):
		t.Fatal("Daemon shutdown timed out")
	}
	time.Sleep(100 * time.Millisecond) // Allow Stop() to finish
}

// TestFirewallLoggingDatabaseOperations tests database operations in isolation
func TestFirewallLoggingDatabaseOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create test database
	dbPath := filepath.Join(harness.configDir, "test.db")

	// Create configs
	dbConfig := map[string]interface{}{
		"enabled":             true,
		"database_path":       dbPath,
		"log_storage_enabled": true,
	}
	dbConfigJSON, err := json.Marshal(dbConfig)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(harness.configDir, "sqlite3.json"), dbConfigJSON, 0644)
	require.NoError(t, err)

	// Open database directly using the plugin's database manager
	// (We can't easily import the plugin's types here, so we use SQL directly)
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create schema manually for testing
	schema := `
		CREATE TABLE IF NOT EXISTS firewall_logs (
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
		);
	`
	_, err = db.Exec(schema)
	require.NoError(t, err)

	// Test insertion
	insertLog := `
		INSERT INTO firewall_logs (timestamp, action, src_ip, dst_ip, protocol, src_port, dst_port, interface_in, packet_length)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = db.Exec(insertLog, time.Now().UTC().Format(time.RFC3339), "DROP", "10.0.0.1", "8.8.8.8", "TCP", 12345, 443, "eth0", 60)
	require.NoError(t, err)

	// Test query
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM firewall_logs").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Test cleanup (retention)
	oldTimestamp := time.Now().UTC().AddDate(0, 0, -10).Format(time.RFC3339)
	_, err = db.Exec(insertLog, oldTimestamp, "DROP", "10.0.0.2", "1.1.1.1", "UDP", 54321, 53, "eth0", 100)
	require.NoError(t, err)

	// Delete old entries (simulate retention cleanup)
	cutoffTime := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	result, err := db.Exec("DELETE FROM firewall_logs WHERE timestamp < ?", cutoffTime)
	require.NoError(t, err)
	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), rowsAffected, "Should delete 1 old entry")

	// Verify only recent entry remains
	err = db.QueryRow("SELECT COUNT(*) FROM firewall_logs").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have 1 entry after cleanup")
}

// insertTestLogs inserts test firewall log entries into the database
func insertTestLogs(t *testing.T, db *sql.DB) {
	t.Helper()

	logs := []struct {
		timestamp    string
		action       string
		srcIP        string
		dstIP        string
		protocol     string
		srcPort      int
		dstPort      int
		interfaceIn  string
		packetLength int
	}{
		{
			timestamp:    time.Now().UTC().Format(time.RFC3339),
			action:       "DROP",
			srcIP:        "192.168.1.100",
			dstIP:        "8.8.8.8",
			protocol:     "TCP",
			srcPort:      54321,
			dstPort:      80,
			interfaceIn:  "eth0",
			packetLength: 60,
		},
		{
			timestamp:    time.Now().UTC().Format(time.RFC3339),
			action:       "DROP",
			srcIP:        "10.0.0.50",
			dstIP:        "1.1.1.1",
			protocol:     "UDP",
			srcPort:      12345,
			dstPort:      53,
			interfaceIn:  "br-lan",
			packetLength: 128,
		},
		{
			timestamp:    time.Now().UTC().Format(time.RFC3339),
			action:       "DROP",
			srcIP:        "172.16.0.100",
			dstIP:        "9.9.9.9",
			protocol:     "ICMP",
			srcPort:      0,
			dstPort:      0,
			interfaceIn:  "eth0",
			packetLength: 84,
		},
	}

	query := `
		INSERT INTO firewall_logs (
			timestamp, action, src_ip, dst_ip, protocol,
			src_port, dst_port, interface_in, packet_length
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, log := range logs {
		_, err := db.Exec(query,
			log.timestamp,
			log.action,
			log.srcIP,
			log.dstIP,
			log.protocol,
			log.srcPort,
			log.dstPort,
			log.interfaceIn,
			log.packetLength,
		)
		require.NoError(t, err)
	}
}
