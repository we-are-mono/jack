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

// mockPlugin implements the Plugin interface for testing
type mockPlugin struct {
	metadataFunc       func() PluginMetadata
	applyConfigFunc    func(config interface{}) error
	validateConfigFunc func(config interface{}) error
	flushFunc          func() error
	statusFunc         func() (interface{}, error)
	closeFunc          func() error
}

func (m *mockPlugin) Metadata() PluginMetadata {
	if m.metadataFunc != nil {
		return m.metadataFunc()
	}
	return PluginMetadata{
		Namespace:   "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		ConfigPath:  "/etc/jack/test.json",
	}
}

func (m *mockPlugin) ApplyConfig(config interface{}) error {
	if m.applyConfigFunc != nil {
		return m.applyConfigFunc(config)
	}
	return nil
}

func (m *mockPlugin) ValidateConfig(config interface{}) error {
	if m.validateConfigFunc != nil {
		return m.validateConfigFunc(config)
	}
	return nil
}

func (m *mockPlugin) Flush() error {
	if m.flushFunc != nil {
		return m.flushFunc()
	}
	return nil
}

func (m *mockPlugin) Status() (interface{}, error) {
	if m.statusFunc != nil {
		return m.statusFunc()
	}
	return map[string]interface{}{"status": "ok"}, nil
}

