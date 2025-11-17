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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRPCServer_Metadata tests RPCServer.Metadata method
func TestRPCServer_Metadata(t *testing.T) {
	tests := []struct {
		name           string
		provider       *mockProvider
		expectedError  string
		expectedNamespace string
	}{
		{
			name:              "successful metadata retrieval",
			provider:          &mockProvider{},
			expectedNamespace: "test",
		},
		{
			name: "metadata error",
			provider: &mockProvider{
				metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
					return MetadataResponse{}, errors.New("metadata error")
				},
			},
			expectedError: "metadata error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &MetadataArgs{}
			reply := &MetadataReply{}

			err := server.Metadata(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
				assert.Equal(t, tt.expectedNamespace, reply.Metadata.Namespace)
			}
		})
	}
}

// TestRPCServer_ApplyConfig tests RPCServer.ApplyConfig method
func TestRPCServer_ApplyConfig(t *testing.T) {
	tests := []struct {
		name          string
		provider      *mockProvider
		configJSON    []byte
		expectedError string
	}{
		{
			name:       "successful config apply",
			provider:   &mockProvider{},
			configJSON: []byte(`{"test":"value"}`),
		},
		{
			name: "apply config error",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
					return errors.New("config error")
				},
			},
			configJSON:    []byte(`{}`),
			expectedError: "config error",
		},
		{
			name: "verify config is passed",
			provider: &mockProvider{
				applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
					assert.Equal(t, `{"key":"val"}`, string(configJSON))
					return nil
				},
			},
			configJSON: []byte(`{"key":"val"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &ApplyConfigArgs{ConfigJSON: tt.configJSON}
			reply := &ApplyConfigReply{}

			err := server.ApplyConfig(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
			}
		})
	}
}

// TestRPCServer_ValidateConfig tests RPCServer.ValidateConfig method
func TestRPCServer_ValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		provider      *mockProvider
		configJSON    []byte
		expectedError string
	}{
		{
			name:       "successful validation",
			provider:   &mockProvider{},
			configJSON: []byte(`{"test":"value"}`),
		},
		{
			name: "validation error",
			provider: &mockProvider{
				validateConfigFunc: func(ctx context.Context, configJSON []byte) error {
					return errors.New("invalid config")
				},
			},
			configJSON:    []byte(`{}`),
			expectedError: "invalid config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &ValidateConfigArgs{ConfigJSON: tt.configJSON}
			reply := &ValidateConfigReply{}

			err := server.ValidateConfig(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
			}
		})
	}
}

// TestRPCServer_Flush tests RPCServer.Flush method
func TestRPCServer_Flush(t *testing.T) {
	tests := []struct {
		name          string
		provider      *mockProvider
		expectedError string
	}{
		{
			name:     "successful flush",
			provider: &mockProvider{},
		},
		{
			name: "flush error",
			provider: &mockProvider{
				flushFunc: func(ctx context.Context) error {
					return errors.New("flush failed")
				},
			},
			expectedError: "flush failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &FlushArgs{}
			reply := &FlushReply{}

			err := server.Flush(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
			}
		})
	}
}

// TestRPCServer_Status tests RPCServer.Status method
func TestRPCServer_Status(t *testing.T) {
	tests := []struct {
		name           string
		provider       *mockProvider
		expectedError  string
		expectedStatus string
	}{
		{
			name:           "successful status",
			provider:       &mockProvider{},
			expectedStatus: `{"status":"ok"}`,
		},
		{
			name: "status error",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					return nil, errors.New("status error")
				},
			},
			expectedError: "status error",
		},
		{
			name: "custom status",
			provider: &mockProvider{
				statusFunc: func(ctx context.Context) ([]byte, error) {
					return []byte(`{"running":true,"count":42}`), nil
				},
			},
			expectedStatus: `{"running":true,"count":42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &StatusArgs{}
			reply := &StatusReply{}

			err := server.Status(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
				assert.Equal(t, tt.expectedStatus, string(reply.StatusJSON))
			}
		})
	}
}

// TestRPCServer_ExecuteCLICommand tests RPCServer.ExecuteCLICommand method
func TestRPCServer_ExecuteCLICommand(t *testing.T) {
	tests := []struct {
		name           string
		provider       *mockProvider
		command        string
		args           []string
		expectedError  string
		expectedOutput string
	}{
		{
			name:           "successful command execution",
			provider:       &mockProvider{},
			command:        "test",
			args:           []string{},
			expectedOutput: "command output",
		},
		{
			name: "command execution error",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					return nil, errors.New("command failed")
				},
			},
			command:       "fail",
			args:          []string{},
			expectedError: "command failed",
		},
		{
			name: "command with arguments",
			provider: &mockProvider{
				executeCLICommandFunc: func(ctx context.Context, command string, args []string) ([]byte, error) {
					assert.Equal(t, "monitor", command)
					assert.Equal(t, []string{"eth0", "stats"}, args)
					return []byte("bandwidth: 100Mbps"), nil
				},
			},
			command:        "monitor",
			args:           []string{"eth0", "stats"},
			expectedOutput: "bandwidth: 100Mbps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &RPCServer{Impl: tt.provider}
			args := &ExecuteCLICommandArgs{
				Command: tt.command,
				Args:    tt.args,
			}
			reply := &ExecuteCLICommandReply{}

			err := server.ExecuteCLICommand(args, reply)
			assert.NoError(t, err, "RPC call itself should not error")

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, reply.Error)
			} else {
				assert.Empty(t, reply.Error)
				assert.Equal(t, tt.expectedOutput, string(reply.Output))
			}
		})
	}
}

