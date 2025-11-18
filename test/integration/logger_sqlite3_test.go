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

	// Trigger some daemon activity that generates logs
	resp, err := harness.SendRequest(daemon.Request{Command: "status"})
	require.NoError(t, err)
	require.True(t, resp.Success, "Status command should succeed")

	// Wait for async log event delivery
	time.Sleep(300 * time.Millisecond)

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

// TestStructuredLoggingFields verifies that structured fields are correctly preserved
func TestStructuredLoggingFields(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Setup sqlite3 plugin
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
	err = os.WriteFile(filepath.Join(harness.configDir, "jack.json"), jackConfigJSON, 0644)
	require.NoError(t, err)

	dbPath := filepath.Join(harness.configDir, "logs.db")
	sqlite3Config := map[string]interface{}{
		"enabled":             true,
		"database_path":       dbPath,
		"log_storage_enabled": true,
		"max_log_entries":     0,
	}
	sqlite3ConfigJSON, err := json.Marshal(sqlite3Config)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(harness.configDir, "sqlite3.json"), sqlite3ConfigJSON, 0644)
	require.NoError(t, err)

	// Minimal configs
	err = os.WriteFile(filepath.Join(harness.configDir, "interfaces.json"), []byte(`{"interfaces":{}}`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(harness.configDir, "routes.json"), []byte(`{"routes":{}}`), 0644)
	require.NoError(t, err)

	// Start daemon
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = harness.StartDaemon(ctx)
	}()

	harness.WaitForDaemon(5 * time.Second)

	// The daemon startup logs should have structured fields
	// Wait for logs to be written
	time.Sleep(300 * time.Millisecond)

	// Query database to find logs with structured fields
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Look for logs that have non-empty fields
	rows, err := db.Query("SELECT message, fields FROM logs WHERE fields != '' AND fields != 'null' AND fields != '{}' LIMIT 5")
	require.NoError(t, err)
	defer rows.Close()

	foundFieldsLog := false
	for rows.Next() {
		var message, fields string
		err := rows.Scan(&message, &fields)
		require.NoError(t, err)

		// Parse fields JSON
		var fieldMap map[string]interface{}
		err = json.Unmarshal([]byte(fields), &fieldMap)
		require.NoError(t, err, "Fields should be valid JSON")

		if len(fieldMap) > 0 {
			foundFieldsLog = true
			t.Logf("Found log with fields: message=%q, fields=%v", message, fieldMap)

			// Verify field values are preserved correctly
			for key, value := range fieldMap {
				assert.NotNil(t, value, "Field %q should have a value", key)
			}
		}
	}

	assert.True(t, foundFieldsLog, "Should find at least one log entry with structured fields")

	// Shutdown
	cancel()
	<-serverDone
	time.Sleep(100 * time.Millisecond)
}
