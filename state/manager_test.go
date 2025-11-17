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

package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tempConfigDir creates a temporary directory for test configuration files
func tempConfigDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "jack-state-test-*")
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// TestLoadConfig tests loading configuration from JSON files
func TestLoadConfig(t *testing.T) {
	dir := tempConfigDir(t)

	// Set config dir for testing
	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Create a test config file
	testConfig := map[string]interface{}{
		"test_field": "test_value",
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	data, err := json.MarshalIndent(testConfig, "", "  ")
	require.NoError(t, err)

	configPath := filepath.Join(dir, "test.json")
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	// Load the config
	var loaded map[string]interface{}
	err = LoadConfig("test", &loaded)
	require.NoError(t, err)

	assert.Equal(t, "test_value", loaded["test_field"])
	assert.Equal(t, "value", loaded["nested"].(map[string]interface{})["key"])
}

// TestLoadConfigFileNotFound tests loading non-existent config
func TestLoadConfigFileNotFound(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	var config map[string]interface{}
	err := LoadConfig("nonexistent", &config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestLoadConfigInvalidJSON tests loading malformed JSON
func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Write invalid JSON
	invalidJSON := []byte(`{"test": "value",}`) // Trailing comma
	configPath := filepath.Join(dir, "invalid.json")
	err := os.WriteFile(configPath, invalidJSON, 0644)
	require.NoError(t, err)

	var config map[string]interface{}
	err = LoadConfig("invalid", &config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

// TestSaveConfig tests saving configuration to JSON files
func TestSaveConfig(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Save a config
	testConfig := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": map[string]interface{}{
			"nested": true,
		},
	}

	err := SaveConfig("test", testConfig)
	require.NoError(t, err)

	// Verify file was created
	configPath := filepath.Join(dir, "test.json")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load it back and verify
	var loaded map[string]interface{}
	err = LoadConfig("test", &loaded)
	require.NoError(t, err)

	assert.Equal(t, "value1", loaded["field1"])
	assert.Equal(t, float64(42), loaded["field2"]) // JSON numbers are float64
	assert.True(t, loaded["field3"].(map[string]interface{})["nested"].(bool))
}

// TestSaveConfigAtomicWrite tests that configs are written atomically
func TestSaveConfigAtomicWrite(t *testing.T) {
	dir := tempConfigDir(t)

	origDir := GetConfigDir()
	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() {
		os.Setenv("JACK_CONFIG_DIR", "")
		_ = origDir // Silence unused warning
	})

	// Write initial config
	initial := map[string]interface{}{"version": 1}
	err := SaveConfig("test", initial)
	require.NoError(t, err)

	// Write new config
	updated := map[string]interface{}{"version": 2}
	err = SaveConfig("test", updated)
	require.NoError(t, err)

	// Verify the file contains the updated config (not corrupted)
	var loaded map[string]interface{}
	err = LoadConfig("test", &loaded)
	require.NoError(t, err)

	assert.Equal(t, float64(2), loaded["version"])

	// Verify no temp files left behind
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		// Should not have .tmp files
		assert.NotContains(t, entry.Name(), ".tmp")
	}
}

