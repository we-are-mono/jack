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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [checkpoint-id]",
	Short: "Rollback system to a previous checkpoint",
	Long: `Restore system configuration to a saved checkpoint.

If no checkpoint ID is provided, rolls back to the most recent checkpoint.
Use 'jack checkpoint list' to see available checkpoints.

Example:
  jack rollback                    # Rollback to latest checkpoint
  jack rollback auto-1234567890    # Rollback to specific checkpoint
  jack rollback manual-important   # Rollback to manual checkpoint`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var checkpointID string
		if len(args) > 0 {
			checkpointID = args[0]
		}

		resp, err := defaultClient.Send(daemon.Request{
			Command:      "rollback",
			CheckpointID: checkpointID,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if resp.Success {
			fmt.Printf("%s\n", resp.Message)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
