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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateGenericJSON tests JSON file validation
func TestValidateGenericJSON(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name:        "valid JSON object",
			content:     `{"key": "value", "number": 42}`,
			expectError: false,
		},
		{
			name:        "valid JSON array",
			content:     `[1, 2, 3, "four"]`,
			expectError: false,
		},
		{
			name:        "empty JSON object",
			content:     `{}`,
			expectError: false,
		},
		{
			name:        "nested JSON",
			content:     `{"parent": {"child": "value"}}`,
			expectError: false,
		},
		{
			name:        "invalid JSON - missing comma",
			content:     `{"key": "value" "key2": "value2"}`,
			expectError: true,
		},
		{
			name:        "invalid JSON - trailing comma",
			content:     `{"key": "value",}`,
			expectError: true,
		},
		{
			name:        "invalid JSON - not closed",
			content:     `{"key": "value"`,
			expectError: true,
		},
		{
			name:        "not JSON",
			content:     `this is not JSON`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test.json")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Validate the file
			err = validateGenericJSON(testFile)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateGenericJSON_FileErrors tests file read errors
func TestValidateGenericJSON_FileErrors(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "non-existent file",
			path: "/tmp/nonexistent-file-xyz-12345.json",
		},
		{
			name: "invalid path",
			path: "/dev/null/invalid/path.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGenericJSON(tt.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to read")
		})
	}
}

// TestValidateCmdExists tests that validate command is registered
func TestValidateCmdExists(t *testing.T) {
	// Check that validate command is registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "validate" {
			found = true
			assert.Equal(t, "validate", cmd.Use)
			assert.Contains(t, cmd.Short, "Validate configuration files")
			break
		}
	}
	assert.True(t, found, "validate command should be registered")
}
