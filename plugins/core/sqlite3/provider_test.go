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
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test database
func setupTestDB(t *testing.T) (*DatabaseProvider, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     0,
	}

	provider, err := NewDatabaseProvider(config)
	require.NoError(t, err, "Failed to create test database")
	require.NotNil(t, provider, "Provider should not be nil")

	return provider, dbPath
}

// Helper function to verify schema exists
func verifySchema(t *testing.T, db *sql.DB) {
	t.Helper()

	// Check if logs table exists
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='logs'").Scan(&tableName)
	require.NoError(t, err, "Logs table should exist")
	assert.Equal(t, "logs", tableName)

	// Verify columns
	rows, err := db.Query("PRAGMA table_info(logs)")
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

	expectedColumns := []string{"id", "timestamp", "level", "component", "message", "fields"}
	for _, col := range expectedColumns {
		assert.True(t, columns[col], "Column %s should exist", col)
	}
}

// Helper function to count log entries
func countLogs(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	require.NoError(t, err)
	return count
}

func TestNewDatabaseProvider_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     0,
	}

	provider, err := NewDatabaseProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	defer provider.Close()

	// Verify database file was created
	assert.FileExists(t, dbPath)

	// Verify provider fields
	assert.Equal(t, config, provider.config)
	assert.NotNil(t, provider.db)

	// Verify schema
	verifySchema(t, provider.db)
}

func TestNewDatabaseProvider_SchemaInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     0,
	}

	provider, err := NewDatabaseProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	// Verify all expected columns and indexes
	verifySchema(t, provider.db)

	// Verify timestamp index exists
	var indexName string
	err = provider.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_logs_timestamp'").Scan(&indexName)
	require.NoError(t, err)
	assert.Equal(t, "idx_logs_timestamp", indexName)

	// Verify level index exists
	err = provider.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_logs_level'").Scan(&indexName)
	require.NoError(t, err)
	assert.Equal(t, "idx_logs_level", indexName)

	// Verify component index exists
	err = provider.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_logs_component'").Scan(&indexName)
	require.NoError(t, err)
	assert.Equal(t, "idx_logs_component", indexName)

	// Verify plugin_metadata table exists
	var metadataTable string
	err = provider.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='plugin_metadata'").Scan(&metadataTable)
	require.NoError(t, err)
	assert.Equal(t, "plugin_metadata", metadataTable)
}

func TestNewDatabaseProvider_InvalidPath(t *testing.T) {
	// Try to create database in non-existent directory without permissions
	invalidPath := "/root/nonexistent/dir/test.db"

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      invalidPath,
		LogStorageEnabled: true,
		MaxLogEntries:     0,
	}

	provider, err := NewDatabaseProvider(config)
	assert.Error(t, err)
	assert.Nil(t, provider)
}

func TestNewDatabaseProvider_ExistingDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     0,
	}

	// Create first provider
	provider1, err := NewDatabaseProvider(config)
	require.NoError(t, err)
	provider1.Close()

	// Open same database again
	provider2, err := NewDatabaseProvider(config)
	require.NoError(t, err)
	defer provider2.Close()

	assert.NotNil(t, provider2)
	verifySchema(t, provider2.db)
}

func TestInsertLog_AllFields(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	timestamp := time.Now().Format(time.RFC3339)
	level := "info"
	component := "daemon"
	message := "Test message"
	fields := `{"key":"value"}`

	err := provider.InsertLog(timestamp, level, component, message, fields)
	require.NoError(t, err)

	// Verify entry was inserted
	assert.Equal(t, 1, countLogs(t, provider.db))

	// Verify entry content
	var ts, lvl, comp, msg, flds string
	err = provider.db.QueryRow("SELECT timestamp, level, component, message, fields FROM logs").Scan(
		&ts, &lvl, &comp, &msg, &flds,
	)
	require.NoError(t, err)
	assert.Equal(t, timestamp, ts)
	assert.Equal(t, level, lvl)
	assert.Equal(t, component, comp)
	assert.Equal(t, message, msg)
	assert.Equal(t, fields, flds)
}

func TestInsertLog_EmptyFields(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	timestamp := time.Now().Format(time.RFC3339)
	err := provider.InsertLog(timestamp, "warn", "system", "Warning without fields", "")
	require.NoError(t, err)

	// Verify entry
	var fields string
	err = provider.db.QueryRow("SELECT fields FROM logs").Scan(&fields)
	require.NoError(t, err)
	assert.Equal(t, "", fields)
}

func TestInsertLog_LogStorageDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := &DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: false,
		MaxLogEntries:     0,
	}

	provider, err := NewDatabaseProvider(config)
	require.NoError(t, err)
	defer provider.Close()

	timestamp := time.Now().Format(time.RFC3339)
	err = provider.InsertLog(timestamp, "info", "test", "Should not be stored", "")
	require.NoError(t, err)

	// Verify no logs were inserted
	assert.Equal(t, 0, countLogs(t, provider.db))
}

func TestInsertLog_SpecialCharacters(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	timestamp := time.Now().Format(time.RFC3339)
	message := "Message with special chars: ' \" \\ \n \t"
	fields := `{"error":"SQL injection'; DROP TABLE logs;--"}`

	err := provider.InsertLog(timestamp, "error", "test", message, fields)
	require.NoError(t, err)

	// Verify entry was inserted correctly (SQL injection should be escaped)
	assert.Equal(t, 1, countLogs(t, provider.db))

	var msg string
	err = provider.db.QueryRow("SELECT message FROM logs").Scan(&msg)
	require.NoError(t, err)
	assert.Equal(t, message, msg)
}

