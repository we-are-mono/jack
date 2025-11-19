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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	resp, err := provider.Metadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "sqlite3", resp.Namespace)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "/etc/jack/sqlite3.json", resp.ConfigPath)
	assert.NotEmpty(t, resp.Description)
	assert.Equal(t, "database", resp.Category)
	assert.NotNil(t, resp.DefaultConfig)

	// Verify default config structure
	assert.Equal(t, true, resp.DefaultConfig["enabled"])
	assert.Equal(t, "/var/lib/jack/data.db", resp.DefaultConfig["database_path"])
	assert.Equal(t, true, resp.DefaultConfig["log_storage_enabled"])
	assert.Equal(t, 100000, resp.DefaultConfig["max_log_entries"])

	// Verify CLI commands
	assert.Len(t, resp.CLICommands, 1)
	assert.Equal(t, "sqlite3", resp.CLICommands[0].Name)
	assert.Contains(t, resp.CLICommands[0].Subcommands, "stats")
	assert.Contains(t, resp.CLICommands[0].Subcommands, "logs")
	assert.Contains(t, resp.CLICommands[0].Subcommands, "vacuum")
	assert.False(t, resp.CLICommands[0].Continuous)
}

func TestApplyConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	provider := &SQLite3RPCProvider{}
	err = provider.ApplyConfig(context.Background(), configJSON)
	require.NoError(t, err)

	// Verify provider was created
	assert.NotNil(t, provider.provider)
	assert.Equal(t, dbPath, provider.provider.config.DatabasePath)
	assert.Equal(t, dbPath, provider.lastPath)

	// Cleanup
	provider.provider.Close()
}

func TestApplyConfig_InvalidJSON(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	err := provider.ApplyConfig(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
}

func TestApplyConfig_InvalidDatabasePath(t *testing.T) {
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      "/proc/test.db", // Read-only filesystem, fails even for root
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	provider := &SQLite3RPCProvider{}
	err = provider.ApplyConfig(context.Background(), configJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create database provider")
}

func TestApplyConfig_ReplacesExistingProvider(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath1 := filepath.Join(tmpDir, "test1.db")
	dbPath2 := filepath.Join(tmpDir, "test2.db")

	// Apply first config
	config1 := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath1,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON1, _ := json.Marshal(config1)

	provider := &SQLite3RPCProvider{}
	err := provider.ApplyConfig(context.Background(), configJSON1)
	require.NoError(t, err)
	assert.Equal(t, dbPath1, provider.provider.config.DatabasePath)
	assert.Equal(t, dbPath1, provider.lastPath)

	// Apply second config with different path (should close first and create new)
	config2 := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath2,
		LogStorageEnabled: true,
		MaxLogEntries:     200,
	}
	configJSON2, _ := json.Marshal(config2)

	err = provider.ApplyConfig(context.Background(), configJSON2)
	require.NoError(t, err)
	assert.Equal(t, dbPath2, provider.provider.config.DatabasePath)
	assert.Equal(t, dbPath2, provider.lastPath)

	// Cleanup
	provider.provider.Close()
}

func TestApplyConfig_SamePathKeepsProviderOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Apply first config
	config1 := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON1, _ := json.Marshal(config1)

	provider := &SQLite3RPCProvider{}
	err := provider.ApplyConfig(context.Background(), configJSON1)
	require.NoError(t, err)

	firstProvider := provider.provider

	// Apply second config with same path (should keep provider open)
	config2 := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     200,
	}
	configJSON2, _ := json.Marshal(config2)

	err = provider.ApplyConfig(context.Background(), configJSON2)
	require.NoError(t, err)

	// Should be the same provider instance
	assert.Equal(t, firstProvider, provider.provider)

	// Cleanup
	provider.provider.Close()
}

func TestValidateConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	provider := &SQLite3RPCProvider{}
	err = provider.ValidateConfig(context.Background(), configJSON)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	err := provider.ValidateConfig(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
}

func TestValidateConfig_EmptyDatabasePath(t *testing.T) {
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      "",
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	provider := &SQLite3RPCProvider{}
	err = provider.ValidateConfig(context.Background(), configJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database_path is required")
}

func TestFlush_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create provider
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	err := provider.ApplyConfig(context.Background(), configJSON)
	require.NoError(t, err)

	// Flush should close the provider
	err = provider.Flush(context.Background())
	require.NoError(t, err)
}

