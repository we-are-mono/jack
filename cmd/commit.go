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

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit pending changes",
	Long:  `Writes pending changes to disk, creating a backup of the previous configuration.`,
	Run:   runCommit,
}

func init() {
	rootCmd.AddCommand(commitCmd)
}

func runCommit(cmd *cobra.Command, args []string) {
	if err := executeCommit(cmd.OutOrStdout(), defaultClient); err != nil {
		cmd.PrintErrln(fmt.Sprintf("[ERROR] %v", err))
		exitWithError()
	}
}

// executeCommit executes the commit command with the given client.
func executeCommit(w io.Writer, client ClientInterface) error {
	resp, err := client.Send(daemon.Request{
		Command: "commit",
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
