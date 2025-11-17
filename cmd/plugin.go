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
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/plugins"
	"github.com/we-are-mono/jack/state"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Jack plugins",
	Long:  `Manage Jack plugins including listing, enabling/disabling, and viewing plugin information.`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed plugins",
	Long:  `List all installed plugins and their status.`,
	RunE:  runPluginList,
}

var pluginStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show currently enabled plugins",
	Long:  `Show which plugins are currently enabled in the configuration.`,
	RunE:  runPluginStatus,
}

var pluginEnableCmd = &cobra.Command{
	Use:   "enable <plugin>",
	Short: "Enable a plugin dynamically",
	Long: `Enable a plugin at runtime without restarting the daemon.

Example:
  jack plugin enable nftables
  jack plugin enable dnsmasq`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginEnable,
}

var pluginDisableCmd = &cobra.Command{
	Use:   "disable <plugin>",
	Short: "Disable a plugin dynamically",
	Long: `Disable a plugin at runtime and clean up its system state.

Example:
  jack plugin disable nftables`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginDisable,
}

var pluginRescanCmd = &cobra.Command{
	Use:   "rescan",
	Short: "Rescan for new plugins",
	Long: `Rescan plugin directories and add any newly installed plugins to the configuration.
New plugins will be added in disabled state.

Example:
  jack plugin rescan`,
	RunE: runPluginRescan,
}

var pluginInfoCmd = &cobra.Command{
	Use:   "info <plugin>",
	Short: "Show detailed information about a plugin",
	Long:  `Show detailed information about a specific plugin including its metadata and status.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runPluginInfo,
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginStatusCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
	pluginCmd.AddCommand(pluginRescanCmd)
	pluginCmd.AddCommand(pluginInfoCmd)
	rootCmd.AddCommand(pluginCmd)
}

func runPluginList(cmd *cobra.Command, args []string) error {
	pm := plugins.NewPluginManager()
	pluginNames, err := pm.ListPlugins()
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	if len(pluginNames) == 0 {
		fmt.Println("No plugins found")
		return nil
	}

	// Load config to check which are enabled
	config, err := state.LoadJackConfig()
	enabledMap := make(map[string]bool)
	if err == nil {
		for name, state := range config.Plugins {
			if state.Enabled {
				enabledMap[name] = true
			}
		}
	}

	sort.Strings(pluginNames)

	fmt.Println("Installed plugins:")
	for _, name := range pluginNames {
		pluginPath, err := pm.FindPlugin(name)
		if err != nil {
			fmt.Printf("  %s - [ERROR: %v]\n", name, err)
			continue
		}

		status := "disabled"
		if enabledMap[name] {
			status = "enabled"
		}

		// Get plugin metadata
		metadata := getPluginMetadata(pluginPath)
		if metadata != "" {
			fmt.Printf("  %s [%s] - %s (%s)\n", name, status, metadata, pluginPath)
		} else {
			fmt.Printf("  %s [%s] - %s\n", name, status, pluginPath)
		}
	}

	return nil
}

func runPluginStatus(cmd *cobra.Command, args []string) error {
	config, err := state.LoadJackConfig()
	if err != nil {
		return fmt.Errorf("failed to load jack config: %w", err)
	}

	if len(config.Plugins) == 0 {
		fmt.Println("No plugins enabled")
		return nil
	}

	pm := plugins.NewPluginManager()

	fmt.Println("Enabled plugins:")
	for name, pluginState := range config.Plugins {
		if !pluginState.Enabled {
			continue
		}

		pluginPath, err := pm.FindPlugin(name)
		if err != nil {
			fmt.Printf("  %s - [NOT FOUND: %v]\n", name, err)
			continue
		}

		// Get plugin metadata
		metadata := getPluginMetadata(pluginPath)
		if metadata != "" {
			fmt.Printf("  %s - %s\n", name, metadata)
		} else {
			fmt.Printf("  %s\n", name)
		}
	}

	return nil
}

func runPluginEnable(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	// Send enable request to daemon
	req := daemon.Request{
		Command: "plugin-enable",
		Plugin:  pluginName,
	}

	resp, err := client.Send(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Printf("%s\n", resp.Message)
	return nil
}

func runPluginDisable(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	// Send disable request to daemon
	req := daemon.Request{
		Command: "plugin-disable",
		Plugin:  pluginName,
	}

	resp, err := client.Send(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Printf("%s\n", resp.Message)
	return nil
}

func runPluginRescan(cmd *cobra.Command, args []string) error {
	// Send rescan request to daemon
	req := daemon.Request{
		Command: "plugin-rescan",
	}

	resp, err := client.Send(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Printf("%s\n", resp.Message)
	return nil
}

func runPluginInfo(cmd *cobra.Command, args []string) error {
	pluginName := args[0]

	pm := plugins.NewPluginManager()
	pluginPath, err := pm.FindPlugin(pluginName)
	if err != nil {
		return fmt.Errorf("plugin '%s' not found: %w", pluginName, err)
	}

	fmt.Printf("Plugin: %s\n", pluginName)
	fmt.Printf("Path: %s\n", pluginPath)

	// Check if file exists and is executable
	info, err := os.Stat(pluginPath)
	if err != nil {
		fmt.Printf("Status: ERROR - %v\n", err)
		return nil
	}

	fmt.Printf("Size: %d bytes\n", info.Size())
	fmt.Printf("Executable: %v\n", (info.Mode().Perm()&0111) != 0)

	// Get plugin metadata by starting it
	client, err := plugins.NewPluginClient(pluginPath)
	if err != nil {
		fmt.Printf("Metadata: ERROR - failed to start plugin: %v\n", err)
		return nil
	}
	defer client.Close()

	provider, err := client.Dispense()
	if err != nil {
		fmt.Printf("Metadata: ERROR - failed to dispense plugin: %v\n", err)
		return nil
	}

	metadata, err := provider.Metadata(context.Background())
	if err != nil {
		fmt.Printf("Metadata: ERROR - %v\n", err)
		return nil
	}

	fmt.Printf("Namespace: %s\n", metadata.Namespace)
	fmt.Printf("Version: %s\n", metadata.Version)
	fmt.Printf("Description: %s\n", metadata.Description)
	if len(metadata.Dependencies) > 0 {
		fmt.Printf("Dependencies: %s\n", strings.Join(metadata.Dependencies, ", "))
	}
	if len(metadata.CLICommands) > 0 {
		fmt.Println("CLI Commands:")
		for _, cliCmd := range metadata.CLICommands {
			if len(cliCmd.Subcommands) > 0 {
				fmt.Printf("  %s (%s)\n", cliCmd.Name, strings.Join(cliCmd.Subcommands, ", "))
			} else {
				fmt.Printf("  %s\n", cliCmd.Name)
			}
		}
	}

	// Show if enabled
	config, err := state.LoadJackConfig()
	if err == nil {
		if pluginState, exists := config.Plugins[pluginName]; exists && pluginState.Enabled {
			fmt.Printf("Status: enabled (version %s)\n", pluginState.Version)
		} else {
			fmt.Println("Status: disabled")
		}
	}

	return nil
}

// getPluginMetadata returns a brief description of the plugin by querying its metadata
func getPluginMetadata(path string) string {
	client, err := plugins.NewPluginClient(path)
	if err != nil {
		return ""
	}
	defer client.Close()

	provider, err := client.Dispense()
	if err != nil {
		return ""
	}

	metadata, err := provider.Metadata(context.Background())
	if err != nil {
		return ""
	}

	return metadata.Description
}
