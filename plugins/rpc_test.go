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

package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements the Provider interface for testing
type mockProvider struct {
	metadataFunc          func(ctx context.Context) (MetadataResponse, error)
	applyConfigFunc       func(ctx context.Context, configJSON []byte) error
	validateConfigFunc    func(ctx context.Context, configJSON []byte) error
	flushFunc             func(ctx context.Context) error
	statusFunc            func(ctx context.Context) ([]byte, error)
	executeCLICommandFunc func(ctx context.Context, command string, args []string) ([]byte, error)
}

func (m *mockProvider) Metadata(ctx context.Context) (MetadataResponse, error) {
	if m.metadataFunc != nil {
		return m.metadataFunc(ctx)
	}
	return MetadataResponse{
		Namespace:   "test",
		Version:     "1.0.0",
		Description: "Test plugin",
		ConfigPath:  "/etc/jack/test.json",
	}, nil
}

func (m *mockProvider) ApplyConfig(ctx context.Context, configJSON []byte) error {
	if m.applyConfigFunc != nil {
		return m.applyConfigFunc(ctx, configJSON)
	}
	return nil
}

func (m *mockProvider) ValidateConfig(ctx context.Context, configJSON []byte) error {
	if m.validateConfigFunc != nil {
		return m.validateConfigFunc(ctx, configJSON)
	}
	return nil
}

func (m *mockProvider) Flush(ctx context.Context) error {
	if m.flushFunc != nil {
		return m.flushFunc(ctx)
	}
	return nil
}

func (m *mockProvider) Status(ctx context.Context) ([]byte, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return []byte(`{"status":"ok"}`), nil
}

func (m *mockProvider) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	if m.executeCLICommandFunc != nil {
		return m.executeCLICommandFunc(ctx, command, args)
	}
	return []byte("command output"), nil
}

func (m *mockProvider) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	// Mock implementation - just return nil
	return nil
}

func TestMetadataResponse_Structure(t *testing.T) {
	metadata := MetadataResponse{
		Namespace:    "test",
		Version:      "1.0.0",
		Description:  "Test plugin",
		ConfigPath:   "/etc/jack/test.json",
		Dependencies: []string{"dep1", "dep2"},
		CLICommands: []CLICommand{
			{
				Name:        "test",
				Short:       "Test command",
				Long:        "Test command description",
				Subcommands: []string{"sub1", "sub2"},
			},
		},
	}

	assert.Equal(t, "test", metadata.Namespace)
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, "Test plugin", metadata.Description)
	assert.Equal(t, "/etc/jack/test.json", metadata.ConfigPath)
	assert.Len(t, metadata.Dependencies, 2)
	assert.Len(t, metadata.CLICommands, 1)
}

// TestCLICommand_Structure tests CLI command structure
func TestCLICommand_Structure(t *testing.T) {
	cmd := CLICommand{
		Name:         "monitor",
		Short:        "Monitor system",
		Long:         "Monitor system resources and network",
		Subcommands:  []string{"stats", "bandwidth"},
		Continuous:   true,
		PollInterval: 5,
	}

	assert.Equal(t, "monitor", cmd.Name)
	assert.Equal(t, "Monitor system", cmd.Short)
	assert.Equal(t, "Monitor system resources and network", cmd.Long)
	assert.Len(t, cmd.Subcommands, 2)
	assert.True(t, cmd.Continuous)
	assert.Equal(t, 5, cmd.PollInterval)
}

// TestCLICommand_DefaultValues tests CLI command default values
func TestCLICommand_DefaultValues(t *testing.T) {
	cmd := CLICommand{
		Name:  "simple",
		Short: "Simple command",
		// Continuous defaults to false
		// PollInterval defaults to 0
	}

	assert.False(t, cmd.Continuous)
	assert.Equal(t, 0, cmd.PollInterval)
}

