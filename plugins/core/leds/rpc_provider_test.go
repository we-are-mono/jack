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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
	provider := NewLEDRPCProvider()

	resp, err := provider.Metadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "led", resp.Namespace)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "/etc/jack/led.json", resp.ConfigPath)
	assert.NotEmpty(t, resp.Description)
	assert.Equal(t, "hardware", resp.Category)
	assert.Equal(t, "leds", resp.PathPrefix)
	assert.Nil(t, resp.DefaultConfig)

	// Verify CLI commands
	assert.Len(t, resp.CLICommands, 1)
	assert.Equal(t, "led", resp.CLICommands[0].Name)
	assert.Contains(t, resp.CLICommands[0].Subcommands, "status")
	assert.Contains(t, resp.CLICommands[0].Subcommands, "list")
	assert.False(t, resp.CLICommands[0].Continuous)
}

func TestApplyConfig_InvalidJSON(t *testing.T) {
	provider := NewLEDRPCProvider()

	err := provider.ApplyConfig(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	provider := NewLEDRPCProvider()

	err := provider.ValidateConfig(context.Background(), []byte("invalid json"))
	assert.Error(t, err)
}

func TestValidateConfig_NilConfig(t *testing.T) {
	provider := NewLEDRPCProvider()

	// This will unmarshal to an empty config, which should validate as OK (no LEDs configured)
	configJSON, _ := json.Marshal(map[string]interface{}{})
	err := provider.ValidateConfig(context.Background(), configJSON)
	// Empty config should be valid (just means no LEDs are configured)
	assert.NoError(t, err)
}

func TestOnLogEvent_NotImplemented(t *testing.T) {
	provider := NewLEDRPCProvider()

	logEvent := map[string]interface{}{
		"timestamp": "2025-01-01T10:00:00Z",
		"level":     "info",
		"component": "test",
		"message":   "Test",
	}
	logEventJSON, _ := json.Marshal(logEvent)

	err := provider.OnLogEvent(context.Background(), logEventJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement log event handling")
}

func TestExecuteCLICommand_EmptyCommand(t *testing.T) {
	provider := NewLEDRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}

func TestExecuteCLICommand_WrongCommand(t *testing.T) {
	provider := NewLEDRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "notled", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestExecuteCLICommand_NoSubcommand(t *testing.T) {
	provider := NewLEDRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "led", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires subcommand")
}

func TestExecuteCLICommand_UnknownSubcommand(t *testing.T) {
	provider := NewLEDRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "led unknown", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown led subcommand")
}

func TestNewLEDProvider(t *testing.T) {
	provider := NewLEDProvider()

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.appliedLEDs)
	assert.NotNil(t, provider.originalState)
	assert.Empty(t, provider.appliedLEDs)
	assert.Empty(t, provider.originalState)
}

func TestNewLEDRPCProvider(t *testing.T) {
	provider := NewLEDRPCProvider()

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.provider)
}

// Note: The following tests cannot run without actual LED hardware or a mock filesystem:
// - TestApplyConfig (requires LEDExists, GetBrightness, SetBrightness, etc.)
// - TestValidateConfig with actual LEDs (requires LEDExists, GetMaxBrightness, GetAvailableTriggers)
// - TestFlush (requires SetTrigger, SetBrightness)
// - TestStatus (requires ListLEDs, GetBrightness, GetMaxBrightness, GetTrigger, GetAvailableTriggers)
// - TestExecuteCLICommand_Status (requires Status() to work, which needs filesystem)
// - TestExecuteCLICommand_List (requires Status() to work, which needs filesystem)
//
// These require integration tests on actual hardware with LEDs or a comprehensive mock filesystem.
// The LED plugin achieves ~7.5% coverage with unit tests alone because most of the business logic
// depends on filesystem operations. Full coverage requires integration tests with LED hardware.
