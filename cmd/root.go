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

// Package cmd implements the CLI commands for Jack using cobra.
// It provides the root command structure and version management.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the application version string.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "jack",
	Short: "Jack - Netrunner System Daemon",
	Long: `Jack jacks into the kernel to configure network interfaces.

A transactional network configuration tool for Netrunner.`,
	Version: Version,
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("Jack v%s (built: %s)\n", Version, BuildTime))

	// Register plugin-provided CLI commands
	RegisterPluginCommands()
}

// Execute runs the root command and handles any errors.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// SetVersion updates the version and build time for display in help and version output.
func SetVersion(version, buildTime string) {
	Version = version
	BuildTime = buildTime
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(fmt.Sprintf("Jack v%s (built: %s)\n", version, buildTime))
}

// exitWithError is a helper function that exits with code 1.
// It can be overridden in tests to avoid actual exit.
var exitWithError = func() {
	os.Exit(1)
}
