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
	provider := NewFirewallLoggingRPCProvider()

	metadata, err := provider.Metadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "firewall-logging", metadata.Namespace)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, "firewall", metadata.Category)
	assert.Equal(t, "/etc/jack/firewall-logging.json", metadata.ConfigPath)
	assert.Contains(t, metadata.Dependencies, "sqlite3")

	// Verify default config
	assert.NotNil(t, metadata.DefaultConfig)
	assert.Equal(t, true, metadata.DefaultConfig["enabled"])
	assert.Equal(t, false, metadata.DefaultConfig["log_accepts"])
	assert.Equal(t, true, metadata.DefaultConfig["log_drops"])

	// Verify CLI commands
	require.Len(t, metadata.CLICommands, 1)
	assert.Equal(t, "firewall", metadata.CLICommands[0].Name)
	assert.Contains(t, metadata.CLICommands[0].Subcommands, "logs")
	assert.Contains(t, metadata.CLICommands[0].Subcommands, "stats")
}

func TestValidateConfig_Valid(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	config := &FirewallLoggingConfig{
		Enabled:          true,
		LogAccepts:       false,
		LogDrops:         true,
		SamplingRate:     1,
		RateLimitPerSec:  100,
		MaxLogEntries:    50000,
		RetentionDays:    7,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	err = provider.ValidateConfig(context.Background(), configJSON)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	err := provider.ValidateConfig(context.Background(), []byte("invalid json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestValidateConfig_NegativeSamplingRate(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	config := &FirewallLoggingConfig{
		Enabled:      true,
		SamplingRate: -1,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	err = provider.ValidateConfig(context.Background(), configJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sampling_rate cannot be negative")
}

func TestValidateConfig_NegativeRateLimit(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	config := &FirewallLoggingConfig{
		Enabled:         true,
		RateLimitPerSec: -1,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	err = provider.ValidateConfig(context.Background(), configJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate_limit_per_sec cannot be negative")
}

func TestValidateConfig_NegativeMaxEntries(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	config := &FirewallLoggingConfig{
		Enabled:       true,
		MaxLogEntries: -1,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	err = provider.ValidateConfig(context.Background(), configJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_log_entries cannot be negative")
}

func TestValidateConfig_NegativeRetentionDays(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	config := &FirewallLoggingConfig{
		Enabled:       true,
		RetentionDays: -1,
	}

	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	err = provider.ValidateConfig(context.Background(), configJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retention_days cannot be negative")
}

func TestFlush(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	// Flush with no active resources should not error
	err := provider.Flush(context.Background())
	assert.NoError(t, err)

	// Verify state is cleared
	assert.Nil(t, provider.config)
	assert.Nil(t, provider.db)
	assert.Nil(t, provider.capture)
}

func TestStatus_Disabled(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	statusJSON, err := provider.Status(context.Background())
	require.NoError(t, err)

	var status map[string]interface{}
	err = json.Unmarshal(statusJSON, &status)
	require.NoError(t, err)

	assert.Equal(t, false, status["enabled"])
}

func TestOnLogEvent_NotImplemented(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	err := provider.OnLogEvent(context.Background(), []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement log event handling")
}

func TestExecuteCLICommand_EmptyCommand(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty command")
}

func TestExecuteCLICommand_UnknownCommand(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "unknown", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestExecuteCLICommand_MissingSubcommand(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "firewall", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires subcommand")
}

func TestExecuteCLICommand_UnknownSubcommand(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "firewall unknown", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown firewall subcommand")
}

func TestExecuteCLICommand_LogsNotEnabled(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "firewall logs", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestExecuteCLICommand_StatsNotEnabled(t *testing.T) {
	provider := NewFirewallLoggingRPCProvider()

	_, err := provider.ExecuteCLICommand(context.Background(), "firewall stats", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}
