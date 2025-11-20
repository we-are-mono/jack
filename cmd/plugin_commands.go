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
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/plugins"
)

// RegisterPluginCommands discovers and registers CLI commands provided by plugins
func RegisterPluginCommands() {
	pluginDir := "/usr/lib/jack/plugins"
	if customDir := os.Getenv("JACK_PLUGIN_DIR"); customDir != "" {
		pluginDir = customDir
	}

	// Find all plugin binaries
	matches, err := filepath.Glob(filepath.Join(pluginDir, "jack-plugin-*"))
	if err != nil {
		log.Printf("[WARN] Failed to scan plugin directory: %v", err)
		return
	}

	for _, pluginPath := range matches {
		// Skip if not executable
		info, err := os.Stat(pluginPath)
		if err != nil || info.Mode()&0111 == 0 {
			continue
		}

		// Try to load plugin metadata
		if err := registerPluginCommands(pluginPath); err != nil {
			pluginName := filepath.Base(pluginPath)
			log.Printf("[WARN] Failed to register commands for %s: %v", pluginName, err)
		}
	}
}

// registerPluginCommands loads a plugin and registers its CLI commands
func registerPluginCommands(pluginPath string) error {
	// Start plugin client
	client, err := plugins.NewPluginClient(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	// Dispense the plugin
	provider, err := client.Dispense()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	// Get metadata
	metadata, err := provider.Metadata(context.Background())
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	// Close the client - we'll start new instances when commands are executed
	client.Close()

	// Sort CLI commands: process commands without spaces first, then multi-word commands
	// This ensures parent commands exist before we try to attach subcommands
	sortedCommands := make([]plugins.CLICommand, len(metadata.CLICommands))
	copy(sortedCommands, metadata.CLICommands)

	// Sort by number of spaces in the name (fewer spaces first)
	for i := 0; i < len(sortedCommands); i++ {
		for j := i + 1; j < len(sortedCommands); j++ {
			iSpaces := strings.Count(sortedCommands[i].Name, " ")
			jSpaces := strings.Count(sortedCommands[j].Name, " ")
			if iSpaces > jSpaces {
				sortedCommands[i], sortedCommands[j] = sortedCommands[j], sortedCommands[i]
			}
		}
	}

	// Register CLI commands in sorted order
	for _, cliCmd := range sortedCommands {
		cmd := createPluginCommand(pluginPath, cliCmd)
		// cmd will be nil if it was added as a subcommand to an existing parent
		if cmd != nil {
			rootCmd.AddCommand(cmd)
		}
	}

	return nil
}

// createPluginCommand creates a cobra command that delegates to a plugin
func createPluginCommand(pluginPath string, cliCmd plugins.CLICommand) *cobra.Command {
	// Handle commands with spaces (e.g., "firewall watch")
	// These should be attached as subcommands to the parent
	parts := strings.Fields(cliCmd.Name)
	if len(parts) > 1 {
		// This is a multi-word command like "firewall watch"
		// Find or create the parent command
		parentName := parts[0]
		subName := strings.Join(parts[1:], " ")

		// Check if parent command exists
		parentCmd := findCommand(rootCmd, parentName)
		if parentCmd == nil {
			// Create parent command
			parentCmd = &cobra.Command{
				Use:   parentName,
				Short: fmt.Sprintf("%s commands", parentName),
			}
			rootCmd.AddCommand(parentCmd)
		}

		// Create subcommand
		subCmd := &cobra.Command{
			Use:                subName,
			Short:              cliCmd.Short,
			Long:               cliCmd.Long,
			DisableFlagParsing: true,
			Run: func(cmd *cobra.Command, args []string) {
				executePluginCommand(pluginPath, cliCmd, "", args)
			},
		}
		parentCmd.AddCommand(subCmd)

		// Return nil since we added it to an existing parent
		return nil
	}

	cmd := &cobra.Command{
		Use:   cliCmd.Name,
		Short: cliCmd.Short,
		Long:  cliCmd.Long,
	}

	// If plugin provides subcommands, create them
	if len(cliCmd.Subcommands) > 0 {
		for _, subName := range cliCmd.Subcommands {
			subCmd := &cobra.Command{
				Use:                subName,
				Short:              fmt.Sprintf("%s %s", cliCmd.Name, subName),
				DisableFlagParsing: true, // Pass all args through to plugin
				Run: func(subCmd *cobra.Command, args []string) {
					executePluginCommand(pluginPath, cliCmd, subCmd.Use, args)
				},
			}
			cmd.AddCommand(subCmd)
		}
	} else {
		// Command has no subcommands, execute directly
		cmd.DisableFlagParsing = true // Pass all args through to plugin
		cmd.Run = func(cmd *cobra.Command, args []string) {
			executePluginCommand(pluginPath, cliCmd, "", args)
		}
	}

	return cmd
}

// findCommand searches for a command by name in the given parent command
func findCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

// executePluginCommand sends a CLI command request to the daemon
func executePluginCommand(pluginPath string, cliCmd plugins.CLICommand, subcommand string, args []string) {
	// Extract plugin name from path (e.g., /usr/lib/jack/plugins/jack-plugin-monitoring -> monitoring)
	pluginName := filepath.Base(pluginPath)
	pluginName = strings.TrimPrefix(pluginName, "jack-plugin-")

	// Build command string
	fullCommand := cliCmd.Name
	if subcommand != "" {
		fullCommand = cliCmd.Name + " " + subcommand
	}

	// Check if this is a continuous command (metadata-driven)
	// Special case: "firewall watch" is continuous even though parent isn't
	isContinuous := cliCmd.Continuous || fullCommand == "firewall watch"
	if isContinuous {
		// Get poll interval (default to 2 seconds if not specified)
		pollInterval := cliCmd.PollInterval
		if pollInterval <= 0 {
			pollInterval = 2
		}
		executeContinuousCommand(pluginName, fullCommand, args, pollInterval)
		return
	}

	// For one-off commands, execute once
	resp, err := client.Send(daemon.Request{
		Command:    "plugin-cli",
		Plugin:     pluginName,
		CLICommand: fullCommand,
		CLIArgs:    args,
	})

	if err != nil {
		log.Fatalf("[ERROR] %v", err)
	}

	if !resp.Success {
		log.Fatalf("[ERROR] %s", resp.Error)
	}

	// Print output
	if resp.Data != nil {
		if outputStr, ok := resp.Data.(string); ok && outputStr != "" {
			fmt.Println(strings.TrimSpace(outputStr))
		}
	}
}

// executeContinuousCommand implements a continuous polling loop for any command marked as continuous
func executeContinuousCommand(pluginName, fullCommand string, args []string, pollInterval int) {
	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create ticker for periodic updates
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	// Print initial output immediately
	shouldContinue := printCommandSnapshot(pluginName, fullCommand, args)
	if !shouldContinue {
		return
	}

	for {
		select {
		case <-sigChan:
			// Ctrl+C pressed, exit gracefully
			fmt.Println("\nStopped.")
			return
		case <-ticker.C:
			// Clear screen and print updated info
			clearScreen()
			shouldContinue := printCommandSnapshot(pluginName, fullCommand, args)
			if !shouldContinue {
				return
			}
		}
	}
}

// printCommandSnapshot fetches and prints a single command snapshot from the daemon.
// Returns false if the command should stop (due to error or degraded state).
func printCommandSnapshot(pluginName, fullCommand string, args []string) bool {
	resp, err := client.Send(daemon.Request{
		Command:    "plugin-cli",
		Plugin:     pluginName,
		CLICommand: fullCommand,
		CLIArgs:    args,
	})

	if err != nil {
		fmt.Printf("[ERROR] %v\n", err)
		fmt.Println("\nCannot continue - daemon communication failed.")
		return false
	}

	if !resp.Success {
		fmt.Printf("[ERROR] %s\n", resp.Error)
		fmt.Println("\nCannot continue - command failed.")
		return false
	}

	// Print output
	if resp.Data != nil {
		if outputStr, ok := resp.Data.(string); ok && outputStr != "" {
			// Check if output indicates an error condition (plugin returned error message as output)
			// This happens when plugins want to provide helpful error messages
			if strings.HasPrefix(strings.TrimSpace(outputStr), "Error:") {
				fmt.Print(outputStr)
				fmt.Println("\nCannot continue - service unavailable.")
				return false
			}
			fmt.Print(outputStr)
		}
	}

	return true
}

// clearScreen clears the terminal screen using ANSI escape codes
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
