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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/we-are-mono/jack/types"
)

// TestGetDefaultJackConfig tests default Jack config generation
func TestGetDefaultJackConfig(t *testing.T) {
	config := getDefaultJackConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, 3, len(config.Plugins))

	// Check default plugins (by namespace, not binary name)
	assert.Contains(t, config.Plugins, "nftables")
	assert.Contains(t, config.Plugins, "dnsmasq")
	assert.Contains(t, config.Plugins, "monitoring")
}

// TestLoadJackConfigNonExistent tests loading jack config when file doesn't exist
func TestLoadJackConfigNonExistent(t *testing.T) {
	// Use a non-existent directory
	t.Setenv("JACK_CONFIG_DIR", "/tmp/nonexistent-jack-config-"+t.Name())

	config, err := LoadJackConfig()
	assert.NoError(t, err, "Should return default config when file doesn't exist")
	assert.NotNil(t, config)
	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, 3, len(config.Plugins))
}

// TestSaveJackConfig tests saving jack config
func TestSaveJackConfig(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	t.Setenv("JACK_CONFIG_DIR", tmpDir)

	// Create a test config
	testConfig := getDefaultJackConfig()
	testConfig.Version = "2.0"
	testConfig.Plugins["test-plugin"] = types.PluginState{
		Enabled: true,
		Version: "1.2.3",
	}

	// Save it
	err := SaveJackConfig(testConfig)
	assert.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(tmpDir + "/jack.json")
	assert.NoError(t, err, "jack.json should be created")
}
