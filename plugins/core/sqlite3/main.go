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

// jack-plugin-sqlite3 is a Jack plugin that provides SQLite3 database storage for logs and data.
// It runs as a separate process and communicates with Jack via RPC.
package main

import (
	"log"
	"os"

	jplugin "github.com/we-are-mono/jack/plugins"
)

func main() {
	// Set up logging to stderr (stdout is used for RPC)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[jack-plugin-sqlite3] ")

	log.Println("Starting sqlite3 plugin...")

	// Create the RPC provider with CLI command support
	provider := NewSQLite3RPCProvider()

	// Serve the plugin using generic protocol
	jplugin.ServePlugin(provider)
}
