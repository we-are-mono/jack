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

// Package daemon implements the Jack daemon server and IPC protocol.
package daemon

// LogFilter defines filtering criteria for log streaming
type LogFilter struct {
	Level     string `json:"level,omitempty"`     // Filter by log level (debug, info, warn, error, alert)
	Component string `json:"component,omitempty"` // Filter by component name
	Tail      int    `json:"tail,omitempty"`      // Show last N log entries before streaming (0 = no tail)
}

// Request represents a command sent to the daemon
type Request struct {
	Value        interface{} `json:"value,omitempty"`
	Command      string      `json:"command"` // set, get, show, commit, revert, status, apply, monitor, plugin-enable, plugin-disable, plugin-rescan, plugin-cli, rollback, checkpoint-list, checkpoint-create, logs-subscribe
	Path         string      `json:"path,omitempty"`
	Plugin       string      `json:"plugin,omitempty"`      // Plugin name for plugin commands
	CLICommand   string      `json:"cli_command,omitempty"` // CLI command to execute (e.g., "monitor stats")
	CLIArgs      []string    `json:"cli_args,omitempty"`    // Arguments for CLI command
	CheckpointID string      `json:"checkpoint_id,omitempty"` // Checkpoint ID for rollback
	LogFilter    *LogFilter  `json:"log_filter,omitempty"`  // Log filter for logs-subscribe command
}

// Response represents the daemon's response
type Response struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success"`
}
