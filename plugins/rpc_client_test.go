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
	"encoding/json"
	"errors"
	"net"
	"net/rpc"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRPCPair creates a connected RPC client and server for testing
func setupRPCPair(provider Provider) (*RPCClient, *rpc.Server) {
	// Create RPC server with provider
	server := rpc.NewServer()
	rpcServer := &RPCServer{Impl: provider}
	err := server.RegisterName("Plugin", rpcServer)
	if err != nil {
		panic(err)
	}

	// Create in-memory connection pair
	clientConn, serverConn := net.Pipe()

	// Serve RPC in background
	go server.ServeConn(serverConn)

	// Create RPC client
	client := rpc.NewClient(clientConn)
	rpcClient := &RPCClient{client: client}

	return rpcClient, server
}

// TestRPCClient_Metadata tests RPCClient.Metadata RPC call
func TestRPCClient_Metadata(t *testing.T) {
	tests := []struct {
		name             string
		provider         Provider
		expectedMetadata MetadataResponse
		expectError      bool
		errorMsg         string
	}{
		{
			name: "successful metadata retrieval",
			provider: &mockProvider{
				metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
					return MetadataResponse{
						Namespace:   "test-plugin",
						Version:     "1.0.0",
						Description: "Test plugin",
						ConfigPath:  "/etc/jack/test.json",
					}, nil
				},
			},
			expectedMetadata: MetadataResponse{
				Namespace:   "test-plugin",
				Version:     "1.0.0",
				Description: "Test plugin",
				ConfigPath:  "/etc/jack/test.json",
			},
		},
		{
			name: "metadata with default config",
			provider: &mockProvider{
				metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
					return MetadataResponse{
						Namespace:   "test-plugin",
						Version:     "2.0.0",
						Description: "Test plugin with defaults",
						ConfigPath:  "/etc/jack/test.json",
						DefaultConfig: map[string]interface{}{
							"enabled": true,
							"timeout": 30,
						},
					}, nil
				},
			},
			expectedMetadata: MetadataResponse{
				Namespace:   "test-plugin",
				Version:     "2.0.0",
				Description: "Test plugin with defaults",
				ConfigPath:  "/etc/jack/test.json",
				DefaultConfig: map[string]interface{}{
					"enabled": true,
					"timeout": 30, // DefaultConfig doesn't go through JSON, so stays as int
				},
			},
		},
		{
			name: "metadata error",
			provider: &mockProvider{
				metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
					return MetadataResponse{}, errors.New("metadata error")
				},
			},
			expectError: true,
			errorMsg:    "metadata error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			metadata, err := client.Metadata(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMetadata.Namespace, metadata.Namespace)
				assert.Equal(t, tt.expectedMetadata.Version, metadata.Version)
				assert.Equal(t, tt.expectedMetadata.Description, metadata.Description)
				assert.Equal(t, tt.expectedMetadata.ConfigPath, metadata.ConfigPath)

				// Compare default config if present
				if tt.expectedMetadata.DefaultConfig != nil {
					require.NotNil(t, metadata.DefaultConfig)
					for key, expected := range tt.expectedMetadata.DefaultConfig {
						assert.Equal(t, expected, metadata.DefaultConfig[key])
					}
				}
			}
		})
	}
}