func TestFlush_NilProvider(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	err := provider.Flush(context.Background())
	assert.NoError(t, err, "Flushing nil provider should not error")
}

func TestStatus_ProviderActive(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create and apply config
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	err := provider.ApplyConfig(context.Background(), configJSON)
	require.NoError(t, err)

	// Insert some test logs
	timestamp := time.Now().Format(time.RFC3339)
	provider.provider.InsertLog(timestamp, "info", "test", "Test message 1", "")
	provider.provider.InsertLog(timestamp, "error", "test", "Test message 2", "")

	// Get status
	statusJSON, err := provider.Status(context.Background())
	require.NoError(t, err)

	var status map[string]interface{}
	err = json.Unmarshal(statusJSON, &status)
	require.NoError(t, err)

	assert.Equal(t, true, status["enabled"])
	assert.Equal(t, dbPath, status["database_path"])
	assert.Equal(t, float64(2), status["log_count"])

	// Cleanup
	provider.provider.Close()
}

func TestStatus_ProviderNotInitialized(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	statusJSON, err := provider.Status(context.Background())
	require.NoError(t, err)

	var status map[string]interface{}
	err = json.Unmarshal(statusJSON, &status)
	require.NoError(t, err)

	assert.Equal(t, false, status["enabled"])
	assert.Equal(t, "Database not initialized", status["message"])
}

func TestExecuteCLICommand_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Setup provider with some logs
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert test logs
	timestamp := time.Now().Format(time.RFC3339)
	provider.provider.InsertLog(timestamp, "info", "test", "Info message", "")
	provider.provider.InsertLog(timestamp, "error", "test", "Error message", "")

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 stats", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Database Statistics")
	assert.Contains(t, outputStr, "Path:")
	assert.Contains(t, outputStr, dbPath)
	assert.Contains(t, outputStr, "Log entries: 2")
}

func TestExecuteCLICommand_StatsNotInitialized(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 stats", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Database not initialized")
}

func TestExecuteCLICommand_Logs_AllLogs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Setup provider
	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert test logs
	provider.provider.InsertLog("2025-01-01T10:00:00Z", "info", "daemon", "Test message", "")

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Found 1 log entries")
	assert.Contains(t, outputStr, "2025-01-01T10:00:00Z")
	assert.Contains(t, outputStr, "INFO")
	assert.Contains(t, outputStr, "daemon")
	assert.Contains(t, outputStr, "Test message")
}

func TestExecuteCLICommand_Logs_WithLevelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert test logs
	provider.provider.InsertLog("2025-01-01T10:00:00Z", "info", "daemon", "Info message", "")
	provider.provider.InsertLog("2025-01-01T10:01:00Z", "error", "system", "Error message", "")

	// Test level filter
	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{"--level", "error"})
	require.NoError(t, err)
	outputStr := string(output)
	assert.Contains(t, outputStr, "ERROR")
	assert.Contains(t, outputStr, "Error message")
	assert.NotContains(t, outputStr, "Info message")
}

func TestExecuteCLICommand_Logs_WithComponentFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert test logs
	provider.provider.InsertLog("2025-01-01T10:00:00Z", "info", "daemon", "Daemon message", "")
	provider.provider.InsertLog("2025-01-01T10:01:00Z", "info", "system", "System message", "")

	// Test component filter
	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{"--component", "daemon"})
	require.NoError(t, err)
	outputStr := string(output)
	assert.Contains(t, outputStr, "daemon")
	assert.Contains(t, outputStr, "Daemon message")
	assert.NotContains(t, outputStr, "System message")
}

func TestExecuteCLICommand_Logs_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert test logs
	for i := 0; i < 10; i++ {
		timestamp := time.Now().Format(time.RFC3339)
		provider.provider.InsertLog(timestamp, "info", "test", "Message", "")
	}

	// Test limit
	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{"--limit", "3"})
	require.NoError(t, err)
	outputStr := string(output)
	assert.Contains(t, outputStr, "Found 3 log entries")
}

