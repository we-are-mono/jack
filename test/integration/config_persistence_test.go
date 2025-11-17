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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/state"
	"github.com/we-are-mono/jack/types"
)

// TestConfigPersistence_LoadValidJSON tests loading valid JSON configuration
func TestConfigPersistence_LoadValidJSON(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Create a valid interfaces config
	configPath := filepath.Join(harness.configDir, "interfaces.json")
	validJSON := `{
  "interfaces": {
    "eth0": {
      "enabled": true,
      "type": "physical",
      "device": "eth0",
      "protocol": "static",
      "ipaddr": "192.168.1.1",
      "netmask": "255.255.255.0"
    }
  }
}`

	err := os.WriteFile(configPath, []byte(validJSON), 0644)
	require.NoError(t, err)

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	// Load the config
	var config types.InterfacesConfig
	err = state.LoadConfig("interfaces", &config)
	require.NoError(t, err)

	// Verify loaded data
	require.Contains(t, config.Interfaces, "eth0")
	iface := config.Interfaces["eth0"]
	assert.True(t, iface.Enabled)
	assert.Equal(t, "physical", iface.Type)
	assert.Equal(t, "eth0", iface.Device)
	assert.Equal(t, "static", iface.Protocol)
	assert.Equal(t, "192.168.1.1", iface.IPAddr)
	assert.Equal(t, "255.255.255.0", iface.Netmask)
}

// TestConfigPersistence_LoadMalformedJSON tests error reporting for malformed JSON
func TestConfigPersistence_LoadMalformedJSON(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	tests := []struct {
		name          string
		malformedJSON string
		expectedError string
	}{
		{
			name: "missing comma",
			malformedJSON: `{
  "eth0": {
    "enabled": true
    "type": "physical"
  }
}`,
			expectedError: "line 4",
		},
		{
			name: "trailing comma",
			malformedJSON: `{
  "eth0": {
    "enabled": true,
  }
}`,
			expectedError: "line 4",
		},
		{
			name: "unclosed brace",
			malformedJSON: `{
  "eth0": {
    "enabled": true
}`,
			expectedError: "line",
		},
		{
			name: "invalid value",
			malformedJSON: `{
  "eth0": {
    "enabled": invalid
  }
}`,
			expectedError: "line 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(harness.configDir, "test.json")
			err := os.WriteFile(configPath, []byte(tt.malformedJSON), 0644)
			require.NoError(t, err)

			// Override JACK_CONFIG_DIR for this test
			oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
			os.Setenv("JACK_CONFIG_DIR", harness.configDir)
			defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

			var config map[string]interface{}
			err = state.LoadConfig("test", &config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError, "Error should contain line number")
			assert.Contains(t, err.Error(), "column", "Error should contain column number")
		})
	}
}

// TestConfigPersistence_LoadMissingFile tests loading non-existent config file
func TestConfigPersistence_LoadMissingFile(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	var config types.InterfacesConfig
	err := state.LoadConfig("nonexistent", &config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestConfigPersistence_SaveAndBackup tests config save with backup creation
func TestConfigPersistence_SaveAndBackup(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	// Create initial config
	initialConfig := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {
				Enabled:  true,
				Type:     "physical",
				Device:   "eth0",
				Protocol: "dhcp",
			},
		},
	}

	err := state.SaveConfig("interfaces", initialConfig)
	require.NoError(t, err)

	configPath := filepath.Join(harness.configDir, "interfaces.json")
	assert.FileExists(t, configPath)

	// Verify initial content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "eth0")
	assert.Contains(t, string(data), "dhcp")

	// Wait a moment to ensure different timestamp
	time.Sleep(1 * time.Second)

	// Modify and save again (should create backup)
	modifiedConfig := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {
				Enabled:  true,
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				IPAddr:   "10.0.0.1",
				Netmask:  "255.255.255.0",
			},
		},
	}

	err = state.SaveConfig("interfaces", modifiedConfig)
	require.NoError(t, err)

	// Verify backup file was created
	files, err := filepath.Glob(filepath.Join(harness.configDir, "interfaces.json.backup.*"))
	require.NoError(t, err)
	require.Len(t, files, 1, "Should have created one backup file")

	// Verify backup contains original content
	backupData, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Contains(t, string(backupData), "dhcp", "Backup should contain original config")

	// Verify current file has new content
	currentData, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(currentData), "static", "Current file should have new config")
	assert.Contains(t, string(currentData), "10.0.0.1")
}

// TestConfigPersistence_AtomicWrite tests that SaveConfig uses atomic writes
func TestConfigPersistence_AtomicWrite(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	config := types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Enabled:     true,
				Destination: "default",
				Gateway:     "192.168.1.1",
				Metric:      100,
			},
		},
	}

	err := state.SaveConfig("routes", config)
	require.NoError(t, err)

	// Verify temp file was cleaned up
	tmpPath := filepath.Join(harness.configDir, "routes.json.tmp")
	assert.NoFileExists(t, tmpPath, "Temporary file should be removed after atomic write")

	// Verify final file exists
	finalPath := filepath.Join(harness.configDir, "routes.json")
	assert.FileExists(t, finalPath)

	// Verify content is valid JSON
	var loadedConfig types.RoutesConfig
	err = state.LoadConfig("routes", &loadedConfig)
	require.NoError(t, err)
	assert.Contains(t, loadedConfig.Routes, "default")
}

