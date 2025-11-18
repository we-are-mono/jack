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
	"bytes"
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

// TestLoggerSQLite3BasicIntegration verifies that the logger emits events to the sqlite3 plugin
// and that logs are correctly stored in the database.
func TestLoggerSQLite3BasicIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create jack.json with sqlite3 plugin enabled
	jackConfig := map[string]interface{}{
		"version": "1.0",
		"plugins": map[string]interface{}{
			"sqlite3": map[string]interface{}{
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

	// Start daemon with log capture
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logBuffer := &bytes.Buffer{}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = harness.StartDaemonWithOutput(ctx, logBuffer)
	}()

	// Wait for daemon to be ready
	harness.WaitForDaemon(5 * time.Second)

	// Print daemon logs for debugging
	t.Logf("Daemon logs:\n%s", logBuffer.String())

	// Trigger some daemon activity that generates logs
	resp, err := harness.SendRequest(daemon.Request{Command: "status"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Status command should succeed")

	// Wait for async log event delivery (increased wait for reliability)
	time.Sleep(1 * time.Second)

	// Verify database was created
	assert.FileExists(t, dbPath, "Database file should be created")

	// Open database and verify logs were stored
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Count log entries
	var logCount int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&logCount)
	require.NoError(t, err)
	assert.Greater(t, logCount, 0, "Should have at least one log entry")

	// Verify log structure
	rows, err := db.Query("SELECT timestamp, level, component, message, fields FROM logs LIMIT 5")
	require.NoError(t, err)
	defer rows.Close()

	logEntries := make([]struct {
		Timestamp string
		Level     string
		Component string
		Message   string
		Fields    string
	}, 0)

	for rows.Next() {
		var entry struct {
			Timestamp string
			Level     string
			Component string
			Message   string
			Fields    string
		}
		err := rows.Scan(&entry.Timestamp, &entry.Level, &entry.Component, &entry.Message, &entry.Fields)
		require.NoError(t, err)
		logEntries = append(logEntries, entry)
	}

	assert.NotEmpty(t, logEntries, "Should have retrieved log entries")

	// Verify each entry has required fields
	for i, entry := range logEntries {
		assert.NotEmpty(t, entry.Timestamp, "Entry %d should have timestamp", i)
		assert.NotEmpty(t, entry.Level, "Entry %d should have level", i)
		assert.NotEmpty(t, entry.Component, "Entry %d should have component", i)
		assert.NotEmpty(t, entry.Message, "Entry %d should have message", i)
		assert.Contains(t, []string{"debug", "info", "warn", "error"}, entry.Level,
			"Entry %d should have valid log level", i)
	}

	// Verify fields are valid JSON
	for i, entry := range logEntries {
		if entry.Fields != "" && entry.Fields != "null" {
			var fields map[string]interface{}
			err := json.Unmarshal([]byte(entry.Fields), &fields)
			assert.NoError(t, err, "Entry %d fields should be valid JSON", i)
		}
	}

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

