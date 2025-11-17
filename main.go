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

// Jack is a network gateway management tool with plugin-based architecture.
// It provides unified configuration for firewall, DHCP, DNS, routing, and network
// interfaces through a transactional API and CLI.
package main

import "github.com/we-are-mono/jack/cmd"

// Version is the application version, set at build time via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	cmd.SetVersion(Version, BuildTime)
	cmd.Execute()
}