// TestConfigPersistence_SaveWithMissingDirectory tests save error handling
func TestConfigPersistence_SaveWithMissingDirectory(t *testing.T) {
	// Set JACK_CONFIG_DIR to a non-existent directory
	nonExistentDir := filepath.Join(os.TempDir(), "jack-test-nonexistent-"+time.Now().Format("20060102-150405"))
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", nonExistentDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	config := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {Enabled: true, Type: "physical"},
		},
	}

	err := state.SaveConfig("interfaces", config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write temp file")
}

// TestConfigPersistence_UnmarshalJSONWithErrors tests enhanced error reporting
func TestConfigPersistence_UnmarshalJSONWithErrors(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "syntax error",
			json:          `{"key": invalid}`,
			expectedError: "line 1",
		},
		{
			name:          "syntax error multiline",
			json:          "{\n  \"key1\": \"value1\",\n  \"key2\": invalid\n}",
			expectedError: "line 3",
		},
		{
			name:          "unclosed string",
			json:          `{"key": "value}`,
			expectedError: "line 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			err := state.UnmarshalJSON([]byte(tt.json), &result)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Contains(t, err.Error(), "column")
		})
	}
}

// TestConfigPersistence_UnmarshalJSONValid tests unmarshaling valid JSON
func TestConfigPersistence_UnmarshalJSONValid(t *testing.T) {
	validJSON := `{
  "eth0": {
    "enabled": true,
    "type": "physical"
  }
}`

	var result map[string]interface{}
	err := state.UnmarshalJSON([]byte(validJSON), &result)
	require.NoError(t, err)
	assert.Contains(t, result, "eth0")
}

// TestConfigPersistence_MultipleBackups tests that multiple saves create multiple backups
func TestConfigPersistence_MultipleBackups(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	// Create initial config
	config1 := map[string]interface{}{"version": 1}
	err := state.SaveConfig("test", config1)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Save again - creates first backup
	config2 := map[string]interface{}{"version": 2}
	err = state.SaveConfig("test", config2)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Save again - creates second backup
	config3 := map[string]interface{}{"version": 3}
	err = state.SaveConfig("test", config3)
	require.NoError(t, err)

	// Verify two backup files exist
	files, err := filepath.Glob(filepath.Join(harness.configDir, "test.json.backup.*"))
	require.NoError(t, err)
	assert.Len(t, files, 2, "Should have created two backup files")

	// Verify backups have different timestamps in filename
	assert.NotEqual(t, files[0], files[1], "Backup filenames should be unique")
}

// TestConfigPersistence_RoundTrip tests save and load round trip
func TestConfigPersistence_RoundTrip(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	// Create complex config
	originalConfig := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"br-lan": {
				Enabled:     true,
				Type:        "bridge",
				Device:      "br-lan",
				Protocol:    "static",
				IPAddr:      "192.168.1.1",
				Netmask:     "255.255.255.0",
				BridgePorts: []string{"lan1", "lan2", "lan3"},
				MTU:         1500,
			},
			"eth0.10": {
				Enabled:    true,
				Type:       "vlan",
				Device:     "eth0",
				DeviceName: "eth0.10",
				VLANId:     10,
				Protocol:   "static",
				IPAddr:     "10.0.10.1",
				Netmask:    "255.255.255.0",
			},
		},
	}

	// Save
	err := state.SaveConfig("interfaces", originalConfig)
	require.NoError(t, err)

	// Load
	var loadedConfig types.InterfacesConfig
	err = state.LoadConfig("interfaces", &loadedConfig)
	require.NoError(t, err)

	// Verify exact match
	require.Len(t, loadedConfig.Interfaces, 2)

	brLan := loadedConfig.Interfaces["br-lan"]
	assert.True(t, brLan.Enabled)
	assert.Equal(t, "bridge", brLan.Type)
	assert.Equal(t, "192.168.1.1", brLan.IPAddr)
	assert.Equal(t, []string{"lan1", "lan2", "lan3"}, brLan.BridgePorts)

	vlan := loadedConfig.Interfaces["eth0.10"]
	assert.Equal(t, "vlan", vlan.Type)
	assert.Equal(t, 10, vlan.VLANId)
	assert.Equal(t, "10.0.10.1", vlan.IPAddr)
}

// TestConfigPersistence_JSONFormatting tests that saved JSON is properly formatted
func TestConfigPersistence_JSONFormatting(t *testing.T) {
	harness := NewTestHarness(t)
	defer harness.Cleanup()

	// Override JACK_CONFIG_DIR for this test
	oldConfigDir := os.Getenv("JACK_CONFIG_DIR")
	os.Setenv("JACK_CONFIG_DIR", harness.configDir)
	defer os.Setenv("JACK_CONFIG_DIR", oldConfigDir)

	config := map[string]interface{}{
		"key1": "value1",
		"key2": map[string]string{
			"nested": "value",
		},
	}

	err := state.SaveConfig("formatted", config)
	require.NoError(t, err)

	// Read the file
	configPath := filepath.Join(harness.configDir, "formatted.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Verify it's indented (contains newlines and spaces)
	content := string(data)
	assert.Contains(t, content, "\n", "JSON should be formatted with newlines")
	assert.True(t, strings.Contains(content, "  "), "JSON should be indented")

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = state.UnmarshalJSON(data, &parsed)
	require.NoError(t, err)
}
