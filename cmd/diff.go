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

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show pending changes",
	Long:  `Displays the differences between pending and committed configuration.`,
	Run:   runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) {
	if err := executeDiff(cmd.OutOrStdout(), defaultClient); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// executeDiff executes the diff command with the given client.
func executeDiff(w io.Writer, client ClientInterface) error {
	resp, err := client.Send(daemon.Request{
		Command: "diff",
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