func (m *mockPlugin) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// TestPluginAdapter_Metadata tests PluginAdapter.Metadata
func TestPluginAdapter_Metadata(t *testing.T) {
	tests := []struct {
		name     string
		plugin   Plugin
		expected MetadataResponse
	}{
		{
			name:   "basic metadata",
			plugin: &mockPlugin{},
			expected: MetadataResponse{
				Namespace:   "test-plugin",
				Version:     "1.0.0",
				Description: "Test plugin",
				ConfigPath:  "/etc/jack/test.json",
			},
		},
		{
			name: "metadata with default config",
			plugin: &mockPlugin{
				metadataFunc: func() PluginMetadata {
					return PluginMetadata{
						Namespace:   "monitoring",
						Version:     "2.0.0",
						Description: "Monitoring plugin",
						ConfigPath:  "/etc/jack/monitoring.json",
						DefaultConfig: map[string]interface{}{
							"enabled": true,
						},
					}
				},
			},
			expected: MetadataResponse{
				Namespace:   "monitoring",
				Version:     "2.0.0",
				Description: "Monitoring plugin",
				ConfigPath:  "/etc/jack/monitoring.json",
				DefaultConfig: map[string]interface{}{
					"enabled": true,
				},
			},
		},
		{
			name: "metadata with dependencies",
			plugin: &mockPlugin{
				metadataFunc: func() PluginMetadata {
					return PluginMetadata{
						Namespace:    "firewall-advanced",
						Version:      "1.5.0",
						Description:  "Advanced firewall",
						Dependencies: []string{"firewall", "monitoring"},
					}
				},
			},
			expected: MetadataResponse{
				Namespace:    "firewall-advanced",
				Version:      "1.5.0",
				Description:  "Advanced firewall",
				Dependencies: []string{"firewall", "monitoring"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPluginAdapter(tt.plugin)
			metadata, err := adapter.Metadata(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Namespace, metadata.Namespace)
			assert.Equal(t, tt.expected.Version, metadata.Version)
			assert.Equal(t, tt.expected.Description, metadata.Description)
			assert.Equal(t, tt.expected.ConfigPath, metadata.ConfigPath)
			assert.Equal(t, tt.expected.DefaultConfig, metadata.DefaultConfig)
			assert.Equal(t, tt.expected.Dependencies, metadata.Dependencies)
		})
	}
}

// TestPluginAdapter_ApplyConfig tests PluginAdapter.ApplyConfig
func TestPluginAdapter_ApplyConfig(t *testing.T) {
	tests := []struct {
		name        string
		plugin      Plugin
		configJSON  []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:       "valid JSON config",
			plugin:     &mockPlugin{},
			configJSON: []byte(`{"enabled":true,"value":42}`),
		},
		{
			name:        "invalid JSON",
			plugin:      &mockPlugin{},
			configJSON:  []byte(`{invalid json`),
			expectError: true,
		},
		{
			name: "config error from plugin",
			plugin: &mockPlugin{
				applyConfigFunc: func(config interface{}) error {
					return errors.New("plugin apply error")
				},
			},
			configJSON:  []byte(`{}`),
			expectError: true,
			errorMsg:    "plugin apply error",
		},
		{
			name: "config is properly passed",
			plugin: &mockPlugin{
				applyConfigFunc: func(config interface{}) error {
					configMap, ok := config.(map[string]interface{})
					assert.True(t, ok)
					assert.Equal(t, "test-value", configMap["key"])
					assert.Equal(t, float64(123), configMap["number"])
					return nil
				},
			},
			configJSON: []byte(`{"key":"test-value","number":123}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPluginAdapter(tt.plugin)
			err := adapter.ApplyConfig(context.Background(), tt.configJSON)

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

// TestPluginAdapter_ValidateConfig tests PluginAdapter.ValidateConfig
func TestPluginAdapter_ValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		plugin      Plugin
		configJSON  []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:       "valid config",
			plugin:     &mockPlugin{},
			configJSON: []byte(`{"valid":true}`),
		},
		{
			name:        "invalid JSON",
			plugin:      &mockPlugin{},
			configJSON:  []byte(`{bad json}`),
			expectError: true,
		},
		{
			name: "validation error from plugin",
			plugin: &mockPlugin{
				validateConfigFunc: func(config interface{}) error {
					return errors.New("validation failed")
				},
			},
			configJSON:  []byte(`{}`),
			expectError: true,
			errorMsg:    "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPluginAdapter(tt.plugin)
			err := adapter.ValidateConfig(context.Background(), tt.configJSON)

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

// TestPluginAdapter_Flush tests PluginAdapter.Flush
func TestPluginAdapter_Flush(t *testing.T) {
	tests := []struct {
		name        string
		plugin      Plugin
		expectError bool
		errorMsg    string
	}{
		{
			name:   "successful flush",
			plugin: &mockPlugin{},
		},
		{
			name: "flush error",
			plugin: &mockPlugin{
				flushFunc: func() error {
					return errors.New("flush failed")
				},
			},
			expectError: true,
			errorMsg:    "flush failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPluginAdapter(tt.plugin)
			err := adapter.Flush(context.Background())

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

// TestPluginAdapter_Status tests PluginAdapter.Status
func TestPluginAdapter_Status(t *testing.T) {
	tests := []struct {
		name           string
		plugin         Plugin
		expectError    bool
		errorMsg       string
		expectedStatus map[string]interface{}
	}{
		{
			name:   "successful status",
			plugin: &mockPlugin{},
			expectedStatus: map[string]interface{}{
				"status": "ok",
			},
		},
		{
			name: "status error",
			plugin: &mockPlugin{
				statusFunc: func() (interface{}, error) {
					return nil, errors.New("status error")
				},
			},
			expectError: true,
			errorMsg:    "status error",
		},
		{
			name: "complex status object",
			plugin: &mockPlugin{
				statusFunc: func() (interface{}, error) {
					return map[string]interface{}{
						"running": true,
						"count":   42,
						"name":    "test-service",
					}, nil
				},
			},
			expectedStatus: map[string]interface{}{
				"running": true,
				"count":   float64(42), // JSON unmarshaling converts numbers to float64
				"name":    "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewPluginAdapter(tt.plugin)
			statusJSON, err := adapter.Status(context.Background())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)

				var status map[string]interface{}
				err = json.Unmarshal(statusJSON, &status)
				require.NoError(t, err)

				for key, expectedValue := range tt.expectedStatus {
					assert.Equal(t, expectedValue, status[key])
				}
			}
		})
	}
}

// TestPluginAdapter_ExecuteCLICommand tests PluginAdapter.ExecuteCLICommand
func TestPluginAdapter_ExecuteCLICommand(t *testing.T) {
	adapter := NewPluginAdapter(&mockPlugin{})

	// By default, plugins don't implement CLI commands
	output, err := adapter.ExecuteCLICommand(context.Background(), "test", []string{})

	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "plugin does not implement CLI commands")
}

// TestPluginAdapter_ContextPropagation tests that context is properly passed
func TestPluginAdapter_ContextPropagation(t *testing.T) {
	// Context is passed to adapter methods but not to plugin methods
	// (plugin interface doesn't accept context)
	adapter := NewPluginAdapter(&mockPlugin{})

	// These should all accept context without error
	_, err := adapter.Metadata(context.Background())
	assert.NoError(t, err)

	err = adapter.ApplyConfig(context.Background(), []byte(`{}`))
	assert.NoError(t, err)

	err = adapter.ValidateConfig(context.Background(), []byte(`{}`))
	assert.NoError(t, err)

	err = adapter.Flush(context.Background())
	assert.NoError(t, err)

	_, err = adapter.Status(context.Background())
	assert.NoError(t, err)
}

// TestPluginAdapter_JSONMarshaling tests JSON marshaling edge cases
func TestPluginAdapter_JSONMarshaling(t *testing.T) {
	t.Run("status with non-marshalable type", func(t *testing.T) {
		plugin := &mockPlugin{
			statusFunc: func() (interface{}, error) {
				// Return a non-marshalable type (channel)
				return make(chan int), nil
			},
		}

		adapter := NewPluginAdapter(plugin)
		_, err := adapter.Status(context.Background())
		assert.Error(t, err, "Should fail to marshal channel")
	})

	t.Run("status with marshalable complex type", func(t *testing.T) {
		type ComplexStatus struct {
			Name    string   `json:"name"`
			Enabled bool     `json:"enabled"`
			Values  []int    `json:"values"`
			Nested  struct {
				Key string `json:"key"`
			} `json:"nested"`
		}

		plugin := &mockPlugin{
			statusFunc: func() (interface{}, error) {
				status := ComplexStatus{
					Name:    "test",
					Enabled: true,
					Values:  []int{1, 2, 3},
				}
				status.Nested.Key = "value"
				return status, nil
			},
		}

		adapter := NewPluginAdapter(plugin)
		statusJSON, err := adapter.Status(context.Background())
		require.NoError(t, err)

		var status ComplexStatus
		err = json.Unmarshal(statusJSON, &status)
		require.NoError(t, err)

		assert.Equal(t, "test", status.Name)
		assert.True(t, status.Enabled)
		assert.Equal(t, []int{1, 2, 3}, status.Values)
		assert.Equal(t, "value", status.Nested.Key)
	})
}

// TestPluginAdapter_EmptyConfig tests handling of empty config
func TestPluginAdapter_EmptyConfig(t *testing.T) {
	called := false
	plugin := &mockPlugin{
		applyConfigFunc: func(config interface{}) error {
			called = true
			configMap, ok := config.(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, 0, len(configMap))
			return nil
		},
	}

	adapter := NewPluginAdapter(plugin)
	err := adapter.ApplyConfig(context.Background(), []byte(`{}`))

	assert.NoError(t, err)
	assert.True(t, called)
}

// TestPluginAdapter_NilPlugin tests behavior with nil plugin (should panic)
func TestPluginAdapter_NilPlugin(t *testing.T) {
	adapter := NewPluginAdapter(nil)

	assert.Panics(t, func() {
		adapter.Metadata(context.Background())
	}, "Should panic with nil plugin")
}
