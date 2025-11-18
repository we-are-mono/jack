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

// DatabaseConfig represents the configuration for the database plugin
type DatabaseConfig struct {
	Enabled           bool   `json:"enabled"`             // Enable database plugin
	DatabasePath      string `json:"database_path"`       // Path to SQLite database file
	LogStorageEnabled bool   `json:"log_storage_enabled"` // Enable storing logs in database
	MaxLogEntries     int    `json:"max_log_entries"`     // Maximum number of log entries to store (0 = unlimited)
}