// TestRPCClient_ApplyConfig tests RPCClient.ApplyConfig RPC call
func TestRPCClient_ApplyConfig(t *testing.T) {
	tests := []struct {
		name        string
		provider    Provider
		configJSON  []byte
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful config apply",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, config []byte) error {
					return nil
				},
			},
			configJSON: []byte(`{"enabled": true}`),
		},
		{
			name: "apply config error",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, config []byte) error {
					return errors.New("apply failed")
				},
			},
			configJSON:  []byte(`{"enabled": true}`),
			expectError: true,
			errorMsg:    "apply failed",
		},
		{
			name: "config is properly passed through RPC",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, config []byte) error {
					var cfg map[string]interface{}
					if err := json.Unmarshal(config, &cfg); err != nil {
						return err
					}
					if cfg["test_key"] != "test_value" {
						return errors.New("config not passed correctly")
					}
					return nil
				},
			},
			configJSON: []byte(`{"test_key": "test_value"}`),
		},
		{
			name: "empty config",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, config []byte) error {
					if len(config) != 2 { // "{}"
						return errors.New("expected empty object")
					}
					return nil
				},
			},
			configJSON: []byte(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			err := client.ApplyConfig(context.Background(), tt.configJSON)

			if tt.expectError {
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

// TestRPCClient_ValidateConfig tests RPCClient.ValidateConfig RPC call
func TestRPCClient_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		provider    Provider
		configJSON  []byte
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful validation",
			provider: &mockProvider{
				validateConfigFunc: func(ctx context.Context, config []byte) error {
					return nil
				},
			},
			configJSON: []byte(`{"valid": true}`),
		},
		{
			name: "validation error",
			provider: &mockProvider{
				validateConfigFunc: func(ctx context.Context, config []byte) error {
					return errors.New("validation failed: missing required field")
				},
			},
			configJSON:  []byte(`{"invalid": true}`),
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name: "config is properly validated through RPC",
			provider: &mockProvider{
				validateConfigFunc: func(ctx context.Context, config []byte) error {
					var cfg map[string]interface{}
					if err := json.Unmarshal(config, &cfg); err != nil {
						return err
					}
					if _, ok := cfg["required_field"]; !ok {
						return errors.New("missing required_field")
					}
					return nil
				},
			},
			configJSON: []byte(`{"required_field": "value"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			err := client.ValidateConfig(context.Background(), tt.configJSON)

			if tt.expectError {
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

// TestRPCClient_Flush tests RPCClient.Flush RPC call
func TestRPCClient_Flush(t *testing.T) {
	tests := []struct {
		name        string
		provider    Provider
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful flush",
			provider: &mockProvider{
				flushFunc: func(ctx context.Context) error {
					return nil
				},
			},
		},
		{
			name: "flush error",
			provider: &mockProvider{
				flushFunc: func(ctx context.Context) error {
					return errors.New("flush failed")
				},
			},
			expectError: true,
			errorMsg:    "flush failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			err := client.Flush(context.Background())

			if tt.expectError {
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

// TestRPCClient_Status tests RPCClient.Status RPC call
func TestRPCClient_Status(t *testing.T) {
	tests := []struct {
		name           string
		provider       Provider
		expectedStatus []byte
		expectError    bool
		errorMsg       string
	}{
		{
			name: "successful status",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					return []byte(`{"status":"running"}`), nil
				},
			},
			expectedStatus: []byte(`{"status":"running"}`),
		},
		{
			name: "status error",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					return nil, errors.New("status error")
				},
			},
			expectError: true,
			errorMsg:    "status error",
		},
		{
			name: "complex status object",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					status := map[string]interface{}{
						"running":    true,
						"pid":        1234,
						"uptime":     3600,
						"error_count": 0,
					}
					return json.Marshal(status)
				},
			},
			expectedStatus: []byte(`{"running":true,"pid":1234,"uptime":3600,"error_count":0}`),
		},
		{
			name: "empty status",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					return []byte(`{}`), nil
				},
			},
			expectedStatus: []byte(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			status, err := client.Status(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)

				// Compare JSON content (not exact bytes, since formatting may differ)
				var expectedObj, actualObj map[string]interface{}
				err := json.Unmarshal(tt.expectedStatus, &expectedObj)
				require.NoError(t, err)
				err = json.Unmarshal(status, &actualObj)
				require.NoError(t, err)

				assert.Equal(t, expectedObj, actualObj)
			}
		})
	}
}

// TestRPCClient_ExecuteCLICommand tests RPCClient.ExecuteCLICommand RPC call
func TestRPCClient_ExecuteCLICommand(t *testing.T) {
	tests := []struct {
		name           string
		provider       Provider
		command        string
		args           []string
		expectedOutput []byte
		expectError    bool
		errorMsg       string
	}{
		{
			name: "successful command execution",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					return []byte("command output"), nil
				},
			},
			command:        "test",
			args:           []string{},
			expectedOutput: []byte("command output"),
		},
		{
			name: "command with arguments",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					if command == "bandwidth" && len(args) == 1 && args[0] == "eth0" {
						return []byte("bandwidth for eth0: 1Gbps"), nil
					}
					return nil, errors.New("unexpected command")
				},
			},
			command:        "bandwidth",
			args:           []string{"eth0"},
			expectedOutput: []byte("bandwidth for eth0: 1Gbps"),
		},
		{
			name: "command execution error",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					return nil, errors.New("command not found")
				},
			},
			command:     "unknown",
			args:        []string{},
			expectError: true,
			errorMsg:    "command not found",
		},
		{
			name: "command with empty output",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					return nil, nil // Empty output as nil slice
				},
			},
			command:        "test",
			args:           []string{},
			expectedOutput: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := setupRPCPair(tt.provider)

			output, err := client.ExecuteCLICommand(context.Background(), tt.command, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, output)
			}
		})
	}
}

// TestRPCClient_FullRoundTrip tests complete RPC communication cycle
func TestRPCClient_FullRoundTrip(t *testing.T) {
	// Create a complete provider with all methods implemented
	provider := &mockProvider{
		metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
			return MetadataResponse{
				Namespace:   "full-test",
				Version:     "1.0.0",
				Description: "Full round-trip test",
			}, nil
		},
		applyConfigFunc: func(ctx context.Context, config []byte) error {
			return nil
		},
		validateConfigFunc: func(ctx context.Context, config []byte) error {
			return nil
		},
		flushFunc: func(ctx context.Context) error {
			return nil
		},
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return []byte(`{"status":"ok"}`), nil
		},
		executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
			return []byte("CLI output"), nil
		},
	}

	client, _ := setupRPCPair(provider)
	ctx := context.Background()

	// Test full lifecycle
	t.Run("metadata", func(t *testing.T) {
		metadata, err := client.Metadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, "full-test", metadata.Namespace)
	})

	t.Run("validate config", func(t *testing.T) {
		err := client.ValidateConfig(ctx, []byte(`{"test":true}`))
		require.NoError(t, err)
	})

	t.Run("apply config", func(t *testing.T) {
		err := client.ApplyConfig(ctx, []byte(`{"test":true}`))
		require.NoError(t, err)
	})

	t.Run("status", func(t *testing.T) {
		status, err := client.Status(ctx)
		require.NoError(t, err)
		assert.Contains(t, string(status), "ok")
	})

	t.Run("execute CLI command", func(t *testing.T) {
		output, err := client.ExecuteCLICommand(ctx, "test", []string{})
		require.NoError(t, err)
		assert.Equal(t, []byte("CLI output"), output)
	})

	t.Run("flush", func(t *testing.T) {
		err := client.Flush(ctx)
		require.NoError(t, err)
	})
}

// TestRPCClient_ErrorPropagation tests that errors propagate correctly through RPC
func TestRPCClient_ErrorPropagation(t *testing.T) {
	provider := &mockProvider{
		metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
			return MetadataResponse{}, errors.New("metadata retrieval failed")
		},
		applyConfigFunc: func(ctx context.Context, config []byte) error {
			return errors.New("config application failed")
		},
		validateConfigFunc: func(ctx context.Context, config []byte) error {
			return errors.New("config validation failed")
		},
		flushFunc: func(ctx context.Context) error {
			return errors.New("flush operation failed")
		},
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return nil, errors.New("status retrieval failed")
		},
		executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
			return nil, errors.New("command execution failed")
		},
	}

	client, _ := setupRPCPair(provider)
	ctx := context.Background()

	t.Run("metadata error", func(t *testing.T) {
		_, err := client.Metadata(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metadata retrieval failed")
	})

	t.Run("apply config error", func(t *testing.T) {
		err := client.ApplyConfig(ctx, []byte(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config application failed")
	})

	t.Run("validate config error", func(t *testing.T) {
		err := client.ValidateConfig(ctx, []byte(`{}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config validation failed")
	})

	t.Run("flush error", func(t *testing.T) {
		err := client.Flush(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "flush operation failed")
	})

	t.Run("status error", func(t *testing.T) {
		_, err := client.Status(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status retrieval failed")
	})

	t.Run("execute CLI command error", func(t *testing.T) {
		_, err := client.ExecuteCLICommand(ctx, "test", []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "command execution failed")
	})
}
