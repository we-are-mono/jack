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

// jack-plugin-nftables is a Jack plugin that provides firewall functionality using nftables.
// This plugin runs as a separate process and communicates with Jack via RPC.
package main

import (
	"log"
	"os"

	jplugin "github.com/we-are-mono/jack/plugins"
)

func main() {
	// Set up logging to stderr (stdout is used for RPC)
	log.SetOutput(os.Stderr)
	log.SetPrefix("[jack-plugin-nftables] ")

	log.Println("Starting nftables plugin...")

	// Create the RPC provider directly
	provider, err := NewNftablesRPCProvider()
	if err != nil {
		log.Fatalf("Failed to create nftables provider: %v", err)
	}

	// Serve the plugin using RPC protocol
	jplugin.ServePlugin(provider)
}