// TestMetadataResponse_DefaultConfig tests default config in metadata
func TestMetadataResponse_DefaultConfig(t *testing.T) {
	metadata := MetadataResponse{
		Namespace:   "monitoring",
		Version:     "1.0.0",
		Description: "Monitoring plugin",
		ConfigPath:  "/etc/jack/monitoring.json",
		DefaultConfig: map[string]interface{}{
			"enabled":             true,
			"collection_interval": 5,
		},
	}

	assert.NotNil(t, metadata.DefaultConfig)
	assert.Equal(t, true, metadata.DefaultConfig["enabled"])
	assert.Equal(t, 5, metadata.DefaultConfig["collection_interval"])
}

// TestMetadataResponse_NilDefaultConfig tests metadata without default config
func TestMetadataResponse_NilDefaultConfig(t *testing.T) {
	metadata := MetadataResponse{
		Namespace:     "firewall",
		Version:       "1.0.0",
		Description:   "Firewall plugin",
		ConfigPath:    "/etc/jack/firewall.json",
		DefaultConfig: nil,
	}

	assert.Nil(t, metadata.DefaultConfig)
}

// TestProvider_Metadata tests provider metadata method
func TestProvider_Metadata(t *testing.T) {
	provider := &mockProvider{}

	metadata, err := provider.Metadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "test", metadata.Namespace)
	assert.Equal(t, "1.0.0", metadata.Version)
}

// TestProvider_ApplyConfig tests provider apply config method
func TestProvider_ApplyConfig(t *testing.T) {
	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			assert.NotEmpty(t, configJSON)
			return nil
		},
	}

	err := provider.ApplyConfig(context.Background(), []byte(`{"test":"config"}`))
	assert.NoError(t, err)
}

// TestProvider_ValidateConfig tests provider validate config method
func TestProvider_ValidateConfig(t *testing.T) {
	provider := &mockProvider{
		validateConfigFunc: func(ctx context.Context, configJSON []byte) error {
			assert.NotEmpty(t, configJSON)
			return nil
		},
	}

	err := provider.ValidateConfig(context.Background(), []byte(`{"test":"config"}`))
	assert.NoError(t, err)
}

// TestProvider_Flush tests provider flush method
func TestProvider_Flush(t *testing.T) {
	flushed := false
	provider := &mockProvider{
		flushFunc: func(ctx context.Context) error {
			flushed = true
			return nil
		},
	}

	err := provider.Flush(context.Background())
	assert.NoError(t, err)
	assert.True(t, flushed)
}

// TestProvider_Status tests provider status method
func TestProvider_Status(t *testing.T) {
	provider := &mockProvider{
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return []byte(`{"running":true,"health":"ok"}`), nil
		},
	}

	status, err := provider.Status(context.Background())
	require.NoError(t, err)
	assert.Contains(t, string(status), "running")
	assert.Contains(t, string(status), "health")
}

// TestProvider_ExecuteCLICommand tests provider CLI command execution
func TestProvider_ExecuteCLICommand(t *testing.T) {
	provider := &mockProvider{
		executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
			assert.Equal(t, "monitor stats", command)
			assert.Len(t, args, 0)
			return []byte("CPU: 45%\nMem: 60%"), nil
		},
	}

	output, err := provider.ExecuteCLICommand(context.Background(), "monitor stats", []string{})
	require.NoError(t, err)
	assert.Contains(t, string(output), "CPU")
	assert.Contains(t, string(output), "Mem")
}

// TestCLICommand_ContinuousCommand tests continuous command configuration
func TestCLICommand_ContinuousCommand(t *testing.T) {
	cmd := CLICommand{
		Name:         "monitor",
		Short:        "Live monitoring",
		Continuous:   true,
		PollInterval: 2,
	}

	assert.True(t, cmd.Continuous)
	assert.Equal(t, 2, cmd.PollInterval)
}

// TestCLICommand_OneOffCommand tests one-off command configuration
func TestCLICommand_OneOffCommand(t *testing.T) {
	cmd := CLICommand{
		Name:       "status",
		Short:      "Show status",
		Continuous: false,
	}

	assert.False(t, cmd.Continuous)
}

