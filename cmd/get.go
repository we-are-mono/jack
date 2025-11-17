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
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
)

var getCmd = &cobra.Command{
	Use:   "get [path...]",
	Short: "Get a configuration value",
	Long: `Gets a value from the configuration using space-separated path components.

If no path is provided, lists all available configuration namespaces.

Examples:
  jack get                              # List available namespaces
  jack get interfaces br-lan ipaddr
  jack get routes default gateway
  jack get firewall zones wan input
  jack get firewall defaults input
  jack get firewall enabled
  jack get dhcp dhcp_pools lan start
  jack get dhcp server enabled
  jack get led status:green brightness`,
	Run: runGet,
}

func init() {
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) {
	if err := executeGet(cmd.OutOrStdout(), defaultClient, args); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// parseGetPath constructs a path from the command arguments.
// Returns empty string if no arguments provided (to list namespaces).
func parseGetPath(args []string) (string, error) {
	if len(args) < 1 {
		return "", nil // Empty path will list available namespaces
	}

	path := args[0]
	for i := 1; i < len(args); i++ {
		path += "." + args[i]
	}

	return path, nil
}

// executeGet executes the get command with the given client and arguments.
func executeGet(w io.Writer, client ClientInterface, args []string) error {
	path, err := parseGetPath(args)
	if err != nil {
		return err
	}

	resp, err := client.Send(daemon.Request{
		Command: "get",
		Path:    path,
	})

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	// Special handling for namespace listing (when path is empty)
	if path == "" {
		if categories, ok := resp.Data.(map[string]interface{}); ok {
			fmt.Fprintln(w, "Available configuration namespaces:")

			// Display in a fixed order for consistency
			categoryOrder := []string{"core", "firewall", "vpn", "dhcp", "monitoring", "hardware", "other"}

			for _, category := range categoryOrder {
				if namespacesRaw, exists := categories[category]; exists {
					// Convert interface{} to []interface{}
					if namespaces, ok := namespacesRaw.([]interface{}); ok && len(namespaces) > 0 {
						fmt.Fprintf(w, "\n%s:\n", category)
						for _, ns := range namespaces {
							fmt.Fprintf(w, "  %v\n", ns)
						}
					}
				}
			}
			return nil
		}
	}

	// Normal value retrieval - format as JSON
	data, err := json.MarshalIndent(resp.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Fprintln(w, string(data))
	return nil
}
