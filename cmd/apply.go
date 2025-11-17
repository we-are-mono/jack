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

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply committed configuration to the system",
	Long:  `Reads the committed configuration and applies it to network interfaces and firewall.`,
	Run:   runApply,
}

func init() {
	rootCmd.AddCommand(applyCmd)
}

func runApply(cmd *cobra.Command, args []string) {
	if err := executeApply(cmd.OutOrStdout(), defaultClient); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// executeApply executes the apply command with the given client.
func executeApply(w io.Writer, client ClientInterface) error {
	fmt.Fprintln(w, "Applying configuration...")

	resp, err := client.Send(daemon.Request{
		Command: "apply",
	})

	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}

	fmt.Fprintf(w, "[OK] %s\n", resp.Message)
	return nil
}