// TestSaveConfigAutoBackup tests that SaveConfig creates automatic backups
func TestSaveConfigAutoBackup(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Create initial config
	original := map[string]interface{}{"data": "original"}
	err := SaveConfig("test", original)
	require.NoError(t, err)

	// Update the config (should create backup)
	updated := map[string]interface{}{"data": "updated"}
	err = SaveConfig("test", updated)
	require.NoError(t, err)

	// Verify backup file exists
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	backupFound := false
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" && contains(entry.Name(), "test.json.backup") {
			backupFound = true

			// Verify backup contains original data
			backupData, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			require.NoError(t, err)

			var backupConfig map[string]interface{}
			err = json.Unmarshal(backupData, &backupConfig)
			require.NoError(t, err)

			assert.Equal(t, "original", backupConfig["data"])
			break
		}
	}

	assert.True(t, backupFound, "Backup file should have been created")

	// Verify current config has updated data
	var current map[string]interface{}
	err = LoadConfig("test", &current)
	require.NoError(t, err)
	assert.Equal(t, "updated", current["data"])
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGetConfigDir tests config directory retrieval
func TestGetConfigDir(t *testing.T) {
	// Test default value
	os.Setenv("JACK_CONFIG_DIR", "")
	dir := GetConfigDir()
	assert.NotEmpty(t, dir)
	assert.True(t, filepath.IsAbs(dir))

	// Test environment variable override
	customDir := "/tmp/custom-jack-config"
	os.Setenv("JACK_CONFIG_DIR", customDir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	dir = GetConfigDir()
	assert.Equal(t, customDir, dir)
}

// TestConfigFilePermissions tests that config files are created with correct permissions
func TestConfigFilePermissions(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	config := map[string]interface{}{"secret": "password"}
	err := SaveConfig("test", config)
	require.NoError(t, err)

	configPath := filepath.Join(dir, "test.json")
	info, err := os.Stat(configPath)
	require.NoError(t, err)

	// File should be readable/writable by owner only (0600 in manager.go)
	mode := info.Mode()
	assert.Equal(t, os.FileMode(0600), mode.Perm())
}

// TestGetLineCol tests line and column calculation for JSON error reporting
func TestGetLineCol(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		offset   int64
		wantLine int
		wantCol  int
	}{
		{
			name:     "first character",
			data:     "hello",
			offset:   0,
			wantLine: 1,
			wantCol:  1,
		},
		{
			name:     "middle of first line",
			data:     "hello world",
			offset:   6,
			wantLine: 1,
			wantCol:  7,
		},
		{
			name:     "after newline",
			data:     "line 1\nline 2",
			offset:   7,
			wantLine: 2,
			wantCol:  1,
		},
		{
			name:     "middle of second line",
			data:     "line 1\nline 2",
			offset:   10,
			wantLine: 2,
			wantCol:  4,
		},
		{
			name:     "multiple lines",
			data:     "{\n  \"test\": \"value\"\n}",
			offset:   15,
			wantLine: 2,
			wantCol:  14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := getLineCol([]byte(tt.data), tt.offset)
			assert.Equal(t, tt.wantLine, line, "line number mismatch")
			assert.Equal(t, tt.wantCol, col, "column number mismatch")
		})
	}
}

// TestUnmarshalJSON tests enhanced JSON unmarshaling with error reporting
func TestUnmarshalJSON(t *testing.T) {
	type TestConfig struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name      string
		data      string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid JSON",
			data:      `{"name": "test", "value": 42}`,
			wantError: false,
		},
		{
			name:      "syntax error with line/col info",
			data:      `{"name": "test", "value": }`,
			wantError: true,
			errorMsg:  "JSON syntax error",
		},
		{
			name:      "type mismatch",
			data:      `{"name": "test", "value": "not-a-number"}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config TestConfig
			err := UnmarshalJSON([]byte(tt.data), &config)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCopyFile tests file copying functionality
func TestCopyFile(t *testing.T) {
	dir := tempConfigDir(t)

	t.Run("successful copy", func(t *testing.T) {
		srcPath := filepath.Join(dir, "source.txt")
		dstPath := filepath.Join(dir, "dest.txt")

		// Create source file
		testData := []byte("test content")
		err := os.WriteFile(srcPath, testData, 0600)
		require.NoError(t, err)

		// Copy file
		err = copyFile(srcPath, dstPath)
		require.NoError(t, err)

		// Verify destination exists
		_, err = os.Stat(dstPath)
		require.NoError(t, err)

		// Verify content matches
		dstData, err := os.ReadFile(dstPath)
		require.NoError(t, err)
		assert.Equal(t, testData, dstData)
	})

	t.Run("source file not found", func(t *testing.T) {
		srcPath := filepath.Join(dir, "nonexistent.txt")
		dstPath := filepath.Join(dir, "dest2.txt")

		err := copyFile(srcPath, dstPath)
		assert.Error(t, err)
	})
}

// TestLoadConfigWithSyntaxError tests enhanced error reporting for JSON syntax errors
func TestLoadConfigWithSyntaxError(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Create config with syntax error
	invalidJSON := []byte(`{
  "field1": "value1",
  "field2":
}`)
	configPath := filepath.Join(dir, "syntaxerror.json")
	err := os.WriteFile(configPath, invalidJSON, 0644)
	require.NoError(t, err)

	// Try to load it
	var config map[string]interface{}
	err = LoadConfig("syntaxerror", &config)
	require.Error(t, err)

	// Check error message contains line and column info
	assert.Contains(t, err.Error(), "line")
	assert.Contains(t, err.Error(), "column")
}

// TestSaveConfigInvalidData tests saving unmarshalable data
func TestSaveConfigInvalidData(t *testing.T) {
	dir := tempConfigDir(t)

	os.Setenv("JACK_CONFIG_DIR", dir)
	t.Cleanup(func() { os.Setenv("JACK_CONFIG_DIR", "") })

	// Try to save a channel (not JSON-marshalable)
	invalidData := make(chan int)
	err := SaveConfig("invalid", invalidData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal")
}
