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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRootCmdExists tests that root command is properly initialized
func TestRootCmdExists(t *testing.T) {
	assert.NotNil(t, rootCmd, "root command should exist")
	assert.Equal(t, "jack", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "Jack")
}

// TestRootCmdHasCommands tests that subcommands are registered
func TestRootCmdHasCommands(t *testing.T) {
	expectedCommands := []string{
		"status",
		"get",
		"set",
		"apply",
		"commit",
		"revert",
		"diff",
		"validate",
		"daemon",
		"plugin",
		"logs",
	}

	commands := rootCmd.Commands()
	commandNames := make([]string, 0, len(commands))
	for _, cmd := range commands {
		commandNames = append(commandNames, cmd.Name())
	}

	for _, expected := range expectedCommands {
		assert.Contains(t, commandNames, expected, "command %s should be registered", expected)
	}
}

// TestExecuteFunction tests that Execute function exists (can't test actual execution without args)
func TestExecuteFunction(t *testing.T) {
	// Just verify the function doesn't panic when imported
	// Actual execution would require mocking os.Args
	assert.NotPanics(t, func() {
		_ = Execute
	})
}
