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

package daemon

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestMarshaling tests Request JSON marshaling
func TestRequestMarshaling(t *testing.T) {
	tests := []struct {
		name    string
		request Request
	}{
		{
			name: "status command",
			request: Request{
				Command: "status",
			},
		},
		{
			name: "get command with path",
			request: Request{
				Command: "get",
				Path:    "interfaces",
			},
		},
		{
			name: "set command with value",
			request: Request{
				Command: "set",
				Path:    "interfaces",
				Value: map[string]interface{}{
					"wan": map[string]interface{}{
						"type": "physical",
					},
				},
			},
		},
		{
			name: "plugin command",
			request: Request{
				Command: "plugin-enable",
				Plugin:  "monitoring",
			},
		},
		{
			name: "plugin CLI command",
			request: Request{
				Command:    "plugin-cli",
				Plugin:     "monitoring",
				CLICommand: "monitor stats",
				CLIArgs:    []string{"arg1", "arg2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Unmarshal back
			var decoded Request
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Verify command field
			assert.Equal(t, tt.request.Command, decoded.Command)
			assert.Equal(t, tt.request.Path, decoded.Path)
			assert.Equal(t, tt.request.Plugin, decoded.Plugin)
			assert.Equal(t, tt.request.CLICommand, decoded.CLICommand)
			assert.Equal(t, tt.request.CLIArgs, decoded.CLIArgs)
		})
	}
}

// TestResponseMarshaling tests Response JSON marshaling
func TestResponseMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		response Response
	}{
		{
			name: "success response",
			response: Response{
				Success: true,
				Message: "Operation completed",
			},
		},
		{
			name: "error response",
			response: Response{
				Success: false,
				Error:   "Something went wrong",
			},
		},
		{
			name: "response with data",
			response: Response{
				Success: true,
				Data: map[string]interface{}{
					"interfaces": map[string]interface{}{
						"wan": map[string]interface{}{
							"type": "physical",
						},
					},
				},
			},
		},
		{
			name: "response with message and data",
			response: Response{
				Success: true,
				Message: "Configuration retrieved",
				Data: map[string]interface{}{
					"count": 42,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Unmarshal back
			var decoded Response
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Verify fields
			assert.Equal(t, tt.response.Success, decoded.Success)
			assert.Equal(t, tt.response.Message, decoded.Message)
			assert.Equal(t, tt.response.Error, decoded.Error)
		})
	}
}

// TestRequestOmitEmpty tests that empty fields are omitted from JSON
func TestRequestOmitEmpty(t *testing.T) {
	req := Request{
		Command: "status",
		// Path, Plugin, etc. are empty
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	// Parse as generic map to check what fields are present
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Only command should be present
	assert.Contains(t, jsonMap, "command")
	assert.NotContains(t, jsonMap, "path")
	assert.NotContains(t, jsonMap, "plugin")
	assert.NotContains(t, jsonMap, "value")
}

// TestResponseOmitEmpty tests that empty response fields are omitted
func TestResponseOmitEmpty(t *testing.T) {
	resp := Response{
		Success: true,
		// Message, Error, Data are empty
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Only success should be present
	assert.Contains(t, jsonMap, "success")
	assert.NotContains(t, jsonMap, "message")
	assert.NotContains(t, jsonMap, "error")
	assert.NotContains(t, jsonMap, "data")
}

// TestRequestAllCommands tests all valid command types
func TestRequestAllCommands(t *testing.T) {
	commands := []string{
		"status",
		"info",
		"diff",
		"commit",
		"revert",
		"apply",
		"show",
		"get",
		"set",
		"plugin-enable",
		"plugin-disable",
		"plugin-rescan",
		"plugin-cli",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			req := Request{Command: cmd}

			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded Request
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, cmd, decoded.Command)
		})
	}
}