// TestMetadataResponse_Dependencies tests dependency declaration
func TestMetadataResponse_Dependencies(t *testing.T) {
	metadata := MetadataResponse{
		Namespace:    "advanced-firewall",
		Version:      "2.0.0",
		Description:  "Advanced firewall with dependencies",
		Dependencies: []string{"firewall", "monitoring"},
	}

	assert.Len(t, metadata.Dependencies, 2)
	assert.Contains(t, metadata.Dependencies, "firewall")
	assert.Contains(t, metadata.Dependencies, "monitoring")
}

// TestMetadataResponse_NoDependencies tests plugin without dependencies
func TestMetadataResponse_NoDependencies(t *testing.T) {
	metadata := MetadataResponse{
		Namespace:    "standalone",
		Version:      "1.0.0",
		Description:  "Standalone plugin",
		Dependencies: nil,
	}

	assert.Nil(t, metadata.Dependencies)
}

// TestCLICommand_WithSubcommands tests command with subcommands
func TestCLICommand_WithSubcommands(t *testing.T) {
	cmd := CLICommand{
		Name:        "led",
		Short:       "LED control",
		Subcommands: []string{"status", "on", "off", "blink"},
	}

	assert.Len(t, cmd.Subcommands, 4)
	assert.Contains(t, cmd.Subcommands, "status")
	assert.Contains(t, cmd.Subcommands, "on")
	assert.Contains(t, cmd.Subcommands, "off")
	assert.Contains(t, cmd.Subcommands, "blink")
}

func TestErrFromString(t *testing.T) {
	// Nil case
	err := ErrFromString("")
	assert.NoError(t, err)

	// Error case
	err = ErrFromString("test error")
	assert.Error(t, err)
	assert.Equal(t, "test error", err.Error())
}

func TestRPCError_Error(t *testing.T) {
	err := &rpcError{msg: "custom error message"}
	assert.Equal(t, "custom error message", err.Error())
}

func TestHandshake_Configuration(t *testing.T) {
	assert.Equal(t, uint(1), Handshake.ProtocolVersion)
	assert.Equal(t, "JACK_PLUGIN", Handshake.MagicCookieKey)
	assert.Equal(t, "generic", Handshake.MagicCookieValue)
}

func TestProviderInterface_Metadata(t *testing.T) {
	provider := &mockProvider{}
	ctx := context.Background()

	metadata, err := provider.Metadata(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test", metadata.Namespace)
	assert.Equal(t, "1.0.0", metadata.Version)
}

func TestProviderInterface_ApplyConfig(t *testing.T) {
	called := false
	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			called = true
			assert.NotNil(t, configJSON)
			return nil
		},
	}

	err := provider.ApplyConfig(context.Background(), []byte(`{"test": "config"}`))
	assert.NoError(t, err)
	assert.True(t, called, "ApplyConfig should be called")
}

func TestProviderInterface_ValidateConfig(t *testing.T) {
	called := false
	provider := &mockProvider{
		validateConfigFunc: func(ctx context.Context, configJSON []byte) error {
			called = true
			return nil
		},
	}

	err := provider.ValidateConfig(context.Background(), []byte(`{}`))
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProviderInterface_Flush(t *testing.T) {
	called := false
	provider := &mockProvider{
		flushFunc: func(ctx context.Context) error {
			called = true
			return nil
		},
	}

	err := provider.Flush(context.Background())
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProviderInterface_Status(t *testing.T) {
	provider := &mockProvider{
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return []byte(`{"running": true}`), nil
		},
	}

	status, err := provider.Status(context.Background())
	require.NoError(t, err)
	assert.Contains(t, string(status), "running")
}

func TestProviderInterface_ExecuteCLICommand(t *testing.T) {
	provider := &mockProvider{
		executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
			assert.Equal(t, "test command", command)
			assert.Equal(t, []string{"arg1", "arg2"}, args)
			return []byte("test output"), nil
		},
	}

	output, err := provider.ExecuteCLICommand(context.Background(), "test command", []string{"arg1", "arg2"})
	require.NoError(t, err)
	assert.Equal(t, "test output", string(output))
}
