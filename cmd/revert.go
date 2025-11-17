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

var revertCmd = &cobra.Command{
	Use:   "revert",
	Short: "Discard pending changes",
	Long:  `Removes all pending changes from memory, reverting to the committed configuration.`,
	Run:   runRevert,
}

func init() {
	rootCmd.AddCommand(revertCmd)
}

func runRevert(cmd *cobra.Command, args []string) {
	if err := executeRevert(cmd.OutOrStdout(), defaultClient); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// executeRevert executes the revert command with the given client.
func executeRevert(w io.Writer, client ClientInterface) error {
	resp, err := client.Send(daemon.Request{
		Command: "revert",
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