func TestQueryLogs_AllLogs(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	// Insert test entries
	entries := []struct {
		timestamp, level, component, message string
	}{
		{"2025-01-01T10:00:00Z", "info", "daemon", "Msg 1"},
		{"2025-01-01T10:01:00Z", "warn", "system", "Msg 2"},
		{"2025-01-01T10:02:00Z", "error", "plugin", "Msg 3"},
	}
	for _, entry := range entries {
		provider.InsertLog(entry.timestamp, entry.level, entry.component, entry.message, "")
	}

	logs, err := provider.QueryLogs("", "", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestQueryLogs_FilterByLevel(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	entries := []struct {
		timestamp, level, component, message string
	}{
		{"2025-01-01T10:00:00Z", "info", "test", "Info"},
		{"2025-01-01T10:01:00Z", "error", "test", "Error"},
		{"2025-01-01T10:02:00Z", "info", "test", "Info 2"},
	}
	for _, entry := range entries {
		provider.InsertLog(entry.timestamp, entry.level, entry.component, entry.message, "")
	}

	logs, err := provider.QueryLogs("error", "", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "error", logs[0].Level)
	assert.Equal(t, "Error", logs[0].Message)
}

func TestQueryLogs_FilterByComponent(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	entries := []struct {
		timestamp, level, component, message string
	}{
		{"2025-01-01T10:00:00Z", "info", "daemon", "Msg 1"},
		{"2025-01-01T10:01:00Z", "info", "system", "Msg 2"},
		{"2025-01-01T10:02:00Z", "info", "daemon", "Msg 3"},
	}
	for _, entry := range entries {
		provider.InsertLog(entry.timestamp, entry.level, entry.component, entry.message, "")
	}

	logs, err := provider.QueryLogs("", "daemon", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
	for _, log := range logs {
		assert.Equal(t, "daemon", log.Component)
	}
}

func TestQueryLogs_Limit(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	// Insert 10 entries
	for i := 0; i < 10; i++ {
		timestamp := time.Now().Format(time.RFC3339)
		provider.InsertLog(timestamp, "info", "test", "Message", "")
	}

	logs, err := provider.QueryLogs("", "", 5)
	require.NoError(t, err)
	assert.Len(t, logs, 5)
}

func TestQueryLogs_OrderByTimestampDesc(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	entries := []struct {
		timestamp, level, component, message string
	}{
		{"2025-01-01T10:00:00Z", "info", "test", "First"},
		{"2025-01-01T10:02:00Z", "info", "test", "Third"},
		{"2025-01-01T10:01:00Z", "info", "test", "Second"},
	}
	for _, entry := range entries {
		provider.InsertLog(entry.timestamp, entry.level, entry.component, entry.message, "")
	}

	logs, err := provider.QueryLogs("", "", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)

	// Should be ordered by timestamp descending (newest first)
	assert.Equal(t, "Third", logs[0].Message)
	assert.Equal(t, "Second", logs[1].Message)
	assert.Equal(t, "First", logs[2].Message)
}

func TestQueryLogs_EmptyResult(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	logs, err := provider.QueryLogs("", "", 0)
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestStats_EmptyDatabase(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	stats, err := provider.Stats()
	require.NoError(t, err)
	assert.Equal(t, 0, stats["log_count"])
	assert.NotNil(t, stats["database_path"])
	assert.Greater(t, stats["database_size_bytes"].(int64), int64(0))
}

func TestStats_WithLogs(t *testing.T) {
	provider, dbPath := setupTestDB(t)
	defer provider.Close()

	// Insert test entries
	for i := 0; i < 5; i++ {
		timestamp := time.Now().Format(time.RFC3339)
		provider.InsertLog(timestamp, "info", "test", "Message", "")
	}

	stats, err := provider.Stats()
	require.NoError(t, err)
	assert.Equal(t, 5, stats["log_count"])
	assert.Equal(t, dbPath, stats["database_path"])
	assert.Greater(t, stats["database_size_bytes"].(int64), int64(0))
}

func TestVacuum_Success(t *testing.T) {
	provider, _ := setupTestDB(t)
	defer provider.Close()

	// Insert and delete some entries to create fragmentation
	for i := 0; i < 100; i++ {
		timestamp := time.Now().Format(time.RFC3339)
		provider.InsertLog(timestamp, "info", "test", "Message", "")
	}

	_, err := provider.db.Exec("DELETE FROM logs")
	require.NoError(t, err)

	// Vacuum should succeed
	err = provider.Vacuum()
	require.NoError(t, err)
}

func TestClose_Success(t *testing.T) {
	provider, _ := setupTestDB(t)

	err := provider.Close()
	require.NoError(t, err)

	// Subsequent operations should fail
	timestamp := time.Now().Format(time.RFC3339)
	err = provider.InsertLog(timestamp, "info", "test", "Should fail", "")
	assert.Error(t, err)
}

func TestClose_NilDB(t *testing.T) {
	provider := &DatabaseProvider{
		db: nil,
	}

	err := provider.Close()
	assert.NoError(t, err, "Closing nil db should not error")
}
