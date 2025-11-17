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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Manage configuration checkpoints",
	Long: `Manage system configuration checkpoints for rollback.

Checkpoints are automatically created before each apply operation,
and can also be created manually for important configuration states.`,
}

var checkpointListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available checkpoints",
	Long: `List all available configuration checkpoints.

Checkpoints include:
  - auto-*: Automatically created before each apply
  - manual-*: Manually created checkpoints

Use 'jack rollback <checkpoint-id>' to restore a checkpoint.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := defaultClient.Send(daemon.Request{
			Command: "checkpoint-list",
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if !resp.Success {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
			os.Exit(1)
		}

		// Parse response data
		dataBytes, err := json.Marshal(resp.Data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse response: %v\n", err)
			os.Exit(1)
		}

		var snapshots []daemon.SnapshotInfo
		if err := json.Unmarshal(dataBytes, &snapshots); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse snapshots: %v\n", err)
			os.Exit(1)
		}

		if len(snapshots) == 0 {
			fmt.Println("No checkpoints available")
			return
		}

		fmt.Printf("%-30s %-25s %-10s\n", "CHECKPOINT ID", "TIMESTAMP", "TRIGGER")
		fmt.Println(strings.Repeat("-", 70))

		for _, snapshot := range snapshots {
			// Format timestamp
			timestamp, err := time.Parse(time.RFC3339, snapshot.Timestamp.Format(time.RFC3339))
			if err != nil {
				timestamp = snapshot.Timestamp
			}

			fmt.Printf("%-30s %-25s %-10s\n",
				snapshot.CheckpointID,
				timestamp.Format("2006-01-02 15:04:05"),
				snapshot.Trigger,
			)
		}
	},
}

var checkpointCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a manual checkpoint",
	Long: `Create a manual checkpoint of the current system state.

This captures the current configuration for potential rollback later.
Manual checkpoints are prefixed with 'manual-' followed by a timestamp.`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := defaultClient.Send(daemon.Request{
			Command: "checkpoint-create",
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
	checkpointCmd.AddCommand(checkpointListCmd)
	checkpointCmd.AddCommand(checkpointCreateCmd)
	rootCmd.AddCommand(checkpointCmd)
}