// TestRPCServer_ErrorPropagation tests that errors are properly propagated
func TestRPCServer_ErrorPropagation(t *testing.T) {
	t.Run("metadata error propagation", func(t *testing.T) {
		server := &RPCServer{
			Impl: &mockProvider{
				metadataFunc: func(ctx context.Context) (MetadataResponse, error) {
					return MetadataResponse{}, errors.New("metadata failed")
				},
			},
		}

		reply := &MetadataReply{}
		err := server.Metadata(&MetadataArgs{}, reply)
		assert.NoError(t, err) // RPC doesn't error
		assert.Equal(t, "metadata failed", reply.Error)
	})

	t.Run("apply config error propagation", func(t *testing.T) {
		server := &RPCServer{
			Impl: &mockProvider{
				applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
					return errors.New("apply failed")
				},
			},
		}

		reply := &ApplyConfigReply{}
		err := server.ApplyConfig(&ApplyConfigArgs{ConfigJSON: []byte(`{}`)}, reply)
		assert.NoError(t, err)
		assert.Equal(t, "apply failed", reply.Error)
	})
}

// TestRPCServer_ContextPassing tests that context is properly passed
func TestRPCServer_ContextPassing(t *testing.T) {
	contextReceived := false

	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			assert.NotNil(t, ctx)
			contextReceived = true
			return nil
		},
	}

	server := &RPCServer{Impl: provider}
	reply := &ApplyConfigReply{}

	err := server.ApplyConfig(&ApplyConfigArgs{ConfigJSON: []byte(`{}`)}, reply)
	assert.NoError(t, err)
	assert.True(t, contextReceived, "Context should be passed to implementation")
}

// TestRPCServer_NilProvider tests behavior with nil provider (should panic)
func TestRPCServer_NilProvider(t *testing.T) {
	server := &RPCServer{Impl: nil}

	assert.Panics(t, func() {
		server.Metadata(&MetadataArgs{}, &MetadataReply{})
	}, "Should panic with nil provider")
}

// TestRPCServer_EmptyConfigJSON tests handling of empty config
func TestRPCServer_EmptyConfigJSON(t *testing.T) {
	called := false
	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			called = true
			assert.NotNil(t, configJSON, "ConfigJSON should not be nil")
			assert.Equal(t, 0, len(configJSON), "ConfigJSON should be empty")
			return nil
		},
	}

	server := &RPCServer{Impl: provider}
	reply := &ApplyConfigReply{}

	err := server.ApplyConfig(&ApplyConfigArgs{ConfigJSON: []byte{}}, reply)
	assert.NoError(t, err)
	assert.True(t, called)
}

// TestRPCServer_LargeConfigJSON tests handling of large config
func TestRPCServer_LargeConfigJSON(t *testing.T) {
	// Create a large JSON config (1MB)
	largeConfig := make([]byte, 1024*1024)
	for i := range largeConfig {
		largeConfig[i] = 'a'
	}

	provider := &mockProvider{
		applyConfigFunc: func(ctx context.Context, configJSON []byte) error {
			assert.Equal(t, len(largeConfig), len(configJSON))
			return nil
		},
	}

	server := &RPCServer{Impl: provider}
	reply := &ApplyConfigReply{}

	err := server.ApplyConfig(&ApplyConfigArgs{ConfigJSON: largeConfig}, reply)
	assert.NoError(t, err)
}

// TestRPCServer_StatusJSONFormat tests that status returns valid JSON
func TestRPCServer_StatusJSONFormat(t *testing.T) {
	provider := &mockProvider{
		statusFunc: func(ctx context.Context) ([]byte, error) {
			return []byte(`{"enabled":true,"count":42,"name":"test"}`), nil
		},
	}

	server := &RPCServer{Impl: provider}
	reply := &StatusReply{}

	err := server.Status(&StatusArgs{}, reply)
	require.NoError(t, err)
	assert.Empty(t, reply.Error)

	// Verify it's valid JSON
	var status map[string]interface{}
	err = json.Unmarshal(reply.StatusJSON, &status)
	require.NoError(t, err)
	assert.Equal(t, true, status["enabled"])
	assert.Equal(t, float64(42), status["count"])
	assert.Equal(t, "test", status["name"])
}
