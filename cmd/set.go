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
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
)

var setCmd = &cobra.Command{
	Use:   "set [path...] [value]",
	Short: "Set a configuration value",
	Long: `Sets a value in the configuration using space-separated path components.

If no path is provided, lists all available configuration namespaces.

Examples:
  jack set                                     # List available namespaces
  jack set interfaces br-lan ipaddr 192.168.1.1
  jack set routes default gateway 192.168.1.254
  jack set firewall zones wan input ACCEPT
  jack set firewall defaults input DROP
  jack set firewall enabled true
  jack set dhcp dhcp_pools lan start 100
  jack set dhcp server enabled true
  jack set led status:green brightness 127`,
	Run: runSet,
}

func init() {
	rootCmd.AddCommand(setCmd)
}

func runSet(cmd *cobra.Command, args []string) {
	if err := executeSet(cmd.OutOrStdout(), defaultClient, args); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// parseSetArgs parses the arguments for the set command.
// Returns empty path if no arguments (to list namespaces).
// It returns the path and the parsed value (bool, int, or string).
func parseSetArgs(args []string) (string, interface{}, error) {
	if len(args) < 1 {
		return "", nil, nil // Empty path will list available namespaces
	}

	if len(args) < 2 {
		return "", nil, fmt.Errorf("requires at least 2 arguments (path and value)")
	}

	// Last argument is always the value
	valueStr := args[len(args)-1]

	// Everything before that is the path
	pathParts := args[:len(args)-1]
	path := pathParts[0]
	for i := 1; i < len(pathParts); i++ {
		path += "." + pathParts[i]
	}

	// Parse value type (bool, int, or string)
	var value interface{}
	if valueStr == "true" {
		value = true
	} else if valueStr == "false" {
		value = false
	} else if i, err := strconv.Atoi(valueStr); err == nil {
		value = i
	} else {
		value = valueStr
	}

	return path, value, nil
}

// executeSet executes the set command with the given client and arguments.
func executeSet(w io.Writer, client ClientInterface, args []string) error {
	path, value, err := parseSetArgs(args)
	if err != nil {
		return err
	}

	// If no arguments provided, list available namespaces (same as get)
	if path == "" {
		resp, err := client.Send(daemon.Request{
			Command: "get",
			Path:    "",
		})

		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("%s", resp.Error)
		}

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
		return fmt.Errorf("unexpected response format")
	}

	resp, err := client.Send(daemon.Request{
		Command: "set",
		Path:    path,
		Value:   value,
	})

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Fprintln(w, resp.Message)
	return nil
}