func TestExecuteCLICommand_Logs_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "No logs found")
}

func TestExecuteCLICommand_Vacuum(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert and delete some data to create fragmentation
	for i := 0; i < 10; i++ {
		timestamp := time.Now().Format(time.RFC3339)
		provider.provider.InsertLog(timestamp, "info", "test", "Message", "")
	}

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 vacuum", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Database vacuum completed successfully")
	assert.Contains(t, outputStr, "Size before:")
	assert.Contains(t, outputStr, "Size after:")
}

func TestExecuteCLICommand_VacuumNotInitialized(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 vacuum", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Database not initialized")
}

func TestExecuteCLICommand_UnknownSubcommand(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	_, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 unknown", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown subcommand")
}

func TestExecuteCLICommand_InvalidCommandFormat(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	_, err := provider.ExecuteCLICommand(context.Background(), "sqlite3", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid command format")
}

func TestOnLogEvent_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Create log event as it would come from daemon
	logEvent := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "info",
		"component": "daemon",
		"message":   "Test log event",
		"fields": map[string]interface{}{
			"key": "value",
		},
	}
	logEventJSON, _ := json.Marshal(logEvent)

	err := provider.OnLogEvent(context.Background(), logEventJSON)
	require.NoError(t, err)

	// Verify log was inserted
	logs, err := provider.provider.QueryLogs("", "", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "Test log event", logs[0].Message)
	assert.Contains(t, logs[0].Fields, "key")
}

func TestOnLogEvent_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	err := provider.OnLogEvent(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal log event")
}

func TestOnLogEvent_NilProvider(t *testing.T) {
	provider := &SQLite3RPCProvider{}

	logEvent := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "info",
		"component": "test",
		"message":   "Test",
	}
	logEventJSON, _ := json.Marshal(logEvent)

	err := provider.OnLogEvent(context.Background(), logEventJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database not initialized")
}

func TestOnLogEvent_WithFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Create log event with complex fields
	logEvent := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     "error",
		"component": "system",
		"message":   "Error occurred",
		"fields": map[string]interface{}{
			"error_code": 500,
			"details":    "Something went wrong",
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}
	logEventJSON, _ := json.Marshal(logEvent)

	err := provider.OnLogEvent(context.Background(), logEventJSON)
	require.NoError(t, err)

	// Verify log was inserted with fields
	logs, err := provider.provider.QueryLogs("", "", 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "Error occurred", logs[0].Message)

	// Verify fields were stored as JSON
	assert.Contains(t, logs[0].Fields, "error_code")
	assert.Contains(t, logs[0].Fields, "details")
	assert.Contains(t, logs[0].Fields, "nested")
}

func TestExecuteCLICommand_Logs_WithFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert log with fields
	provider.provider.InsertLog("2025-01-01T10:00:00Z", "info", "daemon", "Test with fields", `{"key":"value"}`)

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{})
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "Test with fields")
	assert.Contains(t, outputStr, "Fields:")
	assert.Contains(t, outputStr, `{"key":"value"}`)
}

func TestExecuteCLICommand_Logs_SkipsNullFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := DatabaseConfig{
		Enabled:           true,
		DatabasePath:      dbPath,
		LogStorageEnabled: true,
		MaxLogEntries:     100,
	}
	configJSON, _ := json.Marshal(config)

	provider := &SQLite3RPCProvider{}
	provider.ApplyConfig(context.Background(), configJSON)
	defer provider.provider.Close()

	// Insert logs with different field values
	provider.provider.InsertLog("2025-01-01T10:00:00Z", "info", "daemon", "No fields", "")
	provider.provider.InsertLog("2025-01-01T10:01:00Z", "info", "daemon", "Null fields", "null")
	provider.provider.InsertLog("2025-01-01T10:02:00Z", "info", "daemon", "With fields", `{"key":"value"}`)

	output, err := provider.ExecuteCLICommand(context.Background(), "sqlite3 logs", []string{})
	require.NoError(t, err)

	outputStr := string(output)

	// Count how many times "Fields:" appears (should only be once for the log with actual fields)
	fieldsCount := strings.Count(outputStr, "Fields:")
	assert.Equal(t, 1, fieldsCount)
}
